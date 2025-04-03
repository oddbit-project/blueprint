package kafka

import (
	"github.com/oddbit-project/blueprint/log"
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
