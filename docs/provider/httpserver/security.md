# HTTP Security

Blueprint provides comprehensive security features for HTTP applications including authentication, authorization, security headers, CSRF protection, and rate limiting.

## Security Components

Blueprint's security system consists of several layers:

1. **Authentication**: Identity verification ([Authentication Documentation](auth.md))
2. **Security Headers**: Browser security protections  
3. **CSRF Protection**: Cross-Site Request Forgery prevention
4. **Rate Limiting**: Request throttling and DDoS protection
5. **Content Security Policy**: XSS and injection prevention

## Security Headers Middleware

Blueprint provides a comprehensive security headers middleware that implements industry best practices.

### Default Security Configuration

```go
import "github.com/oddbit-project/blueprint/provider/httpserver/security"

// Apply default security headers
securityConfig := security.DefaultSecurityConfig()
router.Use(security.SecurityMiddleware(securityConfig))
```

**Default Headers Applied:**
- **Content Security Policy**: Strict CSP with nonce support
- **X-XSS-Protection**: Browser XSS filtering
- **X-Content-Type-Options**: MIME type sniffing prevention
- **X-Frame-Options**: Clickjacking prevention  
- **Strict-Transport-Security**: HTTPS enforcement
- **Referrer-Policy**: Referrer information control
- **Feature-Policy/Permissions-Policy**: Browser feature restrictions
- **Cache-Control**: Sensitive data caching prevention

### Custom Security Configuration

```go
securityConfig := &security.SecurityConfig{
    CSP:                "default-src 'self'; script-src 'self' 'unsafe-inline'",
    XSSProtection:      "1; mode=block",
    ContentTypeOptions: "nosniff",
    ReferrerPolicy:     "no-referrer",
    HSTS:               "max-age=63072000; includeSubDomains; preload",
    FrameOptions:       "SAMEORIGIN",
    FeaturePolicy:      "camera 'none'; microphone 'none'",
    CacheControl:       "no-store, must-revalidate",
    UseCSPNonce:        true,
    EnableRateLimit:    true,
    RateLimit:          100, // requests per minute
}

router.Use(security.SecurityMiddleware(securityConfig))
```

## Content Security Policy (CSP)

### CSP with Nonce Support

Blueprint automatically generates unique nonces for each request when enabled:

```go
config := security.DefaultSecurityConfig()
config.CSP = "default-src 'self'; script-src 'self' 'nonce-{nonce}'; style-src 'self' 'nonce-{nonce}'"
config.UseCSPNonce = true

router.Use(security.SecurityMiddleware(config))
```

**In your templates:**
```html
<!-- Get nonce from context -->
{{ $nonce := .nonce }}

<!-- Use in script tags -->
<script nonce="{{ $nonce }}">
    console.log('This script is CSP-compliant');
</script>

<!-- Use in style tags -->
<style nonce="{{ $nonce }}">
    .secure-style { color: blue; }
</style>
```

**In handlers:**
```go
func pageHandler(c *gin.Context) {
    // Get the CSP nonce
    nonce, exists := c.Get("csp-nonce")
    if !exists {
        nonce = ""
    }
    
    c.HTML(200, "page.html", gin.H{
        "nonce": nonce,
        "data":  pageData,
    })
}
```

### CSP Reporting

Set up CSP violation reporting:

```go
config.CSP = "default-src 'self'; script-src 'self' 'nonce-{nonce}'; report-uri /csp-report"

// Handle CSP reports
router.POST("/csp-report", func(c *gin.Context) {
    var report map[string]interface{}
    if err := c.ShouldBindJSON(&report); err == nil {
        // Log or process CSP violation
        logger.Warn("CSP Violation", "report", report)
    }
    c.Status(204)
})
```

## CSRF Protection

Blueprint provides built-in CSRF (Cross-Site Request Forgery) protection.

### Basic CSRF Setup

```go
import "github.com/oddbit-project/blueprint/provider/httpserver/security"

// Apply CSRF protection to all routes
router.Use(security.CSRFProtection())

// Generate CSRF tokens in handlers
router.GET("/form", func(c *gin.Context) {
    csrfToken := security.GenerateCSRFToken(c)
    c.HTML(200, "form.html", gin.H{
        "csrfToken": csrfToken,
    })
})
```

### CSRF Token Usage

**In HTML Forms:**
```html
<form method="POST" action="/submit">
    <!-- Include CSRF token as hidden field -->
    <input type="hidden" name="_csrf" value="{{ .csrfToken }}">
    
    <input type="text" name="data" required>
    <button type="submit">Submit</button>
</form>
```

**In AJAX Requests:**
```javascript
// Include CSRF token in header
fetch('/api/data', {
    method: 'POST',
    headers: {
        'Content-Type': 'application/json',
        'X-CSRF-Token': csrfToken
    },
    body: JSON.stringify(data)
});
```

**Getting CSRF Token via API:**
```go
router.GET("/csrf-token", func(c *gin.Context) {
    token := security.GenerateCSRFToken(c)
    c.JSON(200, gin.H{"csrf_token": token})
})
```

### CSRF with Sessions

Combine CSRF with session management for enhanced security:

```go
// Setup session middleware first
sessionManager := session.NewManager(store, sessionConfig, logger)
router.Use(sessionManager.Middleware())

// Then add CSRF protection
router.Use(security.CSRFProtection())

func formHandler(c *gin.Context) {
    // Generate CSRF token (stored in session)
    csrfToken := security.GenerateCSRFToken(c)
    
    // Get session for other data
    sess := session.Get(c)
    
    c.HTML(200, "form.html", gin.H{
        "csrfToken": csrfToken,
        "user":      sess.GetString("user_id"),
    })
}
```

## Rate Limiting

Blueprint provides flexible rate limiting to protect against abuse and DDoS attacks.

### Basic Rate Limiting

```go
import (
    "golang.org/x/time/rate"
    "github.com/oddbit-project/blueprint/provider/httpserver/security"
)

// Apply rate limiting: 60 requests per minute, burst of 10
rateLimit := rate.Every(time.Minute / 60) // 1 request per second
burstSize := 10

router.Use(security.RateLimitMiddleware(rateLimit, burstSize))
```

### Different Rate Limits for Different Routes

```go
func setupRateRoutes(router *gin.Engine) {
    // Strict rate limiting for auth endpoints
    authLimit := rate.Every(time.Minute / 5) // 5 requests per minute
    auth := router.Group("/auth")
    auth.Use(security.RateLimitMiddleware(authLimit, 2))
    {
        auth.POST("/login", loginHandler)
        auth.POST("/register", registerHandler)
    }
    
    // Moderate rate limiting for API
    apiLimit := rate.Every(time.Second) // 1 request per second
    api := router.Group("/api")
    api.Use(security.RateLimitMiddleware(apiLimit, 10))
    {
        api.GET("/data", getDataHandler)
        api.POST("/data", createDataHandler)
    }
    
    // Lenient rate limiting for static content
    staticLimit := rate.Every(time.Second / 10) // 10 requests per second
    static := router.Group("/static")
    static.Use(security.RateLimitMiddleware(staticLimit, 50))
    {
        static.Static("/", "./static")
    }
}
```

### Rate Limiting Configuration

```go
// Create custom rate limiter
limiter := security.NewClientRateLimiter(
    rate.Every(time.Minute/100), // 100 requests per minute
    20,                          // Burst size of 20
)

// The rate limiter automatically:
// - Tracks per-IP limits
// - Handles proxy headers (X-Forwarded-For)
// - Cleans up old limiters
// - Returns 429 Too Many Requests when exceeded
```

## Comprehensive Security Setup

Here's a complete example combining all security features:

```go
package main

import (
    "github.com/gin-gonic/gin"
    "github.com/oddbit-project/blueprint/provider/httpserver/auth"
    "github.com/oddbit-project/blueprint/provider/httpserver/security"
    "github.com/oddbit-project/blueprint/provider/httpserver/session"
    "golang.org/x/time/rate"
    "time"
)

func setupSecureServer() *gin.Engine {
    router := gin.Default()
    
    // 1. Security Headers (apply first)
    securityConfig := security.DefaultSecurityConfig()
    securityConfig.CSP = "default-src 'self'; script-src 'self' 'nonce-{nonce}'; style-src 'self' 'nonce-{nonce}'"
    router.Use(security.SecurityMiddleware(securityConfig))
    
    // 2. Rate Limiting (apply early)
    generalRateLimit := rate.Every(time.Second / 2) // 2 requests per second
    router.Use(security.RateLimitMiddleware(generalRateLimit, 10))
    
    // 3. Session Management
    sessionConfig := session.NewConfig()
    sessionManager := session.NewManager(sessionStore, sessionConfig, logger)
    router.Use(sessionManager.Middleware())
    
    // 4. CSRF Protection (after sessions)
    router.Use(security.CSRFProtection())
    
    // Public routes
    setupPublicRoutes(router)
    
    // Protected routes with authentication
    setupProtectedRoutes(router)
    
    return router
}

func setupPublicRoutes(router *gin.Engine) {
    router.GET("/", homeHandler)
    router.GET("/login", loginFormHandler)
    router.POST("/login", loginHandler)
    router.GET("/csrf-token", csrfTokenHandler)
}

func setupProtectedRoutes(router *gin.Engine) {
    // JWT Authentication for API
    jwtAuth := auth.NewAuthJWT(jwtProvider)
    api := router.Group("/api")
    api.Use(auth.AuthMiddleware(jwtAuth))
    {
        // Stricter rate limit for API
        apiRateLimit := rate.Every(time.Second) // 1 request per second
        api.Use(security.RateLimitMiddleware(apiRateLimit, 5))
        
        api.GET("/user/profile", getProfileHandler)
        api.PUT("/user/profile", updateProfileHandler)
    }
    
    // Token Authentication for admin
    tokenAuth := auth.NewAuthToken("X-Admin-Key", adminKey)
    admin := router.Group("/admin")
    admin.Use(auth.AuthMiddleware(tokenAuth))
    {
        // Very strict rate limit for admin
        adminRateLimit := rate.Every(time.Minute / 10) // 10 requests per minute
        admin.Use(security.RateLimitMiddleware(adminRateLimit, 2))
        
        admin.GET("/users", listUsersHandler)
        admin.DELETE("/users/:id", deleteUserHandler)
    }
}
```

## Security Best Practices

### Production Security Checklist

1. **HTTPS Configuration**
   ```go
   // Enforce HTTPS in production
   securityConfig.HSTS = "max-age=63072000; includeSubDomains; preload"
   
   // Redirect HTTP to HTTPS
   router.Use(func(c *gin.Context) {
       if c.Request.Header.Get("X-Forwarded-Proto") == "http" {
           httpsURL := "https://" + c.Request.Host + c.Request.RequestURI
           c.Redirect(301, httpsURL)
           return
       }
       c.Next()
   })
   ```

2. **Environment-based Configuration**
   ```go
   func getSecurityConfig() *security.SecurityConfig {
       config := security.DefaultSecurityConfig()
       
       if os.Getenv("ENV") == "production" {
           config.HSTS = "max-age=63072000; includeSubDomains; preload"
           config.FrameOptions = "DENY"
           config.RateLimit = 60 // Stricter in production
       } else {
           config.HSTS = "" // No HSTS in development
           config.RateLimit = 1000 // More lenient in development
       }
       
       return config
   }
   ```

3. **Logging and Monitoring**
   ```go
   // Log security events
   func securityLogger() gin.HandlerFunc {
       return gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
           if param.StatusCode == 429 {
               logger.Warn("Rate limit exceeded", 
                   "ip", param.ClientIP,
                   "path", param.Path,
                   "user_agent", param.Request.UserAgent())
           }
           if param.StatusCode == 403 {
               logger.Warn("CSRF validation failed",
                   "ip", param.ClientIP,
                   "path", param.Path)
           }
           return ""
       })
   }
   ```

### Security Headers Explained

1. **Content-Security-Policy (CSP)**
   - Prevents XSS attacks by controlling resource loading
   - Use nonces for inline scripts/styles
   - Report violations for monitoring

2. **X-XSS-Protection**
   - Enables browser XSS filtering
   - `1; mode=block` blocks detected XSS attempts

3. **X-Content-Type-Options**
   - Prevents MIME type sniffing
   - `nosniff` forces declared content types

4. **X-Frame-Options**
   - Prevents clickjacking attacks
   - `DENY` blocks all framing, `SAMEORIGIN` allows same-origin framing

5. **Strict-Transport-Security (HSTS)**
   - Enforces HTTPS connections
   - `includeSubDomains` applies to all subdomains
   - `preload` allows inclusion in browser preload lists

6. **Referrer-Policy**
   - Controls referrer information sent with requests
   - `strict-origin-when-cross-origin` balances privacy and functionality

### Rate Limiting Strategies

1. **Endpoint-specific Limits**
   ```go
   // Authentication endpoints: 5 requests per minute
   authRateLimit := rate.Every(time.Minute / 5)
   
   // Search endpoints: 100 requests per minute  
   searchRateLimit := rate.Every(time.Minute / 100)
   
   // File upload: 10 requests per hour
   uploadRateLimit := rate.Every(time.Hour / 10)
   ```

2. **User-based Rate Limiting**
   ```go
   func userBasedRateLimit() gin.HandlerFunc {
       limiters := make(map[string]*rate.Limiter)
       mu := sync.RWMutex{}
       
       return func(c *gin.Context) {
           userID := getUserID(c) // Get from JWT/session
           
           mu.RLock()
           limiter, exists := limiters[userID]
           mu.RUnlock()
           
           if !exists {
               mu.Lock()
               limiter = rate.NewLimiter(rate.Every(time.Minute/60), 10)
               limiters[userID] = limiter
               mu.Unlock()
           }
           
           if !limiter.Allow() {
               c.AbortWithStatusJSON(429, gin.H{"error": "Rate limit exceeded"})
               return
           }
           
           c.Next()
       }
   }
   ```

### CSRF Best Practices

1. **SameSite Cookies**
   ```go
   sessionConfig.SameSite = int(http.SameSiteStrictMode)
   ```

2. **Double Submit Cookie Pattern**
   ```go
   func doubleSubmitCSRF() gin.HandlerFunc {
       return func(c *gin.Context) {
           if c.Request.Method != "GET" && c.Request.Method != "HEAD" {
               cookieToken, _ := c.Cookie("csrf-token")
               headerToken := c.GetHeader("X-CSRF-Token")
               
               if cookieToken == "" || cookieToken != headerToken {
                   c.AbortWithStatusJSON(403, gin.H{"error": "CSRF token mismatch"})
                   return
               }
           }
           c.Next()
       }
   }
   ```

## Integration Examples

### Security with Authentication

```go
// Complete secure setup
func setupSecureAPI() *gin.Engine {
    router := gin.Default()
    
    // Security headers
    router.Use(security.SecurityMiddleware(security.DefaultSecurityConfig()))
    
    // Rate limiting
    router.Use(security.RateLimitMiddleware(rate.Every(time.Second), 10))
    
    // Authentication
    jwtAuth := auth.NewAuthJWT(jwtProvider)
    
    // Public endpoints
    public := router.Group("/public")
    {
        public.POST("/login", loginHandler)
        public.GET("/health", healthHandler)
    }
    
    // Protected API with additional security
    api := router.Group("/api")
    api.Use(auth.AuthMiddleware(jwtAuth))
    api.Use(security.CSRFProtection())
    {
        api.GET("/user", getUserHandler)
        api.PUT("/user", updateUserHandler)
    }
    
    return router
}
```

### Security for Web Applications

```go
func setupSecureWebApp() *gin.Engine {
    router := gin.Default()
    
    // Security headers with CSP for web content
    securityConfig := security.DefaultSecurityConfig()
    securityConfig.CSP = "default-src 'self'; script-src 'self' 'nonce-{nonce}'; style-src 'self' 'nonce-{nonce}' 'unsafe-inline'"
    router.Use(security.SecurityMiddleware(securityConfig))
    
    // Session management
    router.Use(sessionManager.Middleware())
    
    // CSRF protection for forms
    router.Use(security.CSRFProtection())
    
    // Rate limiting
    router.Use(security.RateLimitMiddleware(rate.Every(time.Second/2), 5))
    
    // Routes
    router.GET("/", homeHandler)
    router.GET("/form", formHandler)
    router.POST("/submit", submitHandler)
    
    return router
}
```

The security middleware provides comprehensive protection against common web vulnerabilities while maintaining flexibility for different application types and requirements.