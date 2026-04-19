package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/oddbit-project/blueprint/provider/httpserver/session"
	"github.com/stretchr/testify/assert"
)

func TestGetSessionIdentity_WithValidSession(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/test", nil)

	sess := &session.SessionData{
		Values: make(map[string]any),
	}
	sess.SetIdentity("user123")
	c.Set(session.ContextSessionKey, sess)

	identity, exists := GetSessionIdentity(c)
	assert.True(t, exists)
	assert.Equal(t, "user123", identity)
}

func TestGetSessionIdentity_WithNoSession(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/test", nil)

	identity, exists := GetSessionIdentity(c)
	assert.False(t, exists)
	assert.Nil(t, identity)
}

func TestGetSessionIdentity_WithWrongType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/test", nil)

	// Store a wrong type under the session key
	c.Set(session.ContextSessionKey, "not-a-session-data")

	assert.NotPanics(t, func() {
		identity, exists := GetSessionIdentity(c)
		assert.False(t, exists)
		assert.Nil(t, identity)
	})
}

func TestGetSessionIdentity_WithNoIdentity(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/test", nil)

	sess := &session.SessionData{
		Values: make(map[string]any),
	}
	c.Set(session.ContextSessionKey, sess)

	identity, exists := GetSessionIdentity(c)
	assert.False(t, exists)
	assert.Nil(t, identity)
}
