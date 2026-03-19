package auth

import (
	"github.com/gin-gonic/gin"
	"github.com/oddbit-project/blueprint/provider/httpserver/response"
)

const (
	// ContextAuthIdentity is the context key for the unified auth identity
	ContextAuthIdentity = "authIdentity"
)

// AuthIdentity represents the authenticated entity, regardless of auth method.
type AuthIdentity struct {
	// Method identifies which auth mechanism succeeded (e.g. "jwt", "hmac", "basic", "session", "token")
	Method string
	// ID is the primary identifier (JWT subject/ID, HMAC keyId, username, etc.)
	ID string
	// Extra holds method-specific data (JWT claims, HMAC details, etc.)
	Extra any
}

// GetAuthIdentity returns the authenticated identity from context, if any.
func GetAuthIdentity(c *gin.Context) (*AuthIdentity, bool) {
	raw, ok := c.Get(ContextAuthIdentity)
	if !ok {
		return nil, false
	}
	identity, ok := raw.(*AuthIdentity)
	return identity, ok
}

type Provider interface {
	CanAccess(c *gin.Context) bool
}

func AuthMiddleware(auth Provider) gin.HandlerFunc {
	return func(c *gin.Context) {
		if auth.CanAccess(c) {
			c.Next()
			return
		}
		response.Http401(c)
	}
}
