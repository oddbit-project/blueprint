# DB Grid Sample

This sample demonstrates how to use the `db/dbgrid.go` functionality in the Blueprint framework to build database queries with filtering, sorting, searching, and pagination.

## Overview

The DB Grid functionality provides a way to:

1. Define field specifications using struct tags
2. Build SQL queries with filtering, sorting, and searching capabilities
3. Implement custom field filtering logic
4. Validate grid queries against the field specifications
5. Integrate with the goqu SQL builder

## Features Demonstrated

- Creating a grid from a struct definition
- Adding custom filter functions for fields
- Building simple and complex queries
- Validating queries against field specifications
- Filtering, sorting, and searching data
- Pagination with limit and offset
- Integration with goqu for SQL generation

## Running the Sample

Run the sample in demo mode (no database connection):

```bash
go run main.go
```

Run with database connection (requires a PostgreSQL server):

```bash
go run main.go --connect
```

**Note:** If connecting to a real database, update the connection string in the code:

```go
pgConfig.DSN = "postgres://username:password@localhost:5432/database?sslmode=allow"
```

## Sample Schema

The sample uses a `users` table with the following structure:

```sql
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    username VARCHAR(100) NOT NULL,
    email VARCHAR(255) NOT NULL,
    active BOOLEAN DEFAULT true,
    role VARCHAR(50) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);
```

## Key Concepts

### Field Tagging

The struct definition includes grid tags that define the behavior of each field:

```go
type User struct {
    ID        int       `db:"id" json:"id" grid:"sort,filter"`
    Username  string    `db:"username" json:"username" grid:"sort,search,filter"`
    Email     string    `db:"email" json:"email" grid:"sort,search,filter"`
    Active    bool      `db:"active" json:"active" grid:"filter"`
    Role      string    `db:"role" json:"role" grid:"sort,filter"`
    CreatedAt time.Time `db:"created_at" json:"createdAt" grid:"sort"`
}
```

- `sort`: Field can be used for sorting
- `search`: Field is included in text searches
- `filter`: Field can be filtered

### Custom Filter Functions

The sample shows how to register custom filter functions that handle type conversion:

```go
userGrid.AddFilterFunc("active", func(value any) (any, error) {
    // Convert various string formats to boolean
    switch v := value.(type) {
    case string:
        switch v {
        case "1", "true", "yes", "y", "on":
            return true, nil
        // ...
        }
    // ...
    }
})
```

### Grid Query Structure

The `GridQuery` structure holds query parameters:

```go
query := db.GridQuery{
    SearchType:   db.SearchAny,
    SearchText:   "searchterm",
    FilterFields: map[string]any{"field": "value"},
    SortFields:   map[string]string{"field": db.SortAscending},
    Limit:        10,
    Offset:       20,
}
```

## Further Reading

For more information on the Blueprint framework and its database capabilities, see:

- [Repository Documentation](../../docs/db/repository.md)
- [PostgreSQL Provider](../../docs/provider/pgsql.md)