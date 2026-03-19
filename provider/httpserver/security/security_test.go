package security

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/oddbit-project/blueprint/provider/httpserver/session"
	"github.com/stretchr/testify/assert"
)

func TestSecurityMiddleware_PermissionsPolicyOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)

	config := DefaultSecurityConfig()
	config.UseCSPNonce = false // simplify test

	router := gin.New()
	router.Use(SecurityMiddleware(config))
	router.GET("/test", func(c *gin.Context) {
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	// Should set Permissions-Policy
	assert.NotEmpty(t, w.Header().Get("Permissions-Policy"))
	// Should NOT set deprecated Feature-Policy
	assert.Empty(t, w.Header().Get("Feature-Policy"))
}

func TestCSRFProtection_SeedsTokenOnGET(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()

	// Simulate session middleware by injecting a SessionData into context
	router.Use(func(c *gin.Context) {
		sess := &session.SessionData{
			Values: make(map[string]any),
		}
		c.Set(session.ContextSessionKey, sess)
		c.Next()
	})
	router.Use(CSRFProtection())
	router.GET("/test", func(c *gin.Context) {
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	// CSRF token should be seeded and returned in header
	token := w.Header().Get("X-CSRF-Token")
	assert.NotEmpty(t, token, "CSRF token should be seeded on first GET")
}

func TestCSRFProtection_DoesNotReseedExistingToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	existingToken := "existing-csrf-token"

	router := gin.New()
	router.Use(func(c *gin.Context) {
		sess := &session.SessionData{
			Values: map[string]any{"_csrf": existingToken},
		}
		c.Set(session.ContextSessionKey, sess)
		c.Next()
	})
	router.Use(CSRFProtection())
	router.GET("/test", func(c *gin.Context) {
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	// Should NOT set a new token header when one already exists
	assert.Empty(t, w.Header().Get("X-CSRF-Token"))
}

func TestCSRFProtection_POSTWithSeededToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	csrfToken := "valid-token-123"

	router := gin.New()
	router.Use(func(c *gin.Context) {
		sess := &session.SessionData{
			Values: map[string]any{"_csrf": csrfToken},
		}
		c.Set(session.ContextSessionKey, sess)
		c.Next()
	})
	router.Use(CSRFProtection())
	router.POST("/test", func(c *gin.Context) {
		c.String(200, "ok")
	})

	req := httptest.NewRequest("POST", "/test", nil)
	req.Header.Set("X-CSRF-Token", csrfToken)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
}

func TestCSRFProtection_POSTWithoutToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		sess := &session.SessionData{
			Values: map[string]any{"_csrf": "expected-token"},
		}
		c.Set(session.ContextSessionKey, sess)
		c.Next()
	})
	router.Use(CSRFProtection())
	router.POST("/test", func(c *gin.Context) {
		c.String(200, "ok")
	})

	req := httptest.NewRequest("POST", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, 403, w.Code)
}
