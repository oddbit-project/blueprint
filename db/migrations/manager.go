package migrations

import (
	"fmt"
	"github.com/oddbit-project/blueprint/console"
	"github.com/oddbit-project/blueprint/utils"
	"time"
)

const (
	ConsoleSkipMigration   = "Skipping migration (already processed)"
	ConsoleStartMigration  = "Executing migration"
	ConsoleFailedMigration = "Failed migration"
	ConsoleFinishMigration = "Finished migration"

	ErrMigrationNameHashMismatch = utils.Error("Migration name or hash exists but they mismatch. Migration file was edited or renamed?")
	ErrMigrationExists           = utils.Error("Migration already executed")
	ErrRegisterMigration         = utils.Error("Migration executed successfully, but registration failed. Register manually")
)

type MigrationProgress func(message, name string)
type ErrorHandler func(err error)

type MigrationRecord struct {
	Created  time.Time `db:"created"`
	Name     string    `db:"name"`
	SHA2     string    `db:"sha2"`
	Contents string    `db:"contents"`
}

type Backend interface {
	Initialize() error
	List() ([]*MigrationRecord, error)
	RunMigration(m *MigrationRecord) error
	RegisterMigration(m *MigrationRecord) error
}

type Manager struct {
	backend Backend
	source  Source
	console MigrationProgress
	error   ErrorHandler
}

func NewManager(src Source, b Backend) (*Manager, error) {
	return &Manager{
		backend: b,
		source:  src,
		console: _consoleHandler,
		error:   _errorHandler,
	}, b.Initialize()
}

func _consoleHandler(message, name string) {
	msg := fmt.Sprintf("%s;  Migration: %s", message, name)
	switch message {
	case ConsoleFinishMigration, ConsoleStartMigration:
		msg = console.Regular(msg)
		break
	case ConsoleFailedMigration:
		msg = console.Error(msg)
		break
	case ConsoleSkipMigration:
		msg = console.Info(msg)
		break
	}
	fmt.Println(msg)
}

func _errorHandler(err error) {
}
