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
func NewAuthToken(headerName string, key string) Provider {
	return &authToken{
		headerName: headerName,
		key:        key,
	}
}
func (a *authToken) CanAccess(c *gin.Context) bool {
	if len(a.key) > 0 {
		return c.Request.Header.Get(a.headerName) == a.key
	}
	return true
}

// NewAuthTokenList create simple auth token provider
// checks if a predefined header has a specific token from a token list
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
