package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/oddbit-project/blueprint/log"
	"github.com/oddbit-project/blueprint/provider/httpserver"
	"github.com/oddbit-project/blueprint/provider/httpserver/auth"
	"github.com/oddbit-project/blueprint/provider/jwtprovider"
	"net/http"
	"os"
)

func main() {
	// Configure logger
	log.Configure(log.NewDefaultConfig())
	logger := log.New("jwtprovider-sample")

	// Create server config
	srvConfig := httpserver.NewServerConfig()
	srvConfig.Host = "localhost"
	srvConfig.Port = 8090
	srvConfig.Debug = true

	// Create HTTP server
	server, err := httpserver.NewServer(srvConfig, logger)
	if err != nil {
		logger.Fatal(err, "could not start http server")
		os.Exit(1)
	}

	// JWT configuration
	// For production, use a secure key or asymmetric algorithm
	cfg, err := jwtprovider.NewJWTConfigWithKey([]byte("your-secret-key-should-be-at-least-32-bytes"))
	if err != nil {
		logger.Fatal(err, "could not start http server")
		os.Exit(1)
	}
	cfg.ExpirationSeconds = 3600 // 1 hour
	cfg.Issuer = "jwtprovider-example"
	cfg.Audience = "api-users"

	// optional - create a revocation manager instance for token revocation
	revocationManager := jwtprovider.NewRevocationManager(jwtprovider.NewMemoryRevocationBackend())

	// create the JWT provider to use with the API server
	provider, err := jwtprovider.NewProvider(cfg, jwtprovider.WithRevocationManager(revocationManager))
	if err != nil {
		logger.Fatal(err, "could not start http server")
		os.Exit(1)
	}

	// Define public routes
	// VerifyUser endpoint - returns a JWT token with user data
	server.Route().POST("/login", func(c *gin.Context) {
		// Simulate authentication
		var credentials struct {
			Username string `json:"username" binding:"required"`
			Password string `json:"password" binding:"required"`
		}

		if err := c.ShouldBindJSON(&credentials); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Invalid request",
			})
			return
		}

		// Check credentials (dummy validation for demo)
		if credentials.Username != "user" || credentials.Password != "password" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid credentials",
			})
			return
		}

		// prepare extra data to be used in JWT token
		userData := map[string]any{
			"username":     credentials.Username,
			"userId":       123,
			"autenticated": true,
		}

		// Generate JWT token
		token, err := provider.GenerateToken(credentials.Username, userData)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to generate token",
			})
			return
		}

		// Set token in Authorization header
		c.Header("Authorization", "Bearer "+token)

		c.JSON(http.StatusOK, gin.H{
			"success":  true,
			"message":  "Authentication successful",
			"user_id":  123,
			"username": credentials.Username,
			"token":    token,
		})
	})

	// create JWT auth middleware for private routes
	server.UseAuth(auth.NewAuthJWT(provider))

	// Protected endpoint - requires authentication
	server.Route().GET("/profile", func(c *gin.Context) {
		// Get the JWT context
		claims, ok := auth.GetJWTClaims(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "No session found",
			})
			return
		}

		// Get user data
		userID, _ := claims.Data["userId"]
		username, _ := claims.Data["username"]

		// Get visits count
		visits, _ := claims.Data["visits"]

		// Return user profile
		c.JSON(http.StatusOK, gin.H{
			"user_id":  userID,
			"username": username,
			"visits":   visits,
		})
	})

	// Endpoint to demonstrate session data persistence
	server.Route().GET("/visit", func(c *gin.Context) {
		// Get the JWT context
		claims, ok := auth.GetJWTClaims(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "No session found",
			})
			return
		}

		// Update visit count
		// please note jwt does not support int format
		var visits float64 = 1
		if v, ok := claims.Data["visits"]; ok {
			visits = v.(float64) + 1.0
		}
		claims.Data["visits"] = visits

		// re-generate JWT token with new calues
		token, err := provider.GenerateToken(claims.Subject, claims.Data)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to generate token",
			})
			return
		}

		// Set token in Authorization header
		c.Header("Authorization", "Bearer "+token)

		c.JSON(http.StatusOK, gin.H{
			"message": fmt.Sprintf("Visit count: %v", visits),
			"visits":  visits,
		})
	})

	// Endpoint to refresh the token
	server.Route().POST("/refresh", func(c *gin.Context) {
		// Get the JWT context
		_, logged := auth.GetJWTClaims(c)
		if !logged {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "No session found",
			})
			return
		}

		// Force token refresh
		token, _ := auth.GetJWTToken(c)
		token, err := provider.Refresh(token)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to refresh token",
			})
			return
		}

		// Set new token in header
		c.Header("Authorization", "Bearer "+token)

		// Return token
		c.JSON(http.StatusOK, gin.H{
			"message": "Token refreshed",
			"token":   token,
		})
	})

	// Logout endpoint
	server.Route().POST("/logout", func(c *gin.Context) {

		// for logout, we revoke the token
		token, exists := auth.GetJWTToken(c)
		if exists {
			if err = provider.RevokeToken(token); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": "Failed to revoke token",
				})
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "Logged out successfully",
		})
	})

	// Health check endpoint (no auth required)
	server.Route().GET("/health", func(c *gin.Context) {
		_, logged := auth.GetJWTClaims(c)
		c.JSON(http.StatusOK, gin.H{
			"status":    "healthy",
			"server":    "jwt-session-example",
			"logged in": logged,
		})
	})

	// Start HTTP server
	logger.Info(fmt.Sprintf("Running JWT session demo at http://%s:%d", srvConfig.Host, srvConfig.Port))
	logger.Info("Available endpoints:")
	logger.Info("  POST /login    - Authenticate with username/password")
	logger.Info("  GET  /profile  - Get user profile (requires auth)")
	logger.Info("  GET  /visit    - Increment visit counter (requires auth)")
	logger.Info("  POST /refresh  - Refresh JWT token (requires auth)")
	logger.Info("  POST /logout   - Logout and clear session")
	logger.Info("  GET  /health   - Health check")
	logger.Info("")
	logger.Info("Default credentials: user / password")

	if err := server.Start(); err != nil {
		logger.Fatal(err, "Failed to start server")
	}
}
