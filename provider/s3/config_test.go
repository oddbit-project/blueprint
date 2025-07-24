package s3

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewConfig(t *testing.T) {
	config := NewConfig()

	assert.NotNil(t, config)
	assert.Equal(t, DefaultRegion, config.Region)
	assert.True(t, config.UseSSL)
	assert.Equal(t, int(DefaultTimeout.Seconds()), config.TimeoutSeconds)
	assert.Equal(t, DefaultMultipartThreshold, config.MultipartThreshold)
	assert.Equal(t, DefaultPartSize, config.PartSize)
	assert.Equal(t, DefaultMaxUploadParts, config.MaxUploadParts)
	assert.Equal(t, 5, config.Concurrency)
	assert.Equal(t, DefaultMaxRetries, config.MaxRetries)
	assert.Equal(t, "standard", config.RetryMode)
}

func TestConfigValidation(t *testing.T) {
	testCases := []struct {
		name        string
		config      *Config
		expectError bool
	}{
		{
			name:        "nil Config",
			config:      nil,
			expectError: true,
		},
		{
			name:        "valid default Config",
			config:      NewConfig(),
			expectError: false,
		},
		{
			name: "missing region and endpoint",
			config: &Config{
				TimeoutSeconds:     30,
				MultipartThreshold: DefaultMultipartThreshold,
				PartSize:           DefaultPartSize,
			},
			expectError: true,
		},
		{
			name: "valid with custom endpoint",
			config: &Config{
				Endpoint:           "https://s3.example.com",
				Region:             "us-west-2",
				UseSSL:             true,
				TimeoutSeconds:     30,
				MultipartThreshold: DefaultMultipartThreshold,
				PartSize:           DefaultPartSize,
			},
			expectError: false,
		},
		{
			name: "invalid timeout",
			config: &Config{
				Region:             DefaultRegion,
				TimeoutSeconds:     -1,
				MultipartThreshold: DefaultMultipartThreshold,
				PartSize:           DefaultPartSize,
			},
			expectError: true,
		},
		{
			name: "invalid part size - too small",
			config: &Config{
				Region:             DefaultRegion,
				TimeoutSeconds:     30,
				MultipartThreshold: DefaultMultipartThreshold,
				PartSize:           1024, // Less than MinPartSize
			},
			expectError: true,
		},
		{
			name: "invalid part size - too large",
			config: &Config{
				Region:             DefaultRegion,
				TimeoutSeconds:     30,
				MultipartThreshold: DefaultMultipartThreshold,
				PartSize:           MaxPartSize + 1,
			},
			expectError: true,
		},
		{
			name: "invalid threshold - smaller than part size",
			config: &Config{
				Region:             DefaultRegion,
				TimeoutSeconds:     30,
				MultipartThreshold: DefaultPartSize - 1,
				PartSize:           DefaultPartSize,
			},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.config.Validate()
			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfigDefaultTimeout(t *testing.T) {
	config := &Config{
		Region:             DefaultRegion,
		MultipartThreshold: DefaultMultipartThreshold,
		PartSize:           DefaultPartSize,
		UseSSL:             true, // Required for AWS endpoints
		// TimeoutSeconds not set (0)
	}

	err := config.Validate()
	require.NoError(t, err)
	assert.Equal(t, int(DefaultTimeout.Seconds()), config.TimeoutSeconds)
}

func TestIsCustomEndpoint(t *testing.T) {
	testCases := []struct {
		name     string
		endpoint string
		expected bool
	}{
		{"empty endpoint", "", false},
		{"custom endpoint", "https://s3.example.com", true},
		{"minio endpoint", "http://localhost:9000", true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config := &Config{Endpoint: tc.endpoint}
			assert.Equal(t, tc.expected, config.IsCustomEndpoint())
		})
	}
}

func TestGetEndpointURL(t *testing.T) {
	testCases := []struct {
		name     string
		config   *Config
		expected string
	}{
		{
			name:     "empty endpoint",
			config:   &Config{Endpoint: ""},
			expected: "",
		},
		{
			name:     "endpoint with https and UseSSL true",
			config:   &Config{Endpoint: "s3.example.com", UseSSL: true},
			expected: "https://s3.example.com",
		},
		{
			name:     "endpoint with http and UseSSL false",
			config:   &Config{Endpoint: "localhost:9000", UseSSL: false},
			expected: "http://localhost:9000",
		},
		{
			name:     "endpoint already has https protocol",
			config:   &Config{Endpoint: "https://s3.example.com", UseSSL: true},
			expected: "https://s3.example.com",
		},
		{
			name:     "endpoint already has http protocol",
			config:   &Config{Endpoint: "http://localhost:9000", UseSSL: false},
			expected: "http://localhost:9000",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, tc.config.GetEndpointURL())
		})
	}
}
