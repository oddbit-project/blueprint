package pgsql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/georgysavva/scany/v2/pgxscan"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/oddbit-project/blueprint/db/migrations"
	"slices"
)

const (
	// engine constants
	EngineSchema      = "public"
	MigrationTable    = "db_migration"
	EngineSchemaTable = EngineSchema + "." + MigrationTable

	MigrationLockId = 2343452349
)

type pgMigrationManager struct {
	pool *pgxpool.Pool
}

func NewMigrationManager(pool *pgxpool.Pool) migrations.Manager {
	return &pgMigrationManager{
		pool: pool,
	}
}

// init checks if migration table exists, and if not, creates
func (b *pgMigrationManager) init(db *pgxpool.Conn, ctx context.Context) error {
	exists, err := TableExists(db, ctx, MigrationTable, SchemaDefault)
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
		EngineSchemaTable)
	_, err = db.Exec(ctx, qry)
	return err
}

// registerMigration internal function to register a migration
func (b *pgMigrationManager) registerMigration(db *pgxpool.Conn, ctx context.Context, m *migrations.MigrationRecord) error {
	qry := fmt.Sprintf("INSERT INTO %s (created, name, sha2, contents) VALUES ($1, $2, $3, $4)", EngineSchemaTable)
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, qry, m.Created, m.Name, m.SHA2, m.Contents)
	if err != nil {
		tx.Rollback(ctx)
		return err
	}
	return tx.Commit(ctx)
}

func (b *pgMigrationManager) List(ctx context.Context) ([]*migrations.MigrationRecord, error) {
	db, err := b.pool.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	defer db.Release()

	if err := b.init(db, ctx); err != nil {
		return nil, err
	}
	return b.list(db, ctx)
}

func (b *pgMigrationManager) list(db *pgxpool.Conn, ctx context.Context) ([]*migrations.MigrationRecord, error) {
	result := make([]*migrations.MigrationRecord, 0)
	qry := fmt.Sprintf("SELECT * FROM %s ORDER BY created", EngineSchemaTable)
	if err := pgxscan.Select(ctx, db, &result, qry); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return result, nil
		}
		return nil, err
	}
	return result, nil
}

func (b *pgMigrationManager) MigrationExists(ctx context.Context, name string, sha2 string) (bool, error) {
	db, err := b.pool.Acquire(ctx)
	if err != nil {
		return false, err
	}
	defer db.Release()

	if err := b.init(db, ctx); err != nil {
		return false, err
	}

	return b.migrationExists(db, ctx, name, sha2)
}

func (b *pgMigrationManager) migrationExists(db *pgxpool.Conn, ctx context.Context, name string, sha2 string) (bool, error) {
	result := &migrations.MigrationRecord{}
	qry := fmt.Sprintf("SELECT * FROM %s WHERE name=$1", EngineSchemaTable)

	if err := pgxscan.Select(ctx, db, qry, name); err != nil {
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
func (b *pgMigrationManager) runMigration(db *pgxpool.Conn, ctx context.Context, m *migrations.MigrationRecord) error {
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}

	// execute migration
	if _, err := tx.Exec(ctx, m.Contents); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}

	if err = tx.Commit(ctx); err != nil {
		return err
	}

	// register migration
	return b.registerMigration(db, ctx, m)
}

// RunMigration applies and registers a single migration
func (b *pgMigrationManager) RunMigration(ctx context.Context, m *migrations.MigrationRecord) error {
	db, err := b.pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer db.Release()

	lock := NewAdvisoryLock(db, MigrationLockId)
	if err = lock.Lock(context.Background()); err != nil {
		return err
	}
	defer lock.Unlock(context.Background())

	if err := b.init(db, ctx); err != nil {
		return err
	}

	exists, err := b.MigrationExists(ctx, m.Name, m.SHA2)
	if err != nil {
		return err
	}
	if exists {
		return migrations.ErrMigrationExists
	}

	return b.runMigration(db, ctx, m)
}

// RegisterMigration registers a single migration but does not apply the contents
func (b *pgMigrationManager) RegisterMigration(ctx context.Context, m *migrations.MigrationRecord) error {
	db, err := b.pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer db.Release()

	lock := NewAdvisoryLock(db, MigrationLockId)
	if err = lock.Lock(context.Background()); err != nil {
		return err
	}
	defer lock.Unlock(context.Background())

	exists, err := b.migrationExists(db, ctx, m.Name, m.SHA2)
	if err != nil {
		return err
	}
	if exists {
		return migrations.ErrMigrationExists
	}

	return b.registerMigration(db, ctx, m)
}

// Run all migrations from a source, and skip the ones already applied
// Example:
//
//	mm := NewMigrationManager(db)
//	if err := mm.Run(context.Background(), diskSrc, DefaultProgressFn); err != nil {
//	   panic(err)
//	}
func (b *pgMigrationManager) Run(ctx context.Context, src migrations.Source, consoleFn migrations.ProgressFn) error {
	db, err := b.pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer db.Release()

	lock := NewAdvisoryLock(db, MigrationLockId)
	if err = lock.Lock(context.Background()); err != nil {
		return err
	}
	defer lock.Unlock(context.Background())

	if err := b.init(db, ctx); err != nil {
		return err
	}

	files, err := src.List()
	if err != nil {
		return err
	}

	migList, err := b.list(db, ctx)
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
			err = b.runMigration(db, ctx, record)
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
