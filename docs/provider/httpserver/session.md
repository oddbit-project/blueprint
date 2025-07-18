# Advanced Session Management

Blueprint provides a cookie-based session management system with encryption, multiple storage backends, and security features.

> Note: when using custom types with sessions, **always** register the types for serialization/deserialization with
> gob.Register()

## Architecture Overview

The session system consists of four main components:

- **SessionData** (`session_data.go`): Core data structure with typed accessors and identity management
- **SessionManager** (`middleware.go`): Gin middleware for automatic session lifecycle management
- **Store** (`store.go`): Backend storage abstraction with encryption and automatic cleanup
- **Config** (`config.go`): Comprehensive configuration with security defaults and validation

## Features

### Core Session Features
- **Flexible Storage**: Support for any KV backend (memory, Redis, or custom implementations)
- **Session Encryption**: Optional AES256GCM encryption for cookie data
- **Type-safe Access**: Typed getters for common data types (string, int, bool)
- **Identity Management**: Built-in user identity support with dedicated methods
- **Flash Messages**: One-time messages that persist across requests
- **Session Regeneration**: Built-in protection against session fixation attacks
- **Automatic Cleanup**: Configurable cleanup intervals for expired sessions
- **Dual Expiration**: Both absolute expiration and idle timeout support

### Security Features
- **Cookie Security**: HttpOnly, Secure, and SameSite configuration
- **Session Fixation Protection**: Regenerate session IDs on authentication
- **Encryption Support**: Optional AES256GCM encryption for sensitive session data
- **Secure Defaults**: Production-ready security settings out of the box

## Session Setup

### Option 1: Memory-based Sessions

```go
package main

import (
    "github.com/oddbit-project/blueprint/provider/httpserver"
    "github.com/oddbit-project/blueprint/provider/httpserver/session"
    "github.com/oddbit-project/blueprint/provider/kv"
    "github.com/oddbit-project/blueprint/log"
)

func main() {
    logger := log.New("session-app")
    
    // Server setup
    config := httpserver.NewConfig()
    server := httpserver.NewServer(config, logger)
    
    // Session configuration
    sessionConfig := session.NewConfig()
    sessionConfig.CookieName = "my_session"
    sessionConfig.ExpirationSeconds = 3600 // 1 hour
    sessionConfig.IdleTimeoutSeconds = 1800 // 30 minutes
    
    // Memory backend for development
    backend := kv.NewMemoryKV()
    
    // Setup session middleware
    sessionManager, err := server.UseSession(sessionConfig, backend, logger)
    if err != nil {
        logger.Fatal(err, "failed to setup sessions")
    }
    
    setupRoutes(server)
    server.Start()
}
```

### Option 2: Redis-based Sessions

```go
func setupRedisSession(server *httpserver.Server, logger *log.Logger) {
    // Redis configuration
    redisConfig := redis.NewConfig()
    redisConfig.Address = "redis:6379"
    redisConfig.Database = 1
    redisConfig.PoolSize = 10
    
    // Create Redis client
    redisClient, err := redis.NewClient(redisConfig)
    if err != nil {
        logger.Fatal(err, "failed to connect to Redis")
    }
    
    // Session configuration with encryption
    sessionConfig := session.NewConfig()
    sessionConfig.ExpirationSeconds = 7200 // 2 hours
    sessionConfig.EncryptionKey = secure.DefaultCredentialConfig{
        PasswordEnvVar: "SESSION_ENCRYPTION_KEY",
    }
    
    // Setup session middleware with Redis
    sessionManager, err := server.UseSession(sessionConfig, redisClient, logger)
    if err != nil {
        logger.Fatal(err, "failed to setup Redis sessions")
    }
}
```

### Option 3: Manual Session Setup (Advanced)

```go
func setupAdvancedSession(server *httpserver.Server, logger *log.Logger) {
    // Custom backend
    backend := kv.NewMemoryKV()
    
    // Session configuration
    sessionConfig := session.NewConfig()
    sessionConfig.Secure = true
    sessionConfig.HttpOnly = true
    sessionConfig.SameSite = int(http.SameSiteStrictMode)
    sessionConfig.CleanupIntervalSeconds = 300
    
    // Create store manually
    sessionStore, err := session.NewStore(sessionConfig, backend, logger)
    if err != nil {
        logger.Fatal(err, "failed to create session store")
    }
    
    // Create manager manually with options
    sessionManager, err := session.NewManager(sessionConfig,
        session.ManagerWithStore(sessionStore),
        session.ManagerWithLogger(logger))
    if err != nil {
        logger.Fatal(err, "failed to create session manager")
    }
    
    // Add middleware
    server.Router().Use(sessionManager.Middleware())
}
```

## Session Configuration

### Complete Configuration Reference

```go
type Config struct {
    // Cookie configuration
    CookieName             string `json:"cookieName"`             // Cookie name (default: "blueprint_session")
    Domain                 string `json:"domain"`                 // Cookie domain scope
    Path                   string `json:"path"`                   // Cookie path scope (default: "/")
    
    // Security configuration
    Secure                 bool   `json:"secure"`                 // HTTPS only (default: true)
    HttpOnly               bool   `json:"httpOnly"`               // No JS access (default: true)
    SameSite               int    `json:"sameSite"`               // CSRF protection (default: Strict)
    
    // Expiration configuration
    ExpirationSeconds      int    `json:"expirationSeconds"`      // Session lifetime (default: 1800)
    IdleTimeoutSeconds     int    `json:"idleTimeoutSeconds"`     // Idle timeout (default: 900)
    CleanupIntervalSeconds int    `json:"cleanupIntervalSeconds"` // Cleanup frequency (default: 300)
    
    // Encryption configuration (optional)
    EncryptionKey          secure.DefaultCredentialConfig `json:"encryptionKey"`
}
```

### Security Defaults

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

### Production Configuration Example

```json
{
  "session": {
    "cookieName": "app_session",
    "expirationSeconds": 7200,
    "idleTimeoutSeconds": 3600,
    "secure": true,
    "httpOnly": true,
    "sameSite": 1,
    "domain": ".example.com",
    "path": "/",
    "encryptionKey": {
      "passwordEnvVar": "SESSION_ENCRYPTION_KEY"
    },
    "cleanupIntervalSeconds": 300
  }
}
```

## Working with Sessions

### Session Data Structure

```go
type SessionData struct {
    Values       map[string]any `json:"values"`
    LastAccessed time.Time      `json:"lastAccessed"`
    Created      time.Time      `json:"created"`
    ID           string         `json:"id"`
}
```

### Basic Session Operations

```go
func sessionHandler(c *gin.Context) {
    // Get session from context
    sess := session.Get(c)
    
    // Store values
    sess.Set("user_id", 123)
    sess.Set("username", "john_doe")
    sess.Set("preferences", map[string]any{
        "theme": "dark",
        "language": "en",
    })
    
    // Retrieve values with type safety
    userID, ok := sess.GetInt("user_id")
    if ok {
        logger.Info("User ID", "id", userID)
    }
    
    username, ok := sess.GetString("username")
    if ok {
        logger.Info("Username", "username", username)
    }
    
    // Check existence
    if sess.Has("preferences") {
        prefs, _ := sess.Get("preferences")
        logger.Info("User preferences", "prefs", prefs)
    }
    
    // Delete values
    sess.Delete("temporary_data")
    
    c.JSON(200, gin.H{
        "session_id": sess.ID,
        "user_id": userID,
        "username": username,
    })
}
```

### Identity Management

```go
// Custom identity type
type UserIdentity struct {
    ID       int    `json:"id"`
    Username string `json:"username"`
    Email    string `json:"email"`
    Roles    []string `json:"roles"`
}

// Register with GOB for serialization
func init() {
    gob.Register(&UserIdentity{})
}

func loginHandler(c *gin.Context) {
    var loginReq struct {
        Username string `json:"username"`
        Password string `json:"password"`
    }
    
    if err := c.ShouldBindJSON(&loginReq); err != nil {
        c.JSON(400, gin.H{"error": "Invalid request"})
        return
    }
    
    // Authenticate user (your authentication logic)
    user, err := authenticateUser(loginReq.Username, loginReq.Password)
    if err != nil {
        c.JSON(401, gin.H{"error": "Invalid credentials"})
        return
    }
    
    // Get session
    sess := session.Get(c)
    
    // Set user identity
    identity := &UserIdentity{
        ID:       user.ID,
        Username: user.Username,
        Email:    user.Email,
        Roles:    user.Roles,
    }
    sess.SetIdentity(identity)
    
    // Regenerate session ID for security
    sessionManager.Regenerate(c)
    
    c.JSON(200, gin.H{"message": "Login successful"})
}

func getCurrentUser(c *gin.Context) *UserIdentity {
    sess := session.Get(c)
    identity, ok := sess.GetIdentity()
    if !ok {
        return nil
    }
    
    user, ok := identity.(*UserIdentity)
    if !ok {
        return nil
    }
    
    return user
}

func protectedHandler(c *gin.Context) {
    user := getCurrentUser(c)
    if user == nil {
        c.JSON(401, gin.H{"error": "Not authenticated"})
        return
    }
    
    c.JSON(200, gin.H{
        "user": user,
        "message": "Access granted",
    })
}

func logoutHandler(c *gin.Context) {
    sess := session.Get(c)
    sess.DeleteIdentity()
    
    // Clear entire session
    sessionManager.Clear(c)
    
    c.JSON(200, gin.H{"message": "Logged out"})
}
```

### Flash Messages

```go
func setFlashMessage(c *gin.Context) {
    sess := session.Get(c)
    
    // Set flash message
    sess.FlashString("Operation completed successfully!")
	
    c.Redirect(302, "/dashboard")
}

func displayFlashMessage(c *gin.Context) {
    sess := session.Get(c)
    
    // Get simple flash message
    message, ok := sess.GetFlashString()
    if ok {
        c.HTML(200, "dashboard.html", gin.H{
            "flash_message": message,
        })
        return
    }
	
    // No flash messages
    c.HTML(200, "dashboard.html", gin.H{})
}
```

## Security Operations

### Session Regeneration (IMPORTANT)

```go
func loginHandler(c *gin.Context) {
    // ... authentication logic ...
    
    sess := session.Get(c)
    sess.SetIdentity(user)
    
    // IMPORTANT: Regenerate session ID after authentication
    // This prevents session fixation attacks
    sessionManager.Regenerate(c)
    
    c.JSON(200, gin.H{"message": "Login successful"})
}

func elevatePrivilegesHandler(c *gin.Context) {
    // When user gains elevated privileges, regenerate session
    sess := session.Get(c)
    
    // Update user role
    user := getCurrentUser(c)
    user.Roles = append(user.Roles, "admin")
    sess.SetIdentity(user)
    
    // Regenerate session for security
    sessionManager.Regenerate(c)
    
    c.JSON(200, gin.H{"message": "Privileges elevated"})
}
```

### Session Clearing

```go
func logoutHandler(c *gin.Context) {
    // Option 1: Clear entire session
    sessionManager.Clear(c)
    
    c.JSON(200, gin.H{"message": "Logged out"})
}

func partialLogoutHandler(c *gin.Context) {
    sess := session.Get(c)
    
    // Option 2: Clear only identity but keep other session data
    sess.DeleteIdentity()
    
    // Keep non-sensitive data like preferences
    c.JSON(200, gin.H{"message": "Logged out, preferences retained"})
}
```

## Session Encryption

### Encryption Configuration

```go
// Environment variable approach
sessionConfig.EncryptionKey = secure.DefaultCredentialConfig{
    PasswordEnvVar: "SESSION_ENCRYPTION_KEY",
}

// File-based key
sessionConfig.EncryptionKey = secure.DefaultCredentialConfig{
    PasswordFile: "/etc/secrets/session-key",
}

// Direct key (not recommended for production)
sessionConfig.EncryptionKey = secure.DefaultCredentialConfig{
    Password: "your-32-byte-encryption-key-here",
}
```

### Key Generation

```bash
# Generate a secure 32-byte key
openssl rand -base64 32

# Set as environment variable
export SESSION_ENCRYPTION_KEY="generated-key-here"
```

## Backend Storage Options

### Memory Backend (Development)

```go
backend := kv.NewMemoryKV()
// Pros: Fast, simple setup
// Cons: Not persistent, single instance only
```

### Redis Backend (Production)

```go
redisConfig := redis.NewConfig()
redisConfig.Address = "redis-cluster:6379"
redisConfig.Password = "redis-password"
redisConfig.Database = 1
redisConfig.PoolSize = 20

backend, err := redis.NewClient(redisConfig)
// Pros: Distributed, persistent, scalable
// Cons: Network latency, additional infrastructure
```

### Custom Backend

```go
type CustomKV struct {
    // Your implementation
}

func (c *CustomKV) SetTTL(key string, value []byte, ttl time.Duration) error {
    // Store with TTL
    return nil
}

func (c *CustomKV) Get(key string) ([]byte, error) {
    // Retrieve value
    return nil, nil
}

func (c *CustomKV) Delete(key string) error {
    // Delete value
    return nil
}

func (c *CustomKV) Prune() error {
    // Clean up expired entries
    return nil
}
```

## Example: Secure Web Application

```go
package main

import (
    "encoding/gob"
    "net/http"
    "github.com/gin-gonic/gin"
    "github.com/oddbit-project/blueprint/provider/httpserver"
    "github.com/oddbit-project/blueprint/provider/httpserver/auth"
    "github.com/oddbit-project/blueprint/provider/httpserver/session"
    "github.com/oddbit-project/blueprint/provider/httpserver/security"
    "github.com/oddbit-project/blueprint/provider/kv"
    "github.com/oddbit-project/blueprint/log"
)

type UserIdentity struct {
    ID       int      `json:"id"`
    Username string   `json:"username"`
    Email    string   `json:"email"`
    Roles    []string `json:"roles"`
}

func init() {
    // Register custom types for GOB serialization
    gob.Register(&UserIdentity{})
}

func main() {
    logger := log.New("secure-web-app")
    
    // Server configuration
    serverConfig := httpserver.NewConfig()
    serverConfig.Host = "localhost"
    serverConfig.Port = 8443
    serverConfig.CertFile = "server.crt"
    serverConfig.CertKeyFile = "server.key"
    
    server := httpserver.NewServer(serverConfig, logger)
    
    // Session configuration
    sessionConfig := session.NewConfig()
    sessionConfig.CookieName = "secure_session"
    sessionConfig.ExpirationSeconds = 7200 // 2 hours
    sessionConfig.IdleTimeoutSeconds = 1800 // 30 minutes
    sessionConfig.Secure = true
    sessionConfig.HttpOnly = true
    sessionConfig.SameSite = int(http.SameSiteStrictMode)
    sessionConfig.EncryptionKey = secure.DefaultCredentialConfig{
        PasswordEnvVar: "SESSION_ENCRYPTION_KEY",
    }
    
    // Setup session store
    backend := kv.NewMemoryKV() // Use Redis in production
    sessionManager, err := server.UseSession(sessionConfig, backend, logger)
    if err != nil {
        logger.Fatal(err, "failed to setup sessions")
    }
    
    // Security headers
    securityConfig := security.DefaultSecurityConfig()
    securityConfig.CSP = "default-src 'self'; script-src 'self' 'nonce-{nonce}'"
    server.Router().Use(security.SecurityMiddleware(securityConfig))
    
    // CSRF protection
    server.Router().Use(security.CSRFProtection())
    
    // Rate limiting
    server.Router().Use(security.RateLimitMiddleware(rate.Every(time.Second), 10))
    
    // Routes
    setupRoutes(server, sessionManager)
    
    // Start server
    if err := server.Start(); err != nil {
        logger.Fatal(err, "failed to start server")
    }
}

func setupRoutes(server *httpserver.Server, sessionManager *session.Manager) {
    router := server.Router()
    
    // Public routes
    router.GET("/", homeHandler)
    router.GET("/login", loginFormHandler)
    router.POST("/login", loginHandler(sessionManager))
    router.GET("/register", registerFormHandler)
    router.POST("/register", registerHandler)
    
    // Protected routes
    protected := router.Group("/dashboard")
    protected.Use(auth.AuthMiddleware(auth.NewAuthSession(&UserIdentity{})))
    {
        protected.GET("/", dashboardHandler)
        protected.GET("/profile", profileHandler)
        protected.POST("/profile", updateProfileHandler)
        protected.POST("/logout", logoutHandler(sessionManager))
    }
    
    // Admin routes
    admin := router.Group("/admin")
    admin.Use(auth.AuthMiddleware(auth.NewAuthSession(&UserIdentity{})))
    admin.Use(requireRole("admin"))
    {
        admin.GET("/users", listUsersHandler)
        admin.DELETE("/users/:id", deleteUserHandler)
    }
}

func loginHandler(sessionManager *session.Manager) gin.HandlerFunc {
    return func(c *gin.Context) {
        var req struct {
            Username string `json:"username" binding:"required"`
            Password string `json:"password" binding:"required"`
        }
        
        if err := c.ShouldBindJSON(&req); err != nil {
            c.JSON(400, gin.H{"error": "Invalid request"})
            return
        }
        
        // Authenticate user
        user, err := authenticateUser(req.Username, req.Password)
        if err != nil {
            c.JSON(401, gin.H{"error": "Invalid credentials"})
            return
        }
        
        // Get session and set identity
        sess := session.Get(c)
        identity := &UserIdentity{
            ID:       user.ID,
            Username: user.Username,
            Email:    user.Email,
            Roles:    user.Roles,
        }
        sess.SetIdentity(identity)
        
        // Set flash message
        sess.FlashString("Welcome back, " + user.Username + "!")
        
        // Regenerate session ID for security
        sessionManager.Regenerate(c)
        
        c.JSON(200, gin.H{
            "message": "Login successful",
            "redirect": "/dashboard",
        })
    }
}

func dashboardHandler(c *gin.Context) {
    sess := session.Get(c)
    user := getCurrentUser(c)
    
    // Get flash message
    flashMessage, _ := sess.GetFlashString()
    
    c.HTML(200, "dashboard.html", gin.H{
        "user": user,
        "flash": flashMessage,
        "csrf_token": security.GenerateCSRFToken(c),
    })
}

func logoutHandler(sessionManager *session.Manager) gin.HandlerFunc {
    return func(c *gin.Context) {
        // Clear session
        sessionManager.Clear(c)
        
        c.JSON(200, gin.H{
            "message": "Logged out successfully",
            "redirect": "/",
        })
    }
}

func requireRole(role string) gin.HandlerFunc {
    return func(c *gin.Context) {
        user := getCurrentUser(c)
        if user == nil {
            c.AbortWithStatusJSON(401, gin.H{"error": "Not authenticated"})
            return
        }
        
        hasRole := false
        for _, userRole := range user.Roles {
            if userRole == role {
                hasRole = true
                break
            }
        }
        
        if !hasRole {
            c.AbortWithStatusJSON(403, gin.H{"error": "Insufficient privileges"})
            return
        }
        
        c.Next()
    }
}

func getCurrentUser(c *gin.Context) *UserIdentity {
    sess := session.Get(c)
    identity, ok := sess.GetIdentity()
    if !ok {
        return nil
    }
    
    user, ok := identity.(*UserIdentity)
    if !ok {
        return nil
    }
    
    return user
}
```

## Best Practices

### Security Best Practices

1. **Always use HTTPS in production**
```go
sessionConfig.Secure = true
```

2. **Regenerate session ID after authentication**
```go
sessionManager.Regenerate(c)
```

3. **Use encryption for sensitive data**
```go
sessionConfig.EncryptionKey = secure.DefaultCredentialConfig{
    PasswordEnvVar: "SESSION_ENCRYPTION_KEY",
}
```

4. **Set appropriate timeouts**
```go
sessionConfig.ExpirationSeconds = 7200  // 2 hours max
sessionConfig.IdleTimeoutSeconds = 1800 // 30 minutes idle
```

5. **Use SameSite cookies**
```go
sessionConfig.SameSite = int(http.SameSiteStrictMode)
```

### Performance Best Practices

1. **Choose appropriate backend**
   - Memory: Development and single-instance applications
   - Redis: Production and distributed applications

2. **Configure cleanup intervals**
```go
sessionConfig.CleanupIntervalSeconds = 300 // 5 minutes
```

3. **Minimize session data**
   - Store only essential user information
   - Use references to database records instead of full objects

4. **Register custom types with GOB**
```go
func init() {
    gob.Register(&UserIdentity{})
    gob.Register(&CustomType{})
}
```

### Development vs Production

```go
func getSessionConfig(env string) *session.Config {
    config := session.NewConfig()
    
    if env == "production" {
        config.Secure = true
        config.HttpOnly = true
        config.SameSite = int(http.SameSiteStrictMode)
        config.ExpirationSeconds = 7200
        config.EncryptionKey = secure.DefaultCredentialConfig{
            PasswordEnvVar: "SESSION_ENCRYPTION_KEY",
        }
    } else {
        config.Secure = false // Allow HTTP in development
        config.ExpirationSeconds = 86400 // Longer for development
    }
    
    return config
}
```

## Troubleshooting

### Common Issues

1. **Sessions not persisting**
   - Check cookie security settings
   - Verify backend connectivity
   - Ensure middleware order

2. **"gob: type not registered" errors**
   - Register custom types with `gob.Register()`
   - Register in `init()` function

3. **Session expiration issues**
   - Check system time synchronization
   - Review timeout configurations
   - Monitor cleanup logs

4. **Performance issues**
   - Monitor backend latency
   - Optimize session data size
   - Adjust cleanup intervals

### Debug Logging

```go
logger := log.New("session")
logger.SetLevel(log.LevelDebug)
sessionManager := server.UseSession(config, backend, logger)
```
