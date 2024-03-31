package httpserver

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

// IsJSONRequest returns true if request is a JSON request
func IsJSONRequest(ctx *gin.Context) bool {
	return ctx.Request.Header.Get(HeaderAccept) == ContentTypeJson ||
		ctx.Request.Header.Get(HeaderContentType) == ContentTypeJson
}

// HttpError401 generates a error 401 response
func HttpError401(ctx *gin.Context) {
	if IsJSONRequest(ctx) {
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, JSONResponseError{
			Success: false,
			Error: JSONErrorDetail{
				Message: http.StatusText(http.StatusUnauthorized),
			},
		})
		return
	}
	ctx.AbortWithStatus(http.StatusUnauthorized)
}
