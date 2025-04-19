package nats

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProducerConfigValidate(t *testing.T) {
	testCases := []struct {
		name     string
		config   *ProducerConfig
		expected error
	}{
		{
			name:     "Nil config",
			config:   nil,
			expected: ErrNilConfig,
		},
		{
			name: "Missing URL",
			config: &ProducerConfig{
				Subject:  "test.subject",
				AuthType: AuthTypeNone,
			},
			expected: ErrMissingProducerURL,
		},
		{
			name: "Missing Subject",
			config: &ProducerConfig{
				URL:      "nats://localhost:4222",
				AuthType: AuthTypeNone,
			},
			expected: ErrMissingProducerTopic,
		},
		{
			name: "Invalid Auth Type",
			config: &ProducerConfig{
				URL:      "nats://localhost:4222",
				Subject:  "test.subject",
				AuthType: "invalid",
			},
			expected: ErrInvalidAuthType,
		},
		{
			name: "Valid Configuration",
			config: &ProducerConfig{
				URL:      "nats://localhost:4222",
				Subject:  "test.subject",
				AuthType: AuthTypeNone,
			},
			expected: nil,
		},
		{
			name: "Valid Basic Auth",
			config: &ProducerConfig{
				URL:      "nats://localhost:4222",
				Subject:  "test.subject",
				AuthType: AuthTypeBasic,
				Username: "user",
			},
			expected: nil,
		},
		{
			name: "Valid Token Auth",
			config: &ProducerConfig{
				URL:                     "nats://localhost:4222",
				Subject:                 "test.subject",
				AuthType:                AuthTypeToken,
				DefaultCredentialConfig: StringPasswordConfig("test-token"),
			},
			expected: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var err error
			if tc.config != nil {
				err = tc.config.Validate()
			} else {
				_, err = NewProducer(tc.config, nil)
			}
			assert.Equal(t, tc.expected, err)
		})
	}
}

func TestConsumerConfigValidate(t *testing.T) {
	testCases := []struct {
		name     string
		config   *ConsumerConfig
		expected error
	}{
		{
			name:     "Nil config",
			config:   nil,
			expected: ErrNilConfig,
		},
		{
			name: "Missing URL",
			config: &ConsumerConfig{
				Subject:  "test.subject",
				AuthType: AuthTypeNone,
			},
			expected: ErrMissingConsumerURL,
		},
		{
			name: "Missing Subject",
			config: &ConsumerConfig{
				URL:      "nats://localhost:4222",
				AuthType: AuthTypeNone,
			},
			expected: ErrMissingConsumerTopic,
		},
		{
			name: "Invalid Auth Type",
			config: &ConsumerConfig{
				URL:      "nats://localhost:4222",
				Subject:  "test.subject",
				AuthType: "invalid",
			},
			expected: ErrInvalidAuthType,
		},
		{
			name: "Valid Configuration",
			config: &ConsumerConfig{
				URL:      "nats://localhost:4222",
				Subject:  "test.subject",
				AuthType: AuthTypeNone,
			},
			expected: nil,
		},
		{
			name: "Valid with Queue Group",
			config: &ConsumerConfig{
				URL:      "nats://localhost:4222",
				Subject:  "test.subject",
				AuthType: AuthTypeNone,
				ConsumerOptions: ConsumerOptions{
					QueueGroup: "test-group",
				},
			},
			expected: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var err error
			if tc.config != nil {
				err = tc.config.Validate()
			} else {
				_, err = NewConsumer(tc.config, nil)
			}
			assert.Equal(t, tc.expected, err)
		})
	}
}
