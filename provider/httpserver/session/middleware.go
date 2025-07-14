package session

import (
	"github.com/gin-gonic/gin"
	"github.com/oddbit-project/blueprint/log"
	"net/http"
)

// SessionManager manages sessions and provides middleware for Gin
type SessionManager struct {
	store  *Store
	config *Config
	logger *log.Logger
}

// NewManager creates a new session manager
func NewManager(store *Store, config *Config, logger *log.Logger) *SessionManager {
	if config == nil {
		config = NewConfig()
	}

	manager := &SessionManager{
		store:  store,
		config: config,
		logger: logger,
	}

	// Start the cleanup goroutine
	store.StartCleanup()

	return manager
}

// Middleware returns a Gin middleware for session management
func (m *SessionManager) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		var session *SessionData
		var sessionID string
		var err error

		// Try to get the session ID from the cookie
		cookie, err := c.Cookie(m.config.CookieName)
		if err == nil && cookie != "" {
			// Cookie exists, try to get the session
			session, err = m.store.Get(cookie)
			if err == nil {
				// Session found, set it to the context
				sessionID = cookie
			}
		}

		// If no valid session was found, create a new one
		if session == nil {
			session, sessionID = m.store.Generate()

			// Save the session
			err = m.store.Set(sessionID, session)
			if err != nil {
				// Log error but proceed
				if m.logger != nil {
					m.logger.Error(err, "Failed to save session")
				}
			}

			// Set the cookie
			m.setSessionCookie(c, sessionID)
		}

		// Store the session in the context
		c.Set(ContextSessionKey, session)

		// Process the request
		c.Next()

		// After the request is processed, save any changes to the session
		modifiedSession, exists := c.Get(ContextSessionKey)
		if exists && modifiedSession != nil {
			// Update the session in the store
			if s, ok := modifiedSession.(*SessionData); ok {
				m.store.Set(sessionID, s)
			}
		} else {
			// session does not exist, delete the session
			_ = m.store.Delete(sessionID)
		}
	}
}

// setSessionCookie sets the session cookie on the response
func (m *SessionManager) setSessionCookie(c *gin.Context, sessionID string) {
	c.SetCookie(
		m.config.CookieName,
		sessionID,
		m.config.ExpirationSeconds,
		m.config.Path,
		m.config.Domain,
		m.config.Secure,
		m.config.HttpOnly,
	)

	// Set SameSite attribute using header (since gin.SetCookie doesn't support SameSite)
	ss := http.SameSite(m.config.SameSite)
	if ss != http.SameSiteDefaultMode {
		var sameSiteValue string
		switch ss {
		case http.SameSiteStrictMode:
			sameSiteValue = "Strict"
		case http.SameSiteLaxMode:
			sameSiteValue = "Lax"
		case http.SameSiteNoneMode:
			sameSiteValue = "None"
		default:
			sameSiteValue = "Lax"
		}
		c.Header("Set-Cookie", c.Writer.Header().Get("Set-Cookie")+"; SameSite="+sameSiteValue)
	}
}

// Get returns the session from the context
func Get(c *gin.Context) *SessionData {
	if val, exists := c.Get(ContextSessionKey); exists {
		if session, ok := val.(*SessionData); ok {
			return session
		}
	}
	return nil
}

// Regenerate regenerates the session ID to prevent session fixation
func (m *SessionManager) Regenerate(c *gin.Context) {
	// Get the current session
	oldSession := Get(c)
	if oldSession == nil {
		return
	}

	// Create a new session with the same data
	newSession, newSessionID := m.store.Generate()
	newSession.Values = oldSession.Values

	// Save the new session
	m.store.Set(newSessionID, newSession)

	// Set the new session in context
	c.Set(ContextSessionKey, newSession)

	// Set the new cookie
	m.setSessionCookie(c, newSessionID)

	// Delete the old session
	oldCookie, err := c.Cookie(m.config.CookieName)
	if err == nil && oldCookie != "" {
		m.store.Delete(oldCookie)
	}
}

// Clear removes the session
func (m *SessionManager) Clear(c *gin.Context) {
	// Delete the session from the store
	cookie, err := c.Cookie(m.config.CookieName)
	if err == nil && cookie != "" {
		m.store.Delete(cookie)
	}

	// Clear the cookie
	c.SetCookie(
		m.config.CookieName,
		"",
		-1, // Expire immediately
		m.config.Path,
		m.config.Domain,
		m.config.Secure,
		m.config.HttpOnly,
	)

	// Remove from context
	c.Set(ContextSessionKey, nil)
}
