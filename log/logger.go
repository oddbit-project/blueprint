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
	LogFmtPretty   = "pretty"
	LogFmtJson     = "json"
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
	TimeFormat       string `json:"timeFormat"`      // Time format string
	NoColor          bool   `json:"noColor"`         // if true, disable color
	OutputToFile     bool   `json:"outputToFile"`    // Enable file output
	FilePath         string `json:"filePath"`        // Path to log file
	FileAppend       bool   `json:"fileAppend"`      // Append to existing file
	FilePermissions  int    `json:"filePermissions"` // File permissions (e.g., 0644)
	FileFormat       string `json:"fileFormat"`      // file format, "pretty" or json
	FileRotation     bool   `json:"fileRotation"`    // Enable log rotation
	MaxSizeMb        int    `json:"maxSizeMb"`       // Max size in MB before rotation
	MaxBackups       int    `json:"maxBackups"`      // Max number of rotated files to keep
	MaxAgeDays       int    `json:"maxAgeDays"`      // Max age in days to keep rotated files
	Compress         bool   `json:"compress"`        // Compress rotated files
}

// track open log files
var openLogFiles []*os.File
var ljack *lumberjack.Logger

// NewDefaultConfig returns a default logging configuration
func NewDefaultConfig() *Config {
	return &Config{
		Level:            "info",
		Format:           LogFmtPretty,
		IncludeTimestamp: true,
		IncludeCaller:    false,
		IncludeHostname:  true,
		CallerSkipFrames: LogCallerSkipFrames,
		TimeFormat:       LogTimestampFormat,
		NoColor:          false,
		OutputToFile:     false,
		FileAppend:       true,
		FilePermissions:  0644,
		FilePath:         DefaultLogFile,
		FileRotation:     false,
		FileFormat:       LogFmtJson,
		MaxSizeMb:        100, // 100 MB
		MaxBackups:       5,   // 5 backup files
		MaxAgeDays:       30,  // 30 days
		Compress:         true,
	}
}

// EnableFileOutput enables file logging with the given file path
func EnableFileOutput(cfg *Config, filePath string) *Config {
	cfg.OutputToFile = true
	cfg.FilePath = filePath
	return cfg
}

// SetFileFormat sets the output format for file logging
func SetFileFormat(cfg *Config, format string) *Config {
	cfg.FileFormat = format
	return cfg
}

// DisableFileAppend disables appending to existing log files (will overwrite)
func DisableFileAppend(cfg *Config) *Config {
	cfg.FileAppend = false
	return cfg
}

// buildLogWriter configures the logger output based on config
func buildLogWriter(cfg *Config) (io.Writer, error) {
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
		return consoleWriter, nil
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
		fileFlags := os.O_CREATE | os.O_WRONLY
		if cfg.FileAppend {
			fileFlags |= os.O_APPEND
		} else {
			fileFlags |= os.O_TRUNC
		}
		file, err := os.OpenFile(cfg.FilePath, fileFlags, os.FileMode(cfg.FilePermissions))
		if err != nil {
			return nil, fmt.Errorf("failed to open log file %s: %w", cfg.FilePath, err)
		}

		if cfg.FileFormat == LogFmtPretty {
			fileWriter = zerolog.NewConsoleWriter(func(w *zerolog.ConsoleWriter) {
				w.TimeFormat = cfg.TimeFormat
				w.Out = file
			})
		} else {
			fileWriter = file
		}

		// Track the file for closing on shutdown
		openLogFiles = append(openLogFiles, file)
	}

	// Combine console and file writers
	return zerolog.MultiLevelWriter(consoleWriter, fileWriter), nil
}

// Validate validate log configuration
func (c *Config) Validate() error {
	// Set global log level
	_, err := zerolog.ParseLevel(c.Level)
	if err != nil {
		return fmt.Errorf("invalid log level: %s", c.Level)
	}

	validFormats := []string{LogFmtPretty, LogFmtJson}
	if !slices.Contains(validFormats, c.Format) {
		return fmt.Errorf("invalid log format: %s", c.Format)
	}

	if c.CallerSkipFrames < 0 {
		return fmt.Errorf("invalid caller skip frames: %d", c.CallerSkipFrames)
	}

	if len(c.TimeFormat) == 0 {
		return fmt.Errorf("missing time format string")
	}

	if c.OutputToFile {
		if !slices.Contains(validFormats, c.FileFormat) {
			return fmt.Errorf("invalid file log format: %s", c.FileFormat)
		}
	}

	return nil
}

// Logger returns a new logger instance based on the configuration
func (c *Config) Logger() (*Logger, error) {
	hostname, _ := os.Hostname()

	level, err := zerolog.ParseLevel(c.Level)
	if err != nil {
		return nil, fmt.Errorf("invalid log level: %s", c.Level)
	}
	zerolog.SetGlobalLevel(level)

	// Configure timestamp format
	zerolog.TimeFieldFormat = c.TimeFormat

	// build output writer
	var output io.Writer
	if output, err = buildLogWriter(c); err != nil {
		return nil, err
	}

	// Configure base logger
	logger := zerolog.New(output).Level(level)

	// Add standard fields
	if c.IncludeTimestamp {
		logger = logger.With().Timestamp().Logger()
	}

	if c.IncludeCaller {
		logger = logger.With().Caller().Logger()
		zerolog.CallerSkipFrameCount = c.CallerSkipFrames
	}

	return &Logger{
		logger:     logger,
		moduleInfo: "",
		hostname:   hostname,
	}, nil
}

// ModuleLogger returns a new module logger based on the configuration
func (c *Config) ModuleLogger(module string) (*Logger, error) {
	l, err := c.Logger()
	if err != nil {
		return nil, err
	}
	hostname, _ := os.Hostname()
	return &Logger{
		logger:     l.logger.With().Str(LogModuleKey, module).Logger(),
		moduleInfo: module,
		hostname:   hostname,
	}, nil
}

// Configure configures the global logger based on the provided configuration
func Configure(cfg *Config) error {
	logger, err := cfg.Logger()
	if err != nil {
		return err
	}

	// Set as global logger
	log.Logger = logger.logger

	return nil
}

// New creates a new logger from global logger with module information
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
func (l *Logger) Debugf(msg string, fields ...any) {
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
func (l *Logger) Infof(msg string, fields ...any) {
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
func (l *Logger) Warnf(msg string, fields ...any) {
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
		callStack := debug.GetStackTrace(2)
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
func (l *Logger) Errorf(err error, msg string, fields ...any) {
	event := l.logger.Error()

	// Add error information
	if err != nil {
		event = event.Err(err)

		// Add stack trace if available
		callStack := debug.GetStackTrace(2)
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
		callStack := debug.GetStackTrace(2)
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
func (l *Logger) Fatalf(err error, msg string, fields ...any) {
	event := l.logger.Fatal()

	// Add error information
	if err != nil {
		event = event.Err(err)

		// Add stack trace if available
		callStack := debug.GetStackTrace(2)
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
