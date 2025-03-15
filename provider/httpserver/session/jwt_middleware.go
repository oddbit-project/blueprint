package session

import (
	"github.com/gin-gonic/gin"
	"strings"
)

// JWTManager manages JWT tokens for session management
type JWTSessionManager struct {
	store  *JWTStore
	config *SessionConfig
}

// NewJWTSessionManager creates a new JWT session manager
func NewJWTSessionManager(jwtManager *JWTManager, sessionConfig *SessionConfig) *JWTSessionManager {
	if sessionConfig == nil {
		sessionConfig = DefaultSessionConfig()
	}

	store := NewJWTStore(jwtManager, sessionConfig)
	
	return &JWTSessionManager{
		store:  store,
		config: sessionConfig,
	}
}

// Middleware returns a Gin middleware for JWT-based session management
func (m *JWTSessionManager) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		var session *SessionData
		var tokenString string
		var err error
		
		// Try to get the token from the Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader != "" && strings.HasPrefix(authHeader, "Bearer ") {
			tokenString = strings.TrimPrefix(authHeader, "Bearer ")
			
			// Get session from token
			session, err = m.store.Get(tokenString)
			if err != nil {
				// If token is invalid or expired, create a new session
				if err == ErrJWTInvalid || err == ErrJWTExpired {
					session, _ = m.store.Generate()
				}
			}
		}
		
		// If no valid token was found, create a new session
		if session == nil {
			session, _ = m.store.Generate()
		}
		
		// Store the session in the context
		c.Set(ContextSessionKey, session)
		
		// Process the request
		c.Next()
		
		// After the request is processed, check if we need to issue a new token
		modifiedSession, exists := c.Get(ContextSessionKey)
		if exists {
			// Check if the session was modified
			if s, ok := modifiedSession.(*SessionData); ok {
				// Generate a new token
				err = m.store.Set(s.ID, s)
				if err == nil && s.Values["_jwt_token"] != nil {
					// Get the token from the session
					if token, ok := s.Values["_jwt_token"].(string); ok {
						// Set the token in the response header
						c.Header("Authorization", "Bearer "+token)
					}
				}
			}
		}
	}
}

// Regenerate creates a new JWT token while preserving session data
func (m *JWTSessionManager) Regenerate(c *gin.Context) {
	// Get the current session
	oldSession := Get(c)
	if oldSession == nil {
		return
	}
	
	// Create a new session with the same data
	newSession, newID := m.store.Generate()
	
	// Copy the session values (excluding the JWT token)
	for k, v := range oldSession.Values {
		if k != "_jwt_token" {
			newSession.Values[k] = v
		}
	}
	
	// Generate a new token
	err := m.store.Set(newID, newSession)
	if err == nil && newSession.Values["_jwt_token"] != nil {
		// Get the token from the session
		if token, ok := newSession.Values["_jwt_token"].(string); ok {
			// Set the token in the response header
			c.Header("Authorization", "Bearer "+token)
		}
	}
	
	// Set the new session in context
	c.Set(ContextSessionKey, newSession)
}

// Clear clears the current JWT session
func (m *JWTSessionManager) Clear(c *gin.Context) {
	// Create a new empty session
	session, _ := m.store.Generate()
	
	// Set it in the context
	c.Set(ContextSessionKey, session)
	
	// Generate a token for the new empty session
	err := m.store.Set(session.ID, session)
	if err == nil && session.Values["_jwt_token"] != nil {
		// Get the token from the session
		if token, ok := session.Values["_jwt_token"].(string); ok {
			// Set the token in the response header
			c.Header("Authorization", "Bearer "+token)
		}
	}
}