package auth

import "github.com/gin-gonic/gin"

const (
	DefaultTokenAuthHeader = "X-Auth-Key"
)

type authToken struct {
	headerName string
	key        string
}

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
