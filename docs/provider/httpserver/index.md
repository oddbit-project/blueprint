# HTTP Server Framework

Blueprint provides a comprehensive HTTP server framework built on Gin with integrated security, authentication, session management, and middleware components for building robust web applications and APIs.

## Architecture Overview

The HTTP server framework consists of several key components:

- **Core Server**: HTTP server setup and lifecycle management (`provider/httpserver/server.go`)
- **Authentication**: Token-based and JWT authentication ([auth.md](auth.md))
- **Security**: Headers, CSRF protection, and rate limiting ([security.md](security.md)) 
- **Session Management**: Cookie-based sessions with multiple storage backends ([session.md](session.md))
- **Middleware**: Response helpers and utility middleware ([middleware.md](middleware.md))
- **Response Utilities**: Standardized HTTP response helpers (`provider/httpserver/response/`)

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
    config.Address = ":8080"
    
    // Create and start server
    server := httpserver.NewServer(config, logger)
    
    // Add routes
    server.Router().GET("/health", func(c *gin.Context) {
        c.JSON(200, gin.H{"status": "healthy"})
    })
    
    // Start server
    if err := server.Start(); err != nil {
        logger.Fatal(err, "failed to start server")
    }
}
```

### Server with Authentication and Security

```go
package main

import (
    "github.com/oddbit-project/blueprint/provider/httpserver"
    "github.com/oddbit-project/blueprint/provider/httpserver/auth"
    "github.com/oddbit-project/blueprint/provider/httpserver/security"
    "github.com/oddbit-project/blueprint/log"
)

func main() {
    logger := log.New("secure-server")
    
    // Server configuration
    config := httpserver.NewConfig()
    config.Address = ":8080"
    
    server := httpserver.NewServer(config, logger)
    router := server.Router()
    
    // Apply security headers
    securityConfig := security.DefaultSecurityConfig()
    router.Use(security.SecurityMiddleware(securityConfig))
    
    // Apply rate limiting
    router.Use(security.RateLimitMiddleware(rate.Every(time.Second), 10))
    
    // Public routes
    router.GET("/health", healthHandler)
    router.POST("/login", loginHandler)
    
    // Protected API routes
    tokenAuth := auth.NewAuthToken("X-API-Key", "your-api-key")
    api := router.Group("/api")
    api.Use(auth.AuthMiddleware(tokenAuth))
    {
        api.GET("/users", getUsersHandler)
        api.POST("/users", createUserHandler)
    }
    
    // Start server
    if err := server.Start(); err != nil {
        logger.Fatal(err, "failed to start server")
    }
}
```

## Components

### Core Server API
- [API Reference](api-reference.md) - Complete server API documentation
- Server lifecycle management (start, stop, graceful shutdown)
- Configuration options and environment variables
- Router and handler registration

### Authentication & Authorization
- [Authentication](auth.md) - Token-based and JWT authentication providers
- Unified authentication interface with multiple implementations
- Bearer token support and JWT claims context injection
- Custom authentication provider development

### Security Features
- [Security](security.md) - Comprehensive security middleware
- Security headers (CSP, HSTS, XSS protection, frame options)
- CSRF protection with token generation and validation
- Rate limiting with per-IP and per-endpoint controls

### Session Management
- [Session Management](session.md) - Cookie-based session system
- Multiple storage backends (memory, Redis, custom KV stores)
- Flash messages and session regeneration
- Configurable expiration and idle timeouts

### Middleware Components
- [Middleware Guide](middleware.md) - All available middleware
- Response helpers and standardized error handling
- Custom middleware development patterns
- Middleware ordering and best practices

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

## Troubleshooting

For debugging and troubleshooting information:
- [Troubleshooting Guide](troubleshooting.md) - Common issues and solutions
- Configuration troubleshooting
- Middleware debugging techniques
- Performance issue diagnosis

## Performance and Production

For production deployment guidance:
- [Performance Guide](performance.md) - Optimization and scaling
- Connection management and timeouts
- Load balancing strategies
- Monitoring and observability setup

## Next Steps

1. Start with the [API Reference](api-reference.md) for complete server documentation
2. Review [Authentication](auth.md) for securing your endpoints
3. Implement [Security](security.md) headers and CSRF protection
4. Add [Session Management](session.md) for stateful web applications
5. Check [Examples](examples.md) for complete integration patterns
