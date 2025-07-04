package main

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/oddbit-project/blueprint/log"
	"github.com/oddbit-project/blueprint/provider/auth/jwt"
	"github.com/oddbit-project/blueprint/provider/httpserver"
	"github.com/oddbit-project/blueprint/provider/httpserver/session"
)

func main() {
	// Configure logger
	log.Configure(log.NewDefaultConfig())
	logger := log.New("jwt-auth-demo")

	// Create server config
	srvConfig := httpserver.NewServerConfig()
	srvConfig.Host = "localhost"
	srvConfig.Port = 8092
	srvConfig.Debug = true

	// Create HTTP server
	server, err := httpserver.NewServer(srvConfig, logger)
	if err != nil {
		logger.Fatal(err, "could not start http server")
		os.Exit(1)
	}

	// JWT configuration with RSA for JWKS support
	jwtConfig, err := jwt.NewJWTConfigWithRSA("RS256", 2048)
	if err != nil {
		logger.Fatal(err, "could not create RSA JWT config")
		os.Exit(1)
	}
	jwtConfig.ExpirationSeconds = 3600 // 1 hour
	jwtConfig.Issuer = "jwt-auth-demo"
	jwtConfig.Audience = "demo-users"
	jwtConfig.RequireIssuer = true
	jwtConfig.RequireAudience = true
	jwtConfig.KeyID = "demo-rsa-key"

	// Enable JWKS for public key distribution
	jwtConfig.JWKSConfig = &jwt.JWKSConfig{
		Enabled:  true,
		Endpoint: "/.well-known/jwks.json",
	}

	// Create JWT manager with revocation support
	revocationBackend := jwt.NewMemoryRevocationBackend()
	revocationManager := jwt.NewRevocationManager(revocationBackend)
	
	jwtManager, err := jwt.NewJWTManagerWithRevocation(jwtConfig, logger, revocationManager)
	if err != nil {
		logger.Fatal(err, "could not create JWT manager")
		os.Exit(1)
	}

	// Register JWKS endpoint for public key distribution
	jwtManager.RegisterJWKSEndpoint(server.Route())

	// Create JWT session manager
	sessionManager := jwt.NewJWTSessionManager(jwtManager)

	// Add JWT session middleware
	server.AddMiddleware(sessionManager.Middleware())

	// Serve static files and templates
	execDir, _ := os.Getwd()
	templatesPath := filepath.Join(execDir, "templates")
	server.Route().Static("/static", templatesPath)
	server.Route().LoadHTMLGlob(filepath.Join(templatesPath, "*.html"))

	// Web interface
	server.Route().GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", gin.H{
			"title": "JWT Auth Demo",
		})
	})

	// Define API routes

	// Login endpoint
	server.Route().POST("/login", func(c *gin.Context) {
		var credentials struct {
			Username string `json:"username" binding:"required"`
			Password string `json:"password" binding:"required"`
		}

		if err := c.ShouldBindJSON(&credentials); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Invalid request format",
			})
			return
		}

		// Simple credential validation (in production, use proper authentication)
		if credentials.Username != "admin" || credentials.Password != "password" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid credentials",
			})
			return
		}

		// Store user data in session
		session.Set(c, "user_id", "123")
		session.Set(c, "username", credentials.Username)
		session.Set(c, "role", "admin")
		session.Set(c, "authenticated", true)
		session.Set(c, "login_time", time.Now().Unix())

		// Force generation of new JWT token by getting the session and generating token
		sess := session.Get(c)
		newToken, err := jwtManager.Generate(sess.ID, sess)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to generate token",
			})
			return
		}

		// Set the token in both response header and session for consistency
		c.Header("Authorization", "Bearer "+newToken)
		sess.Values["_jwt_token"] = newToken

		c.JSON(http.StatusOK, gin.H{
			"success":  true,
			"message":  "Login successful",
			"user_id":  "123",
			"username": credentials.Username,
			"token":    newToken,
		})
	})

	// Protected profile endpoint
	server.Route().GET("/profile", func(c *gin.Context) {
		// Check authentication
		authenticated, ok := session.GetBool(c, "authenticated")
		if !ok || !authenticated {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Authentication required",
			})
			return
		}

		// Get user data from session
		userID, _ := session.GetString(c, "user_id")
		username, _ := session.GetString(c, "username")
		role, _ := session.GetString(c, "role")
		loginTime, _ := session.GetInt(c, "login_time")

		c.JSON(http.StatusOK, gin.H{
			"user_id":    userID,
			"username":   username,
			"role":       role,
			"login_time": time.Unix(int64(loginTime), 0).Format(time.RFC3339),
		})
	})

	// Token refresh endpoint
	server.Route().POST("/refresh", func(c *gin.Context) {
		// Check authentication
		authenticated, ok := session.GetBool(c, "authenticated")
		if !ok || !authenticated {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Authentication required",
			})
			return
		}

		// Force token refresh
		sessionManager.Regenerate(c)

		// Get the new token
		sess := session.Get(c)
		newToken := sess.Values["_jwt_token"]

		c.JSON(http.StatusOK, gin.H{
			"message": "Token refreshed successfully",
			"token":   newToken,
		})
	})

	// Admin endpoint (requires admin role)
	server.Route().GET("/admin", func(c *gin.Context) {
		// Check authentication
		authenticated, ok := session.GetBool(c, "authenticated")
		if !ok || !authenticated {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Authentication required",
			})
			return
		}

		// Check admin role
		role, ok := session.GetString(c, "role")
		if !ok || role != "admin" {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "Admin access required",
			})
			return
		}

		// Get revocation stats
		revokedTokens, _ := revocationManager.GetRevokedTokens()

		c.JSON(http.StatusOK, gin.H{
			"message":        "Admin dashboard",
			"revoked_tokens": len(revokedTokens),
			"server_info": gin.H{
				"jwt_algorithm": jwtConfig.SigningAlgorithm,
				"token_expiry":  fmt.Sprintf("%d seconds", jwtConfig.ExpirationSeconds),
				"jwks_enabled":  jwtConfig.JWKSConfig.Enabled,
			},
		})
	})

	// Token revocation endpoint
	server.Route().POST("/revoke", func(c *gin.Context) {
		// Check authentication
		authenticated, ok := session.GetBool(c, "authenticated")
		if !ok || !authenticated {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Authentication required",
			})
			return
		}

		// Get current session to revoke its token
		sess := session.Get(c)
		if token, exists := sess.Values["_jwt_token"].(string); exists {
			// Parse token to get expiration
			claims, err := jwtManager.Validate(token)
			if err == nil && claims.ExpiresAt != nil {
				// Revoke the token
				err = revocationManager.RevokeToken(claims.ID, claims.ExpiresAt.Time)
				if err != nil {
					logger.Error(err, "failed to revoke token")
				}
			}
		}

		// Clear the session
		sessionManager.Clear(c)

		c.JSON(http.StatusOK, gin.H{
			"message": "Token revoked and session cleared",
		})
	})

	// Logout endpoint
	server.Route().POST("/logout", func(c *gin.Context) {
		// Clear the session
		sessionManager.Clear(c)

		c.JSON(http.StatusOK, gin.H{
			"message": "Logged out successfully",
		})
	})

	// Health check endpoint
	server.Route().GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "healthy",
			"time":   time.Now().Format(time.RFC3339),
		})
	})

	// Start HTTP server
	logger.Info(fmt.Sprintf("JWT Auth Demo running at http://%s:%d", srvConfig.Host, srvConfig.Port))
	logger.Info("Web Interface: Open your browser and visit the URL above")
	logger.Info("")
	logger.Info("Available API endpoints:")
	logger.Info("  POST /login      - Authenticate user")
	logger.Info("  GET  /profile    - Get user profile (requires auth)")
	logger.Info("  POST /refresh    - Refresh JWT token (requires auth)")
	logger.Info("  GET  /admin      - Admin dashboard (requires admin role)")
	logger.Info("  POST /revoke     - Revoke current token (requires auth)")
	logger.Info("  POST /logout     - Logout user")
	logger.Info("  GET  /health     - Health check")
	logger.Info("  GET  /.well-known/jwks.json - JWKS endpoint")
	logger.Info("")
	logger.Info("Test credentials: admin / password")
	logger.Info("Press Ctrl+C to stop the server")
	
	// Start server in a goroutine
	go func() {
		if err := server.Start(); err != nil {
			logger.Fatal(err, "failed to start server")
		}
	}()

	// Wait for interrupt signal to gracefully shut down
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	
	logger.Info("Shutting down server...")
	os.Exit(0)
}