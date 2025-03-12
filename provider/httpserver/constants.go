package httpserver

import "github.com/oddbit-project/blueprint/utils"

const (
	ServerDefaultReadTimeout  = 600
	ServerDefaultWriteTimeout = 600
	ServerDefaultPort         = 5000
	ServerDefaultName         = "http"

	ErrNilConfig = utils.Error("Config is nil")
)
