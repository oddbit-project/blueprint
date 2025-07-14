# HTTP Server Performance Guide

Comprehensive guide for optimizing Blueprint's HTTP server performance, scaling strategies, and production deployment best practices.

## Performance Fundamentals

### Server Configuration Optimization

#### Connection Timeouts

```go
func optimizeServerTimeouts(config *httpserver.ServerConfig) {
    // Production timeouts
    config.ReadTimeout = 30   // 30 seconds read timeout
    config.WriteTimeout = 30  // 30 seconds write timeout
    
    // For API servers with long-running operations
    config.ReadTimeout = 60   // 1 minute
    config.WriteTimeout = 300 // 5 minutes for large responses
    
    // For microservices with quick responses
    config.ReadTimeout = 10   // 10 seconds
    config.WriteTimeout = 10  // 10 seconds
}
```

#### HTTP Server Tuning

```go
func createOptimizedServer(config *httpserver.ServerConfig, logger *log.Logger) (*httpserver.Server, error) {
    server, err := httpserver.NewServer(config, logger)
    if err != nil {
        return nil, err
    }
    
    // Optimize underlying HTTP server
    server.Server.MaxHeaderBytes = 1 << 20 // 1MB max header size
    server.Server.IdleTimeout = 120 * time.Second
    server.Server.ReadHeaderTimeout = 5 * time.Second
    
    return server, nil
}
```

### Gin Router Optimization

#### Release Mode

```go
func optimizeGinRouter() {
    // Always use release mode in production
    gin.SetMode(gin.ReleaseMode)
    
    // Or set via environment
    os.Setenv("GIN_MODE", "release")
}
```

#### Router Configuration

```go
func createOptimizedRouter(logger *log.Logger) *gin.Engine {
    // Disable debug features
    gin.SetMode(gin.ReleaseMode)
    
    router := gin.New()
    
    // Use only necessary middleware
    if logger != nil {
        router.Use(httplog.HTTPLogMiddleware(logger))
    }
    router.Use(gin.Recovery())
    
    // Avoid unnecessary middleware in production
    // router.Use(gin.Logger()) // Skip default logger
    
    return router
}
```

## Middleware Performance

### Middleware Ordering

Optimize middleware order for best performance:

```go
func optimizeMiddlewareOrder(server *httpserver.Server, logger *log.Logger) {
    // 1. Fast security headers (minimal overhead)
    server.UseDefaultSecurityHeaders()
    
    // 2. Rate limiting (early rejection of excess traffic)
    server.UseRateLimiting(1000)
    
    // 3. Request ID (lightweight)
    server.AddMiddleware(requestIDMiddleware())
    
    // 4. Authentication (reject unauthorized early)
    tokenAuth := auth.NewAuthToken("X-API-Key", "secret")
    server.UseAuth(tokenAuth)
    
    // 5. Expensive middleware last
    server.UseSession(sessionConfig, backend, logger)
    server.UseCSRFProtection()
}
```

### Efficient Rate Limiting

```go
func efficientRateLimiting(server *httpserver.Server) {
    // Use efficient rate limiting
    r := rate.Every(time.Second / 100) // 100 requests per second
    burst := 50                        // Allow bursts
    
    server.AddMiddleware(security.RateLimitMiddleware(r, burst))
    
    // For high-traffic scenarios, consider Redis-based rate limiting
    // with connection pooling and distributed counters
}
```

### Lightweight Middleware

```go
func lightweightMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        // Minimal processing
        c.Header("X-Request-ID", generateFastID())
        c.Next()
    }
}

// Avoid expensive operations in middleware
func avoidSlowMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        // Avoid:
        // - Database queries
        // - External API calls
        // - Heavy computations
        // - Large memory allocations
        
        c.Next()
    }
}
```

## Memory Optimization

### Connection Management

```go
func optimizeConnections(server *httpserver.Server) {
    // Configure connection pooling
    server.Server.SetKeepAlivesEnabled(true)
    server.Server.IdleTimeout = 60 * time.Second
    
    // For high-concurrency scenarios
    server.Server.MaxHeaderBytes = 32 << 10 // 32KB max headers
}
```

### Memory Usage Monitoring

```go
func memoryMonitoringMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        // Monitor memory usage periodically, not on every request
        if rand.Intn(1000) == 0 { // 0.1% sampling
            var m runtime.MemStats
            runtime.ReadMemStats(&m)
            
            if m.Alloc > 500<<20 { // 500MB threshold
                log.Warn("High memory usage", "alloc", m.Alloc)
            }
        }
        
        c.Next()
    }
}
```

### Garbage Collection Optimization

```go
func optimizeGC() {
    // Tune GC for server workloads
    debug.SetGCPercent(100) // Default is usually good
    
    // For memory-constrained environments
    debug.SetGCPercent(50)
    
    // For high-throughput scenarios
    debug.SetGCPercent(200)
    
    // Set memory limit (Go 1.19+)
    debug.SetMemoryLimit(1 << 30) // 1GB limit
}
```

## Session Performance

### Efficient Session Storage

```go
func optimizeSessionStorage(logger *log.Logger) kv.KV {
    // For single instance - memory is fastest
    if isSingleInstance() {
        return kv.NewMemoryKV()
    }
    
    // For distributed - Redis with connection pooling
    redisConfig := redis.NewConfig()
    redisConfig.Address = "redis:6379"
    redisConfig.MaxConnections = 100
    redisConfig.MaxIdle = 20
    redisConfig.IdleTimeout = 300 * time.Second
    
    backend, err := redis.NewClient(redisConfig)
    if err != nil {
        logger.Error(err, "failed to connect to Redis, falling back to memory")
        return kv.NewMemoryKV()
    }
    
    return backend
}
```

### Session Configuration

```go
func optimizeSessionConfig() *session.Config {
    config := session.NewConfig()
    
    // Optimize timeouts
    config.ExpirationSeconds = 3600      // 1 hour
    config.IdleTimeoutSeconds = 1800     // 30 minutes
    config.CleanupIntervalSeconds = 300  // 5 minutes
    
    // Optimize for performance
    config.HttpOnly = true               // Prevent XSS
    config.Secure = true                 // HTTPS only in production
    config.SameSite = http.SameSiteStrictMode
    
    return config
}
```

## Database and External Services

### Connection Pooling

```go
type OptimizedService struct {
    db     *sql.DB
    cache  *redis.Client
    logger *log.Logger
}

func NewOptimizedService() *OptimizedService {
    // Database connection pool
    db, _ := sql.Open("postgres", dsn)
    db.SetMaxOpenConns(100)          // Max concurrent connections
    db.SetMaxIdleConns(10)           // Idle connections to keep
    db.SetConnMaxLifetime(time.Hour) // Connection lifetime
    
    // Redis connection pool
    redisClient := redis.NewClient(&redis.Options{
        Addr:         "redis:6379",
        PoolSize:     100,
        MinIdleConns: 10,
        PoolTimeout:  4 * time.Second,
    })
    
    return &OptimizedService{
        db:    db,
        cache: redisClient,
    }
}
```

### Caching Strategies

```go
func cacheMiddleware(cache *redis.Client) gin.HandlerFunc {
    return func(c *gin.Context) {
        // Only cache GET requests
        if c.Request.Method != "GET" {
            c.Next()
            return
        }
        
        cacheKey := generateCacheKey(c.Request.URL.Path, c.Request.URL.RawQuery)
        
        // Try cache first
        if cached, err := cache.Get(ctx, cacheKey).Result(); err == nil {
            c.Header("X-Cache", "HIT")
            c.Data(200, "application/json", []byte(cached))
            return
        }
        
        // Capture response
        w := &responseWriter{ResponseWriter: c.Writer}
        c.Writer = w
        c.Next()
        
        // Cache successful responses
        if w.status == 200 && len(w.body) > 0 {
            cache.Set(ctx, cacheKey, w.body, 5*time.Minute)
        }
    }
}

type responseWriter struct {
    gin.ResponseWriter
    body   []byte
    status int
}

func (w *responseWriter) Write(data []byte) (int, error) {
    w.body = append(w.body, data...)
    return w.ResponseWriter.Write(data)
}

func (w *responseWriter) WriteHeader(status int) {
    w.status = status
    w.ResponseWriter.WriteHeader(status)
}
```

## Load Balancing and Scaling

### Horizontal Scaling

```go
func createLoadBalancedServer(instanceID string) *httpserver.Server {
    config := httpserver.NewServerConfig()
    
    // Each instance gets a unique port for development
    basePort := 8080
    port := basePort + instanceID
    config.Port = port
    
    // Shared configuration
    config.ReadTimeout = 30
    config.WriteTimeout = 30
    
    logger := log.New(fmt.Sprintf("instance-%d", instanceID))
    server, _ := httpserver.NewServer(config, logger)
    
    return server
}
```

### Health Checks for Load Balancers

```go
func setupHealthChecks(server *httpserver.Server) {
    router := server.Route()
    
    // Simple health check
    router.GET("/health", func(c *gin.Context) {
        c.JSON(200, gin.H{"status": "healthy"})
    })
    
    // Detailed health check
    router.GET("/health/detailed", func(c *gin.Context) {
        checks := performHealthChecks()
        
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
            "status": allHealthy,
            "checks": checks,
            "instance": os.Getenv("INSTANCE_ID"),
            "timestamp": time.Now(),
        })
    })
    
    // Readiness check (for Kubernetes)
    router.GET("/ready", func(c *gin.Context) {
        if isReady() {
            c.JSON(200, gin.H{"ready": true})
        } else {
            c.JSON(503, gin.H{"ready": false})
        }
    })
    
    // Liveness check (for Kubernetes)
    router.GET("/live", func(c *gin.Context) {
        c.JSON(200, gin.H{"alive": true})
    })
}

func performHealthChecks() map[string]string {
    checks := make(map[string]string)
    
    // Database check
    if pingDatabase() {
        checks["database"] = "healthy"
    } else {
        checks["database"] = "unhealthy"
    }
    
    // Redis check
    if pingRedis() {
        checks["redis"] = "healthy"
    } else {
        checks["redis"] = "unhealthy"
    }
    
    // Memory check
    if checkMemoryUsage() {
        checks["memory"] = "healthy"
    } else {
        checks["memory"] = "warning"
    }
    
    return checks
}
```

### Graceful Shutdown

```go
func gracefulShutdownServer(server *httpserver.Server, logger *log.Logger) {
    // Channel to receive OS signals
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    
    // Start server in goroutine
    go func() {
        logger.Info("server starting", "port", server.Config.Port)
        if err := server.Start(); err != nil {
            logger.Error(err, "server failed to start")
        }
    }()
    
    // Block until signal received
    <-quit
    logger.Info("shutting down server...")
    
    // Create context with timeout for graceful shutdown
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()
    
    // Attempt graceful shutdown
    if err := server.Shutdown(ctx); err != nil {
        logger.Error(err, "forced shutdown")
        os.Exit(1)
    }
    
    logger.Info("server stopped gracefully")
}
```

## Monitoring and Metrics

### Performance Metrics

```go
func performanceMetricsMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        start := time.Now()
        
        c.Next()
        
        duration := time.Since(start)
        
        // Record metrics (use your preferred metrics library)
        recordMetrics(c.Request.Method, c.FullPath(), c.Writer.Status(), duration)
        
        // Log slow requests
        if duration > 1*time.Second {
            log.Warn("slow request",
                "path", c.Request.URL.Path,
                "method", c.Request.Method,
                "duration", duration,
                "status", c.Writer.Status())
        }
    }
}

func recordMetrics(method, path string, status int, duration time.Duration) {
    // Implementation depends on your metrics system
    // Examples: Prometheus, StatsD, CloudWatch, etc.
}
```

### Prometheus Metrics

```go
import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
    httpDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "http_request_duration_seconds",
            Help: "Duration of HTTP requests in seconds",
        },
        []string{"method", "path", "status"},
    )
    
    httpRequests = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "http_requests_total",
            Help: "Total number of HTTP requests",
        },
        []string{"method", "path", "status"},
    )
)

func init() {
    prometheus.MustRegister(httpDuration)
    prometheus.MustRegister(httpRequests)
}

func prometheusMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        start := time.Now()
        
        c.Next()
        
        duration := time.Since(start)
        status := strconv.Itoa(c.Writer.Status())
        
        httpDuration.WithLabelValues(c.Request.Method, c.FullPath(), status).Observe(duration.Seconds())
        httpRequests.WithLabelValues(c.Request.Method, c.FullPath(), status).Inc()
    }
}

func setupMetricsEndpoint(server *httpserver.Server) {
    server.Route().GET("/metrics", gin.WrapH(promhttp.Handler()))
}
```

## Production Deployment

### Container Optimization

```dockerfile
# Dockerfile optimized for performance
FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY ../../httpserver .
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main .

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/

COPY --from=builder /app/main .

# Performance-optimized runtime
ENV GIN_MODE=release
ENV GOGC=100
ENV GOMAXPROCS=0

EXPOSE 8080
CMD ["./main"]
```

### Environment Configuration

```go
func productionConfig() *httpserver.ServerConfig {
    config := httpserver.NewServerConfig()
    
    // Read from environment
    config.Port = getEnvInt("PORT", 8080)
    config.Host = getEnv("HOST", "0.0.0.0")
    config.ReadTimeout = getEnvInt("READ_TIMEOUT", 30)
    config.WriteTimeout = getEnvInt("WRITE_TIMEOUT", 30)
    
    // Production settings
    config.Debug = false
    
    // TLS configuration
    if certFile := os.Getenv("TLS_CERT_FILE"); certFile != "" {
        config.TLSCert = certFile
        config.TLSKey = os.Getenv("TLS_KEY_FILE")
        config.TLSEnable = true
    }
    
    return config
}

func getEnv(key, defaultValue string) string {
    if value := os.Getenv(key); value != "" {
        return value
    }
    return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
    if value := os.Getenv(key); value != "" {
        if i, err := strconv.Atoi(value); err == nil {
            return i
        }
    }
    return defaultValue
}
```

### Kubernetes Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: http-server
spec:
  replicas: 3
  selector:
    matchLabels:
      app: http-server
  template:
    metadata:
      labels:
        app: http-server
    spec:
      containers:
      - name: http-server
        image: your-app:latest
        ports:
        - containerPort: 8080
        env:
        - name: PORT
          value: "8080"
        - name: GIN_MODE
          value: "release"
        resources:
          requests:
            memory: "128Mi"
            cpu: "100m"
          limits:
            memory: "512Mi"
            cpu: "500m"
        livenessProbe:
          httpGet:
            path: /live
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /ready
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
---
apiVersion: v1
kind: Service
metadata:
  name: http-server-service
spec:
  selector:
    app: http-server
  ports:
  - protocol: TCP
    port: 80
    targetPort: 8080
  type: LoadBalancer
```

## Performance Testing

### Load Testing

```go
func loadTestEndpoint() {
    // Example using built-in testing
    func TestServerPerformance(t *testing.T) {
        server := setupTestServer()
        
        // Concurrent requests
        concurrency := 100
        requests := 1000
        
        var wg sync.WaitGroup
        results := make(chan time.Duration, requests)
        
        for i := 0; i < concurrency; i++ {
            wg.Add(1)
            go func() {
                defer wg.Done()
                for j := 0; j < requests/concurrency; j++ {
                    start := time.Now()
                    
                    req := httptest.NewRequest("GET", "/api/test", nil)
                    w := httptest.NewRecorder()
                    server.Route().ServeHTTP(w, req)
                    
                    results <- time.Since(start)
                }
            }()
        }
        
        wg.Wait()
        close(results)
        
        // Analyze results
        var durations []time.Duration
        for duration := range results {
            durations = append(durations, duration)
        }
        
        sort.Slice(durations, func(i, j int) bool {
            return durations[i] < durations[j]
        })
        
        p50 := durations[len(durations)/2]
        p95 := durations[int(float64(len(durations))*0.95)]
        p99 := durations[int(float64(len(durations))*0.99)]
        
        t.Logf("Performance Results: P50=%v, P95=%v, P99=%v", p50, p95, p99)
        
        // Assert performance requirements
        assert.True(t, p95 < 100*time.Millisecond, "95th percentile should be under 100ms")
    }
}
```

## Best Practices Summary

### Configuration
- Use release mode in production
- Set appropriate timeouts
- Configure connection pooling
- Enable keep-alive connections

### Middleware
- Order middleware by execution cost
- Minimize middleware overhead
- Use efficient rate limiting
- Implement proper caching

### Memory Management
- Monitor memory usage
- Tune garbage collection
- Limit request sizes
- Use connection pooling

### Monitoring
- Implement health checks
- Add performance metrics
- Monitor error rates
- Set up alerting

### Scaling
- Design for horizontal scaling
- Implement graceful shutdown
- Use load balancers
- Cache frequently accessed data

### Security vs Performance
- Balance security and performance
- Use efficient authentication
- Implement reasonable rate limits
- Cache security validations when possible

This performance guide provides a comprehensive foundation for building high-performance HTTP servers with Blueprint while maintaining security and reliability.