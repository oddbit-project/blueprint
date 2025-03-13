package writer

import (
	bl "github.com/oddbit-project/blueprint/log"
	"github.com/rs/zerolog"
	"log"
)

// ZerologWriter implements io.Writer by redirecting writes to a zerolog logger
type ZerologWriter struct {
	logger zerolog.Logger
	level  zerolog.Level
}

// Write implements the io.Writer interface by logging the message to the embedded zerolog logger
func (w *ZerologWriter) Write(p []byte) (n int, err error) {
	n = len(p)

	// Standard logger adds newlines, need to trim them
	msg := string(p)
	if len(msg) > 0 && msg[len(msg)-1] == '\n' {
		msg = msg[:len(msg)-1]
	}

	// Log the message at the configured level
	w.logger.WithLevel(w.level).Msg(msg)

	return n, nil
}

// NewErrorLog creates a standard library logger that writes to a zerolog Logger
// This can be used as the ErrorLog field in http.Server
func NewErrorLog(logger *bl.Logger) *log.Logger {
	// Use error level for HTTP server errors
	writer := &ZerologWriter{
		logger: logger.GetZerolog().With().Logger(),
		level:  zerolog.ErrorLevel,
	}

	// Create a standard library logger that writes to our ZerologWriter
	// Flags are set to 0 because zerolog handles timestamps and other metadata
	return log.New(writer, "", 0)
}
