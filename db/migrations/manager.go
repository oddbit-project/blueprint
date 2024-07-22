package migrations

import (
	"context"
	"fmt"
	"github.com/oddbit-project/blueprint/console"
	"github.com/oddbit-project/blueprint/utils"
	"time"
)

const (
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
	Created  time.Time `db:"created"`
	Name     string    `db:"name"`
	SHA2     string    `db:"sha2"`
	Contents string    `db:"contents"`
}

type Source interface {
	List() ([]string, error)
	Read(name string) (*MigrationRecord, error)
}

type Manager interface {
	List(ctx context.Context) ([]*MigrationRecord, error)
	MigrationExists(ctx context.Context, name string, sha2 string) (bool, error)
	RunMigration(ctx context.Context, m *MigrationRecord) error
	RegisterMigration(ctx context.Context, m *MigrationRecord) error
	Run(ctx context.Context, src Source, consoleFn ProgressFn) error
}

func DefaultProgressFn(msgType int, migrationName string, e error) {
	var msg string
	switch msgType {
	case MsgRunMigration:
		msg = console.Regular(fmt.Sprintf("Running migration '%s'...", migrationName))
		break
	case MsgFinishedMigration:
		msg = console.Regular(fmt.Sprintf("Migration '%s' finished successfully", migrationName))
		break
	case MsgSkipMigration:
		msg = console.Info(fmt.Sprintf("Migration '%s' already run, skipping", migrationName))
		break

	case MsgError:
		msgE := "-"
		if e != nil {
			msgE = e.Error()
		}
		msg = console.Error(fmt.Sprintf("Error executing migration '%s': %s", migrationName, msgE))
		break

	default:
		msg = console.Regular(fmt.Sprintf("Running migration '%s'...", migrationName))
		break
	}
	fmt.Println(msg)
}
