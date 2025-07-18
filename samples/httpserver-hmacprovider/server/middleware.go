package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/oddbit-project/blueprint/log"
)

// HMACRequestLogger logs detailed information about HMAC requests
func HMACRequestLogger(logger *log.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// Process request
		c.Next()

		// Log request details
		latency := time.Since(start)
		statusCode := c.Writer.Status()

		logFields := log.KV{
			"method":     c.Request.Method,
			"path":       c.Request.URL.Path,
			"status":     statusCode,
			"latency":    latency.String(),
			"client_ip":  c.ClientIP(),
			"user_agent": c.Request.UserAgent(),
		}

		// Add authentication details if present
		if authenticated, exists := c.Get("authenticated"); exists && authenticated.(bool) {
			if timestamp, exists := c.Get("auth_timestamp"); exists {
				logFields["auth_timestamp"] = timestamp
			}
		}

		// Log based on status code
		if statusCode >= 500 {
			logger.Error(fmt.Errorf("server error status %d", statusCode), "HTTP request completed with server error", logFields)
		} else if statusCode >= 400 {
			logger.Warn("HTTP request completed with client error", logFields)
		} else {
			logger.Info("HTTP request completed successfully", logFields)
		}
	}
}

// ErrorHandler handles panics and returns proper error responses
func ErrorHandler(logger *log.Logger) gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered interface{}) {
		logger.Error(fmt.Errorf("panic: %v", recovered), "Panic recovered", log.KV{
			"path":      c.Request.URL.Path,
			"method":    c.Request.Method,
			"client_ip": c.ClientIP(),
		})

		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Internal server error",
		})
	})
}
