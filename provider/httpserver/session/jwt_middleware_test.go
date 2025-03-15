package session

import (
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestJWTSessionMiddleware(t *testing.T) {
	// Setup
	gin.SetMode(gin.TestMode)
	
	// Create JWT config
	jwtConfig := DefaultJWTConfig()
	jwtConfig.SigningKey = []byte("test-signing-key-for-jwt-tests-only")
	
	// Create JWT manager
	jwtManager, err := NewJWTManager(jwtConfig)
	assert.NoError(t, err)
	
	// Create session config
	sessionConfig := DefaultSessionConfig()
	
	// Create session manager
	manager := NewJWTSessionManager(jwtManager, sessionConfig)
	
	// Create a test router
	router := gin.New()
	router.Use(manager.Middleware())
	
	// Add a test route that uses the session
	router.GET("/test", func(c *gin.Context) {
		// Get session
		session := Get(c)
		assert.NotNil(t, session)
		
		// Set a value
		Set(c, "test", "value")
		
		c.String(http.StatusOK, "OK")
	})
	
	// Test without authorization header
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)
	
	// Check response
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "OK", w.Body.String())
	
	// Check if JWT token was set in response header
	authHeader := w.Header().Get("Authorization")
	assert.NotEmpty(t, authHeader)
	assert.Contains(t, authHeader, "Bearer ")
	
	// Extract token
	tokenString := authHeader[7:] // Remove "Bearer " prefix
	
	// Make another request with the token
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/test", nil)
	req2.Header.Set("Authorization", "Bearer "+tokenString)
	router.ServeHTTP(w2, req2)
	
	// Check response
	assert.Equal(t, http.StatusOK, w2.Code)
	
	// Now validate that our session value was set
	claims, err := jwtManager.Validate(tokenString)
	assert.NoError(t, err)
	assert.NotNil(t, claims)
	
	// The session data would have been updated
	// but we can't easily access it due to the stateless nature of JWT
}

func TestJWTSessionRegenerateAndClear(t *testing.T) {
	// Setup
	gin.SetMode(gin.TestMode)
	
	// Create JWT config
	jwtConfig := DefaultJWTConfig()
	jwtConfig.SigningKey = []byte("test-signing-key-for-jwt-tests-only")
	
	// Create JWT manager
	jwtManager, err := NewJWTManager(jwtConfig)
	assert.NoError(t, err)
	
	// Create session config
	sessionConfig := DefaultSessionConfig()
	
	// Create session manager
	manager := NewJWTSessionManager(jwtManager, sessionConfig)
	
	// Create a test router
	router := gin.New()
	router.Use(manager.Middleware())
	
	// Add routes to test session regeneration and clearing
	router.GET("/set", func(c *gin.Context) {
		Set(c, "test", "value")
		c.String(http.StatusOK, "Value set")
	})
	
	router.GET("/regenerate", func(c *gin.Context) {
		manager.Regenerate(c)
		c.String(http.StatusOK, "Regenerated")
	})
	
	router.GET("/clear", func(c *gin.Context) {
		manager.Clear(c)
		c.String(http.StatusOK, "Cleared")
	})
	
	// First, set a value
	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest("GET", "/set", nil)
	router.ServeHTTP(w1, req1)
	
	// Get the token
	token1 := w1.Header().Get("Authorization")[7:] // Remove "Bearer " prefix
	
	// Now regenerate the session
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/regenerate", nil)
	req2.Header.Set("Authorization", "Bearer "+token1)
	router.ServeHTTP(w2, req2)
	
	// Get the new token
	token2 := w2.Header().Get("Authorization")[7:] // Remove "Bearer " prefix
	
	// Tokens should be different
	assert.NotEqual(t, token1, token2)
	
	// Now clear the session
	w3 := httptest.NewRecorder()
	req3, _ := http.NewRequest("GET", "/clear", nil)
	req3.Header.Set("Authorization", "Bearer "+token2)
	router.ServeHTTP(w3, req3)
	
	// Get the new token after clearing
	token3 := w3.Header().Get("Authorization")[7:] // Remove "Bearer " prefix
	
	// Should be a new token
	assert.NotEqual(t, token2, token3)
}