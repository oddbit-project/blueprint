# Cookie-Based Session Management

Blueprint provides a flexible cookie-based session management system for HTTP applications with support for multiple storage backends and comprehensive security features.

## Architecture Overview

The session system consists of four main components:

- **SessionData** (`session_data.go`): Core data structure for storing session values with typed accessors
- **SessionManager** (`middleware.go`): Gin middleware for automatic session lifecycle management
- **Store** (`store.go`): Backend storage abstraction with TTL support and automatic cleanup
- **Config** (`config.go`): Comprehensive configuration with security defaults and validation

## Features

### Core Session Features
- **Flexible Storage**: Support for any KV backend (memory, Redis, or custom implementations)
- **Type-safe Access**: Typed getters for common data types (string, int, bool)
- **Flash Messages**: One-time messages that persist across requests
- **Session Regeneration**: Built-in protection against session fixation attacks
- **Automatic Cleanup**: Configurable cleanup intervals for expired sessions
- **Dual Expiration**: Both absolute expiration and idle timeout support

### Cookie Configuration
- **Security Flags**: HttpOnly, Secure, and SameSite support
- **Flexible Expiration**: Separate expiration and idle timeout settings
- **Domain/Path Control**: Fine-grained cookie scope configuration
- **Automatic Management**: Middleware handles all cookie lifecycle operations

## Using Sessions

### Option 1: Memory-based Sessions

For simple applications or development environments:

```go
// Configure logger
logger := log.New("session-sample")

// Configure session
sessionConfig := session.NewConfig()

// Create memory backend
backend := kv.NewMemoryKV()

// Use session middleware with memory store
sessionManager := server.UseSession(sessionConfig, backend, logger)
```

### Option 2: Redis-based Sessions

For distributed applications with multiple server instances:

```go
// Configure logger
logger := log.New("session-sample")

// Configure session
sessionConfig := session.NewConfig()

// Configure Redis
redisConfig := redis.NewConfig()
redisConfig.Address = "localhost:6379"

// Create Redis backend
backend, err := redis.NewClient(redisConfig)
if err != nil {
    logger.Fatal(err, "could not connect to Redis")
    os.Exit(1)
}

// Use session middleware with Redis store
sessionManager := server.UseSession(sessionConfig, backend, logger)
```

### Option 3: Custom Backend

You can implement your own storage backend by implementing the `kv.KV` interface:

```go
type KV interface {
    SetTTL(key string, value []byte, ttl time.Duration) error
    Set(key string, value []byte) error
    Get(key string) ([]byte, error)
    Delete(key string) error
    Prune() error
}
```

## Session Configuration

The `Config` struct in `provider/httpserver/session/config.go` contains all configuration options:

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
    DefaultSessionCookieName  = "blueprint_session"
    DefaultSessionExpiration  = 1800  // 30 minutes
    DefaultSessionIdleTimeout = 900   // 15 minutes  
    DefaultSecure             = true  // HTTPS only
    DefaultHttpOnly           = true  // No JS access
    DefaultSameSite           = http.SameSiteStrictMode
    DefaultCleanupInterval    = 300   // 5 minutes
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

// Get a value with type assertion
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

To prevent session fixation attacks, regenerate the session ID while preserving session data:

```go
// Regenerate the session (typically after login)
sessionManager.Regenerate(c)
```

#### Clearing a Session

To completely clear a session (e.g., at logout):

```go
// Clear the session
sessionManager.Clear(c)
```

## Full Example

See `/samples/session/` for a complete example demonstrating:
- Session creation and management
- Flash messages
- Session regeneration
- CSRF integration

## Session Storage

The session system uses a flexible storage abstraction that works with any KV backend implementing the `kv.KV` interface:

```go
// Store manages session data with automatic serialization and expiration
type Store struct {
    backend        kv.KV
    config         *Config
    stopCleanup    chan bool
    cleanupTicker  *time.Ticker
    cleanupMutex   sync.Mutex
    cleanupRunning bool
    logger         *log.Logger
}

// Create a new store with any KV backend
store := session.NewStore(config, backend, logger)

// Available backends:
// - kv.NewMemoryKV() - In-memory storage (default)
// - redis.NewClient(config) - Redis storage
// - Any implementation of the kv.KV interface
```

### Key Features

1. **Automatic Serialization**: Uses `encoding/gob` for efficient binary serialization
2. **TTL Management**: Automatically sets TTL based on the smaller of expiration or idle timeout
3. **Concurrent Safety**: Thread-safe operations with mutex protection for cleanup
4. **Graceful Shutdown**: Proper cleanup goroutine management with `StopCleanup()`

### Automatic Cleanup

The store automatically cleans up expired sessions:
- Runs in a separate goroutine started by `StartCleanup()`
- Calls `backend.Prune()` to remove expired entries
- Cleanup interval configured via `CleanupIntervalSeconds` (default: 5 minutes)
- Safe to start/stop multiple times with mutex protection

## Best Practices

### Security Configuration

1. **Production Settings**
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
// 3. Updates LastAccessed time on each request
// 4. Saves session changes after request processing
// 5. Manages cookie lifecycle and security settings
```

### Middleware Flow

1. **Session Loading**: Attempts to load session from cookie ID
2. **Session Creation**: Creates new session if none exists or expired
3. **Context Storage**: Stores session in Gin context for handler access
4. **Post-Processing**: Saves any session modifications after handlers complete

### Cookie Security

The middleware properly sets cookie attributes including:
- **SameSite**: Handled via header manipulation for Gin compatibility
- **Secure/HttpOnly**: Set according to configuration
- **Domain/Path**: Scoped according to configuration
- **Max-Age**: Set to `ExpirationSeconds`

### Helper Functions

```go
// Get session from Gin context
sess := session.Get(c)

// Regenerate session ID (e.g., after login)
// - Creates new session with same data
// - Deletes old session
// - Updates cookie with new ID
manager.Regenerate(c)

// Clear session and remove cookie
// - Deletes session from store
// - Sets cookie with negative Max-Age
// - Removes from context
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

## Technical Implementation Details

### Session ID Generation

Session IDs are generated using cryptographically secure random numbers:

```go
// utils.go - generateSessionID()
func generateSessionID() string {
    buf := make([]byte, 128)  // 128 bytes of random data
    rand.Read(buf)            // Uses crypto/rand
    return base64.URLEncoding.EncodeToString(buf)
}
```

### Session Expiration Logic

The store implements dual expiration checking:

1. **Absolute Expiration**: Sessions older than `ExpirationSeconds` are expired
2. **Idle Timeout**: Sessions not accessed for `IdleTimeoutSeconds` are expired
3. **TTL Setting**: Uses the smaller of the two timeouts for backend TTL

```go
// From store.go Get() method:
// Check absolute expiration
if now.Sub(session.Created) > time.Duration(s.config.ExpirationSeconds)*time.Second {
    s.backend.Delete(id)
    return nil, ErrSessionExpired
}

// Check idle timeout
if now.Sub(session.LastAccessed) > time.Duration(s.config.IdleTimeoutSeconds)*time.Second {
    s.backend.Delete(id)
    return nil, ErrSessionExpired
}
```

### Error Handling

The session system defines specific errors for better debugging:

```go
const (
    ErrInvalidExpirationSeconds      = utils.Error("session expiration seconds must be a positive integer")
    ErrInvalidIdleTimeoutSeconds     = utils.Error("session idle timeout seconds must be a positive integer")
    ErrInvalidSameSite               = utils.Error("invalid sameSite value")
    ErrInvalidCleanupIntervalSeconds = utils.Error("session cleanup interval seconds must be a positive integer")
    ErrSessionNotFound               = utils.Error("session not found")
    ErrSessionExpired                = utils.Error("session expired")
)
```

## Performance Considerations

1. **Serialization**: Gob encoding is used for efficiency, but register custom types with `gob.Register()`
2. **Cleanup Frequency**: Balance between memory usage and CPU overhead
3. **Backend Selection**: 
   - Memory: Fastest but not distributed
   - Redis: Distributed with network overhead
   - Custom: Optimize for your specific use case
4. **Session Size**: Keep session data minimal to reduce serialization overhead
5. **ID Generation**: 128 bytes provides strong security with minimal performance impact

## Troubleshooting

### Common Issues

1. **Sessions Not Persisting**
   - Check cookie settings match your environment (Secure flag for HTTPS)
   - Verify backend is properly configured and accessible
   - Ensure middleware is added before route handlers

2. **Session Expiration**
   - Review `ExpirationSeconds` and `IdleTimeoutSeconds` settings
   - Check system time synchronization in distributed setups
   - Monitor cleanup logs for errors

3. **Cookie Issues**
   - Verify domain/path settings match your application
   - Check browser developer tools for cookie errors
   - Ensure SameSite settings are appropriate for your use case

### Debug Logging

Enable debug logging to troubleshoot session issues:

```go
logger := log.New("session")
logger.SetLevel(log.LevelDebug)
sessionManager := server.UseSession(config, backend, logger)
```