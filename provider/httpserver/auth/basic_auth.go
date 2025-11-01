package auth

import (
	"errors"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/oddbit-project/blueprint/provider/httpserver/auth/backend"
	"github.com/oddbit-project/blueprint/provider/httpserver/log"
)

const (
	DefaultRealm = "restricted"
)

type BasicAuthProvider struct {
	backend backend.Authenticator
	realm   string
}

type BasicAuthProviderOption func(*BasicAuthProvider)

func WithRealm(realm string) BasicAuthProviderOption {
	return func(b *BasicAuthProvider) {
		b.realm = realm
	}
}

func NewBasicAuthProvider(b backend.Authenticator, opts ...BasicAuthProviderOption) (*BasicAuthProvider, error) {
	if b == nil {
		return nil, errors.New("authenticator backend is required")
	}

	result := &BasicAuthProvider{
		backend: b,
		realm:   DefaultRealm,
	}

	for _, opt := range opts {
		opt(result)
	}
	return result, nil
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
		c.Header("WWW-Authenticate", fmt.Sprintf(`Basic realm="%s"`, a.realm))
		return false
	}
	if !valid {
		logger.Warnf("BasicAuthProvider: failed login for user '%s'", u)
		c.Header("WWW-Authenticate", fmt.Sprintf(`Basic realm="%s"`, a.realm))
		return false
	}
	logger.Infof("BasicAuthProvider: user '%s' authenticated", u)
	c.Set(gin.AuthUserKey, u)
	return true
}
