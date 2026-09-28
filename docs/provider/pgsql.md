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

// Optional: fill deployment-specific identifiers into the DDL, e.g. the role
// the application connects as
src = migrations.Substitute(src, migrations.Vars{"appRole": appRole})
```

See [Substituted Source](../db/migrations.md#substituted-source) for what a
substituted value may hold and how it is recorded.

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

## Transaction-Local Session Settings

`WithTxSession` runs a function inside a transaction after setting custom configuration parameters
(e.g. `app.tenant_id`) and, optionally, switching role, all transaction-local. It is the building
block for row-level security (RLS) policies that read the current tenant or user from the session.

```go
func WithTxSession(ctx context.Context, q dbx.Querier, settings map[string]string, role string,
	fn func(tx dbx.Querier) error) error
```

It runs `dbx.WithTx` and, inside the transaction and before `fn`:

1. runs `SELECT set_config($1, $2, true)` once per setting, in ascending name order. Name and
   value are bound parameters, so a value containing quotes or semicolons is stored verbatim and
   never parsed as SQL;
2. if `role` is not empty, runs `SET LOCAL ROLE <role>`, with the role name quoted by the
   Querier's gohan dialect (`Dialect.QuoteIdent`).

`fn` receives the transaction; it commits when `fn` returns nil and rolls back otherwise, exactly
as `dbx.WithTx`.

### Usage

```go
import (
	"github.com/oddbit-project/blueprint/dbx"
	"github.com/oddbit-project/blueprint/provider/pgsql"
)

q, err := dbx.FromClient(client)
if err != nil {
	return err
}

err = pgsql.WithTxSession(ctx, q,
	map[string]string{"app.tenant_id": tenantID, "app.user_id": userID},
	"app_tenant", // role to run as; "" keeps the connection's role
	func(tx dbx.Querier) error {
		docs, err := dbx.NewRepository[Document](tx, "documents")
		if err != nil {
			return err
		}
		rows, err := docs.List(ctx, nil) // only this tenant's rows
		if err != nil {
			return err
		}
		return render(rows)
	})
```

### RLS policy pattern

Read the setting in the policy with `current_setting(name, true)`. The second argument
(`missing_ok`) makes an unknown setting return NULL instead of raising an error. Once a
transaction-local setting has been used on a connection, PostgreSQL keeps the name around and
reports it as the empty string after the transaction ends, so wrap it in `nullif(..., '')`.
Either way an unset tenant compares as NULL and the policy matches no rows:

```sql
CREATE ROLE app_tenant NOLOGIN;
GRANT app_tenant TO app_login;          -- the role your DSN connects as
GRANT SELECT, INSERT, UPDATE, DELETE ON documents TO app_tenant;

ALTER TABLE documents ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON documents
    USING (tenant_id = nullif(current_setting('app.tenant_id', true), '')::int);
```

Superusers and roles with `BYPASSRLS` ignore policies, and so does the table owner unless the table
has `FORCE ROW LEVEL SECURITY`. That is why the example switches to a dedicated, unprivileged role.

### Why not `SET`?

A plain `SET app.tenant_id = '42'` (or `SET ROLE`) is session-level: it stays on the database
connection until it is changed or the connection closes. `database/sql` returns connections to a pool
without resetting them, so the next request that borrows the same connection runs with the previous
request's tenant and role. `set_config(..., true)` and `SET LOCAL` only last until the end of the
current transaction; PostgreSQL reverts them on commit and on rollback, so nothing is left behind
for the next user of the connection.

### Validation

Invalid input is rejected before any statement is sent (no transaction is started):

- **Setting names** must be custom parameter names: two or more dot-separated parts, each starting
  with an ASCII letter or underscore, followed by ASCII letters, digits, underscores or `$`
  (`app.tenant_id`, `myapp.v2.user`). Built-in parameters without a dot (`search_path`,
  `statement_timeout`, `role`) are rejected, as are empty names. This follows PostgreSQL's rule
  for custom parameters, limited to ASCII. Two names that differ only in case are rejected
  (PostgreSQL treats them as one setting). Use constant names: a dotted name also matches
  extension settings such as `auto_explain.log_min_duration`, which `set_config` would change too.
  Error: `ErrInvalidSettingName`.
- **Role** names are quoted as a single identifier, so they must not contain a dot or a NUL byte,
  must not be `*`, and must be at most 63 bytes (PostgreSQL would otherwise truncate a longer
  quoted name and switch to whichever role matches the prefix).
  Error: `ErrInvalidRoleName`.
- The Querier's dialect must be PostgreSQL. Error: `ErrNotPostgres`.

### Inside an existing transaction

If `q` is already a transaction (a `dbx.TxQuerier`, e.g. the `tx` of an enclosing `dbx.WithTx`),
`WithTxSession` joins it rather than starting a new one. The settings and role still apply with
`is_local = true`, but they last until the **outer** transaction ends, not just until `fn`
returns. Code that runs later in that transaction sees the same tenant and role.

## Error Constants

```go
const (
    ErrEmptyDSN            = "Empty DSN"
    ErrNilConfig           = "Config is nil"
    ErrInvalidIdleConns    = "Invalid idleConns"
    ErrInvalidMaxConns     = "Invalid maxConns"
    ErrInvalidConnLifeTime = "connLifeTime must be >= 1"
    ErrInvalidConnIdleTime = "connIdleTime must be >= 1"

    // WithTxSession
    ErrInvalidSettingName = "pgsql: invalid custom setting name"
    ErrInvalidRoleName    = "pgsql: invalid role name"
    ErrNotPostgres        = "pgsql: querier dialect is not PostgreSQL"
)
```