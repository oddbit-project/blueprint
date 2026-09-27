// Package sqlb is a SQL query builder where values are always bound and
// identifiers are always quoted and escaped by construction, so a caller
// cannot reintroduce SQL injection through the normal API.
//
// Three escape hatches exist for trusted input only, and must never be
// built from request data: Raw's sql text, Fn's function name, and Cast's
// type string.
//
// String position rule: whether a string is treated as a column name or as
// a bound value depends on where it appears.
//
//   - Column position: the argument to Count, Sum, Avg, Min, Max, select
//     lists, GROUP BY, ORDER BY, and INSERT/UPDATE column names is a
//     column name if it is a string.
//   - Value position: comparison right-hand sides, Fn arguments, Raw
//     arguments, Cast's value, Case's THEN/ELSE, and Val treat a string as
//     a bound value. Pass Col("x") when a column is meant.
//
// Generic assumes ANSI double-quoted identifiers and must not be registered
// for MySQL, where a double-quoted string is a string literal, not an
// identifier.
//
// On SQLite, LIKE is ASCII case-insensitive; on PostgreSQL, ClickHouse and
// Generic, LIKE is case-sensitive.
package sqlb
