# HTTP Server Troubleshooting Guide

Comprehensive troubleshooting guide for Blueprint's HTTP server framework covering common issues, debugging techniques, and solution strategies.

## Common Issues and Solutions

### Server Startup Issues

#### Server fails to start with "address already in use"

**Problem:** Port is already bound by another process.

**Solutions:**
```bash
# Check what's using the port
lsof -i :8080
netstat -tulpn | grep :8080

# Kill the process using the port
kill -9 <PID>

# Or change your server port
config.Port = 8081
```

**Code solution:**
```go
// Graceful port handling
func findAvailablePort(startPort int) int {
    for port := startPort; port < startPort+100; port++ {
        listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
        if err == nil {
            listener.Close()
            return port
        }
    }
    return startPort // fallback
}

config.Port = findAvailablePort(8080)
```

#### Server starts but returns "connection refused"

**Problem:** Server binding to localhost but accessed from external IP.

**Solution:**
```go
// Wrong - only binds to localhost
config.Host = "localhost"

// Correct - binds to all interfaces
config.Host = "0.0.0.0"
config.Host = "" // empty string also binds to all interfaces
```

#### TLS certificate errors

**Problem:** Invalid or missing TLS certificates.

**Debugging:**
```go
func validateTLSConfig(config *httpserver.ServerConfig) error {
    if !config.TLSEnable {
        return nil
    }
    
    // Check certificate files exist
    if _, err := os.Stat(config.TLSCert); os.IsNotExist(err) {
        return fmt.Errorf("TLS certificate file not found: %s", config.TLSCert)
    }
    
    if _, err := os.Stat(config.TLSKey); os.IsNotExist(err) {
        return fmt.Errorf("TLS key file not found: %s", config.TLSKey)
    }
    
    // Test certificate loading
    _, err := tls.LoadX509KeyPair(config.TLSCert, config.TLSKey)
    if err != nil {
        return fmt.Errorf("invalid TLS certificate/key pair: %v", err)
    }
    
    return nil
}
```

### Authentication Issues

#### JWT tokens not working

**Problem:** JWT tokens are rejected or claims are not accessible.

**Debugging steps:**

1. **Check JWT Provider Configuration:**
```go
func debugJWTProvider(provider jwtprovider.JWTParser) {
    // Test token generation
    claims := &jwtprovider.Claims{
        Subject: "test-user",
        ID:      "test-token",
        Data:    map[string]interface{}{"role": "user"},
    }
    
    token, err := provider.GenerateToken(claims)
    if err != nil {
        log.Error("JWT generation failed", "error", err)
        return
    }
    
    // Test token parsing
    parsedClaims, err := provider.ParseToken(token)
    if err != nil {
        log.Error("JWT parsing failed", "error", err)
        return
    }
    
    log.Info("JWT test successful", "claims", parsedClaims)
}
```

2. **Check Authorization Header Format:**
```go
func debugAuthHeader(c *gin.Context) {
    authHeader := c.GetHeader("Authorization")
    log.Info("Authorization header", "value", authHeader)
    
    if len(authHeader) < 7 || authHeader[:7] != "Bearer " {
        log.Error("Invalid authorization header format")
        return
    }
    
    token := authHeader[7:]
    log.Info("Extracted token", "token", token)
}
```

3. **Verify Claims Context:**
```go
func debugJWTClaims(c *gin.Context) {
    claimsValue, exists := c.Get(auth.ContextJwtClaims)
    if !exists {
        log.Error("JWT claims not found in context")
        return
    }
    
    claims, ok := claimsValue.(*jwtprovider.Claims)
    if !ok {
        log.Error("Invalid claims type in context")
        return
    }
    
    log.Info("JWT claims found", "subject", claims.Subject, "id", claims.ID)
}
```

#### Token authentication failing

**Problem:** API key authentication not working.

**Common causes and solutions:**

1. **Wrong header name:**
```go
// Check what header the client is sending
func debugTokenAuth(c *gin.Context) {
    // Log all headers
    for name, values := range c.Request.Header {
        log.Info("Header", "name", name, "values", values)
    }
    
    // Check specific headers
    apiKey := c.GetHeader("X-API-Key")
    authHeader := c.GetHeader("Authorization")
    
    log.Info("Auth headers", "x-api-key", apiKey, "authorization", authHeader)
}

// Ensure header names match
tokenAuth := auth.NewAuthToken("X-API-Key", "your-secret") // Case sensitive
```

2. **Empty token secret (allows all requests):**
```go
// This allows all requests!
tokenAuth := auth.NewAuthToken("X-API-Key", "")

// Use a proper secret
tokenAuth := auth.NewAuthToken("X-API-Key", "your-secret-key")
```

### Session Issues

#### Sessions not persisting

**Problem:** Session data is lost between requests.

**Debugging steps:**

1. **Check cookie settings:**
```go
func debugSessionConfig(config *session.Config) {
    log.Info("Session config", 
        "secure", config.Secure,
        "httpOnly", config.HttpOnly,
        "sameSite", config.SameSite,
        "domain", config.Domain,
        "path", config.Path)
    
    // For development over HTTP
    if config.Secure && isHTTP() {
        log.Warn("Secure cookies enabled but using HTTP - sessions won't work")
    }
}
```

2. **Check session storage:**
```go
func debugSessionStorage(c *gin.Context) {
    sess := session.Get(c)
    
    log.Info("Session info",
        "id", sess.ID,
        "created", sess.Created,
        "lastAccessed", sess.LastAccessed,
        "values", sess.Values)
}
```

3. **Verify middleware order:**
```go
// Wrong order - CSRF before sessions
server.UseCSRFProtection()
server.UseSession(config, backend, logger)

// Correct order - sessions before CSRF
server.UseSession(config, backend, logger)
server.UseCSRFProtection()
```

#### Session expires too quickly

**Problem:** Sessions expire unexpectedly.

**Solution:**
```go
func adjustSessionTimeouts(config *session.Config) {
    // Increase timeouts
    config.ExpirationSeconds = 7200    // 2 hours absolute
    config.IdleTimeoutSeconds = 1800   // 30 minutes idle
    config.CleanupIntervalSeconds = 600 // 10 minutes cleanup
    
    log.Info("Session timeouts", 
        "expiration", config.ExpirationSeconds,
        "idle", config.IdleTimeoutSeconds)
}
```

### CSRF Protection Issues

#### CSRF validation failing

**Problem:** Valid requests are rejected with CSRF errors.

**Debugging:**

1. **Check token generation and inclusion:**
```go
func debugCSRFToken(c *gin.Context) {
    // Generate token
    token := security.GenerateCSRFToken(c)
    log.Info("Generated CSRF token", "token", token)
    
    // Check stored token
    storedToken := c.GetString("csrf-token")
    log.Info("Stored CSRF token", "token", storedToken)
    
    // Check submitted token
    submittedToken := c.GetHeader("X-CSRF-Token")
    if submittedToken == "" {
        submittedToken = c.PostForm("_csrf")
    }
    log.Info("Submitted CSRF token", "token", submittedToken)
}
```

2. **Verify middleware order:**
```go
// Sessions must come before CSRF protection
server.UseSession(config, backend, logger)
server.UseCSRFProtection()
```

3. **Check request method:**
```go
// CSRF only applies to state-changing methods
// GET, HEAD, OPTIONS are automatically allowed
log.Info("Request method", "method", c.Request.Method)
```

### Rate Limiting Issues

#### Rate limits too restrictive

**Problem:** Legitimate requests are being blocked.

**Solutions:**

1. **Adjust rate limits:**
```go
// Too restrictive
server.UseRateLimiting(10) // 10 per minute

// More reasonable
server.UseRateLimiting(100) // 100 per minute

// Or use custom rate limiting
r := rate.Every(time.Second / 10) // 10 requests per second
burst := 20
server.AddMiddleware(security.RateLimitMiddleware(r, burst))
```

2. **Different limits for different endpoints:**
```go
func setupDifferentialRateLimiting(server *httpserver.Server) {
    // Strict limits for auth endpoints
    auth := server.Group("/auth")
    auth.Use(security.RateLimitMiddleware(rate.Every(time.Minute/5), 2))
    
    // Moderate limits for API
    api := server.Group("/api")
    api.Use(security.RateLimitMiddleware(rate.Every(time.Second), 10))
    
    // Lenient limits for static content
    static := server.Group("/static")
    static.Use(security.RateLimitMiddleware(rate.Every(time.Second/10), 50))
}
```

#### Rate limiting not working

**Problem:** Rate limiting isn't being applied.

**Debugging:**
```go
func debugRateLimit(c *gin.Context) {
    // Check client IP detection
    clientIP := c.ClientIP()
    log.Info("Client IP", "ip", clientIP)
    
    // Check headers for proxy information
    forwardedFor := c.GetHeader("X-Forwarded-For")
    realIP := c.GetHeader("X-Real-IP")
    
    log.Info("Proxy headers", 
        "x-forwarded-for", forwardedFor,
        "x-real-ip", realIP)
}
```

### Middleware Issues

#### Middleware not executing

**Problem:** Middleware doesn't seem to be running.

**Debugging:**

1. **Add logging to middleware:**
```go
func debugMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        log.Info("Middleware executing", 
            "path", c.Request.URL.Path,
            "method", c.Request.Method)
        
        c.Next()
        
        log.Info("Middleware completed",
            "status", c.Writer.Status())
    }
}
```

2. **Check middleware order:**
```go
// Middleware executes in order of registration
server.AddMiddleware(debugMiddleware()) // Will run first
server.UseAuth(authProvider)           // Will run second
```

3. **Verify middleware isn't being bypassed:**
```go
func checkMiddlewareChain(c *gin.Context) {
    log.Info("Handler chain length", "count", len(c.Handlers))
    for i, handler := range c.Handlers {
        log.Info("Handler", "index", i, "name", runtime.FuncForPC(reflect.ValueOf(handler).Pointer()).Name())
    }
}
```

#### Middleware causing panics

**Problem:** Custom middleware is causing server crashes.

**Safe middleware pattern:**
```go
func safeMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        defer func() {
            if r := recover(); r != nil {
                log.Error("Middleware panic recovered", 
                    "panic", r,
                    "path", c.Request.URL.Path)
                
                if !c.Writer.Written() {
                    response.Http500(c, fmt.Errorf("internal error"))
                }
            }
        }()
        
        // Your middleware logic here
        c.Next()
    }
}
```

## Debugging Techniques

### Enable Debug Mode

```go
config := httpserver.NewServerConfig()
config.Debug = true // Enables Gin debug mode

// Or set Gin mode directly
gin.SetMode(gin.DebugMode)
```

### Request Logging

```go
func detailedRequestLogging() gin.HandlerFunc {
    return func(c *gin.Context) {
        start := time.Now()
        
        // Log request
        log.Info("Incoming request",
            "method", c.Request.Method,
            "path", c.Request.URL.Path,
            "query", c.Request.URL.RawQuery,
            "user-agent", c.Request.UserAgent(),
            "client-ip", c.ClientIP())
        
        // Log headers
        for name, values := range c.Request.Header {
            log.Info("Request header", "name", name, "values", values)
        }
        
        c.Next()
        
        // Log response
        duration := time.Since(start)
        log.Info("Request completed",
            "status", c.Writer.Status(),
            "duration", duration,
            "size", c.Writer.Size())
    }
}
```

### Error Debugging

```go
func errorDebugMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        c.Next()
        
        // Check for errors
        if len(c.Errors) > 0 {
            for _, err := range c.Errors {
                log.Error("Request error",
                    "error", err.Error(),
                    "type", err.Type,
                    "meta", err.Meta)
            }
        }
    }
}
```

### Configuration Debugging

```go
func debugServerConfig(config *httpserver.ServerConfig) {
    log.Info("Server configuration",
        "host", config.Host,
        "port", config.Port,
        "debug", config.Debug,
        "readTimeout", config.ReadTimeout,
        "writeTimeout", config.WriteTimeout,
        "tlsEnabled", config.TLSEnable)
    
    for key, value := range config.Options {
        log.Info("Server option", "key", key, "value", value)
    }
}
```

## Performance Debugging

### Slow Request Detection

```go
func slowRequestDetection(threshold time.Duration) gin.HandlerFunc {
    return func(c *gin.Context) {
        start := time.Now()
        c.Next()
        
        duration := time.Since(start)
        if duration > threshold {
            log.Warn("Slow request detected",
                "path", c.Request.URL.Path,
                "method", c.Request.Method,
                "duration", duration,
                "threshold", threshold)
        }
    }
}

// Usage
server.AddMiddleware(slowRequestDetection(500 * time.Millisecond))
```

### Memory Usage Monitoring

```go
func memoryMonitoring() gin.HandlerFunc {
    return func(c *gin.Context) {
        var m runtime.MemStats
        runtime.ReadMemStats(&m)
        
        log.Info("Memory stats",
            "alloc", bToMb(m.Alloc),
            "totalAlloc", bToMb(m.TotalAlloc),
            "sys", bToMb(m.Sys),
            "numGoroutines", runtime.NumGoroutine())
        
        c.Next()
    }
}

func bToMb(b uint64) uint64 {
    return b / 1024 / 1024
}
```

## Testing and Validation

### Configuration Validation

```go
func validateConfiguration(config *httpserver.ServerConfig) error {
    var errors []string
    
    if config.Port <= 0 || config.Port > 65535 {
        errors = append(errors, "invalid port number")
    }
    
    if config.ReadTimeout <= 0 {
        errors = append(errors, "read timeout must be positive")
    }
    
    if config.WriteTimeout <= 0 {
        errors = append(errors, "write timeout must be positive")
    }
    
    if config.TLSEnable {
        if config.TLSCert == "" {
            errors = append(errors, "TLS certificate required when TLS enabled")
        }
        if config.TLSKey == "" {
            errors = append(errors, "TLS key required when TLS enabled")
        }
    }
    
    if len(errors) > 0 {
        return fmt.Errorf("configuration errors: %s", strings.Join(errors, ", "))
    }
    
    return nil
}
```

### Health Check Endpoint

```go
func healthCheckEndpoint(server *httpserver.Server) {
    server.Route().GET("/debug/health", func(c *gin.Context) {
        checks := map[string]string{
            "server":    "healthy",
            "memory":    checkMemoryUsage(),
            "goroutines": checkGoroutineCount(),
        }
        
        allHealthy := true
        for _, status := range checks {
            if status != "healthy" {
                allHealthy = false
                break
            }
        }
        
        statusCode := 200
        if !allHealthy {
            statusCode = 503
        }
        
        c.JSON(statusCode, gin.H{
            "status": map[string]bool{"healthy": allHealthy},
            "checks": checks,
            "timestamp": time.Now(),
        })
    })
}

func checkMemoryUsage() string {
    var m runtime.MemStats
    runtime.ReadMemStats(&m)
    
    if m.Alloc > 500*1024*1024 { // 500MB threshold
        return "warning"
    }
    return "healthy"
}

func checkGoroutineCount() string {
    count := runtime.NumGoroutine()
    if count > 1000 { // 1000 goroutines threshold
        return "warning"
    }
    return "healthy"
}
```

## Common Error Messages and Solutions

### "http: multiple response.WriteHeader calls"
**Cause:** Multiple middleware or handlers trying to write response headers.
**Solution:** Ensure only one response is sent per request, use `c.Writer.Written()` to check.

### "context canceled"
**Cause:** Client disconnected or request timeout.
**Solution:** Handle context cancellation in long-running operations.

### "bind: address already in use"
**Cause:** Port is already occupied.
**Solution:** Check for existing processes, use different port, or implement port discovery.

### "tls: private key does not match public key"
**Cause:** TLS certificate and key files don't match.
**Solution:** Regenerate certificate/key pair or verify correct file paths.

### "session not found"
**Cause:** Session expired or storage backend unavailable.
**Solution:** Check session configuration, verify backend connectivity.

This troubleshooting guide covers the most common issues encountered when developing with Blueprint's HTTP server framework. Always check logs first, use debugging middleware, and validate configuration before deploying to production.