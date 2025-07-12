# Query Builder

The Query Builder (qb) package provides a powerful SQL generation system with dialect abstraction, type-safe query construction, and advanced features like RETURNING clauses. It serves as the foundation for Repository operations and can be used directly for complex query construction.

## Overview

The Query Builder system includes:

- SQL dialect abstraction for database portability
- Type-safe query construction with struct integration
- Advanced UPDATE operations with flexible options
- RETURNING clause support for INSERT and UPDATE
- Integration with field metadata for automatic mapping
- Batch operation support

## Core Components

### SqlDialect Interface

```go
type SqlDialect interface {
    Name() string
    Quote(identifier string) string
    Placeholder(position int) string
}
```

The SqlDialect interface abstracts database-specific SQL generation:

- **Name()**: Returns the dialect name (e.g., "postgres", "mysql")
- **Quote()**: Quotes identifiers for the target database
- **Placeholder()**: Generates parameter placeholders ($1, ?, etc.)

### SqlBuilder

```go
type SqlBuilder struct {
    dialect SqlDialect
}

func NewSqlBuilder(dialect SqlDialect) *SqlBuilder
```

The main query builder that coordinates SQL generation using the specified dialect.

**Example:**
```go
package main

import (
    "github.com/oddbit-project/blueprint/db/qb"
    "log"
)

func main() {
    // Create builder with PostgreSQL dialect
    dialect := qb.NewPostgreSqlDialect()
    builder := qb.NewSqlBuilder(dialect)
    
    // Builder is ready for query generation
    log.Printf("Using dialect: %s", builder.Dialect().Name())
}
```

## UpdateBuilder

The UpdateBuilder provides advanced UPDATE query construction with flexible options and RETURNING support.

### Basic Update Operations

```go
type User struct {
    ID        int       `db:"id" goqu:"skipupdate"`
    Name      string    `db:"name"`
    Email     string    `db:"email"`
    UpdatedAt time.Time `db:"updated_at"`
}

func updateUser(builder *qb.SqlBuilder, user *User, userID int) error {
    updateBuilder := builder.Update("users", user).
        WithOptions(qb.DefaultUpdateOptions()).
        Where(qb.Eq("id", userID))
    
    sql, args, err := updateBuilder.Build()
    if err != nil {
        return err
    }
    
    log.Printf("SQL: %s", sql)
    log.Printf("Args: %v", args)
    
    // Execute with your database connection
    return nil
}
```

### Update with Field Selection

```go
func updateUserEmail(builder *qb.SqlBuilder, userID int, email string) error {
    user := &User{
        Email:     email,
        UpdatedAt: time.Now(),
    }
    
    options := &qb.UpdateOptions{
        IncludeFields: []string{"email", "updated_at"},
    }
    
    updateBuilder := builder.Update("users", user).
        WithOptions(options).
        Where(qb.Eq("id", userID))
    
    sql, args, err := updateBuilder.Build()
    if err != nil {
        return err
    }
    
    // SQL: UPDATE users SET email = $1, updated_at = $2 WHERE id = $3
    return executeSQL(sql, args)
}
```

### Update with Field Values Map

```go
func updateUserFields(builder *qb.SqlBuilder, userID int, updates map[string]any) error {
    user := &User{}
    
    updateBuilder := builder.Update("users", user).
        WithOptions(qb.DefaultUpdateOptions()).
        FieldsValues(updates).
        Where(qb.Eq("id", userID))
    
    sql, args, err := updateBuilder.Build()
    if err != nil {
        return err
    }
    
    return executeSQL(sql, args)
}

func main() {
    updates := map[string]any{
        "name":       "John Updated",
        "email":      "john.updated@example.com",
        "updated_at": time.Now(),
    }
    
    err := updateUserFields(builder, 123, updates)
    if err != nil {
        log.Fatal(err)
    }
}
```

## UpdateOptions

The UpdateOptions struct provides fine-grained control over UPDATE query generation:

```go
type UpdateOptions struct {
    IncludeFields     []string
    ExcludeFields     []string
    IncludeZeroValues bool
    UpdateAutoFields  bool
    ReturningFields   []string
}
```

### Field Inclusion/Exclusion

```go
func updateUserSelective(builder *qb.SqlBuilder, user *User, userID int) error {
    options := &qb.UpdateOptions{
        // Only update these fields
        IncludeFields: []string{"name", "email"},
        // Never update these fields
        ExcludeFields: []string{"created_at", "id"},
        // Include zero values (empty strings, 0, false)
        IncludeZeroValues: true,
    }
    
    updateBuilder := builder.Update("users", user).
        WithOptions(options).
        Where(qb.Eq("id", userID))
    
    sql, args, err := updateBuilder.Build()
    return executeSQL(sql, args)
}
```

### Auto Field Handling

```go
type User struct {
    ID        int       `db:"id" goqu:"skipupdate"`        // Never updated
    Name      string    `db:"name"`
    Email     string    `db:"email"`
    CreatedAt time.Time `db:"created_at" goqu:"skipupdate"` // Never updated
    UpdatedAt time.Time `db:"updated_at" auto:"true"`       // Auto field
}

func updateWithAutoFields(builder *qb.SqlBuilder, user *User, userID int) error {
    options := &qb.UpdateOptions{
        // Update auto fields (updated_at will be set to current time)
        UpdateAutoFields: true,
    }
    
    updateBuilder := builder.Update("users", user).
        WithOptions(options).
        Where(qb.Eq("id", userID))
    
    sql, args, err := updateBuilder.Build()
    return executeSQL(sql, args)
}
```

## RETURNING Clause Support

### Update with RETURNING

```go
func updateUserReturning(builder *qb.SqlBuilder, user *User, userID int) (*User, error) {
    options := &qb.UpdateOptions{
        ReturningFields: []string{"id", "name", "email", "updated_at"},
    }
    
    updateBuilder := builder.Update("users", user).
        WithOptions(options).
        Where(qb.Eq("id", userID))
    
    sql, args, err := updateBuilder.Build()
    if err != nil {
        return nil, err
    }
    
    // Execute and scan result back into struct
    result := &User{}
    err = executeReturning(sql, args, result)
    if err != nil {
        return nil, err
    }
    
    return result, nil
}
```

### Insert with RETURNING

```go
func insertUserReturning(builder *qb.SqlBuilder, user *User) (*User, error) {
    returnFields := []string{"id", "name", "email", "created_at"}
    
    sql, args, err := builder.InsertReturning("users", user, returnFields)
    if err != nil {
        return nil, err
    }
    
    result := &User{}
    err = executeReturning(sql, args, result)
    if err != nil {
        return nil, err
    }
    
    return result, nil
}
```

## WHERE Clause Construction

The query builder provides a fluent interface for building WHERE clauses:

### Basic WHERE Conditions

```go
func buildWhereConditions(builder *qb.SqlBuilder) error {
    updateBuilder := builder.Update("users", &User{}).
        Where(qb.Eq("active", true)).
        Where(qb.Gt("age", 18)).
        Where(qb.Like("name", "John%"))
    
    sql, args, err := updateBuilder.Build()
    // SQL: UPDATE users SET ... WHERE active = $1 AND age > $2 AND name LIKE $3
    
    return executeSQL(sql, args)
}
```

### Complex WHERE Conditions

```go
func buildComplexWhere(builder *qb.SqlBuilder) error {
    updateBuilder := builder.Update("users", &User{}).
        WhereAnd(
            qb.Eq("department", "engineering"),
            qb.Or(
                qb.Eq("role", "senior"),
                qb.Gt("experience_years", 5),
            ),
        )
    
    sql, args, err := updateBuilder.Build()
    // SQL: UPDATE users SET ... WHERE (department = $1 AND (role = $2 OR experience_years > $3))
    
    return executeSQL(sql, args)
}
```

### WHERE Clause Helpers

```go
// Common WHERE clause constructors
func whereExamples() {
    // Equality
    condition1 := qb.Eq("status", "active")
    
    // Comparison
    condition2 := qb.Gt("age", 21)
    condition3 := qb.Lt("score", 100)
    condition4 := qb.Gte("rating", 4.0)
    condition5 := qb.Lte("price", 99.99)
    
    // Pattern matching
    condition6 := qb.Like("name", "John%")
    condition7 := qb.ILike("email", "%@EXAMPLE.COM") // Case-insensitive
    
    // NULL checks
    condition8 := qb.IsNull("deleted_at")
    condition9 := qb.IsNotNull("confirmed_at")
    
    // IN clauses
    condition10 := qb.In("status", []any{"active", "pending"})
    condition11 := qb.NotIn("role", []any{"admin", "super_admin"})
}
```

## Batch Operations

### Batch Insert

```go
func batchInsertUsers(builder *qb.SqlBuilder, users []any) error {
    sql, args, err := builder.BuildSQLBatchInsert("users", users)
    if err != nil {
        return err
    }
    
    // Execute batch insert
    return executeBatch(sql, args)
}

func main() {
    users := []any{
        &User{Name: "John", Email: "john@example.com"},
        &User{Name: "Jane", Email: "jane@example.com"},
        &User{Name: "Bob", Email: "bob@example.com"},
    }
    
    err := batchInsertUsers(builder, users)
    if err != nil {
        log.Fatal(err)
    }
}
```

### Batch Update

```go
func batchUpdateUsers(builder *qb.SqlBuilder, users []any) error {
    keyFields := []string{"id"}
    options := qb.DefaultUpdateOptions()
    
    statements, argsList, err := builder.BuildSQLBatchUpdate("users", users, keyFields, options)
    if err != nil {
        return err
    }
    
    // Execute each update in a transaction
    return executeBatchInTransaction(statements, argsList)
}
```

## Integration with Repository

The Query Builder integrates seamlessly with the Repository pattern:

### Repository UpdateX Method

```go
func updateUserWithBuilder(repo db.Repository, user *User, userID int) error {
    updateBuilder := repo.SqlUpdateX(user).
        Where(qb.Eq("id", userID))
    
    // Execute through Repository
    return repo.Do(updateBuilder)
}
```

### Custom Query Building

```go
func customUserQuery(repo db.Repository) error {
    builder := repo.SqlBuilder()
    
    // Build complex update
    updateBuilder := builder.Update("users", &User{}).
        FieldsValues(map[string]any{
            "last_login": time.Now(),
            "login_count": qb.Raw("login_count + 1"),
        }).
        Where(qb.Eq("id", 123))
    
    return repo.Do(updateBuilder)
}
```

## Advanced Features

### Raw SQL Expressions

```go
func useRawExpressions(builder *qb.SqlBuilder) error {
    updateBuilder := builder.Update("users", &User{}).
        FieldsValues(map[string]any{
            "score": qb.Raw("GREATEST(score, $1)", 100),
            "updated_at": qb.Raw("NOW()"),
            "rank": qb.Raw("rank + 1"),
        }).
        Where(qb.Eq("active", true))
    
    sql, args, err := updateBuilder.Build()
    return executeSQL(sql, args)
}
```

### Subqueries

```go
func updateWithSubquery(builder *qb.SqlBuilder) error {
    // Subquery to get average score
    avgSubquery := builder.SqlBuilder().
        Select("AVG(score)").
        From("users").
        Where(qb.Eq("department", "engineering"))
    
    updateBuilder := builder.Update("users", &User{}).
        FieldsValues(map[string]any{
            "performance_rating": qb.Subquery(avgSubquery),
        }).
        Where(qb.Eq("id", 123))
    
    sql, args, err := updateBuilder.Build()
    return executeSQL(sql, args)
}
```

## Error Handling

### Validation Errors

```go
func handleValidationErrors(builder *qb.SqlBuilder) {
    updateBuilder := builder.Update("users", &User{})
    
    sql, args, err := updateBuilder.Build()
    if err != nil {
        switch {
        case errors.Is(err, qb.ErrNoFieldsToUpdate):
            log.Println("No fields specified for update")
        case errors.Is(err, qb.ErrInvalidWhereClause):
            log.Println("Invalid WHERE clause")
        default:
            log.Printf("Build error: %v", err)
        }
        return
    }
    
    executeSQL(sql, args)
}
```

### Field Validation

```go
func validateFields(builder *qb.SqlBuilder, user *User) error {
    // Validate required fields before building query
    if user.Name == "" {
        return errors.New("name is required")
    }
    
    if user.Email == "" {
        return errors.New("email is required")
    }
    
    updateBuilder := builder.Update("users", user).
        Where(qb.Eq("id", user.ID))
    
    sql, args, err := updateBuilder.Build()
    if err != nil {
        return fmt.Errorf("failed to build update query: %w", err)
    }
    
    return executeSQL(sql, args)
}
```

## Performance Considerations

### Query Preparation

```go
func optimizeQueryBuilding(builder *qb.SqlBuilder) {
    // Build query once, reuse multiple times
    updateBuilder := builder.Update("users", &User{}).
        WithOptions(&qb.UpdateOptions{
            IncludeFields: []string{"name", "email", "updated_at"},
        }).
        Where(qb.Eq("id", qb.Placeholder(1)))
    
    baseSQL, _, err := updateBuilder.Build()
    if err != nil {
        log.Fatal(err)
    }
    
    // Reuse prepared statement structure
    for _, user := range users {
        args := []any{user.Name, user.Email, time.Now(), user.ID}
        executeSQL(baseSQL, args)
    }
}
```

### Batch Processing

```go
func efficientBatchUpdate(builder *qb.SqlBuilder, users []*User) error {
    const batchSize = 100
    
    for i := 0; i < len(users); i += batchSize {
        end := i + batchSize
        if end > len(users) {
            end = len(users)
        }
        
        batch := make([]any, end-i)
        for j, user := range users[i:end] {
            batch[j] = user
        }
        
        if err := batchUpdateUsers(builder, batch); err != nil {
            return fmt.Errorf("batch %d failed: %w", i/batchSize, err)
        }
    }
    
    return nil
}
```

## Best Practices

### Query Construction
1. **Use Options**: Leverage UpdateOptions for flexible field control
2. **Validate Input**: Check required fields before building queries
3. **Handle Zero Values**: Use IncludeZeroValues appropriately
4. **Use RETURNING**: Fetch updated data efficiently with RETURNING clauses

### Performance
1. **Batch Operations**: Use batch methods for multiple records
2. **Prepare Queries**: Reuse query structures when possible
3. **Limit Fields**: Only update necessary fields
4. **Use Transactions**: Group related operations

### Error Handling
1. **Check Build Errors**: Always handle query building errors
2. **Validate Data**: Verify data before query construction
3. **Use Contexts**: Include context in all database operations
4. **Log Queries**: Log generated SQL for debugging

### Security
1. **Use Parameters**: Never concatenate user input into SQL
2. **Validate WHERE Clauses**: Ensure proper WHERE conditions
3. **Escape Identifiers**: Use proper identifier quoting
4. **Limit Operations**: Include appropriate WHERE clauses

## Integration Examples

### With HTTP Handlers

```go
func updateUserHandler(w http.ResponseWriter, r *http.Request) {
    userID, _ := strconv.Atoi(mux.Vars(r)["id"])
    
    var updateReq struct {
        Name  string `json:"name"`
        Email string `json:"email"`
    }
    
    if err := json.NewDecoder(r.Body).Decode(&updateReq); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }
    
    updates := map[string]any{
        "name":       updateReq.Name,
        "email":      updateReq.Email,
        "updated_at": time.Now(),
    }
    
    options := &qb.UpdateOptions{
        ReturningFields: []string{"id", "name", "email", "updated_at"},
    }
    
    updateBuilder := builder.Update("users", &User{}).
        WithOptions(options).
        FieldsValues(updates).
        Where(qb.Eq("id", userID))
    
    sql, args, err := updateBuilder.Build()
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    user := &User{}
    if err := executeReturning(sql, args, user); err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(user)
}
```

## See Also

- [Repository Documentation](repository.md)
- [Database Functions](functions.md)
- [Field Metadata](fields.md)
- [Database Package Overview](index.md)