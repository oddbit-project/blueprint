# HTTP Server API Reference

Complete API documentation for Blueprint's HTTP server framework built on Gin with integrated middleware components.

## Core Server API

### ServerConfig

Configuration structure for HTTP server settings:

```go
type ServerConfig struct {
    Host           string            `json:"host"`           // Server bind address (default: "")
    Port           int               `json:"port"`           // Server port (default: 5000)
    ReadTimeout    int               `json:"readTimeout"`    // Read timeout in seconds (default: 600)
    WriteTimeout   int               `json:"writeTimeout"`   // Write timeout in seconds (default: 600)
    Debug          bool              `json:"debug"`          // Enable debug mode (default: false)
    Options        map[string]string `json:"options"`        // Additional server options
    TrustedProxies []string          `json:"trustedProxies"` // List of trusted proxy IPs/CIDRs
    tlsProvider.ServerConfig                                 // TLS configuration
}
```

#### Configuration Methods

```go
func NewServerConfig() *ServerConfig
```
Creates a new server configuration with default values.

```go
func (c *ServerConfig) GetOption(key string, defaultValue string) string
```
Retrieves option value by key, returns defaultValue if not found.

```go
func (c *ServerConfig) Validate() error
```
Validates the server configuration (currently returns nil).

```go
func (c *ServerConfig) NewServer(logger *log.Logger) (*Server, error)
```
Creates a new server instance using this configuration.

#### Default Values

```go
const (
    ServerDefaultReadTimeout  = 600   // 10 minutes
    ServerDefaultWriteTimeout = 600   // 10 minutes
    ServerDefaultPort         = 5000  // Default port
    ServerDefaultName         = "http" // Default server name
)
```

#### Configuration Options

The `Options` map supports these predefined keys:

```go
const (
    OptAuthTokenHeader        = "authTokenHeader"        // Custom auth header name
    OptAuthTokenSecret        = "authTokenSecret"        // Auth token secret
    OptDefaultSecurityHeaders = "defaultSecurityHeaders" // Enable default security headers
    OptHMACSecret             = "hmacSecret"             // HMAC secret for request signing
)
```
> Note: 'authTokenHeader' and 'authTokenSecret' will be used to configure automatically 
> simple token-based auth if ProcessOptions() is called; if no 'authTokenHeader' is specified, a default value is used


**Example:**
```go
config := NewServerConfig()
config.Host = "localhost"
config.Port = 8080
config.Debug = true
config.Options["authTokenSecret"] = "my-secret-key"
config.Options["defaultSecurityHeaders"] = "true"
```

### Server

Main server structure providing HTTP functionality:

```go
type Server struct {
    Config *ServerConfig  // Server configuration
    Router *gin.Engine    // Gin router instance
    Server *http.Server   // Underlying HTTP server
    Logger *log.Logger    // Structured logger
}
```

#### Server Creation

```go
func NewServer(cfg *ServerConfig, logger *log.Logger) (*Server, error)
```
Creates a new HTTP server instance.

**Parameters:**
- `cfg`: Server configuration (nil uses defaults)
- `logger`: Logger instance (nil creates default HTTP logger)

**Returns:**
- Configured Server instance with Gin router and HTTP server
- Error if configuration validation fails

**Example:**
```go
config := NewServerConfig()
config.Port = 8080

logger := log.New("api-server")
server, err := NewServer(config, logger)
if err != nil {
    log.Fatal(err)
}
```

#### Server Lifecycle

```go
func (s *Server) Start() error
```
Starts the HTTP server (blocking call).

- Uses TLS if `TLSConfig` is configured
- Returns `nil` when gracefully shut down
- Returns error for startup failures

```go
func (s *Server) Shutdown(ctx context.Context) error
```
Gracefully shuts down the server.

**Parameters:**
- `ctx`: Context for shutdown timeout control

**Example:**
```go
// Start server in goroutine
go func() {
    if err := server.Start(); err != nil {
        logger.Error(err, "server failed")
    }
}()

// Graceful shutdown
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()

if err := server.Shutdown(ctx); err != nil {
    logger.Error(err, "shutdown failed")
}
```

#### URL Generation

```go
func (c *ServerConfig) GetUrl() string
```
Builds the full URL from the server configuration.

**Returns:**
- `string`: Full URL (e.g., "http://localhost:8080" or "https://localhost:8080" if TLS is enabled)

**Example:**
```go
config := NewServerConfig()
config.Host = "api.example.com"
config.Port = 443
config.TLSEnable = true

url := config.GetUrl()  // Returns "https://api.example.com:443"
```

#### Router Access

```go
func (s *Server) Route() *gin.Engine
```
Returns the underlying Gin router for direct access.

```go
func (s *Server) Group(relativePath string) *gin.RouterGroup
```
Creates a new router group with the specified base path.

**Example:**
```go
// Direct router access
server.Route().GET("/health", healthHandler)

// Router groups
api := server.Group("/api/v1")
api.GET("/users", getUsersHandler)
api.POST("/users", createUserHandler)

admin := server.Group("/admin")
admin.GET("/stats", adminStatsHandler)
```

#### Middleware Management

```go
func (s *Server) AddMiddleware(middlewareFunc gin.HandlerFunc)
```
Adds middleware to the server's router.

**Example:**
```go
// Custom middleware
server.AddMiddleware(func(c *gin.Context) {
    c.Header("X-Custom-Header", "value")
    c.Next()
})

// Third-party middleware
server.AddMiddleware(cors.Default())
```

#### Options Processing

```go
func (s *Server) ProcessOptions(withOptions ...OptionsFunc) error
```
Processes server options and applies configuration-based middleware.

**Automatic Processing:**
- `OptDefaultSecurityHeaders`: Applies default security headers if "true" or "1"
- `OptAuthTokenSecret`: Sets up token authentication with optional custom header

**Example:**
```go
config.Options["defaultSecurityHeaders"] = "true"
config.Options["authTokenSecret"] = "my-api-key"
config.Options["authTokenHeader"] = "X-API-Key"

server, _ := NewServer(config, logger)
err := server.ProcessOptions() // Applies security headers and auth
```

### Router Creation

```go
func NewRouter(serverName string, debug bool, logger *log.Logger) *gin.Engine
```
Creates a new Gin router with standardized configuration.

**Features:**
- Sets release mode for production (`!debug`)
- Adds structured HTTP logging middleware
- Includes recovery middleware
- Configures error logging wrapper

**Example:**
```go
router := NewRouter("my-api", false, logger)
router.GET("/test", testHandler)
```

## Middleware API

### Authentication Middleware

```go
func (s *Server) UseAuth(provider auth.Provider)
```
Registers authentication middleware with the specified provider.

**Parameters:**
- `provider`: Authentication provider implementing `CanAccess(c *gin.Context) bool`

**Example:**
```go
// Token authentication
tokenAuth := auth.NewAuthToken("X-API-Key", "secret-key")
server.UseAuth(tokenAuth)

// JWT authentication
jwtAuth := auth.NewAuthJWT(jwtProvider)
server.UseAuth(jwtAuth)
```

### Security Middleware

```go
func (s *Server) UseSecurityHeaders(config *security.SecurityConfig)
```
Adds security headers middleware with custom configuration.

```go
func (s *Server) UseDefaultSecurityHeaders()
```
Adds security headers middleware with default configuration.

```go
func (s *Server) UseCSRFProtection()
```
Adds CSRF protection middleware.

**Example:**
```go
// Custom security configuration
securityConfig := &security.SecurityConfig{
    CSP: "default-src 'self'",
    HSTS: "max-age=31536000",
    FrameOptions: "DENY",
}
server.UseSecurityHeaders(securityConfig)

// Default security headers
server.UseDefaultSecurityHeaders()

// CSRF protection
server.UseCSRFProtection()
```

### Rate Limiting

```go
func (s *Server) UseRateLimiting(ratePerMinute int)
```
Adds rate limiting middleware.

**Parameters:**
- `ratePerMinute`: Maximum requests per minute per IP
- Uses burst size of 5 requests

**Example:**
```go
// Allow 60 requests per minute
server.UseRateLimiting(60)
```

### Session Management

```go
func (s *Server) UseSession(config *session.Config, backend kv.KV, logger *log.Logger) (*session.Manager, error)
```
Adds session middleware with specified configuration and storage backend.

**Parameters:**
- `config`: Session configuration (nil uses defaults)
- `backend`: KV storage backend (memory, Redis, or custom)
- `logger`: Logger for session operations

**Returns:**
- `*session.Manager`: Session manager instance for additional session operations
- `error`: Error if session setup fails

**Example:**
```go
// Memory-based sessions
backend := kv.NewMemoryKV()
sessionConfig := session.NewConfig()
manager, err := server.UseSession(sessionConfig, backend, logger)
if err != nil {
    log.Fatal(err)
}

// Redis-based sessions
redisBackend, _ := redis.NewClient(redisConfig)
manager, err := server.UseSession(sessionConfig, redisBackend, logger)
if err != nil {
    log.Fatal(err)
}
```

## Response Helper API

### Standard Response Types

```go
type JSONResponse struct {
    Success bool        `json:"success"`
    Data    interface{} `json:"data,omitempty"`
}

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

All response functions automatically detect JSON requests and return appropriate responses.

```go
func Http401(ctx *gin.Context)
```
Generates 401 Unauthorized response with logging.

```go
func Http403(ctx *gin.Context)
```
Generates 403 Forbidden response with logging.

```go
func Http404(ctx *gin.Context)
```
Generates 404 Not Found response with logging.

```go
func Http400(ctx *gin.Context, message string)
```
Generates 400 Bad Request response with custom message.

```go
func Http429(ctx *gin.Context)
```
Generates 429 Too Many Requests response.

```go
func Http500(ctx *gin.Context, err error)
```
Generates 500 Internal Server Error response with error logging.

```go
func ValidationError(ctx *gin.Context, errors interface{})
```
Generates 400 Bad Request response for validation failures.

```go
func Success(ctx *gin.Context, data any)
```
Generates a JSON success response with an optional payload.

**Parameters:**
- `ctx`: Gin context
- `data`: Optional data payload (can be nil)

**Response Format:**
```json
{
    "success": true,
    "data": { ... }
}
```

**JSON Response Example:**
```json
{
    "success": false,
    "error": {
        "message": "Unauthorized",
        "requestError": null
    }
}
```

**Usage Example:**
```go
func protectedHandler(c *gin.Context) {
    token := c.GetHeader("Authorization")
    if token == "" {
        response.Http401(c)
        return
    }
    
    user, err := getUserFromToken(token)
    if err != nil {
        response.Http500(c, err)
        return
    }
    
    c.JSON(200, response.JSONResponse{
        Success: true,
        Data:    user,
    })
}
```

## Complete Server Setup Example

check [Blueprint samples](https://github.com/oddbit-project/samples/httpserver-session)
