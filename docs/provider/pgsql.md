# blueprint.provider.pgsql

Blueprint PostgreSQL client

The client uses the [pgx](https://github.com/jackc/pgx) library.

## Configuration

The PostgreSQL client uses the following configuration:

```json
{
  "pgsql": {
    "dsn": "postgres://username:password@localhost:5432/database?sslmode=allow",
    "maxOpenConns": 4,
    "maxIdleConns": 2,
    "connLifetime": 3600,
    "connIdleTime": 1800
  }
}
```

### ClientConfig

```go
type ClientConfig struct {
    DSN          string `json:"dsn"`          // PostgreSQL connection string
    MaxOpenConns int    `json:"maxOpenConns"` // Max number of pool connections (default: 4)
    MaxIdleConns int    `json:"maxIdleConns"` // Max number of idle pool connections (default: 2)
    ConnLifetime int    `json:"connLifetime"` // Duration in seconds after which connection is closed (default: 3600)
    ConnIdleTime int    `json:"connIdleTime"` // Duration in seconds for idle connection cleanup (default: 1800)
}
```

## Using the Client

```go
package main

import (
	"context"
	"github.com/oddbit-project/blueprint/provider/pgsql"
	"log"
)

func main() {
	pgConfig := pgsql.NewClientConfig()
	pgConfig.DSN = "postgres://username:password@localhost:5432/database?sslmode=allow"

	// Optionally configure connection pool
	pgConfig.MaxOpenConns = 10
	pgConfig.MaxIdleConns = 5
	pgConfig.ConnLifetime = 7200  // 2 hours
	pgConfig.ConnIdleTime = 3600  // 1 hour

	client, err := pgsql.NewClient(pgConfig)
	if err != nil {
		log.Fatal(err)
	}
	if err = client.Connect(); err != nil {
		log.Fatal(err)
	}
	defer client.Disconnect()

	// Use the client
	ctx := context.Background()
	var version string
	err = client.Db().QueryRowContext(ctx, "SELECT version()").Scan(&version)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("PostgreSQL version:", version)
}
```

## Utility Functions

### Database Object Checks

```go
// Check if a table exists
exists, err := pgsql.TableExists(ctx, client, "users", pgsql.SchemaDefault)

// Check if a view exists
exists, err := pgsql.ViewExists(ctx, client, "user_view", pgsql.SchemaDefault)

// Check if a foreign table exists
exists, err := pgsql.ForeignTableExists(ctx, client, "external_users", pgsql.SchemaDefault)

// Check if a column exists
exists, err := pgsql.ColumnExists(ctx, client, "users", "email", pgsql.SchemaDefault)

// Get PostgreSQL server version
version, err := pgsql.GetServerVersion(client.Db(), ctx)
```

### Constants

```go
const (
    SchemaDefault = "public"

    TblTypeTable        = "BASE TABLE"
    TblTypeView         = "VIEW"
    TblTypeForeignTable = "FOREIGN TABLE"
    TblTypeLocal        = "LOCAL TEMPORARY"
)
```

## Migrations

The pgsql package provides a migration system for managing database schema changes.

### Migration Manager

```go
package main

import (
	"context"
	"github.com/oddbit-project/blueprint/db/migrations"
	"github.com/oddbit-project/blueprint/provider/pgsql"
	"log"
)

func main() {
	// Create client
	pgConfig := pgsql.NewClientConfig()
	pgConfig.DSN = "postgres://username:password@localhost:5432/database?sslmode=allow"
	client, err := pgsql.NewClient(pgConfig)
	if err != nil {
		log.Fatal(err)
	}
	if err = client.Connect(); err != nil {
		log.Fatal(err)
	}
	defer client.Disconnect()

	ctx := context.Background()

	// Create migration manager
	mm, err := pgsql.NewMigrationManager(ctx, client)
	if err != nil {
		log.Fatal(err)
	}

	// Create migration source from disk
	src, err := migrations.NewDiskSource("./migrations")
	if err != nil {
		log.Fatal(err)
	}

	// Run all pending migrations
	if err := mm.Run(ctx, src, migrations.DefaultProgressFn); err != nil {
		log.Fatal(err)
	}
}
```

### Migration Sources

The migration system supports multiple sources:

#### Disk Source

```go
// Load migrations from a directory
src, err := migrations.NewDiskSource("./migrations")
```

#### Embed Source

```go
import "embed"

//go:embed migrations/*.sql
var migrationFiles embed.FS

// Load migrations from embedded files
src, err := migrations.NewEmbedSource(migrationFiles, "migrations")
```

### Migration Manager Interface

```go
type Manager interface {
    // List all applied migrations
    List(ctx context.Context) ([]MigrationRecord, error)

    // Check if a migration exists
    MigrationExists(ctx context.Context, name string, sha2 string) (bool, error)

    // Run a single migration
    RunMigration(ctx context.Context, m *MigrationRecord) error

    // Register a migration without executing it
    RegisterMigration(ctx context.Context, m *MigrationRecord) error

    // Run all pending migrations from a source
    Run(ctx context.Context, src Source, consoleFn ProgressFn) error
}
```

### Migration Modules

You can organize migrations by module:

```go
// Create migration manager for a specific module
mm, err := pgsql.NewMigrationManager(ctx, client, pgsql.WithModule("auth"))
```

### Migration File Format

Migration files should be `.sql` files with SQL statements:

```sql
-- migrations/001_create_users.sql
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    username VARCHAR(255) NOT NULL UNIQUE,
    email VARCHAR(255) NOT NULL UNIQUE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_users_email ON users(email);
```

Migration files are sorted alphabetically by filename, so use a numeric prefix for ordering.

## Advisory Locks

PostgreSQL advisory locks for coordinating concurrent access across database sessions.

### Basic Usage

```go
package main

import (
	"context"
	"github.com/oddbit-project/blueprint/provider/pgsql"
	"log"
)

func main() {
	// ... create and connect client ...

	ctx := context.Background()

	// Create an advisory lock with a unique ID
	lock, err := pgsql.NewAdvisoryLock(ctx, client.Db(), 12345)
	if err != nil {
		log.Fatal(err)
	}
	defer lock.Close()

	// Acquire lock (blocks until available)
	if err := lock.Lock(ctx); err != nil {
		log.Fatal(err)
	}
	defer lock.Unlock(ctx)

	// Do work while holding the lock
	// ...
}
```

### Non-blocking Lock

```go
// Try to acquire lock without blocking
acquired, err := lock.TryLock(ctx)
if err != nil {
    log.Fatal(err)
}
if acquired {
    defer lock.Unlock(ctx)
    // Do work
} else {
    log.Println("Lock is held by another session")
}
```

### Advisory Lock Methods

```go
// Create a new advisory lock
func NewAdvisoryLock(ctx context.Context, db *sqlx.DB, id int) (*AdvisoryLock, error)

// Acquire lock (blocking)
func (l *AdvisoryLock) Lock(ctx context.Context) error

// Try to acquire lock (non-blocking)
func (l *AdvisoryLock) TryLock(ctx context.Context) (bool, error)

// Release the lock
func (l *AdvisoryLock) Unlock(ctx context.Context) error

// Close the lock and release the connection
func (l *AdvisoryLock) Close()
```

### Lock Stacking

Advisory locks are stackable - calling `Lock()` multiple times requires the same number of `Unlock()` calls:

```go
lock.Lock(ctx)   // First lock
lock.Lock(ctx)   // Increments lock count

lock.Unlock(ctx) // Lock still held
lock.Unlock(ctx) // Lock released
```

## Error Constants

```go
const (
    ErrEmptyDSN            = "Empty DSN"
    ErrNilConfig           = "Config is nil"
    ErrInvalidIdleConns    = "Invalid idleConns"
    ErrInvalidMaxConns     = "Invalid maxConns"
    ErrInvalidConnLifeTime = "connLifeTime must be >= 1"
    ErrInvalidConnIdleTime = "connIdleTime must be >= 1"
)
```