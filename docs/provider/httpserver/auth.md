# Authentication & Authorization

Blueprint provides an extensible authentication system with multiple provider types, seamless middleware integration, 
and robust security features for protecting HTTP endpoints.

## Architecture Overview

The authentication system is built around a **Provider interface pattern** that enables pluggable authentication mechanisms:

```go
type Provider interface {
    CanAccess(c *gin.Context) bool
}
```

This simple interface allows any authentication method to be integrated by implementing a single method that determines 
request access permissions.

### Core Components

- **Provider Interface**: Unified authentication contract
- **Concrete Providers**: Basic, JWT, Token, HMAC, Session authentication
- **Middleware Integration**: Seamless Gin framework integration
- **Context Storage**: Authentication data available throughout request lifecycle
- **Utility Functions**: Helper methods for extracting authentication information

## Authentication Providers

### 1. Basic Authentication

HTTP Basic Authentication with pluggable authentication backends.

#### Setup with Htpasswd Backend

```go
import (
    "github.com/oddbit-project/blueprint/provider/httpserver/auth"
    "github.com/oddbit-project/blueprint/provider/httpserver/auth/backend"
)

// Create htpasswd backend with user credentials
userMap := map[string]string{
    "admin": "$2a$10$...",  // bcrypt hashed password
    "user":  "$2a$10$...",  // bcrypt hashed password
}

htpasswdBackend, err := backend.NewHtpasswdBackendFromMap(userMap)
if err != nil {
    log.Fatal(err, "failed to create htpasswd backend")
}

// Create Basic Auth provider
authBasic, err := auth.NewBasicAuthProvider(htpasswdBackend)
if err != nil {
    log.Fatal(err, "failed to create basic auth provider")
}

// Apply globally
server.UseAuth(authBasic)

// Or apply to specific routes
protected := router.Group("/api")
protected.Use(auth.AuthMiddleware(authBasic))
```

#### Custom Realm Configuration

```go
// Create Basic Auth with custom realm
authBasic, err := auth.NewBasicAuthProvider(
    htpasswdBackend,
    auth.WithRealm("My Protected Area"),
)
```

#### Client Usage

**HTTP Request:**
```http
GET /api/resource HTTP/1.1
Host: example.com
Authorization: Basic YWRtaW46cGFzc3dvcmQ=
```

**curl Example:**
```bash
curl -u admin:password https://example.com/api/resource
```

#### Handler Access

```go
func protectedHandler(c *gin.Context) {
    // Get authenticated username
    username, exists := c.Get(gin.AuthUserKey)
    if !exists {
        c.JSON(401, gin.H{"error": "Not authenticated"})
        return
    }

    c.JSON(200, gin.H{
        "user": username,
        "message": "Access granted",
    })
}
```

#### Custom Authentication Backend

Implement the `Authenticator` interface for custom backends:

```go
import "github.com/oddbit-project/blueprint/provider/httpserver/auth/backend"

type DatabaseAuthBackend struct {
    db *sql.DB
}

func (d *DatabaseAuthBackend) ValidateUser(userName string, secret string) (bool, error) {
    var hashedPassword string
    err := d.db.QueryRow("SELECT password FROM users WHERE username = ?", userName).Scan(&hashedPassword)
    if err != nil {
        return false, err
    }

    // Verify password (e.g., using bcrypt)
    err = bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(secret))
    return err == nil, nil
}

// Use custom backend
dbBackend := &DatabaseAuthBackend{db: dbConnection}
authBasic, err := auth.NewBasicAuthProvider(dbBackend)
```

**Features:**
- Standards-compliant HTTP Basic Authentication (RFC 7617)
- Pluggable authentication backends via `Authenticator` interface
- Built-in htpasswd backend with bcrypt support
- Custom realm configuration for WWW-Authenticate challenges
- Authenticated username stored in gin.Context
- Comprehensive security logging

**Use Cases:**
- Legacy system integration
- Simple admin panels
- Internal tools and dashboards
- Development and testing environments
- Service-to-service communication with basic credentials

**Security Considerations:**
- **Always use HTTPS**: Basic Auth credentials are base64-encoded, not encrypted
- **Strong passwords**: Use bcrypt or similar for password hashing
- **Rate limiting**: Protect against brute-force attacks
- **Audit logging**: Monitor failed authentication attempts

### 2. Token Authentication

Simple API key authentication for basic access control.

#### Single Token Provider

> Note: NewAuthToken() relies on passing an unprotected token in the request header; this may pose a
> **serious security issue** when used in some production use cases


```go
import "github.com/oddbit-project/blueprint/provider/httpserver/auth"

// Create token provider
authToken := auth.NewAuthToken("X-API-Key", "your-secret-token")

// Apply globally
server.UseAuth(authToken)

// Or apply to specific routes
protected := router.Group("/api")
protected.Use(auth.AuthMiddleware(authToken))
```

**Configuration:**
- **Header Name**: Custom header name (default: `X-API-Key`)
- **Token Value**: Single valid token string
- **Behavior**: Returns `true` if header matches configured token

#### Multiple Token Provider

> Note: NewAuthTokenList() relies on passing an unprotected token in the request header; this may pose a 
> **serious security issue** when used in some production use cases


```go
// Support multiple valid tokens
validTokens := []string{"token1", "token2", "admin-token"}
authTokens := auth.NewAuthTokenList("X-API-Key", validTokens)

server.UseAuth(authTokens)
```

**Use Cases:**
- API key authentication
- Service-to-service communication
- Simple client authentication
- Development and testing environments

### 3. JWT Authentication

JSON Web Token authentication with comprehensive claim validation.

#### Setup

```go
import (
    "github.com/oddbit-project/blueprint/provider/httpserver/auth"
    "github.com/oddbit-project/blueprint/provider/jwtprovider"
)

// Configure JWT provider
jwtConfig := jwtprovider.NewConfig()
jwtConfig.SecretKey = "your-jwt-secret"
jwtConfig.Issuer = "your-app"

jwtProvider, err := jwtprovider.NewProvider(jwtConfig)
if err != nil {
    log.Fatal(err, "failed to create JWT provider")
}

// Create auth provider
authJWT := auth.NewAuthJWT(jwtProvider)
server.UseAuth(authJWT)
```

#### Token Usage

**Client Request:**
```http
Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
```

**Handler Access:**
```go
func protectedHandler(c *gin.Context) {
    // Get JWT token
    token, ok := auth.GetJWTToken(c)
    if !ok {
        c.JSON(401, gin.H{"error": "No token provided"})
        return
    }
    
    // Get parsed claims
    claims, ok := auth.GetJWTClaims(c)
    if !ok {
        c.JSON(401, gin.H{"error": "Invalid token"})
        return
    }
    
    userID := claims.Data["userId"]
    role := claims.Data["role"]
    
    c.JSON(200, gin.H{
        "user_id": userID,
        "role": role,
        "message": "Access granted",
    })
}
```

**Features:**
- Bearer token extraction from Authorization header
- Cryptographic signature validation
- Claims parsing and validation
- Context storage for parsed claims
- Expiration and issuer verification

### 4. HMAC Authentication

High-security authentication using HMAC-SHA256 signatures with replay protection.

#### Setup

```go
import (
    "github.com/oddbit-project/blueprint/provider/httpserver/auth"
    "github.com/oddbit-project/blueprint/provider/hmacprovider"
)

// Create credential configuration
credential := secure.DefaultCredentialConfig{
    Password: "your-hmac-secret",
}

// Configure HMAC provider
hmacProvider := hmacprovider.NewHmacProvider(credential)

// Create auth provider
authHMAC := auth.NewHMACAuthProvider(hmacProvider)
server.UseAuth(authHMAC)
```

#### Client Implementation

**Required Headers:**
```http
X-HMAC-Hash: sha256-calculated-signature
X-HMAC-Timestamp: 1640995200
X-HMAC-Nonce: unique-request-id
Content-Type: application/json
```

**Signature Calculation:**
```go
// Pseudo-code for client-side signature generation
timestamp := time.Now().Unix()
nonce := generateUniqueNonce()
message := httpMethod + "\n" + requestPath + "\n" + requestBody + "\n" + timestamp + "\n" + nonce
signature := hmacSHA256(secret, message)
```

#### Security Features

- **Cryptographic Integrity**: HMAC-SHA256 signature verification
- **Replay Protection**: Timestamp validation prevents old request reuse
- **Duplicate Protection**: Nonce validation prevents request duplication  
- **Body Integrity**: Full request body included in signature calculation
- **Comprehensive Logging**: Request authentication events with client details

#### Handler Access

```go
func hmacProtectedHandler(c *gin.Context) {
    // Check authentication flag
    authenticated, exists := c.Get("AuthFlag")
    if !exists || !authenticated.(bool) {
        c.JSON(401, gin.H{"error": "Authentication failed"})
        return
    }
    
    // Get authentication metadata
    timestamp, _ := c.Get("AuthTimestamp")
    nonce, _ := c.Get("AuthNonce")
    
    c.JSON(200, gin.H{
        "message": "HMAC authentication successful",
        "timestamp": timestamp,
        "nonce": nonce,
    })
}
```

### 5. Session Authentication

Cookie-based authentication integrated with the session management system.

#### Setup

```go
import (
    "encoding/gob"
    "github.com/oddbit-project/blueprint/provider/httpserver/auth"
    "github.com/oddbit-project/blueprint/provider/httpserver/session"
)

// Define user identity type
type UserIdentity struct {
    ID       int    `json:"id"`
    Username string `json:"username"`
    Email    string `json:"email"`
    Roles    []string `json:"roles"`
}

// Register for GOB serialization
func init() {
    gob.Register(&UserIdentity{})
}

// Setup session management
sessionConfig := session.NewConfig()
sessionManager, err := server.UseSession(sessionConfig, backend, logger)
if err != nil {
    log.Fatal(err, "failed to setup sessions")
}

// Create session auth provider
authSession := auth.NewAuthSession(&UserIdentity{})
server.UseAuth(authSession)
```

#### Authentication Flow

**Login Handler:**
```go
func loginHandler(c *gin.Context) {
    var loginReq struct {
        Username string `json:"username" binding:"required"`
        Password string `json:"password" binding:"required"`
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
    
    // Get session and set identity
    sess := session.Get(c)
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
```

**Protected Handler:**
```go
func protectedHandler(c *gin.Context) {
    // Get user identity from session
    identity, exists := auth.GetSessionIdentity(c)
    if !exists {
        c.JSON(401, gin.H{"error": "Not authenticated"})
        return
    }
    
    user, ok := identity.(*UserIdentity)
    if !ok {
        c.JSON(500, gin.H{"error": "Invalid identity type"})
        return
    }
    
    c.JSON(200, gin.H{
        "user": user,
        "message": "Access granted",
    })
}
```

**Logout Handler:**
```go
func logoutHandler(c *gin.Context) {
    // Clear session
    sessionManager := getSessionManager() // Your session manager instance
    sessionManager.Clear(c)
    
    c.JSON(200, gin.H{"message": "Logged out successfully"})
}
```

## Middleware Integration

### Global Authentication

```go
// Apply to all routes
authProvider := auth.NewAuthToken("X-API-Key", "secret")
server.UseAuth(authProvider)
```

### Route Group Authentication

```go
// Protected API routes
api := router.Group("/api")
api.Use(auth.AuthMiddleware(authJWT))
{
    api.GET("/user/profile", getProfileHandler)
    api.PUT("/user/profile", updateProfileHandler)
}

// Admin routes with different authentication
admin := router.Group("/admin") 
admin.Use(auth.AuthMiddleware(authToken))
{
    admin.GET("/users", listUsersHandler)
    admin.DELETE("/users/:id", deleteUserHandler)
}
```

### Multiple Authentication Methods

```go
func setupRoutes(router *gin.Engine) {
    // Public routes
    router.GET("/", homeHandler)
    router.POST("/login", loginHandler)

    // Session-based web routes
    web := router.Group("/dashboard")
    web.Use(auth.AuthMiddleware(authSession))
    {
        web.GET("/", dashboardHandler)
        web.POST("/logout", logoutHandler)
    }

    // JWT-based API routes
    api := router.Group("/api")
    api.Use(auth.AuthMiddleware(authJWT))
    {
        api.GET("/data", getDataHandler)
        api.POST("/data", createDataHandler)
    }

    // Basic Auth for admin panel
    admin := router.Group("/admin")
    admin.Use(auth.AuthMiddleware(authBasic))
    {
        admin.GET("/users", listUsersHandler)
        admin.GET("/settings", settingsHandler)
    }

    // HMAC-secured service routes
    service := router.Group("/service")
    service.Use(auth.AuthMiddleware(authHMAC))
    {
        service.POST("/sync", syncDataHandler)
        service.GET("/health", serviceHealthHandler)
    }
}
```

## Advanced Usage Patterns

### Custom Authentication Provider

```go
type CustomAuthProvider struct {
    validator func(*gin.Context) bool
}

func (c *CustomAuthProvider) CanAccess(ctx *gin.Context) bool {
    return c.validator(ctx)
}

func NewCustomAuth(validatorFunc func(*gin.Context) bool) *CustomAuthProvider {
    return &CustomAuthProvider{
        validator: validatorFunc,
    }
}

// Usage
customAuth := NewCustomAuth(func(c *gin.Context) bool {
    // Your custom authentication logic
    token := c.GetHeader("Custom-Auth")
    return validateCustomToken(token)
})

server.UseAuth(customAuth)
```

### Conditional Authentication

```go
func conditionalAuthMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        path := c.Request.URL.Path
        
        // Skip authentication for public endpoints
        if strings.HasPrefix(path, "/public/") {
            c.Next()
            return
        }
        
        // Use JWT for API endpoints
        if strings.HasPrefix(path, "/api/") {
            auth.AuthMiddleware(authJWT)(c)
            return
        }
        
        // Use session auth for web endpoints
        auth.AuthMiddleware(authSession)(c)
    }
}
```

### Role-Based Authorization

```go
func requireRole(role string) gin.HandlerFunc {
    return func(c *gin.Context) {
        identity, exists := auth.GetSessionIdentity(c)
        if !exists {
            c.AbortWithStatusJSON(401, gin.H{"error": "Not authenticated"})
            return
        }
        
        user, ok := identity.(*UserIdentity)
        if !ok {
            c.AbortWithStatusJSON(500, gin.H{"error": "Invalid identity"})
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

// Usage
admin := router.Group("/admin")
admin.Use(auth.AuthMiddleware(authSession))
admin.Use(requireRole("admin"))
{
    admin.GET("/users", listUsersHandler)
    admin.DELETE("/users/:id", deleteUserHandler)
}
```

## Configuration

### JWT Configuration

```go
type JWTConfig struct {
    SecretKey    string `json:"secretKey"`
    Issuer       string `json:"issuer"`
    Audience     string `json:"audience"`
    ExpirationHours int `json:"expirationHours"`
    Algorithm    string `json:"algorithm"`
}
```

### Session Configuration

```go
type SessionConfig struct {
    CookieName             string `json:"cookieName"`             // Cookie name
    ExpirationSeconds      int    `json:"expirationSeconds"`      // Max lifetime
    IdleTimeoutSeconds     int    `json:"idleTimeoutSeconds"`     // Idle timeout
    Secure                 bool   `json:"secure"`                 // HTTPS only
    HttpOnly               bool   `json:"httpOnly"`               // No JS access
    SameSite               int    `json:"sameSite"`               // CSRF protection
    Domain                 string `json:"domain"`                 // Cookie domain
    Path                   string `json:"path"`                   // Cookie path
    EncryptionKey          secure.DefaultCredentialConfig `json:"encryptionKey"` // Encryption
    CleanupIntervalSeconds int    `json:"cleanupIntervalSeconds"` // Cleanup frequency
}
```

### HMAC Configuration

```go
type HMACConfig struct {
    Secret           string `json:"secret"`
    TimestampWindow  int    `json:"timestampWindow"`  // Seconds
    NonceExpiration  int    `json:"nonceExpiration"`  // Seconds
    IncludeBody      bool   `json:"includeBody"`      // Include body in signature
}
```

## Security Considerations

### Production Security Checklist

1. **Use HTTPS**: All authentication should occur over encrypted connections
   ```go
   sessionConfig.Secure = true
   ```

2. **Strong Secrets**: Use cryptographically secure random secrets
   ```bash
   # Generate secure JWT secret
   openssl rand -base64 32
   ```

3. **Token Expiration**: Configure appropriate token lifetimes
   ```go
   jwtConfig.ExpirationHours = 1  // Short-lived tokens
   sessionConfig.ExpirationSeconds = 3600  // 1 hour sessions
   ```

4. **Session Security**: Enable all cookie security features
   ```go
   sessionConfig.HttpOnly = true
   sessionConfig.SameSite = int(http.SameSiteStrictMode)
   ```

5. **Rate Limiting**: Combine with rate limiting for auth endpoints
   ```go
   authGroup := router.Group("/auth")
   authGroup.Use(security.RateLimitMiddleware(rate.Every(time.Minute/5), 2))
   ```

### Common Security Patterns

**Token Refresh Pattern:**
```go
func refreshTokenHandler(c *gin.Context) {
    // Validate existing token
    claims, ok := auth.GetJWTClaims(c)
    if !ok {
        c.JSON(401, gin.H{"error": "Invalid token"})
        return
    }
    
    // Generate new token
    newToken, err := jwtProvider.Generate(claims.Data)
    if err != nil {
        c.JSON(500, gin.H{"error": "Token generation failed"})
        return
    }
    
    c.JSON(200, gin.H{"token": newToken})
}
```

**Session Regeneration on Privilege Change:**
```go
func elevatePrivilegesHandler(c *gin.Context) {
    sess := session.Get(c)
    identity, _ := auth.GetSessionIdentity(c)
    user := identity.(*UserIdentity)
    
    // Update privileges
    user.Roles = append(user.Roles, "admin")
    sess.SetIdentity(user)
    
    // Regenerate session for security
    sessionManager.Regenerate(c)
    
    c.JSON(200, gin.H{"message": "Privileges elevated"})
}
```
