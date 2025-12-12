package franz

import (
	"fmt"
	"strings"

	"github.com/oddbit-project/blueprint/log"
)

// Logger context keys
const (
	LogKeyTopic     = "kafka_topic"
	LogKeyTopics    = "kafka_topics"
	LogKeyPartition = "kafka_partition"
	LogKeyBroker    = "kafka_broker"
	LogKeyOffset    = "kafka_offset"
	LogKeyKey       = "kafka_key"
	LogKeyGroup     = "kafka_group"
)

// ConsumerLogger creates a child logger with consumer details
func ConsumerLogger(l *log.Logger, topics []string, group string) *log.Logger {
	logger := l.WithField(log.LogComponentKey, "consumer")
	if len(topics) > 0 {
		logger = logger.WithField(LogKeyTopics, strings.Join(topics, ","))
	}
	if group != "" {
		logger = logger.WithField(LogKeyGroup, group)
	}
	return logger
}

// ProducerLogger creates a child logger with producer details
func ProducerLogger(l *log.Logger, topic string) *log.Logger {
	logger := l.WithField(log.LogComponentKey, "producer")
	if topic != "" {
		logger = logger.WithField(LogKeyTopic, topic)
	}
	return logger
}

// AdminLogger creates a child logger with admin details
func AdminLogger(l *log.Logger, broker string) *log.Logger {
	return l.
		WithField(LogKeyBroker, broker).
		WithField(log.LogComponentKey, "admin")
}

// NewConsumerLogger creates a new logger with Kafka consumer information
func NewConsumerLogger(topics []string, group string) *log.Logger {
	return ConsumerLogger(log.New("franz"), topics, group)
}

// NewProducerLogger creates a new logger with Kafka producer information
func NewProducerLogger(topic string) *log.Logger {
	return ProducerLogger(log.New("franz"), topic)
}

// NewAdminLogger creates a new logger with Kafka admin information
func NewAdminLogger(broker string) *log.Logger {
	return AdminLogger(log.New("franz"), broker)
}

// LogRecordReceived logs when a record is received
func LogRecordReceived(logger *log.Logger, record ConsumedRecord, group string) {
	fields := log.KV{
		LogKeyTopic:     record.Topic,
		LogKeyPartition: record.Partition,
		LogKeyOffset:    record.Offset,
	}

	if group != "" {
		fields[LogKeyGroup] = group
	}

	if len(record.Key) > 0 {
		fields[LogKeyKey] = string(record.Key)
	}

	// Add headers
	for _, header := range record.Headers {
		fields[fmt.Sprintf("header_%s", strings.ToLower(header.Key))] = string(header.Value)
	}

	logger.Info(fmt.Sprintf("Received record from topic %s", record.Topic), fields)
}

// LogRecordSent logs when a record is sent
func LogRecordSent(logger *log.Logger, record *Record, partition int32, offset int64) {
	fields := log.KV{
		LogKeyTopic:     record.Topic,
		LogKeyPartition: partition,
		LogKeyOffset:    offset,
	}

	if len(record.Key) > 0 {
		fields[LogKeyKey] = string(record.Key)
	}

	// Add headers
	for _, header := range record.Headers {
		fields[fmt.Sprintf("header_%s", strings.ToLower(header.Key))] = string(header.Value)
	}

	logger.Info(fmt.Sprintf("Sent record to topic %s", record.Topic), fields)
}

// LogBatchReceived logs when a batch is received
func LogBatchReceived(logger *log.Logger, batch Batch, group string) {
	fields := log.KV{
		LogKeyTopic:     batch.Topic,
		LogKeyPartition: batch.Partition,
		"recordCount":   len(batch.Records),
	}

	if group != "" {
		fields[LogKeyGroup] = group
	}

	if len(batch.Records) > 0 {
		fields["firstOffset"] = batch.Records[0].Offset
		fields["lastOffset"] = batch.Records[len(batch.Records)-1].Offset
	}

	logger.Info(fmt.Sprintf("Received batch from topic %s", batch.Topic), fields)
}
