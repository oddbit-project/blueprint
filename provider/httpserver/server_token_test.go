package httpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/oddbit-project/blueprint/log"
	"github.com/stretchr/testify/assert"
)

func TestServer_ProcessOptions_AuthToken(t *testing.T) {
	// Setup
	gin.SetMode(gin.TestMode)
	logger := log.New("test")

	tests := []struct {
		name            string
		options         map[string]string
		requestHeaders  map[string]string
		expectedStatus  int
		testEndpoint    string
	}{
		{
			name: "Valid auth token",
			options: map[string]string{
				OptAuthTokenHeader: "X-Test-Auth",
				OptAuthTokenSecret: "test-secret",
			},
			requestHeaders: map[string]string{
				"X-Test-Auth": "test-secret",
			},
			expectedStatus: http.StatusOK,
			testEndpoint:   "/test",
		},
		{
			name: "Invalid auth token",
			options: map[string]string{
				OptAuthTokenHeader: "X-Test-Auth",
				OptAuthTokenSecret: "test-secret",
			},
			requestHeaders: map[string]string{
				"X-Test-Auth": "wrong-secret",
			},
			expectedStatus: http.StatusUnauthorized,
			testEndpoint:   "/test",
		},
		{
			name: "Missing auth token",
			options: map[string]string{
				OptAuthTokenHeader: "X-Test-Auth",
				OptAuthTokenSecret: "test-secret",
			},
			requestHeaders:  map[string]string{},
			expectedStatus:  http.StatusUnauthorized,
			testEndpoint:    "/test",
		},
		{
			name: "Default header name",
			options: map[string]string{
				OptAuthTokenSecret: "test-secret",
			},
			requestHeaders: map[string]string{
				"X-Auth-Key": "test-secret",
			},
			expectedStatus: http.StatusOK,
			testEndpoint:   "/test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create server config with test options
			config := NewServerConfig()
			config.Options = tt.options

			// Create server
			server, err := NewServer(config, logger)
			assert.NoError(t, err)

			// Process options to set up auth middleware
			err = server.ProcessOptions()
			assert.NoError(t, err)

			// Add a test endpoint
			server.Router.GET("/test", func(c *gin.Context) {
				c.Status(http.StatusOK)
			})

			// Create test request with headers
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", tt.testEndpoint, nil)
			for k, v := range tt.requestHeaders {
				req.Header.Set(k, v)
			}

			// Process the request
			server.Router.ServeHTTP(w, req)

			// Check status code
			assert.Equal(t, tt.expectedStatus, w.Code)
		})
	}
}

func TestServer_ProcessOptions_NoAuthToken(t *testing.T) {
	// Setup
	gin.SetMode(gin.TestMode)
	logger := log.New("test")

	// Create server config without auth token options
	config := NewServerConfig()

	// Create server
	server, err := NewServer(config, logger)
	assert.NoError(t, err)

	// Process options
	err = server.ProcessOptions()
	assert.NoError(t, err)

	// Add a test endpoint
	server.Router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// Create test request with no headers
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)

	// Process the request - should allow access without token
	server.Router.ServeHTTP(w, req)

	// Check status code - should be OK since no auth is configured
	assert.Equal(t, http.StatusOK, w.Code)
}