package db

import (
	"context"
	"github.com/doug-martin/goqu/v9"
	"github.com/oddbit-project/blueprint/db/qb"
	"sync"
)

type FnRepositoryFactory func(ctx context.Context, conn *SqlClient, tableName string) Repository

var (
	// Repository factory registry
	rmx               sync.Mutex
	repositoryFactory = make(map[string]FnRepositoryFactory)
	dialectMap        = sync.Map{}
	dmx               sync.RWMutex
)

func NewRepository(ctx context.Context, conn *SqlClient, tableName string) Repository {
	rmx.Lock()
	defer rmx.Unlock()

	if factory, ok := repositoryFactory[conn.DriverName]; ok {
		return factory(ctx, conn, tableName)
	}

	return &repository{
		conn:       conn.Db(),
		ctx:        ctx,
		tableName:  tableName,
		dialect:    goqu.Dialect(conn.DriverName),
		sqlBuilder: qb.NewSqlBuilder(GetDialect(conn.DriverName)),
	}
}

// RegisterFactory registers a repository factory for the given driver name
func RegisterFactory(driverName string, factory FnRepositoryFactory) {
	rmx.Lock()
	defer rmx.Unlock()
	if _, dup := repositoryFactory[driverName]; dup {
		panic("db.RegisterFactory() called twice for driver " + driverName)
	}
	repositoryFactory[driverName] = factory
}

func RegisterDialect(driverName string, dialect qb.SqlDialect) {
	dmx.Lock()
	defer dmx.Unlock()
	dialectMap.Store(driverName, dialect)
}

func GetDialect(driverName string) qb.SqlDialect {
	dmx.RLock()
	defer dmx.RUnlock()
	if dialect, ok := dialectMap.Load(driverName); ok {
		return dialect.(qb.SqlDialect)
	}
	return qb.DefaultSqlDialect()
}
