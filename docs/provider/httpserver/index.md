# HTTP Server Framework

Blueprint provides a comprehensive HTTP server framework built on Gin with integrated security, authentication, 
session management, and middleware components for building robust web applications and APIs.

## Architecture Overview

The HTTP server framework consists of several key components:

- **Core Server**: HTTP server setup and lifecycle management with TLS support
- **Session Management**: Cookie-based sessions with multiple storage backends and encryption
- **Security**: Comprehensive security headers, CSRF protection, and rate limiting
- **Authentication**: Token-based, JWT, and session-based authentication providers
- **Middleware**: Extensible middleware system with security and utility components
- **Device Fingerprinting**: Multi-factor device identification for enhanced security

## Features

### Core Features
- **Gin Framework Integration**: High-performance HTTP router with middleware support
- **Graceful Shutdown**: Proper server lifecycle management with context handling
- **TLS Configuration**: Built-in HTTPS support with certificate management
- **Structured Logging**: Integrated logging with configurable levels
- **Configuration Management**: JSON-based configuration with validation

### Security Features
- **Session Management**: Secure cookie-based sessions with encryption
- **CSRF Protection**: Token-based protection against cross-site request forgery
- **Security Headers**: Comprehensive browser security protections (CSP, HSTS, XSS protection)
- **Rate Limiting**: Token bucket algorithm with per-IP and per-endpoint controls
- **Device Fingerprinting**: Multi-factor device identification and change detection

### Authentication & Authorization
- **Multiple Auth Providers**: JWT, HMAC, token-based, and session-based authentication
- **Unified Interface**: Consistent authentication API across all providers
- **Context Integration**: Authentication data available in request context
- **Flexible Authorization**: Route-based and middleware-based protection

## Quick Start

### Basic HTTP Server

```go
package main

import (
    "github.com/oddbit-project/blueprint/provider/httpserver"
    "github.com/oddbit-project/blueprint/log"
)

func main() {
    logger := log.New("http-server")
    
    // Create server configuration
    config := httpserver.NewConfig()
    config.Host = "localhost"
    config.Port = 8080
    
    // Create and start server
    server := httpserver.NewServer(config, logger)
    
    // Add routes
    server.Route().GET("/health", func(c *gin.Context) {
        c.JSON(200, gin.H{"status": "healthy"})
    })
    
    // Start server
    if err := server.Start(); err != nil {
        logger.Fatal(err, "failed to start server")
    }
}
```

### Server with Sessions and Security

```go
package main

import (
    "github.com/oddbit-project/blueprint/provider/httpserver"
    "github.com/oddbit-project/blueprint/provider/httpserver/session"
    "github.com/oddbit-project/blueprint/provider/httpserver/security"
    "github.com/oddbit-project/blueprint/provider/kv"
    "github.com/oddbit-project/blueprint/log"
)

func main() {
    logger := log.New("secure-server")
    
    // Server configuration
    config := httpserver.NewConfig()
    config.Host = "localhost"
    config.Port = 8080
    
    server := httpserver.NewServer(config, logger)
    
    // Session configuration
    sessionConfig := session.NewConfig()
    sessionConfig.Secure = true
    sessionConfig.HttpOnly = true
    
    // Use memory-based session store
    backend := kv.NewMemoryKV()
    sessionManager, err := server.UseSession(sessionConfig, backend, logger)
    if err != nil {
        logger.Fatal(err, "failed to setup sessions")
    }
    
    // Apply security headers
    securityConfig := security.DefaultSecurityConfig()
    server.Route().Use(security.SecurityMiddleware(securityConfig))
    
    // Apply CSRF protection
    server.Route().Use(security.CSRFProtection())
    
    // Apply rate limiting
    server.Route().Use(security.RateLimitMiddleware(rate.Every(time.Second), 10))
    
    // Routes
    setupRoutes(server)
    
    // Start server
    if err := server.Start(); err != nil {
        logger.Fatal(err, "failed to start server")
    }
}

func setupRoutes(server *httpserver.Server) {
    router := server.Route()
    
    // Public routes
    router.GET("/", homeHandler)
    router.GET("/login", loginFormHandler)
    router.POST("/login", loginHandler)
    
    // Protected routes with session authentication
    protected := router.Group("/dashboard")
    protected.Use(auth.AuthMiddleware(auth.NewAuthSession(&UserIdentity{}))) // &UserIdentity{} is the user identity type to be used
    {
        protected.GET("/", dashboardHandler)
        protected.POST("/logout", logoutHandler)
    }
}
```

## Components

### Core Server API
- [API Reference](api-reference.md) - Complete server API documentation
- Server lifecycle management (start, stop, graceful shutdown)
- Configuration options and environment variables
- Router and handler registration

### Session Management
- [Session Management](session.md) - Comprehensive session system
- Multiple storage backends (memory, Redis, custom KV stores)
- Cookie security configuration (HttpOnly, Secure, SameSite)
- Session encryption with AES256GCM
- Flash messages and session regeneration
- Automatic cleanup and expiration handling

### Security Features
- [Security](security.md) - Complete security middleware
- Security headers (CSP with nonce support, HSTS, XSS protection, frame options)
- CSRF protection with token generation and validation
- Rate limiting with token bucket algorithm and per-IP tracking
- Device fingerprinting for enhanced security

### Authentication & Authorization
- [Authentication](auth.md) - All authentication providers
- JWT authentication with claims context injection
- Token-based authentication for APIs
- Session-based authentication for web applications
- HMAC authentication for secure service communication
- Custom authentication provider development

### Middleware Components
- [Middleware Guide](middleware.md) - All available middleware
- Response helpers and standardized error handling
- Custom middleware development patterns
- Middleware ordering and best practices

### Request Validation
- [Request Validation](validation.md) - Two-stage validation system
- JSON request body validation with `ValidateJSON()`
- Query parameter validation with `ValidateQuery()`
- Custom validation logic with `Validator` interface
- Nested structure and collection validation
- Field-specific error reporting with full paths

## Configuration

### Server Configuration

```go
type ServerConfig struct {
    Host         string            `json:"host"`         // Server host (default: "localhost")
    Port         int               `json:"port"`         // Server port (default: 8080)
    CertFile     string            `json:"certFile"`     // TLS certificate file
    CertKeyFile  string            `json:"certKeyFile"`  // TLS private key file
    ReadTimeout  int               `json:"readTimeout"`  // Read timeout in seconds
    WriteTimeout int               `json:"writeTimeout"` // Write timeout in seconds
    Debug        bool              `json:"debug"`        // Enable debug mode
    Options      map[string]string `json:"options"`      // Additional options
}
```

### Session Configuration

```go
type SessionConfig struct {
    CookieName             string                         `json:"cookieName"`             // Cookie name (default: "blueprint_session")
    ExpirationSeconds      int                            `json:"expirationSeconds"`      // Session lifetime (default: 1800)
    IdleTimeoutSeconds     int                            `json:"idleTimeoutSeconds"`     // Idle timeout (default: 900)
    Secure                 bool                           `json:"secure"`                 // HTTPS only (default: true)
    HttpOnly               bool                           `json:"httpOnly"`               // No JS access (default: true)
    SameSite               int                            `json:"sameSite"`               // CSRF protection (default: Strict)
    Domain                 string                         `json:"domain"`                 // Cookie domain
    Path                   string                         `json:"path"`                   // Cookie path (default: "/")
    EncryptionKey          secure.DefaultCredentialConfig `json:"encryptionKey"`          // Optional encryption
    CleanupIntervalSeconds int                            `json:"cleanupIntervalSeconds"` // Cleanup frequency (default: 300)
}
```

## Common Use Cases

### REST API Server
- JWT authentication for stateless API access
- Rate limiting to prevent abuse
- Security headers for browser protection
- Structured JSON responses with error handling

### Web Application
- Session-based authentication for user login
- CSRF protection for form submissions
- Security headers and CSP for XSS prevention
- Flash messages for user feedback
- Device fingerprinting for security

### Microservice
- Token-based authentication for service-to-service communication
- Health check endpoints for orchestration
- Structured logging and error handling
- Graceful shutdown for container environments

## Integration Examples

For comprehensive examples showing how to combine all components:
- [Integration Examples](examples.md) - Complete setup examples
- REST API with authentication and rate limiting
- Web application with sessions and CSRF protection
- Microservice with health checks and monitoring

## Sample Applications

### httpserver-session Sample
The `samples/httpserver-session/` directory contains a complete example demonstrating:

- **Session Management**: Memory-based session store with secure cookies
- **Authentication**: Session-based authentication with custom identity types
- **Security**: CSRF protection and security headers
- **Configuration**: JSON-based configuration with validation
- **Best Practices**: Proper middleware ordering and error handling

Key features demonstrated:
```go
// Session setup with security
sessionConfig := session.NewConfig()
sessionConfig.Secure = true
sessionConfig.HttpOnly = true
sessionConfig.ExpirationSeconds = 3600

// Authentication with custom identity
type UserIdentity struct {
    ID       int    `json:"id"`
    Username string `json:"username"`
    Email    string `json:"email"`
}

// Protected routes
protected.Use(auth.AuthMiddleware(auth.NewAuthSession(&UserIdentity{})))
```

## Performance and Production

For production deployment guidance:
- [Performance Guide](performance.md) - Optimization and scaling
- Connection management and timeouts
- Load balancing strategies
- Monitoring and observability setup

## Troubleshooting

For debugging and troubleshooting information:
- [Troubleshooting Guide](troubleshooting.md) - Common issues and solutions
- Configuration troubleshooting
- Middleware debugging techniques
- Performance issue diagnosis

## Next Steps

1. Start with the [API Reference](api-reference.md) for complete server documentation
2. Implement [Request Validation](validation.md) for input validation and business logic
3. Review [Session Management](session.md) for stateful web applications
4. Implement [Security](security.md) headers and CSRF protection
5. Add [Authentication](auth.md) for securing your endpoints
6. Check [Examples](examples.md) for complete integration patterns