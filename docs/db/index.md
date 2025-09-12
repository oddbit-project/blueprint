# Database Package

The Blueprint database package provides a comprehensive, interface-driven abstraction layer for SQL database operations. 
It combines the power of the Repository pattern with advanced features like dynamic query building, field metadata extraction, 
and database migrations.

> The database package functionality focus on working with structs, not individual variables; most functions won't work
> with individual variables;

> Not all funcionality is available for ClickHouse databases 

## Overview

The db package is designed around the principle of interface-based composition, offering different levels of abstraction to suit various use cases:

- **High-level**: Repository pattern with automatic query building
- **Medium-level**: Grid system for dynamic, filterable queries  
- **Low-level**: Raw SQL functions and query builders

## Architecture

The package consists of several interconnected components:

```
db/
├── Core Interfaces
│   ├── Client          - Database connection management
│   ├── Repository      - High-level CRUD operations
│   └── Transaction     - Transactional operations
├── Query Building
│   ├── Grid            - Dynamic query building with filtering/sorting
│   ├── QueryBuilder    - SQL generation and dialect abstraction
│   └── Functions       - Raw SQL operations and utilities
├── Metadata System
│   ├── Field           - Struct field analysis and mapping
│   └── Types           - Type detection and validation
└── Migration System
    ├── Manager         - Migration execution and tracking
    └── Sources         - Migration source implementations
```

## Quick Start

### Basic Repository Usage

```go
package main

import (
    "context"
    "github.com/oddbit-project/blueprint/db"
    "github.com/oddbit-project/blueprint/provider/pgsql"
    "log"
    "time"
)

type User struct {
    ID        int       `db:"id" goqu:"skipinsert"`
    Name      string    `db:"name"`
    Email     string    `db:"email"`
    CreatedAt time.Time `db:"created_at"`
}

func main() {
    // Setup database connection
    pgConfig := pgsql.NewClientConfig()
    pgConfig.DSN = "postgres://user:pass@localhost/db?sslmode=disable"
    
    client, err := pgsql.NewClient(pgConfig)
    if err != nil {
        log.Fatal(err)
    }
    defer client.Disconnect()
    
    // Create repository
    repo := db.NewRepository(context.Background(), client, "users")
    
    // Insert single user
    user := &User{
        Name:      "John Doe",
        Email:     "john@example.com", 
        CreatedAt: time.Now(),
    }
    
    if err := repo.Insert(user); err != nil {
        log.Fatal(err)
    }
    
    // Batch insert multiple users
    batchUsers := []*User{
        {Name: "Alice", Email: "alice@example.com", CreatedAt: time.Now()},
        {Name: "Bob", Email: "bob@example.com", CreatedAt: time.Now()},
        {Name: "Charlie", Email: "charlie@example.com", CreatedAt: time.Now()},
    }
    
    if err := repo.Insert(db.ToAnySlice(batchUsers)...); err != nil {
        log.Fatal(err)
    }
    
    // Fetch all users
    var users []*User
    if err := repo.Fetch(repo.SqlSelect(), &users); err != nil {
        log.Fatal(err)
    }
}
```

### Grid-based Dynamic Queries

```go
type User struct {
    ID       int    `db:"id" json:"id" grid:"sort,filter"`
    Username string `db:"username" json:"username" grid:"sort,search,filter"`
    Email    string `db:"email" json:"email" grid:"search,filter"`
    Active   bool   `db:"active" json:"active" grid:"filter"`
}

func main() {
    // ... setup repository ...
    
    // Create dynamic query
    query, _ := db.NewGridQuery(db.SearchAny, 10, 0)
    query.SearchText = "john"
    query.FilterFields = map[string]any{"active": true}
    query.SortFields = map[string]string{"username": db.SortAscending}
    
    // Execute query
    var users []*User
    err := repo.QueryGrid(&User{}, query, &users)
    if err != nil {
        log.Fatal(err)
    }
}
```

## Core Components

### [Structs and Tags](structs-and-tags.md)
Comprehensive guide to creating database structs and using the tag system. Covers all available tags for field mapping, query behavior, grid functionality, and data serialization.

**Key Features:**
- Complete tag reference (db, goqu, grid, json, auto, etc.)
- Struct composition and embedding patterns
- Best practices for field definition
- Multi-database support with different field mappings

### [Client Interface](client.md)
Database connection management and configuration. Provides the foundation for all database operations with support for connection options and health checking.

**Key Features:**
- Connection lifecycle management
- Provider abstraction (PostgreSQL, ClickHouse)
- Connection health monitoring
- Configuration flexibility

### [Repository Pattern](repository.md)
High-level interface for CRUD operations with automatic query generation and transaction support.

**Key Features:**
- Interface-based design with composition
- Automatic query building from structs
- Transaction support
- Counting and aggregation operations
- Grid integration for dynamic queries

### [Data Grid System](dbgrid.md)
Dynamic query building with filtering, sorting, searching, and pagination capabilities based on struct field tags.

**Key Features:**
- Struct tag-driven configuration
- Dynamic filtering and sorting
- Text search across multiple fields
- Custom filter functions
- Pagination support

### [Field Metadata](fields.md)
Struct field analysis and mapping system that powers the Repository and Grid components.

**Key Features:**
- Automatic field discovery from struct tags
- Alias mapping (JSON, XML, custom)
- Embedded struct support
- Type-aware processing
- Caching for performance

### [Query Builder](query-builder.md)
Low-level SQL generation with dialect abstraction and advanced features like RETURNING clauses.

**Key Features:**
- SQL dialect abstraction
- Type-safe query building
- RETURNING clause support
- Batch operations
- Integration with field metadata

### [Database Functions](functions.md)
Low-level database operations and utilities for advanced use cases.

**Key Features:**
- Raw SQL execution
- Intelligent result scanning
- Context-aware operations
- Type detection and conversion
- Error handling utilities

### [Migration System](migrations.md)
Database schema migration management with progress tracking and multiple source types.

**Key Features:**
- Interface-based migration sources
- Progress tracking and callbacks
- SHA2-based change detection
- Rollback protection
- Multiple source implementations

## Integration with Providers

The db package integrates seamlessly with Blueprint's provider system:

- **[PostgreSQL Provider](../provider/pgsql.md)**: Full-featured PostgreSQL support with advanced types
- **[ClickHouse Provider](../provider/clickhouse.md)**: Analytics database support with specialized features

## Design Principles

### Interface-Driven Design
All major components are defined by interfaces, allowing for easy testing, mocking, and extensibility.

### Composition Over Inheritance
The Repository interface composes multiple smaller interfaces (Reader, Writer, Updater, etc.) for flexibility.

### Context-Aware Operations
All database operations accept and propagate Go contexts for proper cancellation and timeout handling.

### Type Safety
Extensive use of Go's type system to catch errors at compile time and provide clear APIs.

### Performance Focus
Built-in caching, connection pooling, and efficient query generation for production use.

## When to Use Each Component

### Use Repository When:
- Building standard CRUD applications
- Need automatic query generation
- Want transaction support
- Require counting and aggregation

### Use Grid When:
- Building data tables with server-side processing
- Need dynamic filtering and sorting
- Implementing search functionality
- Creating flexible APIs

### Use Functions When:
- Need raw SQL control
- Implementing complex queries
- Building custom abstractions
- Performance-critical operations

### Use Query Builder When:
- Need portable SQL generation
- Building complex queries programmatically
- Require RETURNING clause support
- Want type-safe query construction

## Error Handling

The package provides consistent error handling patterns:

```go
// Check for empty results
if db.EmptyResult(err) {
    // Handle no rows found
}

// Grid errors include scope and field information
if gridErr, ok := err.(db.GridError); ok {
    fmt.Printf("Grid error in %s for field %s: %s", 
        gridErr.Scope, gridErr.Field, gridErr.Message)
}
```

## Best Practices

1. **Use Contexts**: Always pass contexts for proper cancellation handling
2. **Handle Empty Results**: Check for `db.EmptyResult(err)` when fetching single records
3. **Use Transactions**: Wrap related operations in transactions for consistency
4. **Cache Field Specs**: Repository automatically caches field metadata for performance
5. **Validate Grid Queries**: Always validate Grid queries before building SQL
6. **Use Appropriate Abstraction**: Choose the right level based on your needs

## Performance Considerations

- **Field Spec Caching**: Struct metadata is cached automatically
- **Connection Pooling**: Managed by underlying provider packages
- **Prepared Statements**: Used automatically where beneficial
- **Batch Operations**: Available for bulk inserts and updates
- **Lazy Loading**: Grid field specs are built on-demand

## See Also

- [Getting Started Guide](../index.md)
- [Configuration Management](../config/config.md)
- [Security Best Practices](../provider/httpserver/security.md)