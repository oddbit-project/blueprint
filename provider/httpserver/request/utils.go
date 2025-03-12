package request

import (
	"github.com/gin-gonic/gin"
)

const (
	HeaderAccept      = "Accept"
	HeaderContentType = "Content-Type"

	ContentTypeHtml   = "text/html"
	ContentTypeJson   = "application/json"
	ContentTypeBinary = "application/octet-stream"
)

// IsJSONRequest returns true if request is a JSON request
func IsJSONRequest(ctx *gin.Context) bool {
	return ctx.Request.Header.Get(HeaderAccept) == ContentTypeJson ||
		ctx.Request.Header.Get(HeaderContentType) == ContentTypeJson
}
