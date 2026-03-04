package log

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/oddbit-project/blueprint/log"
	"time"
)

const (
	// HTTP request tracing headers
	HeaderRequestID = "X-Request-ID"
	HeaderTraceID   = "X-Trace-ID"

	// Context keys for request tracing
	ContextTraceID   = "trace_id"
	ContextRequestID = "request_id"
)

func NewHTTPLogger(moduleName string) *log.Logger {
	return log.New(moduleName)
}

// HTTPLogMiddleware is a middleware for logging HTTP requests
func HTTPLogMiddleware(logger *log.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get or generate request ID
		requestID := c.GetHeader(HeaderRequestID)
		if requestID == "" {
			requestID = uuid.New().String()
			c.Header(HeaderRequestID, requestID)
		}

		// Get or generate trace ID
		traceID := c.GetHeader(HeaderTraceID)
		if traceID == "" {
			traceID = uuid.New().String()
			c.Header(HeaderTraceID, traceID)
		}

		// only create logger if not available
		if logger == nil {
			logger = NewHTTPLogger("server")
		}

		reqLogger := logger.WithTraceID(traceID).
			WithField("request_id", requestID).
			WithField("method", c.Request.Method).
			WithField("path", c.Request.URL.Path).
			WithField("client_ip", c.ClientIP()).
			WithField("user_agent", c.Request.UserAgent())

		// Store logger in context
		ctx := reqLogger.WithContext(c.Request.Context())
		c.Request = c.Request.WithContext(ctx)

		// Store trace ID in Gin context for easy access
		c.Set(ContextTraceID, traceID)
		c.Set(ContextRequestID, requestID)

		// Start timer
		start := time.Now()

		// Process request
		c.Next()

		// Calculate latency
		latency := time.Since(start)

		// Log request completion
		statusCode := c.Writer.Status()

		// Log the request with different log levels based on status code
		fields := map[string]interface{}{
			"status":     statusCode,
			"latency_ms": latency.Milliseconds(),
			"latency":    latency.String(),
			"bytes":      c.Writer.Size(),
			"errors":     c.Errors.String(),
			"request_id": requestID,
			"trace_id":   traceID,
		}

		msg := c.Request.Method + " " + c.Request.URL.Path

		if len(c.Errors) > 0 {
			reqLogger.Error(nil, msg, fields)
		} else if statusCode >= 500 {
			reqLogger.Error(nil, msg, fields)
		} else if statusCode >= 400 {
			reqLogger.Warn(msg, fields)
		} else {
			reqLogger.Info(msg, fields)
		}
	}
}

// GetRequestLogger retrieves the logger from the gin.Context
func GetRequestLogger(c *gin.Context) *log.Logger {
	ctx := c.Request.Context()
	return log.FromContext(ctx)
}

// GetRequestTraceID retrieves the trace ID from the gin.Context
func GetRequestTraceID(c *gin.Context) string {
	if traceID, exists := c.Get(ContextTraceID); exists {
		if id, ok := traceID.(string); ok {
			return id
		}
	}
	return ""
}

// GetRequestID retrieves the request ID from the gin.Context
func GetRequestID(c *gin.Context) string {
	if requestID, exists := c.Get(ContextRequestID); exists {
		if id, ok := requestID.(string); ok {
			return id
		}
	}
	return ""
}

// RequestDebug logs a debug message for the current request
func RequestDebug(c *gin.Context, msg string, fields ...map[string]interface{}) {
	logger := GetRequestLogger(c)
	if logger == nil {
		logger = log.New("http")
	}

	if len(fields) > 0 {
		logger.Debug(msg, fields[0])
	} else {
		logger.Debug(msg)
	}
}

// RequestInfo logs an info message for the current request
func RequestInfo(c *gin.Context, msg string, fields ...map[string]interface{}) {
	logger := GetRequestLogger(c)
	if logger == nil {
		logger = log.New("http")
	}

	if len(fields) > 0 {
		logger.Info(msg, fields[0])
	} else {
		logger.Info(msg)
	}
}

// RequestWarn logs a warning message for the current request
func RequestWarn(c *gin.Context, msg string, fields ...map[string]interface{}) {
	logger := GetRequestLogger(c)
	if logger == nil {
		logger = log.New("http")
	}

	if len(fields) > 0 {
		logger.Warn(msg, fields[0])
	} else {
		logger.Warn(msg)
	}
}

// RequestError logs an error message for the current request
func RequestError(c *gin.Context, err error, msg string, fields ...map[string]interface{}) {
	logger := GetRequestLogger(c)
	if logger == nil {
		logger = log.New("http")
	}

	if len(fields) > 0 {
		logger.Error(err, msg, fields[0])
	} else {
		logger.Error(err, msg)
	}
}
