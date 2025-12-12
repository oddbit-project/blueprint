package franz

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewConsumer(t *testing.T) {
	t.Run("nil config returns error", func(t *testing.T) {
		consumer, err := NewConsumer(nil, nil)
		assert.Error(t, err)
		assert.Equal(t, ErrNilConfig, err)
		assert.Nil(t, consumer)
	})

	t.Run("invalid config returns error", func(t *testing.T) {
		cfg := &ConsumerConfig{
			BaseConfig: BaseConfig{
				Brokers: "", // Invalid - empty
			},
		}

		consumer, err := NewConsumer(cfg, nil)
		assert.Error(t, err)
		assert.Equal(t, ErrMissingBrokers, err)
		assert.Nil(t, consumer)
	})

	t.Run("missing topics returns error", func(t *testing.T) {
		cfg := &ConsumerConfig{
			BaseConfig: BaseConfig{
				Brokers:  "localhost:9092",
				AuthType: AuthTypeNone,
			},
			Group: "test-group",
		}

		consumer, err := NewConsumer(cfg, nil)
		assert.Error(t, err)
		assert.Equal(t, ErrMissingTopic, err)
		assert.Nil(t, consumer)
	})

	t.Run("valid config creates consumer", func(t *testing.T) {
		cfg := &ConsumerConfig{
			BaseConfig: BaseConfig{
				Brokers:  "localhost:9092",
				AuthType: AuthTypeNone,
			},
			Topics: []string{"test-topic"},
			Group:  "test-group",
		}

		consumer, err := NewConsumer(cfg, nil)
		if err != nil {
			t.Skipf("Cannot connect to broker: %v", err)
		}
		require.NotNil(t, consumer)
		defer consumer.Close()

		assert.True(t, consumer.IsConnected())
		assert.NotNil(t, consumer.Logger)
	})
}

func TestConsumerOperations(t *testing.T) {
	t.Run("poll with nil context returns error", func(t *testing.T) {
		cfg := &ConsumerConfig{
			BaseConfig: BaseConfig{
				Brokers:  "localhost:9092",
				AuthType: AuthTypeNone,
			},
			Topics: []string{"test-topic"},
			Group:  "test-group",
		}

		consumer, err := NewConsumer(cfg, nil)
		if err != nil {
			t.Skipf("Cannot connect to broker: %v", err)
		}
		defer consumer.Close()

		result, err := consumer.Poll(nil)
		assert.Error(t, err)
		assert.Equal(t, ErrNilContext, err)
		assert.Nil(t, result)
	})

	t.Run("consume with nil context returns error", func(t *testing.T) {
		cfg := &ConsumerConfig{
			BaseConfig: BaseConfig{
				Brokers:  "localhost:9092",
				AuthType: AuthTypeNone,
			},
			Topics: []string{"test-topic"},
			Group:  "test-group",
		}

		consumer, err := NewConsumer(cfg, nil)
		if err != nil {
			t.Skipf("Cannot connect to broker: %v", err)
		}
		defer consumer.Close()

		err = consumer.Consume(nil, func(ctx context.Context, record ConsumedRecord) error {
			return nil
		})
		assert.Error(t, err)
		assert.Equal(t, ErrNilContext, err)
	})

	t.Run("consume with nil handler returns error", func(t *testing.T) {
		cfg := &ConsumerConfig{
			BaseConfig: BaseConfig{
				Brokers:  "localhost:9092",
				AuthType: AuthTypeNone,
			},
			Topics: []string{"test-topic"},
			Group:  "test-group",
		}

		consumer, err := NewConsumer(cfg, nil)
		if err != nil {
			t.Skipf("Cannot connect to broker: %v", err)
		}
		defer consumer.Close()

		err = consumer.Consume(context.Background(), nil)
		assert.Error(t, err)
		assert.Equal(t, ErrNilHandler, err)
	})

	t.Run("operations on closed consumer return error", func(t *testing.T) {
		cfg := &ConsumerConfig{
			BaseConfig: BaseConfig{
				Brokers:  "localhost:9092",
				AuthType: AuthTypeNone,
			},
			Topics: []string{"test-topic"},
			Group:  "test-group",
		}

		consumer, err := NewConsumer(cfg, nil)
		if err != nil {
			t.Skipf("Cannot connect to broker: %v", err)
		}

		consumer.Close()
		assert.False(t, consumer.IsConnected())

		ctx := context.Background()

		result, err := consumer.Poll(ctx)
		assert.Error(t, err)
		assert.Equal(t, ErrClientClosed, err)
		assert.Nil(t, result)

		records, err := consumer.PollRecords(ctx, 10)
		assert.Error(t, err)
		assert.Equal(t, ErrClientClosed, err)
		assert.Nil(t, records)

		err = consumer.CommitOffsets(ctx)
		assert.Error(t, err)
		assert.Equal(t, ErrClientClosed, err)
	})

	t.Run("multiple close calls are safe", func(t *testing.T) {
		cfg := &ConsumerConfig{
			BaseConfig: BaseConfig{
				Brokers:  "localhost:9092",
				AuthType: AuthTypeNone,
			},
			Topics: []string{"test-topic"},
			Group:  "test-group",
		}

		consumer, err := NewConsumer(cfg, nil)
		if err != nil {
			t.Skipf("Cannot connect to broker: %v", err)
		}

		assert.NotPanics(t, func() {
			consumer.Close()
			consumer.Close()
			consumer.Close()
		})
	})
}

func TestConsumerPauseResume(t *testing.T) {
	cfg := &ConsumerConfig{
		BaseConfig: BaseConfig{
			Brokers:  "localhost:9092",
			AuthType: AuthTypeNone,
		},
		Topics: []string{"test-topic"},
		Group:  "test-group",
	}

	consumer, err := NewConsumer(cfg, nil)
	if err != nil {
		t.Skipf("Cannot connect to broker: %v", err)
	}
	defer consumer.Close()

	// These should not panic
	assert.NotPanics(t, func() {
		consumer.Pause("test-topic")
		consumer.Resume("test-topic")
		consumer.PausePartitions(map[string][]int32{"test-topic": {0, 1}})
		consumer.ResumePartitions(map[string][]int32{"test-topic": {0, 1}})
	})
}

func TestConsumerTimeout(t *testing.T) {
	t.Run("consumer respects dial timeout", func(t *testing.T) {
		cfg := &ConsumerConfig{
			BaseConfig: BaseConfig{
				Brokers:     "localhost:9092",
				AuthType:    AuthTypeNone,
				DialTimeout: 100 * time.Millisecond,
			},
			Topics: []string{"test-topic"},
			Group:  "test-group",
		}

		// This will likely timeout quickly without a real broker
		_, err := NewConsumer(cfg, nil)
		if err == nil {
			t.Skip("Broker is available, skipping timeout test")
		}
		// Error is expected
	})
}

func TestConsumeChannel(t *testing.T) {
	t.Run("consume channel with nil context returns error", func(t *testing.T) {
		cfg := &ConsumerConfig{
			BaseConfig: BaseConfig{
				Brokers:  "localhost:9092",
				AuthType: AuthTypeNone,
			},
			Topics: []string{"test-topic"},
			Group:  "test-group",
		}

		consumer, err := NewConsumer(cfg, nil)
		if err != nil {
			t.Skipf("Cannot connect to broker: %v", err)
		}
		defer consumer.Close()

		ch := make(chan ConsumedRecord)
		err = consumer.ConsumeChannel(nil, ch)
		assert.Error(t, err)
		assert.Equal(t, ErrNilContext, err)
	})

	t.Run("consume channel with nil channel returns error", func(t *testing.T) {
		cfg := &ConsumerConfig{
			BaseConfig: BaseConfig{
				Brokers:  "localhost:9092",
				AuthType: AuthTypeNone,
			},
			Topics: []string{"test-topic"},
			Group:  "test-group",
		}

		consumer, err := NewConsumer(cfg, nil)
		if err != nil {
			t.Skipf("Cannot connect to broker: %v", err)
		}
		defer consumer.Close()

		err = consumer.ConsumeChannel(context.Background(), nil)
		assert.Error(t, err)
		assert.Equal(t, ErrNilHandler, err)
	})
}
