# dbx Repositories

`dbx` provides typed generic repositories on top of [`gohan`](gohan.md), with a `database/sql`
adapter and a transaction helper. Unlike `db.Repository`, `dbx.Repository[T]` returns `*T`/`[]*T`
from every read, takes a `context.Context` per call instead of storing one at construction, and
builds every statement through `gohan`: values are always bound and identifiers are always
escaped. See [Migrating to dbx](migrating-to-dbx.md) for a full `db`/goqu → `dbx`/`gohan` mapping.

## Overview

- `dbx.Repository[T]` is a typed repository over `T`, backed by a `dbx.Querier`.
- `dbx.FromClient(*db.SqlClient)` builds a `Querier`/`TxBeginner` over an existing
  `database/sql`-backed client, resolving the dialect from the client's driver name.
- ClickHouse does not use `database/sql`; `provider/clickhouse.Client.Querier()` returns a
  `*clickhouse.Querier` that implements `dbx.Querier` and `dbx.BatchInserter` directly over a
  native ClickHouse connection.
- `dbx.Grid[T]` builds `gohan` queries from a client-supplied `GridQuery`, restricted to fields
  the record type flags with `grid:"sort"`/`"filter"`/`"search"`.

## Setting up a repository

```go
q, err := dbx.FromClient(client) // client is a *db.SqlClient (e.g. from provider/sqlite)
repo, err := dbx.NewRepository[User](q, "users")
```

`NewRepository[T]` resolves `T`'s columns via `gohan.RecordColumns`, which applies the same
record-shape checks as `gohan` (see [gohan: Record shapes](https://github.com/oddbit-project/gohan#record-shapes)): a shape it
cannot map safely fails here, at setup, rather than on the first query. Because `dbx` selects an
explicit column list (`repo.Select()` renders `SELECT <T's columns> FROM <table>`, never
`SELECT *`), every field `T` maps must exist as a column, or the query fails ("missing
destination name") — the same failure the old `db` package's `SELECT *` had (`db.Fetch` calls
sqlx's `SelectContext` without `Unsafe()`, which fails the same way on an unmapped column).
What's different is the other direction: `dbx`'s explicit column list *tolerates* extra table
columns the struct doesn't map (they're simply never selected), where the old `SELECT *` would
have included — and then failed to scan — them too.

(`dbx/example_test.go`'s `ExampleNewRepository` is a full, compiling setup — client, table
creation, insert, get, list, transaction.)

## Not-found semantics

`dbx.ErrNotFound` is an alias for `sql.ErrNoRows`, so `errors.Is(err, sql.ErrNoRows)` keeps
working against code written against `database/sql` or `db.Repository`:

```go
user, err := repo.GetBy(ctx, map[string]any{"email": "missing@example.com"})
if errors.Is(err, dbx.ErrNotFound) {
    // no matching row
}
```

`Get`/`GetBy` return `(nil, ErrNotFound)` when no row matches. `Count`/`Exists` never surface
`ErrNotFound` — a `COUNT(*)` query always returns exactly one row.

## Reads: Get, List, GetBy, ListBy, Count, Exists

```go
u, err := repo.Get(ctx, repo.Select().Where(gohan.Col("id").Eq(1)))
users, err := repo.List(ctx, repo.Select().Where(gohan.Col("active").Eq(true)))

u, err = repo.GetBy(ctx, map[string]any{"email": "alice@example.com"})
users, err = repo.ListBy(ctx, map[string]any{"active": true})

n, err := repo.Count(ctx, gohan.Col("active").Eq(true)) // nil where counts every row
ok, err := repo.Exists(ctx, gohan.Col("email").Eq("alice@example.com"))
```

A custom query passed to `Get`/`List` should start from `repo.Select()`, not a bare
`gohan.From(table)`: `gohan.From` renders `SELECT *`, which `sqlx`'s row scanning rejects when the
table has columns `T` does not map. `GetBy`/`ListBy`/`UpdateFields` check their map's keys
against the repository's known columns before running any query — an unknown key fails with
`dbx.ErrUnknownColumn`, not a database error.

## Writes: Insert, Update, UpdateFields, Delete, Upsert

`Delete`, `Update` and `UpdateFields` require a non-nil `where` and fail immediately with
`gohan.ErrNoWhere` — without touching the database — when it is nil. There is deliberately no
"delete all"/"update all" method on `Repository`; a caller who means it uses
`repo.Exec(ctx, gohan.Delete(table).All())` (or `gohan.Update(table).All()`) directly.

```go
if err := repo.Insert(ctx, &User{Name: "alice", Email: "alice@example.com"}); err != nil { ... }

// Update sets every non-auto field of the record, so a partial struct
// literal overwrites the fields it leaves zero (Email would be written as
// "" here without IncludeFields):
n, err := repo.Update(ctx, &User{Name: "alice2"}, gohan.Col("id").Eq(1),
    gohan.IncludeFields("name"))
// or pass a full record to write every field:
n, err = repo.Update(ctx, &User{Name: "alice2", Email: "alice@example.com"}, gohan.Col("id").Eq(1))

n, err = repo.UpdateFields(ctx, map[string]any{"name": "alice2"}, gohan.Col("id").Eq(1))
n, err = repo.Delete(ctx, gohan.Col("id").Eq(1))

err = repo.Upsert(ctx, &User{Email: "alice@example.com", Name: "alice"},
    []string{"email"}) // conflict columns; updates every other written column
```

`Update(ctx, rec, where, opts...)` sets every non-auto field of `rec` per `opts`
(`gohan.RecordOption`s: `gohan.IncludeFields`, `gohan.ExcludeFields`, `gohan.SkipZeroValues`,
`gohan.WithAutoFields`) — the same options as `gohan.UpdateBuilder.SetRecord`. A partial struct
literal without `IncludeFields`/`SkipZeroValues` writes its zero-valued fields too, overwriting
whatever they held. `UpdateFields` sets exactly the map's keys and is the more common choice for
a partial update.

`Insert` writes multiple records in one call: `repo.Insert(ctx, rec1, rec2, rec3)`. If the bound
`Querier` implements `dbx.BatchInserter` (the ClickHouse adapter), `InsertBatch` is used instead
of building `INSERT` statements. Otherwise records are chunked at
`Dialect().MaxArgs() / <column count>` (one statement when `MaxArgs() == 0`, as on ClickHouse);
more than one chunk runs atomically inside a transaction via `WithTx` — `Insert` never issues a
silent, non-atomic multi-statement write. If the bound `Querier` cannot begin a transaction,
`Insert` fails with `dbx.ErrTxUnsupported` rather than writing some chunks and not others.

`InsertReturning` and `Upsert` require `gohan.FeatureReturning`/`FeatureUpsert` — unsupported on
ClickHouse (`gohan.ErrUnsupported`).

## Transactions: WithTx

```go
err := dbx.WithTx(ctx, q, nil, func(tx dbx.Querier) error {
    txRepo := repo.With(tx)
    if err := txRepo.Insert(ctx, &User{Name: "bob"}); err != nil {
        return err
    }
    _, err := txRepo.Delete(ctx, gohan.Col("name").Eq("alice"))
    return err
})
```

`WithTx` joins an existing transaction if `q` is already a `TxQuerier` (composable: no nested
`BEGIN`, no commit/rollback — the outer `WithTx` owns those), begins one if `q` is a
`TxBeginner`, or fails with `dbx.ErrTxUnsupported` otherwise. Inside `fn`, use `repo.With(tx)` —
using the outer repository (bound to the non-transactional `Querier`) runs the statement outside
the transaction, and on SQLite with `MaxOpenConns == 1` this deadlocks waiting for the only
connection, which `WithTx` is already holding.

## Grid[T]

```go
type User struct {
    ID    int64  `db:"id,auto" json:"id" grid:"sort,filter"`
    Name  string `db:"name" json:"name" grid:"sort,search"`
    Email string `db:"email" json:"email" grid:"search"`
}

grid, err := dbx.NewGrid[User]()
grid = grid.WithMaxLimit(100) // configure before concurrent use — WithMaxLimit/AddFilterFunc mutate the grid

q, err := dbx.NewGridQuery(dbx.SearchAny, 10, 0)
q.SearchText = "ali"
q.FilterFields = map[string]any{"id": float64(1)} // GridQuery is JSON-shaped: numbers are float64
q.SortFields = map[string]string{"name": dbx.SortAscending}

users, err := repo.QueryGrid(ctx, grid, q)
```

Only grid-flagged fields are addressable by alias — the struct's **Go field name** by default
(`"ID"`, `"Name"`, `"Email"` above), or an `alias`/`json`/`xml` tag when one is set (the `json`
tags above make the aliases `"id"`/`"name"`/`"email"`, matching what a JSON client would send);
every other field, tagged or not, answers "field is not valid". `NewGrid` fails outright — not
silently — on a duplicate, empty or `"-"` alias among grid-flagged fields, or a searchable field
that is neither string-kind nor a `database/sql/driver.Valuer`.

`AddFilterFunc` and `WithMaxLimit` mutate the `Grid` in place — unlike `gohan`'s builders and
`Repository.With`, which are immutable/return a new value. Call them once at setup, before the
grid is used concurrently.

`dbx.Grid.Build` rejects a compound (`UNION`/`UNION ALL`) base query outright (`base.IsCompound()`
— see [gohan: UNION](https://github.com/oddbit-project/gohan#union)): `Where`/`Having`/`Prewhere` on a compound builder would only
filter its first member, not the whole result set the grid is supposed to page over. Wrap a
compound base in a subquery instead: `gohan.From(q.As("u"))`.

Filter values are checked against an allowlist matching what `encoding/json` produces — `nil`,
`bool`, `float64`, `string`, `json.Number`, or a flat `[]any` of those, capped at
`dbx.MaxFilterValues` elements — so a Go `int` (as opposed to `float64`) is rejected; build
`GridQuery` from `json.Unmarshal`ed request data (the common case) to get this for free, or use
`float64` explicitly in a literal Go map, as above. `SearchText` is capped at `dbx.MaxSearchText` bytes;
`GridQuery.SearchType` is range-checked by `ValidQuery`/`NewGridQuery`. `WithMaxLimit(n)` caps
`Limit`: `n == 0` (the default) applies no cap, so `Limit == 0` returns every row; with a cap set,
`Limit == 0` or `Limit > n` is treated as `Limit == n`. `GridQuery` fields decode from JSON as
`{"searchType": ..., "searchText": ..., "filterFields": ..., "sortFields": ..., "offset": ...,
"limit": ...}` — `searchType`'s JSON name, not `search_type`; decoding is case-insensitive like
the rest of `encoding/json`. Because JSON numbers decode as `float64`, a filter value like
`{"id": 3.9}` matches `id = 3` on PostgreSQL (`pgx` truncates); register a `GridFilterFunc` via
`AddFilterFunc` on integer columns to reject non-integer input if that matters.

## ClickHouse

```go
q := client.Querier() // *clickhouse.Querier, from provider/clickhouse
repo, err := dbx.NewRepository[Event](q, "events")
```

`provider/clickhouse.Client.Querier()`/`clickhouse.NewQuerier(conn)` return a `*clickhouse.Querier`
implementing `dbx.Querier` and `dbx.BatchInserter` directly over a native `clickhouse-go`
connection — ClickHouse doesn't use `database/sql`, binds placeholders client-side, and can't
fill a slice of pointers through its own `Select`, so `dbx.FromClient` (the `database/sql`
adapter) explicitly rejects the ClickHouse driver with `dbx.ErrDialectDriver`.

### Record types need matching `ch` tags

`gohan`/`dbx` build column lists (`SELECT`, `InsertBatch`'s column list) from a record's `db` tag
(or the Go field name). But `clickhouse.Querier` scans and appends rows through clickhouse-go's
own `ScanStruct`/`AppendStruct`, which map columns to fields by the **`ch`** struct tag (also
falling back to the Go field name) — a different tag, read by different code. A record type used
against `clickhouse.Querier` needs `ch` tags equal to its `db` tags (or the field names to already
agree, or only `ch` tags set), or the column list `gohan`/`dbx` builds and the fields clickhouse-go
actually scans into silently diverge:

```go
type Event struct {
    ID   uint32   `ch:"id" db:"id,auto"`
    Name string   `ch:"name" db:"name"`
    Tags []string `ch:"tags" db:"tags"`
}
```

- `Exec` always returns `0` rows affected — ClickHouse does not report the number of rows a
  statement affected.
- `InsertBatch` derives its column list from the first row's `gohan.InsertColumns`, not an
  unqualified `INSERT INTO t` (which would require every non-`MATERIALIZED`/`ALIAS` column of
  `t`); a column name containing `"`, `\`, `,` or a space cannot be used, since the driver's batch
  column-list parser strips quotes with a regex without un-escaping. Every row must agree on
  which columns are omitted (`OmitNil`/`OmitEmpty`, e.g. a nil pointer with an `omitnil` tag) — a
  row whose omitted-column set differs from row 0's fails the whole batch with
  `gohan.ErrInconsistentOmit`, since the batch's column list is fixed once, by row 0.
- `Count`/`Exists` read `COUNT(*)` — `Exists`'s wrapping subquery is specifically shaped so
  `QueryInt64` reads a `COUNT` column even though a bare `SELECT 1` on ClickHouse is `UInt8`, not
  the `UInt64` `COUNT(*)` returns.
- A bound `time.Time` is sent at second precision — a `clickhouse-go` v2.40.3 limitation, not a
  `dbx`/`gohan` choice.
- `Querier` is not transactional (no `TxBeginner`/`TxQuerier`): `dbx.WithTx` fails with
  `dbx.ErrTxUnsupported` over it, matching ClickHouse having no transactions.

## Dialect support

| Capability | PostgreSQL/SQLite (`FromClient`) | ClickHouse (`Client.Querier()`) |
|---|---|---|
| `Get`/`List`/`GetBy`/`ListBy` | yes | yes |
| `Insert` (chunked, transactional) | yes | uses `InsertBatch` instead |
| `Update`/`UpdateFields`/`Delete` | yes | `Delete` only (no `UPDATE`) |
| `InsertReturning`/`Upsert` | yes | no (`gohan.ErrUnsupported`) |
| `WithTx` | yes | no (`dbx.ErrTxUnsupported`) |
| Rows-affected reporting | real count | always `0` |
| `Grid[T]` | yes | yes |

## Limitations

- `dbx.FromClient` assumes `sqlx`'s default field-name mapper; a repository bound to a `*sqlx.DB`
  with a custom mapper is not supported.
- `dbx.FromClient` snapshots the client's connection: call it once at startup, not after a later
  `Disconnect`/`Connect` cycle.
- ClickHouse has no `UPDATE`, `RETURNING`, `ON CONFLICT` upsert, or transactions — see the
  ClickHouse section above and [gohan's dialect support table](https://github.com/oddbit-project/gohan#dialect-support).
- `WithTx`'s `fn` must run every statement through `repo.With(tx)`, not the outer repository —
  nothing in the type system enforces this (see the Transactions section above).
- `dbx` and `gohan` each define a similarly-named error for a different failure, and both can
  surface from the same call — check the specific sentinel, not just the name: `dbx.ErrNoColumns`
  is returned by `NewRepository` when the record type maps zero columns, while
  `gohan.ErrNoColumns` means a statement (`Insert`/`Update`) ended up with nothing to write;
  `dbx.ErrUnknownColumn` is returned when a caller-supplied map key (`GetBy`/`ListBy`/
  `UpdateFields`) isn't one of the repository's known columns, while `gohan.ErrUnknownField` is
  `IncludeFields`/`ExcludeFields`/`DoUpdateExcluded` naming a field or column `gohan` doesn't
  recognize on the record/statement in question.
