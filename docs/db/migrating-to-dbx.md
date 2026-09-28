# Migrating to dbx

This guide moves an application from the legacy `db` package (`db.Repository`, `db.Grid`, goqu,
`db/qb`, `db/field`) and the legacy `provider/clickhouse` repository to
[`dbx`](dbx.md) repositories built on [`gohan`](gohan.md). It gives a per-repository checklist, a
mapping for every `db.Repository` method, the `db.Grid` → `dbx.Grid` differences, a goqu → gohan
expression table, the behaviour that changed on purpose, and a complete before/after example.

Existing `db.Repository` code keeps working; nothing forces the move. `db` and `dbx` run side by
side on the same client, so repositories can move one at a time (see
[Migrating incrementally](#migrating-incrementally)).

## Why migrate

- **Values are always bound.** Since v0.11.0, `db` renders goqu datasets with bound values, but
  a `goqu.L` fragment built with `fmt.Sprintf` is still inlined, and the legacy
  `provider/clickhouse` repository still renders every query with goqu's `ToSQL()`, which inlines
  values as escaped literals. `gohan` binds every value; only `Raw`'s SQL text, `Fn`'s function
  name and `Cast`'s type string are trusted input.
- **Identifiers are always quoted and escaped.** `db` only rejects column names containing `"`,
  `\` or NUL (`db.ErrInvalidIdentifier`); `gohan` quotes and escapes every table and column name
  for the dialect.
- `dbx` also returns typed results (`*T`, `[]*T`), takes a `context.Context` on every call, refuses
  unfiltered `UPDATE`/`DELETE`, validates grid input, and writes multi-statement inserts
  atomically.

## Checklist: migrating one repository

1. **Create the Querier and repository.** For PostgreSQL and SQLite, wrap the `*db.SqlClient`
   the `db.Repository` already uses. For ClickHouse, use the client's `Querier()`:

    ```go
    q, err := dbx.FromClient(client) // the same *db.SqlClient db.NewRepository used
    if err != nil {
        return err
    }
    users, err := dbx.NewRepository[User](q, "users")
    if err != nil {
        return err
    }

    // ClickHouse: provider/clickhouse's *Client
    events, err := dbx.NewRepository[Event](chClient.Querier(), "events")
    if err != nil {
        return err
    }
    ```

    `FromClient` resolves the dialect from the client's driver name (`pgx`, `pgx/v5`, `postgres`,
    `sqlite`, `sqlite3`); any other name fails with `gohan.ErrUnknownDialect` unless registered with
    `gohan.Register`, and the `clickhouse` driver fails with `dbx.ErrDialectDriver`. It connects the
    client if needed and keeps its `*sqlx.DB`, so call it once at startup. Build one `Querier` and
    share it between repositories: `dbx.WithTx` needs it. `NewRepository` checks the record type
    up front (see step 2), so run it where a failure stops startup.

2. **Check the record struct tags.** `dbx` reads struct metadata through `gohan/field`, whose
   rules are the same as `db/field`'s except where noted below:

    | Tag | How `dbx`/`gohan` reads it |
    |---|---|
    | column name | the `db` tag's name, else the `ch` tag's name, else the lower-cased Go field name. `db:"-"` (or `ch:"-"` on a field without a `db` tag) skips the field. |
    | auto | `db:"name,auto"`, `auto:"true"`, `grid:"auto"`, `goqu:"skipinsert"` and `goqu:"skipupdate"` all mean the same thing: the column is never inserted and never updated (unless `gohan.WithAutoFields()`), `Changeset.Set` rejects it and `dbx.Changes` ignores it. `goqu:"skipupdate"` therefore also drops the column from `INSERT`, as it already did in `db.Repository`'s own insert and update builders (only goqu's dataset builders, `SqlInsert()`/`SqlUpdate()`, treated the two separately). |
    | omit | only `goqu:"omitnil"` (skip a nil pointer) and `goqu:"omitempty"` (skip a zero value). They apply to `Insert` and to `Update`'s `SetRecord`. `db:"x,omitnil"` sets nothing, as before. |
    | other `goqu:` options | ignored (e.g. `defaultifempty`). |
    | grid | `grid:"sort,filter,search"`; the grid alias is the `alias` tag, else `json`, else `xml`, else the Go field name. |
    | `mapper` | not read by `db`, `dbx` or `gohan`. For a JSON column use [`jsoncol.JSON[T]`](../types/types.md#jsoncol). |

    Fix these while you are there:

    - **No whitespace in tags.** `db/field` trims tag parts, so `db:"id, auto"` and
      `grid:"sort, filter"` work in `db`. `gohan/field` does not: `" auto"` and `" filter"` are
      silently ignored, so the column is inserted and the field is not filterable. Write
      `db:"id,auto"` and `grid:"sort,filter"`.
    - **Record shapes.** `NewRepository` fails with `gohan.ErrRecordShape` for an embedded pointer
      struct, an unexported or `db`/`ch`-tagged embedded struct, or an ambiguous promoted field,
      and with `gohan.ErrDuplicateColumn` when two fields map to one column.
    - **Reserved types.** A type registered with `db/field.AddReservedType` must also be
      registered with `gohan/field.AddReservedType`; the two registries are separate (see
      [Structs and Tags](structs-and-tags.md#reserved-types)).
    - **ClickHouse: `ch` must match `db`.** The legacy ClickHouse repository scanned and inserted
      through clickhouse-go, which reads only the `ch` tag (falling back to the Go field name as
      written), while its grid used the `db` tag. `dbx` builds column lists from the `db` tag, so
      every mapped field must reach the same column under both rules: give each field a `ch` tag
      equal to its `db` tag, or use only `ch` tags. `NewRepository` over `client.Querier()` checks
      this (`dbx.RecordChecker`) and fails with `clickhouse.ErrRecordMapping`, naming each
      offending column. A record like `db:"id" ch:"event_id"` is rejected, and so is a field
      with only `db:"name"` (clickhouse-go maps it as `Name`). See
      [Record types need matching `ch` tags](dbx.md#record-types-need-matching-ch-tags).

    ```go
    // Before: accepted by db.Repository
    type Account struct {
        ID      int64   `db:"id, auto"`                    // " auto" is ignored by gohan
        Email   string  `db:"email" grid:"search, filter"` // " filter" is ignored by gohan
        Bio     *string `db:"bio,omitnil"`                 // not an omit option anywhere
        Profile string  `db:"profile" mapper:"json"`       // mapper is never read
    }

    // After
    type Account struct {
        ID      int64                 `db:"id,auto"`
        Email   string                `db:"email" grid:"search,filter"`
        Bio     *string               `db:"bio" goqu:"omitnil"`
        Profile jsoncol.JSON[Profile] `db:"profile"`
    }
    ```

3. **Replace the calls** using the [method mapping](#dbrepository-method-mapping). Pass `ctx` to
   every call; reads return `*T`/`[]*T` instead of filling a target.

4. **Move transactions to `dbx.WithTx`.** Wrap the work in `dbx.WithTx(ctx, q, opts, fn)` and run
   every statement inside `fn` through `repo.With(tx)`. Any repository can join the transaction,
   not just the table it was begun from. Serializable retry loops become `dbx.WithTxRetry`, and
   `set_config`/`SET LOCAL ROLE` preambles become `pgsql.WithTxSession` (see
   [Transactions](#transactions)).

5. **Replace goqu expressions** with gohan ones using the [expression table](#goqu-gohan-expressions).
   Start custom `SELECT`s for `Get`/`List` from `repo.Select()`, not `gohan.From(table)`, which
   renders `SELECT *`. Use `dbx.Query[D]`/`QueryOne[D]` for joins, aggregates and other shapes that
   are not `T`.

6. **Update error handling.**
    - `db.EmptyResult(err)` → `errors.Is(err, dbx.ErrNotFound)` (`dbx.ErrNotFound` is
      `sql.ErrNoRows`). `Get`, `GetBy`, `QueryOne` and `UpsertReturning` return it; `List`,
      `ListBy`, `Query`, `QueryGrid` and `UpdateReturning` return an empty, non-nil slice instead.
    - `err.(db.GridError)` → `var gridErr dbx.GridError; errors.As(err, &gridErr)`. It is a
      different type, so an assertion against `db.GridError` never matches.
    - New errors to handle: `gohan.ErrNoWhere` (nil or trivially-true `where` on
      `Update`/`UpdateFields`/`UpdateReturning`/`Delete`), `gohan.ErrEmptyMatch` (an empty map in
      `GetBy`/`ListBy`/`gohan.Match`), `dbx.ErrUnknownColumn` (a map key or column name that is
      not one of `T`'s columns), `gohan.ErrUnsupported` (a feature the dialect lacks, e.g.
      `RETURNING` on ClickHouse), `dbx.ErrTxUnsupported` (a transaction over a Querier that cannot
      begin one) and `gohan.ErrInconsistentOmit` (see `WithGroupedInserts`).
    - `db.ErrInvalidParameters`, `db.ErrInvalidIdentifier` and the `db/qb` validation errors are
      not returned by `dbx`.

7. **Update grid handlers.** Decode `dbx.GridQuery` instead of `db.GridQuery`, build the grid
   once with `dbx.NewGrid[T]()`, and read [the grid differences](#dbgrid-dbxgrid) before
   shipping: the default 1000-row cap and the stricter validation change responses.

8. **Run the tests.** `go test -race ./...` (Docker is needed for the testcontainers suites).
   Cover each migrated repository at least with a test that calls its constructor, so
   `NewRepository` shape errors and ClickHouse `CheckRecord` errors fail the tests, not startup.

## db.Repository method mapping

`repo` is a `*dbx.Repository[T]`, `q` the `Querier` from `dbx.FromClient` (a `*dbx.SQLQuerier`),
`where` a `gohan.Expr`. Raw SQL run through `q` uses the dialect's own placeholders (`$1` on
PostgreSQL, `?` on SQLite), as the `db` raw methods did.

### Construction and identity

| `db` | `dbx` / `gohan` |
|---|---|
| `db.NewRepository(ctx, client, table)` | `q, err := dbx.FromClient(client)` once, then `dbx.NewRepository[T](q, table)` |
| `clickhouse.NewRepository(ctx, conn, table)` / `chClient.NewRepository(ctx, table)` | `dbx.NewRepository[T](chClient.Querier(), table)` |
| `repo.Name()` | `repo.Table()` |
| `repo.Db()` (`*sqlx.DB`) | no equivalent; keep the `*db.SqlClient` and use `client.Db()` |
| `repo.Sql()`, `repo.SqlDialect()`, `repo.SqlBuilder()` | `q.Dialect()` (a `gohan.Dialect`); gohan builders render with `st.Build(q.Dialect())` |
| `repo.SqlSelect()` | `repo.Select()` (`SELECT <T's columns> FROM <table>`) |
| `repo.SqlInsert()` / `SqlUpdate()` / `SqlDelete()` | `gohan.Insert(repo.Table())` / `gohan.Update(repo.Table())` / `gohan.Delete(repo.Table())`, run with `repo.Exec(ctx, st)` |
| `repo.SqlUpdateX(rec)` (`db/qb` builder) | `repo.Update(ctx, rec, where, opts...)`; `qb` options map to `gohan.IncludeFields`, `gohan.ExcludeFields`, `gohan.SkipZeroValues()` (`IncludeZeroValues(false)`) and `gohan.WithAutoFields()` (`UpdateAutoFields(true)`) |
| `repo.Do(ds, target...)` | by statement kind: `repo.Get`/`repo.List`/`dbx.Query[D]` for a `SELECT`, `repo.Exec(ctx, st)` for a write |
| `db.RegisterFactory`, `db.RegisterDialect` | no equivalent; a new driver name is mapped with `gohan.Register(driverName, dialect)` |

### Reads and counts

| `db` | `dbx` / `gohan` |
|---|---|
| `repo.FetchOne(ds, &rec)` | `rec, err := repo.Get(ctx, repo.Select().Where(...))` (adds `LIMIT 1`, like `FetchOne`) |
| `repo.Fetch(ds, &list)` | `list, err := repo.List(ctx, sel)`; `repo.List(ctx, nil)` for every row |
| `repo.FetchRecord(db.FV{...}, &rec)` | `repo.GetBy(ctx, map[string]any{...})` |
| `repo.FetchByKey("id", id, &rec)` | `repo.GetBy(ctx, map[string]any{"id": id})` |
| `repo.FetchWhere(db.FV{...}, &list)` | `repo.ListBy(ctx, map[string]any{...})` |
| `repo.Exists("email", v)` | `repo.Exists(ctx, gohan.Col("email").Eq(v))` |
| `repo.Exists("email", v, "id", id)` | `repo.Exists(ctx, gohan.And(gohan.Col("email").Eq(v), gohan.Col("id").Neq(id)))` |
| `repo.Count()` | `repo.Count(ctx, nil)` |
| `repo.CountWhere(db.FV{...})` | `repo.Count(ctx, gohan.Match(map[string]any{...}))` |
| `repo.Select(sql, &list, args...)` | `q.Select(ctx, &list, sql, args...)`, or build the query with gohan and use `dbx.Query[D](ctx, q, sel)` |
| a join/aggregate scanned into a DTO with `Fetch`/`Select` | `dbx.Query[D](ctx, q, sel)` / `dbx.QueryOne[D](ctx, q, sel)` |

`GetBy`, `ListBy` and `gohan.Match` differ from the `db.FV` methods in two ways. An empty map
fails with `gohan.ErrEmptyMatch` (`FetchWhere`/`CountWhere` with an empty map matched every row),
and a slice value is compared with `=`, not expanded to `IN` (goqu's `Eq` expanded it): use
`gohan.Col(k).In(values...)` for lists.

### Writes

| `db` | `dbx` / `gohan` |
|---|---|
| `repo.Insert(rec)` / `repo.Insert([]*T{...})` | `repo.Insert(ctx, rec)` / `repo.Insert(ctx, recs...)` |
| records that omit different `omitnil`/`omitempty` columns | `repo.WithGroupedInserts().Insert(ctx, recs...)` (PostgreSQL, SQLite) |
| `repo.InsertReturning(rec, fields, &out)` | `out, err := repo.InsertReturning(ctx, rec)` returns a new `*T` with every column; `rec` is not modified |
| `repo.InsertReturning(rec, []string{"id"}, &id)` | `sqlStr, args, err := gohan.Insert(repo.Table()).Rows(rec).Returning("id").Build(q.Dialect())`, then `id, err := q.QueryInt64(ctx, sqlStr, args...)` |
| `SqlInsert().OnConflict(goqu.DoNothing())` | `inserted, err := repo.InsertIgnore(ctx, rec, conflictCols...)` |
| `SqlInsert().OnConflict(goqu.DoUpdate(...))` | `repo.Upsert(ctx, rec, conflict, update...)` / `repo.UpsertReturning(ctx, rec, conflict, update...)` |
| ClickHouse `repo.InsertAsync(rec)` | no equivalent; call `chClient.Conn.AsyncInsert` directly |

### Updates

| `db` | `dbx` / `gohan` |
|---|---|
| `repo.UpdateRecord(rec, db.FV{"id": id})` | `repo.Update(ctx, rec, gohan.Match(map[string]any{"id": id}))` |
| `repo.UpdateByKey(rec, "id", id)` | `repo.Update(ctx, rec, gohan.Col("id").Eq(id))` |
| `repo.UpdateFields(&T{}, fields, db.FV{...})` | `repo.UpdateFields(ctx, fields, gohan.Match(map[string]any{...}))` |
| `repo.UpdateFieldsReturning(&T{}, fields, where, returnFields, &out)` | `rows, err := repo.UpdateReturning(ctx, fields, where)`: every updated row, all columns |
| `repo.UpdateReturning(rec, where, returnFields, &out)` | no record-based equivalent; build `gohan.Update(repo.Table()).SetRecord(rec).Where(where).Returning(cols...)` and scan it with `q.Get(ctx, &out, sqlStr, args...)`, or pass a map to `repo.UpdateReturning` |
| `repo.Update(repo.SqlUpdate().Set(goqu.Record{...}).Where(...))` | `repo.UpdateFields(ctx, map[string]any{...}, where)` or `repo.Exec(ctx, gohan.Update(repo.Table()).Set(col, v).Where(where))` |
| a PATCH map built by hand from a request body | `dbx.NewChangeset(repo)` + `dbx.SetOptional` (or `cs.Set`), then `repo.UpdateFields(ctx, cs.Changes(), where)` |
| load, modify, save the whole record | `changes, err := dbx.Changes(old, updated)`, then `repo.UpdateFields(ctx, changes, where)` when `len(changes) > 0` |

`Update` and `UpdateFields` return the number of rows affected, and `UpdateReturning` the rows;
the `db` methods returned only an error.

### Deletes

| `db` | `dbx` / `gohan` |
|---|---|
| `repo.DeleteWhere(db.FV{...})` | `repo.Delete(ctx, gohan.Match(map[string]any{...}))` |
| `repo.DeleteByKey("id", id)` | `repo.Delete(ctx, gohan.Col("id").Eq(id))` |
| `repo.Delete(repo.SqlDelete().Where(...))` | `repo.Delete(ctx, where)` |
| a delete with no `WHERE` | `repo.Exec(ctx, gohan.Delete(repo.Table()).All())` |

### Raw statements

| `db` | `dbx` / `gohan` |
|---|---|
| `repo.RawExec(sql, args...)` | `n, err := q.Exec(ctx, sql, args...)` |
| `repo.Exec(ds)` | `repo.Exec(ctx, st)` for any gohan statement (returns rows affected) |
| `db.Fetch`, `db.RawExec` and the other package-level `db` functions | the matching `Repository` method, or `q.Get`/`q.Select`/`q.Exec`/`q.QueryInt64` with raw SQL |

### Transactions

| `db` | `dbx` / `gohan` |
|---|---|
| `tx, _ := repo.NewTransaction(opts)` … `tx.Commit()` / `tx.Rollback()` | `dbx.WithTx(ctx, q, opts, func(tx dbx.Querier) error { ... repo.With(tx) ... })` |
| the same, when the transaction must stay open across calls | `tx, err := q.BeginTx(ctx, opts)` (a `dbx.TxQuerier`), `repo.With(tx)`, then `tx.Commit()`/`tx.Rollback()` |
| `tx.Insert(...)`, `tx.FetchOne(...)`, ... | the same `dbx` methods on `repo.With(tx)`, for any repository |
| `tx.Db()` (`*sqlx.Tx`) | no equivalent; run raw SQL with `tx.Exec`/`tx.Get`/`tx.Select` |
| a loop retrying on serialization failures | `dbx.WithTxRetry(ctx, q, opts, attempts, fn)` |
| `set_config`/`SET LOCAL ROLE` at the start of a transaction | `pgsql.WithTxSession(ctx, q, settings, role, fn)` |

### Grid and errors

| `db` | `dbx` / `gohan` |
|---|---|
| `repo.Grid(&T{})` / `db.NewGrid(table, &T{})` | `g, err := dbx.NewGrid[T]()`, built once and reused |
| `repo.QueryGrid(&T{}, gq, &list)` | `list, err := repo.QueryGrid(ctx, g, gq)` |
| a second hand-built `COUNT` for the grid total | `list, total, err := repo.QueryGridWithCount(ctx, g, gq)`; `g.Conds(gq)` for other aggregates |
| `grid.Build(ds, gq)` | `g.Build(repo.Select(), gq)` |
| `db.EmptyResult(err)` | `errors.Is(err, dbx.ErrNotFound)` |
| `err.(db.GridError)` | `errors.As(err, &gridErr)` with `var gridErr dbx.GridError` |
| `db.FV{...}` | `map[string]any{...}` |

## db.Grid → dbx.Grid

| | `db.Grid` | `dbx.Grid[T]` |
|---|---|---|
| Construction | `db.NewGrid(table, &T{})`, or a new grid per `repo.QueryGrid` call | `dbx.NewGrid[T]()` once; `AddFilterFunc`, `WithMaxLimit`, `WithTiebreaker` and `WithCaseInsensitiveSearch` mutate it, so configure it before concurrent use |
| Addressable fields | any field with a `db` tag; a field without the right grid flag answers "field is not filterable"/"not sortable" | only fields with a `grid` flag; any other field answers "field is not valid". "not filterable"/"not sortable" remain for a field flagged for another grid operation |
| Aliases | a duplicate alias silently overwrites the earlier one | `NewGrid` fails on a duplicate, empty or `"-"` alias among grid fields, and on a searchable field that is neither string-kind nor a `driver.Valuer` |
| Base query | `Build(nil, …)` selects `*` from the table | a nil base fails ("base query is required"); a `UNION` base fails (wrap it: `gohan.From(u.As("u"))`); `QueryGrid` uses `repo.Select()` |
| Nil `GridQuery` | panics | `GridError` "query is required" |
| Sort | `SortFields` map, applied in Go map order (unstable) | `Sort` list (`[{"field": ..., "order": ...}]`) in the order given, or `SortFields` in alias order; not both; a repeated field is rejected |
| Default direction | descending | descending (unchanged) |
| Tiebreaker | none | `WithTiebreaker(cols...)` appends a unique key, ascending, so equal sort values keep a stable order across pages |
| Row cap | `Limit == 0` returns every row | a new grid caps `Limit` at `dbx.DefaultMaxLimit` (1000): `Limit == 0` or a larger `Limit` returns at most 1000 rows. `WithMaxLimit(n)` changes the cap; `WithMaxLimit(0)` removes it |
| Offset/limit range | unchecked | above `math.MaxInt64` is a `GridError`; `GridQuery.Page` saturates instead of overflowing |
| Search wildcards | `%` and `_` in the search text are wildcards | `%` and `_` match literally |
| Case-insensitive search | not available (`LIKE` only) | `WithCaseInsensitiveSearch()`: `ILIKE` on PostgreSQL and ClickHouse, `LIKE` on SQLite (whose `LIKE` already ignores ASCII case) |
| Search on a type without searchable fields | no search condition; unfiltered rows are returned | `GridError` "no searchable fields" |
| Search text | unbounded; search type only checked when text is present | at most `dbx.MaxSearchText` (256) bytes; `SearchType` always range-checked |
| Filter values | anything the JSON decoder produced, passed to goqu | JSON scalars (`nil`, `bool`, `float64`, `string`, `json.Number`) or a flat list of at most `dbx.MaxFilterValues` (1000) of them; anything else is "value is not valid" |
| Filter func results | used as returned | a `gohan` expression or a map is "value is not valid"; a typed slice becomes an `IN` list (at most 1000 values); arrays, byte slices and `driver.Valuer`s bind as one value |
| Filter order | Go map order | sorted by alias, so the same query always builds the same SQL |
| Total count | build a second query yourself | `QueryGridWithCount` / `Conds` |
| JSON field names | `SearchType` (the field had a `db` tag, not a `json` tag), `searchText`, `filterFields`, `sortFields`, `offset`, `limit` | `searchType`, plus the new `sort`; the others are unchanged. Decoding is case-insensitive, so an old client sending `"SearchType"` still decodes |
| `GridError` | `db.GridError{Scope, Field, Message}` | `dbx.GridError`, same fields, JSON and `Error()` text |
| Configuration errors | none | `WithMaxLimit` above `math.MaxInt64` or `WithTiebreaker` with a column `T` does not map: a plain error (not a `GridError`) from `ValidQuery` and `Build` |

`dbx.GridError` scopes and messages:

| Scope | Messages |
|---|---|
| `query` | "query is required", "base query is required", "base query must not be a UNION; wrap it with gohan.From(q.As(...))", "offset is out of range", "limit is out of range"; `QueryGridKeyset` only: "offset is not allowed with cursor pagination" |
| `filter` | "field is not valid", "field is not filterable", "value is not valid" |
| `sort` | "field is not valid", "field is not sortable", "sort order is not valid", "field is repeated", "use either sort or sortFields, not both"; `QueryGridKeyset` only: "field cannot be used with cursor pagination", "too many sort fields for cursor pagination" |
| `search` | "search text too long", "search not allowed", "invalid search type", "no searchable fields" |
| `cursor` | "cursor is not valid" (`QueryGridKeyset` only; `errors.Is(err, dbx.ErrInvalidCursor)` holds) |

An error returned by a `GridFilterFunc` is passed through unchanged, as in `db`. For large tables
where `OFFSET` paging gets slow, see
[Cursor pagination](dbx.md#cursor-pagination-listkeyset-and-querygridkeyset) in the dbx guide.

## goqu → gohan expressions

`gohan` builders are immutable: each call returns a new builder.

| goqu | gohan |
|---|---|
| `goqu.C("a")` | `gohan.Col("a")` |
| `goqu.I("t.a")` | `gohan.Col("t.a")` (renders `"t"."a"`) |
| `goqu.T("users")`, `goqu.T("users").As("u")` | `gohan.Table("users")`, `gohan.Table("users").As("u")`; `.Col("id")` qualifies a column with the table or alias |
| `goqu.L("x @> ?", v)` | `gohan.Raw("x @> ?", v)`; a literal `?` is written `??` (rejected on ClickHouse). Raw SQL text must never come from request data |
| `goqu.L("1")` for a constant | `gohan.Int(1)` (inline literal); `gohan.Raw("NOW()")` for other SQL constants |
| `goqu.V(v)` | `gohan.Val(v)` |
| `goqu.Star()` | `gohan.Star()` |
| `goqu.Ex{"a": 1, "b": nil}` | `gohan.Match(map[string]any{"a": 1, "b": nil})` (`nil` renders `IS NULL`; an empty map fails with `gohan.ErrEmptyMatch`) |
| `goqu.Ex{"a": []int{1, 2}}` | `gohan.Col("a").In(1, 2)`: `Match` compares a slice with `=`, it does not expand it |
| `goqu.Ex{"a": goqu.Op{"gt": 5}}` | `gohan.Col("a").Gt(5)` |
| `goqu.ExOr{"a": 1, "b": 2}` | `gohan.Or(gohan.Col("a").Eq(1), gohan.Col("b").Eq(2))` |
| `goqu.And(...)`, `goqu.Or(...)` | `gohan.And(...)`, `gohan.Or(...)`; `gohan.Not(e)` |
| `.Eq`, `.Neq`, `.Gt`, `.Gte`, `.Lt`, `.Lte` | same names; `Eq(nil)`/`Neq(nil)` render `IS NULL`/`IS NOT NULL`; the argument may be another column (`gohan.Col("a").Gt(gohan.Col("b"))`) |
| `.In(...)`, `.NotIn(...)` | `.In(values...)`, `.NotIn(values...)`; a single slice argument is expanded, a `*gohan.SelectBuilder` becomes a subquery; an empty `In` renders `1=0` |
| `.IsNull()`, `.IsNotNull()` | `.IsNull()`, `.IsNotNull()` |
| `.Like(p)`, `.NotLike(p)` | `.Like(p)`, `.NotLike(p)`; to match user text use `.Contains(s)`, `.HasPrefix(s)`, `.HasSuffix(s)`, which escape `%` and `_` |
| `.ILike(p)`, `.NotILike(p)` | `.ILike(p)`, `.NotILike(p)` (`gohan.ErrUnsupported` on SQLite); `.ContainsFold`/`.HasPrefixFold`/`.HasSuffixFold` work on every dialect |
| `.Between(goqu.Range(lo, hi))` | `.Between(lo, hi)`, `.NotBetween(lo, hi)` |
| `ds.Order(goqu.C("a").Desc())`, `ds.OrderAppend(...)` | `sel.OrderBy(gohan.Col("a").Desc())` (appends); `.Asc()`, `.NullsFirst()`, `.NullsLast()` |
| `ds.Limit(n)`, `ds.Offset(n)` | `sel.Limit(n)`, `sel.Offset(n)` (`uint64`) |
| `ds.Select(cols...)`, `ds.SelectAppend(cols...)` | `gohan.Select(cols...)` or `sel.Columns(cols...)` (replaces); `sel.AddColumns(cols...)` (appends) |
| `ds.Distinct()`, `ds.GroupBy(...)`, `ds.Having(...)` | `sel.Distinct()`, `sel.GroupBy(...)`, `sel.Having(...)` |
| `ds.Join(goqu.T("p"), goqu.On(goqu.Ex{"p.user_id": goqu.I("u.id")}))` | `sel.Join(gohan.Table("p"), gohan.Col("p.user_id").Eq(gohan.Col("u.id")))`; also `LeftJoin`, `RightJoin`, `FullJoin`, `CrossJoin`, `JoinUsing`, `LeftJoinUsing` |
| subquery as a source: `goqu.From(ds.As("s"))` | `gohan.From(sel.As("s"))` |
| subquery as a value, `IN (subquery)`, `EXISTS` | `gohan.Sub(sel)`, `gohan.Col("a").In(sel)`, `gohan.Exists(sel)`, `gohan.NotExists(sel)` |
| `ds.With("x", sub)`, `ds.Union(other)`, `ds.UnionAll(other)` | `sel.With("x", sub)` (`WithRecursive`), `sel.Union(other)`, `sel.UnionAll(other)` |
| `goqu.COUNT("*")`, `goqu.COUNT("id")` | `gohan.CountAll()`, `gohan.Count("id")` |
| `goqu.SUM`, `goqu.AVG`, `goqu.MIN`, `goqu.MAX` | `gohan.Sum`, `gohan.Avg`, `gohan.Min`, `gohan.Max` |
| `goqu.Func("lower", goqu.C("a"))` | `gohan.Fn("lower", gohan.Col("a"))` (the name is trusted input) |
| `goqu.Cast(goqu.C("a"), "TEXT")` | `gohan.Cast(gohan.Col("a"), "TEXT")` (the type is trusted input) |
| `goqu.Case().When(cond, v).Else(w)` | `gohan.Case().When(cond, v).Else(w)`, or `.End()` without `ELSE` |
| `expr.As("n")` | `value.As("n")` |
| `ds.ForUpdate(goqu.Wait)` | `sel.ForUpdate()` (PostgreSQL only; `SkipLocked()`, `NoWait()`) |
| `goqu.Record{"a": 1}` in `Set`/`Rows` | `gohan.Update(t).Set("a", 1)` / `.SetMap(m)` / `.SetRecord(rec)`; `gohan.Insert(t).SetMap(m)` / `.Rows(recs...)` / `.Columns(...).Values(...)` |
| `OnConflict(goqu.DoNothing())` | `.OnConflict(cols...).DoNothing()` |
| `OnConflict(goqu.DoUpdate("k", goqu.Record{"b": goqu.I("excluded.b")}))` | `.OnConflict("k").DoUpdate(map[string]any{"b": gohan.Excluded("b")})` or `.DoUpdateExcluded("b")` |
| `.Returning(...)` | `.Returning(cols...)` on insert, update and delete |
| ClickHouse `FINAL`, `PREWHERE`, `SAMPLE`, `ARRAY JOIN`, `SETTINGS` via `goqu.L` | `sel.Final()`, `sel.Prewhere(...)`, `sel.Sample(r)`/`SampleRows(n)`, `sel.ArrayJoin(...)`/`LeftArrayJoin(...)`, `sel.Settings(m)` |

## Behaviour changes

These are intentional. Code that relies on the old behaviour needs to change, not just its call
syntax.

- **Unfiltered DELETE/UPDATE refused.** `Delete`, `Update`, `UpdateFields` and `UpdateReturning`
  fail with `gohan.ErrNoWhere`, before touching the database, for a nil `where`, and gohan
  rejects a `where` that cannot restrict rows (`gohan.And()`, an empty `NotIn`,
  `gohan.Raw("1=1")`). To affect every row, say so: `repo.Exec(ctx, gohan.Delete(table).All())`.
  `db`'s `DeleteWhere` and record updates also refuse an empty condition, but a goqu
  `SqlDelete()`/`SqlUpdate()` dataset without `Where` still affects every row.
- **Empty maps are an error.** `GetBy`, `ListBy` and `gohan.Match` fail with `gohan.ErrEmptyMatch`
  on an empty map; `FetchRecord`, `FetchWhere` and `CountWhere` with an empty, non-nil map
  matched every row. Use `repo.List(ctx, nil)`/`repo.Count(ctx, nil)` to mean "all rows".
- **Slices in equality maps are not expanded.** `db`'s `FV` methods used goqu's `Eq`, which
  turned a slice into `IN (...)`; `GetBy`/`ListBy`/`gohan.Match`/`Col(...).Eq` bind the slice as
  one value. Use `gohan.Col(k).In(values...)`.
- **`ctx` on every call**, not a context stored at construction: a repository is no longer tied to
  one context or deadline for its lifetime.
- **Reads return new values.** `Get`, `InsertReturning`, `UpsertReturning` and the other reads
  return a new `*T`; the record you pass is never written to.
- **Inserts are chunked and atomic.** `Insert` splits records at the dialect's bound-argument
  limit (65535 on PostgreSQL, 32766 on SQLite) and runs several chunks in one transaction, so a
  multi-chunk insert over a Querier that cannot begin a transaction fails with
  `dbx.ErrTxUnsupported`. `db` sent one statement, which failed past the limit. Inserting zero
  records is a no-op; `db` returned an error.
- **Rows affected are returned.** `Update`, `UpdateFields`, `Delete` and `Exec` return the count
  the `db` methods discarded. On ClickHouse it is always `0`, since ClickHouse does not report it:
  do not branch on `n == 0` there.
- **`dbx` selects an explicit column list, never `SELECT *`.** A struct field with no matching
  column fails the query ("missing destination name"), as `db`'s `SELECT *` did. A table column
  the struct does not map is now simply not selected, where `SELECT *` fetched it and failed to
  scan it.
- **Record shapes that could bind the wrong data are rejected.** An embedded pointer struct, an
  unexported or `db`/`ch`-tagged embedded struct, an ambiguous promoted field name, or two fields
  mapped to one column fail at `NewRepository` (`gohan.ErrRecordShape`,
  `gohan.ErrDuplicateColumn`) instead of building a statement that writes the wrong field.
- **Tag parts are not trimmed.** `db:"id, auto"` and `grid:"sort, filter"` lose their second
  option under `dbx` (see [step 2](#checklist-migrating-one-repository)).
- **ClickHouse records must map the same columns in `db` and `ch` tags**, or `NewRepository`
  fails with `clickhouse.ErrRecordMapping`.
- **Grid changes.** The 1000-row default cap, literal search wildcards, stricter validation and
  "field is not valid" for fields without a grid flag are listed in
  [db.Grid → dbx.Grid](#dbgrid-dbxgrid).
- **`Int(n)` for integer constants.** An integer bound as a value inside a `CASE` fed to `SUM` is
  sent as text on PostgreSQL and fails with `function sum(text) does not exist`; write the
  constant with `gohan.Int(n)`, which renders it inline.
- **PostgreSQL: a bound value alone in a select list may need `gohan.Cast(v, "type")`**, when
  PostgreSQL fails with "could not determine data type of parameter".
- **SQLite identifiers are quoted with backticks**, not double quotes: SQLite silently reads an
  unknown double-quoted identifier as a string literal, and backticks avoid that. SQLite's `LIKE`
  ignores ASCII case; PostgreSQL's and ClickHouse's do not (use `ILIKE` or the `Fold` helpers).
- **Numeric grid filters arrive as `float64`.** A `GridQuery` decoded from JSON carries
  `{"id": 3.9}` as `3.9`, which matches `id = 3` on PostgreSQL (pgx truncates). Register a
  `GridFilterFunc` on integer columns to reject non-integer input if that matters.
- **ClickHouse restrictions.** A literal `?` is rejected in identifiers and in `Raw`'s `??` form.
  Values ClickHouse's driver cannot bind safely are rejected with `gohan.ErrUnsafeValue`: maps,
  structs other than `time.Time` (unless the value is a top-level `driver.Valuer` or a
  `fmt.Stringer`), and error or `fmt.Formatter` values, also inside a slice. A `UNION` with an
  outer `ORDER BY`/`LIMIT`/`OFFSET` is wrapped as `SELECT * FROM (<union>) ...`, and
  `Delete(t).All()` renders `... WHERE 1`. Lightweight `DELETE` does not work on Distributed
  tables or tables with projections. `UPDATE`, `RETURNING`, `ON CONFLICT` and transactions are
  not available (`gohan.ErrUnsupported`, `dbx.ErrTxUnsupported`).
- **ClickHouse times keep their precision.** Statements built through `client.Querier()` use named
  placeholders, and a bound `time.Time` (also `*time.Time` or a `driver.Valuer` producing one) is
  sent as a `DateTime64` literal at the smallest exact scale, never below milliseconds, so
  comparisons are by exact instant. A time in an IANA zone keeps its zone; `time.Local` and
  `time.FixedZone` times are sent as the same instant in UTC. SQL you write yourself with
  positional `?` placeholders still gets clickhouse-go's whole-second binding. See
  [ClickHouse](dbx.md#clickhouse) in the dbx guide.

## Migrating incrementally

- **One client, both packages.** `dbx.FromClient` takes the same `*db.SqlClient` that
  `db.NewRepository` uses, and `client.Querier()` wraps the same ClickHouse connection as the
  legacy repository. Move one repository and its callers at a time, test, then move the next.
- **Transactions do not cross packages.** A `db.Transaction` and a `dbx.WithTx` transaction are
  separate transactions on separate connections, and neither can join the other. Move every
  statement of a transaction to `dbx` together. On SQLite with `MaxOpenConns == 1`, mixing them
  deadlocks.
- **Migrations are unchanged.** `db/migrations` and the provider migration managers
  (`pgsql.NewMigrationManager`, `sqlite.NewMigrationManager`, `clickhouse.NewMigrationManager`)
  take the same `*db.SqlClient` or ClickHouse `*Client` and keep using the legacy repositories
  internally for their bookkeeping table. `dbx` has no migration API: run the migrations as
  before, then build the `dbx` repositories.
- **Struct types can be shared** while both packages are in use, as long as their tags follow
  [step 2](#checklist-migrating-one-repository): `db.Repository` reads the same tag forms.

## Worked example

A PostgreSQL user repository with CRUD, a PATCH, a transaction writing two tables, and a grid
endpoint that returns a page and its total.

### Before: db.Repository

```go
package users

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/doug-martin/goqu/v9"

	"github.com/oddbit-project/blueprint/db"
)

type User struct {
	ID        int64     `db:"id" json:"id" goqu:"skipinsert" grid:"sort,filter"`
	Email     string    `db:"email" json:"email" grid:"sort,search,filter"`
	Name      string    `db:"name" json:"name" grid:"sort,search"`
	Phone     *string   `db:"phone" json:"phone"`
	Active    bool      `db:"active" json:"active" grid:"filter"`
	CreatedAt time.Time `db:"created_at" json:"createdAt" goqu:"skipinsert" grid:"sort"`
}

type AuditEntry struct {
	ID     int64  `db:"id" goqu:"skipinsert"`
	UserID int64  `db:"user_id"`
	Action string `db:"action"`
}

type Repository struct {
	ctx   context.Context
	users db.Repository
}

func NewRepository(ctx context.Context, client *db.SqlClient) *Repository {
	return &Repository{ctx: ctx, users: db.NewRepository(ctx, client, "users")}
}

func (r *Repository) ByID(id int64) (*User, error) {
	u := &User{}
	if err := r.users.FetchByKey("id", id, u); err != nil {
		return nil, err
	}
	return u, nil
}

// Create inserts u and copies the generated id and created_at back into it.
func (r *Repository) Create(u *User) error {
	return r.users.InsertReturning(u, []string{"id", "created_at"}, u)
}

// UserPatch is a PATCH body: a nil field is left unchanged.
type UserPatch struct {
	Name  *string `json:"name"`
	Phone *string `json:"phone"`
}

func (r *Repository) Patch(id int64, p UserPatch) error {
	fields := db.FV{}
	if p.Name != nil {
		fields["name"] = *p.Name
	}
	if p.Phone != nil {
		fields["phone"] = *p.Phone // no way to tell "absent" from "null"
	}
	if len(fields) == 0 {
		return nil
	}
	return r.users.UpdateFields(&User{}, fields, db.FV{"id": id})
}

func (r *Repository) Delete(id int64) error {
	return r.users.DeleteByKey("id", id)
}

func (r *Repository) EmailTaken(email string, exceptID int64) (bool, error) {
	return r.users.Exists("email", email, "id", exceptID)
}

// Deactivate marks the user inactive and writes an audit entry, atomically.
func (r *Repository) Deactivate(id int64) error {
	tx, err := r.users.NewTransaction(nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if err := tx.UpdateFields(&User{}, db.FV{"active": false}, db.FV{"id": id}); err != nil {
		return err
	}
	// tx is bound to "users"; another table needs a hand-built goqu dataset
	audit := goqu.Dialect("pgx").Insert("user_audit").
		Rows(&AuditEntry{UserID: id, Action: "deactivate"})
	if err := tx.Do(audit); err != nil {
		return err
	}
	return tx.Commit()
}

// Page returns one grid page and the number of rows its filters match.
func (r *Repository) Page(q *db.GridQuery) ([]*User, int64, error) {
	var rows []*User
	if err := r.users.QueryGrid(&User{}, q, &rows); err != nil {
		return nil, 0, err
	}
	// the total needs the same filters and search, without sort and paging
	grid, err := r.users.Grid(&User{})
	if err != nil {
		return nil, 0, err
	}
	countQ := *q
	countQ.SortFields, countQ.Limit, countQ.Offset = nil, 0, 0
	sel, err := grid.Build(r.users.SqlSelect().Select(goqu.COUNT(goqu.Star())), &countQ)
	if err != nil {
		return nil, 0, err
	}
	total, err := db.Count(r.ctx, r.users.Db(), sel)
	if err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func GetHandler(repo *Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		id, err := strconv.ParseInt(req.PathValue("id"), 10, 64)
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}
		u, err := repo.ByID(id)
		if db.EmptyResult(err) {
			http.NotFound(w, req)
			return
		}
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, u)
	}
}

func GridHandler(repo *Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		var q db.GridQuery
		if err := json.NewDecoder(req.Body).Decode(&q); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		rows, total, err := repo.Page(&q)
		if gridErr, ok := err.(db.GridError); ok {
			writeJSON(w, http.StatusBadRequest, gridErr)
			return
		}
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"rows": rows, "total": total})
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
```

### After: dbx

```go
package users

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/oddbit-project/blueprint/db"
	"github.com/oddbit-project/blueprint/dbx"
	"github.com/oddbit-project/blueprint/types/optional"
	"github.com/oddbit-project/gohan"
)

// Tags are unchanged: goqu:"skipinsert" still marks a column auto.
type User struct {
	ID        int64     `db:"id" json:"id" goqu:"skipinsert" grid:"sort,filter"`
	Email     string    `db:"email" json:"email" grid:"sort,search,filter"`
	Name      string    `db:"name" json:"name" grid:"sort,search"`
	Phone     *string   `db:"phone" json:"phone"`
	Active    bool      `db:"active" json:"active" grid:"filter"`
	CreatedAt time.Time `db:"created_at" json:"createdAt" goqu:"skipinsert" grid:"sort"`
}

type AuditEntry struct {
	ID     int64  `db:"id" goqu:"skipinsert"`
	UserID int64  `db:"user_id"`
	Action string `db:"action"`
}

type Repository struct {
	q     dbx.Querier
	users *dbx.Repository[User]
	audit *dbx.Repository[AuditEntry]
	grid  *dbx.Grid[User]
}

func NewRepository(client *db.SqlClient) (*Repository, error) {
	q, err := dbx.FromClient(client)
	if err != nil {
		return nil, err
	}
	users, err := dbx.NewRepository[User](q, "users")
	if err != nil {
		return nil, err
	}
	audit, err := dbx.NewRepository[AuditEntry](q, "user_audit")
	if err != nil {
		return nil, err
	}
	grid, err := dbx.NewGrid[User]()
	if err != nil {
		return nil, err
	}
	// configure once, before concurrent use: these mutate the grid
	grid.WithMaxLimit(100).WithTiebreaker("id")
	return &Repository{q: q, users: users, audit: audit, grid: grid}, nil
}

func (r *Repository) ByID(ctx context.Context, id int64) (*User, error) {
	return r.users.GetBy(ctx, map[string]any{"id": id})
}

// Create inserts u and returns the stored row (generated id and created_at
// included); u itself is not modified.
func (r *Repository) Create(ctx context.Context, u *User) (*User, error) {
	return r.users.InsertReturning(ctx, u)
}

// UserPatch is a PATCH body: absent, null and a value stay distinct.
type UserPatch struct {
	Name  optional.Optional[string] `json:"name,omitzero"`
	Phone optional.Optional[string] `json:"phone,omitzero"`
}

func (r *Repository) Patch(ctx context.Context, id int64, p UserPatch) error {
	cs := dbx.NewChangeset(r.users)
	if err := errors.Join(
		dbx.SetOptional(cs, "name", p.Name),
		dbx.SetOptional(cs, "phone", p.Phone), // Null clears the column
	); err != nil {
		return err
	}
	changes := cs.Changes()
	if len(changes) == 0 {
		return nil
	}
	_, err := r.users.UpdateFields(ctx, changes, gohan.Col("id").Eq(id))
	return err
}

func (r *Repository) Delete(ctx context.Context, id int64) error {
	_, err := r.users.Delete(ctx, gohan.Col("id").Eq(id))
	return err
}

func (r *Repository) EmailTaken(ctx context.Context, email string, exceptID int64) (bool, error) {
	return r.users.Exists(ctx, gohan.And(gohan.Col("email").Eq(email), gohan.Col("id").Neq(exceptID)))
}

// Deactivate marks the user inactive and writes an audit entry, atomically.
func (r *Repository) Deactivate(ctx context.Context, id int64) error {
	return dbx.WithTx(ctx, r.q, nil, func(tx dbx.Querier) error {
		n, err := r.users.With(tx).UpdateFields(ctx, map[string]any{"active": false}, gohan.Col("id").Eq(id))
		if err != nil {
			return err
		}
		if n == 0 {
			return dbx.ErrNotFound
		}
		return r.audit.With(tx).Insert(ctx, &AuditEntry{UserID: id, Action: "deactivate"})
	})
}

// Page returns one grid page and the number of rows its filters match.
func (r *Repository) Page(ctx context.Context, q *dbx.GridQuery) ([]*User, int64, error) {
	return r.users.QueryGridWithCount(ctx, r.grid, q)
}

func GetHandler(repo *Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		id, err := strconv.ParseInt(req.PathValue("id"), 10, 64)
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}
		u, err := repo.ByID(req.Context(), id)
		if errors.Is(err, dbx.ErrNotFound) {
			http.NotFound(w, req)
			return
		}
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, u)
	}
}

func GridHandler(repo *Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		var q dbx.GridQuery
		if err := json.NewDecoder(req.Body).Decode(&q); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		rows, total, err := repo.Page(req.Context(), &q)
		var gridErr dbx.GridError
		if errors.As(err, &gridErr) {
			writeJSON(w, http.StatusBadRequest, gridErr)
			return
		}
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"rows": rows, "total": total})
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
```

What changed:

- Every method takes `ctx`; `NewRepository` returns an error, so a bad record type fails at
  startup.
- `Create` returns the stored row instead of scanning into its argument.
- `Patch` can now clear `phone` (`{"phone": null}`), and each value is checked against `User`
  before the `UPDATE` runs.
- `Deactivate` writes both tables through one `WithTx`, with no goqu dataset for the second
  table, and reports a missing user as `dbx.ErrNotFound`.
- `Page` returns the total from `QueryGridWithCount` instead of a second hand-built query. The
  grid caps pages at 100 rows and sorts ties by `id`.
- Handlers match `dbx.ErrNotFound` and `dbx.GridError` with `errors.Is`/`errors.As`.
