# Request Validation

Blueprint provides a powerful two-stage validation system for HTTP requests that combines automatic binding validation with custom business logic validation. The validation system supports nested structures, custom validators, and provides detailed error reporting with full field paths.

## Overview

The validation system works in two stages:

1. **Binding Validation**: Validates using `binding` tags (required, email, min, max, etc.)
2. **Custom Validation**: Validates using the `Validator` interface for complex business logic

Both `ValidateJSON()` and `ValidateQuery()` functions follow this pattern and automatically return standardized error responses.

## ValidateJSON - JSON Request Validation

Validates incoming JSON request bodies against struct validation tags and custom validation logic.

### Basic Usage

```go
import (
    "github.com/gin-gonic/gin"
    "github.com/oddbit-project/blueprint/provider/httpserver"
)

type LoginRequest struct {
    Username string `json:"username" binding:"required,email"`
    Password string `json:"password" binding:"required,min=8"`
}

func LoginHandler(c *gin.Context) {
    var req LoginRequest
    if !httpserver.ValidateJSON(c, &req) {
        return // Validation failed, error response already sent
    }

    // Continue with valid request
    // ...
}
```

### Validation Tags

Use standard validator tags in the `binding` field:

```go
type UserRequest struct {
    Name     string `json:"name" binding:"required,min=3,max=50"`
    Email    string `json:"email" binding:"required,email"`
    Age      int    `json:"age" binding:"required,min=18,max=120"`
    Website  string `json:"website" binding:"omitempty,url"`
    Password string `json:"password" binding:"required,min=8,securepassword"`
}
```

#### Built-in Validators

- `required` - Field cannot be empty
- `email` - Must be valid email format
- `min=N` - Minimum value/length
- `max=N` - Maximum value/length
- `len=N` - Exact length
- `url` - Must be valid URL
- `omitempty` - Skip validation if empty
- `securepassword` - Custom Blueprint validator for secure passwords

### Custom Validation with Validator Interface

Implement the `Validator` interface to add custom validation logic:

```go
type UserRequest struct {
    Username string `json:"username" binding:"required"`
    Password string `json:"password" binding:"required,min=8"`
}

func (r *UserRequest) Validate() error {
    // Cross-field validation
    if r.Username == "admin" && len(r.Password) < 12 {
        return httpserver.NewFieldError("password", "admin password must be at least 12 characters")
    }

    // Generic validation
    if isReservedUsername(r.Username) {
        return errors.New("username is reserved")
    }

    return nil
}
```

### Nested Structure Validation

The validation system recursively validates nested structures:

```go
type Address struct {
    Street  string `json:"street" binding:"required"`
    ZipCode string `json:"zip_code" binding:"required,len=5"`
}

func (a *Address) Validate() error {
    if a.ZipCode == "00000" {
        return httpserver.NewFieldError("zip_code", "invalid zip code")
    }
    return nil
}

type UserRequest struct {
    Name    string  `json:"name" binding:"required"`
    Address Address `json:"address" binding:"required"`
}

// Error response includes full path: {"field": "address.zip_code", "message": "invalid zip code"}
```

### Collection Validation

Validate slices, arrays, and maps:

```go
// Slice validation
type OrderRequest struct {
    Items []Item `json:"items" binding:"required,dive"`
}

type Item struct {
    Name string `json:"name" binding:"required"`
}

func (i *Item) Validate() error {
    if i.Name == "forbidden" {
        return errors.New("forbidden item name")
    }
    return nil
}

// Error for second item: {"field": "items[1]", "message": "forbidden item name"}
```

```go
// Map validation
type ConfigRequest struct {
    Settings map[string]*Setting `json:"settings" binding:"required"`
}

type Setting struct {
    Value string `json:"value" binding:"required"`
}

func (s *Setting) Validate() error {
    if s.Value == "invalid" {
        return errors.New("invalid setting value")
    }
    return nil
}

// Error: {"field": "settings[database]", "message": "invalid setting value"}
```

### Field-Specific Error Reporting

Use `NewFieldError()` to create field-specific validation errors:

```go
func (r *UserRequest) Validate() error {
    // Field-specific error
    if r.Age < 18 {
        return httpserver.NewFieldError("age", "must be at least 18 years old")
    }

    // Nested field error
    if r.Email == "blocked@example.com" {
        return httpserver.NewFieldError("email", "this email is blocked")
    }

    // Generic error (appears as "custom" field)
    if hasDuplicateData(r) {
        return errors.New("duplicate data detected")
    }

    return nil
}
```

## ValidateQuery - Query Parameter Validation

Validates URL query parameters using the same two-stage validation system:

### Basic Usage

```go
type SearchRequest struct {
    Query    string `form:"q" binding:"required,min=3"`
    Page     int    `form:"page" binding:"min=1"`
    PageSize int    `form:"page_size" binding:"min=10,max=100"`
}

func (s *SearchRequest) Validate() error {
    if s.Page > 1000 {
        return httpserver.NewFieldError("page", "maximum page number is 1000")
    }
    return nil
}

func SearchHandler(c *gin.Context) {
    var req SearchRequest
    if !httpserver.ValidateQuery(c, &req) {
        return // Validation failed, error response already sent
    }

    // Continue with valid request
    results := performSearch(req.Query, req.Page, req.PageSize)
    c.JSON(200, results)
}
```

### Cross-Field Validation

```go
type FilterRequest struct {
    Category string `form:"category" binding:"required"`
    MinPrice int    `form:"min_price" binding:"min=0"`
    MaxPrice int    `form:"max_price" binding:"min=0"`
}

func (f *FilterRequest) Validate() error {
    if f.MaxPrice > 0 && f.MinPrice > f.MaxPrice {
        return errors.New("min_price cannot be greater than max_price")
    }
    return nil
}
```

## Error Response Format

### JSON Request Errors

Validation errors for JSON requests return a structured response:

```json
{
    "success": false,
    "error": {
        "message": "request validation failed",
        "requestError": [
            {
                "field": "email",
                "message": "Error: Field validation failed on the 'email' validator"
            }
        ]
    }
}
```

### Nested Field Errors

Errors include full field paths for nested structures:

```json
{
    "success": false,
    "error": {
        "message": "request validation failed",
        "requestError": [
            {
                "field": "address.zip_code",
                "message": "invalid zip code"
            }
        ]
    }
}
```

### Custom Validation Errors

Errors from custom validation:

```json
{
    "success": false,
    "error": {
        "message": "request validation failed",
        "requestError": [
            {
                "field": "password",
                "message": "admin password must be at least 12 characters"
            }
        ]
    }
}
```

### Generic Errors

Errors without specific fields appear as "custom":

```json
{
    "success": false,
    "error": {
        "message": "request validation failed",
        "requestError": [
            {
                "field": "custom",
                "message": "duplicate data detected"
            }
        ]
    }
}
```

## Complete Example

```go
package main

import (
    "errors"
    "github.com/gin-gonic/gin"
    "github.com/oddbit-project/blueprint/provider/httpserver"
)

// Nested address validation
type Address struct {
    Street  string `json:"street" binding:"required"`
    City    string `json:"city" binding:"required"`
    ZipCode string `json:"zip_code" binding:"required,len=5"`
}

func (a *Address) Validate() error {
    if a.ZipCode == "00000" {
        return httpserver.NewFieldError("zip_code", "invalid zip code")
    }
    return nil
}

// User request with custom validation
type CreateUserRequest struct {
    Username string   `json:"username" binding:"required,min=3,max=20"`
    Email    string   `json:"email" binding:"required,email"`
    Password string   `json:"password" binding:"required,min=8,securepassword"`
    Age      int      `json:"age" binding:"required,min=18"`
    Address  Address  `json:"address" binding:"required"`
}

func (r *CreateUserRequest) Validate() error {
    // Admin users need stronger passwords
    if r.Username == "admin" && len(r.Password) < 12 {
        return httpserver.NewFieldError("password", "admin password must be at least 12 characters")
    }

    // Check for reserved usernames
    if isReservedUsername(r.Username) {
        return httpserver.NewFieldError("username", "this username is reserved")
    }

    return nil
}

// Search with query validation
type SearchRequest struct {
    Query    string `form:"q" binding:"required,min=3"`
    Page     int    `form:"page" binding:"min=1"`
    PageSize int    `form:"page_size" binding:"min=10,max=100"`
}

func (s *SearchRequest) Validate() error {
    if s.Page > 1000 {
        return httpserver.NewFieldError("page", "maximum page number is 1000")
    }
    return nil
}

func main() {
    router := gin.Default()

    router.POST("/users", createUserHandler)
    router.GET("/search", searchHandler)

    router.Run(":8080")
}

func createUserHandler(c *gin.Context) {
    var req CreateUserRequest

    // Automatic two-stage validation
    if !httpserver.ValidateJSON(c, &req) {
        return // Error response already sent
    }

    // Business logic
    user, err := createUser(req)
    if err != nil {
        c.JSON(500, gin.H{"error": "internal server error"})
        return
    }

    c.JSON(201, gin.H{"user": user})
}

func searchHandler(c *gin.Context) {
    var req SearchRequest

    // Query parameter validation
    if !httpserver.ValidateQuery(c, &req) {
        return // Error response already sent
    }

    // Perform search
    results := performSearch(req.Query, req.Page, req.PageSize)
    c.JSON(200, gin.H{"results": results})
}

// Helper functions
func isReservedUsername(username string) bool {
    reserved := []string{"admin", "root", "system"}
    for _, r := range reserved {
        if username == r {
            return true
        }
    }
    return false
}

func createUser(req CreateUserRequest) (interface{}, error) {
    // Implementation
    return nil, nil
}

func performSearch(query string, page, pageSize int) interface{} {
    // Implementation
    return nil
}
```

## Best Practices

### 1. Use Binding Tags for Basic Validation

Always use `binding` tags for structural validation:

```go
type Request struct {
    Email string `json:"email" binding:"required,email"` // Good
}
```

### 2. Implement Validator for Business Logic

Use the `Validator` interface for complex business rules:

```go
func (r *Request) Validate() error {
    // Business logic validation
    if r.Age < 18 && !r.ParentConsent {
        return httpserver.NewFieldError("parent_consent", "required for users under 18")
    }
    return nil
}
```

### 3. Use NewFieldError for Specific Fields

Always use `NewFieldError()` for field-specific errors to provide clear feedback:

```go
// Good: Field-specific error
return httpserver.NewFieldError("email", "this email is already registered")

// Avoid: Generic error
return errors.New("email is already registered") // Shows as "custom" field
```

### 4. Validate at the Right Level

Place validation logic at the appropriate struct level:

```go
type Address struct {
    ZipCode string `json:"zip_code" binding:"required,len=5"`
}

// Validate address-specific rules here
func (a *Address) Validate() error {
    if a.ZipCode == "00000" {
        return httpserver.NewFieldError("zip_code", "invalid zip code")
    }
    return nil
}

type UserRequest struct {
    Address Address `json:"address" binding:"required"`
}

// Validate user-specific rules here
func (r *UserRequest) Validate() error {
    // Cross-entity validation
    return nil
}
```

### 5. Handle Both JSON and Query Parameters

Use appropriate validation functions:

```go
// For JSON bodies
if !httpserver.ValidateJSON(c, &jsonRequest) {
    return
}

// For query parameters
if !httpserver.ValidateQuery(c, &queryRequest) {
    return
}
```

## Common Patterns

### Date Range Validation

```go
type DateRangeRequest struct {
    StartDate string `json:"start_date" binding:"required"`
    EndDate   string `json:"end_date" binding:"required"`
}

func (r *DateRangeRequest) Validate() error {
    if r.StartDate > r.EndDate {
        return errors.New("start_date must be before end_date")
    }
    return nil
}
```

### Conditional Required Fields

```go
type PaymentRequest struct {
    Method        string `json:"method" binding:"required"`
    CreditCard    string `json:"credit_card" binding:"omitempty"`
    BankAccount   string `json:"bank_account" binding:"omitempty"`
}

func (r *PaymentRequest) Validate() error {
    if r.Method == "card" && r.CreditCard == "" {
        return httpserver.NewFieldError("credit_card", "required for card payments")
    }
    if r.Method == "bank" && r.BankAccount == "" {
        return httpserver.NewFieldError("bank_account", "required for bank transfers")
    }
    return nil
}
```

### Password Confirmation

```go
type RegisterRequest struct {
    Password        string `json:"password" binding:"required,min=8"`
    PasswordConfirm string `json:"password_confirm" binding:"required"`
}

func (r *RegisterRequest) Validate() error {
    if r.Password != r.PasswordConfirm {
        return httpserver.NewFieldError("password_confirm", "passwords do not match")
    }
    return nil
}
```

## Troubleshooting

### Validation Not Working

1. Ensure you're using `binding` tags, not `validate` tags:
   ```go
   // Correct
   Field string `json:"field" binding:"required"`

   // Wrong - won't work
   Field string `json:"field" validate:"required"`
   ```

2. Check that your struct implements `Validator` with a pointer receiver:
   ```go
   // Correct
   func (r *Request) Validate() error { ... }

   // May not work for all cases
   func (r Request) Validate() error { ... }
   ```

### Nested Validation Not Working

For map values, use pointers to ensure `Validate()` is called:
```go
// Works
type Request struct {
    Settings map[string]*Setting `json:"settings"`
}

// May not work for pointer receiver methods
type Request struct {
    Settings map[string]Setting `json:"settings"`
}
```

### Custom Errors Not Showing Correct Field

Use `NewFieldError()` to specify the field:
```go
// Shows correct field
return httpserver.NewFieldError("email", "email is blocked")

// Shows as "custom" field
return errors.New("email is blocked")
```
