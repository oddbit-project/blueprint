package log

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/oddbit-project/blueprint/log"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"testing"
)

func setupTestHTTPRouter(t *testing.T) (*gin.Engine, *bytes.Buffer) {
	gin.SetMode(gin.TestMode)

	buf := &bytes.Buffer{}
	zerolog.TimeFieldFormat = ""
	zerolog.SetGlobalLevel(zerolog.DebugLevel)

	// Create a test router
	router := gin.New()

	// Override the global logger to write to our buffer
	logger := log.New("test-http").WithOutput(buf)

	// Use our middleware with the test logger
	router.Use(func(c *gin.Context) {
		// Use a custom middleware that places our test logger in the context
		ctx := logger.WithContext(c.Request.Context())
		c.Request = c.Request.WithContext(ctx)
		c.Set("trace_id", "test-trace-id")
		c.Set("request_id", "test-request-id")
		c.Next()
	})

	return router, buf
}

func TestHTTPLogMiddleware(t *testing.T) {
	// Set up a test router with the middleware
	gin.SetMode(gin.TestMode)
	router := gin.New()
	logger := log.New("test-http")
	router.Use(HTTPLogMiddleware(logger))

	// Add a test handler
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// Create a test request
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	// Serve the request
	router.ServeHTTP(w, req)

	// Check response
	assert.Equal(t, http.StatusOK, w.Code)

	// Verify headers
	assert.NotEmpty(t, w.Header().Get(HeaderRequestID))
	assert.NotEmpty(t, w.Header().Get(HeaderTraceID))
}

func TestGetRequestLogger(t *testing.T) {
	router, _ := setupTestHTTPRouter(t)

	// Add test handler
	router.GET("/logger", func(c *gin.Context) {
		logger := GetRequestLogger(c)
		assert.NotNil(t, logger)
		assert.Equal(t, "test-http", logger.ModuleInfo())
		c.Status(http.StatusOK)
	})

	// Perform request
	req := httptest.NewRequest("GET", "/logger", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetRequestTraceID(t *testing.T) {
	router, _ := setupTestHTTPRouter(t)

	// Add test handler
	router.GET("/trace-id", func(c *gin.Context) {
		traceID := GetRequestTraceID(c)
		assert.Equal(t, "test-trace-id", traceID)
		c.Status(http.StatusOK)
	})

	// Perform request
	req := httptest.NewRequest("GET", "/trace-id", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetRequestID(t *testing.T) {
	router, _ := setupTestHTTPRouter(t)

	// Add test handler
	router.GET("/request-id", func(c *gin.Context) {
		requestID := GetRequestID(c)
		assert.Equal(t, "test-request-id", requestID)
		c.Status(http.StatusOK)
	})

	// Perform request
	req := httptest.NewRequest("GET", "/request-id", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRequestLoggingFunctions(t *testing.T) {
	router, buf := setupTestHTTPRouter(t)

	// Test RequestDebug
	router.GET("/debug", func(c *gin.Context) {
		RequestDebug(c, "debug message", map[string]interface{}{
			"test_key": "test_value",
		})
		c.Status(http.StatusOK)
	})

	// Test RequestInfo
	router.GET("/info", func(c *gin.Context) {
		RequestInfo(c, "info message")
		c.Status(http.StatusOK)
	})

	// Test RequestWarn
	router.GET("/warn", func(c *gin.Context) {
		RequestWarn(c, "warn message")
		c.Status(http.StatusOK)
	})

	// Test RequestError
	router.GET("/error", func(c *gin.Context) {
		RequestError(c, errors.New("test error"), "error message")
		c.Status(http.StatusInternalServerError)
	})

	// Test all routes and check logs
	testCases := []struct {
		path     string
		level    string
		message  string
		hasError bool
	}{
		{"/debug", "debug", "debug message", false},
		{"/info", "info", "info message", false},
		{"/warn", "warn", "warn message", false},
		{"/error", "error", "error message", true},
	}

	for _, tc := range testCases {
		buf.Reset() // Clear buffer before each test

		// Perform request
		req := httptest.NewRequest("GET", tc.path, nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Parse log
		logMap := map[string]interface{}{}
		err := json.Unmarshal(buf.Bytes(), &logMap)
		assert.NoError(t, err)

		// Check log
		assert.Equal(t, tc.level, logMap["level"])
		assert.Equal(t, tc.message, logMap["message"])

		if tc.hasError {
			assert.Equal(t, "test error", logMap["error"])
		}

		if tc.path == "/debug" {
			assert.Equal(t, "test_value", logMap["test_key"])
		}
	}
}
