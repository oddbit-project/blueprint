package jwt

import (
	"github.com/gin-gonic/gin"
	"github.com/oddbit-project/blueprint/provider/httpserver/session"
	"strings"
	"time"
)

// JWTSessionManager manages JWT tokens for session management
type JWTSessionManager struct {
	manager *JWTManager
}

// NewJWTSessionManager creates a new JWT session manager
func NewJWTSessionManager(manager *JWTManager) *JWTSessionManager {
	return &JWTSessionManager{
		manager: manager,
	}
}

// Middleware returns a Gin middleware for JWT-based session management
func (m *JWTSessionManager) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		var sessionData *session.SessionData
		var tokenString string

		// Try to get the token from the Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader != "" && strings.HasPrefix(authHeader, "Bearer ") {
			tokenString = strings.TrimPrefix(authHeader, "Bearer ")

			// Get session from token
			var err error
			sessionData, err = m.manager.Get(tokenString)
			if err != nil {
				// If token is invalid or expired, create a new session
				if err == ErrJWTInvalid || err == ErrJWTExpired {
					sessionData, _ = m.manager.NewSession()
				}
			}
		}

		// If no valid token was found, create a new session
		if sessionData == nil {
			sessionData, _ = m.manager.NewSession()
		}

		// Store the session in the context
		c.Set(session.ContextSessionKey, sessionData)

		// Process the request
		c.Next()

		// After the request is processed, check if we need to issue a new token
		modifiedSession, exists := c.Get(session.ContextSessionKey)
		if exists {
			// Check if the session was modified or needs rotation
			if s, ok := modifiedSession.(*session.SessionData); ok {
				shouldRotate := m.shouldRotateToken(s, tokenString)
				
				if shouldRotate {
					// Generate a new rotated token
					newToken, err := m.manager.Generate(s.ID, s)
					if err == nil {
						// Set the new token in the response header
						c.Header("Authorization", "Bearer "+newToken)
					}
				} else {
					// Just update session metadata
					_ = m.manager.Set(s.ID, s)
				}
			}
		}
	}
}

// shouldRotateToken determines if a token should be rotated based on age and usage
func (m *JWTSessionManager) shouldRotateToken(sessionData *session.SessionData, currentToken string) bool {
	if currentToken == "" {
		return false
	}

	// Validate the current token to get its claims
	claims, err := m.manager.Validate(currentToken)
	if err != nil {
		return false // If token is invalid, don't rotate, let normal flow handle it
	}

	// Rotate if token is more than half way to expiration
	now := time.Now()
	tokenAge := now.Sub(claims.IssuedAt.Time)
	maxAge := claims.ExpiresAt.Time.Sub(claims.IssuedAt.Time)
	
	// Rotate if token is older than 50% of its lifetime
	if tokenAge > maxAge/2 {
		return true
	}

	// Rotate if session has been modified (new data added)
	if sessionData.LastAccessed.After(claims.IssuedAt.Time.Add(time.Minute)) {
		return true
	}

	return false
}

// Regenerate creates a new JWT token while preserving session data
func (m *JWTSessionManager) Regenerate(c *gin.Context) {
	// Get the current session
	oldSession := session.Get(c)
	if oldSession == nil {
		return
	}

	// Create a new session with the same data
	newSession, newID := m.manager.NewSession()

	// Copy the session values (excluding the JWT token)
	for k, v := range oldSession.Values {
		if k != "_jwt_token" {
			newSession.Values[k] = v
		}
	}

	// Generate a new token
	err := m.manager.Set(newID, newSession)
	if err == nil && newSession.Values["_jwt_token"] != nil {
		// Get the token from the session
		if token, ok := newSession.Values["_jwt_token"].(string); ok {
			// Set the token in the response header
			c.Header("Authorization", "Bearer "+token)
		}
	}

	// Set the new session in context
	c.Set(session.ContextSessionKey, newSession)
}

// Clear clears the current JWT session
func (m *JWTSessionManager) Clear(c *gin.Context) {
	// Create a new empty session
	sessionData, _ := m.manager.NewSession()

	// Set it in the context
	c.Set(session.ContextSessionKey, sessionData)

	// Generate a token for the new empty session
	err := m.manager.Set(sessionData.ID, sessionData)
	if err == nil && sessionData.Values["_jwt_token"] != nil {
		// Get the token from the session
		if token, ok := sessionData.Values["_jwt_token"].(string); ok {
			// Set the token in the response header
			c.Header("Authorization", "Bearer "+token)
		}
	}
}