# db.Grid

Data grid component for building dynamic SQL queries with filtering, sorting, searching, and pagination capabilities.

## Overview

The Grid component provides a structure and methods for building dynamic database queries based on a structured configuration. 
It's particularly useful for:

- Building data tables with server-side processing
- Implementing API endpoints for data retrieval with dynamic criteria
- Constructing complex queries with multiple filtering, sorting, and search conditions

The Grid component uses a struct's field tags to define which fields can be:
- Filtered
- Sorted
- Searched

In addition, it detects alias names such as json field names, and transparently maps them to the appropriate database field.

It then validates and builds queries using the [goqu](https://github.com/doug-martin/goqu) SQL builder.

## Usage

### Basic Usage

```go
package main

import (
    "fmt"
    "github.com/oddbit-project/blueprint/db"
    "log"
)

// Define a struct with grid tags
type UserRecord struct {
    ID        int    `db:"id" json:"id" grid:"sort,filter"`
    Username  string `db:"username" json:"username" grid:"sort,search,filter"`
    Email     string `db:"email" json:"email" grid:"sort,search,filter"`
    Active    bool   `db:"active" json:"active" grid:"filter"`
}

func main() {
    // Create a grid from the struct
    grid, err := db.NewGrid("users", &UserRecord{})
    if err != nil {
        log.Fatal(err)
    }
    
    // Create a query
    query, err := db.NewGridQuery(db.SearchAny, 10, 0)
    if err != nil {
        log.Fatal(err)
    }
    
    // Set search text, filters, and sort conditions
    query.SearchText = "john"
    query.FilterFields = map[string]any{
        "active": true,
    }
    query.SortFields = map[string]string{
        "username": db.SortAscending,
    }
    
    // Validate the query
    if err := grid.ValidQuery(query); err != nil {
        log.Fatal(err)
    }
    
    // Build the query
    statement, err := grid.Build(nil, query)
    if err != nil {
        log.Fatal(err)
    }
    
    // Get the SQL
    sql, args, err := statement.ToSQL()
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Println("SQL:", sql)
    fmt.Println("Args:", args)
}
```

### Custom Filter Functions

You can add custom filter functions to transform filter values before they're used in queries:

```go
// Add a filter function for a boolean field
grid.AddFilterFunc("active", func(value any) (any, error) {
    switch v := value.(type) {
    case string:
        switch v {
        case "1", "true", "yes", "y", "on":
            return true, nil
        case "0", "false", "no", "n", "off":
            return false, nil
        default:
            return nil, db.GridError{
                Scope:   "filter",
                Field:   "active",
                Message: "invalid boolean value",
            }
        }
    case bool:
        return v, nil
    case int:
        return v != 0, nil
    default:
        return nil, db.GridError{
            Scope:   "filter",
            Field:   "active",
            Message: "type not supported",
        }
    }
})
```

### Using with a Database

Here's how to use a Grid with a database connection:

```go
package main

import (
    "context"
    "github.com/oddbit-project/blueprint/db"
    "github.com/oddbit-project/blueprint/provider/pgsql"
    "log"
)

func main() {
    // Create the grid as shown in previous examples
    grid, _ := db.NewGrid("users", &UserRecord{})
    query, _ := db.NewGridQuery(db.SearchNone, 10, 0)
    
    // Set up query parameters
    query.SortFields = map[string]string{
        "username": db.SortAscending,
    }
    
    // Build the query
    statement, _ := grid.Build(nil, query)
    
    // Connect to the database
    pgConfig := pgsql.NewClientConfig()
    pgConfig.DSN = "postgres://username:password@localhost:5432/database?sslmode=allow"
    
    client, err := pgsql.NewClient(pgConfig)
    if err != nil {
        log.Fatal(err)
    }
    defer client.Disconnect()
    
    // Execute the query
    sqlStr, args, _ := statement.ToSQL()
    rows, err := client.Db().QueryxContext(context.Background(), sqlStr, args...)
    if err != nil {
        log.Fatal(err)
    }
    defer rows.Close()
    
    // Process the results
    var users []UserRecord
    for rows.Next() {
        var user UserRecord
        if err := rows.StructScan(&user); err != nil {
            log.Fatal(err)
        }
        users = append(users, user)
    }
    
    if err := rows.Err(); err != nil {
        log.Fatal(err)
    }
    
    // Use the users slice
}
```

## Component Reference

### Constants

```go
// Sort direction constants
SortAscending  = "asc"
SortDescending = "desc"

// Search type constants
SearchNone  = 0  // No search
SearchStart = 1  // Search for terms at the start (%term)
SearchEnd   = 2  // Search for terms at the end (term%)
SearchAny   = 3  // Search for terms anywhere (%term%)
```

### Types

#### GridFilterFunc

```go
type GridFilterFunc func(lookupValue any) (any, error)
```

A function type for custom filtering operations that transform input values to database-compatible values.

#### Grid

```go
type Grid struct {
    tableName  string
    spec       *FieldSpec
    filterFunc map[string]GridFilterFunc
}
```

The main Grid component that handles query building and validation.

#### GridQuery

```go
type GridQuery struct {
    SearchType   uint              `db:"searchType"`
    SearchText   string            `json:"searchText,omitempty"`
    FilterFields map[string]any    `json:"filterFields,omitEmpty"`
    SortFields   map[string]string `json:"sortFields,omitempty"`
    Offset       uint              `json:"offset,omitempty"`
    Limit        uint              `json:"limit,omitempty"`
}
```

Represents a query with search, filter, sort, and pagination options.

#### GridError

```go
type GridError struct {
    Scope   string `json:"scope"`
    Field   string `json:"field"`
    Message string `json:"message"`
}
```

Error type that includes the scope and field where an error occurred.

### Functions

#### NewGridQuery

```go
func NewGridQuery(searchType uint, limit uint, offset uint) (GridQuery, error)
```

Creates a new GridQuery with the specified search type, limit, and offset.

#### NewGrid

```go
func NewGrid(tableName string, record any) (*Grid, error)
```

Creates a new Grid from a struct definition.

#### NewGridWithSpec

```go
func NewGridWithSpec(tableName string, spec *FieldSpec) *Grid
```

Creates a new Grid from an existing FieldSpec.

### Grid Methods

#### AddFilterFunc

```go
func (grid *Grid) AddFilterFunc(dbField string, f GridFilterFunc) *Grid
```

Adds a custom filter function for a specific field.

#### ValidQuery

```go
func (grid *Grid) ValidQuery(query GridQuery) error
```

Validates a GridQuery against the grid's field specifications.

#### Build

```go
func (grid *Grid) Build(qry *goqu.SelectDataset, args GridQuery) (*goqu.SelectDataset, error)
```

Builds a goqu SelectDataset from the grid query.

## Field Tags

The Grid component uses the `grid` tag to determine the capabilities of each field:

```go
type UserRecord struct {
    ID        int    `db:"id" json:"id" grid:"sort,filter"`
    Username  string `db:"username" json:"username" grid:"sort,search,filter"`
    Email     string `db:"email" json:"email" grid:"sort,search,filter"`
    Active    bool   `db:"active" json:"active" grid:"filter"`
}
```

Available tag options:
- `sort`: The field can be used for sorting
- `search`: The field is included in text searches
- `filter`: The field can be used in filters

## Query Building Process

1. Create a `Grid` from a struct
2. Create a `GridQuery` with search type, limit, and offset
3. Set the search text, filter fields, and sort fields in the GridQuery
4. Validate the query using `grid.ValidQuery(query)`
5. Build the SQL query using `grid.Build(nil, query)`
6. Convert to SQL using `statement.ToSQL()`
7. Execute the query against a database

## Error Handling

The Grid component returns well-defined errors with scope, field, and message information:

```go
GridError{
    Scope:   "filter",
    Field:   "active",
    Message: "field is not filterable",
}
```

Common error scopes:
- `filter`: Errors related to filter fields
- `sort`: Errors related to sort fields
- `search`: Errors related to search operations

## Examples

### Filtering Records

```go
// Create a grid and query
grid, _ := db.NewGrid("users", &UserRecord{})
query, _ := db.NewGridQuery(db.SearchNone, 10, 0)

// Set multiple filters
query.FilterFields = map[string]any{
    "active": true,
    "id": 100,
}

// Validate and build the query
grid.ValidQuery(query)
statement, _ := grid.Build(nil, query)

// Get SQL
sql, _, _ := statement.ToSQL()
// SQL: SELECT * FROM "users" WHERE (("active" IS TRUE) AND ("id" = 100)) LIMIT 10
```

### Text Searching

```go
// Create a grid and query
grid, _ := db.NewGrid("users", &UserRecord{})
query, _ := db.NewGridQuery(db.SearchAny, 10, 0)

// Set search text
query.SearchText = "john.doe"

// Validate and build the query
grid.ValidQuery(query)
statement, _ := grid.Build(nil, query)

// Get SQL 
sql, _, _ := statement.ToSQL()
// SQL: SELECT * FROM "users" WHERE (("username" LIKE '%john.doe%') OR ("email" LIKE '%john.doe%')) LIMIT 10
```

### Sorting Results

```go
// Create a grid and query
grid, _ := db.NewGrid("users", &UserRecord{})
query, _ := db.NewGridQuery(db.SearchNone, 10, 0)

// Set multiple sort fields
query.SortFields = map[string]string{
    "username": db.SortAscending,
    "id": db.SortDescending,
}

// Validate and build the query
grid.ValidQuery(query)
statement, _ := grid.Build(nil, query)

// Get SQL
sql, _, _ := statement.ToSQL()
// SQL: SELECT * FROM "users" ORDER BY "username" ASC, "id" DESC LIMIT 10
```

### Pagination

```go
// Create a grid and query with offset and limit
grid, _ := db.NewGrid("users", &UserRecord{})
query, _ := db.NewGridQuery(db.SearchNone, 10, 20)  // Limit 10, offset 20

// Validate and build the query
grid.ValidQuery(query)
statement, _ := grid.Build(nil, query)

// Get SQL
sql, _, _ := statement.ToSQL()
// SQL: SELECT * FROM "users" LIMIT 10 OFFSET 20
```

### Custom Selects

```go
// Create a grid and query
grid, _ := db.NewGrid("users", &UserRecord{})
query, _ := db.NewGridQuery(db.SearchNone, 0, 0)

// Set a filter
query.FilterFields = map[string]any{
    "active": true,
}

// Create a custom select
customSelect := goqu.Select(goqu.COUNT("*")).From("users")

// Build with the custom select
statement, _ := grid.Build(customSelect, query)

// Get SQL
sql, _, _ := statement.ToSQL()
// SQL: SELECT COUNT(*) FROM "users" WHERE ("active" IS TRUE)
```

## See Also

- [Repository Documentation](repository.md)
- [Field Specifications](../db/fields.md)
- [PostgreSQL Provider](../provider/pgsql.md)