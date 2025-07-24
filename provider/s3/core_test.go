package s3

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidationCoverage(t *testing.T) {
	// Test validation functions to improve coverage

	t.Run("ValidateBucketName coverage", func(t *testing.T) {
		validNames := []string{
			"test-bucket",
			"my.bucket.name",
			"bucket123",
			"bucket-with-numbers123",
		}

		for _, name := range validNames {
			err := ValidateBucketName(name)
			assert.NoError(t, err, "Expected %s to be valid", name)
		}

		invalidNames := []string{
			"",
			"ab",
			"UPPERCASE",
			"-starts-with-dash",
			"ends-with-dash-",
			".starts-with-dot",
			"ends-with-dot.",
			"has..consecutive..dots",
			"has.-dot-dash",
			"has-.dash-dot",
			"192.168.1.1",
			"xn--test",
			"test-s3alias",
			"test--ol-s3",
			"test bucket",
			"test_bucket",
		}

		for _, name := range invalidNames {
			err := ValidateBucketName(name)
			assert.Error(t, err, "Expected %s to be invalid", name)
		}
	})

	t.Run("ValidateObjectName coverage", func(t *testing.T) {
		validKeys := []string{
			"simple-key",
			"path/to/file.txt",
			"file with spaces.txt",
			"special-chars!@#$%^&*().txt",
			"unicode-文件.txt",
			"a",                       // minimum length
			strings.Repeat("a", 1024), // maximum length
		}

		for _, key := range validKeys {
			err := ValidateObjectName(key)
			assert.NoError(t, err, "Expected %s to be valid", key)
		}

		invalidKeys := []string{
			"",
			strings.Repeat("a", 1025), // too long
			"key\x00with\x01control\x02chars",
			"key\x7fwith\x80extended\xffchars",
		}

		for _, key := range invalidKeys {
			err := ValidateObjectName(key)
			assert.Error(t, err, "Expected %s to be invalid", key)
		}
	})
}

func TestSanitizationCoverage(t *testing.T) {
	t.Run("SanitizeBucketName coverage", func(t *testing.T) {
		testCases := []struct {
			input    string
			expected string
		}{
			{"UPPERCASE", "uppercase"},
			{"has spaces", "has-spaces"},
			{"has_underscores", "has-underscores"},
			{"has---multiple---dashes", "has-multiple-dashes"},
			{"--starts-and-ends--", "starts-and-ends"},
			{"ab", "abx"}, // padding with random suffix
			{strings.Repeat("a", 70), strings.Repeat("a", 63)}, // truncation
			{"special@#$chars", "special-chars"},
		}

		for _, tc := range testCases {
			result := SanitizeBucketName(tc.input)
			if tc.input == "ab" {
				// For short names, expect padding with random suffix
				assert.True(t, len(result) >= 3, "Short name should be padded")
				assert.True(t, strings.HasPrefix(result, "ab"), "Should preserve original prefix")
			} else {
				assert.Equal(t, tc.expected, result, "Sanitization of %s failed", tc.input)
			}
		}
	})

	t.Run("SanitizeObjectKey coverage", func(t *testing.T) {
		testCases := []struct {
			input    string
			expected string
		}{
			{"back\\slash\\path", "back/slash/path"},
			{`"double quotes"`, "'double quotes'"},
			{"key\x00with\x01control", "keywithcontrol"},
			{"/leading/slashes", "leading/slashes"},
			{"unicode-文件.txt", "unicode-文件.txt"},                   // preserved
			{strings.Repeat("a", 1100), strings.Repeat("a", 1024)}, // truncation
			{"", ""}, // empty becomes random key
		}

		for _, tc := range testCases {
			result := SanitizeObjectKey(tc.input)
			if tc.input == "" {
				// For empty keys, expect a generated key
				assert.True(t, len(result) > 0, "Empty key should generate a default")
				assert.True(t, strings.Contains(result, "object-"), "Should contain object- prefix")
			} else {
				assert.Equal(t, tc.expected, result, "Sanitization of %s failed", tc.input)
			}
		}
	})
}

func TestHelperFunctions(t *testing.T) {
	t.Run("IsBucketNameValid", func(t *testing.T) {
		assert.True(t, IsBucketNameValid("valid-bucket"))
		assert.False(t, IsBucketNameValid("INVALID-BUCKET"))
	})

	t.Run("IsObjectKeyValid", func(t *testing.T) {
		assert.True(t, IsObjectKeyValid("valid-key.txt"))
		assert.False(t, IsObjectKeyValid(""))
	})

	t.Run("BucketNameValidationRules", func(t *testing.T) {
		rules := BucketNameValidationRules()
		assert.Contains(t, rules, "Must be 3-63 characters long")
		assert.Contains(t, rules, "Must contain only lowercase letters, numbers, hyphens, and periods")
	})

	t.Run("ObjectKeyValidationRules", func(t *testing.T) {
		rules := ObjectKeyValidationRules()
		assert.Contains(t, rules, "Must be 1-1024 characters long")
		assert.Contains(t, rules, "Should not contain control characters")
	})
}

func TestConfigCoverage(t *testing.T) {
	t.Run("Config validation coverage", func(t *testing.T) {
		// Test default Config
		config := NewConfig()
		err := config.Validate()
		assert.NoError(t, err)

		// Test invalid timeout
		config.TimeoutSeconds = -1
		err = config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid timeout")

		// Test invalid part size - too small
		config = NewConfig()
		config.PartSize = 1024 // < 5MB minimum
		err = config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid part size")

		// Test invalid part size - too large
		config = NewConfig()
		config.PartSize = 6 * 1024 * 1024 * 1024 // > 5GB maximum
		err = config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid part size")

		// Test invalid threshold
		config = NewConfig()
		config.MultipartThreshold = 1024 // Smaller than part size
		err = config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid multipart threshold")
	})

	t.Run("Config endpoint helpers", func(t *testing.T) {
		config := NewConfig()

		// Test default endpoint
		assert.False(t, config.IsCustomEndpoint())

		// Test custom endpoint
		config.Endpoint = "localhost:9000"
		assert.True(t, config.IsCustomEndpoint())

		// Test endpoint URL generation
		config.UseSSL = false
		url := config.GetEndpointURL()
		assert.Equal(t, "http://localhost:9000", url)

		config.UseSSL = true
		url = config.GetEndpointURL()
		assert.Equal(t, "https://localhost:9000", url)

		// Test with protocol already specified
		config.Endpoint = "https://s3.amazonaws.com"
		url = config.GetEndpointURL()
		assert.Equal(t, "https://s3.amazonaws.com", url)
	})
}

func TestClientBasics(t *testing.T) {
	t.Run("Client creation and connection state", func(t *testing.T) {
		// Test with nil Config
		client, err := NewClient(nil, nil)
		assert.NoError(t, err)
		assert.NotNil(t, client)
		assert.False(t, client.IsConnected())

		// Test with valid Config
		config := NewConfig()
		client, err = NewClient(config, nil)
		assert.NoError(t, err)
		assert.NotNil(t, client)
		assert.False(t, client.IsConnected())

		// Test with invalid Config
		config.TimeoutSeconds = -1
		client, err = NewClient(config, nil)
		assert.Error(t, err)
		assert.Nil(t, client)
	})

	t.Run("Client close", func(t *testing.T) {
		config := NewConfig()
		client, err := NewClient(config, nil)
		require.NoError(t, err)

		err = client.Close()
		assert.NoError(t, err)
		assert.False(t, client.IsConnected())
	})
}

func TestConstants(t *testing.T) {
	// Test that all constants are properly defined
	assert.Equal(t, "eu-west-1", DefaultRegion)
	assert.Equal(t, int64(100*1024*1024), DefaultMultipartThreshold)
	assert.Equal(t, int64(10*1024*1024), DefaultPartSize)
	assert.Equal(t, int64(5*1024*1024), MinPartSize)
	assert.Equal(t, int64(5*1024*1024*1024), MaxPartSize)
	assert.Equal(t, 10000, DefaultMaxUploadParts)
	assert.Equal(t, 3, DefaultMaxRetries)

	// Encryption constants
	assert.Equal(t, "AES256", SSEAlgorithmAES256)
	assert.Equal(t, "aws:kms", SSEAlgorithmKMS)
	assert.Equal(t, "aws:kms:dsse", SSEAlgorithmKMSDSSE)
	assert.Equal(t, "AES256", SSECAlgorithmAES256)
}

func TestOperationsWithoutConnection(t *testing.T) {
	// Create a client that's not connected
	config := NewConfig()
	client, err := NewClient(config, nil)
	require.NoError(t, err)

	assert.False(t, client.connected) // Should not be connected initially

	ctx := context.Background()

	t.Run("Bucket operations fail when not connected", func(t *testing.T) {
		bucket, err := client.Bucket("test-bucket")
		err = bucket.Create(ctx)
		assert.Error(t, err)
		assert.Equal(t, ErrClientNotConnected, err)

		bucket, err = client.Bucket("test-bucket")
		err = bucket.Delete(ctx)
		assert.Error(t, err)
		assert.Equal(t, ErrClientNotConnected, err)

		_, err = client.ListBuckets(ctx)
		assert.Error(t, err)
		assert.Equal(t, ErrClientNotConnected, err)

		bucket, err = client.Bucket("test-bucket")
		_, err = bucket.Exists(ctx)
		assert.Error(t, err)
		assert.Equal(t, ErrClientNotConnected, err)
	})

	t.Run("Object operations fail when not connected", func(t *testing.T) {
		reader := strings.NewReader("test")

		bucket, err := client.Bucket("bucket")
		assert.NoError(t, err)
		err = bucket.PutObject(ctx, "key", reader, 4)
		assert.Error(t, err)
		assert.Equal(t, ErrClientNotConnected, err)

		_, err = bucket.GetObject(ctx, "key")
		assert.Error(t, err)
		assert.Equal(t, ErrClientNotConnected, err)

		err = bucket.DeleteObject(ctx, "key")
		assert.Error(t, err)
		assert.Equal(t, ErrClientNotConnected, err)

		_, err = bucket.ListObjects(ctx)
		assert.Error(t, err)
		assert.Equal(t, ErrClientNotConnected, err)

		_, err = bucket.ObjectExists(ctx, "key")
		assert.Error(t, err)
		assert.Equal(t, ErrClientNotConnected, err)

		_, err = bucket.HeadObject(ctx, "key")
		assert.Error(t, err)
		assert.Equal(t, ErrClientNotConnected, err)
	})
}
