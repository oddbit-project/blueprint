package franz

import (
	"testing"
	"time"

	"github.com/oddbit-project/blueprint/log"
	"github.com/stretchr/testify/assert"
)

func TestConsumerLogger(t *testing.T) {
	t.Run("creates logger with topics and group", func(t *testing.T) {
		logger := NewConsumerLogger([]string{"topic1", "topic2"}, "test-group")

		assert.NotNil(t, logger)
		assert.Equal(t, "franz", logger.ModuleInfo())
	})

	t.Run("creates logger from existing logger", func(t *testing.T) {
		baseLogger := log.New("test-module")
		logger := ConsumerLogger(baseLogger, []string{"topic1"}, "group1")

		assert.NotNil(t, logger)
	})

	t.Run("handles empty topics", func(t *testing.T) {
		logger := NewConsumerLogger(nil, "test-group")
		assert.NotNil(t, logger)
	})

	t.Run("handles empty group", func(t *testing.T) {
		logger := NewConsumerLogger([]string{"topic1"}, "")
		assert.NotNil(t, logger)
	})
}

func TestProducerLogger(t *testing.T) {
	t.Run("creates logger with topic", func(t *testing.T) {
		logger := NewProducerLogger("test-topic")

		assert.NotNil(t, logger)
		assert.Equal(t, "franz", logger.ModuleInfo())
	})

	t.Run("creates logger from existing logger", func(t *testing.T) {
		baseLogger := log.New("test-module")
		logger := ProducerLogger(baseLogger, "topic1")

		assert.NotNil(t, logger)
	})

	t.Run("handles empty topic", func(t *testing.T) {
		logger := NewProducerLogger("")
		assert.NotNil(t, logger)
	})
}

func TestAdminLogger(t *testing.T) {
	t.Run("creates logger with broker", func(t *testing.T) {
		logger := NewAdminLogger("localhost:9092")

		assert.NotNil(t, logger)
		assert.Equal(t, "franz", logger.ModuleInfo())
	})

	t.Run("creates logger from existing logger", func(t *testing.T) {
		baseLogger := log.New("test-module")
		logger := AdminLogger(baseLogger, "localhost:9092")

		assert.NotNil(t, logger)
	})
}

func TestLogRecordReceived(t *testing.T) {
	logger := NewConsumerLogger([]string{"test-topic"}, "test-group")

	record := ConsumedRecord{
		Topic:     "test-topic",
		Partition: 1,
		Offset:    42,
		Key:       []byte("test-key"),
		Value:     []byte("test-value"),
		Headers: []Header{
			{Key: "header1", Value: []byte("value1")},
			{Key: "Header2", Value: []byte("value2")},
		},
		Timestamp: time.Now(),
	}

	// Should not panic
	assert.NotPanics(t, func() {
		LogRecordReceived(logger, record, "test-group")
	})
}

func TestLogRecordSent(t *testing.T) {
	logger := NewProducerLogger("test-topic")

	record := NewRecord([]byte("test-value")).
		WithKey([]byte("test-key")).
		WithTopic("test-topic").
		WithHeader("header1", []byte("value1"))

	// Should not panic
	assert.NotPanics(t, func() {
		LogRecordSent(logger, record, 0, 100)
	})
}

func TestLogBatchReceived(t *testing.T) {
	logger := NewConsumerLogger([]string{"test-topic"}, "test-group")

	batch := Batch{
		Topic:     "test-topic",
		Partition: 0,
		Records: []ConsumedRecord{
			{Topic: "test-topic", Partition: 0, Offset: 0, Value: []byte("msg1")},
			{Topic: "test-topic", Partition: 0, Offset: 1, Value: []byte("msg2")},
			{Topic: "test-topic", Partition: 0, Offset: 2, Value: []byte("msg3")},
		},
	}

	// Should not panic
	assert.NotPanics(t, func() {
		LogBatchReceived(logger, batch, "test-group")
	})
}

func TestLogBatchReceivedEmpty(t *testing.T) {
	logger := NewConsumerLogger([]string{"test-topic"}, "test-group")

	batch := Batch{
		Topic:     "test-topic",
		Partition: 0,
		Records:   []ConsumedRecord{},
	}

	// Should not panic even with empty batch
	assert.NotPanics(t, func() {
		LogBatchReceived(logger, batch, "test-group")
	})
}
