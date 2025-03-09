package log

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/rs/zerolog"
	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestLogKafkaMessageReceived(t *testing.T) {
	// Create a test logger with a buffer
	buf := &bytes.Buffer{}
	logger := &Logger{
		logger:     zerolog.New(buf),
		moduleInfo: "kafka-test",
	}
	ctx := logger.WithContext(context.Background())
	
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
	
	// Log the message
	LogKafkaMessageReceived(ctx, msg, "test-group")
	
	// Parse the log
	logMap := map[string]interface{}{}
	err := json.Unmarshal(buf.Bytes(), &logMap)
	assert.NoError(t, err)
	
	// Check log properties
	assert.Equal(t, "info", logMap["level"])
	assert.Equal(t, "Received message from topic test-topic", logMap["message"])
	assert.Equal(t, "test-topic", logMap[KafkaTopicKey])
	assert.Equal(t, float64(1), logMap[KafkaPartitionKey])
	assert.Equal(t, float64(42), logMap[KafkaOffsetKey])
	assert.Equal(t, "test-key", logMap[KafkaKeyKey])
	assert.Equal(t, "test-group", logMap[KafkaGroupKey])
	assert.Equal(t, "value1", logMap["header_header1"])
	assert.Equal(t, "value2", logMap["header_header2"])
}

func TestLogKafkaMessageSent(t *testing.T) {
	// Create a test logger with a buffer
	buf := &bytes.Buffer{}
	logger := &Logger{
		logger:     zerolog.New(buf),
		moduleInfo: "kafka-test",
	}
	ctx := logger.WithContext(context.Background())
	
	// Create a test Kafka message
	msg := kafka.Message{
		Topic: "test-topic",
		Key:   []byte("test-key"),
		Value: []byte("test-value"),
		Headers: []kafka.Header{
			{Key: "header1", Value: []byte("value1")},
		},
	}
	
	// Log the message
	LogKafkaMessageSent(ctx, msg)
	
	// Parse the log
	logMap := map[string]interface{}{}
	err := json.Unmarshal(buf.Bytes(), &logMap)
	assert.NoError(t, err)
	
	// Check log properties
	assert.Equal(t, "info", logMap["level"])
	assert.Equal(t, "Sent message to topic test-topic", logMap["message"])
	assert.Equal(t, "test-topic", logMap[KafkaTopicKey])
	assert.Equal(t, "test-key", logMap[KafkaKeyKey])
	assert.Equal(t, "value1", logMap["header_header1"])
}

func TestNewKafkaConsumerLogger(t *testing.T) {
	// Simpler test that only tests the module info
	ctx := context.Background()
	logger := NewKafkaConsumerLogger(ctx, "test-topic", "test-group")
	
	assert.NotNil(t, logger)
	assert.Equal(t, "default", logger.moduleInfo) // Default when no logger in context
	
	// Test with another logger in context
	origLogger := New("test-module")
	ctx = origLogger.WithContext(ctx)
	
	logger = NewKafkaConsumerLogger(ctx, "test-topic", "test-group")
	assert.Equal(t, "test-module", logger.moduleInfo)
}

func TestNewKafkaProducerLogger(t *testing.T) {
	// Simpler test that only tests the module info
	ctx := context.Background()
	logger := NewKafkaProducerLogger(ctx, "test-topic")
	
	assert.NotNil(t, logger)
	assert.Equal(t, "default", logger.moduleInfo) // Default when no logger in context
	
	// Test with another logger in context
	origLogger := New("test-module")
	ctx = origLogger.WithContext(ctx)
	
	logger = NewKafkaProducerLogger(ctx, "test-topic")
	assert.Equal(t, "test-module", logger.moduleInfo)
}

func TestLogKafkaError(t *testing.T) {
	// Create a test logger with a buffer
	buf := &bytes.Buffer{}
	logger := &Logger{
		logger:     zerolog.New(buf),
		moduleInfo: "kafka-test",
	}
	ctx := logger.WithContext(context.Background())
	
	// Test error logging
	testErr := errors.New("kafka connection error")
	fields := map[string]interface{}{
		"broker": "localhost:9092",
		"topic":  "test-topic",
	}
	
	LogKafkaError(ctx, testErr, "Failed to connect to Kafka", fields)
	
	// Parse the log
	logMap := map[string]interface{}{}
	err := json.Unmarshal(buf.Bytes(), &logMap)
	assert.NoError(t, err)
	
	// Check log properties
	assert.Equal(t, "error", logMap["level"])
	assert.Equal(t, "Failed to connect to Kafka", logMap["message"])
	assert.Equal(t, "kafka connection error", logMap["error"])
	assert.Equal(t, "localhost:9092", logMap["broker"])
	assert.Equal(t, "test-topic", logMap["topic"])
	assert.NotEmpty(t, logMap["timestamp"])
}

func TestAddKafkaHeadersFromContext(t *testing.T) {
	// Create a logger with trace ID
	traceID := "test-trace-id"
	logger := New("test-module").WithTraceID(traceID)
	
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
	headers := AddKafkaHeadersFromContext(ctx, existingHeaders)
	
	// Check that the headers were added
	assert.Equal(t, 3, len(headers))
	
	// Check existing header is preserved
	assert.Equal(t, "existing", headers[0].Key)
	assert.Equal(t, []byte("value"), headers[0].Value)
	
	// Check trace ID was added
	assert.Equal(t, HeaderTraceID, headers[1].Key)
	assert.Equal(t, []byte(traceID), headers[1].Value)
	
	// Check request ID was added
	assert.Equal(t, HeaderRequestID, headers[2].Key)
	assert.Equal(t, []byte(requestID), headers[2].Value)
}