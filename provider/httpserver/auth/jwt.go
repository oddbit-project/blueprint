package auth

import (
	"github.com/gin-gonic/gin"
	"github.com/oddbit-project/blueprint/provider/jwtprovider"
)

const (
	ContextJwtClaims = "jwtClaims"
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
		return false
	}

	claims, err := a.parser.ParseToken(token)
	if err != nil || len(claims.ID) == 0 {
		return false
	}

	c.Set(ContextJwtClaims, claims)
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

// GetJWTClaims helper to get claims from gin context
func GetJWTClaims(c *gin.Context) (*jwtprovider.Claims, bool) {
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
