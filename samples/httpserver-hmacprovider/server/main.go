package main

import (
	"bytes"
	"context"
	"fmt"
	"github.com/oddbit-project/blueprint/provider/httpserver/auth"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/oddbit-project/blueprint/crypt/secure"
	"github.com/oddbit-project/blueprint/log"
	"github.com/oddbit-project/blueprint/provider/hmacprovider"
	"github.com/oddbit-project/blueprint/provider/hmacprovider/store"
)

const (
	// Server configuration
	ServerPort     = ":8080"
	HMACSecret     = "your-hmac-secret-key-change-this-in-production"
	RequestTimeout = 30 * time.Second
	KeyId          = "myKey"

	// HMAC configuration
	HMACKeyInterval = 5 * time.Minute  // ±5 minutes for clock drift
	HMACMaxInput    = 10 * 1024 * 1024 // 10MB max request size
	NonceStoreTTL   = 1 * time.Hour    // Nonce TTL
)

func main() {
	// Initialize logger
	logger := log.New("hmac-server")
	logger.Info("Starting HMAC HTTP Server Example")

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

	// Wait for interrupt signal to gracefully shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Server shutting down...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Error(err, "Server forced to shutdown")
	} else {
		logger.Info("Server shutdown complete")
	}
}

// createHMACProvider initializes the HMAC provider with secure configuration
func createHMACProvider(logger *log.Logger) (*hmacprovider.HMACProvider, error) {
	// Generate encryption key for secret storage
	key, err := secure.GenerateKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate encryption key: %w", err)
	}

	// Create secure secret
	secret, err := secure.NewCredential([]byte(HMACSecret), key, false)
	if err != nil {
		return nil, fmt.Errorf("failed to create secret: %w", err)
	}

	keyProvider := hmacprovider.NewSingleKeyProvider(KeyId, secret)

	// Create memory nonce store with eviction policy
	nonceStore := store.NewMemoryNonceStore(
		store.WithTTL(NonceStoreTTL),
		store.WithMaxSize(100000), // 100k nonces max
		store.WithCleanupInterval(15*time.Minute),
		store.WithEvictPolicy(store.EvictHalfLife()), // Evict old nonces at capacity
	)

	// Create HMAC provider with security settings
	provider := hmacprovider.NewHmacProvider(
		keyProvider,
		hmacprovider.WithNonceStore(nonceStore),
		hmacprovider.WithKeyInterval(HMACKeyInterval),
		hmacprovider.WithMaxInputSize(HMACMaxInput),
	)

	logger.Info("HMAC provider initialized", log.KV{
		"key_interval":   HMACKeyInterval.String(),
		"max_input_size": HMACMaxInput,
		"nonce_ttl":      NonceStoreTTL.String(),
	})

	return provider, nil
}

// createServer creates and configures the HTTP server
func createServer(hmacProvider *hmacprovider.HMACProvider, logger *log.Logger) (*http.Server, error) {
	// Set Gin mode
	gin.SetMode(gin.ReleaseMode)

	// Create router
	router := gin.New()

	// Add middleware
	router.Use(ErrorHandler(logger))
	router.Use(HMACRequestLogger(logger))

	// Setup routes
	setupRoutes(router, hmacProvider, logger)

	// Create server
	server := &http.Server{
		Addr:           ServerPort,
		Handler:        router,
		ReadTimeout:    RequestTimeout,
		WriteTimeout:   RequestTimeout,
		IdleTimeout:    60 * time.Second,
		MaxHeaderBytes: 1 << 20, // 1MB
	}

	return server, nil
}

// setupRoutes configures all HTTP routes
func setupRoutes(router *gin.Engine, hmacProvider *hmacprovider.HMACProvider, logger *log.Logger) {
	// Public endpoints (no authentication required)
	public := router.Group("/api/public")
	{
		public.GET("/health", healthHandler)
		public.GET("/info", infoHandler)
		public.POST("/sign", signDataHandler(hmacProvider, logger))
	}

	// Add authentication
	router.Use(auth.AuthMiddleware(auth.NewHMACAuthProvider(hmacProvider)))

	// Protected endpoints (HMAC authentication required)
	protected := router.Group("/api/protected")
	{
		protected.GET("/profile", profileHandler)
		protected.POST("/data", dataHandler)
		protected.PUT("/settings", settingsHandler)
		protected.DELETE("/resource/:id", deleteResourceHandler)
	}

	// Admin endpoints (HMAC authentication required)
	admin := router.Group("/api/admin")
	{
		admin.GET("/stats", statsHandler(hmacProvider))
		admin.POST("/maintenance", maintenanceHandler)
		admin.GET("/logs", logsHandler(logger))
	}
}

// Public endpoint handlers
func healthHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"version":   "1.0.0",
	})
}

func infoHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"service":        "HMAC HTTP Server Example",
		"description":    "Example HTTP server with HMAC authentication middleware",
		"authentication": "HMAC-SHA256 with nonce and timestamp",
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
			},
			"admin": []string{
				"GET /api/admin/stats",
				"POST /api/admin/maintenance",
				"GET /api/admin/logs",
			},
		},
		"required_headers": []string{
			"X-HMAC-Hash",
			"X-HMAC-Timestamp",
			"X-HMAC-Nonce",
		},
	})
}

func signDataHandler(hmacProvider *hmacprovider.HMACProvider, logger *log.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		var request struct {
			Data string `json:"data" binding:"required"`
		}

		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Sign the data
		hash, timestamp, nonce, err := hmacProvider.Sign256(KeyId,
			bytes.NewReader([]byte(request.Data)),
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
		"user_id":          "user123",
		"username":         "example_user",
		"email":            "user@example.com",
		"created_at":       "2024-01-01T00:00:00Z",
		"last_login":       time.Now().UTC().Format(time.RFC3339),
		"authenticated_at": c.GetString("auth_timestamp"),
	})
}

func dataHandler(c *gin.Context) {
	var request struct {
		Message string `json:"message" binding:"required"`
		Type    string `json:"type"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":            fmt.Sprintf("msg_%d", time.Now().Unix()),
		"message":       request.Message,
		"type":          request.Type,
		"processed_at":  time.Now().UTC().Format(time.RFC3339),
		"authenticated": true,
	})
}

func settingsHandler(c *gin.Context) {
	var settings struct {
		Theme       string                 `json:"theme"`
		Language    string                 `json:"language"`
		Timezone    string                 `json:"timezone"`
		Preferences map[string]interface{} `json:"preferences"`
	}

	if err := c.ShouldBindJSON(&settings); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "Settings updated successfully",
		"settings":   settings,
		"updated_at": time.Now().UTC().Format(time.RFC3339),
	})
}

func deleteResourceHandler(c *gin.Context) {
	resourceID := c.Param("id")

	if resourceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Resource ID is required"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":     fmt.Sprintf("Resource %s deleted successfully", resourceID),
		"resource_id": resourceID,
		"deleted_at":  time.Now().UTC().Format(time.RFC3339),
	})
}

// Admin endpoint handlers
func statsHandler(hmacProvider *hmacprovider.HMACProvider) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"server": gin.H{
				"uptime":             time.Since(time.Now().Add(-time.Hour)).String(), // Mock uptime
				"requests_processed": 1000,                                            // Mock counter
				"errors":             5,                                               // Mock error count
			},
			"hmac": gin.H{
				"algorithm":      "HMAC-SHA256",
				"key_interval":   HMACKeyInterval.String(),
				"max_input_size": HMACMaxInput,
				"nonce_store":    "memory",
			},
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
	}
}

func maintenanceHandler(c *gin.Context) {
	var request struct {
		Action string `json:"action" binding:"required"`
		Force  bool   `json:"force"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":     fmt.Sprintf("Maintenance action '%s' executed", request.Action),
		"action":      request.Action,
		"force":       request.Force,
		"executed_at": time.Now().UTC().Format(time.RFC3339),
	})
}

func logsHandler(logger *log.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// In a real application, you would query actual logs
		// This is just a mock response
		logs := []gin.H{
			{
				"timestamp": time.Now().Add(-10 * time.Minute).UTC().Format(time.RFC3339),
				"level":     "INFO",
				"message":   "HMAC authentication successful",
				"client_ip": "192.168.1.100",
			},
			{
				"timestamp": time.Now().Add(-5 * time.Minute).UTC().Format(time.RFC3339),
				"level":     "WARN",
				"message":   "HMAC verification failed - invalid signature",
				"client_ip": "192.168.1.200",
			},
			{
				"timestamp": time.Now().Add(-1 * time.Minute).UTC().Format(time.RFC3339),
				"level":     "INFO",
				"message":   "HTTP request completed successfully",
				"status":    200,
			},
		}

		c.JSON(http.StatusOK, gin.H{
			"logs":         logs,
			"count":        len(logs),
			"retrieved_at": time.Now().UTC().Format(time.RFC3339),
		})
	}
}
