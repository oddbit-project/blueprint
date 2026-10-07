# Database Migrations

The migrations package provides a simple database schema migration system with progress tracking and error handling. 
It supports both file-based and embedded migrations with SHA2-based change detection.

> Note: the migration manager is forward-only; to revert the operations of a given migration, a new migration must be
> created. Rollback of schema changes is a destructive operation, and by itself a mutation on the current database state;
> as such, the concept of "rolling back schema changes" is deeply flawed and may result in data loss.

## Overview

The migration system includes:

- Interface-based migration sources (disk, embedded, memory)
- Module support
- Migration execution with progress callbacks
- SHA2-based change detection and validation
- Rollback protection through tracking
- Flexible migration record management
- Provider-agnostic implementation

## Limitations

- The ClickHouse implementation supports only one statement per file, due to ClickHouse limitations;


## Core Interfaces

### Manager Interface

```go
type Manager interface {
    List(ctx context.Context) ([]MigrationRecord, error)
    MigrationExists(ctx context.Context, name string, sha2 string) (bool, error)
    RunMigration(ctx context.Context, m *MigrationRecord) error
    RegisterMigration(ctx context.Context, m *MigrationRecord) error
    Run(ctx context.Context, src Source, consoleFn ProgressFn) error
}
```

The Manager interface handles migration execution and tracking:

- **List()**: Returns all executed migrations
- **MigrationExists()**: Checks if a migration has been executed
- **RunMigration()**: Executes a single migration
- **RegisterMigration()**: Records a migration as executed
- **Run()**: Executes all pending migrations from a source

### Source Interface

```go
type Source interface {
    List() ([]string, error)
    Read(name string) (*MigrationRecord, error)
}
```

The Source interface abstracts migration storage:

- **List()**: Returns available migration names
- **Read()**: Reads a specific migration

`Substitute(src Source, vars Vars) Source` wraps any of them to fill
deployment-specific identifiers into the DDL as it is read; see
[Substituted Source](#substituted-source).

### MigrationRecord

```go
type MigrationRecord struct {
    Created  time.Time `db:"created" ch:"created"`
    Module   string    `db:"module" ch:"module"`
	Name     string    `db:"name" ch:"name"`
    SHA2     string    `db:"sha2" ch:"sha2"`
    Contents string    `db:"contents" ch:"contents"`
}
```

Represents a migration with metadata:

- **Created**: When the migration was executed
- **Module**: The module name (defaults to base)
- **Name**: Migration identifier
- **SHA2**: Content hash for change detection
- **Contents**: The actual migration SQL

## Source Implementations

### Disk Source

Reads migrations from filesystem directories:

```go
package main

import (
    "context"
    "github.com/oddbit-project/blueprint/db/migrations"
    "log"
)

func runDiskMigrations(manager migrations.Manager) error {
    // Create disk source pointing to migrations directory
    source := migrations.NewDiskSource("./migrations")
    
    // Run all pending migrations
    return manager.Run(context.Background(), source, migrations.DefaultProgressFn)
}
```

**Directory Structure:**
```
migrations/
├── 001_create_users.sql
├── 002_add_email_index.sql
├── 003_create_orders.sql
└── 004_add_foreign_keys.sql
```

### Embedded Source

Uses Go's embed package for compiled-in migrations:

```go
package main

import (
    "context"
    "embed"
    "github.com/oddbit-project/blueprint/db/migrations"
    "log"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

func runEmbeddedMigrations(manager migrations.Manager) error {
    // Create embedded source from embedded filesystem
    source := migrations.NewEmbedSource(migrationFiles, "migrations")
    
    // Run all pending migrations
    return manager.Run(context.Background(), source, migrations.DefaultProgressFn)
}
```

### Memory Source

In-memory migrations for testing or dynamic generation:

```go
func runMemoryMigrations(manager migrations.Manager) error {
    source := migrations.NewMemorySource()
    
    // Add migrations programmatically
    source.Add("001_create_users.sql", `
        CREATE TABLE users (
            id SERIAL PRIMARY KEY,
            name VARCHAR(100) NOT NULL,
            email VARCHAR(100) UNIQUE NOT NULL,
            created_at TIMESTAMP DEFAULT NOW()
        );
    `)
    
    source.Add("002_add_index.sql", `
        CREATE INDEX idx_users_email ON users(email);
    `)
    
    return manager.Run(context.Background(), source, migrations.DefaultProgressFn)
}
```

### Substituted Source

Migrations are fixed text, but some identifiers are the deployment's: the role
an application connects as, a schema, a tablespace. `Substitute` wraps any
source and replaces `${name}` as each migration is read.

```go
func runMigrations(manager migrations.Manager, fsys embed.FS, appRole string) error {
    source, err := migrations.NewEmbedSource(fsys, "migrations")
    if err != nil {
        return err
    }
    // the DDL says: REVOKE DELETE ON audit_log FROM ${appRole};
    source = migrations.Substitute(source, migrations.Vars{
        "appRole": appRole,
    })
    return manager.Run(context.Background(), source, migrations.DefaultProgressFn)
}
```

**The recorded hash is the template's, not the substituted text's.** The
contents stored in the migration table are what actually ran, but `SHA2` covers
the file as shipped, so a deployment that renames its application role does not
make its applied migrations look edited. The consequence, worth knowing when
auditing an installation: re-hashing a stored `contents` will not reproduce its
`sha2` for any migration that carried a placeholder.

**Changing a value does not re-run a migration that already ran.** Migrations
are skipped by name, so a new value for `${appRole}` reaches only the migrations
that have yet to run; giving an existing object to a renamed role needs a new
migration.

**A placeholder with no value is an error** (`ErrMissingVar`), not an empty
string, and so is a placeholder whose value is empty. Every `${...}` in the file
must be supplied -- including one that is misspelled, or written in a form the
substitution does not otherwise recognise (`${app-role}`, `${1}`) -- because an
unresolved name would otherwise reach the server as literal text, usually inside
a `GRANT` or an owner clause, and fail in a way that names nothing useful.

**Values are spliced in verbatim**, with no quoting or escaping, so a value must
be a trusted identifier that the deployment owns. Substitution is not a way to
pass data to a statement, and a value must never carry a secret: the substituted
text is stored in the migration table, which is readable by anything that can
read the schema, and travels into every backup and replica.

**The braces are required**, so a `$` not followed by `{` is never a
placeholder: positional parameters (`$1`) and the delimiters of dollar-quoted
bodies (`$$ ... $$`, `$body$ ... $body$`) pass through untouched. Substitution
is textual and does not track quoting, so a `${...}` written *inside* a
dollar-quoted body or a string literal is substituted like any other.

```go
// unchanged by substitution
DO $body$ BEGIN
    IF to_regclass('old_name') IS NOT NULL THEN
        EXECUTE 'DELETE FROM thing WHERE id = $1';
    END IF;
END $body$;
```

Substitution is textual and has no logic -- there is no way to express a
condition or a loop -- so the SQL that runs is the SQL that was shipped with its
identifiers filled in, and nothing else.

## Provider Integration

### PostgreSQL Migrations

```go
package main

import (
    "context"
    "github.com/oddbit-project/blueprint/provider/pgsql"
	"github.com/oddbit-project/blueprint/db/migrations"
    "log"
)

func runPostgreSQLMigrations() error {
    // Setup PostgreSQL client
    config := pgsql.NewClientConfig()
    config.DSN = "postgres://user:pass@localhost/dbname?sslmode=disable"
    
    client, err := pgsql.NewClient(config)
    if err != nil {
        return err
    }
    defer client.Disconnect()
    
    // Create migration manager
    manager, err := pgsql.NewMigrationManager(context.Background(), client)
    if err != nil {
		return err
    }
	
    // Setup migration source
    source, err := migrations.NewDiskSource("./migrations")
	if err != nil {
		return err
	}
	
    // Run migrations with progress reporting
    return manager.Run(context.Background(), source, func(msgType int, migrationName string, err error) {
        switch msgType {
        case migrations.MsgRunMigration:
            log.Printf("Running migration: %s", migrationName)
        case migrations.MsgFinishedMigration:
            log.Printf("Completed migration: %s", migrationName)
        case migrations.MsgSkipMigration:
            log.Printf("Skipping migration (already run): %s", migrationName)
        case migrations.MsgError:
            log.Printf("Migration error in %s: %v", migrationName, err)
        }
    })
}
```

### ClickHouse Migrations

```go
package main

import (
    "context"
    "github.com/oddbit-project/blueprint/provider/clickhouse"
	"github.com/oddbit-project/blueprint/db/migrations"	
    "log"
)

func runClickHouseMigrations() error {
    // Setup ClickHouse client
    config := clickhouse.NewClientConfig()
    config.DSN = "clickhouse://localhost:9000/default"
    
    client, err := clickhouse.NewClient(config)
    if err != nil {
        return err
    }
    defer client.Disconnect()
    
    // Create migration manager
    manager, err := clickhouse.NewMigrationManager(context.Background(), client)
	if err != nil {
		return err
	}

	// Setup migration source
	source, err := migrations.NewDiskSource("./migrations")
	if err != nil {
		return err
	}
    
    // Run migrations
    return manager.Run(context.Background(), source, migrations.DefaultProgressFn)
}
```

#### Repairing a table upgraded before this fix

Every ClickHouse provider up to v0.8.2 copied the rows of a pre-module migration
table with an empty `module`, and `List()` matches on the module, so those rows
are invisible and every historical migration runs again. Upgrades from now on
write `base`, but an installation already upgraded by an older version has to be
repaired by hand -- the table engine (`TinyLog`) supports no updates, so the
table is rewritten:

```sql
CREATE TABLE db_migration_fixed (created DateTime, module String, name String, sha2 String, contents String) ENGINE = TinyLog;
INSERT INTO db_migration_fixed SELECT created, if(module = '', 'base', module), name, sha2, contents FROM db_migration;
DROP TABLE db_migration;
RENAME TABLE db_migration_fixed TO db_migration;
```

Check for the condition with
`SELECT count() FROM db_migration WHERE module = ''` before and after.

#### Cluster mode

`clickhouse.WithCluster("<cluster>")` keeps the migration log in one replicated table shared by
every node of the cluster, across shards, so any node can run the migrations, and a process
that connects through a load balancer or fails over to another node sees the same log:

```go
manager, err := clickhouse.NewMigrationManager(ctx, client, clickhouse.WithCluster("c1"))
```

When no log exists on the connected node, the manager creates it on every node:

```sql
CREATE TABLE IF NOT EXISTS db_migration ON CLUSTER `c1` (...)
ENGINE = ReplicatedMergeTree('/clickhouse/blueprint/c1/<database>/db_migration', '{shard}_{replica}')
ORDER BY (module, name)
```

Requirements:

- Self-managed ClickHouse; cluster mode has not been tested on ClickHouse Cloud.
- ClickHouse Keeper (or ZooKeeper) and distributed DDL configured, as for any `ON CLUSTER` DDL.
- The `shard` and `replica` macros defined on every node; `{shard}_{replica}` must be unique
  across the cluster.
- A database that does not use the `Replicated` database engine. It rejects the explicit
  Keeper path unless `database_replicated_allow_replicated_engine_arguments` is enabled.
- The connecting user may run `SYSTEM SYNC REPLICA`: `List()` and `MigrationExists()` first wait
  until the connected node has every row registered through other nodes. If that cannot happen
  (Keeper unavailable, or the only node holding a row is down), they wait or fail rather than
  read a log that may be incomplete.
- Every node reachable whenever a manager starts on a node that has no log yet, since the log
  is then created `ON CLUSTER`. If a node is down or rejects the DDL, `NewMigrationManager`
  returns the distributed DDL error after `distributed_ddl_task_timeout`, though the log may
  already exist on the reachable nodes; the next start succeeds.
- Read access to `system.replicas`, where the manager checks that the log is the shared one.
- Plain cluster and database names: they are written into the Keeper path, so quotes, `/` or
  `{...}` macros in them break it.
- Every manager on the database, whatever its `WithModule`, using `WithCluster` with the same
  cluster. A manager without it reads the shared log without waiting for replication, and one
  that starts first on a fresh node creates a node-local log that the others then reject.

The Keeper path is fixed per cluster name and database, so a node added to the cluster later
joins the existing log the first time a manager connects through it. Two deployments that
share a Keeper ensemble must not use the same cluster name and database. A node that is
rebuilt with the same macros still has its old replica registered in Keeper; remove it with
`SYSTEM DROP REPLICA '<shard>_<replica>' FROM ZKPATH '/clickhouse/blueprint/<cluster>/<database>/db_migration'`
before the log is created on it again.

The option affects only the migration log. Migrations that create tables on every node must
say `ON CLUSTER` themselves. Do not run migrations through two nodes at the same moment.

##### Moving an existing log to cluster mode

A manager with `WithCluster` refuses to start on a node whose `db_migration` table is not the
shared cluster log, returning `clickhouse.ErrMigrationTableNotReplicated` and leaving the table
untouched. This covers a `TinyLog` created without the option (including a pre-module one) and a
replicated table under another Keeper path. Convert each such node by hand before enabling the
option, one node at a time. The steps run on that node only, so they work whether or not other
nodes already have the shared log:

```sql
-- joins the shared log if another node already created it, and starts it otherwise
CREATE TABLE db_migration_shared (created DateTime, module String, name String, sha2 String, contents String)
ENGINE = ReplicatedMergeTree('/clickhouse/blueprint/<cluster>/<database>/db_migration', '{shard}_{replica}')
ORDER BY (module, name);

SYSTEM SYNC REPLICA db_migration_shared;

INSERT INTO db_migration_shared (created, module, name, sha2, contents)
SELECT created, if(module = '', 'base', module), name, sha2, contents FROM db_migration
WHERE (if(module = '', 'base', module), name) NOT IN (SELECT module, name FROM db_migration_shared);

EXCHANGE TABLES db_migration AND db_migration_shared;
DROP TABLE db_migration_shared SYNC;
```

For a pre-module table (no `module` column), use `'base'` in place of both `if(...)`
expressions. `db_migration` stays in place until the atomic `EXCHANGE`, so a manager starting
meanwhile still sees the old log. Avoid running migrations on the node while it is being
converted. If the `CREATE` fails with `REPLICA_ALREADY_EXISTS`, the node's replica is still
registered in Keeper from an earlier attempt; remove it with `SYSTEM DROP REPLICA` as above.
Rows other nodes have already recorded are not copied twice.

Convert every node that has an old log before the first start in cluster mode. A node without
a log simply joins the shared one, so rows recorded only in another node's unconverted log
would not be seen, and those migrations would run again.

## Migration Workflow

### Basic Migration Execution

```go
func executeMigrations(manager migrations.Manager, source migrations.Source) error {
    ctx := context.Background()
    
    // Get list of available migrations
    available, err := source.List()
    if err != nil {
        return fmt.Errorf("failed to list migrations: %w", err)
    }
    
    log.Printf("Found %d migrations", len(available))
    
    // Get list of executed migrations
    executed, err := manager.List(ctx)
    if err != nil {
        return fmt.Errorf("failed to list executed migrations: %w", err)
    }
    
    log.Printf("Found %d executed migrations", len(executed))
    
    // Run pending migrations
    return manager.Run(ctx, source, migrations.DefaultProgressFn)
}
```

### Custom Progress Tracking

```go
func customProgressTracking(manager migrations.Manager, source migrations.Source) error {
    progressFn := func(msgType int, migrationName string, err error) {
        switch msgType {
        case migrations.MsgRunMigration:
            fmt.Printf("Running: %s\n", migrationName)
        case migrations.MsgFinishedMigration:
            fmt.Printf("Completed: %s\n", migrationName)
        case migrations.MsgSkipMigration:
            fmt.Printf(" Skipped: %s (already executed)\n", migrationName)
        case migrations.MsgError:
            fmt.Printf("Error in %s: %v\n", migrationName, err)
        }
    }
    
    return manager.Run(context.Background(), source, progressFn)
}
```

### Validation and Safety Checks

```go
func validateMigrations(manager migrations.Manager, source migrations.Source) error {
    ctx := context.Background()
    
    // Get available migrations
    available, err := source.List()
    if err != nil {
        return err
    }
    
    // Validate each migration
    for _, name := range available {
        migration, err := source.Read(name)
        if err != nil {
            return fmt.Errorf("failed to read migration %s: %w", name, err)
        }
        
        // Check if migration exists, and whether its contents still match
        exists, err := manager.MigrationExists(ctx, migration.Name, migration.SHA2)
        switch {
        case errors.Is(err, migrations.ErrMigrationNameHashMismatch):
            return fmt.Errorf("migration %s exists but content has changed", name)
        case err != nil:
            return fmt.Errorf("failed to check migration %s: %w", name, err)
        case exists:
            log.Printf("Migration %s already executed", name)
        default:
            log.Printf("Migration %s is pending", name)
        }
    }
    
    return nil
}
```

### Conditional Migrations

```go
type ConditionalSource struct {
    source    migrations.Source
    condition func(string) bool
}

func (cs *ConditionalSource) List() ([]string, error) {
    all, err := cs.source.List()
    if err != nil {
        return nil, err
    }
    
    var filtered []string
    for _, name := range all {
        if cs.condition(name) {
            filtered = append(filtered, name)
        }
    }
    
    return filtered, nil
}

func (cs *ConditionalSource) Read(name string) (*migrations.MigrationRecord, error) {
    if !cs.condition(name) {
        return nil, fmt.Errorf("migration %s not allowed", name)
    }
    
    return cs.source.Read(name)
}

func runConditionalMigrations(manager migrations.Manager, source migrations.Source) error {
    // Only run migrations matching pattern
    conditionalSource := &ConditionalSource{
        source: source,
        condition: func(name string) bool {
            return strings.HasPrefix(name, "prod_")
        },
    }
    
    return manager.Run(context.Background(), conditionalSource, migrations.DefaultProgressFn)
}
```

## Error Handling

### Migration Errors

```go
func handleMigrationErrors(manager migrations.Manager, source migrations.Source) error {
    ctx := context.Background()
    
    progressFn := func(msgType int, migrationName string, err error) {
        switch msgType {
        case migrations.MsgError:
            // Log detailed error information
            log.Printf("Migration %s failed: %v", migrationName, err)
            
            // Check specific error types
            switch {
            case errors.Is(err, migrations.ErrMissingVar):
                log.Printf("Migration %s has a placeholder nothing supplied", migrationName)
            case errors.Is(err, migrations.ErrRegisterMigration):
                log.Printf("Migration %s executed but registration failed", migrationName)
            default:
                log.Printf("Unexpected error in migration %s", migrationName)
            }
        }
    }
    
    err := manager.Run(ctx, source, progressFn)
    if err != nil {
        return fmt.Errorf("migration execution failed: %w", err)
    }
    
    return nil
}
```

`Run()` skips migrations it has already applied, by name, so a progress function
sees only the errors of migrations it actually tries: `ErrMissingVar`, a failure
of the SQL itself, and `ErrRegisterMigration` (the migration ran, recording it
did not). The single-migration API reports the other two:

```go
switch err := manager.RunMigration(ctx, record); {
case errors.Is(err, migrations.ErrMigrationExists):
    // same name, same contents: already applied
case errors.Is(err, migrations.ErrMigrationNameHashMismatch):
    // same name, different contents: the file was edited or renamed
case err != nil:
    return err
}
```


### Recovery and Cleanup

```go
func recoverFromFailedMigration(manager migrations.Manager, migrationName string) error {
    ctx := context.Background()
    
    // Check if migration was partially executed
    migrations, err := manager.List(ctx)
    if err != nil {
        return err
    }
    
    for _, m := range migrations {
        if m.Name == migrationName {
            log.Printf("Migration %s found in database, checking consistency", migrationName)
            
            // Verify migration content matches
            source := migrations.NewDiskSource("./migrations")
            current, err := source.Read(migrationName)
            if err != nil {
                return err
            }
            
            if m.SHA2 != current.SHA2 {
                return fmt.Errorf("migration %s content mismatch: database=%s, file=%s", 
                    migrationName, m.SHA2[:8], current.SHA2[:8])
            }
            
            log.Printf("Migration %s is consistent", migrationName)
            return nil
        }
    }
    
    log.Printf("Migration %s not found in database, may need manual cleanup", migrationName)
    return nil
}
```

## Best Practices

### Migration Design
1. **One Change Per Migration**: Keep migrations focused on single changes
3. **Data Safety**: Include data migration strategies for schema changes
4. **Testing**: Test migrations against representative data

### File Organization
1. **Naming Convention**: Use sequential numbering (001_, 002_, etc.)
2. **Descriptive Names**: Include clear descriptions in filenames
3. **Directory Structure**: Organize by environment or module if needed
4. **Version Control**: Track migrations in version control

### Execution Strategy
1. **Backup First**: Always backup before running migrations
2. **Test Environment**: Run migrations in staging before production
3. **Monitoring**: Monitor migration execution and performance
4. **Rollback Plan**: Have rollback procedures ready

### Error Handling
1. **Fail Fast**: Stop on first error to prevent inconsistent state
2. **Logging**: Log all migration activities for debugging
3. **Validation**: Validate migration state before and after execution
4. **Recovery**: Have procedures for recovering from failed migrations

## Performance Considerations

### Large Migrations
```go
func runLargeMigration(manager migrations.Manager) error {
    // For large data migrations, consider batching
    source := migrations.NewMemorySource()
    source.Add("large_migration.sql", `
        -- Process in batches to avoid long locks
        UPDATE users SET status = 'active' 
        WHERE id BETWEEN 1 AND 10000;
        
        -- Add index concurrently (PostgreSQL)
        CREATE INDEX CONCURRENTLY idx_users_status ON users(status);
    `)
    
    return manager.Run(context.Background(), source, func(msgType int, name string, err error) {
        if msgType == migrations.MsgRunMigration {
            log.Printf("Starting large migration %s - this may take a while", name)
        }
    })
}
```

### Migration Optimization
1. **Batch Processing**: Process large datasets in batches
2. **Index Management**: Create indexes concurrently when possible
3. **Lock Minimization**: Avoid long-running locks on production tables
4. **Resource Monitoring**: Monitor CPU, memory, and disk usage

## See Also

- [PostgreSQL Provider](../provider/pgsql.md)
- [ClickHouse Provider](../provider/clickhouse.md)
- [Database Package Overview](index.md)
- [Client Documentation](client.md)