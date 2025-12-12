package franz

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAdmin(t *testing.T) {
	t.Run("nil config uses defaults", func(t *testing.T) {
		admin, err := NewAdmin(nil, nil)
		// Will fail because default config has no brokers
		assert.Error(t, err)
		assert.Nil(t, admin)
	})

	t.Run("invalid config returns error", func(t *testing.T) {
		cfg := &AdminConfig{
			BaseConfig: BaseConfig{
				Brokers: "", // Invalid - empty
			},
		}

		admin, err := NewAdmin(cfg, nil)
		assert.Error(t, err)
		assert.Equal(t, ErrMissingBrokers, err)
		assert.Nil(t, admin)
	})

	t.Run("valid config creates admin", func(t *testing.T) {
		cfg := &AdminConfig{
			BaseConfig: BaseConfig{
				Brokers:  "localhost:9092",
				AuthType: AuthTypeNone,
			},
		}

		admin, err := NewAdmin(cfg, nil)
		if err != nil {
			t.Skipf("Cannot connect to broker: %v", err)
		}
		require.NotNil(t, admin)
		defer admin.Close()

		assert.True(t, admin.IsConnected())
		assert.NotNil(t, admin.Logger)
		assert.NotNil(t, admin.Client())
		assert.NotNil(t, admin.AdminClient())
	})
}

func TestAdminOperations(t *testing.T) {
	t.Run("operations on closed admin return error", func(t *testing.T) {
		cfg := &AdminConfig{
			BaseConfig: BaseConfig{
				Brokers:  "localhost:9092",
				AuthType: AuthTypeNone,
			},
		}

		admin, err := NewAdmin(cfg, nil)
		if err != nil {
			t.Skipf("Cannot connect to broker: %v", err)
		}

		admin.Close()
		assert.False(t, admin.IsConnected())

		ctx := context.Background()

		topics, err := admin.ListTopics(ctx)
		assert.Error(t, err)
		assert.Equal(t, ErrClientClosed, err)
		assert.Nil(t, topics)

		err = admin.CreateTopics(ctx, NewTopicConfig("test", 1, 1))
		assert.Error(t, err)
		assert.Equal(t, ErrClientClosed, err)

		err = admin.DeleteTopics(ctx, "test")
		assert.Error(t, err)
		assert.Equal(t, ErrClientClosed, err)

		brokers, err := admin.ListBrokers(ctx)
		assert.Error(t, err)
		assert.Equal(t, ErrClientClosed, err)
		assert.Nil(t, brokers)

		groups, err := admin.ListGroups(ctx)
		assert.Error(t, err)
		assert.Equal(t, ErrClientClosed, err)
		assert.Nil(t, groups)
	})

	t.Run("multiple close calls are safe", func(t *testing.T) {
		cfg := &AdminConfig{
			BaseConfig: BaseConfig{
				Brokers:  "localhost:9092",
				AuthType: AuthTypeNone,
			},
		}

		admin, err := NewAdmin(cfg, nil)
		if err != nil {
			t.Skipf("Cannot connect to broker: %v", err)
		}

		assert.NotPanics(t, func() {
			admin.Close()
			admin.Close()
			admin.Close()
		})
	})
}

func TestTopicConfig(t *testing.T) {
	t.Run("NewTopicConfig creates basic config", func(t *testing.T) {
		cfg := NewTopicConfig("test-topic", 3, 2)

		assert.Equal(t, "test-topic", cfg.Name)
		assert.Equal(t, int32(3), cfg.Partitions)
		assert.Equal(t, int16(2), cfg.ReplicationFactor)
		assert.Nil(t, cfg.Configs)
	})

	t.Run("WithConfig adds configuration", func(t *testing.T) {
		cfg := NewTopicConfig("test-topic", 3, 2).
			WithConfig("retention.ms", "86400000").
			WithConfig("cleanup.policy", "delete")

		assert.NotNil(t, cfg.Configs)
		assert.Len(t, cfg.Configs, 2)
		assert.Equal(t, "86400000", *cfg.Configs["retention.ms"])
		assert.Equal(t, "delete", *cfg.Configs["cleanup.policy"])
	})
}
