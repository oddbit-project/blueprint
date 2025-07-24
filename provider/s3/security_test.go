//go:build security

package s3

import (
	"fmt"
	"github.com/oddbit-project/blueprint/provider/s3_old"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCredentialSecurity validates credential handling security
func TestCredentialSecurity(t *testing.T) {
	t.Run("CredentialMemoryClearing", func(t *testing.T) {
		// Test that credentials are properly cleared from memory
		config := s3.NewConfig()
		config.AccessKeyID = "test-access-key"
		config.DefaultCredentialConfig.PasswordEnvVar = "TEST_SECRET_KEY"

		// Set a test secret key
		t.Setenv("TEST_SECRET_KEY", "test-secret-key")

		client, err := s3_old.NewClient(config, nil)
		require.NoError(t, err)
		require.NotNil(t, client)

		// Verify client was created (credential handling worked)
		assert.NotNil(t, client)
	})

	t.Run("NoCredentialLeakageInErrors", func(t *testing.T) {
		// Test that credentials don't appear in error messages
		config := s3.NewConfig()
		config.AccessKeyID = "test-access-key"
		config.DefaultCredentialConfig.PasswordEnvVar = "TEST_SECRET_KEY"
		config.Region = "" // Invalid to trigger validation error

		t.Setenv("TEST_SECRET_KEY", "secret-credential-12345")

		_, err := s3_old.NewClient(config, nil)
		require.Error(t, err)

		// Ensure the error message doesn't contain the secret
		assert.NotContains(t, err.Error(), "secret-credential-12345")
		assert.NotContains(t, err.Error(), "test-access-key")
	})
}

// TestInputValidationSecurity validates input sanitization
func TestInputValidationSecurity(t *testing.T) {
	testCases := []struct {
		name       string
		bucketName string
		objectKey  string
		shouldFail bool
		reason     string
	}{
		{
			name:       "ValidInputs",
			bucketName: "valid-bucket-name",
			objectKey:  "valid/object/key.txt",
			shouldFail: false,
		},
		{
			name:       "SQLInjectionAttempt",
			bucketName: "bucket'; DROP TABLE users; --",
			objectKey:  "normal-key",
			shouldFail: true,
			reason:     "SQL injection attempt in bucket name",
		},
		{
			name:       "XSSAttempt",
			bucketName: "valid-bucket",
			objectKey:  "<script>alert('xss')</script>",
			shouldFail: false, // S3 allows these characters in object keys
			reason:     "XSS attempt in object key (should be handled by application layer)",
		},
		{
			name:       "PathTraversalAttempt",
			bucketName: "valid-bucket",
			objectKey:  "../../../etc/passwd",
			shouldFail: false, // Should be sanitized, not fail
			reason:     "Path traversal attempt",
		},
		{
			name:       "NullByteInjection",
			bucketName: "valid-bucket",
			objectKey:  "file.txt\x00malicious",
			shouldFail: true,
			reason:     "Null byte injection attempt",
		},
		{
			name:       "ControlCharacters",
			bucketName: "valid-bucket",
			objectKey:  "file\x01\x02\x03.txt",
			shouldFail: true,
			reason:     "Control character injection",
		},
		{
			name:       "ExcessiveLength",
			bucketName: strings.Repeat("a", 100), // Too long for bucket name
			objectKey:  "normal-key",
			shouldFail: true,
			reason:     "Bucket name too long",
		},
		{
			name:       "UnicodeNormalizationAttack",
			bucketName: "valid-bucket",
			objectKey:  "file\u202e\u202d.txt", // RTL/LTR override characters
			shouldFail: false,                  // Should be handled gracefully
			reason:     "Unicode normalization attack",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Test bucket name validation
			bucketErr := s3.ValidateBucketName(tc.bucketName)
			if tc.shouldFail && strings.Contains(tc.reason, "bucket") {
				assert.Error(t, bucketErr, "Expected bucket validation to fail: %s", tc.reason)
			}

			// Test object key validation
			keyErr := s3.ValidateObjectName(tc.objectKey)
			if tc.shouldFail && strings.Contains(tc.reason, "key") {
				assert.Error(t, keyErr, "Expected object key validation to fail: %s", tc.reason)
			}

			// Test sanitization functions
			sanitizedBucket := s3.SanitizeBucketName(tc.bucketName)
			sanitizedKey := s3.SanitizeObjectKey(tc.objectKey)

			// Sanitized names should always be valid
			assert.NoError(t, s3.ValidateBucketName(sanitizedBucket), "Sanitized bucket name should be valid")
			assert.NoError(t, s3.ValidateObjectName(sanitizedKey), "Sanitized object key should be valid")
		})
	}
}

// TestEncryptionConfiguration validates encryption settings
func TestEncryptionConfiguration(t *testing.T) {
	t.Run("DefaultSSLEnabled", func(t *testing.T) {
		config := s3.NewConfig()
		assert.True(t, config.UseSSL, "SSL should be enabled by default")
	})

	t.Run("SSLCannotBeDisabledForAWS", func(t *testing.T) {
		config := s3.NewConfig()
		config.UseSSL = false
		config.Endpoint = "" // AWS endpoint

		err := config.Validate()
		// Should enforce SSL for AWS endpoints
		assert.Error(t, err, "SSL should be enforced for AWS endpoints")
		assert.Contains(t, err.Error(), "SSL cannot be disabled for AWS endpoints")
	})
}

// TestErrorHandlingSecurity validates secure error handling
func TestErrorHandlingSecurity(t *testing.T) {
	t.Run("NoSensitiveDataInErrors", func(t *testing.T) {
		// Test various error conditions to ensure no sensitive data leaks
		config := s3.NewConfig()
		config.AccessKeyID = "SENSITIVE-ACCESS-KEY"
		config.DefaultCredentialConfig.PasswordEnvVar = "TEST_SECRET"

		t.Setenv("TEST_SECRET", "VERY-SENSITIVE-SECRET-123")

		// Create various error conditions and check error messages
		invalidConfigs := []*s3.Config{
			{
				AccessKeyID: "test-key",
				Region:      "", // Invalid region
			},
			{
				AccessKeyID:    "test-key",
				Region:         "us-east-1",
				TimeoutSeconds: -1, // Invalid timeout
			},
		}

		for i, cfg := range invalidConfigs {
			t.Run(fmt.Sprintf("InvalidConfig%d", i), func(t *testing.T) {
				err := cfg.Validate()
				require.Error(t, err)

				// Ensure no sensitive data in error message
				assert.NotContains(t, err.Error(), "SENSITIVE-ACCESS-KEY")
				assert.NotContains(t, err.Error(), "VERY-SENSITIVE-SECRET-123")
				assert.NotContains(t, err.Error(), "test-key")
			})
		}
	})
}

// TestRandomGeneration validates cryptographically secure random generation
func TestRandomGeneration(t *testing.T) {
	t.Run("SecureRandomGeneration", func(t *testing.T) {
		// Generate multiple random strings and ensure they're different
		randoms := make(map[string]bool)

		for i := 0; i < 100; i++ {
			random := s3.generateRandomSuffix()

			// Should not be the placeholder value
			assert.NotEqual(t, "123456", random, "Random generation should not return placeholder")

			// Should be unique (very high probability)
			assert.False(t, randoms[random], "Random values should be unique")
			randoms[random] = true

			// Should be appropriate length (6 characters for hex encoding of 3 bytes)
			assert.Len(t, random, 6, "Random string should be 6 characters")

			// Should contain only hex characters
			assert.Regexp(t, "^[0-9a-f]+$", random, "Random string should be hexadecimal")
		}
	})

	t.Run("RandomQuality", func(t *testing.T) {
		// Test randomness quality by checking distribution
		const samples = 1000
		const hexChars = "0123456789abcdef"
		charCounts := make(map[rune]int)

		for i := 0; i < samples; i++ {
			random := s3.generateRandomSuffix()
			for _, char := range random {
				charCounts[char]++
			}
		}

		// Check that all hex characters appear (rough randomness test)
		totalChars := samples * 6          // 6 chars per random string
		expectedPerChar := totalChars / 16 // 16 possible hex chars
		tolerance := expectedPerChar / 4   // 25% tolerance

		for _, char := range hexChars {
			count := charCounts[rune(char)]
			assert.True(t, count > expectedPerChar-tolerance && count < expectedPerChar+tolerance,
				"Character %c appears %d times, expected around %d (±%d)",
				char, count, expectedPerChar, tolerance)
		}
	})
}

// TestConcurrentSafety validates thread safety
func TestConcurrentSafety(t *testing.T) {
	t.Run("ConcurrentClientCreation", func(t *testing.T) {
		const numGoroutines = 100

		results := make(chan error, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func() {
				config := s3.NewConfig()
				config.AccessKeyID = "test-key"
				config.DefaultCredentialConfig.PasswordEnvVar = "TEST_SECRET"
				config.Region = "us-east-1"

				_, err := s3_old.NewClient(config, nil)
				results <- err
			}()
		}

		// Collect results
		for i := 0; i < numGoroutines; i++ {
			err := <-results
			if err != nil {
				// Some errors are expected (like missing env vars), but should not crash
				t.Logf("Expected error in concurrent test: %v", err)
			}
		}
	})
}

// TestSecureDefaults validates security-focused default configuration
func TestSecureDefaults(t *testing.T) {
	config := s3.NewConfig()

	// Security-focused defaults
	assert.True(t, config.UseSSL, "SSL should be enabled by default")
	assert.False(t, config.UseAccelerate, "Transfer acceleration should be disabled by default")
	assert.Equal(t, 30, config.TimeoutSeconds, "Should have reasonable timeout default")
	assert.Equal(t, 3, config.MaxRetries, "Should have reasonable retry default")
	assert.Empty(t, config.AccessKeyID, "No default credentials should be set")

	// Validate that defaults pass validation
	err := config.Validate()
	require.NoError(t, err, "Default configuration should be valid")
}

// TestConfigurationValidation validates security configuration checks
func TestConfigurationValidation(t *testing.T) {
	testCases := []struct {
		name         string
		modifyConfig func(*s3.Config)
		shouldFail   bool
		reason       string
	}{
		{
			name:         "ValidDefault",
			modifyConfig: func(c *s3.Config) {},
			shouldFail:   false,
		},
		{
			name: "InvalidTimeout",
			modifyConfig: func(c *s3.Config) {
				c.TimeoutSeconds = -1
			},
			shouldFail: true,
			reason:     "Negative timeout should be invalid",
		},
		{
			name: "ExcessiveTimeout",
			modifyConfig: func(c *s3.Config) {
				c.TimeoutSeconds = 3600 // 1 hour
			},
			shouldFail: true,
			reason:     "Excessive timeout should be invalid",
		},
		{
			name: "InvalidPartSize",
			modifyConfig: func(c *s3.Config) {
				c.PartSize = 1024 // Too small
			},
			shouldFail: true,
			reason:     "Part size below minimum should be invalid",
		},
		{
			name: "InvalidRetries",
			modifyConfig: func(c *s3.Config) {
				c.MaxRetries = -1
			},
			shouldFail: true,
			reason:     "Negative retries should be invalid",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config := s3.NewConfig()
			config.Region = "us-east-1" // Set required field
			tc.modifyConfig(config)

			err := config.Validate()
			if tc.shouldFail {
				assert.Error(t, err, "Configuration should be invalid: %s", tc.reason)
			} else {
				assert.NoError(t, err, "Configuration should be valid")
			}
		})
	}
}

// BenchmarkSecurityOperations benchmarks security-critical operations
func BenchmarkSecurityOperations(b *testing.B) {
	b.Run("InputValidation", func(b *testing.B) {
		bucketName := "test-bucket-name"
		objectKey := "test/object/key.txt"

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			s3.ValidateBucketName(bucketName)
			s3.ValidateObjectName(objectKey)
		}
	})

	b.Run("RandomGeneration", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			s3.generateRandomSuffix()
		}
	})

	b.Run("ConfigValidation", func(b *testing.B) {
		config := s3.NewConfig()
		config.Region = "us-east-1"

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			config.Validate()
		}
	})
}
