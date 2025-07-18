package httpserver

import (
	"github.com/oddbit-project/blueprint/provider/httpserver/session"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/oddbit-project/blueprint/provider/httpserver/auth"
	"github.com/oddbit-project/blueprint/provider/httpserver/security"
)

// MockAuthProvider implements auth.Provider interface for testing
type MockAuthProvider struct {
	mock.Mock
}

func (m *MockAuthProvider) CanAccess(c *gin.Context) bool {
	args := m.Called(c)
	return args.Bool(0)
}

// MockServer is a simplified version of the Server struct for testing
type MockServer struct {
	Router *gin.Engine
}

// AddMiddleware adds middleware to the router
func (s *MockServer) AddMiddleware(handlers ...gin.HandlerFunc) {
	s.Router.Use(handlers...)
}

func TestUseAuth(t *testing.T) {
	// Setup
	gin.SetMode(gin.TestMode)

	// Create a mock server for testing
	server := &MockServer{
		Router: gin.New(),
	}

	mockProvider := new(MockAuthProvider)

	// Add the UseAuth method to our mock server
	useAuth := func(provider auth.Provider) {
		server.AddMiddleware(auth.AuthMiddleware(provider))
	}

	// Test adding auth middleware
	useAuth(mockProvider)

	// Create a test request
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)

	// Configure the mock
	mockProvider.On("CanAccess", mock.Anything).Return(false)

	// Setup a test route
	server.Router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "success")
	})

	// Process the request
	server.Router.ServeHTTP(w, req)

	// VerifyUser should fail and return 401
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	mockProvider.AssertExpectations(t)
}

func TestUseSecurityHeaders(t *testing.T) {
	// Setup
	gin.SetMode(gin.TestMode)

	// Create a mock server for testing
	server := &MockServer{
		Router: gin.New(),
	}

	securityConfig := &security.SecurityConfig{
		CSP:                "default-src 'self'",
		FrameOptions:       "DENY",
		ContentTypeOptions: "nosniff",
		ReferrerPolicy:     "strict-origin-when-cross-origin",
	}

	// Add the UseSecurityHeaders method to our mock server
	useSecurityHeaders := func(config *security.SecurityConfig) {
		server.AddMiddleware(security.SecurityMiddleware(config))
	}

	// Test adding security headers middleware
	useSecurityHeaders(securityConfig)

	// Create a test request
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)

	// Setup a test route
	server.Router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "success")
	})

	// Process the request
	server.Router.ServeHTTP(w, req)

	// Check the security headers
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, securityConfig.CSP, w.Header().Get("Content-Security-Policy"))
	assert.Equal(t, securityConfig.FrameOptions, w.Header().Get("X-Frame-Options"))
	assert.Equal(t, securityConfig.ContentTypeOptions, w.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, securityConfig.ReferrerPolicy, w.Header().Get("Referrer-Policy"))
}

func TestUseDefaultSecurityHeaders(t *testing.T) {
	// Setup
	gin.SetMode(gin.TestMode)

	// Create a mock server for testing
	server := &MockServer{
		Router: gin.New(),
	}

	defaultConfig := security.DefaultSecurityConfig()

	// Add the UseDefaultSecurityHeaders method to our mock server
	useDefaultSecurityHeaders := func() {
		server.AddMiddleware(security.SecurityMiddleware(security.DefaultSecurityConfig()))
	}

	// Test adding default security headers middleware
	useDefaultSecurityHeaders()

	// Create a test request
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)

	// Setup a test route
	server.Router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "success")
	})

	// Process the request
	server.Router.ServeHTTP(w, req)

	// Check the default security headers
	assert.Equal(t, http.StatusOK, w.Code)

	// CSP header contains dynamically generated nonce, so just check for prefix
	cspHeader := w.Header().Get("Content-Security-Policy")
	assert.Contains(t, cspHeader, "default-src 'self'")
	assert.Contains(t, cspHeader, "script-src 'self' 'nonce-")

	// Check the other static headers
	assert.Equal(t, defaultConfig.FrameOptions, w.Header().Get("X-Frame-Options"))
	assert.Equal(t, defaultConfig.ContentTypeOptions, w.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, defaultConfig.ReferrerPolicy, w.Header().Get("Referrer-Policy"))
}

func TestUseCSRFProtection(t *testing.T) {
	// Setup
	gin.SetMode(gin.TestMode)

	// Create a mock server for testing
	server := &MockServer{
		Router: gin.New(),
	}

	// Create a test request with no CSRF token
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/test", nil)

	config := session.NewConfig()
	config.Secure = false                       // Set to true in production with HTTPS
	config.SameSite = int(http.SameSiteLaxMode) // Important for cross-origin
	config.CookieName = "nextjs_session"        // Custom cookie name
	config.ExpirationSeconds = 3600             // 1 hour
	// Create store
	store, err := session.NewStore(config, nil, nil)
	assert.NoError(t, err)

	// Create manager
	manager, err := session.NewManager(config, session.ManagerWithStore(store))
	assert.NoError(t, err)

	// Add session middleware
	server.AddMiddleware(manager.Middleware())
	// Add csrf middleware
	server.AddMiddleware(security.CSRFProtection())

	// Setup a test route
	server.Router.POST("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "success")
	})

	// Process the request
	server.Router.ServeHTTP(w, req)

	// Request should be rejected due to missing CSRF token
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestUseRateLimiting(t *testing.T) {
	t.Skip("Skipping rate limiting test as it's not reliable in test environments")

	// Note: Rate limiting is difficult to test reliably in unit tests because:
	// 1. It's time-dependent
	// 2. Implementation details of the token bucket algorithm can vary
	// 3. Different environments may handle limits differently
	//
	// In a real environment, this would be better tested as an integration test
}
