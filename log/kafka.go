package log

import (
	"context"
	"fmt"
	"github.com/segmentio/kafka-go"
	"strings"
	"time"
)

// KafkaLogContext provides context keys for Kafka logging
const (
	KafkaTopicKey    = "kafka_topic"
	KafkaPartitionKey = "kafka_partition"
	KafkaOffsetKey   = "kafka_offset"
	KafkaKeyKey      = "kafka_key"
	KafkaGroupKey    = "kafka_group"
)

// LogKafkaMessageReceived logs a message when a Kafka message is received
func LogKafkaMessageReceived(ctx context.Context, msg kafka.Message, group string) {
	logger := FromContext(ctx)
	if logger == nil {
		logger = New("kafka")
	}
	
	fields := map[string]interface{}{
		KafkaTopicKey:    msg.Topic,
		KafkaPartitionKey: msg.Partition,
		KafkaOffsetKey:   msg.Offset,
		KafkaGroupKey:    group,
	}
	
	if len(msg.Key) > 0 {
		fields[KafkaKeyKey] = string(msg.Key)
	}
	
	// Add message headers
	for _, header := range msg.Headers {
		fields[fmt.Sprintf("header_%s", strings.ToLower(header.Key))] = string(header.Value)
	}
	
	logger.Info(fmt.Sprintf("Received message from topic %s", msg.Topic), fields)
}

// LogKafkaMessageSent logs a message when a Kafka message is sent
func LogKafkaMessageSent(ctx context.Context, msg kafka.Message) {
	logger := FromContext(ctx)
	if logger == nil {
		logger = New("kafka")
	}
	
	fields := map[string]interface{}{
		KafkaTopicKey: msg.Topic,
	}
	
	if len(msg.Key) > 0 {
		fields[KafkaKeyKey] = string(msg.Key)
	}
	
	// Add message headers
	for _, header := range msg.Headers {
		fields[fmt.Sprintf("header_%s", strings.ToLower(header.Key))] = string(header.Value)
	}
	
	logger.Info(fmt.Sprintf("Sent message to topic %s", msg.Topic), fields)
}

// NewKafkaConsumerLogger creates a new logger with Kafka consumer information
func NewKafkaConsumerLogger(ctx context.Context, topic string, group string) *Logger {
	logger := FromContext(ctx)
	if logger == nil {
		logger = New("kafka")
	}
	
	return logger.
		WithField(KafkaTopicKey, topic).
		WithField(KafkaGroupKey, group).
		WithField(LogComponentKey, "consumer")
}

// NewKafkaProducerLogger creates a new logger with Kafka producer information
func NewKafkaProducerLogger(ctx context.Context, topic string) *Logger {
	logger := FromContext(ctx)
	if logger == nil {
		logger = New("kafka")
	}
	
	return logger.
		WithField(KafkaTopicKey, topic).
		WithField(LogComponentKey, "producer")
}

// LogKafkaError logs a Kafka error with context information
func LogKafkaError(ctx context.Context, err error, msg string, fields ...map[string]interface{}) {
	logger := FromContext(ctx)
	if logger == nil {
		logger = New("kafka")
	}
	
	errorFields := map[string]interface{}{
		"timestamp": time.Now().Format(time.RFC3339),
	}
	
	// Add additional fields if provided
	if len(fields) > 0 {
		for k, v := range fields[0] {
			errorFields[k] = v
		}
	}
	
	logger.Error(err, msg, errorFields)
}

// AddKafkaHeadersFromContext adds trace and request IDs to Kafka message headers from context
func AddKafkaHeadersFromContext(ctx context.Context, headers []kafka.Header) []kafka.Header {
	logger := FromContext(ctx)
	if logger == nil {
		return headers
	}
	
	// Add trace ID if available
	if logger.GetTraceID() != "" {
		headers = append(headers, kafka.Header{
			Key:   HeaderTraceID,
			Value: []byte(logger.GetTraceID()),
		})
	}
	
	// Check for request ID in context fields
	// This requires the logger to have been created with WithField("request_id", ...)
	// For HTTP requests, this is handled by the HTTP middleware
	requestID := ""
	if ctx.Value("request_id") != nil {
		if id, ok := ctx.Value("request_id").(string); ok {
			requestID = id
		}
	}
	
	if requestID != "" {
		headers = append(headers, kafka.Header{
			Key:   HeaderRequestID,
			Value: []byte(requestID),
		})
	}
	
	return headers
}