package nats

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/oddbit-project/blueprint/log"
	"github.com/stretchr/testify/assert"
)

// setupTestLogger creates a test logger that writes to a buffer
func setupTestLogger(t *testing.T) (*log.Logger, *bytes.Buffer) {
	buf := &bytes.Buffer{}
	
	// Create a new logger and redirect to buffer
	logger := log.New("test")
	logger = logger.WithOutput(buf)

	return logger, buf
}

// parseLogOutput parses the log output into a map
func parseLogOutput(t *testing.T, buf *bytes.Buffer) map[string]interface{} {
	result := map[string]interface{}{}
	if buf.Len() == 0 {
		t.Fatal("Log buffer is empty")
	}
	err := json.Unmarshal(buf.Bytes(), &result)
	assert.NoError(t, err, "Log output should be valid JSON")
	return result
}

func TestProducerLogger(t *testing.T) {
	logger, buf := setupTestLogger(t)
	subject := "test.subject"

	// Apply producer logger
	logger = ProducerLogger(logger, subject)

	// Write a log message to capture fields
	logger.Info("test message")

	// Parse the log output
	logMap := parseLogOutput(t, buf)

	// Check that the fields were properly set
	assert.Equal(t, subject, logMap[NatsSubjectKey])
	assert.Equal(t, "producer", logMap[log.LogComponentKey])
}

func TestNewProducerLogger(t *testing.T) {
	subject := "test.subject"
	logger := NewProducerLogger(subject)

	// Verify the logger was created
	assert.NotNil(t, logger)

	// Create a buffer and redirect output
	buf := &bytes.Buffer{}
	logger = logger.WithOutput(buf)

	// Write a log message to capture fields
	logger.Info("test message")

	// Parse the log output
	logMap := parseLogOutput(t, buf)

	// Check that the fields were properly set
	assert.Equal(t, subject, logMap[NatsSubjectKey])
	assert.Equal(t, "producer", logMap[log.LogComponentKey])
}

func TestConsumerLogger(t *testing.T) {
	logger, buf := setupTestLogger(t)
	subject := "test.subject"
	queue := "test.queue"

	// Apply consumer logger
	logger = ConsumerLogger(logger, subject, queue)

	// Write a log message to capture fields
	logger.Info("test message")

	// Parse the log output
	logMap := parseLogOutput(t, buf)

	// Check that the fields were properly set
	assert.Equal(t, subject, logMap[NatsSubjectKey])
	assert.Equal(t, queue, logMap[NatsQueueKey])
	assert.Equal(t, "consumer", logMap[log.LogComponentKey])

	// Test with empty queue
	logger, buf = setupTestLogger(t)
	logger = ConsumerLogger(logger, subject, "")

	// Write a log message to capture fields
	logger.Info("test message")

	// Parse the log output
	logMap = parseLogOutput(t, buf)

	// Check that subject is set but queue is not present
	assert.Equal(t, subject, logMap[NatsSubjectKey])
	_, hasQueueKey := logMap[NatsQueueKey]
	assert.False(t, hasQueueKey, "Queue key should not be present when queue is empty")
}

func TestNewConsumerLogger(t *testing.T) {
	subject := "test.subject"
	queue := "test.queue"
	logger := NewConsumerLogger(subject, queue)

	// Verify the logger was created
	assert.NotNil(t, logger)

	// Create a buffer and redirect output
	buf := &bytes.Buffer{}
	logger = logger.WithOutput(buf)

	// Write a log message to capture fields
	logger.Info("test message")

	// Parse the log output
	logMap := parseLogOutput(t, buf)

	// Check that the fields were properly set
	assert.Equal(t, subject, logMap[NatsSubjectKey])
	assert.Equal(t, queue, logMap[NatsQueueKey])
	assert.Equal(t, "consumer", logMap[log.LogComponentKey])
}
