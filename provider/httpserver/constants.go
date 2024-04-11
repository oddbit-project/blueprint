package httpserver

import "github.com/oddbit-project/blueprint/utils"

const (
	ServerDefaultReadTimeout  = 600
	ServerDefaultWriteTimeout = 600
	ServerDefaultPort         = 5000
	ServerDefaultName         = "http"

	HeaderAccept      = "Accept"
	HeaderContentType = "Content-Type"

	ContentTypeHtml   = "text/html"
	ContentTypeJson   = "application/json"
	ContentTypeBinary = "application/octet-stream"

	ErrNilConfig = utils.Error("Config is nil")
)
