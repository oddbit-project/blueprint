package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/oddbit-project/blueprint/log"
	"github.com/oddbit-project/blueprint/provider/httpserver"
	"github.com/oddbit-project/blueprint/provider/httpserver/security"
	"github.com/oddbit-project/blueprint/provider/httpserver/session"
	"net/http"
	"os"
)

func main() {
	// Configure logger
	log.Configure(log.NewDefaultConfig())
	logger := log.New("csrf-demo-server")

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

	// Set up session management (required for CSRF protection)
	sessionConfig := session.NewConfig()
	_, err = server.UseSession(sessionConfig, nil, logger)
	if err != nil {
		logger.Fatal(err, "could not initialize session provider")
		os.Exit(1)
	}

	// Apply CSRF protection to all POST/PUT/DELETE routes
	server.Route().Use(security.CSRFProtection())

	// GET endpoint to initialize session and get CSRF token
	server.Route().GET("/", func(c *gin.Context) {
		// Get or create session
		sess := session.Get(c)
		if sess == nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Could not initialize session",
			})
			return
		}

		// Generate CSRF token
		csrfToken := security.GenerateCSRFToken(c)
		sess.Set("_csrf", csrfToken)

		c.JSON(http.StatusOK, gin.H{
			"message":    "CSRF Demo Server",
			"csrf_token": csrfToken,
			"instructions": map[string]string{
				"protected_form": "POST /submit with X-CSRF-Token header or _csrf form field",
				"protected_api":  "POST /api/data with X-CSRF-Token header",
				"public":         "GET /public (no CSRF protection)",
			},
		})
	})

	// GET endpoint for HTML form demo
	server.Route().GET("/form", func(c *gin.Context) {
		sess := session.Get(c)
		if sess == nil {
			c.String(http.StatusInternalServerError, "Could not get session")
			return
		}

		// Generate CSRF token if not exists
		csrfToken, exists := sess.GetString("_csrf")
		if !exists {
			csrfToken = security.GenerateCSRFToken(c)
			sess.Set("_csrf", csrfToken)
		}

		htmlForm := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <title>CSRF Protection Demo</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 40px; }
        .form-group { margin: 15px 0; }
        label { display: block; margin-bottom: 5px; }
        input, textarea { width: 300px; padding: 8px; }
        button { padding: 10px 20px; background: #007cba; color: white; border: none; cursor: pointer; }
        .token { background: #f0f0f0; padding: 10px; margin: 10px 0; font-family: monospace; word-break: break-all; }
        .section { margin: 30px 0; padding: 20px; border: 1px solid #ddd; }
    </style>
</head>
<body>
    <h1>CSRF Protection Demo</h1>
    
    <div class="section">
        <h2>Current CSRF Token</h2>
        <div class="token">%s</div>
    </div>

    <div class="section">
        <h2>Protected Form (with CSRF token)</h2>
        <form action="/submit" method="post">
            <input type="hidden" name="_csrf" value="%s">
            <div class="form-group">
                <label>Name:</label>
                <input type="text" name="name" required>
            </div>
            <div class="form-group">
                <label>Message:</label>
                <textarea name="message" rows="4" required></textarea>
            </div>
            <button type="submit">Submit (Should work)</button>
        </form>
    </div>

    <div class="section">
        <h2>Unprotected Form (missing CSRF token)</h2>
        <form action="/submit" method="post">
            <div class="form-group">
                <label>Name:</label>
                <input type="text" name="name" required>
            </div>
            <div class="form-group">
                <label>Message:</label>
                <textarea name="message" rows="4" required></textarea>
            </div>
            <button type="submit">Submit (Should fail)</button>
        </form>
    </div>

    <div class="section">
        <h2>JavaScript API Example</h2>
        <button onclick="testAPI(true)">Test API with CSRF token</button>
        <button onclick="testAPI(false)">Test API without CSRF token</button>
        
        <script>
        function testAPI(includeToken) {
            const headers = {
                'Content-Type': 'application/json'
            };
            
            if (includeToken) {
                headers['X-CSRF-Token'] = '%s';
            }
            
            fetch('/api/data', {
                method: 'POST',
                headers: headers,
                body: JSON.stringify({
                    data: 'test data',
                    timestamp: new Date().toISOString()
                })
            })
            .then(response => response.json())
            .then(data => {
                alert('Response: ' + JSON.stringify(data, null, 2));
            })
            .catch(error => {
                alert('Error: ' + error);
            });
        }
        </script>
    </div>
</body>
</html>`, csrfToken, csrfToken, csrfToken)

		c.Header("Content-Type", "text/html")
		c.String(http.StatusOK, htmlForm)
	})

	// Public endpoint - no CSRF protection needed for GET requests
	server.Route().GET("/public", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "This is a public endpoint",
			"time":    fmt.Sprintf("%v", c.Request.Header.Get("X-Requested-With")),
		})
	})

	// Protected form submission endpoint
	server.Route().POST("/submit", func(c *gin.Context) {
		name := c.PostForm("name")
		message := c.PostForm("message")

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Form submitted successfully!",
			"data": gin.H{
				"name":    name,
				"message": message,
			},
		})
	})

	// Protected API endpoint
	server.Route().POST("/api/data", func(c *gin.Context) {
		var requestData map[string]interface{}
		if err := c.ShouldBindJSON(&requestData); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "Invalid JSON data",
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success":       true,
			"message":       "API call successful!",
			"received_data": requestData,
		})
	})

	// Another protected endpoint to test different HTTP methods
	server.Route().PUT("/api/update", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Update successful!",
		})
	})

	server.Route().DELETE("/api/delete", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Delete successful!",
		})
	})

	fmt.Println("CSRF Demo Server running at http://localhost:8089")
	fmt.Println("Available endpoints:")
	fmt.Println("  - GET /           - Get CSRF token and instructions")
	fmt.Println("  - GET /form       - HTML form demo")
	fmt.Println("  - GET /public     - Public endpoint (no CSRF)")
	fmt.Println("  - POST /submit    - Protected form endpoint")
	fmt.Println("  - POST /api/data  - Protected API endpoint")
	fmt.Println("  - PUT /api/update - Protected update endpoint")
	fmt.Println("  - DELETE /api/delete - Protected delete endpoint")

	// Start the HTTP server
	err = server.Start()
	if err != nil {
		logger.Fatal(err, "could not start server")
		os.Exit(1)
	}

	fmt.Println("Done!")
}
