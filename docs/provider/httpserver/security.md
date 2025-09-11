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

## Mutual TLS (mTLS) Configuration

Blueprint HTTP server provides comprehensive support for mutual TLS authentication, where both client and server certificates are validated for secure API-to-API communication.

### Working mTLS Server Example

This example is based on the fully tested sample in `samples/httpserver-mtls`:

```go
package main

import (
    "context"
    "crypto/x509"
    "fmt"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/gin-gonic/gin"
    "github.com/oddbit-project/blueprint/log"
    "github.com/oddbit-project/blueprint/provider/httpserver"
    "github.com/oddbit-project/blueprint/provider/httpserver/response"
    tlsProvider "github.com/oddbit-project/blueprint/provider/tls"
)

func main() {
    // Setup logger
    logger := log.New("mtls-server")
    logger.Info("Starting mTLS server demo...")

    // Configure mTLS server
    serverConfig := &httpserver.ServerConfig{
        Host: "localhost",
        Port: 8444,
        ServerConfig: tlsProvider.ServerConfig{
            TLSEnable: true,
            // Server certificate and key
            TLSCert: "../certs/server.crt",
            TLSKey:  "../certs/server.key",
            // CA certificates to validate client certificates
            TLSAllowedCACerts: []string{
                "../certs/ca.crt",
            },
            // Optional: Restrict allowed client DNS names (commented out for demo)
            // TLSAllowedDNSNames: []string{
            //	"demo-client.example.com",
            //	"client.blueprint.demo",
            // },
            // Security settings - use TLS 1.3 for maximum security
            TLSMinVersion: "TLS13",
            TLSMaxVersion: "TLS13",
            // Use strong cipher suites
            TLSCipherSuites: []string{
                "TLS_AES_256_GCM_SHA384",
                "TLS_CHACHA20_POLY1305_SHA256",
                "TLS_AES_128_GCM_SHA256",
            },
        },
    }

    // Create server with mTLS configuration
    server, err := serverConfig.NewServer(logger)
    if err != nil {
        logger.Fatal(err, "Failed to create server")
    }

    // Setup routes
    setupRoutes(server, logger)

    // Setup graceful shutdown
    setupGracefulShutdown(server, logger)

    // Start server
    logger.Info("mTLS server starting", log.KV{
        "host": serverConfig.Host,
        "port": serverConfig.Port,
        "tls":  serverConfig.TLSEnable,
    })

    if err := server.Start(); err != nil {
        logger.Fatal(err, "Server failed to start")
    }
}

func setupRoutes(server *httpserver.Server, logger *log.Logger) {
    // Add mTLS security logger middleware
    server.AddMiddleware(mTLSSecurityLogger(logger))

    // Public endpoint (no client certificate validation)
    server.Route().GET("/health", func(c *gin.Context) {
        response.Success(c, gin.H{
            "status":    "healthy",
            "timestamp": time.Now().Format(time.RFC3339),
            "server":    "mTLS Demo Server",
        })
    })

    // Protected endpoint requiring client certificate
    server.Route().GET("/secure", mTLSAuthorizationMiddleware(logger), func(c *gin.Context) {
        // Get client certificate from context
        clientCert, exists := c.Get("client_cert")
        if !exists {
            c.JSON(500, gin.H{"error": "Internal error: client certificate not found in context"})
            return
        }

        cert := clientCert.(*x509.Certificate)
        clientInfo := extractClientInfo(cert)

        response.Success(c, gin.H{
            "message":     "Access granted to secure endpoint",
            "client_info": clientInfo,
            "timestamp":   time.Now().Format(time.RFC3339),
        })
    })

    // API endpoints with different authorization levels
    api := server.Group("/api/v1")
    api.Use(mTLSAuthorizationMiddleware(logger))
    {
        api.GET("/user/profile", func(c *gin.Context) {
            clientDN, _ := c.Get("client_dn")
            response.Success(c, gin.H{
                "user_id":    "demo_user_123",
                "username":   "demo_user",
                "email":      "demo@example.com",
                "client_dn":  clientDN,
                "privileges": []string{"read", "write"},
            })
        })

        api.POST("/data", func(c *gin.Context) {
            var requestData map[string]interface{}
            if err := c.ShouldBindJSON(&requestData); err != nil {
                c.JSON(400, gin.H{"error": "Invalid JSON payload"})
                return
            }

            clientDN, _ := c.Get("client_dn")
            response.Success(c, gin.H{
                "message":   "Data processed successfully",
                "data_id":   fmt.Sprintf("data_%d", time.Now().Unix()),
                "client_dn": clientDN,
                "received":  requestData,
            })
        })

        api.GET("/admin/stats", func(c *gin.Context) {
            // Only allow specific client organizations for admin endpoints
            clientCert, _ := c.Get("client_cert")
            cert := clientCert.(*x509.Certificate)

            if !isAdminClient(cert) {
                c.JSON(403, gin.H{"error": "Admin access required"})
                return
            }

            response.Success(c, gin.H{
                "active_connections": 42,
                "uptime_seconds":     3600,
                "memory_usage_mb":    128,
                "admin_client":       cert.Subject.String(),
            })
        })
    }
}

func mTLSSecurityLogger(logger *log.Logger) gin.HandlerFunc {
    return func(c *gin.Context) {
        start := time.Now()
        c.Next()
        duration := time.Since(start)

        if c.Request.TLS != nil && len(c.Request.TLS.PeerCertificates) > 0 {
            clientCert := c.Request.TLS.PeerCertificates[0]

            logger.Info("mTLS request", log.KV{
                "client_dn":      clientCert.Subject.String(),
                "client_serial":  clientCert.SerialNumber.String(),
                "path":           c.Request.URL.Path,
                "method":         c.Request.Method,
                "status":         c.Writer.Status(),
                "client_ip":      c.ClientIP(),
                "duration_ms":    duration.Milliseconds(),
                "tls_version":    getTLSVersion(c.Request.TLS.Version),
                "cipher_suite":   getCipherSuite(c.Request.TLS.CipherSuite),
                "user_agent":     c.Request.UserAgent(),
            })
        } else {
            logger.Info("Non-mTLS request", log.KV{
                "path":        c.Request.URL.Path,
                "method":      c.Request.Method,
                "status":      c.Writer.Status(),
                "client_ip":   c.ClientIP(),
                "duration_ms": duration.Milliseconds(),
                "user_agent":  c.Request.UserAgent(),
            })
        }
    }
}

func mTLSAuthorizationMiddleware(logger *log.Logger) gin.HandlerFunc {
    return func(c *gin.Context) {
        if c.Request.TLS == nil || len(c.Request.TLS.PeerCertificates) == 0 {
            logger.Warn("Client certificate required but not provided", log.KV{
                "path":      c.Request.URL.Path,
                "client_ip": c.ClientIP(),
            })
            c.AbortWithStatusJSON(401, gin.H{"error": "Client certificate required for this endpoint"})
            return
        }

        clientCert := c.Request.TLS.PeerCertificates[0]

        // Validate certificate is still valid
        now := time.Now()
        if now.Before(clientCert.NotBefore) || now.After(clientCert.NotAfter) {
            logger.Warn("Client certificate expired or not yet valid", log.KV{
                "client_dn":  clientCert.Subject.String(),
                "not_before": clientCert.NotBefore,
                "not_after":  clientCert.NotAfter,
                "now":        now,
            })
            c.AbortWithStatusJSON(401, gin.H{"error": "Client certificate expired or not yet valid"})
            return
        }

        // Custom authorization logic based on certificate attributes
        if !isAuthorizedClient(clientCert) {
            logger.Warn("Client certificate not authorized", log.KV{
                "client_dn":     clientCert.Subject.String(),
                "client_serial": clientCert.SerialNumber.String(),
                "organizations": clientCert.Subject.Organization,
            })
            c.AbortWithStatusJSON(403, gin.H{"error": "Client certificate not authorized"})
            return
        }

        // Store client identity in context for downstream handlers
        c.Set("client_cert", clientCert)
        c.Set("client_dn", clientCert.Subject.String())
        c.Set("client_serial", clientCert.SerialNumber.String())

        logger.Debug("mTLS client authorized", log.KV{
            "client_dn":     clientCert.Subject.String(),
            "client_serial": clientCert.SerialNumber.String(),
            "path":          c.Request.URL.Path,
        })

        c.Next()
    }
}

func isAuthorizedClient(cert *x509.Certificate) bool {
    // Allow clients from specific organizations
    authorizedOrgs := []string{"Blueprint Demo"}

    for _, org := range cert.Subject.Organization {
        for _, authorizedOrg := range authorizedOrgs {
            if org == authorizedOrg {
                return true
            }
        }
    }
    return false
}

func isAdminClient(cert *x509.Certificate) bool {
    // Admin access requires specific OU
    for _, ou := range cert.Subject.OrganizationalUnit {
        if ou == "Admin" || ou == "Client" { // For demo purposes, allow Client OU as admin
            return true
        }
    }
    return false
}

func extractClientInfo(cert *x509.Certificate) map[string]interface{} {
    return map[string]interface{}{
        "subject":      cert.Subject.String(),
        "issuer":       cert.Issuer.String(),
        "serial":       cert.SerialNumber.String(),
        "not_before":   cert.NotBefore.Format(time.RFC3339),
        "not_after":    cert.NotAfter.Format(time.RFC3339),
        "dns_names":    cert.DNSNames,
        "ip_addresses": cert.IPAddresses,
        "organizations": cert.Subject.Organization,
        "organizational_units": cert.Subject.OrganizationalUnit,
        "common_name":  cert.Subject.CommonName,
    }
}

func getTLSVersion(version uint16) string {
    switch version {
    case 0x0303:
        return "TLS 1.2"
    case 0x0304:
        return "TLS 1.3"
    default:
        return fmt.Sprintf("Unknown (0x%04x)", version)
    }
}

func getCipherSuite(suite uint16) string {
    switch suite {
    case 0x1301:
        return "TLS_AES_128_GCM_SHA256"
    case 0x1302:
        return "TLS_AES_256_GCM_SHA384"
    case 0x1303:
        return "TLS_CHACHA20_POLY1305_SHA256"
    default:
        return fmt.Sprintf("Unknown (0x%04x)", suite)
    }
}

func setupGracefulShutdown(server *httpserver.Server, logger *log.Logger) {
    c := make(chan os.Signal, 1)
    signal.Notify(c, os.Interrupt, syscall.SIGTERM)

    go func() {
        <-c
        logger.Info("Shutting down mTLS server...")

        ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
        defer cancel()

        if err := server.Shutdown(ctx); err != nil {
            logger.Error(err, "Error during server shutdown")
        } else {
            logger.Info("mTLS server shutdown complete")
        }
        os.Exit(0)
    }()
}
```

### mTLS Configuration Options

**Important configuration notes:**
- TLS versions must use `"TLS12"` or `"TLS13"` (not `"1.2"` or `"1.3"`)
- When `TLSAllowedCACerts` is provided, client certificates are required
- Use `TLSAllowedDNSNames` to restrict which client DNS names are allowed

```go
serverConfig := tlsProvider.ServerConfig{
    TLSEnable: true,
    
    // Server identity certificate
    TLSCert: "server.crt",
    TLSKey:  "server.key",
    
    // Client certificate validation
    TLSAllowedCACerts: []string{
        "ca.crt",  // CA certificate for client validation
    },
    
    // Optional client certificate restrictions
    TLSAllowedDNSNames: []string{
        "demo-client.example.com",
        "client.blueprint.demo",
    },
    
    // TLS version control (use TLS12 or TLS13)
    TLSMinVersion: "TLS13",
    TLSMaxVersion: "TLS13",
    
    // Cipher suite restrictions
    TLSCipherSuites: []string{
        "TLS_AES_256_GCM_SHA384",
        "TLS_CHACHA20_POLY1305_SHA256",
        "TLS_AES_128_GCM_SHA256",
    },
}
```

### Client Certificate Authentication

Access client certificate information in request handlers:

```go
func protectedHandler(c *gin.Context) {
    // Access client certificate information
    if c.Request.TLS != nil && len(c.Request.TLS.PeerCertificates) > 0 {
        clientCert := c.Request.TLS.PeerCertificates[0]
        
        // Extract client identity
        clientDN := clientCert.Subject.String()
        clientSerial := clientCert.SerialNumber.String()
        
        // Use client identity for authorization
        if isAuthorizedClient(clientCert) {
            response.Success(c, gin.H{
                "message": "Access granted", 
                "client_dn": clientDN,
                "client_info": extractClientInfo(clientCert),
            })
        } else {
            c.JSON(403, gin.H{"error": "Client not authorized"})
        }
        return
    }
    
    c.JSON(401, gin.H{"error": "Client certificate required"})
}
```

### Certificate Generation for mTLS

**Working certificate generation script (from `samples/httpserver-mtls/generate-certs.sh`):**

```bash
#!/bin/bash
set -e

CERT_DIR="certs"
mkdir -p "$CERT_DIR"
cd "$CERT_DIR"

# 1. Create CA private key
openssl genrsa -out ca.key 4096

# 2. Create CA certificate
openssl req -new -x509 -days 3650 -key ca.key -out ca.crt \
    -subj "/C=US/ST=CA/L=San Francisco/O=Blueprint Demo/OU=Security/CN=Blueprint Demo CA"

# 3. Create server private key
openssl genrsa -out server.key 4096

# 4. Create server certificate signing request
openssl req -new -key server.key -out server.csr \
    -subj "/C=US/ST=CA/L=San Francisco/O=Blueprint Demo/OU=Server/CN=localhost"

# 5. Create server certificate extensions file
cat > server.ext << EOF
authorityKeyIdentifier=keyid,issuer
basicConstraints=CA:FALSE
keyUsage = digitalSignature, nonRepudiation, keyEncipherment, dataEncipherment
subjectAltName = @alt_names

[alt_names]
DNS.1 = localhost
DNS.2 = api.example.com
IP.1 = 127.0.0.1
IP.2 = ::1
EOF

# 6. Sign server certificate with CA
openssl x509 -req -days 365 -in server.csr -CA ca.crt -CAkey ca.key \
    -CAcreateserial -out server.crt -extfile server.ext

# 7. Create client private key
openssl genrsa -out client.key 4096

# 8. Create client certificate signing request
openssl req -new -key client.key -out client.csr \
    -subj "/C=US/ST=CA/L=San Francisco/O=Blueprint Demo/OU=Client/CN=demo-client.example.com"

# 9. Create client certificate extensions file
cat > client.ext << EOF
authorityKeyIdentifier=keyid,issuer
basicConstraints=CA:FALSE
keyUsage = digitalSignature, nonRepudiation, keyEncipherment, dataEncipherment
extendedKeyUsage = clientAuth
subjectAltName = @alt_names

[alt_names]
DNS.1 = demo-client.example.com
DNS.2 = client.blueprint.demo
EOF

# 10. Sign client certificate with CA
openssl x509 -req -days 365 -in client.csr -CA ca.crt -CAkey ca.key \
    -CAcreateserial -out client.crt -extfile client.ext

# 11. Cleanup
rm -f *.csr *.ext ca.srl

echo "✅ Certificate generation complete!"
```

This generates:
- `ca.crt` - CA certificate for validation
- `ca.key` - CA private key  
- `server.crt` - Server certificate (localhost, 127.0.0.1)
- `server.key` - Server private key
- `client.crt` - Client certificate with proper extensions
- `client.key` - Client private key

### mTLS Security Features

**Automatic Certificate Validation:**
- Certificate expiration checking
- Certificate chain verification
- DNS name validation (if configured)  
- CA signature validation
- Organization-based authorization

### Working mTLS Client Example

**Blueprint-style mTLS client (from `samples/httpserver-mtls/client/main.go`):**

```go
package main

import (
    "bytes"
    "crypto/tls"
    "crypto/x509"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "os"
    "strings"
    "time"

    "github.com/oddbit-project/blueprint/log"
)

// ClientConfig holds the mTLS client configuration
type ClientConfig struct {
    ServerURL  string
    CACert     string
    ClientCert string
    ClientKey  string
    Timeout    time.Duration
}

// MTLSClient represents an mTLS-enabled HTTP client following Blueprint patterns
type MTLSClient struct {
    baseURL    string
    httpClient *http.Client
    logger     *log.Logger
    config     *ClientConfig
}

// NewMTLSClient creates a new mTLS client with proper configuration
func NewMTLSClient(config *ClientConfig, logger *log.Logger) (*MTLSClient, error) {
    if config == nil {
        return nil, fmt.Errorf("client config is required")
    }
    if logger == nil {
        logger = log.New("mtls-client")
    }

    // Set default timeout if not specified
    if config.Timeout == 0 {
        config.Timeout = 30 * time.Second
    }

    // Load client certificate
    clientCert, err := tls.LoadX509KeyPair(config.ClientCert, config.ClientKey)
    if err != nil {
        return nil, fmt.Errorf("failed to load client certificate: %w", err)
    }

    // Load CA certificate
    caCertPEM, err := os.ReadFile(config.CACert)
    if err != nil {
        return nil, fmt.Errorf("failed to read CA certificate: %w", err)
    }

    caCertPool := x509.NewCertPool()
    if !caCertPool.AppendCertsFromPEM(caCertPEM) {
        return nil, fmt.Errorf("failed to parse CA certificate")
    }

    // Configure TLS with mTLS
    tlsConfig := &tls.Config{
        Certificates: []tls.Certificate{clientCert},
        RootCAs:      caCertPool,
        MinVersion:   tls.VersionTLS13, // Use TLS 1.3
        CipherSuites: []uint16{
            tls.TLS_AES_256_GCM_SHA384,
            tls.TLS_CHACHA20_POLY1305_SHA256,
            tls.TLS_AES_128_GCM_SHA256,
        },
    }

    // Create HTTP client with mTLS configuration
    httpClient := &http.Client{
        Transport: &http.Transport{
            TLSClientConfig: tlsConfig,
        },
        Timeout: config.Timeout,
    }

    return &MTLSClient{
        baseURL:    strings.TrimSuffix(config.ServerURL, "/"),
        httpClient: httpClient,
        logger:     logger,
        config:     config,
    }, nil
}

// makeRequest creates and sends an mTLS HTTP request with proper logging
func (c *MTLSClient) makeRequest(method, path string, body interface{}) (*http.Response, error) {
    // Prepare request body
    var requestBody []byte
    var err error

    if body != nil {
        requestBody, err = json.Marshal(body)
        if err != nil {
            return nil, fmt.Errorf("failed to marshal request body: %w", err)
        }
    }

    // Create HTTP request
    url := c.baseURL + path
    req, err := http.NewRequest(method, url, bytes.NewReader(requestBody))
    if err != nil {
        return nil, fmt.Errorf("failed to create request: %w", err)
    }

    // Set standard headers
    req.Header.Set("User-Agent", "Blueprint-mTLS-Client/1.0")
    req.Header.Set("Accept", "application/json")
    if requestBody != nil {
        req.Header.Set("Content-Type", "application/json")
    }

    // Log request details
    c.logger.Debug("Making mTLS request", log.KV{
        "method": method,
        "url":    url,
        "path":   path,
    })

    start := time.Now()

    // Make the request
    resp, err := c.httpClient.Do(req)
    if err != nil {
        c.logger.Error(err, "mTLS request failed", log.KV{
            "method":      method,
            "url":         url,
            "duration_ms": time.Since(start).Milliseconds(),
        })
        return nil, fmt.Errorf("request failed: %w", err)
    }

    // Log response details
    c.logger.Info("mTLS request completed", log.KV{
        "method":      method,
        "url":         url,
        "status":      resp.StatusCode,
        "duration_ms": time.Since(start).Milliseconds(),
    })

    return resp, nil
}

// Get performs an HTTP GET request
func (c *MTLSClient) Get(path string) (*http.Response, error) {
    return c.makeRequest("GET", path, nil)
}

// Post performs an HTTP POST request with JSON body
func (c *MTLSClient) Post(path string, body interface{}) (*http.Response, error) {
    return c.makeRequest("POST", path, body)
}

// GetJSON performs a GET request and unmarshals the JSON response
func (c *MTLSClient) GetJSON(path string, result interface{}) error {
    resp, err := c.Get(path)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    if resp.StatusCode < 200 || resp.StatusCode >= 300 {
        body, _ := io.ReadAll(resp.Body)
        return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
    }

    return json.NewDecoder(resp.Body).Decode(result)
}

// PostJSON performs a POST request and unmarshals the JSON response
func (c *MTLSClient) PostJSON(path string, requestBody interface{}, result interface{}) error {
    resp, err := c.Post(path, requestBody)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    if resp.StatusCode < 200 || resp.StatusCode >= 300 {
        body, _ := io.ReadAll(resp.Body)
        return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
    }

    return json.NewDecoder(resp.Body).Decode(result)
}

func main() {
    // Setup logger
    logger := log.New("mtls-client")
    logger.Info("Starting Blueprint mTLS client demo...")

    // Create client configuration
    config := &ClientConfig{
        ServerURL:  "https://localhost:8444",
        CACert:     "../certs/ca.crt",
        ClientCert: "../certs/client.crt",
        ClientKey:  "../certs/client.key",
        Timeout:    30 * time.Second,
    }

    // Create mTLS client
    client, err := NewMTLSClient(config, logger)
    if err != nil {
        logger.Fatal(err, "Failed to create mTLS client")
    }

    // Test the secure endpoint
    type SecureResponse struct {
        Success bool `json:"success"`
        Data    struct {
            Message     string                 `json:"message"`
            ClientInfo  map[string]interface{} `json:"client_info"`
            Timestamp   string                 `json:"timestamp"`
        } `json:"data"`
    }

    var secureResp SecureResponse
    if err := client.GetJSON("/secure", &secureResp); err != nil {
        logger.Error(err, "Secure endpoint failed")
        return
    }

    logger.Info("mTLS authentication successful", log.KV{
        "message": secureResp.Data.Message,
        "client":  secureResp.Data.ClientInfo["common_name"],
    })
}
```

### Testing with curl

You can test the mTLS server with curl:

```bash
# Health check (no client cert required)
curl -k https://localhost:8444/health

# Secure endpoint with mTLS
curl -k \
  --cert certs/client.crt \
  --key certs/client.key \
  --cacert certs/ca.crt \
  https://localhost:8444/secure

# API endpoint with JSON data
curl -k \
  --cert certs/client.crt \
  --key certs/client.key \
  --cacert certs/ca.crt \
  -H "Content-Type: application/json" \
  -d '{"message": "Hello from curl!"}' \
  https://localhost:8444/api/v1/data
```

### Complete Working Demo

For a complete working example, see `samples/httpserver-mtls/` which includes:

- **Certificate generation script** - Creates CA, server, and client certificates
- **mTLS server** - Full server implementation with authorization middleware
- **mTLS client** - Go client demonstrating mTLS authentication
- **Test script** - Automated testing of the complete demo

Run the demo:

```bash
cd samples/httpserver-mtls
./generate-certs.sh    # Generate certificates
./test-demo.sh         # Run complete test
```

Expected output:
```
🚀 Blueprint mTLS Client Demo
==============================

1. Testing health endpoint (no client cert required)...
   ✅ Health check passed
   Server: mTLS Demo Server, Status: healthy

2. Testing secure endpoint (client cert required)...
   ✅ mTLS authentication successful
   Message: Access granted to secure endpoint
   Client: demo-client.example.com

3. Testing user profile API...
   ✅ User profile retrieved successfully
   User: demo_user (demo@example.com)
   Privileges: [read write]

4. Testing data submission API...
   ✅ Data submitted successfully
   Data ID: data_1737722400

5. Testing admin stats API...
   ✅ Admin stats retrieved successfully
   Active connections: 42, Memory: 128 MB

✅ Blueprint mTLS Client Demo completed!
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