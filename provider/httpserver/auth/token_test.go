package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestNewAuthToken(t *testing.T) {
	tests := []struct {
		name       string
		headerName string
		key        string
		wantHeader string
		wantKey    string
	}{
		{
			name:       "Default header name",
			headerName: DefaultTokenAuthHeader,
			key:        "secret-key",
			wantHeader: DefaultTokenAuthHeader,
			wantKey:    "secret-key",
		},
		{
			name:       "Custom header name",
			headerName: "X-Custom-VerifyUser",
			key:        "another-secret",
			wantHeader: "X-Custom-VerifyUser",
			wantKey:    "another-secret",
		},
		{
			name:       "Empty key",
			headerName: "X-API-Key",
			key:        "",
			wantHeader: "X-API-Key",
			wantKey:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewAuthToken(tt.headerName, tt.key)

			// Type assertion to access private fields for testing
			authTokenProvider, ok := provider.(*authToken)
			assert.True(t, ok, "Expected *authToken type")

			assert.Equal(t, tt.wantHeader, authTokenProvider.headerName)
			assert.Equal(t, tt.wantKey, authTokenProvider.key)
		})
	}
}

func TestAuthToken_CanAccess(t *testing.T) {
	tests := []struct {
		name       string
		headerName string
		key        string
		reqHeader  string
		reqValue   string
		wantAccess bool
	}{
		{
			name:       "Valid token",
			headerName: "X-API-Key",
			key:        "secret-token",
			reqHeader:  "X-API-Key",
			reqValue:   "secret-token",
			wantAccess: true,
		},
		{
			name:       "Invalid token",
			headerName: "X-API-Key",
			key:        "secret-token",
			reqHeader:  "X-API-Key",
			reqValue:   "wrong-token",
			wantAccess: false,
		},
		{
			name:       "Missing token",
			headerName: "X-API-Key",
			key:        "secret-token",
			reqHeader:  "",
			reqValue:   "",
			wantAccess: false,
		},
		{
			name:       "Wrong header name",
			headerName: "X-API-Key",
			key:        "secret-token",
			reqHeader:  "X-VerifyUser-Token",
			reqValue:   "secret-token",
			wantAccess: false,
		},
		{
			name:       "Empty key always allows access",
			headerName: "X-API-Key",
			key:        "",
			reqHeader:  "",
			reqValue:   "",
			wantAccess: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up test context
			gin.SetMode(gin.TestMode)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			// Create request with headers
			req, _ := http.NewRequest("GET", "/test", nil)
			if tt.reqHeader != "" {
				req.Header.Set(tt.reqHeader, tt.reqValue)
			}
			c.Request = req

			// Create token provider and test
			provider := NewAuthToken(tt.headerName, tt.key)
			gotAccess := provider.CanAccess(c)

			assert.Equal(t, tt.wantAccess, gotAccess)
		})
	}
}

func TestAuthMiddleware(t *testing.T) {
	tests := []struct {
		name       string
		setup      func() (Provider, *gin.Context, *httptest.ResponseRecorder)
		wantStatus int
		wantCalled bool
	}{
		{
			name: "Access granted",
			setup: func() (Provider, *gin.Context, *httptest.ResponseRecorder) {
				w := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(w)
				req, _ := http.NewRequest("GET", "/test", nil)
				req.Header.Set("X-API-Key", "valid-token")
				c.Request = req

				provider := NewAuthToken("X-API-Key", "valid-token")
				return provider, c, w
			},
			wantStatus: http.StatusOK,
			wantCalled: true,
		},
		{
			name: "Access denied",
			setup: func() (Provider, *gin.Context, *httptest.ResponseRecorder) {
				w := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(w)
				req, _ := http.NewRequest("GET", "/test", nil)
				req.Header.Set("X-API-Key", "invalid-token")
				c.Request = req

				provider := NewAuthToken("X-API-Key", "valid-token")
				return provider, c, w
			},
			wantStatus: http.StatusUnauthorized,
			wantCalled: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			provider, c, w := tt.setup()

			// Set up router for testing middleware
			router := gin.New()

			// Add the middleware
			router.Use(AuthMiddleware(provider))

			// Add a handler that records it was called
			var handlerCalled bool
			router.GET("/test", func(c *gin.Context) {
				handlerCalled = true
				c.Status(http.StatusOK)
			})

			// Serve the request through the router
			router.ServeHTTP(w, c.Request)

			// Check results
			assert.Equal(t, tt.wantStatus, w.Code)
			assert.Equal(t, tt.wantCalled, handlerCalled)
		})
	}
}
