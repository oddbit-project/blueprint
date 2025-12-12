package franz

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBaseConfigValidate(t *testing.T) {
	tests := []struct {
		name        string
		config      BaseConfig
		expectedErr error
	}{
		{
			name: "valid config",
			config: BaseConfig{
				Brokers:  "localhost:9092",
				AuthType: AuthTypeNone,
			},
			expectedErr: nil,
		},
		{
			name: "missing brokers",
			config: BaseConfig{
				AuthType: AuthTypeNone,
			},
			expectedErr: ErrMissingBrokers,
		},
		{
			name: "invalid auth type",
			config: BaseConfig{
				Brokers:  "localhost:9092",
				AuthType: "invalid",
			},
			expectedErr: ErrInvalidAuthType,
		},
		{
			name: "valid plain auth",
			config: BaseConfig{
				Brokers:  "localhost:9092",
				AuthType: AuthTypePlain,
				Username: "user",
			},
			expectedErr: nil,
		},
		{
			name: "valid scram256 auth",
			config: BaseConfig{
				Brokers:  "localhost:9092",
				AuthType: AuthTypeScram256,
				Username: "user",
			},
			expectedErr: nil,
		},
		{
			name: "valid scram512 auth",
			config: BaseConfig{
				Brokers:  "localhost:9092",
				AuthType: AuthTypeScram512,
				Username: "user",
			},
			expectedErr: nil,
		},
		{
			name: "empty auth type defaults to none",
			config: BaseConfig{
				Brokers: "localhost:9092",
			},
			expectedErr: nil,
		},
		{
			name: "valid aws-msk-iam auth",
			config: BaseConfig{
				Brokers:  "localhost:9092",
				AuthType: AuthTypeAWSMSKIAM,
			},
			expectedErr: nil,
		},
		{
			name: "valid oauth auth",
			config: BaseConfig{
				Brokers:  "localhost:9092",
				AuthType: AuthTypeOAuth,
			},
			expectedErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			assert.Equal(t, tt.expectedErr, err)
		})
	}
}

func TestProducerConfigValidate(t *testing.T) {
	tests := []struct {
		name        string
		config      *ProducerConfig
		expectedErr error
	}{
		{
			name: "valid config",
			config: &ProducerConfig{
				BaseConfig: BaseConfig{
					Brokers:  "localhost:9092",
					AuthType: AuthTypeNone,
				},
				DefaultTopic: "test-topic",
				Acks:         AcksLeader,
			},
			expectedErr: nil,
		},
		{
			name: "missing brokers",
			config: &ProducerConfig{
				BaseConfig: BaseConfig{
					AuthType: AuthTypeNone,
				},
			},
			expectedErr: ErrMissingBrokers,
		},
		{
			name: "invalid acks",
			config: &ProducerConfig{
				BaseConfig: BaseConfig{
					Brokers:  "localhost:9092",
					AuthType: AuthTypeNone,
				},
				Acks: "invalid",
			},
			expectedErr: ErrInvalidAcks,
		},
		{
			name: "invalid compression",
			config: &ProducerConfig{
				BaseConfig: BaseConfig{
					Brokers:  "localhost:9092",
					AuthType: AuthTypeNone,
				},
				Compression: "invalid",
			},
			expectedErr: ErrInvalidCompression,
		},
		{
			name: "valid all acks",
			config: &ProducerConfig{
				BaseConfig: BaseConfig{
					Brokers:  "localhost:9092",
					AuthType: AuthTypeNone,
				},
				Acks: AcksAll,
			},
			expectedErr: nil,
		},
		{
			name: "valid gzip compression",
			config: &ProducerConfig{
				BaseConfig: BaseConfig{
					Brokers:  "localhost:9092",
					AuthType: AuthTypeNone,
				},
				Compression: CompressionGzip,
			},
			expectedErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			assert.Equal(t, tt.expectedErr, err)
		})
	}
}

func TestConsumerConfigValidate(t *testing.T) {
	tests := []struct {
		name        string
		config      *ConsumerConfig
		expectedErr error
	}{
		{
			name: "valid config with group",
			config: &ConsumerConfig{
				BaseConfig: BaseConfig{
					Brokers:  "localhost:9092",
					AuthType: AuthTypeNone,
				},
				Topics: []string{"test-topic"},
				Group:  "test-group",
			},
			expectedErr: nil,
		},
		{
			name: "valid config without group",
			config: &ConsumerConfig{
				BaseConfig: BaseConfig{
					Brokers:  "localhost:9092",
					AuthType: AuthTypeNone,
				},
				Topics: []string{"test-topic"},
			},
			expectedErr: nil,
		},
		{
			name: "missing topics",
			config: &ConsumerConfig{
				BaseConfig: BaseConfig{
					Brokers:  "localhost:9092",
					AuthType: AuthTypeNone,
				},
				Group: "test-group",
			},
			expectedErr: ErrMissingTopic,
		},
		{
			name: "invalid start offset",
			config: &ConsumerConfig{
				BaseConfig: BaseConfig{
					Brokers:  "localhost:9092",
					AuthType: AuthTypeNone,
				},
				Topics:      []string{"test-topic"},
				StartOffset: "invalid",
			},
			expectedErr: ErrInvalidOffset,
		},
		{
			name: "invalid isolation level",
			config: &ConsumerConfig{
				BaseConfig: BaseConfig{
					Brokers:  "localhost:9092",
					AuthType: AuthTypeNone,
				},
				Topics:         []string{"test-topic"},
				IsolationLevel: "invalid",
			},
			expectedErr: ErrInvalidIsolation,
		},
		{
			name: "valid start offset",
			config: &ConsumerConfig{
				BaseConfig: BaseConfig{
					Brokers:  "localhost:9092",
					AuthType: AuthTypeNone,
				},
				Topics:      []string{"test-topic"},
				StartOffset: OffsetStart,
			},
			expectedErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			assert.Equal(t, tt.expectedErr, err)
		})
	}
}

func TestAdminConfigValidate(t *testing.T) {
	tests := []struct {
		name        string
		config      *AdminConfig
		expectedErr error
	}{
		{
			name: "valid config",
			config: &AdminConfig{
				BaseConfig: BaseConfig{
					Brokers:  "localhost:9092",
					AuthType: AuthTypeNone,
				},
			},
			expectedErr: nil,
		},
		{
			name: "missing brokers",
			config: &AdminConfig{
				BaseConfig: BaseConfig{
					AuthType: AuthTypeNone,
				},
			},
			expectedErr: ErrMissingBrokers,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			assert.Equal(t, tt.expectedErr, err)
		})
	}
}

func TestDefaultConfigs(t *testing.T) {
	t.Run("DefaultBaseConfig", func(t *testing.T) {
		cfg := DefaultBaseConfig()
		assert.Equal(t, AuthTypeNone, cfg.AuthType)
		assert.Equal(t, 30*time.Second, cfg.DialTimeout)
		assert.Equal(t, 30*time.Second, cfg.RequestTimeout)
		assert.Equal(t, 100*time.Millisecond, cfg.RetryBackoff)
		assert.Equal(t, 3, cfg.MaxRetries)
	})

	t.Run("DefaultProducerConfig", func(t *testing.T) {
		cfg := DefaultProducerConfig()
		require.NotNil(t, cfg)
		assert.Equal(t, AuthTypeNone, cfg.AuthType)
		assert.Equal(t, 10000, cfg.BatchMaxRecords)
		assert.Equal(t, 1048576, cfg.BatchMaxBytes)
		assert.Equal(t, AcksLeader, cfg.Acks)
		assert.Equal(t, CompressionNone, cfg.Compression)
	})

	t.Run("DefaultConsumerConfig", func(t *testing.T) {
		cfg := DefaultConsumerConfig()
		require.NotNil(t, cfg)
		assert.Equal(t, AuthTypeNone, cfg.AuthType)
		assert.Equal(t, OffsetEnd, cfg.StartOffset)
		assert.Equal(t, IsolationReadCommitted, cfg.IsolationLevel)
		assert.Equal(t, 45*time.Second, cfg.SessionTimeout)
		assert.Equal(t, 60*time.Second, cfg.RebalanceTimeout)
		assert.Equal(t, 3*time.Second, cfg.HeartbeatInterval)
		assert.True(t, cfg.AutoCommit)
		assert.Equal(t, 5*time.Second, cfg.AutoCommitInterval)
	})

	t.Run("DefaultAdminConfig", func(t *testing.T) {
		cfg := DefaultAdminConfig()
		require.NotNil(t, cfg)
		assert.Equal(t, AuthTypeNone, cfg.AuthType)
	})
}

func TestBrokerList(t *testing.T) {
	tests := []struct {
		name     string
		brokers  string
		expected []string
	}{
		{
			name:     "single broker",
			brokers:  "localhost:9092",
			expected: []string{"localhost:9092"},
		},
		{
			name:     "multiple brokers",
			brokers:  "broker1:9092,broker2:9092,broker3:9092",
			expected: []string{"broker1:9092", "broker2:9092", "broker3:9092"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := BaseConfig{Brokers: tt.brokers}
			assert.Equal(t, tt.expected, cfg.brokerList())
		})
	}
}
