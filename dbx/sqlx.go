package dbx

import (
	"context"
	"database/sql"

	"github.com/jmoiron/sqlx"

	"github.com/oddbit-project/blueprint/db"
	"github.com/oddbit-project/blueprint/sqlb"
)

// SQLQuerier implements Querier and TxBeginner over a *sqlx.DB.
type SQLQuerier struct {
	db *sqlx.DB
	d  sqlb.Dialect
}

var (
	_ Querier    = (*SQLQuerier)(nil)
	_ TxBeginner = (*SQLQuerier)(nil)
)

// NewSQL returns a Querier/TxBeginner over conn, building statements for
// dialect d.
func NewSQL(conn *sqlx.DB, d sqlb.Dialect) *SQLQuerier {
	return &SQLQuerier{db: conn, d: d}
}

// FromClient builds a Querier/TxBeginner from c. The dialect is resolved
// from c.DriverName via sqlb.DialectFor before connecting, so an unknown
// driver fails with sqlb.ErrUnknownDialect without a wasted connection
// attempt; a ClickHouse dialect fails with ErrDialectDriver (use
// provider/clickhouse's Querier instead). If c is not yet connected,
// FromClient connects it. FromClient snapshots c.Conn: call it once at
// startup, not after a later Disconnect/Connect cycle.
func FromClient(c *db.SqlClient) (*SQLQuerier, error) {
	d, err := sqlb.DialectFor(c.DriverName)
	if err != nil {
		return nil, err
	}
	if d.Name() == "clickhouse" {
		return nil, ErrDialectDriver
	}
	if !c.IsConnected() {
		if err := c.Connect(); err != nil {
			return nil, err
		}
	}
	return NewSQL(c.Conn, d), nil
}

func (q *SQLQuerier) Dialect() sqlb.Dialect { return q.d }

func (q *SQLQuerier) Exec(ctx context.Context, query string, args ...any) (int64, error) {
	return execRowsAffected(ctx, q.db, query, args...)
}

func (q *SQLQuerier) Get(ctx context.Context, dest any, query string, args ...any) error {
	return q.db.GetContext(ctx, dest, query, args...)
}

func (q *SQLQuerier) Select(ctx context.Context, dest any, query string, args ...any) error {
	return q.db.SelectContext(ctx, dest, query, args...)
}

func (q *SQLQuerier) QueryInt64(ctx context.Context, query string, args ...any) (int64, error) {
	return queryInt64(ctx, q.db, query, args...)
}

func (q *SQLQuerier) BeginTx(ctx context.Context, opts *sql.TxOptions) (TxQuerier, error) {
	tx, err := q.db.BeginTxx(ctx, opts)
	if err != nil {
		return nil, err
	}
	return &sqlTx{tx: tx, d: q.d}, nil
}

// sqlxExecer is the subset of *sqlx.DB / *sqlx.Tx that execRowsAffected
// needs.
type sqlxExecer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// sqlxRowxQueryer is the subset of *sqlx.DB / *sqlx.Tx that queryInt64
// needs.
type sqlxRowxQueryer interface {
	QueryRowxContext(ctx context.Context, query string, args ...any) *sqlx.Row
}

func execRowsAffected(ctx context.Context, e sqlxExecer, query string, args ...any) (int64, error) {
	res, err := e.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// queryInt64 scans the first column of the first row of query, returning
// ErrNotFound (sql.ErrNoRows) unchanged when the query matches no row.
func queryInt64(ctx context.Context, r sqlxRowxQueryer, query string, args ...any) (int64, error) {
	var n int64
	if err := r.QueryRowxContext(ctx, query, args...).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

// sqlTx wraps a *sqlx.Tx as a TxQuerier. It does not implement TxBeginner:
// WithTx joins an existing TxQuerier instead of nesting a BEGIN.
type sqlTx struct {
	tx *sqlx.Tx
	d  sqlb.Dialect
}

var _ TxQuerier = (*sqlTx)(nil)

func (t *sqlTx) Dialect() sqlb.Dialect { return t.d }

func (t *sqlTx) Exec(ctx context.Context, query string, args ...any) (int64, error) {
	return execRowsAffected(ctx, t.tx, query, args...)
}

func (t *sqlTx) Get(ctx context.Context, dest any, query string, args ...any) error {
	return t.tx.GetContext(ctx, dest, query, args...)
}

func (t *sqlTx) Select(ctx context.Context, dest any, query string, args ...any) error {
	return t.tx.SelectContext(ctx, dest, query, args...)
}

func (t *sqlTx) QueryInt64(ctx context.Context, query string, args ...any) (int64, error) {
	return queryInt64(ctx, t.tx, query, args...)
}

func (t *sqlTx) Commit() error   { return t.tx.Commit() }
func (t *sqlTx) Rollback() error { return t.tx.Rollback() }
