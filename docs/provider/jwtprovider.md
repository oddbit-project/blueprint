# blueprint.provider.jwtprovider

Blueprint JWT provider for comprehensive JSON Web Token authentication and management.

## Overview

The JWT provider offers a complete solution for managing JWT tokens in Go applications. It supports multiple signing algorithms, token revocation, refresh capabilities, and secure key management.

Key features:

- Multiple signing algorithms (HMAC, RSA, ECDSA, EdDSA)
- Token revocation system with pluggable backends
- **User token tracking and session management**
- **Configurable concurrent session limits**
- Token refresh functionality
- Secure key management with Blueprint's credential system
- Comprehensive validation options
- Thread-safe operations


## Supported Signing Algorithms

| Algorithm | Type      | Security | Key Requirements    | Recommended |
|-----------|-----------|----------|---------------------|-------------|
| **HS256** | HMAC      | High     | Shared secret       | **Yes**     |
| **HS384** | HMAC      | High     | Shared secret       | **Yes**     |
| **HS512** | HMAC      | High     | Shared secret       | **Yes**     |
| **RS256** | RSA       | High     | RSA key pair        | **Yes**     |
| **RS384** | RSA       | High     | RSA key pair        | **Yes**     |
| **RS512** | RSA       | High     | RSA key pair        | **Yes**     |
| **ES256** | ECDSA     | High     | ECDSA key pair      | **Yes**     |
| **ES384** | ECDSA     | High     | ECDSA key pair      | **Yes**     |
| **ES512** | ECDSA     | High     | ECDSA key pair      | **Yes**     |
| **EdDSA** | EdDSA     | High     | Ed25519 key pair    | **Yes**     |

## Configuration

### Basic Configuration

```go
type JWTConfig struct {
	CfgSigningKey     *secure.DefaultCredentialConfig `json:"signingKey,omitempty"`     // For HMAC algorithms
	CfgPrivateKey     *secure.KeyConfig               `json:"privateKey,omitempty"`     // For asymmetric algorithms
	CfgPublicKey      *secure.KeyConfig               `json:"publicKey,omitempty"`      // For asymmetric algorithms
	SigningAlgorithm  string                          `json:"signingAlgorithm"`         // Algorithm to use
	ExpirationSeconds int                             `json:"expirationSeconds"`        // Token expiration
	Issuer            string                          `json:"issuer"`                   // Token issuer
	Audience          string                          `json:"audience"`                 // Token audience
	KeyID             string                          `json:"keyID"`                    // Key ID for JWKS
	MaxTokenSize      int                             `json:"maxTokenSize,omitempty"`   // Maximum token size (bytes)
	RequireIssuer     bool                            `json:"requireIssuer"`            // Enforce issuer validation
	RequireAudience   bool                            `json:"requireAudience"`          // Enforce audience validation
    
    // User Token Tracking
	TrackUserTokens   bool                            `json:"trackUserTokens"`          // Enable user token tracking
	MaxUserSessions   int                             `json:"maxUserSessions,omitempty"` // Max concurrent sessions per user (0 = unlimited)
}
```

### Default Values

```go
const (
	DefaultTTL      = time.Second * 86400 // 1 day
	DefaultIssuer   = "blueprint"
	DefaultAudience = "api"
)
```

## Basic Usage

### HMAC Algorithms (HS256/HS384/HS512)

```go
package main

import (
    "fmt"
    "log"
    "github.com/oddbit-project/blueprint/provider/jwtprovider"
    "github.com/oddbit-project/blueprint/crypt/secure"
)

func main() {
    // Create configuration for HMAC signing
	config := jwtprovider.NewJWTConfig()
	config.SigningAlgorithm = jwtprovider.HS256
	config.ExpirationSeconds = 3600 // 1 hour
	config.Issuer = "my-app"
	config.Audience = "api"
	config.RequireIssuer = true
	config.RequireAudience = true

    // Set up signing key
	signingKey, err := secure.GenerateKey()
	if err != nil {
        log.Fatal(err)
    }
    
	config.CfgSigningKey = &secure.DefaultCredentialConfig{
        Password: string(signingKey),
    }

    // Create JWT provider
	provider, err := jwtprovider.NewProvider(config)
	if err != nil {
        log.Fatal(err)
    }

    // Generate a token
	customData := map[string]any{
        "role":        "admin",
        "permissions": []string{"read", "write"},
    }
    
	token, err := provider.GenerateToken("user123", customData)
	if err != nil {
        log.Fatal(err)
    }
    
	fmt.Printf("Generated token: %s\n", token)

    // Parse and validate the token
	claims, err := provider.ParseToken(token)
	if err != nil {
        log.Fatal(err)
    }
    
	fmt.Printf("Subject: %s\n", claims.Subject)
	fmt.Printf("Custom data: %v\n", claims.Data)
}
```

### RSA Algorithms (RS256/RS384/RS512)

```go
package main

import (
    "fmt"
    "log"
    "github.com/oddbit-project/blueprint/provider/jwtprovider"
    "github.com/oddbit-project/blueprint/crypt/secure"
)

func main() {
    // Create configuration for RSA signing
	config := jwtprovider.NewJWTConfig()
	config.SigningAlgorithm = jwtprovider.RS256
	config.ExpirationSeconds = 3600
	config.Issuer = "my-app"
	config.Audience = "api"
	config.KeyID = "key-1" // For JWKS support

    // Set up RSA key pair (PEM encoded PKCS#8)
	privateKeyPEM := `-----BEGIN PRIVATE KEY-----
MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQC7...
-----END PRIVATE KEY-----`

	publicKeyPEM := `-----BEGIN PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAu8...
-----END PUBLIC KEY-----`

    // Configure private key
	privateKeyConfig, err := secure.NewKeyConfig([]byte(privateKeyPEM))
	if err != nil {
        log.Fatal(err)
    }
	config.CfgPrivateKey = privateKeyConfig

    // Configure public key  
	publicKeyConfig, err := secure.NewKeyConfig([]byte(publicKeyPEM))
	if err != nil {
        log.Fatal(err)
    }
	config.CfgPublicKey = publicKeyConfig

    // Create provider
	provider, err := jwtprovider.NewProvider(config)
	if err != nil {
        log.Fatal(err)
    }

    // Generate and parse tokens same as HMAC example...
}
```

## Token Revocation

### Setting Up Revocation

```go
// Create in-memory revocation backend
revocationBackend := jwtprovider.NewMemoryRevocationBackend()

// Create revocation manager
revocationManager := jwtprovider.NewRevocationManager(revocationBackend)

// Create provider with revocation support
provider, err := jwtprovider.NewProvider(config, 
	jwtprovider.WithRevocationManager(revocationManager))
if err != nil {
	log.Fatal(err)
}
```

### Revoking Tokens

```go
// Revoke a specific token
err := provider.RevokeToken(tokenString)
if err != nil {
	log.Fatal(err)
}

// Revoke by token ID (from claims)
claims, err := provider.ParseToken(tokenString)
if err != nil {
	log.Fatal(err)
}

err = provider.RevokeTokenByID(claims.ID, claims.ExpiresAt.Time)
if err != nil {
	log.Fatal(err)
}

// Check if token is revoked
isRevoked := provider.IsTokenRevoked(claims.ID)
fmt.Printf("Token revoked: %v\n", isRevoked)
```

### Custom Revocation Backend

```go
// Implement custom revocation backend
type DatabaseRevocationBackend struct {
	db *sql.DB
}

func (d *DatabaseRevocationBackend) RevokeToken(tokenID string, expiresAt time.Time) error {
	_, err := d.db.Exec("INSERT INTO revoked_tokens (token_id, expires_at, revoked_at) VALUES (?, ?, ?)",
        tokenID, expiresAt, time.Now())
	return err
}

func (d *DatabaseRevocationBackend) IsTokenRevoked(tokenID string) bool {
	var count int
	err := d.db.QueryRow("SELECT COUNT(*) FROM revoked_tokens WHERE token_id = ? AND expires_at > ?",
        tokenID, time.Now()).Scan(&count)
	return err == nil && count > 0
}

// Implement other RevocationBackend methods...

// Use custom backend
customBackend := &DatabaseRevocationBackend{db: yourDB}
revocationManager := jwtprovider.NewRevocationManager(customBackend)
```

## User Token Tracking

The JWT provider supports comprehensive user token tracking for session management, security auditing, and bulk operations.

### Enabling Token Tracking

```go
// Configure provider with user token tracking
config := jwtprovider.NewJWTConfig()
config.TrackUserTokens = true    // Enable token tracking
config.MaxUserSessions = 5       // Limit concurrent sessions (0 = unlimited)

// Create revocation manager (required for tracking)
revocationMgr := jwtprovider.NewRevocationManager(
	jwtprovider.NewMemoryRevocationBackend(),
)

provider, err := jwtprovider.NewProvider(config,
	jwtprovider.WithRevocationManager(revocationMgr))
if err != nil {
	log.Fatal(err)
}
defer revocationMgr.Close()
```

### Session Management

```go
userID := "user123"

// Generate token (automatically tracked when enabled)
token, err := provider.GenerateToken(userID, map[string]any{
    "role": "admin",
})

// Handle session limit exceeded
if err == jwtprovider.ErrMaxSessionsExceeded {
	return fmt.Errorf("maximum concurrent sessions reached")
}

// Check active session count
sessionCount := provider.GetUserSessionCount(userID)
fmt.Printf("Active sessions: %d\n", sessionCount)

// Get all active tokens for user
activeTokens, err := provider.GetActiveUserTokens(userID)
if err != nil {
	log.Fatal(err)
}

fmt.Printf("User has %d active tokens\n", len(activeTokens))
for i, tokenID := range activeTokens {
	fmt.Printf("  %d. %s\n", i+1, tokenID)
}
```

### Bulk Token Operations

```go
// Revoke all tokens for a user (e.g., on password change)
err := provider.RevokeAllUserTokens(userID)
if err != nil {
	log.Fatal(err)
}

// Useful for security events:
// - Password reset
// - Account compromise
// - Security policy changes
// - User logout from all devices
```

### Complete Session Management Example

```go
package main

import (
    "fmt"
    "log"
    "github.com/oddbit-project/blueprint/provider/jwtprovider"
)

func main() {
    // Setup provider with session tracking
	config := jwtprovider.NewJWTConfig()
	config.TrackUserTokens = true
	config.MaxUserSessions = 3
    
    // Set up signing key
	signingKey := []byte("your-secret-key-32-bytes-minimum")
	config.CfgSigningKey = &secure.DefaultCredentialConfig{
        Password: string(signingKey),
    }
    
	revocationMgr := jwtprovider.NewRevocationManager(
        jwtprovider.NewMemoryRevocationBackend(),
    )
    
	provider, err := jwtprovider.NewProvider(config,
        jwtprovider.WithRevocationManager(revocationMgr))
	if err != nil {
        log.Fatal(err)
    }
	defer revocationMgr.Close()

	userID := "user123"
    
    // Generate multiple tokens
	fmt.Printf("Generating tokens for user %s...\n", userID)
    
	var tokens []string
	for i := 1; i <= 4; i++ {
        token, err := provider.GenerateToken(userID, map[string]any{
            "session_id": fmt.Sprintf("session_%d", i),
        })
        
        if err != nil {
            if err == jwtprovider.ErrMaxSessionsExceeded {
                fmt.Printf("Token %d: FAILED - session limit exceeded\n", i)
                continue
            }
            log.Fatal(err)
        }
        
        tokens = append(tokens, token)
        fmt.Printf("Token %d: SUCCESS\n", i)
        
        count := provider.GetUserSessionCount(userID)
        fmt.Printf("  Current sessions: %d\n", count)
    }
    
    // Security event: revoke all user tokens
	fmt.Println("\nSecurity event: revoking all user tokens...")
	err = provider.RevokeAllUserTokens(userID)
	if err != nil {
        log.Fatal(err)
    }
    
    // Verify all tokens are revoked
	fmt.Println("Verifying token revocation...")
	for i, token := range tokens {
        _, err := provider.ParseToken(token)
        if err != nil {
            fmt.Printf("Token %d: REVOKED ✓\n", i+1)
        } else {
            fmt.Printf("Token %d: STILL VALID ✗\n", i+1)
        }
    }
    
	finalCount := provider.GetUserSessionCount(userID)
	fmt.Printf("\nFinal session count: %d\n", finalCount)
}
```

### Security Benefits

**Session Control:**
- Prevent credential sharing by limiting concurrent sessions
- Automatically handle session limits during token generation
- Track all active sessions per user

**Security Response:**
- Quickly revoke all user tokens during security incidents
- Audit trail of token issuance and revocation
- Detect unusual session patterns

**Memory Management:**
- Automatic cleanup of expired token metadata
- Efficient storage with O(1) lookup performance
- Background cleanup prevents memory leaks

### Token Metadata

When tracking is enabled, the provider stores comprehensive metadata:

```go
type TokenMetadata struct {
	TokenID   string    `json:"tokenId"`
	UserID    string    `json:"userId"`
	IssuedAt  time.Time `json:"issuedAt"`
	ExpiresAt time.Time `json:"expiresAt"`
	ClientIP  string    `json:"clientIP,omitempty"`   // For future use
	UserAgent string    `json:"userAgent,omitempty"`  // For future use
}
```

### Error Handling

```go
const (
	ErrMaxSessionsExceeded = "maximum concurrent sessions exceeded"
)

// Handle session limits
token, err := provider.GenerateToken(userID, data)
if err != nil {
	switch err {
	case jwtprovider.ErrMaxSessionsExceeded:
        // Inform user about session limit
        return "Too many active sessions. Please log out from other devices."
	case jwtprovider.ErrNoRevocationManager:
        // Tracking requires revocation manager
        log.Error("Token tracking requires revocation manager")
	default:
        log.Errorf("Token generation failed: %v", err)
    }
}
```

### Performance Considerations

**Memory Usage:**
- O(n) memory where n = number of active tokens
- Automatic cleanup of expired metadata
- Configurable cleanup intervals

**Lookup Performance:**
- O(1) token revocation checks
- O(1) user session count queries
- O(k) user token retrieval where k = user's token count

**Concurrency:**
- Thread-safe operations with read-write locks
- Minimal lock contention for read operations
- Background cleanup doesn't block operations

## Token Refresh

```go
// Refresh an existing token
newToken, err := provider.Refresh(oldTokenString)
if err != nil {
	log.Fatal(err)
}

fmt.Printf("Refreshed token: %s\n", newToken)
```

## Claims Structure

```go
type Claims struct {
	jwt.RegisteredClaims              // Standard JWT claims
	Data map[string]any `json:"data,omitempty"` // Custom data
}

// Standard claims include:
// - Subject (sub)
// - Issuer (iss) 
// - Audience (aud)
// - ExpiresAt (exp)
// - NotBefore (nbf)
// - IssuedAt (iat)
// - ID (jti)
```

## Error Handling

```go
const (
	ErrInvalidSigningAlgorithm = "JWT signing algorithm is invalid"
	ErrInvalidToken            = "invalid token"
	ErrTokenExpired            = "token has expired"
	ErrMissingIssuer           = "issuer validation failed"
	ErrMissingAudience         = "audience validation failed"
	ErrNoRevocationManager     = "revocation manager not available"
	ErrTokenAlreadyRevoked     = "token is already revoked"
	ErrInvalidTokenID          = "invalid token ID"
	ErrMaxSessionsExceeded     = "maximum concurrent sessions exceeded"
	ErrTokenTooLarge           = "token too large"
	ErrTokenParsingTimeout     = "token parsing timeout"
)

// Example error handling
token, err := provider.GenerateToken("user123", data)
if err != nil {
	switch err {
	case jwtprovider.ErrInvalidSigningAlgorithm:
        log.Fatal("Invalid signing algorithm configured")
	case jwtprovider.ErrMaxSessionsExceeded:
        log.Printf("User has too many concurrent sessions")
        // Handle session limit - maybe offer to logout other devices
	case jwtprovider.ErrTokenTooLarge:
        log.Printf("Token payload too large, reduce custom claims")
	default:
        log.Fatalf("Token generation failed: %v", err)
    }
}
```

## Integration with HTTP Server

### Middleware Integration

```go
package main

import (
    "github.com/gin-gonic/gin"
    "github.com/oddbit-project/blueprint/log"
    "github.com/oddbit-project/blueprint/provider/httpserver"
    "github.com/oddbit-project/blueprint/provider/httpserver/auth"
    "github.com/oddbit-project/blueprint/provider/jwtprovider"
    "net/http"
    "os"
)

func main() {
    // Configure logger
	log.Configure(log.NewDefaultConfig())
	logger := log.New("jwt-demo")

    // Create server config
	serverConfig := httpserver.NewServerConfig()
	serverConfig.Host = "localhost"
	serverConfig.Port = 8080
	serverConfig.Debug = true

    // Create HTTP server
	server, err := httpserver.NewServer(serverConfig, logger)
	if err != nil {
        logger.Fatal(err, "could not create server")
        os.Exit(1)
    }

    // Set up JWT provider...
	provider, err := jwtprovider.NewProvider(config)
	if err != nil {
        logger.Fatal(err, "could not create JWT provider")
        os.Exit(1)
    }

    // Public route for login
	server.Route().POST("/login", loginHandler(provider))
    
    // Apply JWT authentication to all subsequent routes
	server.UseAuth(auth.NewAuthJWT(provider))
    
    // Protected routes
	server.Route().GET("/api/profile", profileHandler)
    
    // Start server
	if err := server.Start(); err != nil {
        logger.Fatal(err, "failed to start server")
    }
}

func profileHandler(c *gin.Context) {
    // Get claims using Blueprint auth helper
	claims, ok := auth.GetClaims(c)
	if !ok {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
        return
    }
	c.JSON(http.StatusOK, gin.H{"user": claims})
}

func loginHandler(provider jwtprovider.JWTProvider) gin.HandlerFunc {
	return func(c *gin.Context) {
        // Simple authentication
        var credentials struct {
            Username string `json:"username" binding:"required"`
            Password string `json:"password" binding:"required"`
        }
        
        if err := c.ShouldBindJSON(&credentials); err != nil {
            c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
            return
        }
        
        // Dummy validation
        if credentials.Username != "admin" || credentials.Password != "secret" {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
            return
        }
        
        token, err := provider.GenerateToken(credentials.Username, map[string]any{
            "role": "admin",
        })
        if err != nil {
            // Handle session limits gracefully
            if err == jwtprovider.ErrMaxSessionsExceeded {
                c.JSON(http.StatusTooManyRequests, gin.H{
                    "error": "Too many active sessions",
                    "message": "Please log out from other devices",
                    "active_sessions": provider.GetUserSessionCount(credentials.Username),
                })
                return
            }
            c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
            return
        }
        
        c.JSON(http.StatusOK, gin.H{
            "token": token,
            "active_sessions": provider.GetUserSessionCount(credentials.Username),
        })
    }
}
```

## Security Best Practices

### Key Management

1. **Use strong keys**: Minimum 256 bits for HMAC, 2048 bits for RSA
2. **Secure storage**: Use Blueprint's secure credential system
3. **Key rotation**: Implement regular key rotation
4. **Separate keys**: Use different keys for different environments

### Token Security

1. **Short expiration**: Use appropriate token lifetimes (15-60 minutes)
2. **Revocation**: Implement token revocation for security events
3. **Refresh tokens**: Use refresh tokens for long-lived sessions
4. **Secure transmission**: Always use HTTPS
5. **Validation**: Validate all claims (issuer, audience, expiration)
6. **Session limits**: Configure reasonable concurrent session limits
7. **Input validation**: Enable token size limits and parsing timeouts
8. **User tracking**: Enable token tracking for security audit trails

### Configuration Security

```go
// Production configuration example
config := jwtprovider.NewJWTConfig()
config.SigningAlgorithm = jwtprovider.RS256  // Asymmetric algorithm
config.ExpirationSeconds = 900               // 15 minutes
config.Issuer = "my-production-app"
config.Audience = "api-production"
config.RequireIssuer = true                  // Enforce validation
config.RequireAudience = true                // Enforce validation
config.KeyID = "prod-key-2024-01"           // Key identification
config.MaxTokenSize = 8192                   // 8KB token limit
config.TrackUserTokens = true                // Enable session tracking
config.MaxUserSessions = 5                   // Reasonable session limit
```

## JWKS (JSON Web Key Set) Support

```go
// Configure with Key ID for JWKS
config.KeyID = "my-key-1"

// The provider will include the "kid" header in generated tokens
// This allows for key rotation and multiple simultaneous keys
```

## Performance Considerations

1. **Algorithm choice**: HMAC algorithms are faster than asymmetric
2. **Key caching**: Keys are cached after first use
3. **Revocation backend**: In-memory backend is fastest, database backend for persistence
4. **Token size**: Minimize custom claims to reduce token size
5. **Validation caching**: Consider caching validation results for frequently accessed tokens
6. **Token tracking**: Adds minimal overhead (O(1) operations, automatic cleanup)
7. **Memory usage**: User tracking requires O(n) memory where n = active tokens
8. **Session limits**: Early validation prevents unnecessary token generation

## Complete Example Application

```go
package main

import (
    "github.com/gin-gonic/gin"
    "github.com/oddbit-project/blueprint/log"
    "github.com/oddbit-project/blueprint/provider/httpserver"
    "github.com/oddbit-project/blueprint/provider/httpserver/auth"
    "github.com/oddbit-project/blueprint/provider/jwtprovider"
    "github.com/oddbit-project/blueprint/crypt/secure"
    "net/http"
    "os"
)

func main() {
    // Configure logger
	log.Configure(log.NewDefaultConfig())
	logger := log.New("jwt-demo")

    // Create server config
	serverConfig := httpserver.NewServerConfig()
	serverConfig.Host = "localhost"
	serverConfig.Port = 8080
	serverConfig.Debug = true

    // Create HTTP server
	server, err := httpserver.NewServer(serverConfig, logger)
	if err != nil {
        logger.Fatal(err, "could not create server")
        os.Exit(1)
    }

    // Configure JWT provider using proper constructor
	config := jwtprovider.NewJWTConfig()
	config.SigningAlgorithm = jwtprovider.HS256
	config.ExpirationSeconds = 3600
	config.Issuer = "demo-app"
	config.Audience = "api"
	config.RequireIssuer = true
	config.RequireAudience = true
	config.TrackUserTokens = true // Enable session tracking
	config.MaxUserSessions = 3    // Limit concurrent sessions

    // Generate signing key
	signingKey, err := secure.GenerateKey()
	if err != nil {
        logger.Fatal(err, "could not generate signing key")
        os.Exit(1)
    }
    
	config.CfgSigningKey = &secure.DefaultCredentialConfig{Password: string(signingKey)}

    // Create provider with revocation
	revocationBackend := jwtprovider.NewMemoryRevocationBackend()
	revocationManager := jwtprovider.NewRevocationManager(revocationBackend)
    
	provider, err := jwtprovider.NewProvider(config,
        jwtprovider.WithRevocationManager(revocationManager))
	if err != nil {
        logger.Fatal(err, "could not create JWT provider")
        os.Exit(1)
    }

    // Public login route
	server.Route().POST("/login", func(c *gin.Context) {
        // Simple authentication (replace with real auth)
        var loginReq struct {
            Username string `json:"username" binding:"required"`
            Password string `json:"password" binding:"required"`
        }
        
        if err := c.ShouldBindJSON(&loginReq); err != nil {
            c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
            return
        }
        
        if loginReq.Username == "admin" && loginReq.Password == "secret" {
            token, err := provider.GenerateToken(loginReq.Username, map[string]any{
                "role": "admin",
            })
            if err != nil {
                // Handle session limits
                if err == jwtprovider.ErrMaxSessionsExceeded {
                    c.JSON(http.StatusTooManyRequests, gin.H{
                        "error": "Too many active sessions",
                        "active_sessions": provider.GetUserSessionCount(loginReq.Username),
                    })
                    return
                }
                c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
                return
            }
            
            c.JSON(http.StatusOK, gin.H{
                "token": token,
                "active_sessions": provider.GetUserSessionCount(loginReq.Username),
            })
        } else {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
        }
    })

    // Apply JWT authentication to protected routes
	server.UseAuth(auth.NewAuthJWT(provider))
    
	server.Route().POST("/logout", func(c *gin.Context) {
        claims, ok := auth.GetClaims(c)
        if !ok {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
            return
        }
        
        err := provider.RevokeTokenByID(claims.ID, claims.ExpiresAt.Time)
        if err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to logout"})
            return
        }
        
        c.JSON(http.StatusOK, gin.H{"message": "Logged out successfully"})
    })
    
	server.Route().GET("/api/profile", func(c *gin.Context) {
        claims, ok := auth.GetClaims(c)
        if !ok {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
            return
        }
        c.JSON(http.StatusOK, gin.H{"user": claims})
    })
        
    // Session management endpoints
	server.Route().GET("/api/sessions", func(c *gin.Context) {
        claims, ok := auth.GetClaims(c)
        if !ok {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
            return
        }
        
        activeTokens, err := provider.GetActiveUserTokens(claims.Subject)
        if err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get sessions"})
            return
        }
        
        c.JSON(http.StatusOK, gin.H{
            "active_sessions": len(activeTokens),
            "tokens": activeTokens,
        })
    })
        
	server.Route().DELETE("/api/sessions", func(c *gin.Context) {
        claims, ok := auth.GetClaims(c)
        if !ok {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
            return
        }
        
        err := provider.RevokeAllUserTokens(claims.Subject)
        if err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to revoke sessions"})
            return
        }
        
        c.JSON(http.StatusOK, gin.H{"message": "All sessions revoked"})
    })

    // Start server
	logger.Info("Server starting on http://localhost:8080")
	logger.Info("Available endpoints:")
	logger.Info("  POST /login        - Authenticate (public)")
	logger.Info("  POST /logout       - Logout (protected)")
	logger.Info("  GET  /api/profile  - User profile (protected)")
	logger.Info("  GET  /api/sessions - List sessions (protected)")
	logger.Info("  DELETE /api/sessions - Revoke all sessions (protected)")
    
	if err := server.Start(); err != nil {
        logger.Fatal(err, "failed to start server")
    }
}
```
