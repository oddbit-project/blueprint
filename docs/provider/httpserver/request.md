# HTTP Request and Response Utilities

Blueprint provides utilities for working with HTTP requests and responses in the Gin framework, making it easier to handle different content types, validate request data, and generate standardized responses.

## Request Utilities

### Content Type Detection

You can use the `IsJSONRequest` function to determine if a request expects or contains JSON data:

```go
import (
    "github.com/gin-gonic/gin"
    "github.com/oddbit-project/blueprint/provider/httpserver/request"
)

func HandleRequest(ctx *gin.Context) {
    if request.IsJSONRequest(ctx) {
        // Handle JSON request
        // ...
        ctx.JSON(200, gin.H{"message": "Success"})
    } else {
        // Handle other content types
        // ...
        ctx.HTML(200, "template.html", gin.H{"message": "Success"})
    }
}
```

### Content Type Constants

The request package provides constants for common content types:

```go
const (
    HeaderAccept      = "Accept"
    HeaderContentType = "Content-Type"

    ContentTypeHtml   = "text/html"
    ContentTypeJson   = "application/json"
    ContentTypeBinary = "application/octet-stream"
)
```

### CSRF Protection

The request package includes CSRF protection utilities:

```go
import (
    "github.com/gin-gonic/gin"
    "github.com/oddbit-project/blueprint/provider/httpserver/request"
)

func SetupRouter() *gin.Engine {
    router := gin.Default()
    
    // Add CSRF protection middleware
    router.Use(request.CSRFMiddleware())
    
    // Generate CSRF token in handlers
    router.GET("/form", func(c *gin.Context) {
        csrfToken := request.GenerateCSRFToken(c)
        c.HTML(200, "form.html", gin.H{
            "csrfToken": csrfToken,
        })
    })
    
    return router
}
```

#### CSRF Token Usage

Include CSRF tokens in forms:

```html
<form method="POST" action="/submit">
    <input type="hidden" name="_csrf" value="{{.csrfToken}}">
    <!-- other form fields -->
    <button type="submit">Submit</button>
</form>
```

Or in AJAX requests:

```javascript
fetch('/api/data', {
    method: 'POST',
    headers: {
        'Content-Type': 'application/json',
        'X-CSRF-Token': csrfToken
    },
    body: JSON.stringify(data)
});
```

## Response Utilities

Blueprint provides standardized response helpers that automatically detect request type and format responses appropriately.

### Response Types

The response package defines standard structures for consistent API responses:

```go
// Success response structure
type JSONResponse struct {
    Success bool        `json:"success"`
    Data    interface{} `json:"data,omitempty"`
}

// Error response structure
type JSONResponseError struct {
    Success bool        `json:"success"`
    Error   ErrorDetail `json:"error"`
}

type ErrorDetail struct {
    Message      string      `json:"message,omitempty"`
    RequestError interface{} `json:"requestError,omitempty"`
}
```

### Error Response Functions

All response functions automatically detect JSON requests using `request.IsJSONRequest()` and return appropriate responses:

#### Http401 - Unauthorized
```go
import "github.com/oddbit-project/blueprint/provider/httpserver/response"

func protectedHandler(c *gin.Context) {
    if !isAuthenticated(c) {
        response.Http401(c)
        return
    }
    
    // Handle authenticated request
}
```

#### Http403 - Forbidden
```go
func adminHandler(c *gin.Context) {
    if !isAdmin(c) {
        response.Http403(c)
        return
    }
    
    // Handle admin request
}
```

#### Http404 - Not Found
```go
func getUserHandler(c *gin.Context) {
    userID := c.Param("id")
    user, err := findUser(userID)
    if err != nil {
        response.Http404(c)
        return
    }
    
    c.JSON(200, user)
}
```

#### Http400 - Bad Request
```go
func createUserHandler(c *gin.Context) {
    var user User
    if err := c.ShouldBindJSON(&user); err != nil {
        response.Http400(c, "Invalid user data provided")
        return
    }
    
    // Process valid user data
}
```

#### Http429 - Too Many Requests
```go
func rateLimitedHandler(c *gin.Context) {
    if isRateLimited(c) {
        response.Http429(c)
        return
    }
    
    // Handle request
}
```

#### Http500 - Internal Server Error
```go
func databaseHandler(c *gin.Context) {
    data, err := queryDatabase()
    if err != nil {
        response.Http500(c, err)
        return
    }
    
    c.JSON(200, data)
}
```

#### ValidationError - Request Validation Failed
```go
func validateAndCreateUser(c *gin.Context) {
    var user User
    if err := c.ShouldBindJSON(&user); err != nil {
        // Pass validation errors for detailed feedback
        response.ValidationError(c, err)
        return
    }
    
    // Additional validation
    if validationErrors := validateUser(user); len(validationErrors) > 0 {
        response.ValidationError(c, validationErrors)
        return
    }
    
    // Create user
}
```

### Response Behavior

#### JSON Requests
For requests with `Accept: application/json` or `Content-Type: application/json`, responses are structured JSON:

```json
// Success response
{
    "success": true,
    "data": {
        "id": 123,
        "name": "John Doe"
    }
}

// Error response
{
    "success": false,
    "error": {
        "message": "Resource not found"
    }
}

// Validation error response
{
    "success": false,
    "error": {
        "message": "request validation failed",
        "requestError": {
            "field": "email",
            "error": "invalid email format"
        }
    }
}
```

#### Non-JSON Requests
For HTML or other content types, responses return appropriate HTTP status codes without JSON body.

### Logging Integration

All response helpers automatically log events with appropriate levels:

- **Http401, Http403**: Warning level with access attempt details
- **Http404**: Info level with resource path
- **Http400, Http429**: Warning level with request details
- **Http500**: Error level with full error details and stack trace
- **ValidationError**: Warning level with validation failure details

### Complete Usage Example

```go
package main

import (
    "github.com/gin-gonic/gin"
    "github.com/oddbit-project/blueprint/provider/httpserver/request"
    "github.com/oddbit-project/blueprint/provider/httpserver/response"
)

type User struct {
    ID    int    `json:"id"`
    Name  string `json:"name" binding:"required"`
    Email string `json:"email" binding:"required,email"`
}

func main() {
    router := gin.Default()
    
    // Add CSRF protection
    router.Use(request.CSRFMiddleware())
    
    // Routes
    router.GET("/users/:id", getUserHandler)
    router.POST("/users", createUserHandler)
    router.GET("/form", formHandler)
    
    router.Run(":8080")
}

func getUserHandler(c *gin.Context) {
    userID := c.Param("id")
    
    user, err := findUserByID(userID)
    if err != nil {
        response.Http404(c)
        return
    }
    
    if request.IsJSONRequest(c) {
        c.JSON(200, response.JSONResponse{
            Success: true,
            Data:    user,
        })
    } else {
        c.HTML(200, "user.html", gin.H{"user": user})
    }
}

func createUserHandler(c *gin.Context) {
    var user User
    
    // Validate request data
    if err := c.ShouldBindJSON(&user); err != nil {
        response.ValidationError(c, err)
        return
    }
    
    // Business logic validation
    if exists := checkUserExists(user.Email); exists {
        response.Http400(c, "User with this email already exists")
        return
    }
    
    // Create user
    createdUser, err := createUser(user)
    if err != nil {
        response.Http500(c, err)
        return
    }
    
    if request.IsJSONRequest(c) {
        c.JSON(201, response.JSONResponse{
            Success: true,
            Data:    createdUser,
        })
    } else {
        c.Redirect(302, "/users/"+string(createdUser.ID))
    }
}

func formHandler(c *gin.Context) {
    csrfToken := request.GenerateCSRFToken(c)
    c.HTML(200, "form.html", gin.H{
        "csrfToken": csrfToken,
    })
}

// Helper functions (implement as needed)
func findUserByID(id string) (*User, error) { /* ... */ }
func checkUserExists(email string) bool { /* ... */ }
func createUser(user User) (*User, error) { /* ... */ }
```

## Best Practices

### Request Handling
1. **Always validate user input** using Gin's binding features
2. **Use content type detection** to handle requests appropriately
3. **Implement CSRF protection** for state-changing operations
4. **Handle both JSON and HTML requests** in the same handlers when possible

### Response Handling
1. **Use standardized response helpers** instead of manual status codes
2. **Provide meaningful error messages** without exposing internal details
3. **Log errors appropriately** using the built-in logging integration
4. **Return consistent response formats** for API clients
5. **Handle validation errors gracefully** with detailed feedback

### Security Considerations
1. **Don't expose internal error details** in production responses
2. **Use CSRF protection** for all state-changing operations
3. **Validate all inputs** before processing
4. **Log security events** (unauthorized access, validation failures)
5. **Return appropriate HTTP status codes** for different scenarios

### Error Handling Patterns

```go
// Good: Use response helpers
func goodHandler(c *gin.Context) {
    if !isAuthenticated(c) {
        response.Http401(c)  // Automatic logging and consistent format
        return
    }
    
    data, err := processRequest()
    if err != nil {
        response.Http500(c, err)  // Error logged with stack trace
        return
    }
    
    c.JSON(200, response.JSONResponse{Success: true, Data: data})
}

// Bad: Manual status codes
func badHandler(c *gin.Context) {
    if !isAuthenticated(c) {
        c.JSON(401, gin.H{"error": "unauthorized"})  // No logging, inconsistent format
        return
    }
    
    data, err := processRequest()
    if err != nil {
        c.JSON(500, gin.H{"error": err.Error()})  // Exposes internal errors
        return
    }
    
    c.JSON(200, gin.H{"data": data})  // Inconsistent response format
}
```

The request and response utilities work together to provide a complete foundation for HTTP handling in Blueprint applications, ensuring consistent behavior, proper logging, and security best practices.