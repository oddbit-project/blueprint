# HTTP Server Middleware

Blueprint provides a comprehensive middleware system for HTTP servers with built-in components for common functionality and easy custom middleware development.

## Middleware Overview

The HTTP server supports middleware through Gin's middleware system with additional Blueprint-specific components:

- **Authentication Middleware**: Token and JWT-based authentication ([auth.md](auth.md))
- **Security Middleware**: Headers, CSRF, and rate limiting ([security.md](security.md))
- **Session Middleware**: Cookie-based session management ([session.md](session.md))
- **Logging Middleware**: Structured HTTP request logging
- **Response Helpers**: Standardized error and success responses
- **Recovery Middleware**: Panic recovery with logging

## Core Middleware Components

### HTTP Logging Middleware

Blueprint provides structured HTTP logging that integrates with the application logger.

#### Features
- Structured logging with request details
- Configurable log levels
- Integration with Blueprint's logging framework
- Request correlation IDs
- Performance metrics

#### Usage

```go
import "github.com/oddbit-project/blueprint/provider/httpserver/log"

// Automatic integration when creating router
router := httpserver.NewRouter("api-server", false, logger)
// Logging middleware is automatically added

// Manual integration
router.Use(log.HTTPLogMiddleware(logger))
```

#### Log Output

The middleware logs requests with structured data:

```json
{
    "level": "info",
    "time": "2024-01-15T10:30:00Z",
    "logger": "api-server",
    "message": "HTTP Request",
    "method": "GET",
    "path": "/api/users",
    "status": 200,
    "latency": "15ms",
    "client_ip": "192.168.1.100",
    "user_agent": "Mozilla/5.0...",
    "request_id": "req-123456"
}
```

#### Request Logging Functions

```go
// Log informational messages with request context
log.RequestInfo(ctx *gin.Context, message string, fields map[string]interface{})

// Log warnings with request context
log.RequestWarn(ctx *gin.Context, message string, fields map[string]interface{})

// Log errors with request context and stack trace
log.RequestError(ctx *gin.Context, err error, message string, fields map[string]interface{})
```

**Example:**
```go
func userHandler(c *gin.Context) {
    userID := c.Param("id")
    
    log.RequestInfo(c, "fetching user", map[string]interface{}{
        "user_id": userID,
    })
    
    user, err := getUserByID(userID)
    if err != nil {
        log.RequestError(c, err, "failed to fetch user", map[string]interface{}{
            "user_id": userID,
        })
        response.Http500(c, err)
        return
    }
    
    c.JSON(200, user)
}
```

### Recovery Middleware

Gin's recovery middleware is automatically included with Blueprint's error logging.

#### Features
- Catches panics and recovers gracefully
- Logs panic details with stack trace
- Returns 500 Internal Server Error response
- Integrates with Blueprint's logging system

#### Configuration

```go
// Recovery is automatically added in NewRouter
router := httpserver.NewRouter("api-server", false, logger)

// Manual addition (if needed)
router.Use(gin.Recovery())
```

### Request Detection Middleware

Blueprint provides utilities for detecting request types and content.

#### JSON Request Detection

```go
import "github.com/oddbit-project/blueprint/provider/httpserver/request"

func handler(c *gin.Context) {
    if request.IsJSONRequest(c) {
        // Handle JSON request
        c.JSON(200, gin.H{"type": "json"})
    } else {
        // Handle non-JSON request
        c.String(200, "text response")
    }
}
```

## Response Middleware

### Standardized Response Helpers

Blueprint provides consistent response helpers that automatically detect request type and format responses appropriately.

#### Success Responses

```go
import "github.com/oddbit-project/blueprint/provider/httpserver/response"

func successHandler(c *gin.Context) {
    // For JSON requests, returns structured JSON
    // For other requests, may return different formats
    c.JSON(200, response.JSONResponse{
        Success: true,
        Data:    gin.H{"message": "Operation successful"},
    })
}
```

#### Error Responses

```go
func errorHandler(c *gin.Context) {
    // Automatic request type detection and logging
    response.Http400(c, "Invalid input provided")
    
    // For validation errors
    validationErrors := map[string]string{
        "email": "Invalid email format",
        "age": "Must be a positive number",
    }
    response.ValidationError(c, validationErrors)
}
```

#### Response Types

All response helpers support both JSON and non-JSON requests:

- **JSON Requests**: Return structured JSON with `success`, `data`, and `error` fields
- **Non-JSON Requests**: Return appropriate HTTP status codes

## Custom Middleware Development

### Creating Custom Middleware

```go
// Simple middleware example
func CustomHeaderMiddleware(headerValue string) gin.HandlerFunc {
    return func(c *gin.Context) {
        c.Header("X-Custom-Header", headerValue)
        c.Next()
    }
}

// Middleware with error handling
func ValidationMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        if c.Request.Header.Get("Content-Type") == "" {
            response.Http400(c, "Content-Type header required")
            return
        }
        c.Next()
    }
}

// Usage
server.AddMiddleware(CustomHeaderMiddleware("my-value"))
server.AddMiddleware(ValidationMiddleware())
```

### Middleware with Dependencies

```go
type DatabaseMiddleware struct {
    db *sql.DB
    logger *log.Logger
}

func NewDatabaseMiddleware(db *sql.DB, logger *log.Logger) *DatabaseMiddleware {
    return &DatabaseMiddleware{
        db: db,
        logger: logger,
    }
}

func (m *DatabaseMiddleware) Middleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        // Add database connection to context
        c.Set("db", m.db)
        
        // Log database operations
        m.logger.Info("database middleware applied", map[string]interface{}{
            "path": c.Request.URL.Path,
        })
        
        c.Next()
    }
}

// Usage
dbMiddleware := NewDatabaseMiddleware(db, logger)
server.AddMiddleware(dbMiddleware.Middleware())
```

### Request Context Middleware

```go
func RequestIDMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        requestID := uuid.New().String()
        c.Set("request_id", requestID)
        c.Header("X-Request-ID", requestID)
        c.Next()
    }
}

func UserContextMiddleware(userService UserService) gin.HandlerFunc {
    return func(c *gin.Context) {
        // Extract user from token/session
        userID := extractUserIDFromRequest(c)
        if userID != "" {
            user, err := userService.GetUser(userID)
            if err == nil {
                c.Set("current_user", user)
            }
        }
        c.Next()
    }
}
```

## Middleware Ordering

Middleware order is crucial for proper functionality. Blueprint recommends this order:

### 1. Security Headers
```go
server.UseDefaultSecurityHeaders()
```

### 2. Request Identification
```go
server.AddMiddleware(RequestIDMiddleware())
```

### 3. Rate Limiting
```go
server.UseRateLimiting(100)
```

### 4. Session Management
```go
sessionManager := server.UseSession(sessionConfig, backend, logger)
```

### 5. CSRF Protection
```go
server.UseCSRFProtection()
```

### 6. Authentication
```go
server.UseAuth(authProvider)
```

### 7. Business Logic Middleware
```go
server.AddMiddleware(CustomBusinessLogicMiddleware())
```

### Complete Middleware Stack Example

```go
func setupMiddleware(server *httpserver.Server, logger *log.Logger) {
    // 1. Security headers (first)
    server.UseDefaultSecurityHeaders()
    
    // 2. Request identification
    server.AddMiddleware(RequestIDMiddleware())
    
    // 3. Rate limiting (early to prevent abuse)
    server.UseRateLimiting(100)
    
    // 4. Sessions (before CSRF and auth)
    backend := kv.NewMemoryKV()
    sessionManager := server.UseSession(nil, backend, logger)
    
    // 5. CSRF protection (after sessions)
    server.UseCSRFProtection()
    
    // 6. Custom business middleware
    server.AddMiddleware(DatabaseConnectionMiddleware(db))
    server.AddMiddleware(MetricsMiddleware())
    
    // 7. Authentication (last, so other middleware is available)
    tokenAuth := auth.NewAuthToken("X-API-Key", "secret")
    server.UseAuth(tokenAuth)
}
```

## Route-Specific Middleware

### Group Middleware

```go
func setupRoutes(server *httpserver.Server) {
    router := server.Route()
    
    // Public routes (no additional middleware)
    router.GET("/health", healthHandler)
    router.POST("/login", loginHandler)
    
    // API routes with rate limiting
    api := server.Group("/api/v1")
    api.Use(RateLimitMiddleware(60)) // 60 requests per minute
    {
        api.GET("/users", getUsersHandler)
        api.POST("/users", createUserHandler)
    }
    
    // Admin routes with stricter auth
    admin := server.Group("/admin")
    admin.Use(AdminAuthMiddleware())
    admin.Use(AuditLogMiddleware())
    {
        admin.GET("/users", adminListUsersHandler)
        admin.DELETE("/users/:id", adminDeleteUserHandler)
    }
}
```

### Conditional Middleware

```go
func ConditionalMiddleware(condition func(*gin.Context) bool, middleware gin.HandlerFunc) gin.HandlerFunc {
    return func(c *gin.Context) {
        if condition(c) {
            middleware(c)
        } else {
            c.Next()
        }
    }
}

// Usage
server.AddMiddleware(ConditionalMiddleware(
    func(c *gin.Context) bool {
        return strings.HasPrefix(c.Request.URL.Path, "/api/")
    },
    RateLimitMiddleware(100),
))
```

## Error Handling in Middleware

### Graceful Error Handling

```go
func SafeMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        defer func() {
            if r := recover(); r != nil {
                log.RequestError(c, fmt.Errorf("middleware panic: %v", r), 
                    "middleware panic recovered", nil)
                response.Http500(c, fmt.Errorf("internal error"))
            }
        }()
        
        // Middleware logic here
        c.Next()
    }
}
```

### Error Response Middleware

```go
func ErrorHandlingMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        c.Next()
        
        // Check for errors after request processing
        if len(c.Errors) > 0 {
            err := c.Errors.Last()
            log.RequestError(c, err, "request processing error", nil)
            
            // Return appropriate error response
            if !c.Writer.Written() {
                response.Http500(c, err)
            }
        }
    }
}
```

## Middleware Testing

### Testing Custom Middleware

```go
func TestCustomMiddleware(t *testing.T) {
    // Create test router
    router := gin.New()
    router.Use(CustomHeaderMiddleware("test-value"))
    
    // Add test route
    router.GET("/test", func(c *gin.Context) {
        c.String(200, "OK")
    })
    
    // Create test request
    req := httptest.NewRequest("GET", "/test", nil)
    w := httptest.NewRecorder()
    
    // Execute request
    router.ServeHTTP(w, req)
    
    // Assert results
    assert.Equal(t, 200, w.Code)
    assert.Equal(t, "test-value", w.Header().Get("X-Custom-Header"))
}
```

### Integration Testing

```go
func TestMiddlewareStack(t *testing.T) {
    logger := log.New("test")
    config := httpserver.NewServerConfig()
    server, _ := httpserver.NewServer(config, logger)
    
    // Apply middleware
    setupMiddleware(server, logger)
    
    // Add test route
    server.Route().GET("/protected", func(c *gin.Context) {
        c.JSON(200, gin.H{"status": "ok"})
    })
    
    // Test requests
    req := httptest.NewRequest("GET", "/protected", nil)
    req.Header.Set("X-API-Key", "secret")
    
    w := httptest.NewRecorder()
    server.Route().ServeHTTP(w, req)
    
    assert.Equal(t, 200, w.Code)
}
```

## Best Practices

### Middleware Design

1. **Single Responsibility**: Each middleware should have one clear purpose
2. **Error Handling**: Always handle errors gracefully and log appropriately
3. **Performance**: Minimize processing time in middleware
4. **Context Management**: Use context appropriately for request-scoped data
5. **Testing**: Write tests for all custom middleware

### Performance Considerations

1. **Order Matters**: Place faster middleware first
2. **Early Exit**: Stop processing on authentication failures
3. **Caching**: Cache expensive operations where appropriate
4. **Avoid Blocking**: Don't perform blocking operations in middleware

### Security Guidelines

1. **Input Validation**: Validate all inputs in middleware
2. **Error Information**: Don't expose sensitive information in error responses
3. **Logging**: Log security events appropriately
4. **Dependencies**: Keep middleware dependencies minimal

The middleware system provides a flexible foundation for building secure, performant HTTP applications with Blueprint's integrated components and custom business logic.