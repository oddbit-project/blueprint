package log

import (
	"context"
	"fmt"
	"github.com/google/uuid"
)

// ContextFields is a map of fields to add to log messages
type ContextFields map[string]interface{}

// MergeContextFields merges multiple context fields maps into a single map
func MergeContextFields(fieldSets ...ContextFields) ContextFields {
	result := make(ContextFields)

	for _, fields := range fieldSets {
		for k, v := range fields {
			result[k] = v
		}
	}

	return result
}

// NewTraceID generates a new trace ID for distributed tracing
func NewTraceID() string {
	return uuid.New().String()
}

// NewRequestContext creates a new context with a logger that has a trace ID
// This is useful for tracking requests through multiple services
func NewRequestContext(parentCtx context.Context, moduleName string) (context.Context, *Logger) {
	traceID := NewTraceID()
	logger := New(moduleName).WithTraceID(traceID)
	ctx := logger.WithContext(parentCtx)
	return ctx, logger
}

// ExtractLoggerFromContext extracts a logger from the given context
// If no logger is found, a new default logger is created
func ExtractLoggerFromContext(ctx context.Context) *Logger {
	if ctx == nil {
		return New("default")
	}

	logger, ok := ctx.Value(LogContextKey).(*Logger)
	if !ok {
		return New("default")
	}

	return logger
}

// WithField adds a field to the logger in the context and returns the updated context
func WithField(ctx context.Context, key string, value interface{}) context.Context {
	logger := ExtractLoggerFromContext(ctx)
	updatedLogger := logger.WithField(key, value)
	return updatedLogger.WithContext(ctx)
}

// WithFields adds multiple fields to the logger in the context and returns the updated context
func WithFields(ctx context.Context, fields ContextFields) context.Context {
	logger := ExtractLoggerFromContext(ctx)

	for k, v := range fields {
		logger = logger.WithField(k, v)
	}

	return logger.WithContext(ctx)
}

// Debug logs a debug message with the logger from the context
func Debug(ctx context.Context, msg string, fields ...ContextFields) {
	logger := ExtractLoggerFromContext(ctx)
	if len(fields) > 0 {
		logger.Debug(msg, fields[0])
	} else {
		logger.Debug(msg)
	}
}

// Debugf logs a debug message with the logger from the context
func Debugf(ctx context.Context, msg string, args ...any) {
	logger := ExtractLoggerFromContext(ctx)
	if len(args) > 0 {
		logger.Debug(fmt.Sprintf(msg, args...))
	} else {
		logger.Debug(msg)
	}
}

// Info logs an info message with the logger from the context
func Info(ctx context.Context, msg string, fields ...ContextFields) {
	logger := ExtractLoggerFromContext(ctx)
	if len(fields) > 0 {
		logger.Info(msg, fields[0])
	} else {
		logger.Info(msg)
	}
}

// Infof logs an info message with the logger from the context
func Infof(ctx context.Context, msg string, args ...any) {
	logger := ExtractLoggerFromContext(ctx)
	if len(args) > 0 {
		logger.Info(fmt.Sprintf(msg, args...), nil)
	} else {
		logger.Info(msg)
	}
}

// Warn logs a warning message with the logger from the context
func Warn(ctx context.Context, msg string, fields ...ContextFields) {
	logger := ExtractLoggerFromContext(ctx)
	if len(fields) > 0 {
		logger.Warn(msg, fields[0])
	} else {
		logger.Warn(msg)
	}
}

// Warnf logs a warning message with the logger from the context
func Warnf(ctx context.Context, msg string, args ...any) {
	logger := ExtractLoggerFromContext(ctx)
	if len(args) > 0 {
		logger.Warn(fmt.Sprintf(msg, args...), nil)
	} else {
		logger.Warn(msg)
	}
}

// Error logs an error message with the logger from the context
func Error(ctx context.Context, err error, msg string, fields ...ContextFields) {
	logger := ExtractLoggerFromContext(ctx)
	if len(fields) > 0 {
		logger.Error(err, msg, fields[0])
	} else {
		logger.Error(err, msg)
	}
}

// Errorf logs an error message with the logger from the context
func Errorf(ctx context.Context, err error, msg string, args ...any) {
	logger := ExtractLoggerFromContext(ctx)
	if len(args) > 0 {
		logger.Error(err, fmt.Sprintf(msg, args...), nil)
	} else {
		logger.Error(err, msg)
	}
}

// Fatal logs a fatal message with the logger from the context
func Fatal(ctx context.Context, err error, msg string, fields ...ContextFields) {
	logger := ExtractLoggerFromContext(ctx)
	if len(fields) > 0 {
		logger.Fatal(err, msg, fields[0])
	} else {
		logger.Fatal(err, msg)
	}
}

// Fatalf logs a fatal message with the logger from the context
func Fatalf(ctx context.Context, err error, msg string, args ...any) {
	logger := ExtractLoggerFromContext(ctx)
	if len(args) > 0 {
		logger.Fatal(err, fmt.Sprintf(msg, args...), nil)
	} else {
		logger.Fatal(err, msg)
	}
}
