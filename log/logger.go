package log

import (
	"context"
	"fmt"
	"github.com/oddbit-project/blueprint"
	"github.com/oddbit-project/blueprint/utils/debug"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gopkg.in/natefinch/lumberjack.v2" // For log rotation
	"io"
	"os"
	"slices"
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

	DefaultLogFile = "application.log"
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
	Format           string `json:"format"` // "pretty" or "json"
	IncludeTimestamp bool   `json:"includeTimestamp"`
	IncludeCaller    bool   `json:"includeCaller"`
	IncludeHostname  bool   `json:"includeHostname"`
	CallerSkipFrames int    `json:"callerSkipFrames"`
	TimeFormat       string `json:"timeFormat"`   // Time format string
	NoColor          bool   `json:"noColor"`      // if true, disable color
	OutputToFile     bool   `json:"outputToFile"` // Enable file output
	FilePath         string `json:"filePath"`     // Path to log file
	FileRotation     bool   `json:"fileRotation"` // Enable log rotation
	MaxSizeMb        int    `json:"maxSizeMb"`    // Max size in MB before rotation
	MaxBackups       int    `json:"maxBackups"`   // Max number of rotated files to keep
	MaxAgeDays       int    `json:"maxAgeDays"`   // Max age in days to keep rotated files
	Compress         bool   `json:"compress"`     // Compress rotated files
}

// track open log files
var openLogFiles []*os.File
var ljack *lumberjack.Logger

// NewDefaultConfig returns a default logging configuration
func NewDefaultConfig() *Config {
	return &Config{
		Level:            "info",
		Format:           "pretty",
		IncludeTimestamp: true,
		IncludeCaller:    false,
		IncludeHostname:  true,
		CallerSkipFrames: LogCallerSkipFrames,
		TimeFormat:       LogTimestampFormat,
		NoColor:          false,
		OutputToFile:     false,
		FilePath:         DefaultLogFile,
		FileRotation:     true,
		MaxSizeMb:        100, // 100 MB
		MaxBackups:       5,   // 5 backup files
		MaxAgeDays:       30,  // 30 days
		Compress:         true,
	}
}

// buildLogWriter configures the logger output based on config
func buildLogWriter(cfg *Config) io.Writer {
	var consoleWriter io.Writer

	// Set up console writer
	consoleWriter = os.Stdout
	if cfg.Format == "pretty" {
		consoleWriter = zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: cfg.TimeFormat,
			NoColor:    cfg.NoColor,
		}
	} else {
		consoleWriter = os.Stdout
	}

	// If file output is not enabled, just return console writer
	if !cfg.OutputToFile {
		return consoleWriter
	}

	// Set up file writer with rotation if enabled
	var fileWriter io.Writer
	if cfg.FileRotation {
		ljack = &lumberjack.Logger{
			Filename:   cfg.FilePath,
			MaxSize:    cfg.MaxSizeMb,
			MaxBackups: cfg.MaxBackups,
			MaxAge:     cfg.MaxAgeDays,
			Compress:   cfg.Compress,
		}
		fileWriter = ljack
	} else {
		// Simple file without rotation
		file, err := os.OpenFile(cfg.FilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			// Fall back to console if file can't be opened
			log.Err(err).Msg("Failed to open log file, falling back to console only")
			return consoleWriter
		}
		fileWriter = file

		// Track the file for closing on shutdown
		openLogFiles = append(openLogFiles, file)
	}

	// Combine console and file writers
	return zerolog.MultiLevelWriter(consoleWriter, fileWriter)
}

// Validate validate log configuration
func (c *Config) Validate() error {
	// Set global log level
	_, err := zerolog.ParseLevel(c.Level)
	if err != nil {
		return fmt.Errorf("invalid log level: %s", c.Level)
	}

	validFormats := []string{"pretty", "json"}
	if !slices.Contains(validFormats, c.Format) {
		return fmt.Errorf("invalid log format: %s", c.Format)
	}

	if c.CallerSkipFrames < 0 {
		return fmt.Errorf("invalid caller skip frames: %d", c.CallerSkipFrames)
	}

	if len(c.TimeFormat) == 0 {
		return fmt.Errorf("missing time format string")
	}
	return nil
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
	zerolog.TimeFieldFormat = cfg.TimeFormat

	// build output writer
	output := buildLogWriter(cfg)

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

// CloseLogFiles close log files
func CloseLogFiles() error {
	var lastErr error
	for _, file := range openLogFiles {
		if err := file.Close(); err != nil {
			lastErr = err
		}
	}
	openLogFiles = nil

	if ljack != nil {
		// close lumberjack files
		return ljack.Close()
	}
	return lastErr
}

// Register shutdown hook
func init() {
	openLogFiles = make([]*os.File, 0)

	// Register a destructor to close log files
	blueprint.RegisterDestructor(func() error {
		return CloseLogFiles()
	})
}
