# Migrating to dbx

`db.Repository` builds its SQL through goqu, and goqu's default rendering inlines values and
identifiers into the SQL string rather than binding them — the class of bug fixed in commit
`654df80`. `sqlb` and `dbx` exist so new code stops doing that: every value is always bound and
every identifier is always quoted and escaped by construction. This guide maps every common
`db`/goqu call to its `sqlb`/`dbx` replacement, and lists the behaviour that changed on purpose
along the way.

New code should use `dbx`/`sqlb` directly (see [sqlb Query Builder](sqlb.md) and
[dbx Repositories](dbx.md)). Existing `db.Repository` code is not being force-migrated by this
guide — see "Migrating incrementally" below.

## Mapping table

| `db` / goqu | `dbx` / `sqlb` |
|---|---|
| `db.NewRepository(ctx, client, table)` | `dbx.NewRepository[T](q, table)` with `q, _ := dbx.FromClient(client)` |
| `repo.FetchOne(repo.SqlSelect().Where(goqu.C("id").Eq(id)), &rec)` | `rec, err := repo.Get(ctx, repo.Select().Where(sqlb.Col("id").Eq(id)))` |
| `repo.Fetch(qry, &list)` | `list, err := repo.List(ctx, q)` |
| `repo.FetchWhere(db.FV{...}, &list)` / `FetchRecord` / `FetchByKey` | `repo.ListBy(ctx, map[string]any{...})` / `repo.GetBy(...)` |
| `repo.Exists("f", v)` | `repo.Exists(ctx, sqlb.Col("f").Eq(v))` |
| `repo.CountWhere(db.FV{...})` | `repo.Count(ctx, sqlb.Match(map[string]any{...}))` |
| `repo.Insert(rec)` / `InsertReturning` | `repo.Insert(ctx, recs...)` / `repo.InsertReturning(ctx, rec)` |
| `repo.UpdateRecord` / `UpdateFields` / `SqlUpdateX(rec)...` | `repo.Update(ctx, rec, where, opts...)` / `repo.UpdateFields(ctx, fields, where)` |
| `repo.DeleteWhere(db.FV{...})` | `repo.Delete(ctx, sqlb.Match(map[string]any{...}))` |
| `goqu.C("a")`, `goqu.I("t.a")` | `sqlb.Col("a")`, `sqlb.Col("t.a")` |
| `goqu.L("x @> ?", v)` | `sqlb.Raw("x @> ?", v)` (literal `?` → `??`) |
| `goqu.Ex{"a": 1}` | `sqlb.Match(map[string]any{"a": 1})` |
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

if _, err := repo.Delete(ctx, sqlb.Col("id").Eq(staleID)); err != nil {
    log.Fatal(err)
}
```

## Behaviour changes

These are intentional differences, not bugs — code relying on the old behaviour needs to change,
not just its call syntax:

- **Unfiltered DELETE/UPDATE refused.** `db.DeleteWhere(db.FV{})` with an empty map (or a
  goqu dataset with no WHERE) would delete every row. `dbx.Repository.Delete`/`Update`/
  `UpdateFields` require a non-nil `where` and fail with `sqlb.ErrNoWhere` instead — before
  touching the database. A trivially-true `where` (`sqlb.And()`, an empty `NotIn`) is rejected
  the same way. A caller who means "every row" now says so explicitly:
  `repo.Exec(ctx, sqlb.Delete(table).All())`.
- **Grid search wildcards are literal.** `db.Grid`'s search text was passed to `LIKE` without
  escaping `%`/`_`; a user typing `%` got wildcard behaviour they didn't ask for. `dbx.Grid`'s
  `Contains`/`HasPrefix`/`HasSuffix` escape `%` and `_` in the search text, so a literal `%` in
  input matches a literal `%`.
- **Deterministic grid filter/sort order.** `dbx.Grid.Build` applies filters and sorts in sorted
  alias order, not map iteration order, so the same `GridQuery` always builds the same SQL.
- **Only grid-flagged fields are addressable.** `db.Grid` could be coaxed into filtering/sorting
  on any struct field; `dbx.Grid[T]` only recognizes fields tagged `grid:"sort"`/`"filter"`/
  `"search"` — anything else answers "field is not valid", not a database error.
- **`GridQuery.SearchType` is JSON key `searchType`**, not `search_type` — decoding itself is
  case-insensitive (`encoding/json`'s usual behaviour), so this only matters for a client that
  serializes the field name explicitly.
- **`ctx` on every call**, not a context stored at construction — a repository is no longer tied
  to one context/deadline for its whole lifetime.
- **`Int(n)` for CASE constants.** A goqu integer literal bound as a value inside a `CASE` fed to
  `SUM` on PostgreSQL is sent as `text` and fails with `function sum(text) does not exist`; use
  `sqlb.Int(n)` for an inline integer literal instead of a bound value.
- **ClickHouse rows-affected is always 0.** Code checking `n, err := repo.Delete(...)` for the
  rows a ClickHouse-backed repository affected always sees `n == 0` — ClickHouse does not report
  it. Rewrite `if n == 0 { ... }` logic that assumed a real count.
- **`dbx` selects an explicit column list, never `SELECT *`.** A record type must map every
  column it selects; the old `SELECT *` tolerated a table column the struct didn't map.

## Migrating incrementally

`db` and `dbx` share the same `*db.SqlClient` (`dbx.FromClient(client)` takes one directly), so a
single application can run both side by side while repositories move one at a time — there is no
"big bang" cutover required. Move one repository (and its callers) to `dbx.Repository[T]` at a
time, verify it, then move the next.
