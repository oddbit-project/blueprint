package auth

import (
	"encoding/gob"
	"github.com/gin-gonic/gin"
	"github.com/oddbit-project/blueprint/db"
	"github.com/oddbit-project/blueprint/provider/httpserver/session"
)

type authSession struct{}

// NewAuthSession creates a new auth session
// gobIdentityTypes are optional identity types for user identity that need to be registered
// with gob() for correct serialization/deserialization
func NewAuthSession(gobIdentityTypes ...any) Provider {
	for _, v := range gobIdentityTypes {
		gob.Register(v)
	}

	// always register db.FV
	gob.Register(db.FV{})

	return &authSession{}
}

// CanAccess returns true if current session has a stored identity
func (i *authSession) CanAccess(c *gin.Context) bool {
	identity, exists := GetSessionIdentity(c)
	if exists {
		return identity != nil
	}
	return false
}

func GetSessionIdentity(c *gin.Context) (any, bool) {
	sessionData, exists := c.Get(session.ContextSessionKey)
	if exists {
		return sessionData.(*session.SessionData).GetIdentity()
	}
	return nil, false
}
