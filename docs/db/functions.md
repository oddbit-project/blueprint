# Database Functions

The database functions module provides low-level database operations and utilities for advanced use cases. These functions offer direct SQL execution capabilities with intelligent result scanning and type detection.

## Overview

The functions module includes:

- Raw SQL execution functions
- Intelligent result scanning utilities
- Type detection and conversion helpers
- Context-aware database operations
- Error handling utilities

These functions are used internally by the Repository but are also available for direct use when you need more control over SQL operations.

## Raw Execution Functions

### RawExec

```go
func RawExec(ctx context.Context, conn sqlx.ExecerContext, sql string, args ...any) error
```

Executes a raw SQL statement that doesn't return rows (INSERT, UPDATE, DELETE, DDL).

**Example:**
```go
package main

import (
    "context"
    "github.com/oddbit-project/blueprint/db"
    "github.com/oddbit-project/blueprint/provider/pgsql"
    "log"
)

func main() {
    // Setup client
    config := pgsql.NewClientConfig()
    config.DSN = "postgres://user:pass@localhost/dbname?sslmode=disable"
    
    client, err := pgsql.NewClient(config)
    if err != nil {
        log.Fatal(err)
    }
    defer client.Disconnect()
    
    ctx := context.Background()
    
    // Create table
    createSQL := `
        CREATE TABLE IF NOT EXISTS users (
            id SERIAL PRIMARY KEY,
            name VARCHAR(100) NOT NULL,
            email VARCHAR(100) UNIQUE NOT NULL,
            created_at TIMESTAMP DEFAULT NOW()
        )`
    
    if err := db.RawExec(ctx, client.GetClient(), createSQL); err != nil {
        log.Fatal("Failed to create table:", err)
    }
    
    // Insert data
    insertSQL := "INSERT INTO users (name, email) VALUES ($1, $2)"
    if err := db.RawExec(ctx, client.GetClient(), insertSQL, "John Doe", "john@example.com"); err != nil {
        log.Fatal("Failed to insert user:", err)
    }
}
```

### RawInsert

```go
func RawInsert(ctx context.Context, conn sqlx.ExecerContext, qry string, values []any) error
```

Executes a raw INSERT statement with a slice of values.

**Example:**
```go
func batchInsertUsers(ctx context.Context, client db.Client, users []User) error {
    sql := "INSERT INTO users (name, email) VALUES "
    var values []any
    var placeholders []string
    
    for i, user := range users {
        placeholders = append(placeholders, fmt.Sprintf("($%d, $%d)", i*2+1, i*2+2))
        values = append(values, user.Name, user.Email)
    }
    
    sql += strings.Join(placeholders, ", ")
    return db.RawInsert(ctx, client.GetClient(), sql, values)
}
```

## Query and Fetch Functions

### FetchOne

```go
func FetchOne(ctx context.Context, conn sqlx.QueryerContext, qry *goqu.SelectDataset, target any) error
```

Fetches a single record using a goqu SelectDataset. The target must be a struct pointer.

**Example:**
```go
package main

import (
    "context"
    "github.com/doug-martin/goqu/v9"
    "github.com/oddbit-project/blueprint/db"
    "log"
)

type User struct {
    ID    int    `db:"id"`
    Name  string `db:"name"`
    Email string `db:"email"`
}

func getUserByID(ctx context.Context, client db.Client, userID int) (*User, error) {
    dialect := goqu.Dialect("postgres")
    query := dialect.From("users").Where(goqu.C("id").Eq(userID))
    
    user := &User{}
    err := db.FetchOne(ctx, client.GetClient(), query, user)
    if err != nil {
        return nil, err
    }
    
    return user, nil
}
```

### Fetch

```go
func Fetch(ctx context.Context, conn SqlxReaderCtx, qry *goqu.SelectDataset, target any) error
```

Fetches multiple records using a goqu SelectDataset. The target must be a slice pointer.

**Example:**
```go
func getActiveUsers(ctx context.Context, client db.Client) ([]*User, error) {
    dialect := goqu.Dialect("postgres")
    query := dialect.From("users").Where(goqu.C("active").IsTrue())
    
    var users []*User
    err := db.Fetch(ctx, client.GetClient(), query, &users)
    if err != nil {
        return nil, err
    }
    
    return users, nil
}
```

### FetchRecord

```go
func FetchRecord(ctx context.Context, conn sqlx.QueryerContext, qry *goqu.SelectDataset, fieldValues map[string]any, target any) error
```

Fetches a single record with WHERE clauses built from a field values map.

**Example:**
```go
func getUserByEmail(ctx context.Context, client db.Client, email string) (*User, error) {
    dialect := goqu.Dialect("postgres")
    query := dialect.From("users")
    
    fieldValues := map[string]any{
        "email": email,
        "active": true,
    }
    
    user := &User{}
    err := db.FetchRecord(ctx, client.GetClient(), query, fieldValues, user)
    if err != nil {
        return nil, err
    }
    
    return user, nil
}
```

### FetchByKey

```go
func FetchByKey(ctx context.Context, conn sqlx.QueryerContext, qry *goqu.SelectDataset, keyField string, value any, target any) error
```

Fetches a single record by a specific key field.

**Example:**
```go
func getUserByID(ctx context.Context, client db.Client, id int) (*User, error) {
    dialect := goqu.Dialect("postgres")
    query := dialect.From("users")
    
    user := &User{}
    err := db.FetchByKey(ctx, client.GetClient(), query, "id", id, user)
    if err != nil {
        return nil, err
    }
    
    return user, nil
}
```

### FetchWhere

```go
func FetchWhere(ctx context.Context, conn SqlxReaderCtx, qry *goqu.SelectDataset, fieldValues map[string]any, target any) error
```

Fetches multiple records with WHERE clauses from field values map.

**Example:**
```go
func getUsersByStatus(ctx context.Context, client db.Client, active bool, role string) ([]*User, error) {
    dialect := goqu.Dialect("postgres")
    query := dialect.From("users")
    
    fieldValues := map[string]any{
        "active": active,
        "role":   role,
    }
    
    var users []*User
    err := db.FetchWhere(ctx, client.GetClient(), query, fieldValues, &users)
    if err != nil {
        return nil, err
    }
    
    return users, nil
}
```

## Utility Functions

### Exists

```go
func Exists(ctx context.Context, conn sqlx.QueryerContext, qry *goqu.SelectDataset, fieldName string, fieldValue any, skip ...any) (bool, error)
```

Checks if records exist matching the given criteria. The optional skip parameter allows excluding specific records.

**Example:**
```go
func emailExists(ctx context.Context, client db.Client, email string, excludeID ...int) (bool, error) {
    dialect := goqu.Dialect("postgres")
    query := dialect.From("users")
    
    var skip []any
    if len(excludeID) > 0 {
        skip = []any{"id", excludeID[0]}
    }
    
    return db.Exists(ctx, client.GetClient(), query, "email", email, skip...)
}

func main() {
    // Check if email exists
    exists, err := emailExists(ctx, client, "john@example.com")
    if err != nil {
        log.Fatal(err)
    }
    
    if exists {
        log.Println("Email already exists")
    }
    
    // Check if email exists, excluding specific user ID
    exists, err = emailExists(ctx, client, "john@example.com", 123)
    if err != nil {
        log.Fatal(err)
    }
}
```

### Count

```go
func Count(ctx context.Context, conn sqlx.QueryerContext, qry *goqu.SelectDataset) (int64, error)
```

Executes a COUNT query and returns the result.

**Example:**
```go
func countActiveUsers(ctx context.Context, client db.Client) (int64, error) {
    dialect := goqu.Dialect("postgres")
    query := dialect.From("users").
        Select(goqu.L("COUNT(*)")).
        Where(goqu.C("active").IsTrue())
    
    return db.Count(ctx, client.GetClient(), query)
}
```

## Delete Functions

### Delete

```go
func Delete(ctx context.Context, conn sqlx.ExecerContext, qry *goqu.DeleteDataset) error
```

Executes a DELETE query using a goqu DeleteDataset.

**Example:**
```go
func deleteInactiveUsers(ctx context.Context, client db.Client, daysInactive int) error {
    dialect := goqu.Dialect("postgres")
    cutoff := time.Now().AddDate(0, 0, -daysInactive)
    
    query := dialect.Delete("users").
        Where(goqu.C("last_login").Lt(cutoff)).
        Where(goqu.C("active").IsFalse())
    
    return db.Delete(ctx, client.GetClient(), query)
}
```

### DeleteWhere

```go
func DeleteWhere(ctx context.Context, conn sqlx.ExecerContext, qry *goqu.DeleteDataset, fieldNameValue map[string]any) error
```

Deletes records matching field values.

**Example:**
```go
func deleteUsersByRole(ctx context.Context, client db.Client, role string) error {
    dialect := goqu.Dialect("postgres")
    query := dialect.Delete("users")
    
    fieldValues := map[string]any{
        "role":   role,
        "active": false,
    }
    
    return db.DeleteWhere(ctx, client.GetClient(), query, fieldValues)
}
```

### DeleteByKey

```go
func DeleteByKey(ctx context.Context, conn sqlx.ExecerContext, qry *goqu.DeleteDataset, keyField string, value any) error
```

Deletes a record by key field.

**Example:**
```go
func deleteUserByID(ctx context.Context, client db.Client, userID int) error {
    dialect := goqu.Dialect("postgres")
    query := dialect.Delete("users")
    
    return db.DeleteByKey(ctx, client.GetClient(), query, "id", userID)
}
```

## RETURNING Clause Support

### RawInsertReturning

```go
func RawInsertReturning(ctx context.Context, conn sqlx.QueryerContext, qry string, values []any, target ...any) error
```

Executes an INSERT with RETURNING clause for positional scanning.

**Example:**
```go
func insertUserReturning(ctx context.Context, client db.Client, name, email string) (int, error) {
    sql := "INSERT INTO users (name, email) VALUES ($1, $2) RETURNING id"
    values := []any{name, email}
    
    var id int
    err := db.RawInsertReturning(ctx, client.GetClient(), sql, values, &id)
    if err != nil {
        return 0, err
    }
    
    return id, nil
}
```

### RawInsertReturningFlexible

```go
func RawInsertReturningFlexible(ctx context.Context, conn sqlx.QueryerContext, sql string, args []any, target any) error
```

Executes INSERT with RETURNING clause using intelligent type detection for scanning.

**Example:**
```go
func insertUserReturningStruct(ctx context.Context, client db.Client, user *User) error {
    sql := `INSERT INTO users (name, email) VALUES ($1, $2) 
            RETURNING id, name, email, created_at`
    args := []any{user.Name, user.Email}
    
    // Scan directly into struct - fields are mapped by name/tag
    return db.RawInsertReturningFlexible(ctx, client.GetClient(), sql, args, user)
}

func insertUserReturningFields(ctx context.Context, client db.Client, name, email string) (int, string, time.Time, error) {
    sql := `INSERT INTO users (name, email) VALUES ($1, $2) 
            RETURNING id, name, created_at`
    args := []any{name, email}
    
    var id int
    var returnedName string
    var createdAt time.Time
    
    // Scan into multiple variables positionally
    targets := []any{&id, &returnedName, &createdAt}
    err := db.RawInsertReturningFlexible(ctx, client.GetClient(), sql, args, targets)
    
    return id, returnedName, createdAt, err
}
```

## Update Functions

### Update

```go
func Update(ctx context.Context, conn sqlx.ExecerContext, qry *goqu.UpdateDataset) error
```

Executes an UPDATE query using goqu UpdateDataset.

**Example:**
```go
func updateUserEmail(ctx context.Context, client db.Client, userID int, newEmail string) error {
    dialect := goqu.Dialect("postgres")
    query := dialect.Update("users").
        Set(goqu.Record{"email": newEmail, "updated_at": time.Now()}).
        Where(goqu.C("id").Eq(userID))
    
    return db.Update(ctx, client.GetClient(), query)
}
```

### RawUpdateReturningFlexible

```go
func RawUpdateReturningFlexible(ctx context.Context, conn sqlx.QueryerContext, sql string, args []any, target any) error
```

Executes UPDATE with RETURNING clause using intelligent scanning.

**Example:**
```go
func updateUserReturning(ctx context.Context, client db.Client, userID int, name string) (*User, error) {
    sql := `UPDATE users SET name = $1, updated_at = NOW() 
            WHERE id = $2 
            RETURNING id, name, email, updated_at`
    args := []any{name, userID}
    
    user := &User{}
    err := db.RawUpdateReturningFlexible(ctx, client.GetClient(), sql, args, user)
    if err != nil {
        return nil, err
    }
    
    return user, nil
}
```

## Type Detection and Scanning

The scanning functions use intelligent type detection to handle different target types:

### Struct Scanning
When the target is a struct pointer, fields are mapped by name or struct tags:

```go
type User struct {
    ID    int    `db:"id"`
    Name  string `db:"name"`
    Email string `db:"email"`
}

user := &User{}
// Automatically maps database columns to struct fields
err := db.RawInsertReturningFlexible(ctx, conn, sql, args, user)
```

### Positional Scanning
When the target is a slice of interfaces, values are scanned positionally:

```go
targets := []any{&id, &name, &email}
err := db.RawInsertReturningFlexible(ctx, conn, sql, args, targets)
```

### Single Value Scanning
When the target is a single variable pointer:

```go
var id int
err := db.RawInsertReturningFlexible(ctx, conn, sql, args, &id)
```

## Error Handling

### EmptyResult

```go
func EmptyResult(err error) bool
```

Checks if an error indicates no rows were found.

**Example:**
```go
func getUserSafely(ctx context.Context, client db.Client, userID int) (*User, error) {
    user := &User{}
    err := db.FetchByKey(ctx, client.GetClient(), query, "id", userID, user)
    
    if err != nil {
        if db.EmptyResult(err) {
            return nil, nil // User not found, not an error
        }
        return nil, err // Real error
    }
    
    return user, nil
}
```

## Advanced Usage Patterns

### Transaction Support

All functions work with both regular connections and transactions:

```go
func transferFunds(ctx context.Context, client db.Client, fromID, toID int, amount decimal.Decimal) error {
    tx, err := client.GetClient().BeginTxx(ctx, nil)
    if err != nil {
        return err
    }
    defer tx.Rollback()
    
    // Debit from account
    debitSQL := "UPDATE accounts SET balance = balance - $1 WHERE id = $2"
    if err := db.RawExec(ctx, tx, debitSQL, amount, fromID); err != nil {
        return err
    }
    
    // Credit to account
    creditSQL := "UPDATE accounts SET balance = balance + $1 WHERE id = $2"
    if err := db.RawExec(ctx, tx, creditSQL, amount, toID); err != nil {
        return err
    }
    
    return tx.Commit()
}
```

### Batch Operations

```go
func batchUpdateUsers(ctx context.Context, client db.Client, updates []UserUpdate) error {
    tx, err := client.GetClient().BeginTxx(ctx, nil)
    if err != nil {
        return err
    }
    defer tx.Rollback()
    
    updateSQL := "UPDATE users SET name = $1, email = $2 WHERE id = $3"
    
    for _, update := range updates {
        if err := db.RawExec(ctx, tx, updateSQL, update.Name, update.Email, update.ID); err != nil {
            return fmt.Errorf("failed to update user %d: %w", update.ID, err)
        }
    }
    
    return tx.Commit()
}
```

## Performance Considerations

### Connection Reuse
- Functions accept connection interfaces, allowing reuse across operations
- Use transactions for multiple related operations
- Avoid creating new connections for each operation

### Context Usage
- Always pass contexts for proper cancellation and timeout handling
- Use context.WithTimeout() for operations with time limits
- Respect context cancellation in loops and batch operations

### Error Handling
- Check for specific error types (EmptyResult, constraint violations)
- Use appropriate error handling for your use case
- Log errors with sufficient context for debugging

## Best Practices

1. **Use Contexts**: Always pass contexts for cancellation and timeout support
2. **Handle Empty Results**: Use EmptyResult() to distinguish between no data and errors
3. **Use Transactions**: Group related operations in transactions for consistency
4. **Type Safety**: Use struct scanning when possible for better type safety
5. **Error Context**: Provide meaningful error context in your functions
6. **Resource Cleanup**: Ensure rows are closed and connections are managed properly

## See Also

- [Repository Documentation](repository.md)
- [Query Builder Documentation](query-builder.md)
- [Client Documentation](client.md)
- [Database Package Overview](index.md)