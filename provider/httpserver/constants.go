package httpserver

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
)
