package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestNewAuthTokenList(t *testing.T) {
	tests := []struct {
		name       string
		headerName string
		keyList    []string
		wantHeader string
		wantKey    string
	}{
		{
			name:       "Default header name",
			headerName: DefaultTokenAuthHeader,
			keyList:    []string{"abc", "secret-key"},
			wantHeader: DefaultTokenAuthHeader,
			wantKey:    "secret-key",
		},
		{
			name:       "Custom header name",
			headerName: "X-Custom-VerifyUser",
			keyList:    []string{"another-secret"},
			wantHeader: "X-Custom-VerifyUser",
			wantKey:    "another-secret",
		},
		{
			name:       "Empty key",
			headerName: "X-API-Key",
			keyList:    []string{},
			wantHeader: "X-API-Key",
			wantKey:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewAuthTokenList(tt.headerName, tt.keyList)

			// Type assertion to access private fields for testing
			authTokenProvider, ok := provider.(*authTokenList)
			assert.True(t, ok, "Expected *authToken type")

			assert.Equal(t, tt.wantHeader, authTokenProvider.headerName)
		})
	}
}

func TestAuthTokenList_CanAccess(t *testing.T) {
	tests := []struct {
		name       string
		headerName string
		keyList    []string
		reqHeader  string
		reqValue   string
		wantAccess bool
	}{
		{
			name:       "Valid token",
			headerName: "X-API-Key",
			keyList:    []string{"saa", "secret-token"},
			reqHeader:  "X-API-Key",
			reqValue:   "secret-token",
			wantAccess: true,
		},
		{
			name:       "Invalid token",
			headerName: "X-API-Key",
			keyList:    []string{"secret-token"},
			reqHeader:  "X-API-Key",
			reqValue:   "wrong-token",
			wantAccess: false,
		},
		{
			name:       "Missing token",
			headerName: "X-API-Key",
			keyList:    []string{"def", "secret-token", "abcde"},
			reqHeader:  "",
			reqValue:   "",
			wantAccess: false,
		},
		{
			name:       "Wrong header name",
			headerName: "X-API-Key",
			keyList:    []string{"secret-token", "abc"},
			reqHeader:  "X-VerifyUser-Token",
			reqValue:   "secret-token",
			wantAccess: false,
		},
		{
			name:       "Empty key always allows access",
			headerName: "X-API-Key",
			keyList:    []string{},
			reqHeader:  "",
			reqValue:   "",
			wantAccess: true,
		},
		{
			name:       "Slice with empty key always allows access",
			headerName: "X-API-Key",
			keyList:    []string{""},
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
			provider := NewAuthTokenList(tt.headerName, tt.keyList)
			gotAccess := provider.CanAccess(c)

			assert.Equal(t, tt.wantAccess, gotAccess)
		})
	}
}

func TestAuthListMiddleware(t *testing.T) {
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

				provider := NewAuthTokenList("X-API-Key", []string{"one", "valid-token", "two"})
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

				provider := NewAuthTokenList("X-API-Key", []string{"valid-token"})
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
