package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/oddbit-project/blueprint/log"
	"github.com/oddbit-project/blueprint/provider/httpserver"
	"github.com/oddbit-project/blueprint/provider/httpserver/session"
	"github.com/oddbit-project/blueprint/provider/kv"
	"net/http"
	"os"
	"time"
)

func main() {
	// Configure logger
	log.Configure(log.NewDefaultConfig())
	logger := log.New("session-sample")

	// Create server config
	srvConfig := httpserver.NewServerConfig()
	srvConfig.Host = "localhost"
	srvConfig.Port = 8089
	srvConfig.Debug = true

	// Create HTTP server
	server, err := httpserver.NewServer(srvConfig, logger)
	if err != nil {
		logger.Fatal(err, "could not start http server")
		os.Exit(1)
	}

	// Configure session
	sessionConfig := session.NewConfig()

	// Set cookie properties for demonstration
	sessionConfig.CookieName = "blueprint_sample_session"
	sessionConfig.ExpirationSeconds = 1800 // 30min
	sessionConfig.IdleTimeoutSeconds = 900 // 15min
	sessionConfig.Secure = false           // For local testing

	// session backend
	backend := kv.NewMemoryKV()

	// Use session middleware with memory store
	sessionManager := server.UseSession(sessionConfig, backend, logger)

	// Define routes
	// Home page - shows session info
	server.Route().GET("/", func(c *gin.Context) {
		// Get session
		sess := session.Get(c)

		// Get visit count
		visits := 1
		if v, ok := session.GetInt(c, "visits"); ok {
			visits = v + 1
		}

		// Update visit count
		session.Set(c, "visits", visits)

		// Get last visit time
		var lastVisit string
		if v, ok := session.GetString(c, "lastVisit"); ok {
			lastVisit = v
		} else {
			lastVisit = "First visit"
		}

		// Update last visit time
		currentTime := time.Now().Format(time.RFC1123)
		session.Set(c, "lastVisit", currentTime)

		// Check for flash messages
		var flashMessage string
		if msg, ok := session.GetFlashString(c, "message"); ok {
			flashMessage = msg
		}

		// Render response
		c.HTML(http.StatusOK, "index.html", gin.H{
			"sessionID": sess.ID,
			"visits":    visits,
			"lastVisit": lastVisit,
			"created":   sess.Created.Format(time.RFC1123),
			"flash":     flashMessage,
		})
	})

	// Reset session - demonstrates session regeneration
	server.Route().GET("/reset", func(c *gin.Context) {
		// Regenerate session
		sessionManager.Regenerate(c)

		// Set a flash message
		session.FlashString(c, "message", "Session has been reset")

		// Redirect to home
		c.Redirect(http.StatusFound, "/")
	})

	// Clear session - demonstrates session clearing
	server.Route().GET("/clear", func(c *gin.Context) {
		// Clear session
		sessionManager.Clear(c)

		// Redirect to home
		c.Redirect(http.StatusFound, "/")
	})

	// Set flash message - demonstrates flash messages
	server.Route().GET("/flash", func(c *gin.Context) {
		// Set a flash message
		session.FlashString(c, "message", "This is a flash message that will disappear after being viewed")

		// Redirect to home
		c.Redirect(http.StatusFound, "/")
	})

	// Load HTML templates
	server.Router.LoadHTMLGlob("./templates/*")

	// Start HTTP server
	logger.Info(fmt.Sprintf("Running session demo at http://%s:%d", srvConfig.Host, srvConfig.Port))
	server.Start()
}
