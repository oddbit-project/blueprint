# HTTP Authentication

Blueprint provides a flexible authentication system for HTTP applications with support for multiple authentication methods:

1. **Token-based Authentication**: Simple API key authentication using custom headers
2. **JWT-based Authentication**: Stateless JWT token authentication with full claim validation
3. **HMAC-based Authentication**: Cryptographic authentication using HMAC signatures with timestamp and nonce protection
4. **Custom Authentication**: Extensible provider interface for custom authentication methods

## Architecture Overview

The authentication system consists of a unified interface with multiple implementations:

- **Provider Interface**: Common authentication contract (`provider/httpserver/auth/auth.go`)
- **AuthMiddleware**: Gin middleware for applying authentication to routes
- **Token Provider**: API key authentication (`provider/httpserver/auth/token.go`)
- **JWT Provider**: JWT token authentication (`provider/httpserver/auth/jwt.go`)
- **HMAC Provider**: HMAC signature authentication (`provider/httpserver/auth/hmac.go`)

> **Integration Notes**: 
> - JWT authentication integrates with the `provider/jwtprovider` package for full JWT functionality including asymmetric algorithms, token revocation, and JWKS support.
> - HMAC authentication integrates with the `provider/hmacprovider` package for cryptographic signature verification with replay attack protection.

## Features

### Core Authentication Features
- **Unified Interface**: Single provider interface for all authentication methods
- **Gin Middleware**: Easy integration with Gin router middleware
- **Flexible Headers**: Configurable header names for token authentication
- **JWT Integration**: Full JWT validation with claims context injection
- **HMAC Authentication**: Cryptographic signature-based authentication
- **Extensible Design**: Easy to add custom authentication providers

### Security Features
- **Bearer Token Support**: Standard Authorization header parsing
- **Header Validation**: Strict token format validation
- **Claims Context**: JWT claims available in request context
- **HMAC Signatures**: Request body integrity verification
- **Replay Protection**: Timestamp and nonce validation for HMAC
- **DoS Protection**: Input size limits and rate limiting
- **401 Response**: Automatic unauthorized response handling

## Authentication Providers

### Provider Interface

All authentication providers implement a simple interface:

```go
type Provider interface {
    CanAccess(c *gin.Context) bool
}
```

### Token-based Authentication

Simple API key authentication using custom headers:

```go
// Create token provider with default header
provider := auth.NewAuthToken(auth.DefaultTokenAuthHeader, "your-secret-api-key")

// Create token provider with custom header
provider := auth.NewAuthToken("X-Custom-Auth", "your-secret-api-key")

// Apply to routes
router.Use(auth.AuthMiddleware(provider))
```

**Configuration Options:**
- `headerName`: HTTP header name (default: "X-API-Key")
- `key`: The expected API key value
- Empty key allows all requests (useful for development)

### JWT-based Authentication

Stateless JWT token authentication with full validation:

```go
// Configure JWT provider (requires jwtprovider.JWTParser)
jwtConfig := jwt.NewJWTConfig()
jwtConfig.SigningAlgorithm = "HS256"
jwtConfig.CfgSigningKey = &secure.DefaultCredentialConfig{Password: "your-jwt-secret"}

jwtProvider, err := jwt.NewProvider(jwtConfig)
if err != nil {
    log.Fatal(err)
}

// Create JWT auth provider
authProvider := auth.NewAuthJWT(jwtProvider)

// Apply to routes
router.Use(auth.AuthMiddleware(authProvider))
```

**JWT Features:**
- **Bearer Token Format**: Expects `Authorization: Bearer <token>`
- **Full Validation**: Signature, expiration, and claims validation
- **Claims Context**: JWT claims available via `auth.ContextJwtClaims`
- **Integration**: Uses `provider/jwtprovider` for all JWT operations

### HMAC-based Authentication

High-security authentication using HMAC-SHA256 signatures with timestamp and nonce protection:

```go
import (
    "github.com/oddbit-project/blueprint/provider/hmacprovider"
    "github.com/oddbit-project/blueprint/crypt/secure"
)

// Create HMAC provider
secretConfig := &secure.DefaultCredentialConfig{Password: "your-hmac-secret"}
key, err := secure.GenerateKey()
if err != nil {
    log.Fatal(err)
}

credential, err := secure.CredentialFromConfig(secretConfig, key, false)
if err != nil {
    log.Fatal(err)
}

hmacProvider := hmacprovider.NewHmacProvider(credential)

// Create HMAC auth provider
authProvider := auth.HMACAuth(hmacProvider)

// Apply to routes
router.Use(auth.AuthMiddleware(authProvider))
```

**HMAC Features:**
- **Signature Verification**: HMAC-SHA256 signature validation of request body
- **Timestamp Protection**: Configurable time window validation (default: 5 minutes)
- **Replay Protection**: Nonce-based replay attack prevention
- **Request Integrity**: Complete request body verification
- **Storage Options**: Memory or Redis-based nonce storage
- **DoS Protection**: Maximum input size limits (default: 32MB)

**Required Headers:**
- `X-HMAC-Hash`: HMAC-SHA256 signature of `timestamp:nonce:body`
- `X-HMAC-Timestamp`: RFC3339 timestamp string
- `X-HMAC-Nonce`: UUID v4 nonce for replay protection

**HMAC Configuration Options:**
```go
// Custom nonce store (Redis)
// Redis nonce store setup
redisStore := store.NewRedisNonceStore(redisClient)

// Create credential from config
secretConfig := &secure.DefaultCredentialConfig{PasswordEnvVar: "HMAC_SECRET"}
key, _ := secure.GenerateKey()
credential, _ := secure.CredentialFromConfig(secretConfig, key, false)

hmacProvider := hmacprovider.NewHmacProvider(
    credential,
    hmacprovider.WithNonceStore(redisStore),
    hmacprovider.WithKeyInterval(10*time.Minute), // Allow 10-minute time drift
    hmacprovider.WithMaxInputSize(64*1024*1024),  // 64MB max input
)
```

## Using Authentication

### Basic Setup

```go
package main

import (
    "github.com/gin-gonic/gin"
    "github.com/oddbit-project/blueprint/provider/httpserver/auth"
)

func main() {
    router := gin.Default()
    
    // Option 1: Token authentication
    tokenAuth := auth.NewAuthToken("X-API-Key", "secret-key-123")
    
    // Option 2: JWT authentication
    jwtProvider := setupJWTProvider() // See JWT setup below
    jwtAuth := auth.NewAuthJWT(jwtProvider)
    
    // Option 3: HMAC authentication
    hmacProvider := setupHMACProvider() // See HMAC setup below
    hmacAuth := auth.HMACAuth(hmacProvider)
    
    // Apply authentication to specific routes
    protected := router.Group("/api")
    protected.Use(auth.AuthMiddleware(tokenAuth))
    {
        protected.GET("/users", getUsersHandler)
        protected.POST("/users", createUserHandler)
    }
    
    // Public routes
    router.GET("/health", healthHandler)
    
    router.Run(":8080")
}
```

### JWT Setup with Provider

```go
import (
    "github.com/oddbit-project/blueprint/provider/jwtprovider"
    "github.com/oddbit-project/blueprint/crypt/secure"
)

func setupJWTProvider() jwtprovider.JWTParser {
    // Configure JWT
    config := jwtprovider.NewJWTConfig()
    config.SigningAlgorithm = jwtprovider.HS256
    config.CfgSigningKey = &secure.DefaultCredentialConfig{
        Password: "your-jwt-secret-key",
    }
    config.Issuer = "your-app"
    config.Audience = "your-api"
    config.ExpirationSeconds = 3600 // 1 hour
    
    // Create provider
    provider, err := jwtprovider.NewProvider(config)
    if err != nil {
        panic(err)
    }
    
    return provider
}

func setupHMACProvider() *hmacprovider.HMACProvider {
    // Configure HMAC
    secretConfig := &secure.DefaultCredentialConfig{
        Password: "your-hmac-secret-key",
    }
    
    key, err := secure.GenerateKey()
    if err != nil {
        panic(err)
    }
    
    credential, err := secure.CredentialFromConfig(secretConfig, key, false)
    if err != nil {
        panic(err)
    }
    
    // Create provider with custom options
    provider := hmacprovider.NewHmacProvider(
        credential,
        hmacprovider.WithKeyInterval(10*time.Minute), // 10-minute time drift
        hmacprovider.WithMaxInputSize(64*1024*1024),  // 64MB max
    )
    
    return provider
}
```

### Accessing Authentication Context

#### Token Authentication

Token authentication validates the header but doesn't inject additional context:

```go
func protectedHandler(c *gin.Context) {
    // Request has been authenticated by middleware
    // Original token available in header
    token := c.GetHeader("X-API-Key")
    
    c.JSON(200, gin.H{"message": "Access granted"})
}
```

#### JWT Authentication

JWT authentication injects validated claims into the context:

```go
import "github.com/oddbit-project/blueprint/provider/jwtprovider"

func protectedHandler(c *gin.Context) {
    // Get JWT claims from context
    claimsValue, exists := c.Get(auth.ContextJwtClaims)
    if !exists {
        c.JSON(500, gin.H{"error": "claims not found"})
        return
    }
    
    claims, ok := claimsValue.(*jwtprovider.Claims)
    if !ok {
        c.JSON(500, gin.H{"error": "invalid claims type"})
        return
    }
    
    // Access JWT claims
    userID := claims.Subject
    tokenID := claims.ID
    issuer := claims.Issuer
    customData := claims.Data
    
    c.JSON(200, gin.H{
        "message": "Access granted",
        "user_id": userID,
        "token_id": tokenID,
        "custom_data": customData,
    })
}
```

#### HMAC Authentication

HMAC authentication injects authentication metadata into the context:

```go
func protectedHandler(c *gin.Context) {
    // Check if request was authenticated via HMAC
    authenticated, exists := c.Get(auth.AuthFlag)
    if !exists || !authenticated.(bool) {
        c.JSON(500, gin.H{"error": "authentication info not found"})
        return
    }
    
    // Get authentication metadata
    timestamp, _ := c.Get(auth.AuthTimestamp)
    nonce, _ := c.Get(auth.AuthNonce)
    
    c.JSON(200, gin.H{
        "message": "Access granted via HMAC",
        "timestamp": timestamp,
        "nonce": nonce,
        "client_ip": c.ClientIP(),
    })
}
```

### Route-specific Authentication

```go
func setupRoutes(router *gin.Engine) {
    // Public routes
    router.GET("/health", healthHandler)
    router.POST("/login", loginHandler)
    
    // API key protected routes
    apiRoutes := router.Group("/api")
    apiRoutes.Use(auth.AuthMiddleware(tokenAuth))
    {
        apiRoutes.GET("/status", statusHandler)
    }
    
    // JWT protected routes
    userRoutes := router.Group("/users")
    userRoutes.Use(auth.AuthMiddleware(jwtAuth))
    {
        userRoutes.GET("/profile", getProfileHandler)
        userRoutes.PUT("/profile", updateProfileHandler)
    }
    
    // Admin routes with stricter authentication
    adminRoutes := router.Group("/admin")
    adminRoutes.Use(auth.AuthMiddleware(adminJWTAuth))
    {
        adminRoutes.GET("/users", listUsersHandler)
        adminRoutes.DELETE("/users/:id", deleteUserHandler)
    }
    
    // High-security routes with HMAC authentication
    secureRoutes := router.Group("/secure")
    secureRoutes.Use(auth.AuthMiddleware(hmacAuth))
    {
        secureRoutes.POST("/webhook", webhookHandler)
        secureRoutes.PUT("/config", updateConfigHandler)
    }
}
```

## Custom Authentication Providers

You can create custom authentication providers by implementing the `Provider` interface:

```go
type customAuth struct {
    // Your custom fields
    database *sql.DB
    cache    *redis.Client
}

func NewCustomAuth(db *sql.DB, cache *redis.Client) auth.Provider {
    return &customAuth{
        database: db,
        cache:    cache,
    }
}

func (a *customAuth) CanAccess(c *gin.Context) bool {
    // Custom authentication logic
    sessionID := c.GetHeader("X-Session-ID")
    if sessionID == "" {
        return false
    }
    
    // Validate session in database/cache
    valid, err := a.validateSession(sessionID)
    if err != nil || !valid {
        return false
    }
    
    // Optionally inject user data into context
    user, _ := a.getUserFromSession(sessionID)
    c.Set("user", user)
    
    return true
}

func (a *customAuth) validateSession(sessionID string) (bool, error) {
    // Your validation logic here
    return true, nil
}

func (a *customAuth) getUserFromSession(sessionID string) (*User, error) {
    // Your user lookup logic here
    return &User{}, nil
}
```

## Error Handling

### Authentication Failures

The middleware automatically handles authentication failures:

```go
func (a *authJWT) CanAccess(c *gin.Context) bool {
    authHeader := c.GetHeader("Authorization")
    if len(authHeader) < 7 || authHeader[:7] != "Bearer " {
        c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
            "error": "missing or invalid Authorization header"
        })
        return false
    }
    
    claims, err := a.parser.ParseToken(authHeader[7:])
    if err != nil || len(claims.ID) == 0 {
        c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
            "error": "invalid token"
        })
        return false
    }
    
    // Success path...
    return true
}
```

### Custom Error Responses

You can customize error responses in your own providers:

```go
func (a *customAuth) CanAccess(c *gin.Context) bool {
    token := c.GetHeader("X-API-Key")
    
    if token == "" {
        c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
            "error": "API key required",
            "code": "MISSING_API_KEY",
        })
        return false
    }
    
    if !a.validateToken(token) {
        c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
            "error": "Invalid API key",
            "code": "INVALID_API_KEY",
        })
        return false
    }
    
    return true
}
```

## Configuration Examples

### Environment-based Configuration

```go
import (
    "os"
    "github.com/oddbit-project/blueprint/crypt/secure"
)

func createAuthProvider() auth.Provider {
    authType := os.Getenv("AUTH_TYPE")
    
    switch authType {
    case "jwt":
        return createJWTAuth()
    case "token":
        return createTokenAuth()
    case "hmac":
        return createHMACAuth()
    default:
        panic("Invalid AUTH_TYPE")
    }
}

func createJWTAuth() auth.Provider {
    config := jwtprovider.NewJWTConfig()
    config.SigningAlgorithm = os.Getenv("JWT_ALGORITHM")
    config.CfgSigningKey = &secure.DefaultCredentialConfig{
        PasswordEnvVar: "JWT_SECRET",
    }
    config.Issuer = os.Getenv("JWT_ISSUER")
    config.Audience = os.Getenv("JWT_AUDIENCE")
    
    provider, err := jwtprovider.NewProvider(config)
    if err != nil {
        panic(err)
    }
    
    return auth.NewAuthJWT(provider)
}

func createTokenAuth() auth.Provider {
    headerName := os.Getenv("TOKEN_HEADER")
    if headerName == "" {
        headerName = auth.DefaultTokenAuthHeader
    }
    
    apiKey := os.Getenv("API_KEY")
    return auth.NewAuthToken(headerName, apiKey)
}

func createHMACAuth() auth.Provider {
    secretConfig := &secure.DefaultCredentialConfig{
        PasswordEnvVar: "HMAC_SECRET",
    }
    
    key, err := secure.GenerateKey()
    if err != nil {
        panic(err)
    }
    
    credential, err := secure.CredentialFromConfig(secretConfig, key, false)
    if err != nil {
        panic(err)
    }
    
    // Create HMAC provider with environment-based configuration
    opts := []hmacprovider.HMACProviderOption{}
    
    // Optional: Configure with environment variables
    if intervalStr := os.Getenv("HMAC_KEY_INTERVAL"); intervalStr != "" {
        if interval, err := time.ParseDuration(intervalStr); err == nil {
            opts = append(opts, hmacprovider.WithKeyInterval(interval))
        }
    }
    
    provider := hmacprovider.NewHmacProvider(credential, opts...)
    return auth.HMACAuth(provider)
}
```

### Multiple Authentication Methods

```go
func setupMultipleAuth(router *gin.Engine) {
    // Create different auth providers
    apiKeyAuth := auth.NewAuthToken("X-API-Key", "api-secret")
    jwtAuth := auth.NewAuthJWT(jwtProvider)
    hmacAuth := auth.HMACAuth(hmacProvider)
    adminAuth := auth.NewAuthToken("X-Admin-Key", "admin-secret")
    
    // Public endpoints
    router.GET("/health", healthHandler)
    
    // API key authentication for basic API
    api := router.Group("/api/v1")
    api.Use(auth.AuthMiddleware(apiKeyAuth))
    {
        api.GET("/data", getDataHandler)
    }
    
    // JWT authentication for user operations
    user := router.Group("/user")
    user.Use(auth.AuthMiddleware(jwtAuth))
    {
        user.GET("/profile", getUserProfileHandler)
        user.PUT("/profile", updateUserProfileHandler)
    }
    
    // HMAC authentication for high-security operations
    secure := router.Group("/secure")
    secure.Use(auth.AuthMiddleware(hmacAuth))
    {
        secure.POST("/webhook", webhookHandler)
        secure.PUT("/sensitive-data", updateSensitiveDataHandler)
    }
    
    // Admin authentication for admin operations
    admin := router.Group("/admin")
    admin.Use(auth.AuthMiddleware(adminAuth))
    {
        admin.GET("/users", listAllUsersHandler)
        admin.DELETE("/users/:id", deleteUserHandler)
    }
}
```

## Best Practices

### Security Recommendations

1. **Token Management**
   - Use strong, randomly generated API keys
   - Rotate API keys regularly
   - Store keys securely (environment variables, secret management)
   - Use HTTPS in production to protect tokens in transit

2. **JWT Security**
   - Use asymmetric algorithms (RS256, ES256, EdDSA) for production
   - Set appropriate expiration times (15-60 minutes)
   - Implement token revocation for sensitive applications
   - Validate issuer and audience claims

3. **Header Security**
   - Use standard headers when possible (`Authorization: Bearer`)
   - Avoid exposing tokens in URLs or logs
   - Implement rate limiting on authentication endpoints
   - Log authentication failures for monitoring

4. **HMAC Security**
   - Use cryptographically strong secret keys (minimum 32 bytes)
   - Implement proper time drift tolerance (5-10 minutes maximum)
   - Use Redis or persistent storage for nonce store in production
   - Monitor and alert on HMAC verification failures
   - Implement request size limits to prevent DoS attacks

### Development Practices

1. **Testing**
   - Mock authentication providers for unit tests
   - Test both successful and failed authentication scenarios
   - Verify context injection for JWT authentication
   - Test middleware integration

2. **Configuration**
   - Use environment variables for secrets
   - Implement configuration validation
   - Provide sensible defaults for development
   - Document required environment variables

3. **Error Handling**
   - Return consistent error responses
   - Don't expose sensitive information in error messages
   - Log authentication failures appropriately
   - Implement proper HTTP status codes

### Performance Considerations

1. **Caching**
   - Cache validated tokens when appropriate
   - Use Redis for session-based custom authentication
   - Implement token validation caching for JWT
   - Use Redis nonce store for HMAC in distributed systems

2. **Database Queries**
   - Optimize user lookup queries in custom providers
   - Use database connection pooling
   - Consider read replicas for authentication queries

## Integration with Other Components

### Session Integration

Combine authentication with session management:

```go
// Setup both authentication and sessions
authProvider := auth.NewAuthJWT(jwtProvider)
sessionManager := session.NewManager(store, sessionConfig, logger)

// Apply both middlewares
protected := router.Group("/app")
protected.Use(auth.AuthMiddleware(authProvider))
protected.Use(sessionManager.Middleware())
{
    protected.GET("/dashboard", dashboardHandler)
}

func dashboardHandler(c *gin.Context) {
    // Access JWT claims
    claims, _ := c.Get(auth.ContextJwtClaims)
    
    // Access session data
    sess := session.Get(c)
    sess.Set("last_access", time.Now())
    
    // Use both authentication and session data
    c.JSON(200, gin.H{"user": claims, "session": sess.Values})
}
```

### CSRF Protection

Combine with CSRF protection for web applications:

```go
// Setup authentication, sessions, and CSRF
router.Use(sessionManager.Middleware())
router.Use(server.UseCSRFProtection())

// Authentication not needed for CSRF-protected forms
// (session-based CSRF handles authentication)
```

## Migration Guide

### From Manual Header Checking

**Before:**
```go
func protectedHandler(c *gin.Context) {
    apiKey := c.GetHeader("X-API-Key")
    if apiKey != "expected-key" {
        c.JSON(401, gin.H{"error": "Unauthorized"})
        return
    }
    
    // Handle request...
}
```

**After:**
```go
// Setup authentication middleware once
auth := auth.NewAuthToken("X-API-Key", "expected-key")
router.Use(auth.AuthMiddleware(auth))

func protectedHandler(c *gin.Context) {
    // Request is already authenticated
    // Handle request...
}
```

### From Custom JWT Parsing

**Before:**
```go
func jwtHandler(c *gin.Context) {
    authHeader := c.GetHeader("Authorization")
    if !strings.HasPrefix(authHeader, "Bearer ") {
        c.JSON(401, gin.H{"error": "Invalid auth header"})
        return
    }
    
    token := authHeader[7:]
    claims, err := parseJWT(token) // Custom parsing
    if err != nil {
        c.JSON(401, gin.H{"error": "Invalid token"})
        return
    }
    
    // Use claims...
}
```

**After:**
```go
// Setup JWT authentication middleware once
jwtAuth := auth.NewAuthJWT(jwtProvider)
router.Use(auth.AuthMiddleware(jwtAuth))

func jwtHandler(c *gin.Context) {
    // Get validated claims from context
    claims, _ := c.Get(auth.ContextJwtClaims)
    
    // Use claims...
}
```

## Constants and Error Messages

```go
const (
    DefaultTokenAuthHeader = "X-API-Key"           // Default header for token auth
    ErrMissingAuthHeader   = "missing or invalid Authorization header"  // JWT auth error
    ContextJwtClaims       = "jwtClaims"          // Context key for JWT claims
    
    // HMAC Authentication Headers
    HeaderHMACHash         = "X-HMAC-Hash"        // HMAC signature header
    HeaderHMACTimestamp    = "X-HMAC-Timestamp"   // HMAC timestamp header
    HeaderHMACNonce        = "X-HMAC-Nonce"       // HMAC nonce header
    
    // HMAC Context Keys
    AuthFlag               = "Authenticated"       // Authentication status flag
    AuthTimestamp          = "AuthTimestamp"      // Authentication timestamp
    AuthNonce              = "AuthNonce"          // Authentication nonce
)
```

## Client-Side HMAC Implementation

When using HMAC authentication, clients must generate the proper headers using the same hmacprovider. Here's an example implementation:

```go
package main

import (
    "bytes"
    "fmt"
    "net/http"
    
    "github.com/oddbit-project/blueprint/provider/hmacprovider"
    "github.com/oddbit-project/blueprint/crypt/secure"
)

func makeHMACRequest(url, method string, body []byte, secretKey string) error {
    // Create HMAC provider (same configuration as server)
    key, err := secure.GenerateKey()
    if err != nil {
        return fmt.Errorf("failed to generate key: %w", err)
    }
    
    credential, err := secure.NewCredential([]byte(secretKey), key, false)
    if err != nil {
        return fmt.Errorf("failed to create credential: %w", err)
    }
    
    provider := hmacprovider.NewHmacProvider(credential)
    
    // Generate HMAC signature with timestamp and nonce
    bodyReader := bytes.NewReader(body)
    hash, timestamp, nonce, err := provider.Sign256(bodyReader)
    if err != nil {
        return fmt.Errorf("failed to generate HMAC: %w", err)
    }
    
    // Create request
    req, err := http.NewRequest(method, url, bytes.NewReader(body))
    if err != nil {
        return err
    }
    
    // Set HMAC headers
    req.Header.Set("X-HMAC-Hash", hash)
    req.Header.Set("X-HMAC-Timestamp", timestamp)
    req.Header.Set("X-HMAC-Nonce", nonce)
    req.Header.Set("Content-Type", "application/json")
    
    // Send request
    client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    
    fmt.Printf("Response Status: %s\n", resp.Status)
    return nil
}

// Example usage
func main() {
    body := []byte(`{"message": "Hello, World!"}`)
    secretKey := "your-shared-secret-key"
    
    err := makeHMACRequest("https://api.example.com/webhook", "POST", body, secretKey)
    if err != nil {
        fmt.Printf("Error: %v\n", err)
    }
}
```

The authentication system provides a clean, unified interface for multiple authentication methods while maintaining flexibility and security. Choose the appropriate provider based on your application's requirements and integrate it seamlessly with Blueprint's other HTTP components.