package log

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestNewTraceID(t *testing.T) {
	traceID := NewTraceID()
	assert.NotEmpty(t, traceID)

	// Generate another one to ensure they're different
	traceID2 := NewTraceID()
	assert.NotEqual(t, traceID, traceID2)
}

func TestNewRequestContext(t *testing.T) {
	moduleName := "test"
	ctx := context.Background()
	
	newCtx, logger := NewRequestContext(ctx, moduleName)
	
	// Verify the logger has a trace ID
	assert.NotEmpty(t, logger.GetTraceID())
	
	// Verify the logger is in the context
	extractedLogger := ExtractLoggerFromContext(newCtx)
	assert.Equal(t, logger.GetTraceID(), extractedLogger.GetTraceID())
	assert.Equal(t, moduleName, extractedLogger.moduleInfo)
}

func TestExtractLoggerFromContext_NilContext(t *testing.T) {
	logger := ExtractLoggerFromContext(nil)
	assert.NotNil(t, logger)
	assert.Equal(t, "default", logger.moduleInfo)
}

func TestExtractLoggerFromContext_NoLogger(t *testing.T) {
	ctx := context.Background()
	logger := ExtractLoggerFromContext(ctx)
	assert.NotNil(t, logger)
	assert.Equal(t, "default", logger.moduleInfo)
}

func TestExtractLoggerFromContext_WithLogger(t *testing.T) {
	originalLogger := New("test")
	ctx := originalLogger.WithContext(context.Background())
	
	extractedLogger := ExtractLoggerFromContext(ctx)
	assert.Equal(t, originalLogger.moduleInfo, extractedLogger.moduleInfo)
}

func TestWithField(t *testing.T) {
	// Create a test logger with a buffer for immediate capture
	buf := &bytes.Buffer{}
	logger := &Logger{
		logger:     zerolog.New(buf),
		moduleInfo: "test",
	}
	ctx := logger.WithContext(context.Background())
	
	// Add field and log directly
	newCtx := WithField(ctx, "key", "value")
	
	// Log using the updated context
	Info(newCtx, "test message")
	
	// Parse and verify
	logMap := map[string]interface{}{}
	err := json.Unmarshal(buf.Bytes(), &logMap)
	assert.NoError(t, err)
	
	assert.Equal(t, "test message", logMap["message"])
	assert.Equal(t, "value", logMap["key"])
}

func TestWithFields(t *testing.T) {
	// Create a test logger with a buffer for immediate capture
	buf := &bytes.Buffer{}
	logger := &Logger{
		logger:     zerolog.New(buf),
		moduleInfo: "test",
	}
	ctx := logger.WithContext(context.Background())
	
	fields := ContextFields{
		"key1": "value1",
		"key2": 123,
	}
	
	// Add fields and log directly
	newCtx := WithFields(ctx, fields)
	
	// Log using the updated context
	Info(newCtx, "test message")
	
	// Parse and verify
	logMap := map[string]interface{}{}
	err := json.Unmarshal(buf.Bytes(), &logMap)
	assert.NoError(t, err)
	
	assert.Equal(t, "test message", logMap["message"])
	assert.Equal(t, "value1", logMap["key1"])
	assert.Equal(t, float64(123), logMap["key2"])
}

func TestMergeContextFields(t *testing.T) {
	fields1 := ContextFields{
		"key1": "value1",
		"key2": 123,
	}
	
	fields2 := ContextFields{
		"key2": 456, // This should override the previous value
		"key3": true,
	}
	
	merged := MergeContextFields(fields1, fields2)
	
	assert.Equal(t, 3, len(merged))
	assert.Equal(t, "value1", merged["key1"])
	assert.Equal(t, 456, merged["key2"]) // Should have the value from fields2
	assert.Equal(t, true, merged["key3"])
}

func TestContextLoggingFunctions(t *testing.T) {
	// Create a context that will be overridden in the test loop
	_ = context.Background()
	
	testCases := []struct {
		name     string
		logFunc  func(context.Context, string, ...ContextFields)
		level    string
		message  string
		hasError bool
	}{
		{
			name:    "Debug",
			logFunc: Debug,
			level:   "debug",
			message: "debug message",
		},
		{
			name:    "Info",
			logFunc: Info,
			level:   "info",
			message: "info message",
		},
		{
			name:    "Warn",
			logFunc: Warn,
			level:   "warn",
			message: "warn message",
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create logger with a buffer
			buf := &bytes.Buffer{}
			logger := &Logger{
				logger:     zerolog.New(buf),
				moduleInfo: "test",
			}
			
			// Add logger to context
			ctx := logger.WithContext(context.Background())
			
			// Call the log function
			fields := ContextFields{"test_key": "test_value"}
			tc.logFunc(ctx, tc.message, fields)
			
			// Parse output
			logMap := map[string]interface{}{}
			err := json.Unmarshal(buf.Bytes(), &logMap)
			assert.NoError(t, err)
			
			// Check output
			assert.Equal(t, tc.level, logMap["level"])
			assert.Equal(t, tc.message, logMap["message"])
			assert.Equal(t, "test_value", logMap["test_key"])
		})
	}
}

func TestErrorAndFatalContextLogging(t *testing.T) {
	// Test Error
	buf := &bytes.Buffer{}
	logger := &Logger{
		logger:     zerolog.New(buf),
		moduleInfo: "test",
	}
	ctx := logger.WithContext(context.Background())
	
	testError := errors.New("test error")
	Error(ctx, testError, "error message", ContextFields{"test_key": "test_value"})
	
	logMap := map[string]interface{}{}
	err := json.Unmarshal(buf.Bytes(), &logMap)
	assert.NoError(t, err)
	
	assert.Equal(t, "error", logMap["level"])
	assert.Equal(t, "error message", logMap["message"])
	assert.Equal(t, "test error", logMap["error"])
	assert.Equal(t, "test_value", logMap["test_key"])
	
	// Fatal can't be fully tested without exiting the program
	// but we can at least verify it generates the expected log
	// We don't call the actual Fatal function to avoid exiting
}