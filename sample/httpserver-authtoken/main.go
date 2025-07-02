package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/oddbit-project/blueprint/log"
	"github.com/oddbit-project/blueprint/provider/httpserver"
	"github.com/oddbit-project/blueprint/provider/httpserver/auth"
	"net/http"
	"os"
)

func main() {
	// Configure logger
	log.Configure(log.NewDefaultConfig())
	logger := log.New("auth-token-server")

	// Create server configuration
	srvConfig := httpserver.NewServerConfig()
	srvConfig.Host = "localhost"
	srvConfig.Port = 8089
	srvConfig.Debug = true

	// Initialize the server
	server, err := httpserver.NewServer(srvConfig, logger)
	if err != nil {
		logger.Fatal(err, "could not start http server")
		os.Exit(1)
	}

	// Don't use ProcessOptions() as it applies auth globally
	// Instead, create auth middleware manually and apply it only to protected routes
	authToken := auth.NewAuthToken("X-API-Key", "secret-token-value")

	// Public endpoint - no authentication required
	server.Route().GET("/public", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "This is a public endpoint, no authentication required!",
		})
	})

	// Create a protected group that will use the auth token middleware
	protected := server.Route().Group("/protected")
	protected.Use(auth.AuthMiddleware(authToken))

	// Protected endpoints - require valid token
	protected.GET("/resource", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "You've accessed a protected resource!",
			"data":    "Secret information",
		})
	})

	fmt.Println("Server running at http://localhost:8089")
	fmt.Println("Try these endpoints:")
	fmt.Println("  - GET /public (no auth required)")
	fmt.Println("  - GET /protected/resource (requires X-API-Key: secret-token-value header)")

	// Start the HTTP server
	err = server.Start()
	if err != nil {
		logger.Fatal(err, "could not start server")
		os.Exit(1)
	}

	fmt.Println("Done!")
}
