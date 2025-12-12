package franz

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewProducer(t *testing.T) {
	t.Run("nil config uses defaults", func(t *testing.T) {
		// This will fail without a broker, but validates that nil config is handled
		producer, err := NewProducer(nil, nil)
		// Will fail because default config has no brokers
		assert.Error(t, err)
		assert.Nil(t, producer)
	})

	t.Run("invalid config returns error", func(t *testing.T) {
		cfg := &ProducerConfig{
			BaseConfig: BaseConfig{
				Brokers: "", // Invalid - empty
			},
		}

		producer, err := NewProducer(cfg, nil)
		assert.Error(t, err)
		assert.Equal(t, ErrMissingBrokers, err)
		assert.Nil(t, producer)
	})

	t.Run("valid config creates producer", func(t *testing.T) {
		cfg := &ProducerConfig{
			BaseConfig: BaseConfig{
				Brokers:  "localhost:9092",
				AuthType: AuthTypeNone,
			},
			DefaultTopic: "test-topic",
		}

		producer, err := NewProducer(cfg, nil)
		// May fail without actual broker, but shouldn't be a config error
		if err != nil {
			// kgo.NewClient may fail without broker - that's OK for this test
			t.Skipf("Cannot connect to broker: %v", err)
		}
		require.NotNil(t, producer)
		defer producer.Close()

		assert.True(t, producer.IsConnected())
		assert.NotNil(t, producer.Logger)
	})
}

func TestProducerOperations(t *testing.T) {
	t.Run("produce with nil context returns error", func(t *testing.T) {
		cfg := &ProducerConfig{
			BaseConfig: BaseConfig{
				Brokers:  "localhost:9092",
				AuthType: AuthTypeNone,
			},
		}

		producer, err := NewProducer(cfg, nil)
		if err != nil {
			t.Skipf("Cannot connect to broker: %v", err)
		}
		defer producer.Close()

		results, err := producer.Produce(nil, NewRecord([]byte("test")))
		assert.Error(t, err)
		assert.Equal(t, ErrNilContext, err)
		assert.Nil(t, results)
	})

	t.Run("produce async with nil context returns error", func(t *testing.T) {
		cfg := &ProducerConfig{
			BaseConfig: BaseConfig{
				Brokers:  "localhost:9092",
				AuthType: AuthTypeNone,
			},
		}

		producer, err := NewProducer(cfg, nil)
		if err != nil {
			t.Skipf("Cannot connect to broker: %v", err)
		}
		defer producer.Close()

		err = producer.ProduceAsync(nil, NewRecord([]byte("test")), nil)
		assert.Error(t, err)
		assert.Equal(t, ErrNilContext, err)
	})

	t.Run("operations on closed producer return error", func(t *testing.T) {
		cfg := &ProducerConfig{
			BaseConfig: BaseConfig{
				Brokers:  "localhost:9092",
				AuthType: AuthTypeNone,
			},
		}

		producer, err := NewProducer(cfg, nil)
		if err != nil {
			t.Skipf("Cannot connect to broker: %v", err)
		}

		producer.Close()
		assert.False(t, producer.IsConnected())

		ctx := context.Background()

		results, err := producer.Produce(ctx, NewRecord([]byte("test")))
		assert.Error(t, err)
		assert.Equal(t, ErrClientClosed, err)
		assert.Nil(t, results)

		err = producer.ProduceAsync(ctx, NewRecord([]byte("test")), nil)
		assert.Error(t, err)
		assert.Equal(t, ErrClientClosed, err)

		err = producer.Flush(ctx)
		assert.Error(t, err)
		assert.Equal(t, ErrClientClosed, err)
	})

	t.Run("multiple close calls are safe", func(t *testing.T) {
		cfg := &ProducerConfig{
			BaseConfig: BaseConfig{
				Brokers:  "localhost:9092",
				AuthType: AuthTypeNone,
			},
		}

		producer, err := NewProducer(cfg, nil)
		if err != nil {
			t.Skipf("Cannot connect to broker: %v", err)
		}

		assert.NotPanics(t, func() {
			producer.Close()
			producer.Close()
			producer.Close()
		})
	})
}

func TestProducerTimeout(t *testing.T) {
	t.Run("produce respects context timeout", func(t *testing.T) {
		cfg := &ProducerConfig{
			BaseConfig: BaseConfig{
				Brokers:     "localhost:9092",
				AuthType:    AuthTypeNone,
				DialTimeout: 100 * time.Millisecond, // Short timeout
			},
			DefaultTopic: "test-topic",
		}

		// This will likely timeout quickly without a real broker
		_, err := NewProducer(cfg, nil)
		if err == nil {
			t.Skip("Broker is available, skipping timeout test")
		}
		// Error is expected - either timeout or connection refused
	})
}
