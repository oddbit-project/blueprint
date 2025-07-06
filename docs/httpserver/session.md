# Session Management

Blueprint provides a flexible session management system for HTTP applications with support for multiple storage backends and session types:

1. **Cookie-based Sessions**: Traditional server-side sessions using cookies for session ID storage
2. **JWT-based Sessions**: Stateless sessions using JWT tokens (see [JWT Authentication](../auth/jwt.md))

## Architecture Overview

The session system consists of three main components:

- **SessionData**: Core data structure for storing session values (`provider/httpserver/session/session_data.go`)
- **SessionManager**: Middleware and session lifecycle management (`provider/httpserver/session/middleware.go`)
- **Store**: Backend storage abstraction supporting various KV stores (`provider/httpserver/session/store.go`)

> **Integration Note**: JWT-based sessions use the same `SessionData` structure but store the data within JWT claims instead of server-side storage. This provides a consistent API regardless of session type.

## Features

### Core Session Features
- **Flexible Storage**: Support for any KV backend (memory, Redis, etc.)
- **Type-safe Access**: Typed getters for common data types (string, int, bool)
- **Flash Messages**: One-time messages that persist across requests
- **Session Regeneration**: Built-in protection against session fixation
- **Automatic Cleanup**: Configurable cleanup intervals for expired sessions

### Cookie Configuration
- **Security Flags**: HttpOnly, Secure, and SameSite support
- **Flexible Expiration**: Separate expiration and idle timeout settings
- **Domain/Path Control**: Fine-grained cookie scope configuration

### JWT Integration
- **Seamless API**: Same session interface for both cookie and JWT sessions
- **Stateless Operation**: JWT tokens carry session data in claims
- **Enhanced Security**: Support for asymmetric algorithms and token revocation
- **JWKS Support**: Public key distribution for third-party verification

## Using Sessions

Blueprint provides several session management options, all with a consistent API for session data access.

### Option 1: Memory-based Cookie Sessions

For simple applications or development environments:

```go
// configure logger
logger := log.New("session-sample")

// Configure session
sessionConfig := session.NewConfig()

// session backend
backend := kv.NewMemoryKV()

// Use session middleware with memory store
sessionManager := server.UseSession(sessionConfig, backend, logger)
```

### Option 2: Redis-based Cookie Sessions

For distributed applications with multiple server instances:

```go
// configure logger
logger := log.New("session-sample")

// Configure session
sessionConfig := session.NewConfig()

// Configure Redis
redisConfig := redis.NewConfig()
redisConfig.Address = "localhost:6379"

// redis client
backend, err := redis.NewClient(redisConfig)
utils.PanicOnError(err)

// Use session middleware with Redis store
sessionManager, err := server.UseSession(sessionConfig, backend, logger)
if err != nil {
    logger.Fatal(err, "could not connect to Redis")
    os.Exit(1)
}
```

### Option 3: JWT-based Token Sessions

For stateless, API-focused applications:

```go
// configure logger
logger := log.New("session-sample")

// Configure JWT (from provider/auth/jwt package)
jwtConfig := jwt.NewJWTConfig(jwt.RandomJWTKey())
jwtConfig.SigningAlgorithm = "HS256" // or RS256, ES256, EdDSA
jwtConfig.ExpirationSeconds = 3600

// Use JWT session middleware
jwtManager, err := server.UseJWTSession(jwtConfig, logger)
if err != nil {
    logger.Fatal(err, "could not create JWT session manager")
    os.Exit(1)
}
```

> **Note**: JWT configuration is now handled by the `provider/auth/jwt` package. See the [JWT Authentication documentation](../auth/jwt.md) for advanced features like asymmetric algorithms, JWKS, and token revocation.

## Session Configuration

The `Config` struct in `provider/httpserver/session/config.go` contains all configuration options for cookie-based sessions:

```go
type Config struct {
    CookieName             string `json:"cookieName"`             // Cookie name for session ID (default: "blueprint_session")
    ExpirationSeconds      int    `json:"expirationSeconds"`      // Maximum session lifetime (default: 1800 = 30 min)
    IdleTimeoutSeconds     int    `json:"idleTimeoutSeconds"`     // Maximum idle time (default: 900 = 15 min)
    Secure                 bool   `json:"secure"`                 // HTTPS-only cookies (default: true)
    HttpOnly               bool   `json:"httpOnly"`               // No JS access (default: true)
    SameSite               int    `json:"sameSite"`               // CSRF protection (default: Strict)
    Domain                 string `json:"domain"`                 // Cookie domain scope
    Path                   string `json:"path"`                   // Cookie path scope (default: "/")
    CleanupIntervalSeconds int    `json:"cleanupIntervalSeconds"` // Cleanup frequency (default: 300 = 5 min)
}
```

### Default Configuration

```go
const (
    DefaultSessionCookieName  = "blueprint_session"  // Cookie name
    DefaultSessionExpiration  = 1800                  // 30 minutes
    DefaultSessionIdleTimeout = 900                   // 15 minutes
    DefaultSecure             = true                  // HTTPS only
    DefaultHttpOnly           = true                  // No JS access
    DefaultSameSite           = http.SameSiteStrictMode
    DefaultCleanupInterval    = 300                   // 5 minutes
)
```

### Configuration Validation

The configuration includes built-in validation:

```go
func (c *Config) Validate() error {
    // Validates positive integers for timeouts
    // Validates SameSite values
    // Returns specific errors for invalid configurations
}
```

## Working with Sessions

### Session Data Structure

All session types use the same `SessionData` structure:

```go
type SessionData struct {
    Values       map[string]any
    LastAccessed time.Time
    Created      time.Time
    ID           string
}
```

### Reading and Writing Session Data

```go
// Get the session from the gin context
sess := session.Get(c)

// Store a value
sess.Set("user_id", 123)

// Get a value
userId, ok := sess.GetInt("user_id")
if ok {
    // Use userId
}

// Delete a value
sess.Delete("user_id")

// Check if a key exists
if sess.Has("user_id") {
    // Key exists
}
```

### Typed Getters

The `SessionData` struct provides typed getters for common data types:

```go
sess := session.Get(c)

// Get string
str, ok := sess.GetString("name")

// Get int
num, ok := sess.GetInt("count")

// Get bool
val, ok := sess.GetBool("enabled")

// Get any value
val, ok := sess.Get("complex")
```

### Flash Messages

Flash messages are one-time values that persist only until retrieved:

```go
sess := session.Get(c)

// Set a flash message
sess.FlashString("Operation completed successfully")

// Get a flash message (automatically removes it)
message, ok := sess.GetFlashString()

// Generic flash for non-string values
sess.Flash(complexData)
data, ok := sess.GetFlash()
```

### Security Operations

#### Session Regeneration

To prevent session fixation attacks, you can regenerate the session ID while preserving session data:

```go
// Regenerate the session
sessionManager.Regenerate(c)
```

This is typically done after login/authentication.

#### Clearing a Session

To completely clear a session (e.g., at logout):

```go
// Clear the session
sessionManager.Clear(c)
```

## Full Examples

### Cookie-based Sessions
See `/samples/session/` for a complete example demonstrating:
- Session creation and management
- Flash messages
- Session regeneration
- CSRF integration

### JWT-based Sessions  
See `/samples/jwt-auth/` for a comprehensive JWT example featuring:
- JWT token generation and validation
- Session data in JWT claims
- Token refresh and revocation
- JWKS endpoint for public keys
- Interactive web interface

## Session Storage

The session system uses a flexible storage abstraction that works with any KV backend:

```go
// Store manages session data in a KV backend
type Store struct {
    config   *Config
    backend  kv.KV
    logger   *log.Logger
    stopChan chan struct{}
}

// Create a new store with any KV backend
store := session.NewStore(config, backend, logger)

// Available backends:
// - kv.NewMemoryKV() - In-memory storage
// - redis.NewClient(config) - Redis storage
// - Any implementation of the kv.KV interface
```

### Automatic Cleanup

The store automatically cleans up expired sessions based on the configured interval:
- Runs in a separate goroutine
- Removes sessions older than `ExpirationSeconds`
- Removes sessions idle longer than `IdleTimeoutSeconds`
- Cleanup interval configured via `CleanupIntervalSeconds`

## Best Practices

### Cookie-based Sessions

1. **Security Configuration**
   - Always use `Secure = true` in production (HTTPS only)
   - Keep `HttpOnly = true` to prevent XSS attacks
   - Use `SameSite = Strict` or `Lax` for CSRF protection
   - Enable additional CSRF protection with `server.UseCSRFProtection()`

2. **Session Management**
   - Regenerate session ID after authentication
   - Clear sessions explicitly on logout
   - Use appropriate expiration and idle timeouts
   - Configure cleanup intervals based on traffic

3. **Storage Selection**
   - Use in-memory storage for development/single-instance
   - Use Redis for distributed/production deployments
   - Consider custom KV backends for specific requirements

### JWT-based Sessions

1. **Algorithm Selection**
   - Use asymmetric algorithms (RS256, ES256, EdDSA) for production
   - Reserve HMAC (HS256) for simple, trusted environments
   - Enable JWKS for public key distribution

2. **Security Configuration**
   - Use strong keys (min 2048-bit for RSA)
   - Enable issuer and audience validation
   - Implement token revocation for sensitive apps
   - Use short expiration times (15-60 minutes)

3. **Integration Guidelines**
   - Use Authorization header, not cookies
   - Implement automatic token refresh
   - Handle token expiration gracefully
   - See [JWT Authentication](../auth/jwt.md) for detailed configuration

## Session Middleware Integration

The `SessionManager` provides Gin middleware for automatic session handling:

```go
type SessionManager struct {
    store  *Store
    config *Config
    logger *log.Logger
}

// Middleware automatically:
// 1. Loads existing sessions from cookies
// 2. Creates new sessions for new visitors
// 3. Saves session changes after request processing
// 4. Manages cookie lifecycle
```

### Helper Functions

```go
// Get session from Gin context
sess := session.Get(c)

// Regenerate session ID (e.g., after login)
manager.Regenerate(c)

// Clear session and remove cookie
manager.Clear(c)
```

## CSRF Protection Integration

Cookie sessions integrate seamlessly with CSRF protection:

```go
// 1. Set up session management
logger := log.New("app")
backend := kv.NewMemoryKV()
sessionConfig := session.NewConfig()
manager := server.UseSession(sessionConfig, backend, logger)

// 2. Enable CSRF protection
server.UseCSRFProtection()

// 3. Generate CSRF token in handlers
router.GET("/form", func(c *gin.Context) {
    csrfToken := security.GenerateCSRFToken(c)
    c.HTML(http.StatusOK, "form.html", gin.H{
        "csrfToken": csrfToken,
    })
})

// 4. Include token in forms
// <input type="hidden" name="_csrf" value="{{ .csrfToken }}">
```

## Migration from JWT in Session Package

If you have existing code using JWT from the old session package location:

```go
// OLD: JWT types were in session package
import "github.com/oddbit-project/blueprint/provider/httpserver/session"
jwtConfig := session.NewJWTConfig()

// NEW: JWT functionality moved to dedicated package
import "github.com/oddbit-project/blueprint/provider/auth/jwt"
jwtConfig := jwt.NewJWTConfig(jwt.RandomJWTKey())
```

The session API remains the same - only the JWT configuration and management has moved. See [JWT Authentication](../auth/jwt.md) for the complete JWT documentation.