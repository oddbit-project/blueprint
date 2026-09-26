package dbx

import (
	"context"
	"database/sql"

	"github.com/oddbit-project/blueprint/sqlb"
)

// Querier executes sqlb-built SQL and scans results back into Go values. It
// is the sole abstraction dbx.Repository depends on, so a repository can be
// bound to a plain connection, a transaction, or (via BatchInserter) an
// adapter with a bulk-insert fast path.
type Querier interface {
	// Dialect returns the sqlb.Dialect statements are built against.
	Dialect() sqlb.Dialect
	// Exec runs query and returns the number of rows affected.
	Exec(ctx context.Context, query string, args ...any) (int64, error)
	// Get scans the first row of query into dest (a *T). It returns
	// ErrNotFound when the query matches no row.
	Get(ctx context.Context, dest any, query string, args ...any) error
	// Select scans every row of query into dest (a *[]*T).
	Select(ctx context.Context, dest any, query string, args ...any) error
	// QueryInt64 scans the first column of the first row of query. It
	// returns ErrNotFound when the query matches no row.
	QueryInt64(ctx context.Context, query string, args ...any) (int64, error)
}

// TxQuerier is a Querier already running inside a transaction.
type TxQuerier interface {
	Querier
	Commit() error
	Rollback() error
}

// TxBeginner is a Querier that can start a new transaction.
type TxBeginner interface {
	BeginTx(ctx context.Context, opts *sql.TxOptions) (TxQuerier, error)
}

// BatchInserter is implemented by adapters with a bulk-insert fast path
// (e.g. the ClickHouse adapter in plan 007). Repository.Insert prefers it
// over chunked INSERT statements when the bound Querier implements it.
type BatchInserter interface {
	// InsertBatch inserts rows (each a *T) into table in one operation.
	InsertBatch(ctx context.Context, table string, rows []any) error
}
