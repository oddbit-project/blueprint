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

### Insert-or-ignore: InsertIgnore

```go
inserted, err := repo.InsertIgnore(ctx, &User{Name: "alice", Email: "alice@example.com"}, "email")
// inserted == false: a row with that email already existed; nothing was written
inserted, err = repo.InsertIgnore(ctx, rec) // no columns: ON CONFLICT DO NOTHING, any unique constraint
```

`InsertIgnore` renders `INSERT ... ON CONFLICT [(cols)] DO NOTHING` and reports whether the row was
inserted (rows affected > 0) instead of failing on a conflict. With no conflict columns the clause is
untargeted and any unique or exclusion constraint violation is skipped. Conflict names are checked
against the repository's columns (`dbx.ErrUnknownColumn`); ClickHouse fails with
`gohan.ErrUnsupported`.

### Reading back writes: UpdateReturning, UpsertReturning

```go
// every updated row, as stored (triggers and defaults included)
users, err := repo.UpdateReturning(ctx, map[string]any{"active": false},
    gohan.Col("last_login").Lt(cutoff))

// the inserted or updated row; rec itself is left untouched
u, err := repo.UpsertReturning(ctx, rec, []string{"email"})
if errors.Is(err, dbx.ErrNotFound) {
    // the upsert resolved to DO NOTHING and the row already existed
}
```

`UpdateReturning` is `UpdateFields` plus `RETURNING <all of T's columns>`: the same guards apply
before any query (`gohan.ErrNoWhere`, `dbx.ErrUnknownColumn`), and it returns an empty, non-nil slice
when nothing matched. `UpsertReturning` builds the same statement as `Upsert` plus `RETURNING`, and
scans into a new `T` — never into the record you passed, so a failed scan cannot leave it
half-overwritten. When every column the record writes is a conflict column, `Upsert`'s statement is
`DO NOTHING`; a conflicting row then returns nothing, and `UpsertReturning` fails with
`dbx.ErrNotFound`. Both need `RETURNING` (PostgreSQL, SQLite 3.35+); ClickHouse fails with
`gohan.ErrUnsupported`.

### Records that omit different columns: WithGroupedInserts

A record field tagged `omitnil`/`omitempty` is left out of the `INSERT` when it is nil/zero, so the
column takes its database default. `Insert` puts all records of a chunk in one statement with one
column list, so records that omit different columns fail with `gohan.ErrInconsistentOmit` (and a
record that writes no column at all with `gohan.ErrNoColumns`). Opt in to grouping to accept them:

```go
grouped := repo.WithGroupedInserts() // a copy; repo keeps the default behaviour
err := grouped.Insert(ctx, &User{Name: "a"}, &User{Name: "b", Bio: &bio}, &User{Name: "c"})
// INSERT ... ("name") VALUES (...), (...)   -- a, c
// INSERT ... ("name", "bio") VALUES (...)   -- b
```

Records are grouped by the set of columns they write; each group is chunked like a plain `Insert`,
and a record that writes no column becomes its own `INSERT INTO <table> DEFAULT VALUES`. More than one
statement runs atomically through `WithTx` (so `dbx.ErrTxUnsupported` without transaction support).
Groups are written in order of their first record, not argument order, so auto-generated ids
follow the groups. `With` keeps the option. It does not apply to a `BatchInserter` Querier: on
ClickHouse, `InsertBatch` still rejects mixed records, because several batches cannot be written
atomically there.

## Partial updates (PATCH)

`UpdateFields` writes whatever map it is given. For a PATCH endpoint, two helpers build that map safely:
`dbx.Changeset[T]`, fed from a request body, and `dbx.Changes`, which diffs two copies of a record.

### From a request body: Changeset and Optional

Decode the body into a struct of [`optional.Optional[T]`](../types/types.md#optional) fields, so an absent field
(None), an explicit `null` (Null) and a value (Some) stay distinct, then apply each field to a changeset:

```go
type UserPatch struct {
    Name  optional.Optional[string] `json:"name,omitzero"`
    Phone optional.Optional[string] `json:"phone,omitzero"` // phone is a *string field
    Age   optional.Optional[int]    `json:"age,omitzero"`
}

var p UserPatch
if err := json.Unmarshal(body, &p); err != nil { ... }

cs := dbx.NewChangeset(repo)
if err := errors.Join(
    dbx.SetOptional(cs, "name", p.Name),
    dbx.SetOptional(cs, "phone", p.Phone),
    dbx.SetOptional(cs, "age", p.Age),
); err != nil {
    return err // a client error: each message names the column
}

changes := cs.Changes()
if len(changes) == 0 {
    return nil // nothing to update
}
n, err := repo.UpdateFields(ctx, changes, gohan.Col("id").Eq(id))
```

`SetOptional` skips None, sets Null as `nil` and Some as its value. `cs.Set(column, value)` can also be called
directly. Every value is checked against `T` before it is accepted, and a rejected `Set` leaves the changeset
unchanged:

| Error | When |
|-------|------|
| `dbx.ErrUnknownColumn` | `column` is not one of the repository's columns (db column names, not Go field names) |
| `dbx.ErrAutoColumn` | the field is marked auto (`db:",auto"`, `auto:"true"`, `grid:"auto"`, `goqu:"skipinsert"`/`"skipupdate"`) |
| `dbx.ErrValueType` | the value does not fit the field's Go type |

A value fits when it is assignable to the field's type or converts to it without loss:

- Numbers convert between integer and float kinds only when exactly representable: `float64(30)` (what
  `encoding/json` produces for `30` in an `any`) fits an `int` field; `30.7`, `NaN`, `±Inf`, an out-of-range
  value or a negative value for an unsigned field do not. An integer too large to be exact in a float field is
  rejected; a `float64` into a `float32` field is rounded, and only rejected beyond `float32`'s range.
- A `json.Number` (a body decoded into `any` with `json.Decoder.UseNumber`) fits a numeric field under the same
  rules, parsed exactly from its text: an integer field takes only an integer literal (`30`, not `30.0` or `3e1`),
  so an integer above 2^53 arrives intact; a float field takes any JSON number. A non-numeric `json.Number`, or one
  for a non-numeric field (including a string field), is rejected.
- A plain `float64` from `encoding/json` has already lost precision above 2^53 before `Set` sees it: `Set` checks
  the `float64` it is given, not the original text. For integer columns that can exceed 2^53 (ids, counters),
  declare typed fields in the PATCH struct (`optional.Optional[int64]`), which decode the number exactly, or
  decode with `UseNumber`.
- Other conversions are allowed only within the same kind (`string` into a named string type) or between
  `string` and `[]byte`. An `int` is never turned into a one-character string, and a string is never parsed into
  a `time.Time`: decode such values first (`Optional[time.Time]` does this).
- A pointer field accepts the pointee or a pointer; the stored value is the pointee (`nil` for a nil pointer).
- `nil` is accepted only by fields that can hold NULL: pointers, interfaces, maps, slices, and NULL-style structs,
  that is, structs whose pointer implements `sql.Scanner` and that have an exported `bool` field named `Valid`
  (`sql.NullString`, `sql.Null[T]`, pgtype types). Other `sql.Scanner` types such as `uuid.UUID` and
  `jsoncol.JSON[T]` reject `nil`; declare the field as a pointer (`*uuid.UUID`, `*jsoncol.JSON[T]`) for a
  nullable column.

`Changes()` returns a copy of the accepted values; it is empty when nothing was set, and an `UPDATE` with no
columns fails to build, so check its length first.

### Load, modify, save the diff: Changes

```go
old, err := repo.GetBy(ctx, map[string]any{"id": id})
if err != nil { ... }
updated := *old
updated.Name = "alice2"
updated.Phone = nil

changes, err := dbx.Changes(old, &updated) // map[name:alice2 phone:<nil>] if phone was set
if err != nil { ... }
if len(changes) > 0 {
    _, err = repo.UpdateFields(ctx, changes, gohan.Col("id").Eq(id))
}
```

`dbx.Changes(oldRec, newRec *T)` returns `newRec`'s value for every non-auto column whose field differs, compared
with `reflect.DeepEqual`: pointers by pointee, slices and maps by content. A `time.Time` compares strictly, so
the same instant in another location, or with its monotonic reading stripped, counts as a change (a redundant
write, never a missed one). Pointer fields contribute their pointee, or `nil`. It fails with `dbx.ErrNilRecord`
for a nil record and with the same shape errors as `NewRepository` for a record type it would reject.

Copying a record (`updated := *old`) shares its slices and maps: modify them by assigning a new slice or map,
not in place, or both records change and no difference is found.

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

### Retrying transient failures: WithTxRetry

Under `sql.LevelSerializable` (or with lock contention) a transaction can fail only because another
one ran at the same time; running it again usually succeeds. `WithTxRetry` does that loop:

```go
opts := &sql.TxOptions{Isolation: sql.LevelSerializable}
err := dbx.WithTxRetry(ctx, q, opts, 5, func(tx dbx.Querier) error {
    txRepo := repo.With(tx)
    acc, err := txRepo.GetBy(ctx, map[string]any{"id": id})
    if err != nil {
        return err
    }
    _, err = txRepo.UpdateFields(ctx, map[string]any{"balance": acc.Balance - amount}, gohan.Col("id").Eq(id))
    return err
})
```

- Each attempt is a fresh `WithTx`: a failed attempt is rolled back before the next begins, and
  `fn` runs at most `attempts` times (a value below 1 is treated as 1). When every attempt fails,
  the last attempt's error is returned.
- Between attempts it waits a random delay below `min(2ms << n, 100ms)` (full jitter). If `ctx` is
  done during that wait it stops and returns `errors.Join(ctx.Err(), lastErr)`.
- Only errors the `Querier` classifies as transient are retried: `q` must implement
  `dbx.RetryClassifier` (`IsRetryable(err error) bool`). `SQLQuerier` does, by dialect —
  PostgreSQL SQLSTATE `40001` (serialization failure) and `40P01` (deadlock), for both pgx and
  lib/pq; SQLite `SQLITE_BUSY` and `SQLITE_LOCKED`, including extended codes such as
  `SQLITE_BUSY_SNAPSHOT` (modernc.org/sqlite, as used by `provider/sqlite`). Any other error, or a
  `Querier` without a classifier (e.g. ClickHouse, which fails with `dbx.ErrTxUnsupported`), returns
  after the first attempt, exactly like `WithTx`.
- If `q` is already a `TxQuerier`, `fn` joins that transaction and runs **once**, with no retry: a
  serialization failure aborts the outer transaction, so only the code that began it can retry.
  Put `WithTxRetry` at the outermost transaction boundary.

`fn` may run several times, so it must be idempotent: its database writes are rolled back with each
failed attempt, but anything it changes outside the transaction is not. In particular a record
declared outside `fn` and reused across attempts keeps the values a failed attempt assigned to it
(e.g. an id copied from `InsertReturning`'s result, whose row was rolled back) — build such records
inside `fn`.

## Grid[T]

```go
type User struct {
    ID    int64  `db:"id,auto" json:"id" grid:"sort,filter"`
    Name  string `db:"name" json:"name" grid:"sort,search"`
    Email string `db:"email" json:"email" grid:"search"`
}

grid, err := dbx.NewGrid[User]()
grid = grid.WithMaxLimit(100).WithTiebreaker("id") // configure before concurrent use — these mutate the grid

q, err := dbx.NewGridQuery(dbx.SearchAny, 10, 0)
q.SearchText = "ali"
q.FilterFields = map[string]any{"id": float64(1)} // GridQuery is JSON-shaped: numbers are float64
q.Sort = []dbx.SortField{{Field: "name", Order: dbx.SortAscending}}

users, err := repo.QueryGrid(ctx, grid, q)

// the page plus the total number of matching rows
users, total, err := repo.QueryGridWithCount(ctx, grid, q)
```

**Totals.** `QueryGridWithCount` returns `QueryGrid`'s rows plus the number of rows the query's
filters and search match: it counts with `repo.Count` over `grid.Conds(q)`, so the `COUNT` carries
no `ORDER BY`, `LIMIT` or `OFFSET` (an `ORDER BY` on a bare `COUNT` fails on PostgreSQL and
ClickHouse, and an `OFFSET` past the first row would count nothing). The rows and the total are two
separate statements, not one snapshot: under concurrent writes the total can disagree with the page.
If that matters, both statements must read the same snapshot, which a transaction alone does not
guarantee everywhere:

- PostgreSQL: its default `READ COMMITTED` isolation takes a new snapshot per statement, so the two
  can still disagree inside a plain transaction. Use `REPEATABLE READ` (or `SERIALIZABLE`):

    ```go
    err := dbx.WithTx(ctx, q, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true},
        func(tx dbx.Querier) error {
            users, total, err = repo.With(tx).QueryGridWithCount(ctx, grid, q)
            return err
        })
    ```

- SQLite: transactions are serializable, so any transaction is enough (`dbx.WithTx(ctx, q, nil, ...)`).
- ClickHouse: there are no transactions (`WithTx` fails with `dbx.ErrTxUnsupported`), so the total
  is always a separate read; treat it as approximate while data is being written.

`grid.Conds(q)` validates `q`
like `ValidQuery` and returns the `WHERE` conditions `Build` would add (one per filter, then the
search expression), without sort or paging — use it to count or aggregate over a grid's filters
yourself, e.g. `repo.Count(ctx, gohan.And(conds...))` (pass `nil` when `conds` is empty).

Only grid-flagged fields are addressable by alias — the struct's **Go field name** by default
(`"ID"`, `"Name"`, `"Email"` above), or an `alias`/`json`/`xml` tag when one is set (the `json`
tags above make the aliases `"id"`/`"name"`/`"email"`, matching what a JSON client would send);
every other field, tagged or not, answers "field is not valid". `NewGrid` fails outright — not
silently — on a duplicate, empty or `"-"` alias among grid-flagged fields, or a searchable field
that is neither string-kind nor a `database/sql/driver.Valuer`.

`AddFilterFunc`, `WithMaxLimit`, `WithTiebreaker` and `WithCaseInsensitiveSearch` mutate the `Grid` in place — unlike `gohan`'s builders and
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
`GridQuery.SearchType` is range-checked by `ValidQuery`/`NewGridQuery`, and search text on a type with
no searchable fields is rejected ("no searchable fields") rather than ignored. `Offset` and `Limit`
above `math.MaxInt64` are rejected as a `GridError`, and `GridQuery.Page` saturates instead of
overflowing.

**Row cap.** A new `Grid` caps `Limit` at `dbx.DefaultMaxLimit` (1000): `Limit == 0` or
`Limit > n` is treated as `Limit == n`. `WithMaxLimit(n)` changes the cap; `WithMaxLimit(0)`
removes it, so `Limit == 0` returns every row.

**Sort.** `Sort` (`[{"field": ..., "order": ...}]`) applies the fields in the order given, so the
client controls precedence; a repeated field is rejected. `SortFields` (a map) is the older form and
applies in alias order; a query may use one or the other, not both. The default order is
descending. `WithTiebreaker(cols...)` appends T's db columns (grid-flagged or not), ascending, after
the client's sort fields, skipping any already sorted; give it a unique key so rows with equal sort
values keep a stable order across `LIMIT`/`OFFSET` pages. A column T does not map makes `Build`
fail.

**Case-insensitive search.** Search uses `LIKE`, which is case-sensitive on PostgreSQL and
ClickHouse (SQLite's `LIKE` ignores ASCII case). `WithCaseInsensitiveSearch()` switches the grid to
gohan's `ContainsFold`/`HasPrefixFold`/`HasSuffixFold`: `ILIKE` on PostgreSQL and ClickHouse, `LIKE`
on SQLite (ASCII letters only there). Wildcards in the search text stay literal either way.

**Filter funcs.** A `GridFilterFunc`'s result is checked too. It may return any value the driver
binds as one value — a scalar, `time.Time`, an array, a byte slice (`[]byte`, `json.RawMessage`,
`net.IP`) or any `driver.Valuer` (`uuid.UUID`, `pq.StringArray`, ...) — or any other slice of such
values, which becomes an `IN` list (capped at `dbx.MaxFilterValues`). A `gohan` type (an expression
or a subquery, which gohan would render as SQL rather than bind) or a map is rejected as "value is
not valid", also inside a list.

`WithMaxLimit` and `WithTiebreaker` report configuration errors (a cap above `math.MaxInt64`, a
column T does not map) from both `ValidQuery` and `Build`, as plain errors rather than `GridError`s;
each call replaces the previous setting.

`GridQuery` fields decode from JSON as
`{"searchType": ..., "searchText": ..., "filterFields": ..., "sort": ..., "sortFields": ...,
"offset": ..., "limit": ...}` — `searchType`'s JSON name, not `search_type`; decoding is case-insensitive like
the rest of `encoding/json`. Because JSON numbers decode as `float64`, a filter value like
`{"id": 3.9}` matches `id = 3` on PostgreSQL (`pgx` truncates); register a `GridFilterFunc` via
`AddFilterFunc` on integer columns to reject non-integer input if that matters.

## Cursor pagination: ListKeyset and QueryGridKeyset

`LIMIT`/`OFFSET` paging re-reads and discards every skipped row, and shifts under concurrent inserts
and deletes (a row can be shown twice or never). Keyset (cursor) pagination instead continues from
the last row returned: each page ends with an opaque cursor holding that row's key values, and the
next page selects the rows that sort after them.

```go
type Event struct {
    ID        int64     `db:"id,auto" json:"id" grid:"sort,filter"`
    Kind      string    `db:"kind" json:"kind" grid:"sort,filter"`
    CreatedAt time.Time `db:"created_at" json:"createdAt" grid:"sort"`
}

keys := []dbx.KeysetKey{dbx.KeyDesc("created_at"), dbx.KeyAsc("id")}

// first page: empty cursor
page, err := repo.ListKeyset(ctx, gohan.Col("kind").Eq("login"), keys, 50, "")
// page.Items ([]*Event, never nil), page.HasMore, page.NextCursor ("" on the last page)

// next page: the cursor the client sent back
page, err = repo.ListKeyset(ctx, gohan.Col("kind").Eq("login"), keys, 50, page.NextCursor)
```

`where` is a filter expression, not a builder: keyset pagination owns `ORDER BY` and `LIMIT`, and
must not have an `OFFSET`. `nil` (or an empty `gohan.And()`) means no filter.

For a grid, `QueryGridKeyset` takes the cursor as a separate argument and pages over the same
filters, search and order as `QueryGrid` (the client's sort fields, then the tiebreaker). The grid
needs a tiebreaker that identifies rows uniquely:

```go
grid, err := dbx.NewGrid[Event]()
grid.WithTiebreaker("id")

// handler: q from the request body, cursor from a query parameter
page, err := repo.QueryGridKeyset(ctx, grid, q, r.URL.Query().Get("cursor"))
var gerr dbx.GridError
switch {
case errors.As(err, &gerr): // invalid query or cursor: 400, gerr is safe to return as JSON
case err != nil: // everything else, including dbx.ErrCursorTooLarge: 500
}
// respond with page (JSON: {"items": [...], "nextCursor": "...", "hasMore": true})
```

**Keys.** The key columns, in order, define the page order; together they must identify a row
uniquely, so end the list with a unique column (`id`). Two fetched rows with the same key values fail
the page with `dbx.ErrKeysetNotUnique`, rather than silently skipping or repeating rows. Key columns
must be `NOT NULL`. A key's Go field must be an integer (not `uintptr`), a `string`, a `time.Time` or
a `uuid.UUID` (named types count by their kind); pointers, `sql.Null*` and other
`driver.Valuer`/`sql.Scanner` types, floats, `bool`, `[]byte` and structs are rejected with
`dbx.ErrInvalidKeysetKey`, as are an empty key list, more than `dbx.MaxKeysetKeys` (8) keys and a
repeated column; an unknown column fails with `dbx.ErrUnknownColumn`. All of this is checked before
any query. Match the Go integer width to the column (`int32` for an `integer` column): the width is
part of the cursor and bounds the values it accepts. Uniqueness is checked with Go equality, so a
case-insensitive collation can make two rows equal to the database but not to `dbx` — keep string
keys on a case-sensitive collation, or follow them with a unique key.

On a grid, the keys are the client's sort fields (default direction descending) then the
`WithTiebreaker` columns (ascending). A grid without a tiebreaker, or whose tiebreaker has an
ineligible type, fails with `dbx.ErrInvalidKeysetKey`; a client sort on an ineligible field, or more
than 8 keys in all, fails with a `GridError` of scope `"sort"`.

**Cursor.** The cursor is base64url-encoded JSON carrying a format version, a fingerprint of the
table, key columns, directions and key types, and the last row's key values. It is **not signed or
encrypted**: clients can read the key values, so never use a secret column as a key. Every value is
re-validated when the cursor comes back and bound as an ordinary parameter. A cursor is at most
`dbx.MaxCursorBytes` (4096) bytes. A malformed or tampered cursor, or one minted for another table,
key list, direction or key type, fails with `dbx.ErrInvalidCursor` from `ListKeyset` and with
`GridError{Scope: "cursor"}` from `QueryGridKeyset` (`errors.Is(err, dbx.ErrInvalidCursor)` holds for
both) — a client error; neither echoes the cursor. A next-page cursor that would exceed the size
limit (very long string keys) fails with `dbx.ErrCursorTooLarge`, a server error. The fingerprint
does not cover filters, search or page size, so a client may change them and keep its position;
changing the sort invalidates the cursor. Renaming a key column or changing its Go type or width
invalidates outstanding cursors (clients restart from the first page); a database-only type change
does not.

**String keys must be short and valid UTF-8.** The next-page cursor carries the last row's key
values verbatim, so a string key's stored value decides whether its page can be served. A value
that would push the cursor past `dbx.MaxCursorBytes` (roughly 3000 bytes of JSON-escaped key text in
all, since base64 adds a third; control characters escape to 6 bytes each) fails the whole page with
`dbx.ErrCursorTooLarge`, and a value that is not valid UTF-8 (which JSON cannot carry) with
`dbx.ErrInvalidKeysetKey`. Both are server errors returned instead of a page, not a silently broken
cursor, and they recur for as long as that row ends a page. On a grid the client picks the sort, so
any `grid:"sort"` string field can become a key: bound the length of every string column used as a
key, sortable grid fields included, well below that limit (e.g. validated on write and
`CHECK (octet_length(name) <= 500)`), keep it valid UTF-8 (PostgreSQL `text` always is; SQLite and
ClickHouse `String` accept arbitrary bytes), or leave long free-text columns out of `grid:"sort"`.

**Paging rules.**

- `ListKeyset` treats a page size below 1 as `dbx.DefaultPageSize` and clamps one above
  `dbx.DefaultMaxLimit`; `QueryGridKeyset` caps `q.Limit` as `QueryGrid` does, and uses
  `DefaultPageSize` when the grid has no cap and `q.Limit` is 0.
- Each query fetches one extra row to learn whether another page follows, so `HasMore` is exact.
- `QueryGridKeyset` rejects a non-zero `q.Offset` with a `GridError`, even on the first page.
- Rows inserted before the cursor position are not seen, rows inserted after it are; a row updated
  so that its keys move across the cursor can be missed or seen twice. Under concurrent deletes the
  last page can be empty.
- There is no backwards paging and no total; use `QueryGridWithCount` for a total.

**Indexes.** A keyset page is an index range scan when an index covers the keys in order with
matching directions (or all reversed), e.g. `CREATE INDEX ON events (created_at DESC, id)`. The seek
condition repeats a bound on the first key so every dialect can use it.

**Dialects.**

- PostgreSQL: `timestamp` and `timestamptz` keys are exact to the microsecond.
- SQLite stores `time.Time` as text and compares it as text, so a time key only pages correctly if
  every value is stored in UTC (`t.UTC()`) with the same `_time_format` setting; see
  [Limitations](#limitations).
- ClickHouse: `DateTime` and `DateTime64` keys work at any scale; `Date`/`Date32` keys are not
  supported, and neither are time values after 2262 (clickhouse-go v2.40.3 mis-scans them). A cursor's
  time values must lie between 1900-01-01 and 2262-04-11T23:47:16.854775807Z: a forged cursor
  outside that range fails with `ErrInvalidCursor`, and a stored time outside it in the last row of a
  page fails that page with `ErrInvalidKeysetKey` (a server error), rather than handing out a cursor
  the next request would reject. `UUID` keys compare with the same ordering ClickHouse's
  `ORDER BY` uses.

## Typed projections

A result that is not a repository's record type — a join, an aggregate, a subset of columns —
scans into a DTO with the package-level `dbx.Query[D]`/`dbx.QueryOne[D]` (Go methods cannot take
type parameters, so these are functions over a `Querier`):

```go
type UserPosts struct {
    Name  string `db:"name"`
    Posts int64  `db:"posts"`
}

st := gohan.Select(gohan.Col("u.name"), gohan.Count(gohan.Col("p.id")).As("posts")).
    From(gohan.Table("users").As("u")).
    Join(gohan.Table("posts").As("p"), gohan.Col("p.user_id").Eq(gohan.Col("u.id"))).
    GroupBy(gohan.Col("u.name"))

rows, err := dbx.Query[UserPosts](ctx, q, st)                 // []*UserPosts, never nil
one, err := dbx.QueryOne[UserPosts](ctx, q, st.Limit(1))      // *UserPosts or dbx.ErrNotFound
```

Both build `st` against `q.Dialect()`, so the same statement works on any `Querier` (pass
`tx` inside `WithTx`). `QueryOne` scans the first row and does not add a `LIMIT` itself. `D`'s fields
map to result columns by their `db` tag through `sqlx`; on ClickHouse, `clickhouse.Querier` scans
through clickhouse-go, so `D` must be a struct with matching `ch` tags (see
[Record types need matching `ch` tags](#record-types-need-matching-ch-tags)); both functions run that
check on `D` before the query and return its error.

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
(falling back to the `ch` tag, then the lower-cased Go field name). But `clickhouse.Querier` scans
and appends rows through clickhouse-go's own `ScanStruct`/`AppendStruct`, which map columns to
fields by the **`ch`** struct tag (falling back to the Go field name as written) — a different
tag, read by different code. Each column a record maps must therefore reach the same field under
both rules: set `ch` tags equal to the `db` tags, or only `ch` tags.

```go
type Event struct {
    ID   uint32   `ch:"id" db:"id,auto"`
    Name string   `ch:"name" db:"name"`
    Tags []string `ch:"tags" db:"tags"`
}
```

`clickhouse.Querier` implements `dbx.RecordChecker`, so `dbx.NewRepository` checks this at setup
and fails with `clickhouse.ErrRecordMapping`, naming every offending column, its field and the
reason, when clickhouse-go would map a column to a different field or to none:

- a field without a `ch` tag whose Go name differs from its column (``Name string `db:"name"` ``
  maps to `"Name"` in clickhouse-go);
- a `ch` tag naming a different column, including a `ch` tag with options (`ch:"name,omitempty"`
  is the column `name,omitempty` to clickhouse-go);
- `ch:"-"` on a field `dbx` maps;
- two fields clickhouse-go maps to the same name (the one declared later, including inside an
  embedded struct, wins);
- an embedded `time.Time` (one column for `dbx`, but clickhouse-go descends into its fields);
- an embedded non-struct type anywhere in the record, even one `dbx` skips with `db:"-"`
  (clickhouse-go's mapper panics on it; `ch:"-"` hides it from both).

A field clickhouse-go maps but `dbx` doesn't (e.g. `db:"-"` without a `ch` tag) is allowed:
`dbx` only selects and inserts its own column list, so the driver never looks such a field up.
The result is cached per record type. The check covers `dbx.NewRepository` and the typed projections
`dbx.Query[D]`/`dbx.QueryOne[D]`; calling `Get`/`Select` directly with your own SQL and record types is
unchecked (call `q.CheckRecord(reflect.TypeFor[T]())` yourself if needed). `Repository.With(q)` does
not re-run the check either (it cannot return an error): rebinding a repository built over another
`Querier` (e.g. PostgreSQL) to a ClickHouse `Querier` skips it, so build ClickHouse repositories with
`dbx.NewRepository` over the ClickHouse `Querier` itself.

!!! note "Maintenance"
    `CheckRecord` mirrors clickhouse-go **v2.40.3**'s struct mapper (`struct_map.go`, copied as
    `structIdx` in `provider/clickhouse/mapper.go`). On every clickhouse-go upgrade, diff the
    driver's `struct_map.go` against that copy and re-verify `CheckRecord`'s rules and tests.

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
- Statements are built with `gohan.ClickHouseNamed()` (`@pN` named placeholders), not
  `gohan.ClickHouse()`: this is what lets a bound `time.Time` keep full sub-second precision.
  clickhouse-go's native positional binding only ever sends whole seconds; named binding lets
  `Querier` convert each argument to `clickhouse.DateNamed`, which the driver renders as a
  `toDateTime64(...)` literal at the smallest scale that holds the value exactly, but never below
  milliseconds (3, 6 or 9): a scale-9 literal overflows after 2262, so a time with nanosecond
  precision can only be bound up to 2262, while millisecond or microsecond times bind across
  `DateTime64(3)`'s 1900-2299. A time in an IANA zone (e.g. `Europe/Lisbon`) keeps its zone, so a
  `Date` column or `toDate` sees the calendar day in that zone. A `time.Local` time (what
  `time.Now()` returns) or a `time.FixedZone` time is bound as the same instant in UTC: clickhouse-go
  renders `Local` as a numeric string ClickHouse misreads, and ClickHouse cannot load a fixed zone
  by name. Comparisons are then **exact-instant**: a
  `DateTime64(3)` column storing `.123` does not equal a bound value of `.123456789` (only a value
  truncated to milliseconds). This conversion applies to `sql.NamedArg` values gohan builds
  (`time.Time`, `*time.Time`, or a `driver.Valuer` producing one, such as `sql.NullTime`); SQL you
  write yourself with positional `?` placeholders still gets clickhouse-go's whole-second binding.
  clickhouse-go v2.40.3 itself mishandles `DateTime64` values after 2262 in columnar inserts
  (`Insert`/`InsertBatch`) and when scanning into `time.Time`; use bound statements (`Exec`) to
  write such values, and read them with `toString` or compare them in SQL.
- `Querier` is not transactional (no `TxBeginner`/`TxQuerier`): `dbx.WithTx` fails with
  `dbx.ErrTxUnsupported` over it, matching ClickHouse having no transactions.

## Dialect support

| Capability | PostgreSQL (`FromClient`) | SQLite (`FromClient`) | ClickHouse (`Client.Querier()`) |
|---|---|---|---|
| `Get`/`List`/`GetBy`/`ListBy`/`Count`/`Exists` | yes | yes | yes |
| `Query[D]`/`QueryOne[D]` | yes | yes | yes (`D` must be a struct with matching `ch` tags) |
| `Insert` (chunked, transactional) | yes | yes | uses `InsertBatch` instead |
| `WithGroupedInserts` | yes | yes | no effect: `InsertBatch` still rejects mixed records (`gohan.ErrInconsistentOmit`) |
| `Update`/`UpdateFields`/`Delete` | yes | yes | `Delete` only (no `UPDATE`) |
| `UpdateReturning` | yes | yes (SQLite 3.35+) | no (`gohan.ErrUnsupported`) |
| `InsertReturning`/`UpsertReturning` | yes | yes (SQLite 3.35+) | no (`gohan.ErrUnsupported`) |
| `Upsert`/`InsertIgnore` | yes | yes | no (`gohan.ErrUnsupported`) |
| `Changeset`/`SetOptional`/`Changes` | yes | yes | build the map, but there is no `UPDATE` to apply it |
| `WithTx` | yes | yes | no (`dbx.ErrTxUnsupported`) |
| `WithTxRetry` | retries SQLSTATE `40001`/`40P01` | retries `SQLITE_BUSY`/`SQLITE_LOCKED` | no retry; fails like `WithTx` (`dbx.ErrTxUnsupported`) |
| `pgsql.WithTxSession` | yes | no (`pgsql.ErrNotPostgres`) | no (`pgsql.ErrNotPostgres`) |
| Rows-affected reporting | real count | real count | always `0` |
| `Grid[T]`, `QueryGrid`, `QueryGridWithCount` | yes | yes | yes |
| `ListKeyset`/`QueryGridKeyset` | yes | yes (time keys must be stored in UTC) | yes (time keys 1900 to 2262-04-11) |
| `Grid.WithCaseInsensitiveSearch` | `ILIKE` | `LIKE` (ASCII letters only) | `ILIKE` |

## Limitations

- `dbx.FromClient` assumes `sqlx`'s default field-name mapper; a repository bound to a `*sqlx.DB`
  with a custom mapper is not supported.
- `dbx.FromClient` snapshots the client's connection: call it once at startup, not after a later
  `Disconnect`/`Connect` cycle.
- ClickHouse has no `UPDATE`, `RETURNING`, `ON CONFLICT` upsert, or transactions — see the
  ClickHouse section above and [gohan's dialect support table](https://github.com/oddbit-project/gohan#dialect-support).
- `WithTx`'s `fn` must run every statement through `repo.With(tx)`, not the outer repository —
  nothing in the type system enforces this (see the Transactions section above).
- On SQLite, a `time.Time` keyset key pages correctly only if every value is stored in UTC and the
  `_time_format` DSN setting never changes between writes: the driver writes times as text in the
  value's own zone (plus a monotonic clock suffix for `time.Now()` values not passed through `UTC()`)
  and SQLite compares that text, not the instant.
- `dbx` and `gohan` each define a similarly-named error for a different failure, and both can
  surface from the same call — check the specific sentinel, not just the name: `dbx.ErrNoColumns`
  is returned by `NewRepository` when the record type maps zero columns, while
  `gohan.ErrNoColumns` means a statement (`Insert`/`Update`) ended up with nothing to write;
  `dbx.ErrUnknownColumn` is returned when a caller-supplied column name (a `GetBy`/`ListBy`/
  `UpdateFields`/`UpdateReturning` map key, an `Upsert`/`UpsertReturning`/`InsertIgnore`
  conflict or update column, or a `Changeset.Set` column) isn't one of the repository's known
  columns, while `gohan.ErrUnknownField` is
  `IncludeFields`/`ExcludeFields`/`DoUpdateExcluded` naming a field or column `gohan` doesn't
  recognize on the record/statement in question.
