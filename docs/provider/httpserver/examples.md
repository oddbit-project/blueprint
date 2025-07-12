# HTTP Server Integration Examples

Comprehensive examples showing how to combine Blueprint's HTTP server components for common use cases including 
REST APIs, web applications, and microservices.

## Example 1: REST API Server

Complete REST API with JWT authentication, rate limiting, and security headers.

```go
package main

import (
    "context"
    "os"
    "os/signal"
    "syscall"
    "time"
    
    "github.com/gin-gonic/gin"
    "github.com/oddbit-project/blueprint/provider/httpserver"
    "github.com/oddbit-project/blueprint/provider/httpserver/auth"
    "github.com/oddbit-project/blueprint/provider/httpserver/response"
    "github.com/oddbit-project/blueprint/provider/jwtprovider"
    "github.com/oddbit-project/blueprint/crypt/secure"
    "github.com/oddbit-project/blueprint/log"
)

type User struct {
    ID    int    `json:"id"`
    Name  string `json:"name"`
    Email string `json:"email"`
}

func main() {
    logger := log.New("rest-api")
    
    // Server configuration
    config := httpserver.NewServerConfig()
    config.Port = 8080
    config.Debug = false
    
    // Create server
    server, err := httpserver.NewServer(config, logger)
    if err != nil {
        logger.Fatal(err, "failed to create server")
    }
    
    // Setup middleware and routes
    setupRESTAPIMiddleware(server, logger)
    setupRESTAPIRoutes(server)
    
    // Start with graceful shutdown
    startWithGracefulShutdown(server, logger)
}

func setupRESTAPIMiddleware(server *httpserver.Server, logger *log.Logger) {
    // 1. Security headers
    server.UseDefaultSecurityHeaders()
    
    // 2. Rate limiting - 100 requests per minute
    server.UseRateLimiting(100)
    
    // 3. Request ID middleware
    server.AddMiddleware(func(c *gin.Context) {
        requestID := generateRequestID()
        c.Set("request_id", requestID)
        c.Header("X-Request-ID", requestID)
        c.Next()
    })
}

func setupRESTAPIRoutes(server *httpserver.Server) {
    router := server.Route()
    
    // Health check endpoint
    router.GET("/health", func(c *gin.Context) {
        c.JSON(200, response.JSONResponse{
            Success: true,
            Data:    gin.H{"status": "healthy", "timestamp": time.Now()},
        })
    })
    
    // Authentication endpoint
    router.POST("/auth/login", loginHandler)
    
    // Protected API routes
    jwtProvider := setupJWTProvider()
    jwtAuth := auth.NewAuthJWT(jwtProvider)
    
    api := server.Group("/api/v1")
    api.Use(auth.AuthMiddleware(jwtAuth))
    {
        // User endpoints
        api.GET("/users", listUsersHandler)
        api.GET("/users/:id", getUserHandler)
        api.POST("/users", createUserHandler)
        api.PUT("/users/:id", updateUserHandler)
        api.DELETE("/users/:id", deleteUserHandler)
        
        // Profile endpoints
        api.GET("/profile", getProfileHandler)
        api.PUT("/profile", updateProfileHandler)
    }
}

func setupJWTProvider() jwtprovider.JWTParser {
    config := jwtprovider.NewJWTConfig()
    config.SigningAlgorithm = jwtprovider.HS256
    config.CfgSigningKey = &secure.DefaultCredentialConfig{
        Password: "your-jwt-secret-key-here",
    }
    config.Issuer = "rest-api"
    config.Audience = "api-users"
    config.ExpirationSeconds = 3600 // 1 hour
    
    provider, err := jwtprovider.NewProvider(config)
    if err != nil {
        panic(err)
    }
    
    return provider
}

// Authentication handler
func loginHandler(c *gin.Context) {
    var loginRequest struct {
        Email    string `json:"email" binding:"required,email"`
        Password string `json:"password" binding:"required"`
    }
    
    if err := c.ShouldBindJSON(&loginRequest); err != nil {
        response.ValidationError(c, err)
        return
    }
    
    // Validate credentials (implement your logic)
    user, err := authenticateUser(loginRequest.Email, loginRequest.Password)
    if err != nil {
        response.Http401(c)
        return
    }
    
    // Generate JWT token
    token, err := generateJWTToken(user)
    if err != nil {
        response.Http500(c, err)
        return
    }
    
    c.JSON(200, response.JSONResponse{
        Success: true,
        Data: gin.H{
            "token": token,
            "user":  user,
        },
    })
}

// User CRUD handlers
func listUsersHandler(c *gin.Context) {
    users := []User{
        {ID: 1, Name: "John Doe", Email: "john@example.com"},
        {ID: 2, Name: "Jane Smith", Email: "jane@example.com"},
    }
    
    c.JSON(200, response.JSONResponse{
        Success: true,
        Data:    users,
    })
}

func getUserHandler(c *gin.Context) {
    userID := c.Param("id")
    
    // Get user from database (implement your logic)
    user, err := getUserByID(userID)
    if err != nil {
        response.Http404(c)
        return
    }
    
    c.JSON(200, response.JSONResponse{
        Success: true,
        Data:    user,
    })
}

func createUserHandler(c *gin.Context) {
    var newUser User
    if err := c.ShouldBindJSON(&newUser); err != nil {
        response.ValidationError(c, err)
        return
    }
    
    // Create user in database (implement your logic)
    createdUser, err := createUser(newUser)
    if err != nil {
        response.Http500(c, err)
        return
    }
    
    c.JSON(201, response.JSONResponse{
        Success: true,
        Data:    createdUser,
    })
}

func updateUserHandler(c *gin.Context) {
    userID := c.Param("id")
    var updateData User
    
    if err := c.ShouldBindJSON(&updateData); err != nil {
        response.ValidationError(c, err)
        return
    }
    
    // Update user in database (implement your logic)
    updatedUser, err := updateUser(userID, updateData)
    if err != nil {
        response.Http500(c, err)
        return
    }
    
    c.JSON(200, response.JSONResponse{
        Success: true,
        Data:    updatedUser,
    })
}

func deleteUserHandler(c *gin.Context) {
    userID := c.Param("id")
    
    // Delete user from database (implement your logic)
    if err := deleteUser(userID); err != nil {
        response.Http500(c, err)
        return
    }
    
    c.JSON(200, response.JSONResponse{
        Success: true,
        Data:    gin.H{"deleted": true},
    })
}

func getProfileHandler(c *gin.Context) {
    // Get JWT claims from context
    claimsValue, exists := c.Get(auth.ContextJwtClaims)
    if !exists {
        response.Http401(c)
        return
    }
    
    claims, ok := claimsValue.(*jwtprovider.Claims)
    if !ok {
        response.Http401(c)
        return
    }
    
    // Get user profile
    user, err := getUserByID(claims.Subject)
    if err != nil {
        response.Http404(c)
        return
    }
    
    c.JSON(200, response.JSONResponse{
        Success: true,
        Data:    user,
    })
}

func updateProfileHandler(c *gin.Context) {
    // Implementation similar to updateUserHandler but using JWT claims for user ID
    // ... implementation here
}

// Helper functions (implement according to your needs)
func generateRequestID() string {
    // Generate unique request ID
    return "req-" + time.Now().Format("20060102150405")
}

func authenticateUser(email, password string) (*User, error) {
    // Implement user authentication logic
    return &User{ID: 1, Name: "Test User", Email: email}, nil
}

func generateJWTToken(user *User) (string, error) {
    // Implement JWT token generation
    return "jwt-token-here", nil
}

func getUserByID(id string) (*User, error) {
    // Implement user lookup
    return &User{ID: 1, Name: "Test User", Email: "test@example.com"}, nil
}

func createUser(user User) (*User, error) {
    // Implement user creation
    user.ID = 123
    return &user, nil
}

func updateUser(id string, user User) (*User, error) {
    // Implement user update
    return &user, nil
}

func deleteUser(id string) error {
    // Implement user deletion
    return nil
}

func startWithGracefulShutdown(server *httpserver.Server, logger *log.Logger) {
    // Start server in goroutine
    go func() {
        logger.Info("starting REST API server", "port", server.Config.Port)
        if err := server.Start(); err != nil {
            logger.Error(err, "server failed")
        }
    }()
    
    // Wait for interrupt signal
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit
    
    logger.Info("shutting down server...")
    
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    
    if err := server.Shutdown(ctx); err != nil {
        logger.Error(err, "forced shutdown")
    }
    
    logger.Info("server stopped")
}
```

## Example 2: Web Application with Sessions

Complete web application with session management, CSRF protection, and form handling.

```go
package main

import (
    "net/http"
    "github.com/gin-gonic/gin"
    "github.com/oddbit-project/blueprint/provider/httpserver"
    "github.com/oddbit-project/blueprint/provider/httpserver/security"
    "github.com/oddbit-project/blueprint/provider/httpserver/session"
    "github.com/oddbit-project/blueprint/provider/kv"
    "github.com/oddbit-project/blueprint/log"
)

func main() {
    logger := log.New("web-app")
    
    // Server configuration
    config := httpserver.NewServerConfig()
    config.Port = 8080
    config.Debug = true // Enable for template development
    
    server, err := httpserver.NewServer(config, logger)
    if err != nil {
        logger.Fatal(err, "failed to create server")
    }
    
    // Setup web application
    setupWebAppMiddleware(server, logger)
    setupWebAppRoutes(server)
    
    // Load HTML templates
    server.Route().LoadHTMLGlob("templates/*")
    
    // Serve static files
    server.Route().Static("/static", "./static")
    
    logger.Info("starting web application", "port", config.Port)
    if err := server.Start(); err != nil {
        logger.Fatal(err, "server failed")
    }
}

func setupWebAppMiddleware(server *httpserver.Server, logger *log.Logger) {
    // 1. Security headers for web content
    securityConfig := security.DefaultSecurityConfig()
    securityConfig.CSP = "default-src 'self'; script-src 'self' 'nonce-{nonce}'; style-src 'self' 'nonce-{nonce}' 'unsafe-inline'"
    server.UseSecurityHeaders(securityConfig)
    
    // 2. Rate limiting
    server.UseRateLimiting(60)
    
    // 3. Session management
    backend := kv.NewMemoryKV()
    sessionConfig := session.NewConfig()
    sessionConfig.Secure = false // For development over HTTP
    sessionManager := server.UseSession(sessionConfig, backend, logger)
    
    // 4. CSRF protection
    server.UseCSRFProtection()
    
    // 5. Flash message middleware
    server.AddMiddleware(flashMiddleware())
}

func setupWebAppRoutes(server *httpserver.Server) {
    router := server.Route()
    
    // Home page
    router.GET("/", homeHandler)
    
    // Authentication routes
    router.GET("/login", loginFormHandler)
    router.POST("/login", loginPostHandler)
    router.POST("/logout", logoutHandler)
    
    // User registration
    router.GET("/register", registerFormHandler)
    router.POST("/register", registerPostHandler)
    
    // Protected user area
    protected := server.Group("/dashboard")
    protected.Use(authRequiredMiddleware())
    {
        protected.GET("/", dashboardHandler)
        protected.GET("/profile", profileHandler)
        protected.POST("/profile", updateProfileHandler)
    }
}

// Page handlers
func homeHandler(c *gin.Context) {
    sess := session.Get(c)
    isLoggedIn := sess.Has("user_id")
    
    c.HTML(http.StatusOK, "home.html", gin.H{
        "title":      "Welcome",
        "loggedIn":   isLoggedIn,
        "user":       sess.Get("user_name"),
        "flashMsg":   getFlashMessage(c),
        "csrfToken":  security.GenerateCSRFToken(c),
    })
}

func loginFormHandler(c *gin.Context) {
    sess := session.Get(c)
    if sess.Has("user_id") {
        c.Redirect(http.StatusFound, "/dashboard")
        return
    }
    
    c.HTML(http.StatusOK, "login.html", gin.H{
        "title":     "Login",
        "csrfToken": security.GenerateCSRFToken(c),
        "flashMsg":  getFlashMessage(c),
    })
}

func loginPostHandler(c *gin.Context) {
    var loginForm struct {
        Email    string `form:"email" binding:"required,email"`
        Password string `form:"password" binding:"required"`
    }
    
    if err := c.ShouldBind(&loginForm); err != nil {
        setFlashMessage(c, "error", "Please provide valid email and password")
        c.Redirect(http.StatusFound, "/login")
        return
    }
    
    // Authenticate user (implement your logic)
    user, err := authenticateWebUser(loginForm.Email, loginForm.Password)
    if err != nil {
        setFlashMessage(c, "error", "Invalid email or password")
        c.Redirect(http.StatusFound, "/login")
        return
    }
    
    // Create session
    sess := session.Get(c)
    sess.Set("user_id", user.ID)
    sess.Set("user_name", user.Name)
    sess.Set("user_email", user.Email)
    
    // Regenerate session ID for security
    if manager, exists := c.Get("session_manager"); exists {
        if sessionManager, ok := manager.(*session.SessionManager); ok {
            sessionManager.Regenerate(c)
        }
    }
    
    setFlashMessage(c, "success", "Welcome back, "+user.Name+"!")
    c.Redirect(http.StatusFound, "/dashboard")
}

func logoutHandler(c *gin.Context) {
    // Clear session
    if manager, exists := c.Get("session_manager"); exists {
        if sessionManager, ok := manager.(*session.SessionManager); ok {
            sessionManager.Clear(c)
        }
    }
    
    setFlashMessage(c, "info", "You have been logged out")
    c.Redirect(http.StatusFound, "/")
}

func registerFormHandler(c *gin.Context) {
    c.HTML(http.StatusOK, "register.html", gin.H{
        "title":     "Register",
        "csrfToken": security.GenerateCSRFToken(c),
        "flashMsg":  getFlashMessage(c),
    })
}

func registerPostHandler(c *gin.Context) {
    var registerForm struct {
        Name            string `form:"name" binding:"required"`
        Email           string `form:"email" binding:"required,email"`
        Password        string `form:"password" binding:"required,min=6"`
        ConfirmPassword string `form:"confirm_password" binding:"required"`
    }
    
    if err := c.ShouldBind(&registerForm); err != nil {
        setFlashMessage(c, "error", "Please check your input")
        c.Redirect(http.StatusFound, "/register")
        return
    }
    
    if registerForm.Password != registerForm.ConfirmPassword {
        setFlashMessage(c, "error", "Passwords do not match")
        c.Redirect(http.StatusFound, "/register")
        return
    }
    
    // Create user (implement your logic)
    user, err := createWebUser(registerForm.Name, registerForm.Email, registerForm.Password)
    if err != nil {
        setFlashMessage(c, "error", "Registration failed: "+err.Error())
        c.Redirect(http.StatusFound, "/register")
        return
    }
    
    setFlashMessage(c, "success", "Registration successful! Please log in.")
    c.Redirect(http.StatusFound, "/login")
}

func dashboardHandler(c *gin.Context) {
    sess := session.Get(c)
    
    c.HTML(http.StatusOK, "dashboard.html", gin.H{
        "title":     "Dashboard",
        "user":      sess.Get("user_name"),
        "email":     sess.Get("user_email"),
        "flashMsg":  getFlashMessage(c),
        "csrfToken": security.GenerateCSRFToken(c),
    })
}

func profileHandler(c *gin.Context) {
    sess := session.Get(c)
    
    c.HTML(http.StatusOK, "profile.html", gin.H{
        "title":     "Profile",
        "user":      sess.Get("user_name"),
        "email":     sess.Get("user_email"),
        "flashMsg":  getFlashMessage(c),
        "csrfToken": security.GenerateCSRFToken(c),
    })
}

func updateProfileHandler(c *gin.Context) {
    var profileForm struct {
        Name  string `form:"name" binding:"required"`
        Email string `form:"email" binding:"required,email"`
    }
    
    if err := c.ShouldBind(&profileForm); err != nil {
        setFlashMessage(c, "error", "Invalid input")
        c.Redirect(http.StatusFound, "/dashboard/profile")
        return
    }
    
    sess := session.Get(c)
    userID := sess.GetInt("user_id")
    
    // Update user profile (implement your logic)
    err := updateWebUserProfile(userID, profileForm.Name, profileForm.Email)
    if err != nil {
        setFlashMessage(c, "error", "Update failed: "+err.Error())
        c.Redirect(http.StatusFound, "/dashboard/profile")
        return
    }
    
    // Update session data
    sess.Set("user_name", profileForm.Name)
    sess.Set("user_email", profileForm.Email)
    
    setFlashMessage(c, "success", "Profile updated successfully")
    c.Redirect(http.StatusFound, "/dashboard/profile")
}

// Middleware functions
func authRequiredMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        sess := session.Get(c)
        if !sess.Has("user_id") {
            setFlashMessage(c, "error", "Please log in to access this page")
            c.Redirect(http.StatusFound, "/login")
            c.Abort()
            return
        }
        c.Next()
    }
}

func flashMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        // Store session manager in context for flash message access
        c.Next()
    }
}

// Flash message helpers
func setFlashMessage(c *gin.Context, msgType, message string) {
    sess := session.Get(c)
    sess.FlashString(msgType + ":" + message)
}

func getFlashMessage(c *gin.Context) gin.H {
    sess := session.Get(c)
    if flashMsg, ok := sess.GetFlashString(); ok {
        parts := strings.SplitN(flashMsg, ":", 2)
        if len(parts) == 2 {
            return gin.H{
                "type":    parts[0],
                "message": parts[1],
            }
        }
    }
    return nil
}

// Helper functions (implement according to your needs)
func authenticateWebUser(email, password string) (*User, error) {
    // Implement web user authentication
    return &User{ID: 1, Name: "Web User", Email: email}, nil
}

func createWebUser(name, email, password string) (*User, error) {
    // Implement user creation
    return &User{ID: 123, Name: name, Email: email}, nil
}

func updateWebUserProfile(userID int, name, email string) error {
    // Implement profile update
    return nil
}
```

## Example 3: Microservice with Health Checks

Lightweight microservice with health checks, metrics, and monitoring endpoints.

```go
package main

import (
    "github.com/gin-gonic/gin"
    "github.com/oddbit-project/blueprint/provider/httpserver"
    "github.com/oddbit-project/blueprint/provider/httpserver/auth"
    "github.com/oddbit-project/blueprint/provider/httpserver/response"
    "github.com/oddbit-project/blueprint/log"
)

type HealthStatus struct {
    Status      string            `json:"status"`
    Version     string            `json:"version"`
    Timestamp   string            `json:"timestamp"`
    Uptime      string            `json:"uptime"`
    Dependencies map[string]string `json:"dependencies"`
}

func main() {
    logger := log.New("microservice")
    
    // Minimal configuration for microservice
    config := httpserver.NewServerConfig()
    config.Port = 8080
    config.Debug = false
    
    server, err := httpserver.NewServer(config, logger)
    if err != nil {
        logger.Fatal(err, "failed to create server")
    }
    
    setupMicroserviceMiddleware(server, logger)
    setupMicroserviceRoutes(server)
    
    logger.Info("starting microservice", "port", config.Port)
    if err := server.Start(); err != nil {
        logger.Fatal(err, "server failed")
    }
}

func setupMicroserviceMiddleware(server *httpserver.Server, logger *log.Logger) {
    // Minimal middleware for microservice
    server.UseDefaultSecurityHeaders()
    server.UseRateLimiting(1000) // Higher limit for microservice
    
    // Service-to-service authentication
    tokenAuth := auth.NewAuthToken("X-Service-Token", "service-secret-key")
    
    // Only protect internal endpoints
    internal := server.Group("/internal")
    internal.Use(auth.AuthMiddleware(tokenAuth))
}

func setupMicroserviceRoutes(server *httpserver.Server) {
    router := server.Route()
    
    // Public health endpoints
    router.GET("/health", healthCheckHandler)
    router.GET("/health/ready", readinessHandler)
    router.GET("/health/live", livenessHandler)
    router.GET("/metrics", metricsHandler)
    
    // Business logic endpoints
    router.GET("/api/data", getDataHandler)
    router.POST("/api/process", processDataHandler)
    
    // Internal endpoints (protected)
    internal := server.Group("/internal")
    {
        internal.GET("/config", getConfigHandler)
        internal.POST("/refresh", refreshCacheHandler)
        internal.GET("/stats", getStatsHandler)
    }
}

func healthCheckHandler(c *gin.Context) {
    status := HealthStatus{
        Status:    "healthy",
        Version:   "1.0.0",
        Timestamp: time.Now().Format(time.RFC3339),
        Uptime:    getUptime(),
        Dependencies: map[string]string{
            "database": "healthy",
            "cache":    "healthy",
            "queue":    "healthy",
        },
    }
    
    c.JSON(200, status)
}

func readinessHandler(c *gin.Context) {
    // Check if service is ready to serve traffic
    if !isServiceReady() {
        c.JSON(503, gin.H{
            "status": "not ready",
            "reason": "dependencies not available",
        })
        return
    }
    
    c.JSON(200, gin.H{"status": "ready"})
}

func livenessHandler(c *gin.Context) {
    // Check if service is alive
    c.JSON(200, gin.H{
        "status": "alive",
        "timestamp": time.Now().Format(time.RFC3339),
    })
}

func metricsHandler(c *gin.Context) {
    // Return Prometheus-style metrics
    metrics := `
# HELP http_requests_total Total number of HTTP requests
# TYPE http_requests_total counter
http_requests_total{method="GET"} 1234
http_requests_total{method="POST"} 567

# HELP http_request_duration_seconds HTTP request latency
# TYPE http_request_duration_seconds histogram
http_request_duration_seconds_bucket{le="0.1"} 100
http_request_duration_seconds_bucket{le="0.5"} 200
http_request_duration_seconds_bucket{le="1.0"} 300
http_request_duration_seconds_bucket{le="+Inf"} 350
`
    
    c.String(200, metrics)
}

func getDataHandler(c *gin.Context) {
    // Business logic endpoint
    data := gin.H{
        "message": "Service is working",
        "data":    []string{"item1", "item2", "item3"},
    }
    
    c.JSON(200, response.JSONResponse{
        Success: true,
        Data:    data,
    })
}

func processDataHandler(c *gin.Context) {
    var request map[string]interface{}
    if err := c.ShouldBindJSON(&request); err != nil {
        response.ValidationError(c, err)
        return
    }
    
    // Process the data
    result := processBusinessLogic(request)
    
    c.JSON(200, response.JSONResponse{
        Success: true,
        Data:    result,
    })
}

// Protected internal endpoints
func getConfigHandler(c *gin.Context) {
    config := gin.H{
        "database_url": "***hidden***",
        "cache_ttl":    3600,
        "worker_count": 10,
    }
    
    c.JSON(200, response.JSONResponse{
        Success: true,
        Data:    config,
    })
}

func refreshCacheHandler(c *gin.Context) {
    // Refresh internal caches
    err := refreshInternalCache()
    if err != nil {
        response.Http500(c, err)
        return
    }
    
    c.JSON(200, response.JSONResponse{
        Success: true,
        Data:    gin.H{"cache_refreshed": true},
    })
}

func getStatsHandler(c *gin.Context) {
    stats := gin.H{
        "requests_processed": 12345,
        "errors_count":      23,
        "average_latency":   "45ms",
        "memory_usage":      "128MB",
    }
    
    c.JSON(200, response.JSONResponse{
        Success: true,
        Data:    stats,
    })
}

// Helper functions
func getUptime() string {
    // Calculate service uptime
    return "2h 15m 30s"
}

func isServiceReady() bool {
    // Check dependencies
    return true
}

func processBusinessLogic(data map[string]interface{}) gin.H {
    // Implement business logic
    return gin.H{
        "processed": true,
        "result":    data,
    }
}

func refreshInternalCache() error {
    // Implement cache refresh
    return nil
}
```

## Configuration Examples

### Environment-based Configuration

```go
func createServerFromEnv() (*httpserver.Server, error) {
    config := httpserver.NewServerConfig()
    
    // Read from environment variables
    if port := os.Getenv("SERVER_PORT"); port != "" {
        if p, err := strconv.Atoi(port); err == nil {
            config.Port = p
        }
    }
    
    config.Host = os.Getenv("SERVER_HOST")
    config.Debug = os.Getenv("DEBUG") == "true"
    
    // TLS configuration
    if certFile := os.Getenv("TLS_CERT_FILE"); certFile != "" {
        config.TLSCert = certFile
        config.TLSKey = os.Getenv("TLS_KEY_FILE")
        config.TLSEnable = true
    }
    
    // Options from environment
    config.Options[httpserver.OptAuthTokenSecret] = os.Getenv("API_SECRET")
    config.Options[httpserver.OptDefaultSecurityHeaders] = "true"
    
    logger := log.New("app")
    return httpserver.NewServer(config, logger)
}
```

### Docker-ready Configuration

```go
func createDockerServer() (*httpserver.Server, error) {
    config := httpserver.NewServerConfig()
    config.Host = "0.0.0.0" // Bind to all interfaces in container
    config.Port = 8080
    config.Debug = false
    
    // Production timeouts
    config.ReadTimeout = 30
    config.WriteTimeout = 30
    
    logger := log.New("docker-app")
    server, err := httpserver.NewServer(config, logger)
    if err != nil {
        return nil, err
    }
    
    // Production middleware
    server.UseDefaultSecurityHeaders()
    server.UseRateLimiting(100)
    
    return server, nil
}
```

These examples demonstrate complete, production-ready applications using Blueprint's HTTP server framework with all the integrated components working together seamlessly.