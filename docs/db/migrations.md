# Database Migrations

The migrations package provides a comprehensive database schema migration system with progress tracking, multiple source types, and robust error handling. It supports both file-based and embedded migrations with SHA2-based change detection.

## Overview

The migration system includes:

- Interface-based migration sources (disk, embedded, memory)
- Migration execution with progress callbacks
- SHA2-based change detection and validation
- Rollback protection through tracking
- Flexible migration record management
- Provider-agnostic implementation

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

### MigrationRecord

```go
type MigrationRecord struct {
    Created  time.Time `db:"created" ch:"created"`
    Name     string    `db:"name" ch:"name"`
    SHA2     string    `db:"sha2" ch:"sha2"`
    Contents string    `db:"contents" ch:"contents"`
}
```

Represents a migration with metadata:

- **Created**: When the migration was executed
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
    source.AddMigration("001_create_users", `
        CREATE TABLE users (
            id SERIAL PRIMARY KEY,
            name VARCHAR(100) NOT NULL,
            email VARCHAR(100) UNIQUE NOT NULL,
            created_at TIMESTAMP DEFAULT NOW()
        );
    `)
    
    source.AddMigration("002_add_index", `
        CREATE INDEX idx_users_email ON users(email);
    `)
    
    return manager.Run(context.Background(), source, migrations.DefaultProgressFn)
}
```

## Provider Integration

### PostgreSQL Migrations

```go
package main

import (
    "context"
    "github.com/oddbit-project/blueprint/provider/pgsql"
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
    manager := pgsql.NewMigrationManager(client)
    
    // Setup migration source
    source := migrations.NewDiskSource("./migrations")
    
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
    manager := clickhouse.NewMigrationManager(client)
    
    // Setup embedded migrations
    source := migrations.NewEmbedSource(migrationFiles, "migrations")
    
    // Run migrations
    return manager.Run(context.Background(), source, migrations.DefaultProgressFn)
}
```

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
            fmt.Printf("⏳ Running: %s\n", migrationName)
        case migrations.MsgFinishedMigration:
            fmt.Printf("✅ Completed: %s\n", migrationName)
        case migrations.MsgSkipMigration:
            fmt.Printf("⏭️  Skipped: %s (already executed)\n", migrationName)
        case migrations.MsgError:
            fmt.Printf("❌ Error in %s: %v\n", migrationName, err)
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
        
        // Check if migration exists with different content
        exists, err := manager.MigrationExists(ctx, migration.Name, migration.SHA2)
        if err != nil {
            return fmt.Errorf("failed to check migration %s: %w", name, err)
        }
        
        if exists {
            log.Printf("Migration %s already executed", name)
        } else {
            // Check if migration name exists with different hash
            executed, err := manager.List(ctx)
            if err != nil {
                return err
            }
            
            for _, exec := range executed {
                if exec.Name == migration.Name && exec.SHA2 != migration.SHA2 {
                    return fmt.Errorf("migration %s exists but content has changed", name)
                }
            }
            
            log.Printf("Migration %s is pending", name)
        }
    }
    
    return nil
}
```

## Advanced Usage Patterns

### Transaction-based Migrations

```go
func runMigrationsInTransaction(manager migrations.Manager, source migrations.Source) error {
    ctx := context.Background()
    
    // Start transaction (implementation depends on your manager)
    tx, err := beginMigrationTransaction(manager)
    if err != nil {
        return err
    }
    defer tx.Rollback()
    
    // Run migrations in transaction
    err = manager.Run(ctx, source, func(msgType int, name string, err error) {
        if msgType == migrations.MsgError {
            log.Printf("Migration failed, rolling back: %v", err)
        }
    })
    
    if err != nil {
        return err
    }
    
    return tx.Commit()
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

### Migration Rollback Tracking

```go
func trackRollbacks(manager migrations.Manager) error {
    ctx := context.Background()
    
    // Get all executed migrations
    migrations, err := manager.List(ctx)
    if err != nil {
        return err
    }
    
    // Sort by execution order (most recent first)
    sort.Slice(migrations, func(i, j int) bool {
        return migrations[i].Created.After(migrations[j].Created)
    })
    
    log.Println("Migration history (newest first):")
    for _, m := range migrations {
        log.Printf("- %s (executed: %s, hash: %s)", 
            m.Name, 
            m.Created.Format("2006-01-02 15:04:05"), 
            m.SHA2[:8])
    }
    
    return nil
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
            case errors.Is(err, migrations.ErrMigrationExists):
                log.Printf("Migration %s already exists", migrationName)
            case errors.Is(err, migrations.ErrMigrationNameHashMismatch):
                log.Printf("Migration %s content has changed", migrationName)
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

## Testing Migrations

### Unit Testing

```go
func TestMigrations(t *testing.T) {
    // Create in-memory database for testing
    db := setupTestDatabase(t)
    defer db.Close()
    
    // Create test manager
    manager := createTestManager(db)
    
    // Create memory source with test migrations
    source := migrations.NewMemorySource()
    source.AddMigration("001_test", "CREATE TABLE test_table (id INT);")
    source.AddMigration("002_test", "ALTER TABLE test_table ADD COLUMN name VARCHAR(50);")
    
    // Run migrations
    err := manager.Run(context.Background(), source, nil)
    assert.NoError(t, err)
    
    // Verify migrations were executed
    executed, err := manager.List(context.Background())
    assert.NoError(t, err)
    assert.Len(t, executed, 2)
    
    // Verify table structure
    verifyTableExists(t, db, "test_table")
    verifyColumnExists(t, db, "test_table", "name")
}
```

### Integration Testing

```go
func TestMigrationIntegration(t *testing.T) {
    // Setup real database connection
    config := pgsql.NewClientConfig()
    config.DSN = os.Getenv("TEST_DATABASE_URL")
    
    client, err := pgsql.NewClient(config)
    require.NoError(t, err)
    defer client.Disconnect()
    
    // Create clean database state
    setupCleanDatabase(t, client)
    
    // Create manager
    manager := pgsql.NewMigrationManager(client)
    
    // Test disk-based migrations
    source := migrations.NewDiskSource("./testdata/migrations")
    
    // Run migrations
    err = manager.Run(context.Background(), source, nil)
    assert.NoError(t, err)
    
    // Verify final state
    verifyDatabaseSchema(t, client)
}
```

## Best Practices

### Migration Design
1. **One Change Per Migration**: Keep migrations focused on single changes
2. **Reversible Operations**: Design migrations that can be rolled back
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
    source.AddMigration("large_migration", `
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