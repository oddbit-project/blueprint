# Database Client

The Client interface provides the foundation for database connections in the Blueprint db package. It abstracts database connection management and provides a consistent interface across different database providers.

## Overview

The Client interface and SqlClient implementation handle:

- Database connection lifecycle management
- Connection health monitoring
- Configuration through connection options
- Provider abstraction for different database types

## Client Interface

```go
type Client interface {
    GetClient() *sqlx.DB
    IsConnected() bool
    Connect() error
    Disconnect()
}
```

### Methods

#### GetClient() *sqlx.DB
Returns the underlying sqlx.DB connection. This provides direct access to the database connection for advanced operations.

#### IsConnected() bool
Returns true if the client has an active database connection.

#### Connect() error
Establishes a connection to the database using the configured DSN and options. This method:
- Opens a connection using the specified driver
- Applies any connection options
- Performs a health check with Ping()

#### Disconnect()
Closes the database connection and cleans up resources. Safe to call multiple times.

## SqlClient Implementation

The SqlClient struct provides the standard implementation of the Client interface:

```go
type SqlClient struct {
    Conn        *sqlx.DB
    Dsn         string
    DriverName  string
    connOptions ConnectionOptions
}
```

### Fields

- **Conn**: The active sqlx.DB connection (nil when disconnected)
- **Dsn**: Database connection string
- **DriverName**: SQL driver name (e.g., "postgres", "clickhouse")
- **connOptions**: Optional connection configuration

## ConnectionOptions Interface

```go
type ConnectionOptions interface {
    Apply(db *sqlx.DB) error
}
```

Connection options allow customization of the database connection after it's established. This is used by provider packages to configure:

- Connection pool settings
- Timeout values
- SSL/TLS configuration
- Database-specific parameters

## Usage Examples

### Basic Connection

```go
package main

import (
    "github.com/oddbit-project/blueprint/db"
    "github.com/oddbit-project/blueprint/provider/pgsql"
    "log"
)

func main() {
    // Create client with basic configuration
    client := db.NewSqlClient(
        "postgres://user:pass@localhost/dbname?sslmode=disable",
        "postgres",
        nil, // no connection options
    )
    
    // Connect to database
    if err := client.Connect(); err != nil {
        log.Fatal("Failed to connect:", err)
    }
    defer client.Disconnect()
    
    // Check connection status
    if client.IsConnected() {
        log.Println("Connected to database")
    }
    
    // Get underlying sqlx.DB for direct operations
    db := client.GetClient()
    rows, err := db.Query("SELECT version()")
    if err != nil {
        log.Fatal(err)
    }
    defer rows.Close()
}
```

### Using Provider Packages

The recommended approach is to use provider packages that handle client creation and configuration:

```go
package main

import (
    "github.com/oddbit-project/blueprint/provider/pgsql"
    "log"
)

func main() {
    // Create PostgreSQL client with provider
    config := pgsql.NewClientConfig()
    config.DSN = "postgres://user:pass@localhost/dbname?sslmode=disable"
    config.MaxOpenConns = 25
    config.MaxIdleConns = 5
    
    client, err := pgsql.NewClient(config)
    if err != nil {
        log.Fatal("Failed to create client:", err)
    }
    defer client.Disconnect()
    
    // Client is automatically connected and configured
    if client.IsConnected() {
        log.Println("PostgreSQL client ready")
    }
}
```

### Connection with Custom Options

```go
package main

import (
    "github.com/jmoiron/sqlx"
    "github.com/oddbit-project/blueprint/db"
    "time"
)

// Custom connection options
type CustomOptions struct {
    MaxOpenConns int
    MaxIdleConns int
    MaxLifetime  time.Duration
}

func (opts *CustomOptions) Apply(db *sqlx.DB) error {
    db.SetMaxOpenConns(opts.MaxOpenConns)
    db.SetMaxIdleConns(opts.MaxIdleConns)
    db.SetConnMaxLifetime(opts.MaxLifetime)
    return nil
}

func main() {
    options := &CustomOptions{
        MaxOpenConns: 20,
        MaxIdleConns: 5,
        MaxLifetime:  time.Hour,
    }
    
    client := db.NewSqlClient(
        "postgres://user:pass@localhost/dbname?sslmode=disable",
        "postgres",
        options,
    )
    
    if err := client.Connect(); err != nil {
        log.Fatal(err)
    }
    defer client.Disconnect()
}
```

### Connection Health Checking

```go
func checkConnectionHealth(client db.Client) error {
    if !client.IsConnected() {
        return errors.New("client not connected")
    }
    
    // Perform health check
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    return client.GetClient().PingContext(ctx)
}

func maintainConnection(client db.Client) {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()
    
    for range ticker.C {
        if err := checkConnectionHealth(client); err != nil {
            log.Printf("Connection health check failed: %v", err)
            
            // Attempt reconnection
            if err := client.Connect(); err != nil {
                log.Printf("Reconnection failed: %v", err)
            } else {
                log.Println("Reconnected successfully")
            }
        }
    }
}
```

## Integration with Repository

The Client is typically used as the foundation for Repository instances:

```go
package main

import (
    "context"
    "github.com/oddbit-project/blueprint/db"
    "github.com/oddbit-project/blueprint/provider/pgsql"
)

func main() {
    // Create and configure client
    config := pgsql.NewClientConfig()
    config.DSN = "postgres://user:pass@localhost/dbname?sslmode=disable"
    
    client, err := pgsql.NewClient(config)
    if err != nil {
        log.Fatal(err)
    }
    defer client.Disconnect()
    
    // Create repository using the client
    repo := db.NewRepository(context.Background(), client, "users")
    
    // Repository operations use the client's connection
    count, err := repo.Count()
    if err != nil {
        log.Fatal(err)
    }
    
    log.Printf("User count: %d", count)
}
```

## Error Handling

The Client interface uses standard Go error handling patterns:

```go
func connectWithRetry(client db.Client, maxRetries int) error {
    var lastErr error
    
    for i := 0; i < maxRetries; i++ {
        if err := client.Connect(); err != nil {
            lastErr = err
            log.Printf("Connection attempt %d failed: %v", i+1, err)
            time.Sleep(time.Duration(i+1) * time.Second)
            continue
        }
        return nil
    }
    
    return fmt.Errorf("failed to connect after %d attempts: %w", maxRetries, lastErr)
}
```

## Provider Implementations

Different database providers implement the Client interface through their own client types:

### PostgreSQL Client
```go
// Provider-specific client with PostgreSQL optimizations
client, err := pgsql.NewClient(config)
```

### ClickHouse Client
```go
// Provider-specific client with ClickHouse optimizations  
client, err := clickhouse.NewClient(config)
```

Each provider client:
- Implements the Client interface
- Provides database-specific connection options
- Handles provider-specific configuration
- Optimizes for the target database type

## Best Practices

### Connection Management
1. **Always defer Disconnect()**: Ensure connections are properly closed
2. **Check connection status**: Use IsConnected() before operations
3. **Handle connection failures**: Implement retry logic for robustness
4. **Monitor connection health**: Periodically check connection status

### Configuration
1. **Use provider packages**: They handle database-specific optimizations
2. **Configure connection pools**: Set appropriate pool sizes for your workload
3. **Set timeouts**: Configure appropriate timeout values
4. **Use SSL/TLS**: Enable encryption for production deployments

### Error Handling
1. **Handle connection errors**: Network issues, authentication failures, etc.
2. **Implement reconnection logic**: For long-running applications
3. **Log connection events**: For debugging and monitoring
4. **Graceful degradation**: Handle database unavailability

## Performance Considerations

### Connection Pooling
- Provider packages handle connection pooling automatically
- Configure pool sizes based on your application's concurrency needs
- Monitor pool utilization and adjust as needed

### Connection Reuse
- The Repository pattern reuses the client connection efficiently
- Avoid creating multiple clients for the same database
- Share clients across Repository instances when appropriate

### Health Checking
- Implement periodic health checks for long-running connections
- Use reasonable timeout values to avoid blocking operations
- Consider using connection pool health checking features

## See Also

- [Repository Documentation](repository.md)
- [PostgreSQL Provider](../provider/pgsql.md)
- [ClickHouse Provider](../provider/clickhouse.md)
- [Database Package Overview](index.md)