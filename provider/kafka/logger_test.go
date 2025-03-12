package kafka

import (
	"context"
	"errors"
	"github.com/oddbit-project/blueprint/log"
	log2 "github.com/oddbit-project/blueprint/provider/httpserver/log"
	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestLogKafkaMessageReceived(t *testing.T) {
	// Create a test logger
	logger := NewConsumerLogger("sample-topic", "sample-group")

	// Create a test Kafka message
	msg := kafka.Message{
		Topic:     "test-topic",
		Partition: 1,
		Offset:    42,
		Key:       []byte("test-key"),
		Value:     []byte("test-value"),
		Headers: []kafka.Header{
			{Key: "header1", Value: []byte("value1")},
			{Key: "header2", Value: []byte("value2")},
		},
	}

	// This just tests that the log function doesn't panic
	LogMessageReceived(logger, msg, "test-group")
	
	// Verify logger has correct module
	assert.Equal(t, "kafka", logger.ModuleInfo())
}

func TestLogKafkaMessageSent(t *testing.T) {
	// Create a test logger
	logger := NewProducerLogger("sample-topic")

	// Create a test Kafka message
	msg := kafka.Message{
		Topic: "test-topic",
		Key:   []byte("test-key"),
		Value: []byte("test-value"),
		Headers: []kafka.Header{
			{Key: "header1", Value: []byte("value1")},
		},
	}

	// This just tests that the log function doesn't panic
	LogMessageSent(logger, msg)
	
	// Verify logger has correct module
	assert.Equal(t, "kafka", logger.ModuleInfo())
}

func TestNewKafkaConsumerLogger(t *testing.T) {
	// Simpler test that only tests the module info
	logger := NewConsumerLogger("test-topic", "test-group")

	assert.NotNil(t, logger)
	assert.Equal(t, "kafka", logger.ModuleInfo()) // Default when no logger in context

	// Test with another logger in context
	logger = log.New("test-module")
	assert.Equal(t, "test-module", logger.ModuleInfo())
}

func TestNewKafkaProducerLogger(t *testing.T) {
	// Simpler test that only tests the module info
	logger := NewProducerLogger("test-topic")

	assert.NotNil(t, logger)
	assert.Equal(t, "kafka", logger.ModuleInfo()) // Module name is "kafka"

	// Test with another logger in context
	logger = log.New("test-module")
	assert.Equal(t, "test-module", logger.ModuleInfo())
}

func TestLogKafkaError(t *testing.T) {
	// Create a test logger
	logger := log.New("kafka-test")

	// Test error logging
	testErr := errors.New("kafka connection error")
	fields := map[string]interface{}{
		"broker": "localhost:9092",
		"topic":  "test-topic",
	}

	// This just tests that the log function doesn't panic
	LogError(logger, testErr, "Failed to connect to Kafka", fields)
	
	// Basic validation of logger
	assert.Equal(t, "kafka-test", logger.ModuleInfo())
}

func TestAddKafkaHeadersFromContext(t *testing.T) {
	// Create a logger with trace ID
	traceID := "test-trace-id"
	logger := log.New("test-module").WithTraceID(traceID)

	// Add logger to context
	ctx := logger.WithContext(context.Background())

	// Add request ID to context
	requestID := "test-request-id"
	ctx = context.WithValue(ctx, "request_id", requestID)

	// Create existing headers
	existingHeaders := []kafka.Header{
		{Key: "existing", Value: []byte("value")},
	}

	// Add headers from context
	headers := LoggerAddHeadersFromContext(ctx, logger, existingHeaders)

	// Check that the headers were added
	assert.Equal(t, 3, len(headers))

	// Check existing header is preserved
	assert.Equal(t, "existing", headers[0].Key)
	assert.Equal(t, []byte("value"), headers[0].Value)

	// Check trace ID was added
	assert.Equal(t, log2.HeaderTraceID, headers[1].Key)
	assert.Equal(t, []byte(traceID), headers[1].Value)

	// Check request ID was added
	assert.Equal(t, log2.HeaderRequestID, headers[2].Key)
	assert.Equal(t, []byte(requestID), headers[2].Value)
}
