package pgsql

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
	EngineSchema         = "public"
	MigrationTable       = "db_migration"
	EngineMigrationTable = EngineSchema + "." + MigrationTable

	MigrationLockId = 2343452349
)

type pgMigrationManager struct {
	client *db.SqlClient
	module string
	repo   db.Repository
}

type PgMigrationOption func(s *pgMigrationManager) error

func WithModule(module string) PgMigrationOption {
	return func(s *pgMigrationManager) error {
		s.module = module
		return nil
	}
}

func NewMigrationManager(ctx context.Context, client *db.SqlClient, opts ...PgMigrationOption) (migrations.Manager, error) {
	result := &pgMigrationManager{
		client: client,
		module: migrations.ModuleBase,
		repo:   db.NewRepository(ctx, client, EngineMigrationTable),
	}

	for _, opt := range opts {
		err := opt(result)
		if err != nil {
			return nil, err
		}
	}

	if err := result.init(ctx); err != nil {
		return nil, err
	}
	return result, nil
}

// updateTable updates migration table to latest version
func (b *pgMigrationManager) updateTable(ctx context.Context) error {
	exists, err := ColumnExists(ctx, b.client, MigrationTable, "module", SchemaDefault)
	if err != nil {
		return err
	}
	if !exists {
		// add column "module"
		// old blueprint versions did not implement the module column
		qry := fmt.Sprintf(`ALTER TABLE  %s ADD COLUMN module TEXT DEFAULT %s`, EngineMigrationTable, migrations.ModuleBase)
		result := b.client.Db().QueryRowContext(ctx, qry)
		return result.Err()
	}
	return nil
}

// init checks if migration table exists, and if not, creates
func (b *pgMigrationManager) init(ctx context.Context) error {
	exists, err := TableExists(ctx, b.client, MigrationTable, SchemaDefault)
	if err != nil {
		return err
	}

	if exists {
		// table already exists, migrate to new version if necessary
		return b.updateTable(ctx)
	}

	qry := fmt.Sprintf(`CREATE TABLE  %s (
			created TIMESTAMP WITH TIME ZONE,
			module TEXT,
			name TEXT,
			sha2 TEXT,
			contents TEXT)`,
		EngineMigrationTable)
	_, err = b.client.Db().ExecContext(ctx, qry)
	return err
}

// registerMigration internal function to register a migration
func (b *pgMigrationManager) registerMigration(ctx context.Context, m *migrations.MigrationRecord) error {
	m.Module = b.module
	return b.repo.Insert(m)
}

func (b *pgMigrationManager) List(ctx context.Context) ([]migrations.MigrationRecord, error) {
	result := make([]migrations.MigrationRecord, 0)
	return result, b.repo.FetchWhere(db.FV{"module": b.module}, &result)
}

func (b *pgMigrationManager) MigrationExists(ctx context.Context, name string, sha2 string) (bool, error) {
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
func (b *pgMigrationManager) runMigration(ctx context.Context, m *migrations.MigrationRecord) error {
	tx, err := b.client.Db().Begin()
	if err != nil {
		return err
	}

	// execute migration
	if _, err := tx.ExecContext(ctx, m.Contents); err != nil {
		_ = tx.Rollback()
		return err
	}

	if err = tx.Commit(); err != nil {
		return err
	}

	// register migration
	return b.registerMigration(ctx, m)
}

// RunMigration applies and registers a single migration
func (b *pgMigrationManager) RunMigration(ctx context.Context, m *migrations.MigrationRecord) error {
	lock, err := NewAdvisoryLock(ctx, b.client.Db(), MigrationLockId)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err := lock.Lock(ctx); err != nil {
		return err
	}
	defer lock.Unlock(ctx)

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
// Note: the field Module is ignored; instead, it uses the module defined in pgMigrationManager
func (b *pgMigrationManager) RegisterMigration(ctx context.Context, m *migrations.MigrationRecord) error {
	lock, err := NewAdvisoryLock(ctx, b.client.Db(), MigrationLockId)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err := lock.Lock(context.Background()); err != nil {
		return err
	}
	defer lock.Unlock(context.Background())

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
func (b *pgMigrationManager) Run(ctx context.Context, src migrations.Source, consoleFn migrations.ProgressFn) error {
	if consoleFn == nil {
		consoleFn = migrations.DefaultProgressFn
	}

	lock, err := NewAdvisoryLock(ctx, b.client.Db(), MigrationLockId)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err := lock.Lock(context.Background()); err != nil {
		return err
	}
	defer lock.Unlock(context.Background())

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
