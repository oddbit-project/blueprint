package db

import (
	"context"
	"github.com/doug-martin/goqu/v9"
	"sync"
)

type FnRepositoryFactory func(ctx context.Context, conn *SqlClient, tableName string) Repository

var (
	// Repository factory registry
	mx                sync.Mutex
	repositoryFactory = make(map[string]FnRepositoryFactory)
)

func NewRepository(ctx context.Context, conn *SqlClient, tableName string) Repository {
	mx.Lock()
	defer mx.Unlock()

	if factory, ok := repositoryFactory[conn.DriverName]; ok {
		return factory(ctx, conn, tableName)
	}

	return &repository{
		conn:      conn.Db(),
		ctx:       ctx,
		tableName: tableName,
		dialect:   goqu.Dialect(conn.DriverName),
	}
}

// RegisterFactory registers a repository factory for the given driver name
func RegisterFactory(driverName string, factory FnRepositoryFactory) {
	mx.Lock()
	defer mx.Unlock()
	if _, dup := repositoryFactory[driverName]; dup {
		panic("db.RegisterFactory() called twice for driver " + driverName)
	}
	repositoryFactory[driverName] = factory
}
