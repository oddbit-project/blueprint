// Package dbx provides typed generic repositories on top of gohan, with a
// database/sql adapter and transaction helper.
//
// Unlike db.Repository, dbx.Repository[T] returns *T / []*T from every read,
// takes a context.Context per call instead of storing one at construction,
// and builds every statement through gohan: values are always bound and
// identifiers are always escaped.
//
// # Not-found semantics
//
// ErrNotFound is an alias for sql.ErrNoRows, so errors.Is(err,
// sql.ErrNoRows) keeps working against code written against database/sql or
// db.Repository. Get returns (nil, ErrNotFound) when no row matches;
// QueryInt64 (used by Count/Exists internally) also returns ErrNotFound on
// no row, though Count/Exists themselves never surface it (a COUNT(*) query
// always returns exactly one row).
//
// # No unfiltered DELETE/UPDATE
//
// Delete, Update and UpdateFields require a non-nil where expression and
// return gohan.ErrNoWhere without touching the database when where is nil.
// There is deliberately no "delete all" or "update all" method: a caller who
// means it uses Exec with an explicit gohan.Delete(table).All() (or
// gohan.Update(table).All()) statement.
//
// # Caller obligations (nothing here can enforce these)
//
//   - Inside WithTx, use repo.With(tx) to run through the transaction —
//     using the outer repository (bound to the non-transactional Querier)
//     runs the statement outside the transaction, and on SQLite with
//     MaxOpenConns == 1 this deadlocks waiting for the only connection,
//     which WithTx is already holding.
//   - Custom queries passed to Get/List should start from r.Select(),
//     not a bare gohan.From(table): gohan.From renders "SELECT *", which sqlx
//     rejects when the table has columns T does not map.
//   - sqlx's default field-name mapper is assumed throughout; a repository
//     bound to a *sqlx.DB with a custom mapper is not supported.
package dbx
