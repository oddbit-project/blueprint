# Migrating to dbx

`db.Repository` builds its SQL through goqu, and goqu's default rendering inlines values and
identifiers into the SQL string rather than binding them — the class of bug fixed in the first
release after Blueprint v0.10.3. `gohan` and `dbx` exist so new code stops doing that: every value is always bound and
every identifier is always quoted and escaped by construction. This guide maps every common
`db`/goqu call to its `gohan`/`dbx` replacement, and lists the behaviour that changed on purpose
along the way.

New code should use `dbx`/`gohan` directly (see [gohan Query Builder](gohan.md) and
[dbx Repositories](dbx.md)). Existing `db.Repository` code is not being force-migrated by this
guide — see "Migrating incrementally" below.

## Mapping table

| `db` / goqu | `dbx` / `gohan` |
|---|---|
| `db.NewRepository(ctx, client, table)` | `dbx.NewRepository[T](q, table)` with `q, _ := dbx.FromClient(client)` |
| `repo.FetchOne(repo.SqlSelect().Where(goqu.C("id").Eq(id)), &rec)` | `rec, err := repo.Get(ctx, repo.Select().Where(gohan.Col("id").Eq(id)))` |
| `repo.Fetch(qry, &list)` | `list, err := repo.List(ctx, q)` |
| `repo.FetchWhere(db.FV{...}, &list)` / `FetchRecord` / `FetchByKey` | `repo.ListBy(ctx, map[string]any{...})` / `repo.GetBy(...)` |
| `repo.Exists("f", v)` | `repo.Exists(ctx, gohan.Col("f").Eq(v))` |
| `repo.CountWhere(db.FV{...})` | `repo.Count(ctx, gohan.Match(map[string]any{...}))` |
| `repo.Insert(rec)` / `InsertReturning` | `repo.Insert(ctx, recs...)` / `repo.InsertReturning(ctx, rec)` |
| `repo.UpdateRecord` / `UpdateFields` / `SqlUpdateX(rec)...` | `repo.Update(ctx, rec, where, opts...)` / `repo.UpdateFields(ctx, fields, where)` |
| `repo.DeleteWhere(db.FV{...})` | `repo.Delete(ctx, gohan.Match(map[string]any{...}))` |
| `goqu.C("a")`, `goqu.I("t.a")` | `gohan.Col("a")`, `gohan.Col("t.a")` |
| `goqu.L("x @> ?", v)` | `gohan.Raw("x @> ?", v)` (literal `?` → `??`) |
| `goqu.Ex{"a": 1}` | `gohan.Match(map[string]any{"a": 1})` |
| `db.EmptyResult(err)` | `errors.Is(err, dbx.ErrNotFound)` |
| `repo.NewTransaction(nil)` … `Commit()` | `dbx.WithTx(ctx, q, nil, func(tx dbx.Querier) error { ... repo.With(tx) ... })` |
| `db.NewGrid(table, &Rec{})` + `repo.QueryGrid(...)` | `dbx.NewGrid[Rec]()` + `repo.QueryGrid(ctx, g, q)` |
| ClickHouse `clickhouse.NewRepository(ctx, conn, table)` | `dbx.NewRepository[T](client.Querier(), table)` |

## Example: a repository using FetchWhere and DeleteWhere

Before (`db.Repository`):

```go
repo := db.NewRepository(ctx, client, "users")

var users []*User
if err := repo.FetchWhere(db.FV{"active": true}, &users); err != nil {
    log.Fatal(err)
}

if err := repo.DeleteWhere(db.FV{"id": staleID}); err != nil {
    log.Fatal(err)
}
```

After (`dbx`):

```go
q, _ := dbx.FromClient(client)
repo, _ := dbx.NewRepository[User](q, "users")

users, err := repo.ListBy(ctx, map[string]any{"active": true})
if err != nil {
    log.Fatal(err)
}

if _, err := repo.Delete(ctx, gohan.Col("id").Eq(staleID)); err != nil {
    log.Fatal(err)
}
```

## Behaviour changes

These are intentional differences, not bugs — code relying on the old behaviour needs to change,
not just its call syntax:

- **Unfiltered DELETE/UPDATE refused.** Up to v0.10.3, `db.DeleteWhere(db.FV{})` with an empty,
  non-nil map (or a goqu dataset with no WHERE) would delete every row — the check was
  `fieldNameValue == nil`, which an empty non-nil map passes; the first release after Blueprint
  v0.10.3 tightened it to `len(fieldNameValue) == 0`. `dbx.Repository.Delete`/`Update`/
  `UpdateFields` require a non-nil `where` and fail with `gohan.ErrNoWhere` instead — before
  touching the database. A trivially-true `where` (`gohan.And()`, an empty `NotIn`) is rejected
  the same way. A caller who means "every row" now says so explicitly:
  `repo.Exec(ctx, gohan.Delete(table).All())`.
- **Grid search wildcards are literal.** `db.Grid`'s search text was passed to `LIKE` without
  escaping `%`/`_`; a user typing `%` got wildcard behaviour they didn't ask for. `dbx.Grid`'s
  `Contains`/`HasPrefix`/`HasSuffix` escape `%` and `_` in the search text, so a literal `%` in
  input matches a literal `%`.
- **Deterministic grid filter/sort order.** `dbx.Grid.Build` applies filters and sorts in sorted
  alias order, not map iteration order, so the same `GridQuery` always builds the same SQL.
- **A known-but-unflagged field now answers "field is not valid", not "not filterable"/"not
  sortable".** `db.Grid` also enforced grid flags — an unflagged field was always rejected — but
  it distinguished a struct field that exists without the right flag ("field is not
  filterable"/"not sortable") from an alias that isn't a struct field at all ("field is not
  valid"). `dbx.Grid[T]` collapses that distinction: only grid-flagged fields are addressable at
  all, so *any* unaddressable alias — unflagged struct field or bogus name alike — answers
  "field is not valid". This is a visible response change for a client that branches on the
  `GridError.Message` text.
- **`GridQuery.SearchType` is JSON key `searchType`**, not the old `SearchType` (the `db` package's
  `GridQuery.SearchType` carried no `json` tag, so it serialized under the Go field name,
  `SearchType`, capitalized). Decoding is case-insensitive either way (`encoding/json`'s usual
  behaviour), so an old client sending `"SearchType"` still decodes correctly against the new
  struct; only a client that reads the field back out of JSON by exact key needs to change.
- **`ctx` on every call**, not a context stored at construction — a repository is no longer tied
  to one context/deadline for its whole lifetime.
- **`Int(n)` for CASE constants.** A goqu integer literal bound as a value inside a `CASE` fed to
  `SUM` on PostgreSQL is sent as `text` and fails with `function sum(text) does not exist`; use
  `gohan.Int(n)` for an inline integer literal instead of a bound value.
- **ClickHouse rows-affected is always 0.** Code checking `n, err := repo.Delete(...)` for the
  rows a ClickHouse-backed repository affected always sees `n == 0` — ClickHouse does not report
  it. Rewrite `if n == 0 { ... }` logic that assumed a real count.
- **`dbx` selects an explicit column list, never `SELECT *`.** A record type must map every
  column it selects, or the query fails with "missing destination name" — the same failure the
  old `SELECT *` had (`db.Fetch` calls sqlx's `SelectContext` without `Unsafe()`, so it already
  failed on an unmapped column, not tolerated it). What changed is the other direction: `dbx`'s
  explicit column list tolerates a table column the struct doesn't map (it's just never
  selected), where the old `SELECT *` would have included, and then failed to scan, it too.
- **Record shapes that used to bind silently wrong data are now rejected outright.** An embedded
  pointer struct, an unexported or `db`/`ch`-tagged embedded struct, an ambiguous promoted field
  name, or the same column set more than once all fail at `NewRepository`/`Build` time
  (`gohan.ErrRecordShape`, `gohan.ErrDuplicateColumn`) instead of building a statement. The old `qb`
  path silently bound the wrong value for some of these shapes — most notably a duplicate
  promoted field name, where one of the two fields silently won and the other was never written.
- **Grid input limits.** `dbx.Grid` caps a `[]any` filter value at `dbx.MaxFilterValues` elements
  and `SearchText` at `dbx.MaxSearchText` bytes, range-checks `SearchType` in `ValidQuery`, and
  requires `Grid.Build` to start from a non-nil base query. `Limit == 0` still returns every row
  unless `Grid.WithMaxLimit` is set, in which case `Limit == 0` (or a `Limit` over the cap) is
  treated as the cap. Because `GridQuery` decodes from JSON, a numeric filter value always
  arrives as `float64` — e.g. `{"id": 3.9}` matches `id = 3` on PostgreSQL (`pgx` truncates);
  register a `GridFilterFunc` via `AddFilterFunc` on an integer column to reject non-integer
  input if that matters.
- **SQLite identifiers are quoted with backticks, not double quotes.** SQLite silently treats an
  unrecognized double-quoted identifier as a string literal rather than raising an error; `gohan`
  avoids that failure mode entirely by never using double quotes on SQLite. SQLite's `LIKE` is
  also ASCII case-insensitive (PostgreSQL, ClickHouse and Generic are case-sensitive; use
  `ILIKE` there).
- **ClickHouse-specific restrictions**: a literal `?` is rejected in identifiers and in `Raw`'s
  `??` form; map, struct and named (driver-bound) values are rejected outright
  (`gohan.ErrUnsafeValue`) rather than sent to a driver that can't bind them safely; a `UNION`
  with an outer `ORDER BY`/`LIMIT`/`OFFSET` is wrapped as `SELECT * FROM (<union>) ...`;
  `Delete(t).All()` renders `... WHERE 1`; lightweight `DELETE` does not work on Distributed
  tables or tables with projections; and a bound `time.Time` reaches the server at second
  precision (a `clickhouse-go` v2.40.3 limitation, not a `dbx`/`gohan` choice).
- **PostgreSQL: a bound value alone in a select list may need `gohan.Cast(...)`.** PostgreSQL
  cannot always infer a parameter's type from context alone and fails with "could not determine
  data type of parameter" — wrap the value in `gohan.Cast(v, "type")` when that happens.

## Migrating incrementally

`db` and `dbx` share the same `*db.SqlClient` (`dbx.FromClient(client)` takes one directly), so a
single application can run both side by side while repositories move one at a time — there is no
"big bang" cutover required. Move one repository (and its callers) to `dbx.Repository[T]` at a
time, verify it, then move the next.
