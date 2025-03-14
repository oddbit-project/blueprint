package log

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileLoggerHelpers(t *testing.T) {
	cfg := NewDefaultConfig()

	t.Run("EnableFileOutput", func(t *testing.T) {
		filePath := "/path/to/logs/app.log"
		cfg = EnableFileOutput(cfg, filePath)

		assert.True(t, cfg.OutputToFile)
		assert.Equal(t, filePath, cfg.FilePath)
	})

	t.Run("SetFileFormat", func(t *testing.T) {
		cfg = SetFileFormat(cfg, "console")
		assert.Equal(t, "console", cfg.FileFormat)
	})

	t.Run("DisableFileAppend", func(t *testing.T) {
		cfg = DisableFileAppend(cfg)
		assert.False(t, cfg.FileAppend)
	})
}

func TestFileLogging(t *testing.T) {
	// Create a temporary directory for log files
	tempDir, err := os.MkdirTemp("", "filelogger_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Setup a log file path
	logFile := filepath.Join(tempDir, "test.log")

	// Run sub-tests
	t.Run("JSON file logging", func(t *testing.T) {
		// Configure with file output
		cfg := NewDefaultConfig()
		cfg = EnableFileOutput(cfg, logFile)
		cfg = DisableFileAppend(cfg) // Start with a clean file
		cfg.Level = "debug"

		err := Configure(cfg)
		require.NoError(t, err)

		// Log some messages
		logger := New("filetest")
		logger.Info("Test info message")
		logger.Debug("Test debug message")
		logger.Warn("Test warning message")
		logger.Error(nil, "Test error message")

		// Close log files
		CloseLogFiles()

		// Read log file contents
		content, err := os.ReadFile(logFile)
		require.NoError(t, err)
		require.NotEmpty(t, content)

		// Split into lines (each line is a JSON object)
		lines := splitJsonLines(t, content)
		require.Equal(t, 4, len(lines), "Expected 4 log entries")

		// Verify each log entry
		assertLogEntry(t, lines[0], "info", "Test info message", "filetest")
		assertLogEntry(t, lines[1], "debug", "Test debug message", "filetest")
		assertLogEntry(t, lines[2], "warn", "Test warning message", "filetest")
		assertLogEntry(t, lines[3], "error", "Test error message", "filetest")
	})

	t.Run("Console format file logging", func(t *testing.T) {
		// Configure with console format file output
		cfg := NewDefaultConfig()
		cfg = EnableFileOutput(cfg, logFile)
		cfg = DisableFileAppend(cfg) // Start with a clean file
		cfg = SetFileFormat(cfg, "console")
		cfg.Level = "info"

		err := Configure(cfg)
		require.NoError(t, err)

		// Log some messages
		logger := New("consolefiletest")
		logger.Info("Test console file message")

		// Close log files
		CloseLogFiles()

		// Read log file contents
		content, err := os.ReadFile(logFile)
		require.NoError(t, err)
		require.NotEmpty(t, content)

		// For console format, just check that the message is present
		// (not checking JSON structure since it's formatted for console)
		require.Contains(t, string(content), "Test console file message")
		require.Contains(t, string(content), "consolefiletest")
	})

	t.Run("Append mode file logging", func(t *testing.T) {
		// First create a file with some content
		initialContent := "Initial log content\n"
		err := os.WriteFile(logFile, []byte(initialContent), 0644)
		require.NoError(t, err)

		// Configure with append mode
		cfg := NewDefaultConfig()
		cfg = EnableFileOutput(cfg, logFile)
		// cfg.FileAppend is true by default
		cfg.Level = "info"

		err = Configure(cfg)
		require.NoError(t, err)

		// Log a message
		logger := New("appendtest")
		logger.Info("Appended log message")

		// Close log files
		CloseLogFiles()

		// Read log file contents
		content, err := os.ReadFile(logFile)
		require.NoError(t, err)

		// Check both initial content and new log are present
		contentStr := string(content)
		require.Contains(t, contentStr, initialContent)
		require.Contains(t, contentStr, "Appended log message")
	})

	t.Run("Invalid file path", func(t *testing.T) {
		// Try to log to an invalid path
		invalidPath := filepath.Join(tempDir, "nonexistent", "invalid.log")

		cfg := NewDefaultConfig()
		cfg = EnableFileOutput(cfg, invalidPath)

		// This should return an error
		err := Configure(cfg)
		require.Error(t, err)
	})
}

// Helper functions for testing

// splitJsonLines splits the content into separate JSON objects
func splitJsonLines(t *testing.T, content []byte) []map[string]interface{} {
	t.Helper()

	// Split by newline
	lines := splitLines(string(content))
	result := make([]map[string]interface{}, 0, len(lines))

	for _, line := range lines {
		if line == "" {
			continue
		}

		var entry map[string]interface{}
		err := json.Unmarshal([]byte(line), &entry)
		require.NoError(t, err, "Log entry should be valid JSON: %s", line)

		result = append(result, entry)
	}

	return result
}

// splitLines splits a string by newlines
func splitLines(s string) []string {
	var lines []string
	var line string

	for _, r := range s {
		if r == '\n' {
			lines = append(lines, line)
			line = ""
		} else {
			line += string(r)
		}
	}

	if line != "" {
		lines = append(lines, line)
	}

	return lines
}

// assertLogEntry checks that a log entry has the expected level, message and module
func assertLogEntry(t *testing.T, entry map[string]interface{}, level, message, module string) {
	t.Helper()
	assert.Equal(t, level, entry["level"])
	assert.Equal(t, message, entry["message"])
	assert.Equal(t, module, entry["module"])
}
