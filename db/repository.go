package db

import (
	"context"
)

type Executor interface {
	Exec(ctx context.Context, sql string, arguments ...any) (any, error)
}

type RowSet interface {
	Scan(dest ...any) error
	Next() bool
	Err() error
}

type Querier interface {
	Query(ctx context.Context, sql string, arguments ...any) (RowSet, error)
}
