package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"

	"github.com/oddbit-project/blueprint/db"
	"github.com/oddbit-project/blueprint/db/migrations"
)

const (
	MigrationTable = "db_migration"
)

type sqliteMigrationManager struct {
	client *db.SqlClient
	module string
	repo   db.Repository
}

type SqliteMigrationOption func(s *sqliteMigrationManager) error

func WithModule(module string) SqliteMigrationOption {
	return func(s *sqliteMigrationManager) error {
		s.module = module
		return nil
	}
}

func NewMigrationManager(ctx context.Context, client *db.SqlClient, opts ...SqliteMigrationOption) (migrations.Manager, error) {
	result := &sqliteMigrationManager{
		client: client,
		module: migrations.ModuleBase,
		repo:   db.NewRepository(ctx, client, MigrationTable),
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
func (b *sqliteMigrationManager) updateTable(ctx context.Context) error {
	exists, err := ColumnExists(ctx, b.client, MigrationTable, "module")
	if err != nil {
		return err
	}
	if !exists {
		qry := fmt.Sprintf(`ALTER TABLE %s ADD COLUMN module TEXT`, MigrationTable)
		if _, err = b.client.Db().ExecContext(ctx, qry); err != nil {
			return err
		}

		qry = fmt.Sprintf(`UPDATE %s SET module = ?`, MigrationTable)
		if _, err = b.client.Db().ExecContext(ctx, qry, migrations.ModuleBase); err != nil {
			return err
		}
	}
	return nil
}

// init checks if migration table exists, and if not, creates
func (b *sqliteMigrationManager) init(ctx context.Context) error {
	exists, err := TableExists(ctx, b.client, MigrationTable)
	if err != nil {
		return err
	}

	if exists {
		return b.updateTable(ctx)
	}

	qry := fmt.Sprintf(`CREATE TABLE %s (
			created DATETIME,
			module TEXT,
			name TEXT,
			sha2 TEXT,
			contents TEXT)`,
		MigrationTable)
	_, err = b.client.Db().ExecContext(ctx, qry)
	return err
}

// registerMigration internal function to register a migration
func (b *sqliteMigrationManager) registerMigration(ctx context.Context, m *migrations.MigrationRecord) error {
	m.Module = b.module
	return b.repo.Insert(m)
}

func (b *sqliteMigrationManager) List(ctx context.Context) ([]migrations.MigrationRecord, error) {
	result := make([]migrations.MigrationRecord, 0)
	return result, b.repo.FetchWhere(db.FV{"module": b.module}, &result)
}

func (b *sqliteMigrationManager) MigrationExists(ctx context.Context, name string, sha2 string) (bool, error) {
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

// runMigration internal function to execute migrations
func (b *sqliteMigrationManager) runMigration(ctx context.Context, m *migrations.MigrationRecord) error {
	tx, err := b.client.Db().Begin()
	if err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, m.Contents); err != nil {
		_ = tx.Rollback()
		return err
	}

	if err = tx.Commit(); err != nil {
		return err
	}

	return b.registerMigration(ctx, m)
}

// RunMigration applies and registers a single migration
func (b *sqliteMigrationManager) RunMigration(ctx context.Context, m *migrations.MigrationRecord) error {
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
// Note: the field Module is ignored; instead, it uses the module defined in sqliteMigrationManager
func (b *sqliteMigrationManager) RegisterMigration(ctx context.Context, m *migrations.MigrationRecord) error {
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
func (b *sqliteMigrationManager) Run(ctx context.Context, src migrations.Source, consoleFn migrations.ProgressFn) error {
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
			record, err := src.Read(f)
			if err != nil {
				consoleFn(migrations.MsgError, f, err)
				return err
			}
			consoleFn(migrations.MsgRunMigration, f, nil)
			err = b.runMigration(ctx, record)
			if err != nil {
				consoleFn(migrations.MsgError, f, err)
				return err
			}
			consoleFn(migrations.MsgFinishedMigration, f, nil)
		} else {
			consoleFn(migrations.MsgSkipMigration, f, nil)
		}
	}
	return nil
}
