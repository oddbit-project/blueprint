package pgsql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jmoiron/sqlx"
	"github.com/oddbit-project/blueprint/db"
	"github.com/oddbit-project/blueprint/db/migrations"
	"slices"
)

const (
	// engine constants
	EngineSchema         = "public"
	MigrationTable       = "db_migration"
	EngineMigrationTable = EngineSchema + "." + MigrationTable

	MigrationLockId = 2343452349
)

type pgMigrationManager struct {
	db *sqlx.DB
}

func NewMigrationManager(ctx context.Context, client *db.SqlClient) (migrations.Manager, error) {
	result := &pgMigrationManager{
		db: client.Db(),
	}
	if err := result.init(ctx); err != nil {
		return nil, err
	}
	return result, nil
}

// init checks if migration table exists, and if not, creates
func (b *pgMigrationManager) init(ctx context.Context) error {
	exists, err := TableExists(ctx, b.db, MigrationTable, SchemaDefault)
	if err != nil {
		return err
	}

	if exists {
		return nil
	}
	qry := fmt.Sprintf(`CREATE TABLE  %s (
			created TIMESTAMP WITH TIME ZONE,
			name TEXT,
			sha2 TEXT,
			contents TEXT)`,
		EngineMigrationTable)
	_, err = b.db.ExecContext(ctx, qry)
	return err
}

// registerMigration internal function to register a migration
func (b *pgMigrationManager) registerMigration(ctx context.Context, m *migrations.MigrationRecord) error {
	qry := fmt.Sprintf("INSERT INTO %s (created, name, sha2, contents) VALUES ($1, $2, $3, $4)", EngineMigrationTable)
	tx, err := b.db.Begin()
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, qry, m.Created, m.Name, m.SHA2, m.Contents)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (b *pgMigrationManager) List(ctx context.Context) ([]*migrations.MigrationRecord, error) {
	result := make([]*migrations.MigrationRecord, 0)
	qry := fmt.Sprintf("SELECT * FROM %s ORDER BY created", EngineMigrationTable)
	if err := b.db.SelectContext(ctx, &result, qry); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return result, nil
		}
		return nil, err
	}
	return result, nil

}

func (b *pgMigrationManager) MigrationExists(ctx context.Context, name string, sha2 string) (bool, error) {
	result := &migrations.MigrationRecord{}
	qry := fmt.Sprintf("SELECT * FROM %s WHERE name=$1 LIMIT 1", EngineMigrationTable)

	if err := b.db.SelectContext(ctx, qry, name); err != nil {
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
	tx, err := b.db.Begin()
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
	lock, err := NewAdvisoryLock(ctx, b.db, MigrationLockId)
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
func (b *pgMigrationManager) RegisterMigration(ctx context.Context, m *migrations.MigrationRecord) error {
	lock, err := NewAdvisoryLock(ctx, b.db, MigrationLockId)
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
//	mm := NewMigrationManager(db)
//	if err := mm.Run(context.Background(), diskSrc, DefaultProgressFn); err != nil {
//	   panic(err)
//	}
func (b *pgMigrationManager) Run(ctx context.Context, src migrations.Source, consoleFn migrations.ProgressFn) error {
	if consoleFn == nil {
		consoleFn = migrations.DefaultProgressFn
	}

	lock, err := NewAdvisoryLock(ctx, b.db, MigrationLockId)
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
