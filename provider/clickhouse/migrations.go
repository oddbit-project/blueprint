package clickhouse

import (
	"context"
	"fmt"
	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/oddbit-project/blueprint/db/migrations"
	"slices"
)

const (
	MigrationTable = "db_migration"
	sqlCreateTable = `CREATE TABLE IF NOT EXISTS  %s %s(created DateTime, name String, sha2 String, contents String) ENGINE = TinyLog`
)

type chMigrationManager struct {
	db clickhouse.Conn
}

func NewMigrationManager(ctx context.Context, client *Client, cluster string) (migrations.Manager, error) {
	result := &chMigrationManager{
		db: client.Conn,
	}
	if err := result.init(ctx, cluster); err != nil {
		return nil, err
	}
	return result, nil
}

// init creates the migration table, if it doesnt exist
// Note: the migration table is created on the current database!
func (b *chMigrationManager) init(ctx context.Context, cluster string) error {
	if cluster != "" {
		cluster = fmt.Sprintf("ON CLUSTER %s.", cluster)
	}
	qry := fmt.Sprintf(sqlCreateTable, MigrationTable, cluster)
	return b.db.Exec(ctx, qry)
}

// registerMigration internal function to register a migration
func (b *chMigrationManager) registerMigration(ctx context.Context, m *migrations.MigrationRecord) error {
	batch, err := b.db.PrepareBatch(ctx, fmt.Sprintf("INSERT INTO %s", MigrationTable))
	if err != nil {
		return err
	}
	if err = batch.AppendStruct(m); err != nil {
		batch.Abort()
		return err
	}
	return batch.Send()
}

func (b *chMigrationManager) List(ctx context.Context) ([]migrations.MigrationRecord, error) {
	result := make([]migrations.MigrationRecord, 0)
	qry := fmt.Sprintf("SELECT * FROM %s ORDER BY created", MigrationTable)
	if err := b.db.Select(ctx, &result, qry); err != nil {
		return nil, err
	}
	return result, nil

}

func (b *chMigrationManager) MigrationExists(ctx context.Context, name string, sha2 string) (bool, error) {
	result := &migrations.MigrationRecord{}
	qry := fmt.Sprintf("SELECT * FROM %s WHERE name=$1 LIMIT 1", MigrationTable)
	if err := b.db.Select(ctx, qry, name); err != nil {
		return false, err
	}
	if result.SHA2 != sha2 {
		return true, migrations.ErrMigrationNameHashMismatch
	}
	return len(result.Name) > 0, nil
}

// runMigration internal function to execute migrations, called by RunMigration() and Run()
func (b *chMigrationManager) runMigration(ctx context.Context, m *migrations.MigrationRecord) error {
	// execute migration
	if err := b.db.Exec(ctx, m.Contents); err != nil {
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
//	mm := NewMigrationManager(db)
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
