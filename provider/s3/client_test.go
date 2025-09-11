package s3

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {
	testCases := []struct {
		name        string
		config      *Config
		expectError bool
	}{
		{
			name:        "nil Config uses default",
			config:      nil,
			expectError: false,
		},
		{
			name:        "valid Config",
			config:      NewConfig(),
			expectError: false,
		},
		{
			name: "invalid Config",
			config: &Config{
				TimeoutSeconds: -1, // Invalid
			},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client, err := NewClient(tc.config, nil)

			if tc.expectError {
				assert.Error(t, err)
				assert.Nil(t, client)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, client)
				assert.False(t, client.IsConnected()) // Should not be connected initially
			}
		})
	}
}

func TestClientConnectionState(t *testing.T) {
	client, err := NewClient(NewConfig(), nil)
	require.NoError(t, err)
	require.NotNil(t, client)

	// Initially not connected
	assert.False(t, client.IsConnected())

	// After closing (even without connecting), should still be not connected
	err = client.Close()
	assert.NoError(t, err)
	assert.False(t, client.IsConnected())
}

func TestClientTimeoutContext(t *testing.T) {
	config := NewConfig()
	config.TimeoutSeconds = 30

	client, err := NewClient(config, nil)
	require.NoError(t, err)

	// Test timeout context creation
	ctx := context.Background()
	timeoutCtx, cancel := getContextWithTimeout(client.timeout, ctx)
	defer cancel()

	assert.NotNil(t, timeoutCtx)
	// The timeout context should be different from the original when timeout is set
	assert.NotEqual(t, ctx, timeoutCtx)
}

// Mock tests would require actual AWS credentials or a mock service
// For now, we focus on unit testing the logic that doesn't require external services

func TestClientConstants(t *testing.T) {
	assert.Equal(t, int64(100*1024*1024), DefaultMultipartThreshold)
	assert.Equal(t, int64(10*1024*1024), DefaultPartSize) // Updated to 10MB
	assert.Equal(t, int64(5*1024*1024), MinPartSize)
	assert.Equal(t, int64(5*1024*1024*1024), MaxPartSize)
	assert.Equal(t, "eu-west-1", DefaultRegion)
	assert.Equal(t, 10000, DefaultMaxUploadParts)
	assert.Equal(t, 3, DefaultMaxRetries)
}
