package auth

import (
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// mockAuthenticator is a mock implementation of backend.Authenticator for testing
type mockAuthenticator struct {
	validateFunc func(userName string, secret string) (bool, error)
}

func (m *mockAuthenticator) ValidateUser(userName string, secret string) (bool, error) {
	if m.validateFunc != nil {
		return m.validateFunc(userName, secret)
	}
	return false, nil
}

func TestNewBasicAuthProvider(t *testing.T) {
	t.Run("Valid backend with default realm", func(t *testing.T) {
		backend := &mockAuthenticator{}
		provider, err := NewBasicAuthProvider(backend)

		assert.NoError(t, err)
		assert.NotNil(t, provider)

		basicAuthProvider, ok := provider.(*BasicAuthProvider)
		assert.True(t, ok, "Expected *BasicAuthProvider type")
		assert.Equal(t, DefaultRealm, basicAuthProvider.realm)
		assert.Equal(t, backend, basicAuthProvider.backend)
	})

	t.Run("Valid backend with custom realm", func(t *testing.T) {
		backend := &mockAuthenticator{}
		provider, err := NewBasicAuthProvider(backend, WithRealm("custom-realm"))

		assert.NoError(t, err)
		assert.NotNil(t, provider)

		basicAuthProvider := provider.(*BasicAuthProvider)
		assert.Equal(t, "custom-realm", basicAuthProvider.realm)
	})

	t.Run("Nil backend returns error", func(t *testing.T) {
		provider, err := NewBasicAuthProvider(nil)

		assert.Error(t, err)
		assert.Nil(t, provider)
		assert.Contains(t, err.Error(), "authenticator backend is required")
	})

	t.Run("Multiple options applied in order", func(t *testing.T) {
		backend := &mockAuthenticator{}
		provider, err := NewBasicAuthProvider(backend,
			WithRealm("first-realm"),
			WithRealm("second-realm"),
		)

		assert.NoError(t, err)
		basicAuthProvider := provider.(*BasicAuthProvider)
		assert.Equal(t, "second-realm", basicAuthProvider.realm)
	})
}

func TestWithRealm(t *testing.T) {
	tests := []struct {
		name      string
		realm     string
		wantRealm string
	}{
		{
			name:      "Set custom realm",
			realm:     "my-realm",
			wantRealm: "my-realm",
		},
		{
			name:      "Empty realm",
			realm:     "",
			wantRealm: "",
		},
		{
			name:      "Realm with spaces",
			realm:     "My Protected Area",
			wantRealm: "My Protected Area",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backend := &mockAuthenticator{}
			provider, err := NewBasicAuthProvider(backend, WithRealm(tt.realm))

			assert.NoError(t, err)
			basicAuthProvider := provider.(*BasicAuthProvider)
			assert.Equal(t, tt.wantRealm, basicAuthProvider.realm)
		})
	}
}

func TestBasicAuthProvider_CanAccess(t *testing.T) {
	tests := []struct {
		name               string
		setupRequest       func(*http.Request)
		validateFunc       func(userName string, secret string) (bool, error)
		wantAccess         bool
		wantAuthHeader     bool
		wantAuthHeaderText string
		wantUserInContext  bool
		wantUsername       string
	}{
		{
			name: "Valid credentials",
			setupRequest: func(req *http.Request) {
				req.SetBasicAuth("testuser", "testpass")
			},
			validateFunc: func(userName string, secret string) (bool, error) {
				return userName == "testuser" && secret == "testpass", nil
			},
			wantAccess:        true,
			wantAuthHeader:    false,
			wantUserInContext: true,
			wantUsername:      "testuser",
		},
		{
			name: "Invalid credentials",
			setupRequest: func(req *http.Request) {
				req.SetBasicAuth("testuser", "wrongpass")
			},
			validateFunc: func(userName string, secret string) (bool, error) {
				return userName == "testuser" && secret == "testpass", nil
			},
			wantAccess:         false,
			wantAuthHeader:     true,
			wantAuthHeaderText: `Basic realm="restricted"`,
			wantUserInContext:  false,
		},
		{
			name: "Custom realm in header",
			setupRequest: func(req *http.Request) {
				req.SetBasicAuth("user", "pass")
			},
			validateFunc: func(userName string, secret string) (bool, error) {
				return false, nil
			},
			wantAccess:         false,
			wantAuthHeader:     true,
			wantAuthHeaderText: `Basic realm="custom-area"`,
			wantUserInContext:  false,
		},
		{
			name: "Missing credentials",
			setupRequest: func(req *http.Request) {
				// No credentials set
			},
			validateFunc: func(userName string, secret string) (bool, error) {
				t.Error("ValidateUser should not be called with missing credentials")
				return false, nil
			},
			wantAccess:        false,
			wantAuthHeader:    false,
			wantUserInContext: false,
		},
		{
			name: "Empty username",
			setupRequest: func(req *http.Request) {
				req.SetBasicAuth("", "password")
			},
			validateFunc: func(userName string, secret string) (bool, error) {
				t.Error("ValidateUser should not be called with empty username")
				return false, nil
			},
			wantAccess:        false,
			wantAuthHeader:    false,
			wantUserInContext: false,
		},
		{
			name: "Empty password",
			setupRequest: func(req *http.Request) {
				req.SetBasicAuth("username", "")
			},
			validateFunc: func(userName string, secret string) (bool, error) {
				t.Error("ValidateUser should not be called with empty password")
				return false, nil
			},
			wantAccess:        false,
			wantAuthHeader:    false,
			wantUserInContext: false,
		},
		{
			name: "Backend validation error",
			setupRequest: func(req *http.Request) {
				req.SetBasicAuth("testuser", "testpass")
			},
			validateFunc: func(userName string, secret string) (bool, error) {
				return false, errors.New("database connection failed")
			},
			wantAccess:         false,
			wantAuthHeader:     true,
			wantAuthHeaderText: `Basic realm="restricted"`,
			wantUserInContext:  false,
		},
		{
			name: "Malformed Basic Auth header",
			setupRequest: func(req *http.Request) {
				// Manually set a malformed Basic Auth header
				req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("malformed")))
			},
			validateFunc: func(userName string, secret string) (bool, error) {
				t.Error("ValidateUser should not be called with malformed credentials")
				return false, nil
			},
			wantAccess:        false,
			wantAuthHeader:    false,
			wantUserInContext: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up test context
			gin.SetMode(gin.TestMode)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			// Create request
			req, _ := http.NewRequest("GET", "/test", nil)
			tt.setupRequest(req)
			c.Request = req

			// Create mock backend
			backend := &mockAuthenticator{
				validateFunc: tt.validateFunc,
			}

			// Create provider with custom realm if needed
			var provider Provider
			var err error
			if tt.wantAuthHeaderText != "" && tt.wantAuthHeaderText != `Basic realm="restricted"` {
				// Extract realm from expected header text
				provider, err = NewBasicAuthProvider(backend, WithRealm("custom-area"))
			} else {
				provider, err = NewBasicAuthProvider(backend)
			}
			assert.NoError(t, err)

			// Test CanAccess
			gotAccess := provider.CanAccess(c)

			// Assertions
			assert.Equal(t, tt.wantAccess, gotAccess)

			if tt.wantAuthHeader {
				authHeader := w.Header().Get("WWW-Authenticate")
				assert.NotEmpty(t, authHeader)
				if tt.wantAuthHeaderText != "" {
					assert.Equal(t, tt.wantAuthHeaderText, authHeader)
				}
			}

			if tt.wantUserInContext {
				username, exists := c.Get(gin.AuthUserKey)
				assert.True(t, exists, "Expected user to be set in context")
				assert.Equal(t, tt.wantUsername, username)
			} else {
				_, exists := c.Get(gin.AuthUserKey)
				assert.False(t, exists, "Expected user not to be set in context")
			}
		})
	}
}

func TestBasicAuthProvider_Integration(t *testing.T) {
	// This test verifies the integration with the AuthMiddleware
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		setupRequest   func(*http.Request)
		validateFunc   func(userName string, secret string) (bool, error)
		wantStatus     int
		wantHandlerRun bool
	}{
		{
			name: "Successful authentication allows access",
			setupRequest: func(req *http.Request) {
				req.SetBasicAuth("admin", "secret")
			},
			validateFunc: func(userName string, secret string) (bool, error) {
				return userName == "admin" && secret == "secret", nil
			},
			wantStatus:     http.StatusOK,
			wantHandlerRun: true,
		},
		{
			name: "Failed authentication blocks access",
			setupRequest: func(req *http.Request) {
				req.SetBasicAuth("admin", "wrongpass")
			},
			validateFunc: func(userName string, secret string) (bool, error) {
				return userName == "admin" && secret == "secret", nil
			},
			wantStatus:     http.StatusUnauthorized,
			wantHandlerRun: false,
		},
		{
			name: "Missing credentials blocks access",
			setupRequest: func(req *http.Request) {
				// No credentials
			},
			validateFunc: func(userName string, secret string) (bool, error) {
				return false, nil
			},
			wantStatus:     http.StatusUnauthorized,
			wantHandlerRun: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock backend
			backend := &mockAuthenticator{
				validateFunc: tt.validateFunc,
			}

			// Create provider
			provider, err := NewBasicAuthProvider(backend)
			assert.NoError(t, err)

			// Set up router with middleware
			router := gin.New()
			router.Use(AuthMiddleware(provider))

			// Track if handler was called
			handlerCalled := false
			router.GET("/protected", func(c *gin.Context) {
				handlerCalled = true
				c.Status(http.StatusOK)
			})

			// Create request
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/protected", nil)
			tt.setupRequest(req)

			// Serve request
			router.ServeHTTP(w, req)

			// Assertions
			assert.Equal(t, tt.wantStatus, w.Code)
			assert.Equal(t, tt.wantHandlerRun, handlerCalled)
		})
	}
}
