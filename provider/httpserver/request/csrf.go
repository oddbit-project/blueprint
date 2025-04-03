package request

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// CSRFMiddleware implements CSRF protection middleware
func CSRFMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip CSRF check for GET, HEAD, OPTIONS
		if c.Request.Method == "GET" ||
			c.Request.Method == "HEAD" ||
			c.Request.Method == "OPTIONS" {
			c.Next()
			return
		}

		// Check CSRF token in header or form
		token := c.GetHeader("X-CSRF-Token")
		if token == "" {
			token = c.PostForm("_csrf")
		}

		// Validate token (in a real implementation, this would validate against a stored token)
		expected := c.GetString("csrf-token")
		if token == "" || token != expected {
			c.AbortWithStatusJSON(403, gin.H{
				"success": false,
				"error":   "CSRF token validation failed",
			})
			return
		}

		c.Next()
	}
}

// GenerateCSRFToken generates a CSRF token for the current session
func GenerateCSRFToken(c *gin.Context) string {
	token := uuid.New().String()
	c.Set("csrf-token", token)
	return token
}
