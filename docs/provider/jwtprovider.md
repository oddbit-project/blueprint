# blueprint.provider.jwtprovider

Blueprint JWT provider for comprehensive JSON Web Token authentication and management.

## Overview

The JWT provider offers a complete solution for managing JWT tokens in Go applications. It supports multiple signing algorithms, token revocation, refresh capabilities, and secure key management.

Key features:

- Multiple signing algorithms (HMAC, RSA, ECDSA, EdDSA)
- Token revocation system with pluggable backends
- Token refresh functionality
- JWKS (JSON Web Key Set) support
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
    SigningKey        *secure.DefaultCredentialConfig `json:\"signingKey,omitempty\"`     // For HMAC algorithms
    PrivateKey        *secure.KeyConfig               `json:\"privateKey,omitempty\"`     // For asymmetric algorithms
    PublicKey         *secure.KeyConfig               `json:\"publicKey,omitempty\"`      // For asymmetric algorithms
    SigningAlgorithm  string                          `json:\"signingAlgorithm\"`         // Algorithm to use
    ExpirationSeconds int                             `json:\"expirationSeconds\"`        // Token expiration
    Issuer            string                          `json:\"issuer\"`                   // Token issuer
    Audience          string                          `json:\"audience\"`                 // Token audience
    KeyID             string                          `json:\"keyID\"`                    // Key ID for JWKS
    RequireIssuer     bool                            `json:\"requireIssuer\"`            // Enforce issuer validation
    RequireAudience   bool                            `json:\"requireAudience\"`          // Enforce audience validation
}
```

### Default Values

```go
const (
    DefaultTTL      = time.Second * 86400 // 1 day
    DefaultIssuer   = \"blueprint\"
    DefaultAudience = \"api\"
)
```

## Basic Usage

### HMAC Algorithms (HS256/HS384/HS512)

```go
package main

import (
    \"fmt\"
    \"log\"
    \"github.com/oddbit-project/blueprint/provider/jwtprovider\"
    \"github.com/oddbit-project/blueprint/crypt/secure\"
)

func main() {
    // Create configuration for HMAC signing
    config := &jwtprovider.JWTConfig{
        SigningAlgorithm:  jwtprovider.HS256,
        ExpirationSeconds: 3600, // 1 hour
        Issuer:           \"my-app\",
        Audience:         \"api\",
        RequireIssuer:    true,
        RequireAudience:  true,
    }

    // Set up signing key
    signingKey, err := secure.GenerateKey()
    if err != nil {
        log.Fatal(err)
    }
    
    config.SigningKey = &secure.DefaultCredentialConfig{
        Key: signingKey,
    }

    // Create JWT provider
    provider, err := jwtprovider.NewProvider(config)
    if err != nil {
        log.Fatal(err)
    }

    // Generate a token
    customData := map[string]any{
        \"role\":        \"admin\",
        \"permissions\": []string{\"read\", \"write\"},
    }
    
    token, err := provider.GenerateToken(\"user123\", customData)
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Printf(\"Generated token: %s\\n\", token)

    // Parse and validate the token
    claims, err := provider.ParseToken(token)
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Printf(\"Subject: %s\\n\", claims.Subject)
    fmt.Printf(\"Custom data: %v\\n\", claims.Data)
}
```

### RSA Algorithms (RS256/RS384/RS512)

```go
package main

import (
    \"fmt\"
    \"log\"
    \"github.com/oddbit-project/blueprint/provider/jwtprovider\"
    \"github.com/oddbit-project/blueprint/crypt/secure\"
)

func main() {
    // Create configuration for RSA signing
    config := &jwtprovider.JWTConfig{
        SigningAlgorithm:  jwtprovider.RS256,
        ExpirationSeconds: 3600,
        Issuer:           \"my-app\",
        Audience:         \"api\",
        KeyID:            \"key-1\", // For JWKS support
    }

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
    config.PrivateKey = privateKeyConfig

    // Configure public key  
    publicKeyConfig, err := secure.NewKeyConfig([]byte(publicKeyPEM))
    if err != nil {
        log.Fatal(err)
    }
    config.PublicKey = publicKeyConfig

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
fmt.Printf(\"Token revoked: %v\\n\", isRevoked)
```

### Custom Revocation Backend

```go
// Implement custom revocation backend
type DatabaseRevocationBackend struct {
    db *sql.DB
}

func (d *DatabaseRevocationBackend) RevokeToken(tokenID string, expiresAt time.Time) error {
    _, err := d.db.Exec(\"INSERT INTO revoked_tokens (token_id, expires_at, revoked_at) VALUES (?, ?, ?)\",
        tokenID, expiresAt, time.Now())
    return err
}

func (d *DatabaseRevocationBackend) IsTokenRevoked(tokenID string) bool {
    var count int
    err := d.db.QueryRow(\"SELECT COUNT(*) FROM revoked_tokens WHERE token_id = ? AND expires_at > ?\",
        tokenID, time.Now()).Scan(&count)
    return err == nil && count > 0
}

// Implement other RevocationBackend methods...

// Use custom backend
customBackend := &DatabaseRevocationBackend{db: yourDB}
revocationManager := jwtprovider.NewRevocationManager(customBackend)
```

## Token Refresh

```go
// Refresh an existing token
newToken, err := provider.Refresh(oldTokenString)
if err != nil {
    log.Fatal(err)
}

fmt.Printf(\"Refreshed token: %s\\n\", newToken)
```

## Claims Structure

```go
type Claims struct {
    jwt.RegisteredClaims              // Standard JWT claims
    Data map[string]any `json:\"data,omitempty\"` // Custom data
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
    ErrInvalidSigningAlgorithm = \"JWT signing algorithm is invalid\"
    ErrInvalidToken            = \"invalid token\"
    ErrTokenExpired            = \"token has expired\"
    ErrMissingIssuer           = \"issuer validation failed\"
    ErrMissingAudience         = \"audience validation failed\"
    ErrNoRevocationManager     = \"revocation manager not available\"
    ErrTokenAlreadyRevoked     = \"token is already revoked\"
    ErrInvalidTokenID          = \"invalid token ID\"
)

// Example error handling
token, err := provider.GenerateToken(\"user123\", data)
if err != nil {
    switch err {
    case jwtprovider.ErrInvalidSigningAlgorithm:
        log.Fatal(\"Invalid signing algorithm configured\")
    default:
        log.Fatalf(\"Token generation failed: %v\", err)
    }
}
```

## Integration with HTTP Server

### Middleware Integration

```go
package main

import (
    \"github.com/gin-gonic/gin\"
    \"github.com/oddbit-project/blueprint/provider/jwtprovider\"
    \"github.com/oddbit-project/blueprint/provider/httpserver/response\"
)

func JWTMiddleware(provider jwtprovider.JWTProvider) gin.HandlerFunc {
    return func(c *gin.Context) {
        token := c.GetHeader(\"Authorization\")
        if token == \"\" {
            response.Http401(c)
            return
        }

        // Remove \"Bearer \" prefix
        if len(token) > 7 && token[:7] == \"Bearer \" {
            token = token[7:]
        }

        claims, err := provider.ParseToken(token)
        if err != nil {
            response.Http401(c)
            return
        }

        // Check revocation
        if provider.IsTokenRevoked(claims.ID) {
            response.Http401(c)
            return
        }

        // Store claims in context
        c.Set(\"jwt_claims\", claims)
        c.Next()
    }
}

func main() {
    // Set up provider...
    provider, err := jwtprovider.NewProvider(config)
    if err != nil {
        log.Fatal(err)
    }

    router := gin.Default()
    
    // Public routes
    router.POST(\"/login\", loginHandler(provider))
    
    // Protected routes
    protected := router.Group(\"/api\")
    protected.Use(JWTMiddleware(provider))
    protected.GET(\"/profile\", profileHandler)
    
    router.Run(\":8080\")
}

func loginHandler(provider jwtprovider.JWTProvider) gin.HandlerFunc {
    return func(c *gin.Context) {
        // Authenticate user...
        userID := \"user123\"
        
        token, err := provider.GenerateToken(userID, map[string]any{
            \"role\": \"user\",
        })
        if err != nil {
            response.Http500(c, err)
            return
        }
        
        c.JSON(200, gin.H{\"token\": token})
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

1. **Short expiration**: Use appropriate token lifetimes
2. **Revocation**: Implement token revocation for security events
3. **Refresh tokens**: Use refresh tokens for long-lived sessions
4. **Secure transmission**: Always use HTTPS
5. **Validation**: Validate all claims (issuer, audience, expiration)

### Configuration Security

```go
// Production configuration example
config := &jwtprovider.JWTConfig{
    SigningAlgorithm:  jwtprovider.RS256,        // Asymmetric algorithm
    ExpirationSeconds: 900,                       // 15 minutes
    Issuer:           \"my-production-app\",
    Audience:         \"api-production\",
    RequireIssuer:    true,                      // Enforce validation
    RequireAudience:  true,                      // Enforce validation
    KeyID:            \"prod-key-2024-01\",       // Key identification
}
```

## JWKS (JSON Web Key Set) Support

```go
// Configure with Key ID for JWKS
config.KeyID = \"my-key-1\"

// The provider will include the \"kid\" header in generated tokens
// This allows for key rotation and multiple simultaneous keys
```

## Performance Considerations

1. **Algorithm choice**: HMAC algorithms are faster than asymmetric
2. **Key caching**: Keys are cached after first use
3. **Revocation backend**: In-memory backend is fastest, database backend for persistence
4. **Token size**: Minimize custom claims to reduce token size
5. **Validation caching**: Consider caching validation results for frequently accessed tokens

## Complete Example Application

```go
package main

import (
    \"log\"
    \"time\"
    
    \"github.com/gin-gonic/gin\"
    \"github.com/oddbit-project/blueprint/provider/jwtprovider\"
    \"github.com/oddbit-project/blueprint/crypt/secure\"
    \"github.com/oddbit-project/blueprint/provider/httpserver/response\"
)

func main() {
    // Configure JWT provider
    config := &jwtprovider.JWTConfig{
        SigningAlgorithm:  jwtprovider.HS256,
        ExpirationSeconds: 3600,
        Issuer:           \"demo-app\",
        Audience:         \"api\",
        RequireIssuer:    true,
        RequireAudience:  true,
    }

    // Generate signing key
    signingKey, err := secure.GenerateKey()
    if err != nil {
        log.Fatal(err)
    }
    
    config.SigningKey = &secure.DefaultCredentialConfig{Key: signingKey}

    // Create provider with revocation
    revocationBackend := jwtprovider.NewMemoryRevocationBackend()
    revocationManager := jwtprovider.NewRevocationManager(revocationBackend)
    
    provider, err := jwtprovider.NewProvider(config,
        jwtprovider.WithRevocationManager(revocationManager))
    if err != nil {
        log.Fatal(err)
    }

    // Set up HTTP server
    router := gin.Default()
    
    router.POST(\"/login\", func(c *gin.Context) {
        // Simple authentication (replace with real auth)
        var loginReq struct {
            Username string `json:\"username\"`
            Password string `json:\"password\"`
        }
        
        if err := c.ShouldBindJSON(&loginReq); err != nil {
            response.Http400(c, \"Invalid request\")
            return
        }
        
        if loginReq.Username == \"admin\" && loginReq.Password == \"secret\" {
            token, err := provider.GenerateToken(loginReq.Username, map[string]any{
                \"role\": \"admin\",
            })
            if err != nil {
                response.Http500(c, err)
                return
            }
            
            c.JSON(200, gin.H{\"token\": token})
        } else {
            response.Http401(c)
        }
    })
    
    router.POST(\"/logout\", JWTMiddleware(provider), func(c *gin.Context) {
        claims, _ := c.Get(\"jwt_claims\")
        jwtClaims := claims.(*jwtprovider.Claims)
        
        err := provider.RevokeTokenByID(jwtClaims.ID, jwtClaims.ExpiresAt.Time)
        if err != nil {
            response.Http500(c, err)
            return
        }
        
        c.JSON(200, gin.H{\"message\": \"Logged out successfully\"})
    })
    
    protected := router.Group(\"/api\")
    protected.Use(JWTMiddleware(provider))
    {
        protected.GET(\"/profile\", func(c *gin.Context) {
            claims, _ := c.Get(\"jwt_claims\")
            c.JSON(200, gin.H{\"user\": claims})
        })
    }

    log.Println(\"Server starting on :8080\")
    router.Run(\":8080\")
}

func JWTMiddleware(provider jwtprovider.JWTProvider) gin.HandlerFunc {
    return func(c *gin.Context) {
        authHeader := c.GetHeader(\"Authorization\")
        if authHeader == \"\" {
            response.Http401(c)
            return
        }

        tokenString := authHeader
        if len(authHeader) > 7 && authHeader[:7] == \"Bearer \" {
            tokenString = authHeader[7:]
        }

        claims, err := provider.ParseToken(tokenString)
        if err != nil {
            response.Http401(c)
            return
        }

        if provider.IsTokenRevoked(claims.ID) {
            response.Http401(c)
            return
        }

        c.Set(\"jwt_claims\", claims)
        c.Next()
    }
}
```
