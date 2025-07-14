package auth

import (
	"github.com/gin-gonic/gin"
	"github.com/oddbit-project/blueprint/provider/httpserver/response"
)

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
		c.Abort()
	}
}
