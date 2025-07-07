package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/oddbit-project/blueprint/log"
)

func main() {
	// Create logs directory if it doesn't exist
	logsDir := "logs"
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		fmt.Printf("Error creating logs directory: %v\n", err)
		os.Exit(1)
	}

	// Generate a timestamp-based filename for the log file
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	logFilePath := filepath.Join(logsDir, fmt.Sprintf("application_%s.log", timestamp))

	// Create a logger configuration
	logConfig := log.NewDefaultConfig()

	// Enable file logging with the generated path
	log.EnableFileOutput(logConfig, logFilePath)

	// Optional: Set file format to console-friendly format (remove for JSON)
	log.SetFileFormat(logConfig, "console")

	// Configure the logger
	if err := log.Configure(logConfig); err != nil {
		fmt.Printf("Error configuring logger: %v\n", err)
		os.Exit(1)
	}

	// Create a base context
	ctx := context.Background()

	// Create a logger for this application
	logger := log.New("filelogger-sample")

	// Log some messages
	logger.Info("Application started", log.KV{
		"timestamp": time.Now().String(),
		"logfile":   logFilePath,
	})

	logger.Debug("This is a debug message with context", log.KV{
		"detail": "Some debug information",
		"value":  42,
	})

	// Demonstrate context-based logging
	ctxWithLogger := logger.WithContext(ctx)
	log.Info(ctxWithLogger, "This message uses the context-based logging")

	// Demonstrate child logger with component
	dbLogger := log.NewWithComponent("filelogger-sample", "database")
	dbLogger.Info("Database connection established", log.KV{
		"host":     "localhost",
		"database": "testdb",
		"user":     "testuser",
	})

	// Demonstrate error logging with stack trace
	err := fmt.Errorf("database query failed: connection timeout")
	dbLogger.Error(err, "Failed to execute query", log.KV{
		"query": "SELECT * FROM users",
		"retry": true,
	})

	// Log application shutdown
	logger.Info("Application shutting down", log.KV{
		"runtime": time.Since(time.Now().Add(-5 * time.Second)).String(),
	})

	// Clean up log resources
	log.CloseLogFiles()

	// Print where to find the logs
	fmt.Printf("Log file created at: %s\n", logFilePath)
}
