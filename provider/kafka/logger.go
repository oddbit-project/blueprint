package kafka

import (
	"context"
	"fmt"
	"github.com/oddbit-project/blueprint/log"
	log2 "github.com/oddbit-project/blueprint/provider/httpserver/log"
	"github.com/segmentio/kafka-go"
	"strings"
	"time"
)

// KafkaLogContext provides context keys for Kafka logging
const (
	KafkaTopicKey     = "kafka_topic"
	KafkaPartitionKey = "kafka_partition"
	KafkaBrokerKey    = "kafka_broker"
	KafkaOffsetKey    = "kafka_offset"
	KafkaKeyKey       = "kafka_key"
	KafkaGroupKey     = "kafka_group"
)

// LogMessageReceived logs a message when a Kafka message is received
func LogMessageReceived(logger *log.Logger, msg kafka.Message, group string) {

	fields := map[string]interface{}{
		KafkaTopicKey:     msg.Topic,
		KafkaPartitionKey: msg.Partition,
		KafkaOffsetKey:    msg.Offset,
		KafkaGroupKey:     group,
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

// LogMessageSent logs a message when a Kafka message is sent
func LogMessageSent(logger *log.Logger, msg kafka.Message) {

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

// ConsumerLogger
func ConsumerLogger(l *log.Logger, topic string, group string) *log.Logger {
	return l.
		WithField(KafkaTopicKey, topic).
		WithField(KafkaGroupKey, group).
		WithField(log.LogComponentKey, "consumer")
}

// ProducerLogger
func ProducerLogger(l *log.Logger, topic string) *log.Logger {
	return l.
		WithField(KafkaTopicKey, topic).
		WithField(log.LogComponentKey, "producer")

}

// AdminLogger
func AdminLogger(l *log.Logger, broker string) *log.Logger {
	return l.
		WithField(KafkaBrokerKey, broker).
		WithField(log.LogComponentKey, "admin")
}

// NewConsumerLogger creates a new logger with Kafka consumer information
func NewConsumerLogger(topic string, group string) *log.Logger {
	return ConsumerLogger(log.New("kafka"), topic, group)
}

// NewProducerLogger creates a new logger with Kafka producer information
func NewProducerLogger(topic string) *log.Logger {
	return ProducerLogger(log.New("kafka"), topic)
}

// NewAdminLogger creates a new logger with Kafka admin information
func NewAdminLogger(broker string) *log.Logger {
	return AdminLogger(log.New("kafka"), broker)
}

// LogError logs a Kafka error with context information
func LogError(logger *log.Logger, err error, msg string, fields ...map[string]interface{}) {
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

// LoggerAddHeadersFromContext adds trace and request IDs to Kafka message headers from context
func LoggerAddHeadersFromContext(ctx context.Context, logger *log.Logger, headers []kafka.Header) []kafka.Header {
	// Add trace ID if available
	if logger.GetTraceID() != "" {
		headers = append(headers, kafka.Header{
			Key:   log2.HeaderTraceID,
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
			Key:   log2.HeaderRequestID,
			Value: []byte(requestID),
		})
	}

	return headers
}
