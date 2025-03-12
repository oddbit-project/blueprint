package kafka

import (
	"fmt"
	"github.com/oddbit-project/blueprint/log"
	"github.com/segmentio/kafka-go"
	"strings"
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

// ConsumerLogger creates a child logger with consumer details
func ConsumerLogger(l *log.Logger, topic string, group string) *log.Logger {
	return l.
		WithField(KafkaTopicKey, topic).
		WithField(KafkaGroupKey, group).
		WithField(log.LogComponentKey, "consumer")
}

// ProducerLogger creates a child logger with producer details
func ProducerLogger(l *log.Logger, topic string) *log.Logger {
	return l.
		WithField(KafkaTopicKey, topic).
		WithField(log.LogComponentKey, "producer")

}

// AdminLogger creates a child logger with admin details
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
