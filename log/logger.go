package log

import (
	"context"
	"fmt"
	"github.com/oddbit-project/blueprint/utils/debug"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"io"
	"os"
	"time"
)

// Configuration constants
const (
	// LogContextKey is used to store/retrieve logger from context
	LogContextKey       = "logger"
	LogTraceIDKey       = "trace_id"
	LogModuleKey        = "module"
	LogComponentKey     = "component"
	LogTimestampFormat  = time.RFC3339Nano
	LogCallerSkipFrames = 2
)

// Logger wraps zerolog.Logger to provide consistent logging patterns
type Logger struct {
	logger     zerolog.Logger
	moduleInfo string
	hostname   string
	traceID    string
}

// KV is a helper to field map
type KV map[string]interface{}

// Config contains configuration for the logger
type Config struct {
	Level            string `json:"level"`
	Format           string `json:"format"` // "console" or "json"
	IncludeTimestamp bool   `json:"includeTimestamp"`
	IncludeCaller    bool   `json:"includeCaller"`
	IncludeHostname  bool   `json:"includeHostname"`
	CallerSkipFrames int    `json:"callerSkipFrames"`
}

// NewDefaultConfig returns a default logging configuration
func NewDefaultConfig() *Config {
	return &Config{
		Level:            "info",
		Format:           "console",
		IncludeTimestamp: true,
		IncludeCaller:    true,
		IncludeHostname:  true,
		CallerSkipFrames: LogCallerSkipFrames,
	}
}

// Configure configures the global logger based on the provided configuration
func Configure(cfg *Config) error {
	// Set global log level
	level, err := zerolog.ParseLevel(cfg.Level)
	if err != nil {
		return fmt.Errorf("invalid log level: %s", cfg.Level)
	}
	zerolog.SetGlobalLevel(level)

	// Configure timestamp format
	zerolog.TimeFieldFormat = LogTimestampFormat

	// Determine output writer
	var output zerolog.ConsoleWriter
	if cfg.Format == "console" {
		output = zerolog.NewConsoleWriter(func(w *zerolog.ConsoleWriter) {
			w.TimeFormat = LogTimestampFormat
		})
	}

	// Configure base logger
	baseLogger := zerolog.New(output).Level(level)

	// Add standard fields
	if cfg.IncludeTimestamp {
		baseLogger = baseLogger.With().Timestamp().Logger()
	}

	if cfg.IncludeCaller {
		baseLogger = baseLogger.With().Caller().Logger()
		zerolog.CallerSkipFrameCount = cfg.CallerSkipFrames
	}

	// Set as global logger
	log.Logger = baseLogger

	return nil
}

// New creates a new logger with module information
func New(module string) *Logger {
	hostname, _ := os.Hostname()

	return &Logger{
		logger:     log.With().Str(LogModuleKey, module).Logger(),
		moduleInfo: module,
		hostname:   hostname,
	}
}

// ModuleInfo get module name
func (l *Logger) ModuleInfo() string {
	return l.moduleInfo
}

// Hostname get configured hostname
func (l *Logger) Hostname() string {
	return l.hostname
}

// NewWithComponent creates a new logger with module and component information
func NewWithComponent(module, component string) *Logger {
	hostname, _ := os.Hostname()

	return &Logger{
		logger: log.With().
			Str(LogModuleKey, module).
			Str(LogComponentKey, component).
			Logger(),
		moduleInfo: fmt.Sprintf("%s.%s", module, component),
		hostname:   hostname,
	}
}

// WithOutput use a custom output
func (l *Logger) WithOutput(output io.Writer) *Logger {
	newLogger := &Logger{
		logger:     l.logger.Output(output),
		moduleInfo: l.moduleInfo,
		hostname:   l.hostname,
		traceID:    l.traceID,
	}
	return newLogger
}

// WithTraceID creates a new logger with the specified trace ID
func (l *Logger) WithTraceID(traceID string) *Logger {
	newLogger := &Logger{
		logger:     l.logger.With().Str(LogTraceIDKey, traceID).Logger(),
		moduleInfo: l.moduleInfo,
		hostname:   l.hostname,
		traceID:    traceID,
	}
	return newLogger
}

// WithField adds a field to the logger
func (l *Logger) WithField(key string, value interface{}) *Logger {
	newLogger := &Logger{
		logger:     l.logger.With().Interface(key, value).Logger(),
		moduleInfo: l.moduleInfo,
		hostname:   l.hostname,
		traceID:    l.traceID,
	}
	return newLogger
}

// FromContext retrieves a logger from the context
// If no logger is found, a new default logger is returned
func FromContext(ctx context.Context) *Logger {
	if ctx == nil {
		return New("default")
	}

	value := ctx.Value(LogContextKey)
	if value == nil {
		return New("default")
	}

	logger, ok := value.(*Logger)
	if !ok {
		return New("default")
	}

	return logger
}

// WithContext adds the logger to the context
func (l *Logger) WithContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, LogContextKey, l)
}

// Standard logging methods

// Debug logs a debug message with the given fields
func (l *Logger) Debug(msg string, fields ...map[string]interface{}) {
	event := l.logger.Debug()
	if len(fields) > 0 {
		for k, v := range fields[0] {
			event = event.Interface(k, v)
		}
	}
	event.Msg(msg)
}

// Debugf logs a debug message with the given fields
func (l *Logger) Debugf(msg string, fields ...interface{}) {
	event := l.logger.Debug()
	if len(fields) > 0 {
		msg = fmt.Sprintf(msg, fields...)
	}
	event.Msg(msg)
}

// Info logs an info message with the given fields
func (l *Logger) Info(msg string, fields ...map[string]interface{}) {
	event := l.logger.Info()
	if len(fields) > 0 {
		for k, v := range fields[0] {
			event = event.Interface(k, v)
		}
	}
	event.Msg(msg)
}

// Infof logs an info message with the given fields
func (l *Logger) Infof(msg string, fields ...interface{}) {
	event := l.logger.Info()
	if len(fields) > 0 {
		msg = fmt.Sprintf(msg, fields...)
	}
	event.Msg(msg)
}

// Warn logs a warning message with the given fields
func (l *Logger) Warn(msg string, fields ...map[string]interface{}) {
	event := l.logger.Warn()
	if len(fields) > 0 {
		for k, v := range fields[0] {
			event = event.Interface(k, v)
		}
	}
	event.Msg(msg)
}

// Warnf logs a warning message with the given fields
func (l *Logger) Warnf(msg string, fields ...interface{}) {
	event := l.logger.Warn()
	if len(fields) > 0 {
		msg = fmt.Sprintf(msg, fields...)
	}
	event.Msg(msg)
}

// Error logs an error message with the given fields
// It automatically adds stack information
func (l *Logger) Error(err error, msg string, fields ...map[string]interface{}) {
	event := l.logger.Error()

	// Add error information
	if err != nil {
		event = event.Err(err)

		// Add stack trace if available
		callStack := debug.GetStackTrace(1)
		if len(callStack) > 0 {
			event = event.Strs("stack", callStack)
		}
	}

	// Add additional fields
	if len(fields) > 0 {
		for k, v := range fields[0] {
			event = event.Interface(k, v)
		}
	}

	event.Msg(msg)
}

// Errorf logs an error message with the given fields
// It automatically adds stack information
func (l *Logger) Errorf(err error, msg string, fields ...interface{}) {
	event := l.logger.Error()

	// Add error information
	if err != nil {
		event = event.Err(err)

		// Add stack trace if available
		callStack := debug.GetStackTrace(1)
		if len(callStack) > 0 {
			event = event.Strs("stack", callStack)
		}
	}

	// Add additional fields
	if len(fields) > 0 {
		msg = fmt.Sprintf(msg, fields...)
	}

	event.Msg(msg)
}

// Fatal logs a fatal message with the given fields and exits the application
func (l *Logger) Fatal(err error, msg string, fields ...map[string]interface{}) {
	event := l.logger.Fatal()

	// Add error information
	if err != nil {
		event = event.Err(err)

		// Add stack trace if available
		callStack := debug.GetStackTrace(1)
		if len(callStack) > 0 {
			event = event.Strs("stack", callStack)
		}
	}

	// Add additional fields
	if len(fields) > 0 {
		for k, v := range fields[0] {
			event = event.Interface(k, v)
		}
	}

	event.Msg(msg)
}

// Fatalf logs a fatal message with the given fields and exits the application
func (l *Logger) Fatalf(err error, msg string, fields ...interface{}) {
	event := l.logger.Fatal()

	// Add error information
	if err != nil {
		event = event.Err(err)

		// Add stack trace if available
		callStack := debug.GetStackTrace(1)
		if len(callStack) > 0 {
			event = event.Strs("stack", callStack)
		}
	}

	// Add additional fields
	if len(fields) > 0 {
		msg = fmt.Sprintf(msg, fields...)
	}

	event.Msg(msg)
}

// GetTraceID returns the trace ID associated with this logger
func (l *Logger) GetTraceID() string {
	return l.traceID
}

// GetZerolog returns the underlying zerolog.Logger
func (l *Logger) GetZerolog() zerolog.Logger {
	return l.logger
}
