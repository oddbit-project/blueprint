package session

import (
	"github.com/gin-gonic/gin"
	"github.com/oddbit-project/blueprint/provider/kv"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSessionMiddleware(t *testing.T) {
	// Setup
	gin.SetMode(gin.TestMode)
	config := NewConfig()
	store := NewStore(config, kv.NewMemoryKV(), nil)
	manager := NewManager(store, config, nil)

	// Create a test router
	router := gin.New()
	router.Use(manager.Middleware())

	// Add a test route that uses the session
	router.GET("/test", func(c *gin.Context) {
		// Get session
		session := Get(c)
		assert.NotNil(t, session)

		// Set a value
		session.Set("test", "value")

		c.String(http.StatusOK, "OK")
	})

	// Test the route
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)

	// Check response
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "OK", w.Body.String())

	// Check if session cookie was set
	cookies := w.Result().Cookies()
	assert.NotEmpty(t, cookies)

	var sessionCookie *http.Cookie
	for _, cookie := range cookies {
		if cookie.Name == config.CookieName {
			sessionCookie = cookie
			break
		}
	}

	assert.NotNil(t, sessionCookie)
	assert.NotEmpty(t, sessionCookie.Value)
	assert.Equal(t, config.HttpOnly, sessionCookie.HttpOnly)
	assert.Equal(t, config.Secure, sessionCookie.Secure)
	assert.Equal(t, config.Path, sessionCookie.Path)
}

func TestSessionRegenerate(t *testing.T) {
	// Setup
	gin.SetMode(gin.TestMode)
	config := NewConfig()
	store := NewStore(config, kv.NewMemoryKV(), nil)
	manager := NewManager(store, config, nil)

	// Create a test router
	router := gin.New()
	router.Use(manager.Middleware())

	// Add a route to set a session value
	router.GET("/set", func(c *gin.Context) {
		Get(c).Set("test", "value")
		c.String(http.StatusOK, "OK")
	})

	// Add a route to regenerate the session
	router.GET("/regenerate", func(c *gin.Context) {
		manager.Regenerate(c)
		c.String(http.StatusOK, "Regenerated")
	})

	// Add a route to get the session value
	router.GET("/get", func(c *gin.Context) {
		val, ok := Get(c).GetString("test")
		if ok {
			c.String(http.StatusOK, val)
		} else {
			c.String(http.StatusNotFound, "Not found")
		}
	})

	// First, set a value
	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest("GET", "/set", nil)
	router.ServeHTTP(w1, req1)

	// Get the session cookie
	cookies1 := w1.Result().Cookies()
	var oldSessionCookie *http.Cookie
	for _, cookie := range cookies1 {
		if cookie.Name == config.CookieName {
			oldSessionCookie = cookie
			break
		}
	}

	// Now regenerate the session
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/regenerate", nil)
	if oldSessionCookie != nil {
		req2.AddCookie(oldSessionCookie)
	}
	router.ServeHTTP(w2, req2)

	// Get the new session cookie
	cookies2 := w2.Result().Cookies()
	var newSessionCookie *http.Cookie
	for _, cookie := range cookies2 {
		if cookie.Name == config.CookieName {
			newSessionCookie = cookie
			break
		}
	}

	// Verify the session ID changed
	assert.NotEqual(t, oldSessionCookie.Value, newSessionCookie.Value)

	// Verify the session data is still accessible
	w3 := httptest.NewRecorder()
	req3, _ := http.NewRequest("GET", "/get", nil)
	if newSessionCookie != nil {
		req3.AddCookie(newSessionCookie)
	}
	router.ServeHTTP(w3, req3)

	// The value should still be accessible
	assert.Equal(t, "value", w3.Body.String())
}

func TestClearSession(t *testing.T) {
	// Setup
	gin.SetMode(gin.TestMode)
	config := NewConfig()
	store := NewStore(config, kv.NewMemoryKV(), nil)
	manager := NewManager(store, config, nil)

	// Create a test router
	router := gin.New()
	router.Use(manager.Middleware())

	// Add a route to set a session value
	router.GET("/set", func(c *gin.Context) {
		Get(c).Set("test", "value")
		c.String(http.StatusOK, "OK")
	})

	// Add a route to clear the session
	router.GET("/clear", func(c *gin.Context) {
		manager.Clear(c)
		c.String(http.StatusOK, "Cleared")
	})

	// Add a route to get the session value
	router.GET("/get", func(c *gin.Context) {
		val, ok := Get(c).GetString("test")
		if ok {
			c.String(http.StatusOK, val)
		} else {
			c.String(http.StatusNotFound, "Not found")
		}
	})

	// First, set a value
	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest("GET", "/set", nil)
	router.ServeHTTP(w1, req1)

	// Get the session cookie
	cookies1 := w1.Result().Cookies()
	var sessionCookie *http.Cookie
	for _, cookie := range cookies1 {
		if cookie.Name == config.CookieName {
			sessionCookie = cookie
			break
		}
	}

	// Now clear the session
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/clear", nil)
	if sessionCookie != nil {
		req2.AddCookie(sessionCookie)
	}
	router.ServeHTTP(w2, req2)

	// Try to get the session value
	w3 := httptest.NewRecorder()
	req3, _ := http.NewRequest("GET", "/get", nil)

	// Get the new (cleared) cookie
	cookies2 := w2.Result().Cookies()
	var clearedCookie *http.Cookie
	for _, cookie := range cookies2 {
		if cookie.Name == config.CookieName {
			clearedCookie = cookie
			break
		}
	}

	if clearedCookie != nil {
		req3.AddCookie(clearedCookie)
	}

	router.ServeHTTP(w3, req3)

	// The value should no longer be accessible
	assert.Equal(t, "Not found", w3.Body.String())
}

func TestFlashMessages(t *testing.T) {
	// Setup
	gin.SetMode(gin.TestMode)
	config := NewConfig()
	store := NewStore(config, kv.NewMemoryKV(), nil)
	manager := NewManager(store, config, nil)

	// Create a test router
	router := gin.New()
	router.Use(manager.Middleware())

	// Add a route to set a flash message
	router.GET("/flash", func(c *gin.Context) {
		Get(c).FlashString("Hello, flash!")
		c.String(http.StatusOK, "Flash set")
	})

	// Add a route to get the flash message
	router.GET("/get-flash", func(c *gin.Context) {
		msg, ok := Get(c).GetFlashString()
		if ok {
			c.String(http.StatusOK, msg)
		} else {
			c.String(http.StatusNotFound, "No flash message")
		}
	})

	// First, set a flash message
	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest("GET", "/flash", nil)
	router.ServeHTTP(w1, req1)

	// Get the session cookie
	cookies := w1.Result().Cookies()
	var sessionCookie *http.Cookie
	for _, cookie := range cookies {
		if cookie.Name == config.CookieName {
			sessionCookie = cookie
			break
		}
	}

	// Get the flash message
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/get-flash", nil)
	req2.AddCookie(sessionCookie)
	router.ServeHTTP(w2, req2)

	// The flash message should be returned
	assert.Equal(t, "Hello, flash!", w2.Body.String())

	// Try to get the flash message again (should be gone)
	w3 := httptest.NewRecorder()
	req3, _ := http.NewRequest("GET", "/get-flash", nil)

	// Get the cookie from the previous response
	cookies2 := w2.Result().Cookies()
	var updatedCookie *http.Cookie
	for _, cookie := range cookies2 {
		if cookie.Name == config.CookieName {
			updatedCookie = cookie
			break
		}
	}

	// Make sure we have a cookie before adding it
	if updatedCookie != nil {
		req3.AddCookie(updatedCookie)
	}

	router.ServeHTTP(w3, req3)

	// The flash message should be gone
	assert.Equal(t, "No flash message", w3.Body.String())
}
