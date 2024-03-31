package writer

import (
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	stdlog "log"
	"time"
)

func DefaultWriter() zerolog.ConsoleWriter {
	return zerolog.NewConsoleWriter(func(w *zerolog.ConsoleWriter) {
		w.TimeFormat = time.RFC3339Nano
	})
}

// UseDefaultWriter use zerolog console writer as default writer for all logging
func UseDefaultWriter() {
	zerolog.TimeFieldFormat = time.RFC3339Nano
	log.Logger = zerolog.New(DefaultWriter()).With().Timestamp().Logger()
	stdlog.SetOutput(log.Logger)
}
