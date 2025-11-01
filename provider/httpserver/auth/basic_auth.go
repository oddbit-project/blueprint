package auth

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/oddbit-project/blueprint/provider/httpserver/auth/backend"
	"github.com/oddbit-project/blueprint/provider/httpserver/log"
)

type BasicAuthProvider struct {
	backend backend.Authenticator
}

func NewBasicAuthProvider(b backend.Authenticator) (*BasicAuthProvider, error) {
	if b == nil {
		return nil, errors.New("authenticator backend is required")
	}

	return &BasicAuthProvider{
		backend: b,
	}, nil
}

func (a *BasicAuthProvider) CanAccess(c *gin.Context) bool {
	u, p, ok := c.Request.BasicAuth()
	if !ok || len(u) == 0 || len(p) == 0 {
		return false
	}
	logger := log.GetRequestLogger(c)
	valid, err := a.backend.ValidateUser(u, p)
	if err != nil {
		logger.Error(err, "BasicAuthProvider: error validating user")
		return false
	}
	if !valid {
		logger.Warnf("BasicAuthProvider: failed login for user '%s'", u)
		return false
	}
	logger.Infof("BasicAuthProvider: user '%s' authenticated", u)
	c.Set(gin.AuthUserKey, u)
	return true
}
