package auth

import (
	"github.com/gin-gonic/gin"
	"github.com/oddbit-project/blueprint/provider/jwtprovider"
	"net/http"
)

const (
	ErrMissingAuthHeader = "missing or invalid Authorization header"
	ContextJwtClaims     = "jwtClaims"
)

type authJWT struct {
	parser jwtprovider.JWTParser
}

func NewAuthJWT(p jwtprovider.JWTParser) Provider {
	return &authJWT{
		parser: p,
	}
}

func (a *authJWT) CanAccess(c *gin.Context) bool {
	token, valid := GetJWTToken(c)
	if !valid {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": ErrMissingAuthHeader})
		return false
	}

	claims, err := a.parser.ParseToken(token)
	if err != nil || len(claims.ID) == 0 {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
		return false
	}

	c.Set(ContextJwtClaims, claims)
	c.Next()
	return true
}

// GetJWTToken helper to get JWT token from gin context
func GetJWTToken(c *gin.Context) (string, bool) {
	authHeader := c.GetHeader("Authorization")
	if len(authHeader) < 7 || authHeader[:7] != "Bearer " {
		return "", false
	}
	return authHeader[7:], true
}

// GetClaims helper to get claims from gin context
func GetClaims(c *gin.Context) (*jwtprovider.Claims, bool) {
	raw, ok := c.Get(ContextJwtClaims)
	if !ok {
		return nil, false
	}
	claims, ok := raw.(*jwtprovider.Claims)
	if ok {
		return claims, true
	}
	return nil, false
}
