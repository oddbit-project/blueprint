# Session Management

Blueprint provides built-in session management for HTTP applications. The session management system is designed to be flexible and secure, with support for multiple storage backends:

1. **Cookie-based Sessions**: Using in-memory or Redis storage
2. **JWT-based Sessions**: Using stateless JSON Web Tokens

> **Note**: The cookie-based session management integrates well with Blueprint's built-in CSRF protection. For enhanced security when using cookies, it's recommended to use both features together.

## Features

- Multiple session mechanisms:
  - Cookie-based sessions with server-side storage
  - JWT-based sessions using Authorization header
- Multiple storage backends:
  - In-memory session store
  - Redis session store
  - JWT stateless tokens
- Security features:
  - Automatic session expiration and cleanup
  - Session regeneration (prevents session fixation attacks)
  - Secure cookies with configurable options (HttpOnly, Secure, SameSite, etc.)
- Developer-friendly features:
  - Flash messages
  - Type-safe getters and setters
  - Consistent API across all storage types

## Using Sessions

Blueprint provides several session management options, all with a consistent API for session data access.

### Option 1: Memory-based Cookie Sessions

For simple applications or development environments:

```go
// Configure session
sessionConfig := session.DefaultSessionConfig()
sessionConfig.Logger = logger

// Use session middleware with memory store
sessionManager := server.UseSessionWithMemoryStore(sessionConfig)
```

### Option 2: Redis-based Cookie Sessions

For distributed applications with multiple server instances:

```go
// Configure session
sessionConfig := session.DefaultSessionConfig()
sessionConfig.Logger = logger

// Configure Redis
redisConfig := session.DefaultRedisConfig()
redisConfig.Address = "localhost:6379"

// Use session middleware with Redis store
sessionManager, err := server.UseSessionWithRedisStore(sessionConfig, redisConfig)
if err != nil {
    logger.Fatal(err, "could not connect to Redis")
    os.Exit(1)
}
```

### Option 3: JWT-based Token Sessions

For stateless, API-focused applications:

```go
// Configure session
sessionConfig := session.DefaultSessionConfig()
sessionConfig.Logger = logger

// Configure JWT
jwtConfig := session.DefaultJWTConfig()
jwtConfig.SigningKey = []byte("your-secure-signing-key")
jwtConfig.Logger = logger

// Use session middleware with JWT
jwtManager, err := server.UseSessionWithJWT(sessionConfig, jwtConfig)
if err != nil {
    logger.Fatal(err, "could not create JWT session manager")
    os.Exit(1)
}
```

## Session Configuration

The `SessionConfig` struct contains all the configuration options for sessions:

```go
type SessionConfig struct {
    // CookieName is the name of the cookie used to store the session ID
    CookieName string

    // Expiration is the maximum lifetime of a session
    Expiration time.Duration

    // IdleTimeout is the maximum time a session can be inactive
    IdleTimeout time.Duration

    // MaxSessions is the maximum number of sessions to store in memory
    MaxSessions int

    // Secure sets the Secure flag on cookies (should be true in production)
    Secure bool

    // HttpOnly sets the HttpOnly flag on cookies (should be true)
    HttpOnly bool

    // SameSite sets the SameSite policy for cookies
    SameSite http.SameSite

    // Domain sets the domain for the cookie
    Domain string

    // Path sets the path for the cookie
    Path string

    // CleanupInterval sets how often the session cleanup runs
    CleanupInterval time.Duration

    // Logger for the session store
    Logger *log.Logger
}
```

Default sensible options are provided by `DefaultSessionConfig()`:

```go
func DefaultSessionConfig() *SessionConfig {
    return &SessionConfig{
        CookieName:      "blueprint_session",
        Expiration:      30 * time.Minute,
        IdleTimeout:     15 * time.Minute,
        MaxSessions:     10000,
        Secure:          true,
        HttpOnly:        true,
        SameSite:        http.SameSiteStrictMode,
        Path:            "/",
        Domain:          "",
        CleanupInterval: 5 * time.Minute,
        Logger:          nil,
    }
}
```

## Working with Sessions

### Reading and Writing Session Data

```go
// Get the session
session := session.Get(c)

// Store a value
session.Set(c, "user_id", 123)

// Get a value
userId, ok := session.GetInt(c, "user_id")
if ok {
    // Use userId
}

// Delete a value
session.Delete(c, "user_id")

// Check if a key exists
if session.Has(c, "user_id") {
    // Key exists
}
```

### Typed Getters

The session package provides typed getters for convenience:

```go
// Get string
str, ok := session.GetString(c, "name")

// Get int
num, ok := session.GetInt(c, "count")

// Get bool
val, ok := session.GetBool(c, "enabled")

// Get any value
val, ok := session.GetValue(c, "complex")
```

### Flash Messages

Flash messages are temporary messages that are typically used to display one-time notifications:

```go
// Set a flash message
session.FlashString(c, "message", "Operation completed successfully")

// Get a flash message (this will remove it from the session)
message, ok := session.GetFlashString(c, "message")
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

## Session Stores

Blueprint provides two session store implementations:

### MemoryStore

The `MemoryStore` keeps sessions in memory. It's easy to set up but not suitable for distributed applications.

```go
store := session.NewMemoryStore(config)
```

### RedisStore

The `RedisStore` stores sessions in Redis, making it suitable for distributed applications.

```go
store, err := session.NewRedisStore(sessionConfig, redisConfig)
if err != nil {
    // Handle error
}
```

## Full Examples

- **Cookie-based Sessions**: See `/sample/session/main.go` for a complete example of cookie-based session usage
- **JWT-based Sessions**: See `/sample/jwt_session/main.go` for a RESTful API example using JWT sessions

## JWT Configuration

When using JWT-based sessions, you can configure the token behavior using the `JWTConfig`:

```go
type JWTConfig struct {
    // SigningKey is the key used to sign JWT tokens
    SigningKey []byte

    // SigningMethod is the method used to sign the token
    SigningMethod jwt.SigningMethod

    // Expiration is the expiration time for tokens
    Expiration time.Duration

    // Issuer is the issuer of the token
    Issuer string

    // Audience is the audience of the token
    Audience string

    // Logger for operations
    Logger *log.Logger
}
```

Default values are provided by `DefaultJWTConfig()`:

```go
func DefaultJWTConfig() *JWTConfig {
    return &JWTConfig{
        SigningKey:    nil, // Must be set by the user
        SigningMethod: jwt.SigningMethodHS256,
        Expiration:    time.Hour * 24, // 24 hours
        Issuer:        "blueprint",
        Audience:      "api",
        Logger:        nil,
    }
}
```

## Best Practices

### For Cookie-based Sessions:

1. Always use HTTPS in production with `config.Secure = true`
2. Use `HttpOnly` cookies to prevent JavaScript access to session IDs
3. Use appropriate `SameSite` settings (Strict or Lax) to prevent CSRF attacks
4. Enable CSRF protection with `server.UseCSRFProtection()`
5. Regenerate sessions after login to prevent session fixation
6. Clear sessions at logout
7. Use Redis store for distributed applications

### For JWT-based Sessions:

1. Use a strong, secure signing key (at least 32 bytes)
2. Store signing keys securely, not in source code
3. Set appropriate token expiration time
4. Regenerate tokens regularly for sensitive operations
5. Use HTTPS for all API communication
6. Implement token revocation for logout if needed (requires additional storage)
7. Consider using asymmetric keys (RS256) for production applications

## Integrating Cookie Sessions with CSRF Protection

Blueprint's session management works well with the built-in CSRF protection. Here's how to set up both:

```go
// 1. Set up session management
sessionConfig := session.DefaultSessionConfig()
sessionManager := server.UseSessionWithMemoryStore(sessionConfig)

// 2. Enable CSRF protection
server.UseCSRFProtection()

// 3. In your handler, generate and provide CSRF token
router.GET("/form", func(c *gin.Context) {
    // Generate CSRF token
    csrfToken := security.GenerateCSRFToken(c)
    
    // Render the form with the CSRF token
    c.HTML(http.StatusOK, "form.html", gin.H{
        "csrfToken": csrfToken,
    })
})

// 4. In your HTML form
// <form method="POST" action="/submit">
//     <input type="hidden" name="_csrf" value="{{ .csrfToken }}">
//     <!-- other form fields -->
// </form>
```

This integration ensures that:
- Each user session has its own CSRF token
- Form submissions are protected against CSRF attacks
- The CSRF token is verified server-side before processing the request