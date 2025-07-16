package clickhouse

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/oddbit-project/blueprint/db"
	"github.com/oddbit-project/blueprint/db/migrations"
	"slices"
)

const (
	MigrationTable = "db_migration"
	sqlCreateTable = `CREATE TABLE IF NOT EXISTS  %s %s(created DateTime, module String, name String, sha2 String, contents String) ENGINE = TinyLog`
)

type chMigrationManager struct {
	client  *Client
	module  string
	repo    Repository
	cluster string
}

type ChMigrationOption func(s *chMigrationManager) error

func WithModule(module string) ChMigrationOption {
	return func(s *chMigrationManager) error {
		s.module = module
		return nil
	}
}

func WithCluster(cluster string) ChMigrationOption {
	return func(s *chMigrationManager) error {
		s.cluster = cluster
		return nil
	}
}

func NewMigrationManager(ctx context.Context, client *Client, opts ...ChMigrationOption) (migrations.Manager, error) {
	result := &chMigrationManager{
		client:  client,
		module:  migrations.ModuleBase,
		repo:    NewRepository(ctx, client.Conn, MigrationTable),
		cluster: "",
	}
	for _, opt := range opts {
		if err := opt(result); err != nil {
			return nil, err
		}
	}

	if err := result.init(ctx); err != nil {
		return nil, err
	}
	return result, nil
}

// updateTable updates migration table to latest version
func (b *chMigrationManager) updateTable(ctx context.Context) error {
	currentDb, err := CurrentDatabase(ctx, b.client)
	if err != nil {
		return err
	}
	// check if module column exists
	// old blueprint versions did not implement the module column
	exists, err := ColumnExists(ctx, b.client, currentDb, MigrationTable, "module")
	if err != nil {
		return err
	}
	if !exists {
		cluster := b.cluster
		// create new table
		if cluster != "" {
			cluster = fmt.Sprintf("ON CLUSTER %s.", cluster)
		}
		newTable := fmt.Sprintf("%s_new", MigrationTable)
		qry := fmt.Sprintf(sqlCreateTable, newTable, cluster)
		if err := b.client.Conn.Exec(ctx, qry); err != nil {
			return err
		}

		// copy from old to new
		qry = "INSERT INTO %s (created, module, name, sha2, contents) SELECT created, '', name, sha2, contents FROM %s;"
		qry = fmt.Sprintf(qry, newTable, MigrationTable)
		if err := b.client.Conn.Exec(ctx, qry); err != nil {
			return err
		}

		// drop old
		qry = fmt.Sprintf("DROP TABLE %s;", MigrationTable)
		if err := b.client.Conn.Exec(ctx, qry); err != nil {
			return err
		}
		// rename new
		qry = fmt.Sprintf("RENAME TABLE %s TO %s;", newTable, MigrationTable)
		if err := b.client.Conn.Exec(ctx, qry); err != nil {
			return err
		}
	}
	return nil
}

// init creates the migration table, if it doesnt exist
// Note: the migration table is created on the current database!
func (b *chMigrationManager) init(ctx context.Context) error {
	currentDb, err := CurrentDatabase(ctx, b.client)
	if err != nil {
		return err
	}
	exists, err := TableExists(ctx, b.client, currentDb, MigrationTable)
	if err != nil {
		return err
	}
	if !exists {
		cluster := b.cluster
		if cluster != "" {
			cluster = fmt.Sprintf("ON CLUSTER %s.", cluster)
		}
		qry := fmt.Sprintf(sqlCreateTable, MigrationTable, cluster)
		return b.client.Conn.Exec(ctx, qry)
	}

	// table exists, perform update if necessary
	return b.updateTable(ctx)
}

// registerMigration internal function to register a migration
func (b *chMigrationManager) registerMigration(ctx context.Context, m *migrations.MigrationRecord) error {
	m.Module = b.module
	return b.repo.Insert(m)
}

func (b *chMigrationManager) List(ctx context.Context) ([]migrations.MigrationRecord, error) {
	result := make([]migrations.MigrationRecord, 0)
	return result, b.repo.FetchWhere(db.FV{"module": b.module}, &result)
}

func (b *chMigrationManager) MigrationExists(ctx context.Context, name string, sha2 string) (bool, error) {
	result := &migrations.MigrationRecord{}
	err := b.repo.FetchWhere(db.FV{"module": b.module, "name": name, "sha2": sha2}, result)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}

	if result.SHA2 != sha2 {
		return true, migrations.ErrMigrationNameHashMismatch
	}
	return true, nil
}

// runMigration internal function to execute migrations, called by RunMigration() and Run()
func (b *chMigrationManager) runMigration(ctx context.Context, m *migrations.MigrationRecord) error {
	// execute migration
	if err := b.client.Conn.Exec(ctx, m.Contents); err != nil {
		return err
	}
	// register migration
	return b.registerMigration(ctx, m)
}

// RunMigration applies and registers a single migration
func (b *chMigrationManager) RunMigration(ctx context.Context, m *migrations.MigrationRecord) error {
	exists, err := b.MigrationExists(ctx, m.Name, m.SHA2)
	if err != nil {
		return err
	}
	if exists {
		return migrations.ErrMigrationExists
	}

	return b.runMigration(ctx, m)
}

// RegisterMigration registers a single migration but does not apply the contents
func (b *chMigrationManager) RegisterMigration(ctx context.Context, m *migrations.MigrationRecord) error {
	exists, err := b.MigrationExists(ctx, m.Name, m.SHA2)
	if err != nil {
		return err
	}
	if exists {
		return migrations.ErrMigrationExists
	}
	return b.registerMigration(ctx, m)
}

// Run all migrations from a source, and skip the ones already applied
// Example:
//
//	mm := NewMigrationManager(client)
//	if err := mm.Run(context.Background(), diskSrc, DefaultProgressFn); err != nil {
//	   panic(err)
//	}
func (b *chMigrationManager) Run(ctx context.Context, src migrations.Source, consoleFn migrations.ProgressFn) error {
	if consoleFn == nil {
		consoleFn = migrations.DefaultProgressFn
	}

	files, err := src.List()
	if err != nil {
		return err
	}

	migList, err := b.List(ctx)
	prevNames := make([]string, len(migList))
	for i, r := range migList {
		prevNames[i] = r.Name
	}

	for _, f := range files {
		if !slices.Contains(prevNames, f) {
			// read migration
			record, err := src.Read(f)
			if err != nil {
				consoleFn(migrations.MsgError, f, err)
				return err
			}
			// execute migration
			consoleFn(migrations.MsgRunMigration, f, nil)
			err = b.runMigration(ctx, record)
			if err != nil {
				consoleFn(migrations.MsgError, f, err)
				return err
			}
			consoleFn(migrations.MsgFinishedMigration, f, nil)
		} else {
			// already processed, skipping
			// we're ignoring different contents
			consoleFn(migrations.MsgSkipMigration, f, nil)
		}
	}
	return nil
}
