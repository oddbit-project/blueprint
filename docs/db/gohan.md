# gohan Query Builder

The SQL query builder Blueprint's `dbx` is built on is the standalone module
[`github.com/oddbit-project/gohan`](https://github.com/oddbit-project/gohan) — values are always
bound and identifiers are always quoted and escaped by construction, so a caller cannot
reintroduce SQL injection through the normal API. It replaces the pattern of building SQL with
goqu and inlined values (the class of bug fixed in the first release after Blueprint `v0.10.3`) —
see [Migrating to dbx](migrating-to-dbx.md) for a full mapping.

> Three escape hatches exist for trusted input only, and must never be built from request data:
> `Raw`'s SQL text, `Fn`'s function name, and `Cast`'s type string.

## How Blueprint uses it

`dbx` (see [dbx Repositories](dbx.md)) builds every statement it runs through `gohan`:
`Select`, `Insert`, `Update`, `Delete` builders, each immutable, and three built-in dialects
(`gohan.Postgres()`, `gohan.SQLite()`, `gohan.ClickHouse()`, plus `gohan.Generic()` for ANSI
SQL — never register it for MySQL, where a double-quoted string is a literal, not an
identifier). `Build(dialect) (string, []any, error)` renders a statement; nothing runs a
query — `gohan` has no I/O.

Full usage (SELECT/INSERT/UPDATE/DELETE examples, the string-position rule, `Raw`, `Int`,
`UNION`, ClickHouse-specific clauses, SQLite identifiers, `LIKE` case sensitivity, record
shapes and the dialect support table) is documented in
[gohan's README](https://github.com/oddbit-project/gohan#readme).

## Reserved-type registry

`dbx`'s struct-metadata reading goes through `github.com/oddbit-project/gohan/field`, **not**
Blueprint's own `db/field` package — the two keep separate reserved-type registries:

- `github.com/oddbit-project/gohan/field.AddReservedType(name)` affects `dbx` (and anything else
  that calls into `gohan`'s `field` package).
- Blueprint's `db/field.AddReservedType(name)` affects only the legacy `db` package.

Calling one does not register the type with the other, and `errors.Is` against
`db/field.ErrInvalidStruct` does not match an equivalent error from `dbx`/`gohan`. See
[Struct Tags and Reserved Types](structs-and-tags.md#reserved-types) for the full picture.

## Dialect support

See [gohan's dialect support table](https://github.com/oddbit-project/gohan#dialect-support) for
what each dialect (PostgreSQL, SQLite, ClickHouse) supports — placeholders, identifier quoting,
`RETURNING`, `ON CONFLICT`, transactions, `ILIKE`, `UNION`, ClickHouse-only clauses, and each
dialect's bound-argument limit.
