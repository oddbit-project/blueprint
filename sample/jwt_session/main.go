package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/oddbit-project/blueprint/log"
	"github.com/oddbit-project/blueprint/provider/httpserver"
	"github.com/oddbit-project/blueprint/provider/httpserver/session"
	"net/http"
	"os"
)

func main() {
	// Configure logger
	log.Configure(log.NewDefaultConfig())
	logger := log.New("jwt-session-sample")

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

	// Configure JWT session
	sessionConfig := session.DefaultSessionConfig()
	sessionConfig.Logger = logger
	
	// JWT configuration
	jwtConfig := session.DefaultJWTConfig()
	jwtConfig.SigningKey = []byte("your-secret-key-should-be-at-least-32-bytes")
	jwtConfig.Logger = logger
	
	// Use JWT session middleware
	sessionManager, err := server.UseSessionWithJWT(sessionConfig, jwtConfig)
	if err != nil {
		logger.Fatal(err, "could not create JWT session manager")
		os.Exit(1)
	}

	// Define routes
	// Auth endpoint - returns a JWT token with user data
	server.Route().POST("/auth", func(c *gin.Context) {
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
		
		// Get the session
		sess := session.Get(c)
		
		// Store user data in session
		session.Set(c, "user_id", 123)
		session.Set(c, "username", credentials.Username)
		session.Set(c, "authenticated", true)
		
		// Session set in middleware will automatically add the JWT token to
		// the Authorization header in the response
		
		c.JSON(http.StatusOK, gin.H{
			"success":  true,
			"message":  "Authentication successful",
			"user_id":  123,
			"username": credentials.Username,
			// Note: In a real application, you'd get the token from the header
			// For convenience in this demo, we'll include it in the response
			"token":    sess.Values["_jwt_token"],
		})
	})
	
	// Protected endpoint - requires authentication
	server.Route().GET("/profile", func(c *gin.Context) {
		// Get the session
		sess := session.Get(c)
		
		// Check if user is authenticated
		authenticated, ok := session.GetBool(c, "authenticated")
		if !ok || !authenticated {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Authentication required",
			})
			return
		}
		
		// Get user data
		userID, _ := session.GetInt(c, "user_id")
		username, _ := session.GetString(c, "username")
		
		// Return user profile
		c.JSON(http.StatusOK, gin.H{
			"user_id":  userID,
			"username": username,
			"visits":   sess.Values["visits"],
		})
	})
	
	// Endpoint to demonstrate session data persistence
	server.Route().GET("/visit", func(c *gin.Context) {
		// Check if user is authenticated
		authenticated, ok := session.GetBool(c, "authenticated")
		if !ok || !authenticated {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Authentication required",
			})
			return
		}
		
		// Update visit count
		visits := 1
		if v, ok := session.GetInt(c, "visits"); ok {
			visits = v + 1
		}
		session.Set(c, "visits", visits)
		
		c.JSON(http.StatusOK, gin.H{
			"message": fmt.Sprintf("Visit count: %d", visits),
			"visits":  visits,
		})
	})
	
	// Endpoint to refresh the token
	server.Route().POST("/refresh", func(c *gin.Context) {
		// This will force a token refresh
		sessionManager.Regenerate(c)
		
		// Get the refreshed session
		sess := session.Get(c)
		
		// Return token
		c.JSON(http.StatusOK, gin.H{
			"message": "Token refreshed",
			"token":   sess.Values["_jwt_token"],
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

	// Start HTTP server
	logger.Info(fmt.Sprintf("Running JWT session demo at http://%s:%d", srvConfig.Host, srvConfig.Port))
	server.Start()
}