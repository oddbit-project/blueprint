# db.FieldSpec

Field specification component for mapping struct fields to database columns with extended functionality, used
in the Grid component.

## Overview

The FieldSpec component provides a way to extract and map information from struct field tags to facilitate database operations.
It's particularly useful for:

- Mapping struct fields to database columns
- Creating aliases for fields
- Defining which fields can be sorted, filtered, or searched
- Supporting Grid functionality for dynamic query building

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
    // Create a FieldSpec from the struct
    spec, err := db.NewFieldSpec(&UserRecord{})
    if err != nil {
        log.Fatal(err)
    }
    
    // Use the spec to look up field information
    dbField, exists := spec.LookupAlias("username")
    if exists {
        fmt.Println("DB field for 'username':", dbField)
    }
    
    // Get all sortable fields
    sortFields := spec.SortFields()
    fmt.Println("Sortable fields:", sortFields)
    
    // Get all searchable fields
    searchFields := spec.SearchFields()
    fmt.Println("Searchable fields:", searchFields)
    
    // Get all filterable fields
    filterFields := spec.FilterFields()
    fmt.Println("Filterable fields:", filterFields)
}
```

### Manual Field Specification

You can also create an empty FieldSpec and add fields manually:

```go
// Create an empty spec
spec := db.NewEmptyFieldSpec()

// Add fields with their properties
spec.AddField("id", "ID", false, true, true)           // id -> ID (sortable, filterable)
spec.AddField("username", "Username", true, true, true) // username -> Username (searchable, sortable, filterable)
spec.AddField("email", "Email", true, false, true)      // email -> Email (searchable, filterable)
spec.AddField("status", "Status", false, false, true)   // status -> Status (filterable only)

// Use the spec as needed
```

## Component Reference

### Constants and Error Types

```go
// Error constants
ErrDuplicatedAlias  = utils.Error("alias already exists")
ErrDuplicatedField  = utils.Error("field already exists")
ErrInvalidStructPtr = utils.Error("field spec requires a pointer to a struct")
ErrNilPointer       = utils.Error("ptr to struct to be parsed by field spec is nil")
ErrInvalidStruct    = utils.Error("field spec requires a pointer to a struct; invalid type")

// Tag constants
tagGrid   = "grid"
optSort   = "sort"
optSearch = "search"
optFilter = "filter"
```

### Types

#### FieldSpec

```go
type FieldSpec struct {
    fieldAlias   map[string]string // maps db fields to alias
    aliasField   map[string]string // maps alias to db fields
    sortFields   []string          // sortable db fields
    filterFields []string          // filterable db fields
    searchFields []string          // searchable db fields
}
```

The main FieldSpec component that handles field mapping and property management.

### Functions

#### NewFieldSpec

```go
func NewFieldSpec(from any) (*FieldSpec, error)
```

Creates a new FieldSpec from a struct, scanning its field tags to populate the maps and lists.

#### NewEmptyFieldSpec

```go
func NewEmptyFieldSpec() *FieldSpec
```

Creates a new empty FieldSpec with initialized maps and lists.

### FieldSpec Methods

#### AddField

```go
func (f *FieldSpec) AddField(dbField, alias string, searchable bool, sortable bool, filterable bool) error
```

Adds a field to the specification with its properties.

#### LookupAlias

```go
func (f *FieldSpec) LookupAlias(alias string) (string, bool)
```

Looks up an alias and returns the corresponding database field name.

#### FieldAliasMap

```go
func (f *FieldSpec) FieldAliasMap() map[string]string
```

Returns a copy of the map that maps database fields to aliases.

#### FilterFields

```go
func (f *FieldSpec) FilterFields() []string
```

Returns the list of filterable database fields.

#### SortFields

```go
func (f *FieldSpec) SortFields() []string
```

Returns the list of sortable database fields.

#### SearchFields

```go
func (f *FieldSpec) SearchFields() []string
```

Returns the list of searchable database fields.

## Struct Tags

The FieldSpec component processes several struct tags:

### Database Field Tags

- `db`: Standard database field tag (primary)
- `ch`: Alternative database field tag (for ClickHouse)

### Alias Tags

- `json`: Used as alias if available
- `alias`: Explicitly defined alias

### Grid Tags

The `grid` tag can contain comma-separated options:
- `sort`: Field can be used for sorting
- `search`: Field is included in text searches
- `filter`: Field can be used in filters

Example:
```go
type User struct {
    ID      int    `db:"id" json:"id" grid:"sort,filter"`
    Name    string `db:"name" json:"name" grid:"sort,search,filter"`
    Email   string `db:"email" json:"email" grid:"search,filter"`
    IsAdmin bool   `db:"is_admin" json:"isAdmin" grid:"filter"`
}
```

## Field Processing Rules

1. Database field name comes from the `db` or `ch` tag, falling back to the struct field name
2. Alias comes from the `alias` or `json` tag, falling back to the struct field name
3. Options (searchable, sortable, filterable) come from the `grid` tag
4. Fields marked with `db:"-"` are ignored
5. Unexported fields are ignored
6. Embedded structs are processed recursively, including their fields in the spec
7. Embedded pointer-to-struct fields are skipped

## Field Mapping Process

1. Create a new FieldSpec with `NewFieldSpec(&MyStruct{})`
2. FieldSpec scans all fields in the struct and processes their tags
3. For each field:
   - Extract the database field name from db tags
   - Extract the alias from json/alias tags
   - Extract grid options from grid tags
   - Add the field to the appropriate maps and lists
4. If a field is an embedded struct, recursively process its fields
5. Validation ensures no duplicate fields or aliases

## Examples

### Processing Struct with Embedded Fields

```go
type BaseRecord struct {
    ID        int       `db:"id" json:"id" grid:"sort,filter"`
    CreatedAt time.Time `db:"created_at" json:"createdAt" grid:"sort"`
    UpdatedAt time.Time `db:"updated_at" json:"updatedAt" grid:"sort"`
}

type UserRecord struct {
    BaseRecord         // Embedded struct - fields will be included
    Username  string   `db:"username" json:"username" grid:"sort,search,filter"`
    Email     string   `db:"email" json:"email" grid:"sort,search,filter"`
}

// Create a FieldSpec from the struct
spec, _ := db.NewFieldSpec(&UserRecord{})

// All fields from both structs are included
fmt.Println(spec.SortFields()) 
// Output: [id created_at updated_at username email]

fmt.Println(spec.SearchFields())
// Output: [username email]
```

### Custom Field Mapping

```go
// Create an empty spec
spec := db.NewEmptyFieldSpec()

// Add fields with aliases different from DB names
spec.AddField("u_id", "userId", false, true, true)
spec.AddField("u_name", "userName", true, true, true)
spec.AddField("u_email", "userEmail", true, false, true)

// Look up using aliases
dbField, _ := spec.LookupAlias("userName")
fmt.Println(dbField) // Output: u_name

// Get the full mapping
mapping := spec.FieldAliasMap()
fmt.Println(mapping) // Output: map[u_id:userId u_name:userName u_email:userEmail]
```

## See Also

- [Grid Documentation](dbgrid.md)
- [Repository Documentation](repository.md)