package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/oddbit-project/blueprint/crypt/secure"
	"github.com/oddbit-project/blueprint/log"
	"github.com/oddbit-project/blueprint/provider/hmacprovider"
	"github.com/oddbit-project/blueprint/provider/httpserver/auth"
)

const (
	ServerPort  = ":8080"
	HMACSecret  = "python-client-demo-secret"
	TestMessage = "Hello from Python client!"
)

func main() {
	// Initialize logger
	logger := log.New("hmac-python-server")
	logger.Info("Starting HMAC Python Client Demo Server")

	// Create HMAC provider
	hmacProvider, err := createHMACProvider(logger)
	if err != nil {
		logger.Fatal(err, "Failed to create HMAC provider")
	}

	// Create HTTP server
	server, err := createServer(hmacProvider, logger)
	if err != nil {
		logger.Fatal(err, "Failed to create HTTP server")
	}

	// Start server
	go func() {
		logger.Info("Server starting", log.KV{"port": ServerPort})
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal(err, "Server failed to start")
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Server shutting down...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Error(err, "Server forced to shutdown")
	} else {
		logger.Info("Server shutdown complete")
	}
}

func createHMACProvider(logger *log.Logger) (*hmacprovider.HMACProvider, error) {
	// Generate encryption key
	key, err := secure.GenerateKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate key: %w", err)
	}

	// Create credential
	credential, err := secure.NewCredential([]byte(HMACSecret), key, false)
	if err != nil {
		return nil, fmt.Errorf("failed to create credential: %w", err)
	}

	// Create HMAC provider with default settings
	provider := hmacprovider.NewHmacProvider(credential)

	logger.Info("HMAC provider initialized", log.KV{
		"secret_preview": HMACSecret[:8] + "...",
	})

	return provider, nil
}

func createServer(hmacProvider *hmacprovider.HMACProvider, logger *log.Logger) (*http.Server, error) {
	// Set Gin mode
	gin.SetMode(gin.ReleaseMode)

	// Create router
	router := gin.New()

	// Add basic middleware
	router.Use(gin.Recovery())
	router.Use(corsMiddleware())
	router.Use(requestLogger(logger))

	// Setup routes
	setupRoutes(router, hmacProvider, logger)

	// Create server
	server := &http.Server{
		Addr:    ServerPort,
		Handler: router,
	}

	return server, nil
}

func setupRoutes(router *gin.Engine, hmacProvider *hmacprovider.HMACProvider, logger *log.Logger) {
	// Public endpoints (no authentication)
	public := router.Group("/api/public")
	{
		public.GET("/health", healthHandler)
		public.GET("/info", infoHandler)
		public.POST("/sign", signHandler(hmacProvider, logger))
	}

	// Protected endpoints (HMAC authentication required)
	protected := router.Group("/api/protected")
	protected.Use(auth.AuthMiddleware(auth.NewHMACAuthProvider(hmacProvider)))
	{
		protected.GET("/profile", profileHandler)
		protected.POST("/data", dataHandler)
		protected.PUT("/settings", settingsHandler)
		protected.DELETE("/resource/:id", deleteHandler)
		protected.POST("/echo", echoHandler)
	}

	// Test endpoints for Python client validation
	test := router.Group("/api/test")
	test.Use(auth.AuthMiddleware(auth.NewHMACAuthProvider(hmacProvider)))
	{
		test.GET("/simple", simpleTestHandler)
		test.POST("/json", jsonTestHandler)
		test.POST("/large", largeDataTestHandler)
	}
}

// Middleware functions
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, X-HMAC-Hash, X-HMAC-Timestamp, X-HMAC-Nonce")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

func requestLogger(logger *log.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		logger.Info("Request completed", log.KV{
			"method":    c.Request.Method,
			"path":      c.Request.URL.Path,
			"status":    c.Writer.Status(),
			"latency":   time.Since(start).String(),
			"client_ip": c.ClientIP(),
		})
	}
}

// Public endpoint handlers
func healthHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"service":   "HMAC Python Client Demo Server",
	})
}

func infoHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"service":        "HMAC Python Client Demo Server",
		"description":    "Go server for testing Python HMAC client compatibility",
		"authentication": "HMAC-SHA256 with nonce and timestamp",
		"secret_preview": HMACSecret[:8] + "...",
		"endpoints": gin.H{
			"public": []string{
				"GET /api/public/health",
				"GET /api/public/info",
				"POST /api/public/sign",
			},
			"protected": []string{
				"GET /api/protected/profile",
				"POST /api/protected/data",
				"PUT /api/protected/settings",
				"DELETE /api/protected/resource/:id",
				"POST /api/protected/echo",
			},
			"test": []string{
				"GET /api/test/simple",
				"POST /api/test/json",
				"POST /api/test/large",
			},
		},
	})
}

func signHandler(hmacProvider *hmacprovider.HMACProvider, logger *log.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		var request struct {
			Data string `json:"data" binding:"required"`
		}

		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Sign the data
		hash, timestamp, nonce, err := hmacProvider.Sign256(
			strings.NewReader(request.Data),
		)
		if err != nil {
			logger.Error(err, "Failed to sign data")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to sign data"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"hash":      hash,
			"timestamp": timestamp,
			"nonce":     nonce,
			"message":   "Data signed successfully",
		})
	}
}

// Protected endpoint handlers
func profileHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"user_id":    "python-client-user",
		"username":   "python_tester",
		"email":      "python@example.com",
		"created_at": "2024-01-01T00:00:00Z",
		"last_login": time.Now().UTC().Format(time.RFC3339),
		"message":    "Hello from Go server! Python client authentication successful.",
	})
}

func dataHandler(c *gin.Context) {
	var request struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":          fmt.Sprintf("msg_%d", time.Now().Unix()),
		"message":     request.Message,
		"type":        request.Type,
		"processed":   time.Now().UTC().Format(time.RFC3339),
		"server":      "Go Blueprint HMAC Server",
		"client_type": "Python HMAC Client",
	})
}

func settingsHandler(c *gin.Context) {
	var settings map[string]interface{}

	if err := c.ShouldBindJSON(&settings); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "Settings updated successfully via Python client",
		"settings":   settings,
		"updated_at": time.Now().UTC().Format(time.RFC3339),
		"server":     "Go Blueprint HMAC Server",
	})
}

func deleteHandler(c *gin.Context) {
	resourceID := c.Param("id")

	c.JSON(http.StatusOK, gin.H{
		"message":     fmt.Sprintf("Resource %s deleted via Python client", resourceID),
		"resource_id": resourceID,
		"deleted_at":  time.Now().UTC().Format(time.RFC3339),
		"server":      "Go Blueprint HMAC Server",
	})
}

func echoHandler(c *gin.Context) {
	// Read request body
	body, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read body"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"echo":      string(body),
		"size":      len(body),
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"headers":   c.Request.Header,
		"message":   "Request echoed successfully from Go server",
	})
}

// Test endpoint handlers
func simpleTestHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"test":    "simple",
		"status":  "success",
		"message": "Simple authenticated endpoint working",
		"time":    time.Now().UTC().Format(time.RFC3339),
	})
}

func jsonTestHandler(c *gin.Context) {
	var payload map[string]interface{}

	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"test":     "json",
		"status":   "success",
		"received": payload,
		"message":  "JSON payload processed successfully",
		"time":     time.Now().UTC().Format(time.RFC3339),
	})
}

func largeDataTestHandler(c *gin.Context) {
	body, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read body"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"test":     "large",
		"status":   "success",
		"size":     len(body),
		"checksum": fmt.Sprintf("%x", body[:min(16, len(body))]),
		"message":  "Large data processed successfully",
		"time":     time.Now().UTC().Format(time.RFC3339),
	})
}
