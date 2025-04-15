# blueprint.provider.clickhouse

Blueprint ClickHouse client implementation

## Overview

The ClickHouse client provides a simple interface for connecting to and working with ClickHouse databases in Go applications. It supports:

- Multiple connection hosts with different connection strategies
- Secure connections with TLS
- Various compression algorithms
- Connection pooling configuration
- Repository pattern for database operations

## Configuration

The client can be configured with the following options:

```go
type ClientConfig struct {
    Hosts            []string       // List of ClickHouse hosts to connect to
    Database         string         // Database name
    Username         string         // Username for authentication
    Debug            bool           // Enable debug mode
    Compression      string         // Compression algorithm: lz4, none, zstd, gzip, br, deflate
    DialTimeout      int            // Connection timeout in seconds
    MaxOpenConns     int            // Maximum number of open connections
    MaxIdleConns     int            // Maximum number of idle connections
    ConnMaxLifetime  int            // Maximum connection lifetime in seconds
    ConnStrategy     string         // Connection strategy: sequential or roundRobin
    BlockBufferSize  uint8          // Block buffer size
    Settings         map[string]any // ClickHouse settings
    // Secure password configuration
    DefaultCredentialConfig
    // TLS configuration
    ClientConfig
}
```

## Using the client

```go
package main

import (
    "context"
    "fmt"
    "github.com/oddbit-project/blueprint/provider/clickhouse"
    "log"
)

func main() {
    // Create a new client configuration
    config := clickhouse.NewClientConfig()
    config.Hosts = []string{"localhost:9000"}
    config.Database = "default"
    config.Username = "default"
    config.Password = "password" // Set password via DefaultCredentialConfig

    // Connect to ClickHouse
    client, err := clickhouse.NewClient(config)
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    // Ping the server
    ctx := context.Background()
    if err = client.Ping(ctx); err != nil {
        log.Fatal(err)
    }

    // Create a repository for a table
    repo := client.NewRepository(ctx, "my_table")

    // Use the repository for database operations
    var records []MyRecord
    err = repo.FetchWhere(map[string]any{"active": true}, &records)
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Found %d active records\n", len(records))
}
```

## Repository Interface

The client provides a Repository interface that implements the following operations:

- Table identification (Name)
- SQL builders (Select, Insert, Update, Delete)
- Reading operations (FetchOne, Fetch, FetchRecord, FetchByKey, FetchWhere)
- Query execution (Exec, RawExec)
- Data modification (Insert, InsertAsync)
- Record deletion (Delete, DeleteWhere, DeleteByKey)
- Record counting (Count, CountWhere)

## Compression Options

The client supports the following compression algorithms:

- `lz4` - Default, fast compression
- `none` - No compression
- `zstd` - High compression ratio
- `gzip` - Traditional compression
- `br` - Brotli compression
- `deflate` - Deflate compression

## Connection Strategies

Two connection strategies are available:

- `sequential` (default) - Try hosts in order
- `roundRobin` - Distribute connections among hosts

## Notes

- ClickHouse doesn't fully support all SQL operations like traditional RDBMS
- DELETE operations have limitations in ClickHouse (see documentation)
- INSERT...RETURNING is not supported