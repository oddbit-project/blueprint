# sqlb Query Builder

`sqlb` is a SQL query builder where values are always bound and identifiers are always quoted
and escaped by construction, so a caller cannot reintroduce SQL injection through the normal API.
It replaces the pattern of building SQL with goqu and inlined values (the class of bug fixed in
commit `654df80`) — see [Migrating to dbx](migrating-to-dbx.md) for a full mapping.

> Three escape hatches exist for trusted input only, and must never be built from request data:
> `Raw`'s SQL text, `Fn`'s function name, and `Cast`'s type string.

## Overview

- `Select`, `Insert`, `Update`, `Delete` builders, each immutable — every method returns a new
  value and never mutates the receiver.
- Three built-in dialects: `sqlb.Postgres()`, `sqlb.SQLite()`, `sqlb.ClickHouse()` (plus
  `sqlb.Generic()`, ANSI SQL — never register it for MySQL, where a double-quoted string is a
  literal, not an identifier).
- `Build(dialect) (string, []any, error)` renders a statement; nothing runs a query — `sqlb`
  has no I/O.
- `dbx` (see [dbx Repositories](dbx.md)) builds every statement it runs through `sqlb`.

## String-position rule

Whether a Go `string` is treated as a column name or a bound value depends on where it appears:

- **Column position**: the argument to `Count`, `Sum`, `Avg`, `Min`, `Max`, select-list columns,
  `GROUP BY`, `ORDER BY`, and `INSERT`/`UPDATE` column names — a string there is a column name.
- **Value position**: comparison right-hand sides, `Fn` arguments, `Raw` arguments, `Cast`'s
  value, `Case`'s `WHEN`/`THEN`/`ELSE`, and `Val` — a string there is a bound value. Pass
  `Col("x")` when a column is meant.

## SELECT, WHERE, ORDER BY, LIMIT

```go
sql, args, err := sqlb.Select("id", "name").
    From("users").
    Where(sqlb.Col("active").Eq(true)).
    OrderBy(sqlb.Col("name").Asc()).
    Limit(10).
    Build(sqlb.Postgres())
// SELECT "id", "name" FROM "users" WHERE "active" = $1 ORDER BY "name" ASC LIMIT 10 [true]
```

(`sqlb/example_test.go`'s `ExampleSelect`.)

## INSERT and upsert

```go
sql, args, err := sqlb.Insert("users").
    Columns("id", "name", "email").
    Values(1, "alice", "alice@example.com").
    OnConflict("id").
    DoUpdateExcluded("name", "email").
    Build(sqlb.Postgres())
// INSERT INTO "users" ("id", "name", "email") VALUES ($1, $2, $3)
// ON CONFLICT ("id") DO UPDATE SET "name" = excluded."name", "email" = excluded."email"
```

(`ExampleInsert_upsert`.) `Rows(records...)` builds an `INSERT` from structs instead of
`Values`/`Columns`; `sqlb.InsertColumns(rec)` and `sqlb.RecordColumns(t)` report the column list a
record would use without building a statement. `OnConflict(cols...).DoNothing()` /
`.DoUpdate(map[string]any{...})` are the other two conflict actions.

## UPDATE

```go
sql, args, err := sqlb.Update("users").
    Set("name", "bob").
    Where(sqlb.Col("id").Eq(1)).
    Build(sqlb.Postgres())
// UPDATE "users" SET "name" = $1 WHERE "id" = $2 [bob 1]
```

(`ExampleUpdate`.) `SetRecord(rec, opts...)` sets one `col = v` per updatable field of a struct,
with `RecordOption`s (`SkipZeroValues`, `IncludeFields`, `ExcludeFields`, `WithAutoFields`) to
control which fields are written.

## DELETE and UPDATE require a WHERE

`Delete` and `Update` refuse to build with no `WHERE` clause, or one that is trivially true (e.g.
`And()` with no arguments, or an empty `NotIn`) — there is no accidental "delete every row". Both
fail at `Build` with `sqlb.ErrNoWhere`:

```go
sql, args, err := sqlb.Delete("users").Build(sqlb.Postgres())
// [] sqlb: statement requires a WHERE clause; call All() to affect every row
```

(`ExampleDelete_requiresWhere`.) A caller who means to affect every row calls `.All()`:
`sqlb.Delete("users").All()`. On ClickHouse specifically, `Delete(t).All()` with no other
condition renders `... WHERE 1` (ClickHouse's lightweight-delete syntax requires a `WHERE`
clause even to select every row); on PostgreSQL/SQLite the `WHERE` clause is omitted entirely.

The trivially-true/false check sees through `Not`/`Or` and `Val`/subqueries, not just a bare
`And()`/`NotIn()`: `Not(sqlb.Or())` (the negation of an always-false `OR`) is recognized as
always-true and refused the same way. This check is per-condition, not per-statement: a `WHERE`
that combines a real, non-trivial condition with a trivially-true one — e.g.
`Where(sqlb.Col("tenant").Eq(id), sqlb.Col("status").NotIn())` ("tenant, and status not in
nothing") — is **not** refused, because the overall `AND` is not trivially true; it deletes/updates
every row of that tenant. `NotIn()` with no arguments is only ever a bug in the caller's own
condition-building code, not a caller expressing "no rows" — write the condition you mean instead
of relying on the guard to catch this shape.

## Raw: the escape hatch

`Raw(sql, args...)` renders `sql` text verbatim, substituting each `?` marker with the
corresponding argument (bound, or rendered inline if it is itself an `Expr`); `??` writes a
literal `?` without consuming an argument — needed because PostgreSQL itself uses a bare `?` as
an operator (e.g. jsonb's key-exists operator, `data ? 'key'`), which would otherwise collide
with `Raw`'s own placeholder syntax. `Raw`'s SQL text is trusted input and must never be built
from request data.

```go
sql, args, err := sqlb.Select().
    From("docs").
    Where(sqlb.Raw("data ?? ?", "owner")).
    Build(sqlb.Postgres())
// SELECT * FROM "docs" WHERE (data ? $1) [owner]
```

(`ExampleRaw`.) Note the parentheses: a `Raw` expression is parenthesized when it is rendered as
an operand of `And`/`Or`/`Not`/a comparison (`WHERE (data ? $1)` above) or as a select-list
column (`SELECT (k + 1) ...`), so it composes safely next to other expressions there. It renders
bare — no parentheses — as an `UPDATE`/`DO UPDATE` SET value (`SET "x" = y + 1`, not
`SET "x" = (y + 1)`), including through `Val(Raw(...))`, which unwraps to the same `Raw` value.
`??` is rejected outright on ClickHouse (`ErrRawPlaceholder`) — there is no literal `?` in
ClickHouse `Raw` text.
A `$` followed by a digit is also rejected everywhere (it would collide with PostgreSQL's own
`$n` placeholder syntax).

## Int for integer constants

`Int(n int64)` renders `n` as an inline integer literal (negative values parenthesized), for
places where a *bound* constant would be typed wrong. The canonical case is an integer constant
inside a `CASE` fed to `SUM` on PostgreSQL: a bound `int` argument is sent as `text`, and
`SUM(CASE ... THEN $1 ...)` fails with `function sum(text) does not exist`. `Int` avoids the
bind entirely:

```go
sql, args, err := sqlb.Select(
    sqlb.Sum(sqlb.Case().
        When(sqlb.Col("status").Eq("done"), sqlb.Int(1)).
        Else(sqlb.Int(0))).As("done_count"),
).From("tasks").Build(sqlb.Postgres())
// SELECT SUM(CASE WHEN "status" = $1 THEN 1 ELSE 0 END) AS "done_count" FROM "tasks" [done]
```

(`ExampleCase`.) `Sub(q)` renders a scalar subquery, `(<select>)`, sharing the outer statement's
placeholder counter.

## UNION

```go
q := sqlb.Select("id").From("t1").Union(sqlb.Select("id").From("t2"))

sqlPg, _, _ := q.Build(sqlb.Postgres())
// SELECT "id" FROM "t1" UNION SELECT "id" FROM "t2"

sqlCh, _, _ := q.Build(sqlb.ClickHouse())
// SELECT "id" FROM "t1" UNION DISTINCT SELECT "id" FROM "t2"
```

(`ExampleSelectBuilder_Union`.) `UnionAll` is always `UNION ALL`. A UNION member cannot have its
own `ORDER BY`, `LIMIT`, `OFFSET`, `WITH`, `SETTINGS` or `UNION` (`ErrCompoundPart`). On
ClickHouse, a UNION with an outer `ORDER BY`/`LIMIT`/`OFFSET` is wrapped as
`SELECT * FROM (<union>) ORDER BY ... LIMIT ...` — ClickHouse's own UNION grammar does not
accept a trailing `ORDER BY`/`LIMIT` the way PostgreSQL/SQLite do.

`SelectBuilder.IsCompound()` reports whether a builder has any UNION members. `Where`/`Having`/
`Prewhere` called on a compound builder apply to the first member's core only — not the whole
result set — while `OrderBy`/`Limit`/`Offset` apply to the compound as a whole; a caller that
needs a filter applied across the whole result set (e.g. `dbx.Grid`, which rejects a compound
base query outright) must wrap the compound in a subquery first: `sqlb.From(q.As("u"))`.

## ClickHouse-specific clauses

`FeatureClickHouse` gates `Final()`, `Sample(ratio)`/`SampleRows(n)`, `ArrayJoin`/
`LeftArrayJoin`, `Prewhere`, and `Settings(map[string]any)` — each fails with `ErrUnsupported`
on a dialect without the feature. ClickHouse identifiers additionally reject `?`, `@`, `{`, `}`
and `$` followed by a digit (`ErrInvalidIdentifier`) — these would collide with ClickHouse's own
parameterized-query and settings placeholder syntax. Values are checked recursively: a map,
struct, or a statement builder passed where a value is expected fails with `ErrUnsafeValue` /
`ErrInvalidColumn` rather than being sent to the driver, since `clickhouse-go` does not bind
these safely (verified against `clickhouse-go` v2.40.3). This recursion also rejects a
`database/sql/driver.Valuer` nested inside a slice, pointer or interface argument (`ErrUnsafeValue`)
unless it implements `fmt.Stringer` — `clickhouse-go` only calls `Value()` on a `Valuer` that is
itself the top-level bound argument, so a nested one would otherwise reach the driver unconverted;
pass it unwrapped as its own top-level value, or via `In(...)`, which binds each element at top
level. A top-level `nil` pointer whose type implements `driver.Valuer` is bound as `NULL` instead
of being passed to the driver, which would otherwise panic calling a value-receiver `Value()`
method on a nil pointer. ClickHouse's `LEFT JOIN` fills
non-matching columns with each column's type default (`0`, `''`, etc.), not `NULL`, unless the
`join_use_nulls = 1` setting is used — a `sqlb`-built `LEFT JOIN` sends valid SQL either way, but
code that checks a joined column for `NULL` needs that setting set (via `.Settings(...)`) to see
one.

## SQLite identifiers

SQLite identifiers are quoted with backticks, not double quotes: SQLite treats an unrecognized
double-quoted identifier as a string literal instead of raising an error, which is exactly the
kind of silent misbehavior `sqlb` is built to avoid. `sqlb.SQLite()`'s upsert-from-select
(`InsertBuilder.FromSelect` combined with `OnConflict`) also adds a `WHERE true` to the *last*
select core when it has none — SQLite's own upsert grammar is otherwise ambiguous between the
`SELECT`'s `WHERE` and the `ON CONFLICT` clause.

## LIKE case sensitivity

On SQLite, `LIKE` is ASCII case-insensitive; on PostgreSQL, ClickHouse and Generic, `LIKE` is
case-sensitive (use `ILIKE` there, gated by `FeatureILike`). `Contains`, `HasPrefix` and
`HasSuffix` build a `LIKE` pattern with `%`/`_` in the input escaped, so untrusted search text
cannot inject its own wildcards.

## Record shapes

`sqlb.RecordColumns(t)` / `sqlb.InsertColumns(rec)` (and every place `sqlb` reads a struct —
`Insert(...).Rows(...)`, `Update(...).SetRecord(...)`) reject field shapes `db`/`field` and
`sqlx` would disagree on, rather than silently mapping the wrong column:

- an embedded pointer struct, an unexported or explicitly tagged embedded struct, or an
  ambiguous promoted field name — `ErrRecordShape`;
- the same column set more than once — `ErrDuplicateColumn`.

`dbx.Repository[T]` (see [dbx Repositories](dbx.md)) relies on these checks at
`NewRepository[T]` time, so a record type it cannot map safely fails at startup, not on the
first query.

## Dialect support

| Feature | PostgreSQL | SQLite | ClickHouse |
|---|---|---|---|
| Placeholders | `$n` | `?` | `?` |
| Identifier quoting | `"..."` | `` `...` `` | `"..."` (backslash-escaped, restricted charset) |
| `RETURNING` | yes | yes | no |
| `ON CONFLICT` upsert | yes | yes | no |
| `UPDATE` statement | yes | yes | no |
| Transactions (via `dbx`) | yes | yes | no |
| `ILIKE` | yes | no (use case-insensitive `LIKE`) | yes |
| `UNION` keyword | `UNION` | `UNION` | `UNION DISTINCT` |
| `FINAL`/`SAMPLE`/`ARRAY JOIN`/`PREWHERE`/`SETTINGS` | no | no | yes |
| Bound-argument limit | 65535 | 32766 | none |

## Limitations

- `Raw`'s SQL text, `Fn`'s function name and `Cast`'s type string are trusted input: never build
  them from request data — nothing downstream escapes them.
- ClickHouse has no `UPDATE`, `RETURNING`, `ON CONFLICT` upsert, or transactions; `dbx.WithTx`
  and `Repository.Update`/`InsertReturning`/`Upsert` are therefore unusable against a
  ClickHouse-backed `Querier`.
- ClickHouse's lightweight `DELETE` does not work on Distributed tables or tables with
  projections — a `sqlb`-built `DELETE` still sends valid SQL, but whether ClickHouse accepts it
  depends on the table engine.
- A bound value alone in a PostgreSQL select list may need `Cast(...)` to give it a concrete
  type (PostgreSQL infers `text` for an untyped bound parameter in some positions).
