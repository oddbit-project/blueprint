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
	authHeader := c.GetHeader("Authorization")
	if len(authHeader) < 7 || authHeader[:7] != "Bearer " {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": ErrMissingAuthHeader})
		return false
	}

	claims, err := a.parser.ParseToken(authHeader[7:])
	if err != nil || len(claims.ID) == 0 {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
		return false
	}

	c.Set(ContextJwtClaims, claims)
	c.Next()
	return true
}
