# dbx Repositories

`dbx` provides typed generic repositories on top of [`sqlb`](sqlb.md), with a `database/sql`
adapter and a transaction helper. Unlike `db.Repository`, `dbx.Repository[T]` returns `*T`/`[]*T`
from every read, takes a `context.Context` per call instead of storing one at construction, and
builds every statement through `sqlb`: values are always bound and identifiers are always
escaped. See [Migrating to dbx](migrating-to-dbx.md) for a full `db`/goqu → `dbx`/`sqlb` mapping.

## Overview

- `dbx.Repository[T]` is a typed repository over `T`, backed by a `dbx.Querier`.
- `dbx.FromClient(*db.SqlClient)` builds a `Querier`/`TxBeginner` over an existing
  `database/sql`-backed client, resolving the dialect from the client's driver name.
- ClickHouse does not use `database/sql`; `provider/clickhouse.Client.Querier()` returns a
  `*clickhouse.Querier` that implements `dbx.Querier` and `dbx.BatchInserter` directly over a
  native ClickHouse connection.
- `dbx.Grid[T]` builds `sqlb` queries from a client-supplied `GridQuery`, restricted to fields
  the record type flags with `grid:"sort"`/`"filter"`/`"search"`.

## Setting up a repository

```go
q, err := dbx.FromClient(client) // client is a *db.SqlClient (e.g. from provider/sqlite)
repo, err := dbx.NewRepository[User](q, "users")
```

`NewRepository[T]` resolves `T`'s columns via `sqlb.RecordColumns`, which applies the same
record-shape checks as `sqlb` (see [sqlb: Record shapes](sqlb.md#record-shapes)): a shape it
cannot map safely fails here, at setup, rather than on the first query. Because `dbx` selects an
explicit column list (`repo.Select()` renders `SELECT <T's columns> FROM <table>`, never
`SELECT *`), every field `T` maps must exist as a column — the old `db` package's `SELECT *`
tolerated extra table columns a struct didn't map; `dbx` does not.

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
u, err := repo.Get(ctx, repo.Select().Where(sqlb.Col("id").Eq(1)))
users, err := repo.List(ctx, repo.Select().Where(sqlb.Col("active").Eq(true)))

u, err = repo.GetBy(ctx, map[string]any{"email": "alice@example.com"})
users, err = repo.ListBy(ctx, map[string]any{"active": true})

n, err := repo.Count(ctx, sqlb.Col("active").Eq(true)) // nil where counts every row
ok, err := repo.Exists(ctx, sqlb.Col("email").Eq("alice@example.com"))
```

A custom query passed to `Get`/`List` should start from `repo.Select()`, not a bare
`sqlb.From(table)`: `sqlb.From` renders `SELECT *`, which `sqlx`'s row scanning rejects when the
table has columns `T` does not map. `GetBy`/`ListBy`/`UpdateFields` check their map's keys
against the repository's known columns before running any query — an unknown key fails with
`dbx.ErrUnknownColumn`, not a database error.

## Writes: Insert, Update, UpdateFields, Delete, Upsert

`Delete`, `Update` and `UpdateFields` require a non-nil `where` and fail immediately with
`sqlb.ErrNoWhere` — without touching the database — when it is nil. There is deliberately no
"delete all"/"update all" method on `Repository`; a caller who means it uses
`repo.Exec(ctx, sqlb.Delete(table).All())` (or `sqlb.Update(table).All()`) directly.

```go
if err := repo.Insert(ctx, &User{Name: "alice", Email: "alice@example.com"}); err != nil { ... }

n, err := repo.Update(ctx, &User{Name: "alice2"}, sqlb.Col("id").Eq(1))
n, err = repo.UpdateFields(ctx, map[string]any{"name": "alice2"}, sqlb.Col("id").Eq(1))
n, err = repo.Delete(ctx, sqlb.Col("id").Eq(1))

err = repo.Upsert(ctx, &User{Email: "alice@example.com", Name: "alice"},
    []string{"email"}) // conflict columns; updates every other written column
```

`Insert` writes multiple records in one call: `repo.Insert(ctx, rec1, rec2, rec3)`. If the bound
`Querier` implements `dbx.BatchInserter` (the ClickHouse adapter), `InsertBatch` is used instead
of building `INSERT` statements. Otherwise records are chunked at
`Dialect().MaxArgs() / <column count>` (one statement when `MaxArgs() == 0`, as on ClickHouse);
more than one chunk runs atomically inside a transaction via `WithTx` — `Insert` never issues a
silent, non-atomic multi-statement write. If the bound `Querier` cannot begin a transaction,
`Insert` fails with `dbx.ErrTxUnsupported` rather than writing some chunks and not others.

`InsertReturning` and `Upsert` require `sqlb.FeatureReturning`/`FeatureUpsert` — unsupported on
ClickHouse (`sqlb.ErrUnsupported`).

## Transactions: WithTx

```go
err := dbx.WithTx(ctx, q, nil, func(tx dbx.Querier) error {
    txRepo := repo.With(tx)
    if err := txRepo.Insert(ctx, &User{Name: "bob"}); err != nil {
        return err
    }
    _, err := txRepo.Delete(ctx, sqlb.Col("name").Eq("alice"))
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
    ID    int64  `db:"id,auto" grid:"sort,filter"`
    Name  string `db:"name" grid:"sort,search"`
    Email string `db:"email" grid:"search"`
}

grid, err := dbx.NewGrid[User]()
grid = grid.WithMaxLimit(100)

q, err := dbx.NewGridQuery(dbx.SearchAny, 10, 0)
q.SearchText = "ali"
q.FilterFields = map[string]any{"id": 1}
q.SortFields = map[string]string{"name": dbx.SortAscending}

users, err := repo.QueryGrid(ctx, grid, q)
```

Only grid-flagged fields are addressable by alias (the struct's `db` name unless overridden);
every other field, tagged or not, answers "field is not valid". `NewGrid` fails outright — not
silently — on a duplicate, empty or `"-"` alias among grid-flagged fields, or a searchable field
that is neither string-kind nor a `database/sql/driver.Valuer`.

Filter values are checked against an allowlist (JSON scalars and flat lists, capped at
`dbx.MaxFilterValues` elements); `SearchText` is capped at `dbx.MaxSearchText` bytes;
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

- `Exec` always returns `0` rows affected — ClickHouse does not report the number of rows a
  statement affected.
- `InsertBatch` derives its column list from the first row's `sqlb.InsertColumns`, not an
  unqualified `INSERT INTO t` (which would require every non-`MATERIALIZED`/`ALIAS` column of
  `t`); a column name containing `"`, `\`, `,` or a space cannot be used, since the driver's batch
  column-list parser strips quotes with a regex without un-escaping.
- `Count`/`Exists` read `COUNT()` — `Exists`'s wrapping subquery is specifically shaped so
  `QueryInt64` reads a `COUNT` column even though a bare `SELECT 1` on ClickHouse is `UInt8`, not
  the `UInt64` `COUNT()` returns.
- A bound `time.Time` is sent at second precision — a `clickhouse-go` v2.40.3 limitation, not a
  `dbx`/`sqlb` choice.
- `Querier` is not transactional (no `TxBeginner`/`TxQuerier`): `dbx.WithTx` fails with
  `dbx.ErrTxUnsupported` over it, matching ClickHouse having no transactions.

## Dialect support

| Capability | PostgreSQL/SQLite (`FromClient`) | ClickHouse (`Client.Querier()`) |
|---|---|---|
| `Get`/`List`/`GetBy`/`ListBy` | yes | yes |
| `Insert` (chunked, transactional) | yes | uses `InsertBatch` instead |
| `Update`/`UpdateFields`/`Delete` | yes | `Delete` only (no `UPDATE`) |
| `InsertReturning`/`Upsert` | yes | no (`sqlb.ErrUnsupported`) |
| `WithTx` | yes | no (`dbx.ErrTxUnsupported`) |
| Rows-affected reporting | real count | always `0` |
| `Grid[T]` | yes | yes |

## Limitations

- `dbx.FromClient` assumes `sqlx`'s default field-name mapper; a repository bound to a `*sqlx.DB`
  with a custom mapper is not supported.
- `dbx.FromClient` snapshots the client's connection: call it once at startup, not after a later
  `Disconnect`/`Connect` cycle.
- ClickHouse has no `UPDATE`, `RETURNING`, `ON CONFLICT` upsert, or transactions — see the
  ClickHouse section above and [sqlb's dialect support table](sqlb.md#dialect-support).
- `WithTx`'s `fn` must run every statement through `repo.With(tx)`, not the outer repository —
  nothing in the type system enforces this (see the Transactions section above).
