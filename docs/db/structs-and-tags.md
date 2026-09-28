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
    ProfileData string    `db:"profile_data" json:"-"`
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

### `ch` Tag (ClickHouse)

The ClickHouse column name. Two different pieces of code read it, with opposite precedence:

- **Blueprint's field metadata** (`db/field`, and `gohan/field` for `dbx`), which builds grids and
  `dbx` column lists, uses the `db` tag. The `ch` tag is used only when a field has no `db` tag.
- **clickhouse-go**, which scans and inserts rows for both the legacy `provider/clickhouse`
  repository and `dbx`'s ClickHouse `Querier`, reads only the `ch` tag, falling back to the Go
  field name as written (`Name`, not `name`). It ignores the `db` tag.

**Legacy `provider/clickhouse` repository.** `Fetch*`, `Insert` and `InsertAsync` go through
clickhouse-go, so there the `ch` tag is the column name. The grid (`QueryGrid`) resolves columns
from the `db` tag, so a field whose `db` and `ch` names differ is scanned from one column and
filtered or sorted on another.

**`dbx` over ClickHouse** (`dbx.NewRepository[T](client.Querier(), table)`). `dbx` selects and
inserts the columns named by the `db` tags, and clickhouse-go maps them back by the `ch` tags, so
both must name the same column. Give every field a `ch` tag equal to its `db` tag, or use only
`ch` tags:

```go
type Event struct {
    ID        uint64    `db:"id" ch:"id"`
    Timestamp time.Time `db:"created_at" ch:"created_at"`
    Data      string    `ch:"data"` // no db tag: the ch tag is the column for dbx too
}
```

`dbx.NewRepository` rejects a record that breaks this rule with `clickhouse.ErrRecordMapping`,
naming each offending column: `db:"id" ch:"event_id"` is rejected, and so is a field with only a
`db` tag whose Go name differs from the column (`Name string` with `db:"name"` is `Name` to
clickhouse-go). See [dbx: Record types need matching `ch` tags](dbx.md#record-types-need-matching-ch-tags).

## Query Behavior Tags

### `goqu` Tag

Controls query generation behavior for insert and update operations.

```go
type User struct {
    ID        int       `db:"id" goqu:"skipinsert"`     // Auto: never in INSERT or UPDATE
    CreatedAt time.Time `db:"created_at" goqu:"skipupdate"` // Also auto: never in INSERT or UPDATE
    Phone     string    `db:"phone" goqu:"omitempty"`   // Skip if empty string
    Address   *string   `db:"address" goqu:"omitnil"`   // Skip if nil pointer
}
```

**Available Options:**
- `skipinsert`, `skipupdate` - Either one marks the field auto, like `auto:"true"`: it is left out
  of both INSERT and UPDATE statements built by `db.Repository` (`Insert`, `UpdateRecord`, ...)
  and by `dbx`. Only goqu's own dataset builders (`SqlInsert()`/`SqlUpdate()` with `Rows`/`Set`
  of a struct) apply them separately.
- `omitempty` - Skip field if it has zero value (empty string, 0, false), in INSERT and record
  UPDATE
- `omitnil` - Skip field if it's nil (for pointer types), in INSERT and record UPDATE

Other `goqu` options are ignored by `db.Repository`'s builders and by `dbx`.

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

!!! warning
    The `mapper` tag is not read by `db`, `dbx` or `gohan`: it has no effect on how a field is
    stored or scanned. For a JSON column, use a type that implements `driver.Valuer` and
    `sql.Scanner`, such as [`jsoncol.JSON[T]`](../types/types.md#jsoncol).

```go
type User struct {
    // instead of `db:"preferences" mapper:"json"`, which does nothing:
    Preferences jsoncol.JSON[map[string]any] `db:"preferences"`
}
```

## Complete Tag Reference

### Tag Priority Order

When multiple tags define the same property, the priority is:
1. `alias` (explicit alias)
2. `json` (JSON field name)
3. `xml` (XML field name)
4. Struct field name (default)

### Field Processing Rules

1. **Database Field Name** (`db/field` and `gohan/field`: `db.Repository`, grids and `dbx`):
   - `db` tag
   - `ch` tag (only when there is no `db` tag)
   - Lower-cased struct field name (fallback)

   clickhouse-go, which scans and inserts ClickHouse rows, reads only the `ch` tag, then the
   struct field name as written; see [`ch` Tag](#ch-tag-clickhouse).

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
    *BaseModel  // Pointer embedding - not supported
    Name string `db:"name"`
}
```

**Note:** Pointer-to-struct embedding is not supported. `db/field` does not descend into it and
maps the pointer itself as one column (`basemodel`); `dbx.NewRepository` rejects the record with
`gohan.ErrRecordShape`. Embed the struct by value.

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
    CreatedBy int        `db:"created_by"` // skipupdate would also drop it from INSERT
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

Use the same column name in the `db` and `ch` tags. The struct then works with `db.Repository`,
the legacy ClickHouse repository and `dbx` on every database:

```go
type Event struct {
    ID        uint64    `db:"id" ch:"id"`
    Timestamp time.Time `db:"created_at" ch:"created_at"`
    UserID    uint64    `db:"user_id" ch:"user_id"`
    EventType string    `db:"event_type" ch:"event_type"`
    Data      string    `db:"data" ch:"data"`
}
```

Different names per database (`db:"id" ch:"event_id"`) are rejected by `dbx` on ClickHouse, and
in the legacy ClickHouse repository they make the grid filter and sort on the `db` column while
rows are read from the `ch` column (see [`ch` Tag](#ch-tag-clickhouse)).

### JSON/Complex Fields

```go
type User struct {
    ID          int                    `db:"id"`
    Name        string                 `db:"name"`
    Preferences jsoncol.JSON[map[string]any] `db:"preferences"`
    Tags        jsoncol.JSON[[]string]       `db:"tags"`
    Metadata    jsoncol.JSON[any]            `db:"metadata"`
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
    Config    MyStruct  `db:"config"`      // Named struct field - one column (only embedded structs are decomposed)
}
```

**Reserved Types:** only `time.Time` is reserved by default. A reserved type matters for embedded
fields: an embedded struct is decomposed into its fields unless its type is reserved, in which
case it is one column. A named (non-embedded) struct field such as `sql.NullString` or
`decimal.Decimal` is always one column and needs no registration. Register other types to embed
as single columns with `AddReservedType("pkg.Type")` (see the two registries below).

**Two separate registries.** `field.AddReservedType(name)` (this package, `db/field`) registers
a type with the legacy `db` package's own registry — it affects `db.Repository` and nothing
else. `dbx` reads struct metadata through the standalone `field` subpackage of
[`gohan`](gohan.md) instead (`github.com/oddbit-project/gohan/field`), which keeps its own,
separate reserved-type registry: `gohan/field.AddReservedType(name)` is what affects `dbx`.
Registering a type with one package's registry does not register it with the other's, and
`errors.Is` against `db/field.ErrInvalidStruct` does not match the equivalent error `dbx`/`gohan`
returns.

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
    
    // Test grid capabilities: sorting by id and name is accepted
    grid, err := db.NewGrid("users", user)
    assert.NoError(t, err)
    assert.NoError(t, grid.ValidQuery(&db.GridQuery{SortFields: map[string]string{"id": "asc", "name": "desc"}}))
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