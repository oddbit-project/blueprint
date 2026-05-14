package request

import (
	"strings"

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
	return strings.Contains(ctx.Request.Header.Get(HeaderAccept), ContentTypeJson) ||
		strings.Contains(ctx.Request.Header.Get(HeaderContentType), ContentTypeJson)
}
