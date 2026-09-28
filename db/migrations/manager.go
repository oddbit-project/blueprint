package migrations

import (
	"context"
	"fmt"
	"github.com/oddbit-project/blueprint/console"
	"github.com/oddbit-project/blueprint/utils"
	"time"
)

const (
	ModuleBase = "base"

	MsgSkipMigration = iota + 1
	MsgRunMigration
	MsgFinishedMigration
	MsgError

	ErrMigrationNameHashMismatch = utils.Error("Migration name or hash exists but they mismatch. Migration file was edited or renamed?")
	ErrMigrationExists           = utils.Error("Migration already executed")
	ErrRegisterMigration         = utils.Error("Migration executed successfully, but registration failed. Register manually")
)

type ProgressFn func(msgType int, migrationName string, e error)

type MigrationRecord struct {
	Created  time.Time `db:"created" ch:"created"`
	Module   string    `db:"module" ch:"module"`
	Name     string    `db:"name" ch:"name"`
	SHA2     string    `db:"sha2" ch:"sha2"`
	Contents string    `db:"contents" ch:"contents"`
}

type Source interface {
	List() ([]string, error)
	Read(name string) (*MigrationRecord, error)
}

type Manager interface {
	List(ctx context.Context) ([]MigrationRecord, error)
	MigrationExists(ctx context.Context, name string, sha2 string) (bool, error)
	RunMigration(ctx context.Context, m *MigrationRecord) error
	RegisterMigration(ctx context.Context, m *MigrationRecord) error
	Run(ctx context.Context, src Source, consoleFn ProgressFn) error
}

func DefaultProgressFn(msgType int, migrationName string, e error) {
	var msg string
	switch msgType {
	case MsgRunMigration:
		msg = console.Regular("%s", fmt.Sprintf("Running migration '%s'...", migrationName))
	case MsgFinishedMigration:
		msg = console.Regular("%s", fmt.Sprintf("Migration '%s' finished successfully", migrationName))
	case MsgSkipMigration:
		msg = console.Info("%s", fmt.Sprintf("Migration '%s' already run, skipping", migrationName))

	case MsgError:
		msgE := "-"
		if e != nil {
			msgE = e.Error()
		}
		msg = console.Error("%s", fmt.Sprintf("Error executing migration '%s': %s", migrationName, msgE))

	default:
		msg = console.Regular("%s", fmt.Sprintf("Running migration '%s'...", migrationName))
	}
	fmt.Println(msg)
}
