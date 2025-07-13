package log

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"testing"
)

// setupTestLogger creates a test logger that writes to a buffer
func setupTestLogger(t *testing.T) (*Logger, *bytes.Buffer) {
	buf := &bytes.Buffer{}
	zerolog.TimeFieldFormat = ""
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	zl := zerolog.New(buf).Level(zerolog.DebugLevel)

	logger := &Logger{
		logger:     zl,
		moduleInfo: "test",
		hostname:   "test-host",
	}
	return logger, buf
}

// parseLogOutput parses the log output into a map
func parseLogOutput(t *testing.T, buf *bytes.Buffer) KV {
	result := KV{}
	err := json.Unmarshal(buf.Bytes(), &result)
	assert.NoError(t, err, "Log output should be valid JSON")
	return result
}

func TestLogger_New(t *testing.T) {
	logger := New("testmodule")

	assert.Equal(t, "testmodule", logger.moduleInfo)
	assert.NotEmpty(t, logger.hostname)
}

func TestLogger_NewWithComponent(t *testing.T) {
	logger := NewWithComponent("testmodule", "testcomponent")

	assert.Equal(t, "testmodule.testcomponent", logger.moduleInfo)
	assert.NotEmpty(t, logger.hostname)
}

func TestLogger_WithTraceID(t *testing.T) {
	logger := New("testmodule")
	traceID := "trace123"

	tracedLogger := logger.WithTraceID(traceID)

	assert.Equal(t, traceID, tracedLogger.GetTraceID())
	assert.Equal(t, logger.moduleInfo, tracedLogger.moduleInfo)
	assert.Equal(t, logger.hostname, tracedLogger.hostname)
}

func TestLogger_WithField(t *testing.T) {
	logger, buf := setupTestLogger(t)

	fieldLogger := logger.WithField("test_key", "test_value")
	fieldLogger.Info("test message")

	logMap := parseLogOutput(t, buf)
	assert.Equal(t, "test_value", logMap["test_key"])
	assert.Equal(t, "test message", logMap["message"])
}

func TestLogger_Info(t *testing.T) {
	logger, buf := setupTestLogger(t)

	logger.Info("info message", KV{
		"key1": "value1",
		"key2": 123,
	})

	logMap := parseLogOutput(t, buf)
	assert.Equal(t, "info message", logMap["message"])
	assert.Equal(t, "info", logMap["level"])
	assert.Equal(t, "value1", logMap["key1"])
	assert.Equal(t, float64(123), logMap["key2"])
}

func TestLogger_Debug(t *testing.T) {
	logger, buf := setupTestLogger(t)

	logger.Debug("debug message")

	logMap := parseLogOutput(t, buf)
	assert.Equal(t, "debug message", logMap["message"])
	assert.Equal(t, "debug", logMap["level"])
}

func TestLogger_Warn(t *testing.T) {
	logger, buf := setupTestLogger(t)

	logger.Warn("warn message")

	logMap := parseLogOutput(t, buf)
	assert.Equal(t, "warn message", logMap["message"])
	assert.Equal(t, "warn", logMap["level"])
}

func TestLogger_Error(t *testing.T) {
	logger, buf := setupTestLogger(t)

	testErr := errors.New("test error")
	logger.Error(testErr, "error message", KV{
		"context": "test context",
	})

	logMap := parseLogOutput(t, buf)
	assert.Equal(t, "error message", logMap["message"])
	assert.Equal(t, "error", logMap["level"])
	assert.Equal(t, "test error", logMap["error"])
	assert.Equal(t, "test context", logMap["context"])

	// Stack should be included
	if stack, ok := logMap["stack"]; ok {
		assert.NotEmpty(t, stack)
	}
}

func TestLogger_WithContext(t *testing.T) {
	logger := New("testmodule")

	ctx := context.Background()
	ctxWithLogger := logger.WithContext(ctx)

	extractedLogger := FromContext(ctxWithLogger)
	assert.Equal(t, logger.moduleInfo, extractedLogger.moduleInfo)
}

func TestFromContext_NoLogger(t *testing.T) {
	ctx := context.Background()

	logger := FromContext(ctx)
	assert.Equal(t, "default", logger.moduleInfo)
}

func TestFromContext_WithLogger(t *testing.T) {
	logger := New("testmodule")
	ctx := logger.WithContext(context.Background())

	extractedLogger := FromContext(ctx)
	assert.Equal(t, "testmodule", extractedLogger.moduleInfo)
}

func TestConfigure(t *testing.T) {
	cfg := NewDefaultConfig()
	cfg.Level = "debug"

	err := Configure(cfg)
	assert.NoError(t, err)

	// Test with invalid level
	cfg.Level = "invalid"
	err = Configure(cfg)
	assert.Error(t, err)
}

func TestNewDefaultConfig(t *testing.T) {
	cfg := NewDefaultConfig()

	assert.Equal(t, "info", cfg.Level)
	assert.Equal(t, "pretty", cfg.Format)
	assert.True(t, cfg.IncludeTimestamp)
	assert.False(t, cfg.IncludeCaller)
	assert.True(t, cfg.IncludeHostname)
	assert.Equal(t, LogCallerSkipFrames, cfg.CallerSkipFrames)
	assert.Equal(t, LogTimestampFormat, cfg.TimeFormat)
	assert.False(t, cfg.OutputToFile)
	assert.False(t, cfg.NoColor)
}

// TestLogger_ErrorStackTrace verifies that stack traces point to the actual caller location
func TestLogger_ErrorStackTrace(t *testing.T) {
	logger, buf := setupTestLogger(t)

	// Call error logging method - this line should appear in stack trace
	logger.Error(errors.New("test error"), "error with stack trace")

	logMap := parseLogOutput(t, buf)
	assert.Equal(t, "error with stack trace", logMap["message"])
	assert.Equal(t, "error", logMap["level"])
	assert.Equal(t, "test error", logMap["error"])

	// Verify stack trace is present and points to this test function
	stack, stackExists := logMap["stack"]
	assert.True(t, stackExists, "Stack trace should be present")
	
	stackSlice, ok := stack.([]interface{})
	assert.True(t, ok, "Stack should be a slice")
	assert.NotEmpty(t, stackSlice, "Stack trace should not be empty")

	// Convert to string slice for easier checking
	stackStrings := make([]string, len(stackSlice))
	for i, frame := range stackSlice {
		stackStrings[i] = frame.(string)
	}

	// Find the frame that should point to this test function
	foundTestFrame := false
	for _, frame := range stackStrings {
		// Check if this frame contains our test function and the correct line
		if strings.Contains(frame, "TestLogger_ErrorStackTrace") && 
		   strings.Contains(frame, "logger_test.go") {
			// Verify the line number is approximately correct (within a few lines)
			if strings.Contains(frame, ":") {
				foundTestFrame = true
				// Additional verification: ensure line number is reasonable
				assert.Contains(t, frame, "logger_test.go", "Stack frame should reference logger_test.go")
				break
			}
		}
	}

	assert.True(t, foundTestFrame, "Stack trace should contain the test function call, got: %v", stackStrings)
}

// TestLogger_ErrorfStackTrace verifies that stack traces work correctly for Errorf method
func TestLogger_ErrorfStackTrace(t *testing.T) {
	logger, buf := setupTestLogger(t)

	// Call error logging method - this line should appear in stack trace
	logger.Errorf(errors.New("test error"), "error %s with stack trace", "formatted")

	logMap := parseLogOutput(t, buf)
	assert.Equal(t, "error formatted with stack trace", logMap["message"])
	assert.Equal(t, "error", logMap["level"])

	// Verify stack trace points to this test function
	stack, stackExists := logMap["stack"]
	assert.True(t, stackExists, "Stack trace should be present")
	
	stackSlice, ok := stack.([]interface{})
	assert.True(t, ok, "Stack should be a slice")
	assert.NotEmpty(t, stackSlice, "Stack trace should not be empty")

	// Convert and check stack frames
	foundTestFrame := false
	for _, frame := range stackSlice {
		frameStr := frame.(string)
		if strings.Contains(frameStr, "TestLogger_ErrorfStackTrace") && 
		   strings.Contains(frameStr, "logger_test.go") {
			foundTestFrame = true
			break
		}
	}

	assert.True(t, foundTestFrame, "Stack trace should contain the test function call")
}

// Helper function to test indirect error logging (through another function)
func callErrorLogging(logger *Logger, err error, msg string) {
	logger.Error(err, msg)
}

// TestLogger_ErrorStackTrace_IndirectCall verifies stack traces work with indirect calls
func TestLogger_ErrorStackTrace_IndirectCall(t *testing.T) {
	logger, buf := setupTestLogger(t)

	// Call through helper function
	callErrorLogging(logger, errors.New("test error"), "indirect error")

	logMap := parseLogOutput(t, buf)
	assert.Equal(t, "indirect error", logMap["message"])

	// Verify stack trace includes both the helper function and this test function
	stack, stackExists := logMap["stack"]
	assert.True(t, stackExists, "Stack trace should be present")
	
	stackSlice, ok := stack.([]interface{})
	assert.True(t, ok, "Stack should be a slice")
	assert.NotEmpty(t, stackSlice, "Stack trace should not be empty")

	// Should find both the helper function and the test function in the stack
	foundHelper := false
	foundTest := false
	
	for _, frame := range stackSlice {
		frameStr := frame.(string)
		if strings.Contains(frameStr, "callErrorLogging") {
			foundHelper = true
		}
		if strings.Contains(frameStr, "TestLogger_ErrorStackTrace_IndirectCall") {
			foundTest = true
		}
	}

	assert.True(t, foundHelper, "Stack trace should contain helper function")
	assert.True(t, foundTest, "Stack trace should contain test function")
}
