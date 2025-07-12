# UPDATE API Examples

This document shows examples of using the improved UPDATE API with pointer parameters instead of variadic options.

All examples use the `github.com/oddbit-project/blueprint/db/sqlbuilder` package.

## Basic UPDATE Operations

### Simple UPDATE with default options

```go
import "github.com/oddbit-project/blueprint/db/sqlbuilder"

builder := sqlbuilder.NewSqlBuilder(sqlbuilder.DefaultSqlDialect())

user := User{
    Name:     "John Doe",
    Email:    "john@example.com",
    Age:      30,
    IsActive: true,
}

whereConditions := []WhereCondition{
    {Field: "id", Operator: "=", Value: 1},
}

// Use nil for default options
sql, args, err := builder.BuildSQLUpdate("users", user, whereConditions, nil)
```

### UPDATE with custom options

```go
options := &UpdateOptions{
    IncludeFields: []string{"Name", "Email"},
    IncludeZeroValues: true,
}

sql, args, err := builder.BuildSQLUpdate("users", user, whereConditions, options)
```

### UPDATE by ID with default options

```go
// Use nil for default options
sql, args, err := builder.BuildSQLUpdateByID("users", user, 123, nil)
```

### UPDATE by ID with custom options

```go
options := &UpdateOptions{
    ExcludeFields: []string{"CreatedAt", "UpdatedAt"},
}

sql, args, err := builder.BuildSQLUpdateByID("users", user, 123, options)
```

## Batch UPDATE Operations

### Batch UPDATE with default options

```go
users := []any{
    User{ID: 1, Name: "User 1", Email: "user1@example.com"},
    User{ID: 2, Name: "User 2", Email: "user2@example.com"},
    User{ID: 3, Name: "User 3", Email: "user3@example.com"},
}

// Use nil for default options
statements, argsList, err := builder.BuildSQLBatchUpdate("users", users, []string{"ID"}, nil)
```

### Batch UPDATE with custom options

```go
options := &UpdateOptions{
    IncludeFields: []string{"Name", "Email"},
    IncludeZeroValues: true,
}

statements, argsList, err := builder.BuildSQLBatchUpdate("users", users, []string{"ID"}, options)
```

## Advanced Examples

### UPDATE with field exclusion

```go
options := &UpdateOptions{
    ExcludeFields: []string{"CreatedAt", "UpdatedAt", "ID"},
}

sql, args, err := builder.BuildSQLUpdate("users", user, whereConditions, options)
```

### UPDATE with zero value inclusion

```go
options := &UpdateOptions{
    IncludeZeroValues: true,
}

sql, args, err := builder.BuildSQLUpdate("users", user, whereConditions, options)
```

### UPDATE with auto field updates

```go
options := &UpdateOptions{
    UpdateAutoFields: true,
    IncludeFields:    []string{"Name", "Email", "UpdatedAt"},
}

sql, args, err := builder.BuildSQLUpdate("users", user, whereConditions, options)
```

## Benefits of Pointer Parameters

1. **Clearer Intent**: `nil` clearly indicates default options
2. **No Ambiguity**: No confusion about multiple options
3. **Memory Efficient**: Options are passed by reference
4. **Go Idiomatic**: Follows Go conventions for optional parameters
5. **Type Safety**: Compile-time checking of parameter types

## Migration from Variadic Parameters

### Before (Variadic)
```go
// Multiple ways to call, confusing
builder.BuildSQLUpdate("users", user, whereConditions) // no options
builder.BuildSQLUpdate("users", user, whereConditions, options) // with options
builder.BuildSQLUpdate("users", user, whereConditions, DefaultUpdateOptions()) // explicit default
```

### After (Pointer)
```go
// Clear, consistent API
builder.BuildSQLUpdate("users", user, whereConditions, nil) // default options
builder.BuildSQLUpdate("users", user, whereConditions, &options) // custom options
```

## See Also

- [Database Package Overview](index.md)
- [Query Builder Documentation](query-builder.md)
- [Repository Documentation](repository.md)
- [Database Functions](functions.md)
- [Field Specifications](fields.md)