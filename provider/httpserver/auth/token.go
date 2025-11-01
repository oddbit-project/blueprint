package auth

import (
	"crypto/subtle"

	"github.com/gin-gonic/gin"
)

const (
	DefaultTokenAuthHeader = "X-API-Key"
)

type authToken struct {
	headerName string
	key        string
}

type authTokenList struct {
	headerName string
	keyList    []string
}

// NewAuthToken create simple auth token provider
// checks a predefined header for a specific token
// Security Note: while this approach may be somewhat fine for backend, machine-to-machine authentication in a highly
// secured and controlled environment, it is inherently as insecure as it can get. DO NOT use it as a means of authentication on web
// environments!!
func NewAuthToken(headerName string, key string) Provider {
	return &authToken{
		headerName: headerName,
		key:        key,
	}
}

// CanAccess returns true if request is valid
// Note: this method supports empty keys as a means to disable authentication
func (a *authToken) CanAccess(c *gin.Context) bool {
	return subtle.ConstantTimeCompare([]byte(c.Request.Header.Get(a.headerName)), []byte(a.key)) == 1
}

// NewAuthTokenList create simple auth token provider
// checks if a predefined header has a specific token from a token list
// Security Note: while this approach may be somewhat fine for backend, machine-to-machine authentication in a highly
// secured and controlled environment, it is inherently as insecure as it can get. DO NOT use it as a means of authentication on web
// environments!!
func NewAuthTokenList(headerName string, keyList []string) Provider {
	return &authTokenList{
		headerName: headerName,
		keyList:    keyList,
	}
}

// CanAccess returns true if request is valid
// Note: this method supports empty keys as a means to disable authentication
func (a *authTokenList) CanAccess(c *gin.Context) bool {
	key := c.Request.Header.Get(a.headerName)
	for _, existingKey := range a.keyList {
		if subtle.ConstantTimeCompare([]byte(key), []byte(existingKey)) == 1 {
			return true
		}
	}
	return false
}
