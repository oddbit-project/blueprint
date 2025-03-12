# HTTP Request Utilities

Blueprint provides utilities for working with HTTP requests in the Gin framework, making it easier to handle different content types and validate request data.

## Content Type Detection

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

## Content Type Constants

The package provides constants for common content types:

```go
const (
    HeaderAccept      = "Accept"
    HeaderContentType = "Content-Type"

    ContentTypeHtml   = "text/html"
    ContentTypeJson   = "application/json"
    ContentTypeBinary = "application/octet-stream"
)
```

## CSRF Protection

The request package includes CSRF protection utilities:

```go
import (
    "github.com/gin-gonic/gin"
    "github.com/oddbit-project/blueprint/provider/httpserver/request"
)

func SetupRouter() *gin.Engine {
    router := gin.Default()
    
    // Add CSRF protection middleware
    router.Use(request.CSRFProtection())
    
    // Routes
    // ...
    
    return router
}
```

## Form Validation

Blueprint provides utilities for validating request data:

```go
import (
    "github.com/gin-gonic/gin"
    "github.com/oddbit-project/blueprint/provider/httpserver/request/validator"
)

type LoginForm struct {
    Username string `json:"username" binding:"required"`
    Password string `json:"password" binding:"required,securePassword"`
}

func init() {
    // Register the secure password validator
    validator.RegisterSecurePasswordValidator()
}

func LoginHandler(ctx *gin.Context) {
    var form LoginForm
    if err := ctx.ShouldBindJSON(&form); err != nil {
        ctx.JSON(400, gin.H{"error": err.Error()})
        return
    }
    
    // Process valid form data
    // ...
}
```

The `securePassword` validator ensures passwords meet security requirements, including:

- Minimum length
- Character diversity (uppercase, lowercase, numbers, symbols)
- Common password checks

## Best Practices

1. Always validate user input
2. Use content type detection to handle requests appropriately
3. Implement CSRF protection for forms
4. Use secure password validation for user credentials
5. Return appropriate status codes and content types in responses