package s3

import (
	"errors"
	"github.com/oddbit-project/blueprint/log"
	"time"
)

// securityEvent represents a security-relevant event for audit logging
type securityEvent struct {
	Operation string        `json:"operation"`
	Resource  string        `json:"resource"`
	Success   bool          `json:"success"`
	Error     string        `json:"error,omitempty"`
	Timestamp time.Time     `json:"timestamp"`
	Duration  time.Duration `json:"duration,omitempty"`
	Details   log.KV        `json:"details,omitempty"`
}

// logEvent log an event
func logEvent(logger *log.Logger, event securityEvent) {
	if logger == nil {
		return
	}

	// Add standard security context
	event.Timestamp = time.Now().UTC()

	logFields := log.KV{
		"event_type": "s3_security_event",
		"operation":  event.Operation,
		"resource":   event.Resource,
		"success":    event.Success,
		"timestamp":  event.Timestamp,
	}

	if event.Duration > 0 {
		logFields["duration_ms"] = event.Duration.Milliseconds()
	}

	if event.Error != "" {
		logFields["error"] = event.Error
	}

	// Add additional details
	if event.Details != nil {
		for k, v := range event.Details {
			logFields[k] = v
		}
	}

	// Log at appropriate level based on success/failure
	if event.Success {
		logger.Info("S3 Security Event", logFields)
	} else {
		if event.Error != "" {
			logger.Error(errors.New(event.Error), "S3 Security Event - Failed", logFields)
		} else {
			logger.Warn("S3 Security Event - Failed", logFields)
		}
	}
}

func logOperationStart(logger *log.Logger, operation, resource string, details log.KV) time.Time {
	startTime := time.Now()
	if logger == nil {
		return startTime
	}
	event := securityEvent{
		Operation: operation + "_start",
		Resource:  resource,
		Success:   true,
		Details:   details,
	}

	logEvent(logger, event)
	return startTime
}

func logOperationEnd(logger *log.Logger, operation, resource string, startTime time.Time, err error, details log.KV) {
	if logger == nil {
		return
	}
	event := securityEvent{
		Operation: operation + "_end",
		Resource:  resource,
		Success:   err == nil,
		Duration:  time.Since(startTime),
		Details:   details,
	}

	if err != nil {
		event.Error = err.Error()
	}

	logEvent(logger, event)
}
