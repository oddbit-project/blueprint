# blueprint.provider.sqlite

Blueprint SQLite client

The client uses [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite), a pure-Go (non-CGO) SQLite driver. No C toolchain is required to build.

## Configuration

The SQLite client uses the following configuration:

```json
{
  "sqlite": {
    "dsn": "file:/var/lib/app/app.db",
    "maxOpenConns": 1,
    "maxIdleConns": 1,
    "connLifetime": 3600,
    "connIdleTime": 1800
  }
}
```

### ClientConfig

```go
type ClientConfig struct {
    DSN          string `json:"dsn"`          // SQLite DSN (file path, file: URI, or :memory:)
    MaxOpenConns int    `json:"maxOpenConns"` // Max number of pool connections (default: 1)
    MaxIdleConns int    `json:"maxIdleConns"` // Max number of idle pool connections (default: 1)
    ConnLifetime int    `json:"connLifetime"` // Duration in seconds after which connection is closed (default: 3600)
    ConnIdleTime int    `json:"connIdleTime"` // Duration in seconds for idle connection cleanup (default: 1800)
}
```

SQLite serializes writes on the database file, so a pool of 1 connection is the safe default. Increase `MaxOpenConns` only if you know your workload tolerates `SQLITE_BUSY` under concurrent writers.

### DSN examples

```
# File on disk
file:/var/lib/app/app.db
/var/lib/app/app.db

# In-memory (per-connection, not shared)
:memory:

# Shared in-memory (visible to all connections of the same process)
file::memory:?cache=shared

# PRAGMAs via query string (modernc.org/sqlite extension)
file:/var/lib/app/app.db?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)
```

## Using the Client

```go
package main

import (
	"context"
	"github.com/oddbit-project/blueprint/provider/sqlite"
	"log"
)

func main() {
	cfg := sqlite.NewClientConfig()
	cfg.DSN = "file:/var/lib/app/app.db"

	client, err := sqlite.NewClient(cfg)
	if err != nil {
		log.Fatal(err)
	}
	if err = client.Connect(); err != nil {
		log.Fatal(err)
	}
	defer client.Disconnect()

	ctx := context.Background()
	var version string
	err = client.Db().QueryRowContext(ctx, "SELECT sqlite_version()").Scan(&version)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("SQLite version:", version)
}
```

## Utility Functions

### Database Object Checks

```go
// Check if a table exists
exists, err := sqlite.TableExists(ctx, client, "users")

// Check if a view exists
exists, err := sqlite.ViewExists(ctx, client, "user_view")

// Check if a column exists
exists, err := sqlite.ColumnExists(ctx, client, "users", "email")

// Get SQLite engine version
version, err := sqlite.GetServerVersion(client.Db(), ctx)
```

### Constants

```go
const (
    DriverName = "sqlite"

    TblTypeTable = "table"
    TblTypeView  = "view"
)
```

SQLite has no concept of schemas in the PostgreSQL sense, so object lookups do not take a schema argument.

## Migrations

The sqlite package provides a migration system for managing database schema changes, with the same `migrations.Manager` interface as the other database providers.

### Migration Manager

```go
package main

import (
	"context"
	"github.com/oddbit-project/blueprint/db/migrations"
	"github.com/oddbit-project/blueprint/provider/sqlite"
	"log"
)

func main() {
	cfg := sqlite.NewClientConfig()
	cfg.DSN = "file:/var/lib/app/app.db"
	client, err := sqlite.NewClient(cfg)
	if err != nil {
		log.Fatal(err)
	}
	if err = client.Connect(); err != nil {
		log.Fatal(err)
	}
	defer client.Disconnect()

	ctx := context.Background()

	mm, err := sqlite.NewMigrationManager(ctx, client)
	if err != nil {
		log.Fatal(err)
	}

	src, err := migrations.NewDiskSource("./migrations")
	if err != nil {
		log.Fatal(err)
	}

	if err := mm.Run(ctx, src, migrations.DefaultProgressFn); err != nil {
		log.Fatal(err)
	}
}
```

### Migration Modules

Organize migrations by module:

```go
mm, err := sqlite.NewMigrationManager(ctx, client, sqlite.WithModule("auth"))
```

### Migration File Format

Migration files are `.sql` files using SQLite syntax. Files are sorted alphabetically, so prefix them for ordering:

```sql
-- migrations/001_create_users.sql
CREATE TABLE users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT NOT NULL UNIQUE,
    email TEXT NOT NULL UNIQUE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_users_email ON users(email);
```

### Concurrency note

Unlike the PostgreSQL provider, the SQLite migration manager does not acquire a cross-process advisory lock. SQLite has no server-side locking primitive; writes are serialized at the file-lock level by the engine itself. If multiple processes run migrations against the same database file simultaneously, coordinate them externally.

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
