# Database Structs and Tags

The Blueprint database package is designed around struct-based operations. This document covers how to create and configure structs for database interaction using the comprehensive tag system.

## Overview

The Blueprint db package uses Go structs to represent database tables and records. Struct fields are mapped to database columns through a sophisticated tag system that controls:

- Database field mapping
- Query behavior (insert/update operations)
- Grid functionality (sorting, filtering, searching)
- Data serialization and aliases
- Field metadata and validation

## Basic Struct Definition

### Simple Example

```go
type User struct {
    ID        int       `db:"id"`
    Name      string    `db:"name"`
    Email     string    `db:"email"`
    CreatedAt time.Time `db:"created_at"`
}
```

### Complete Example with All Tags

```go
type User struct {
    ID          int       `db:"id" json:"id" goqu:"skipinsert" grid:"sort,filter" alias:"userId"`
    Name        string    `db:"name" json:"name" grid:"sort,search,filter"`
    Email       string    `db:"email" json:"email" grid:"search,filter"`
    Phone       string    `db:"phone" json:"phone,omitempty" goqu:"omitempty"`
    IsActive    bool      `db:"is_active" json:"isActive" grid:"filter" ch:"is_active"`
    CreatedAt   time.Time `db:"created_at" json:"createdAt" goqu:"skipupdate" grid:"sort"`
    UpdatedAt   time.Time `db:"updated_at" json:"updatedAt" auto:"true"`
    DeletedAt   *time.Time `db:"deleted_at" json:"deletedAt,omitempty" goqu:"omitnil"`
    ProfileData string    `db:"profile_data" json:"-" mapper:"json"`
}
```

## Database Field Tags

### `db` Tag (Primary)

The primary tag for mapping struct fields to database columns.

```go
type User struct {
    ID    int    `db:"id"`           // Maps to 'id' column
    Name  string `db:"user_name"`    // Maps to 'user_name' column
    Email string `db:"email"`        // Maps to 'email' column
}
```

**Special Values:**
- `db:"-"` - Excludes field from database operations entirely

```go
type User struct {
    ID       int    `db:"id"`
    Name     string `db:"name"`
    Internal string `db:"-"`  // Not persisted to database
}
```

### `ch` Tag (ClickHouse Alternative)

Alternative database tag specifically for ClickHouse databases. When present, it takes precedence over the `db` tag for ClickHouse operations.

```go
type Event struct {
    ID        int       `db:"id" ch:"event_id"`
    Timestamp time.Time `db:"created_at" ch:"timestamp"`
    Data      string    `db:"data" ch:"event_data"`
}
```

## Query Behavior Tags

### `goqu` Tag

Controls query generation behavior for insert and update operations.

```go
type User struct {
    ID        int       `db:"id" goqu:"skipinsert"`     // Never included in INSERT
    CreatedAt time.Time `db:"created_at" goqu:"skipupdate"` // Never included in UPDATE
    Phone     string    `db:"phone" goqu:"omitempty"`   // Skip if empty string
    Address   *string   `db:"address" goqu:"omitnil"`   // Skip if nil pointer
}
```

**Available Options:**
- `skipinsert` - Exclude from INSERT operations (auto-generated fields)
- `skipupdate` - Exclude from UPDATE operations (immutable fields)
- `omitempty` - Skip field if it has zero value (empty string, 0, false)
- `omitnil` - Skip field if it's nil (for pointer types)

### `auto` Tag

Marks fields as automatically generated or managed by the database/application.

```go
type User struct {
    ID        int       `db:"id" auto:"true"`        // Auto-generated ID
    CreatedAt time.Time `db:"created_at" auto:"true"` // Auto-set timestamp
    UpdatedAt time.Time `db:"updated_at" auto:"true"` // Auto-updated timestamp
}
```

**Effect:** Fields marked with `auto:"true"` are treated the same as `goqu:"skipinsert,skipupdate"`.

## Grid System Tags

### `grid` Tag

Configures fields for use with the Grid system (dynamic queries, filtering, sorting, searching).

```go
type User struct {
    ID       int    `db:"id" grid:"sort,filter"`           // Sortable and filterable
    Name     string `db:"name" grid:"sort,search,filter"`  // All grid operations
    Email    string `db:"email" grid:"search,filter"`      // Searchable and filterable
    IsActive bool   `db:"is_active" grid:"filter"`         // Filterable only
    Internal string `db:"internal"`                        // No grid operations
}
```

**Available Options:**
- `sort` - Field can be used for sorting results
- `search` - Field is included in text search operations
- `filter` - Field can be used for filtering/WHERE clauses
- `auto` - Equivalent to `auto:"true"` (marks as auto-generated)

**Usage Example:**
```go
query, _ := db.NewGridQuery(db.SearchAny, 10, 0)
query.SearchText = "john"                    // Searches 'name' and 'email' fields
query.FilterFields = map[string]any{         // Filters on 'id' and 'is_active'
    "id": 123,
    "isActive": true,
}
query.SortFields = map[string]string{        // Sorts by 'name' and 'id'
    "name": db.SortAscending,
    "id": db.SortDescending,
}
```

## Alias and Serialization Tags

### `json` Tag

Standard JSON serialization tag, also used for field aliasing in Grid operations.

```go
type User struct {
    ID       int    `db:"id" json:"id"`
    Name     string `db:"name" json:"userName"`      // JSON uses "userName"
    Email    string `db:"email" json:"email"`
    Internal string `db:"internal" json:"-"`         // Excluded from JSON
    Optional string `db:"optional" json:"optional,omitempty"`
}
```

**Grid Integration:** The JSON field names are used as aliases in Grid queries:
```go
// Grid query can use JSON field names
query.FilterFields = map[string]any{
    "userName": "John Doe",  // Maps to 'name' database field
    "id": 123,               // Maps to 'id' database field
}
```

### `xml` Tag

XML serialization tag, also used for field aliasing.

```go
type User struct {
    ID   int    `db:"id" xml:"userId"`
    Name string `db:"name" xml:"userName"`
}
```

### `alias` Tag

Explicit alias for field names in Grid operations and API responses.

```go
type User struct {
    ID   int    `db:"id" alias:"userId"`      // Grid uses "userId"
    Name string `db:"name" alias:"fullName"`  // Grid uses "fullName"
}
```

**Precedence:** `alias` > `json` > `xml` > field name

## Advanced Tags

### `mapper` Tag

Specifies custom field transformation or mapping behavior.

```go
type User struct {
    Preferences map[string]any `db:"preferences" mapper:"json"`  // JSON encode/decode
    Tags        []string       `db:"tags" mapper:"csv"`          // CSV encode/decode
    Metadata    interface{}    `db:"metadata" mapper:"custom"`   // Custom mapper
}
```

**Common Mappers:**
- `json` - JSON serialization for complex types
- `csv` - Comma-separated values for slices
- `custom` - Application-defined transformation

## Complete Tag Reference

### Tag Priority Order

When multiple tags define the same property, the priority is:
1. `alias` (explicit alias)
2. `json` (JSON field name)
3. `xml` (XML field name)
4. Struct field name (default)

### Field Processing Rules

1. **Database Field Name:**
   - `ch` tag (for ClickHouse)
   - `db` tag
   - Struct field name (fallback)

2. **Alias/Display Name:**
   - `alias` tag
   - `json` tag
   - `xml` tag
   - Struct field name (fallback)

3. **Query Behavior:**
   - `goqu` tag options
   - `auto` tag
   - Default behavior

4. **Grid Capabilities:**
   - `grid` tag options
   - No capabilities (default)

## Struct Composition and Embedding

### Embedded Structs

```go
type BaseModel struct {
    ID        int       `db:"id" goqu:"skipinsert" grid:"sort,filter"`
    CreatedAt time.Time `db:"created_at" goqu:"skipupdate" grid:"sort"`
    UpdatedAt time.Time `db:"updated_at" auto:"true"`
}

type User struct {
    BaseModel                                    // Embedded struct
    Name      string `db:"name" grid:"sort,search,filter"`
    Email     string `db:"email" grid:"search,filter"`
}

type Product struct {
    BaseModel                                    // Same base fields
    Title       string          `db:"title" grid:"sort,search,filter"`
    Price       decimal.Decimal `db:"price" grid:"sort,filter"`
    Description string          `db:"description" grid:"search"`
}
```

**Rules for Embedded Structs:**
- All exported fields from embedded structs are included
- Tags from embedded struct fields are preserved
- Anonymous embedding only (named embedding is ignored)
- Conflicts result in error (same database field name)

### Pointer Embedding

```go
type User struct {
    *BaseModel  // Pointer embedding - IGNORED by field scanner
    Name string `db:"name"`
}
```

**Note:** Pointer-to-struct embedding is skipped during field scanning.

## Best Practices

### Naming Conventions

```go
type User struct {
    // Database: snake_case, Struct: PascalCase, JSON: camelCase
    UserID      int    `db:"user_id" json:"userId"`
    FirstName   string `db:"first_name" json:"firstName"`
    LastName    string `db:"last_name" json:"lastName"`
    EmailAddr   string `db:"email_address" json:"emailAddress"`
}
```

### Auto-Generated Fields

```go
type BaseEntity struct {
    ID        int       `db:"id" goqu:"skipinsert" grid:"sort,filter"`
    CreatedAt time.Time `db:"created_at" goqu:"skipupdate" grid:"sort"`
    UpdatedAt time.Time `db:"updated_at" auto:"true"`
}
```

### Grid-Enabled Structs

```go
type SearchableUser struct {
    ID          int     `db:"id" json:"id" grid:"sort,filter"`
    Name        string  `db:"name" json:"name" grid:"sort,search,filter"`
    Email       string  `db:"email" json:"email" grid:"search,filter"`
    Department  string  `db:"department" json:"department" grid:"filter"`
    Salary      float64 `db:"salary" json:"salary" grid:"sort,filter"`
    IsActive    bool    `db:"is_active" json:"isActive" grid:"filter"`
    HireDate    time.Time `db:"hire_date" json:"hireDate" grid:"sort,filter"`
}
```

### Nullable Fields

```go
type User struct {
    ID          int        `db:"id"`
    Name        string     `db:"name"`
    Email       *string    `db:"email" goqu:"omitnil"`        // Nullable string
    PhoneNumber *string    `db:"phone_number" goqu:"omitnil"` // Nullable string
    LastLogin   *time.Time `db:"last_login" goqu:"omitnil"`   // Nullable timestamp
}
```

## Common Patterns

### Audit Fields

```go
type AuditFields struct {
    CreatedAt time.Time  `db:"created_at" goqu:"skipupdate" grid:"sort"`
    UpdatedAt time.Time  `db:"updated_at" auto:"true"`
    CreatedBy int        `db:"created_by" goqu:"skipupdate"`
    UpdatedBy *int       `db:"updated_by" goqu:"omitnil"`
}

type User struct {
    ID    int    `db:"id" goqu:"skipinsert" grid:"sort,filter"`
    Name  string `db:"name" grid:"sort,search,filter"`
    Email string `db:"email" grid:"search,filter"`
    AuditFields
}
```

### Soft Delete

```go
type SoftDelete struct {
    DeletedAt *time.Time `db:"deleted_at" goqu:"omitnil"`
    DeletedBy *int       `db:"deleted_by" goqu:"omitnil"`
}

type User struct {
    ID    int    `db:"id" goqu:"skipinsert"`
    Name  string `db:"name"`
    Email string `db:"email"`
    SoftDelete
}
```

### Multi-Database Support

```go
type Event struct {
    ID        int       `db:"id" ch:"event_id"`                    // Different field names
    Timestamp time.Time `db:"created_at" ch:"timestamp"`           // per database
    UserID    int       `db:"user_id" ch:"user_id"`
    EventType string    `db:"event_type" ch:"event_type"`
    Data      string    `db:"data" ch:"event_data" mapper:"json"`  // JSON in both
}
```

### JSON/Complex Fields

```go
type User struct {
    ID          int                    `db:"id"`
    Name        string                 `db:"name"`
    Preferences map[string]interface{} `db:"preferences" mapper:"json"`
    Tags        []string               `db:"tags" mapper:"json"`
    Metadata    interface{}            `db:"metadata" mapper:"json"`
}
```

## Validation and Error Handling

### Field Validation

The field metadata system performs validation:

```go
type User struct {
    ID    int    `db:"id"`
    Name  string `db:"name"`
    Email string `db:"id"`  // ERROR: duplicate field name
}
```

**Common Errors:**
- Duplicate database field names
- Duplicate alias names
- Invalid tag syntax
- Unsupported field types for certain operations

### Reserved Types

Some types are treated specially and cannot be decomposed:

```go
type User struct {
    ID        int       `db:"id"`
    CreatedAt time.Time `db:"created_at"`  // Reserved type - treated as single field
    Config    MyStruct  `db:"config"`      // Custom struct - decomposed if not reserved
}
```

**Reserved Types:**
- `time.Time`
- `sql.NullString`, `sql.NullInt64`, etc.
- `decimal.Decimal` (if using shopspring/decimal)
- Database-specific types (e.g., PostgreSQL arrays, JSON types)

## Testing Struct Definitions

### Validation Example

```go
func TestUserStructMetadata(t *testing.T) {
    user := &User{}
    
    // Test field metadata extraction
    metadata, err := field.GetStructMeta(reflect.TypeOf(user).Elem())
    assert.NoError(t, err)
    
    // Validate expected fields
    expectedFields := []string{"id", "name", "email", "created_at"}
    actualFields := make([]string, len(metadata))
    for i, m := range metadata {
        actualFields[i] = m.DbName
    }
    
    assert.ElementsMatch(t, expectedFields, actualFields)
    
    // Test grid capabilities
    grid, err := db.NewGrid("users", user)
    assert.NoError(t, err)
    
    sortFields := grid.SortFields()
    assert.Contains(t, sortFields, "id")
    assert.Contains(t, sortFields, "name")
}
```

### Integration Testing

```go
func TestUserRepository(t *testing.T) {
    // Setup test database
    client := setupTestDB(t)
    repo := db.NewRepository(context.Background(), client, "users")
    
    user := &User{
        Name:  "John Doe",
        Email: "john@example.com",
    }
    
    // Test insert
    err := repo.Insert(user)
    assert.NoError(t, err)
    
    // Test fetch
    var users []*User
    err = repo.Fetch(repo.SqlSelect(), &users)
    assert.NoError(t, err)
    assert.Len(t, users, 1)
    
    // Test grid operations
    query, _ := db.NewGridQuery(db.SearchAny, 10, 0)
    query.SearchText = "john"
    
    var searchResults []*User
    err = repo.QueryGrid(user, query, &searchResults)
    assert.NoError(t, err)
}
```

## Performance Considerations

### Field Metadata Caching

The system automatically caches field metadata:

```go
// First call - extracts and caches metadata
grid1, err := db.NewGrid("users", &User{})

// Subsequent calls - uses cached metadata
grid2, err := db.NewGrid("users", &User{}) // Fast - uses cache
```

### Large Structs

For structs with many fields, consider:

```go
type User struct {
    // Core fields with grid support
    ID       int    `db:"id" grid:"sort,filter"`
    Name     string `db:"name" grid:"sort,search,filter"`
    Email    string `db:"email" grid:"search,filter"`
    
    // Extended fields without grid support (reduces metadata size)
    Address1    string `db:"address1"`
    Address2    string `db:"address2"`
    City        string `db:"city"`
    State       string `db:"state"`
    PostalCode  string `db:"postal_code"`
    Country     string `db:"country"`
    
    // Audit fields
    CreatedAt time.Time `db:"created_at" goqu:"skipupdate"`
    UpdatedAt time.Time `db:"updated_at" auto:"true"`
}
```

## Troubleshooting

### Common Issues

1. **Field Not Found in Grid Operations**
   ```go
   // Problem: Field not marked for grid operations
   type User struct {
       Name string `db:"name"` // Missing grid tag
   }
   
   // Solution: Add grid tag
   type User struct {
       Name string `db:"name" grid:"search,filter"`
   }
   ```

2. **Insert/Update Including Auto Fields**
   ```go
   // Problem: Auto field included in operations
   type User struct {
       ID int `db:"id"` // Should be auto-generated
   }
   
   // Solution: Mark as auto or skip
   type User struct {
       ID int `db:"id" goqu:"skipinsert"` // or auto:"true"
   }
   ```

3. **JSON Alias Conflicts**
   ```go
   // Problem: JSON and database names conflict
   type User struct {
       UserID int `db:"user_id" json:"id"` // JSON uses "id"
       ID     int `db:"id" json:"userId"`  // Confusing aliases
   }
   
   // Solution: Use consistent naming
   type User struct {
       ID     int `db:"id" json:"id"`
       UserID int `db:"user_id" json:"userId"`
   }
   ```

## See Also

- [Database Package Overview](index.md)
- [Repository Documentation](repository.md)
- [Field Specifications](fields.md)
- [Data Grid System](dbgrid.md)
- [Query Builder Documentation](query-builder.md)