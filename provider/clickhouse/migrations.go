package clickhouse

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/oddbit-project/blueprint/db"
	"github.com/oddbit-project/blueprint/db/migrations"
	"github.com/oddbit-project/blueprint/utils"
	"slices"
)

const (
	MigrationTable = "db_migration"
	sqlCreateTable = `CREATE TABLE IF NOT EXISTS %s%s (created DateTime, module String, name String, sha2 String, contents String) ENGINE = %s`

	migrationEngine = "TinyLog"
	// sqlClusterEngine keeps one log for the whole cluster: the Keeper path names the
	// cluster and database but no shard or table UUID, so a node that creates the table
	// later joins the existing log; replica names combine shard and replica to stay
	// unique across shards
	sqlClusterPath   = "/clickhouse/blueprint/%s/%s/%s"
	sqlClusterEngine = "ReplicatedMergeTree('" + sqlClusterPath + "', '{shard}_{replica}') ORDER BY (module, name)"
)

// ErrMigrationTableNotReplicated is returned in cluster mode when the migration table on
// the connected node is not the replicated cluster log (a node-local or pre-module table,
// or one replicated under another Keeper path);
// docs/db/migrations.md describes the manual conversion
const ErrMigrationTableNotReplicated = utils.Error("clickhouse: migration table is not the replicated cluster log")

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

// onCluster returns the ON CLUSTER clause for DDL, or an empty string without a cluster
func (b *chMigrationManager) onCluster() string {
	if b.cluster == "" {
		return ""
	}
	return " ON CLUSTER `" + b.cluster + "`"
}

// createTable creates the migration table under the given name
func (b *chMigrationManager) createTable(ctx context.Context, database, name string) error {
	engine := migrationEngine
	if b.cluster != "" {
		engine = fmt.Sprintf(sqlClusterEngine, b.cluster, database, MigrationTable)
	}
	return b.client.Conn.Exec(ctx, fmt.Sprintf(sqlCreateTable, name, b.onCluster(), engine))
}

// isClusterLog reports whether the migration table on the connected node is a replica of
// the shared cluster log, at the Keeper path createTable gives it
func (b *chMigrationManager) isClusterLog(ctx context.Context, database string) (bool, error) {
	var path string
	qry := "SELECT zookeeper_path FROM system.replicas WHERE database = ? AND table = ?"
	err := b.client.Conn.QueryRow(ctx, qry, database, MigrationTable).Scan(&path)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("migration table replica lookup: %w", err)
	}
	return path == fmt.Sprintf(sqlClusterPath, b.cluster, database, MigrationTable), nil
}

// syncLog waits until the connected node's copy of the replicated log has every row
// registered through other nodes; a no-op without a cluster
func (b *chMigrationManager) syncLog(ctx context.Context) error {
	if b.cluster == "" {
		return nil
	}
	if err := b.client.Conn.Exec(ctx, "SYSTEM SYNC REPLICA "+MigrationTable); err != nil {
		return fmt.Errorf("migration table sync: %w", err)
	}
	return nil
}

// checkClusterTable verifies that an existing migration table is the shared cluster log;
// any other table (node-local, pre-module, or replicated elsewhere) must be converted by
// hand, since converting it here could race with other processes and lose rows
func (b *chMigrationManager) checkClusterTable(ctx context.Context, currentDb string) error {
	ok, err := b.isClusterLog(ctx, currentDb)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("%w: %s.%s", ErrMigrationTableNotReplicated, currentDb, MigrationTable)
	}
	return nil
}

// updateTable updates migration table to latest version
func (b *chMigrationManager) updateTable(ctx context.Context, currentDb string) error {
	// check if module column exists
	// old blueprint versions did not implement the module column
	exists, err := ColumnExists(ctx, b.client, currentDb, MigrationTable, "module")
	if err != nil {
		return err
	}
	if !exists {
		// create new table
		newTable := fmt.Sprintf("%s_new", MigrationTable)
		if err := b.createTable(ctx, currentDb, newTable); err != nil {
			return err
		}

		// copy from old to new
		// rows predating the module column belong to the base module; an empty
		// module would be invisible to List(), and every migration would re-run
		qry := "INSERT INTO %s (created, module, name, sha2, contents) SELECT created, '%s', name, sha2, contents FROM %s;"
		qry = fmt.Sprintf(qry, newTable, migrations.ModuleBase, MigrationTable)
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
		return b.createTable(ctx, currentDb, MigrationTable)
	}

	if b.cluster != "" {
		return b.checkClusterTable(ctx, currentDb)
	}
	// table exists, perform update if necessary
	return b.updateTable(ctx, currentDb)
}

// registerMigration internal function to register a migration
func (b *chMigrationManager) registerMigration(ctx context.Context, m *migrations.MigrationRecord) error {
	m.Module = b.module
	return b.repo.Insert(m)
}

func (b *chMigrationManager) List(ctx context.Context) ([]migrations.MigrationRecord, error) {
	result := make([]migrations.MigrationRecord, 0)
	if err := b.syncLog(ctx); err != nil {
		return result, err
	}
	return result, b.repo.FetchWhere(db.FV{"module": b.module}, &result)
}

func (b *chMigrationManager) MigrationExists(ctx context.Context, name string, sha2 string) (bool, error) {
	if err := b.syncLog(ctx); err != nil {
		return false, err
	}
	result := &migrations.MigrationRecord{}
	// FetchRecord, not FetchWhere: the target is a single record, and the lookup
	// is by name only, so that a migration recorded under the same name with a
	// different hash is reported as a mismatch instead of looking absent
	err := b.repo.FetchRecord(db.FV{"module": b.module, "name": name}, result)
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
	// the migration itself succeeded; a failed registration is the documented
	// ErrRegisterMigration case, and must be told apart from a failed migration
	if err := b.registerMigration(ctx, m); err != nil {
		return fmt.Errorf("%w: %w", migrations.ErrRegisterMigration, err)
	}
	return nil
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
	if err != nil {
		return err
	}
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
