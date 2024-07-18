package pgsql

import (
	"database/sql"
	"errors"
	"fmt"
	"github.com/jmoiron/sqlx"
	"github.com/oddbit-project/blueprint/db/migrations"
)

const (
	// engine constants
	EngineSchema      = "public"
	MigrationTable    = "db_migration"
	EngineSchemaTable = EngineSchema + "." + MigrationTable
)

type pgBackend struct {
	db *sqlx.DB
}

func NewMigrationBackend(db *sqlx.DB) migrations.Backend {
	return &pgBackend{db: db}
}

func (b *pgBackend) Initialize() error {
	installed, err := b.isInstalled()
	if err != nil {
		return err
	}
	if !installed {
		return b.install()
	}
	return nil
}

func (b *pgBackend) isInstalled() (bool, error) {
	return TableExists(b.db, MigrationTable, SchemaDefault)
}

func (b *pgBackend) install() error {
	qry := fmt.Sprintf(`CREATE TABLE  %s (
			created TIMESTAMP WITH TIME ZONE,
			name TEXT,
			sha2 TEXT,
			contents TEXT)`,
		EngineSchemaTable)
	_, err := b.db.Exec(qry)
	return err
}

func (b *pgBackend) List() ([]*migrations.MigrationRecord, error) {
	result := make([]*migrations.MigrationRecord, 0)
	qry := fmt.Sprintf("SELECT * FROM %s ORDER BY created", EngineSchemaTable)
	if err := b.db.Select(result, qry); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return result, nil
		}
	}
	return result, nil
}

func (b *pgBackend) migrationExists(name string, sha2 string) (bool, error) {
	result := &migrations.MigrationRecord{}
	qry := fmt.Sprintf("SELECT * FROM %s WHERE name=$1", EngineSchemaTable)
	if err := b.db.Select(result, qry, name); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
	}
	if result.SHA2 != sha2 {
		return true, migrations.ErrMigrationNameHashMismatch
	}
	return true, nil
}

func (b *pgBackend) RunMigration(m *migrations.MigrationRecord) error {
	exists, err := b.migrationExists(m.Name, m.SHA2)
	if err != nil {
		return err
	}
	if exists {
		return migrations.ErrMigrationExists
	}

	tx, err := b.db.Begin()
	if err != nil {
		return err
	}

	if _, err := tx.Exec(m.Contents); err != nil {
		_ = tx.Rollback()
		return err
	}

	if err = tx.Commit(); err != nil {
		return err
	}

	if err := b.RegisterMigration(m); err != nil {
		// critical error, migration was run successfully, but failed to register
		// it must be registered manually
		return migrations.ErrRegisterMigration
	}
	return nil
}

func (e *pgBackend) RegisterMigration(m *migrations.MigrationRecord) error {
	exists, err := e.migrationExists(m.Name, m.SHA2)
	if err != nil {
		return err
	}
	if !exists {
		qry := fmt.Sprintf("INSERT INTO %s (created, name, sha2, contents) VALUES (:created, :name, :sha2, :contents)", EngineSchemaTable)
		tx := e.db.MustBegin()
		_, err = tx.NamedExec(qry, m)
		if err != nil {
			tx.Rollback()
			return err
		}
		return tx.Commit()
	}
	return nil
}
