package hmacprovider

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/oddbit-project/blueprint/crypt/secure"
	"github.com/oddbit-project/blueprint/provider/hmacprovider/store"
	"github.com/oddbit-project/blueprint/provider/kv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// global userId list for tests
var userNames = []string{"", "someUser", "bob"}

// Integration tests combining HMAC provider with different nonce stores
func TestHMACProviderWithMemoryStore(t *testing.T) {
	// Create provider with memory store
	key, err := secure.GenerateKey()
	require.NoError(t, err)

	credential, err := secure.NewCredential([]byte("integration-test-secret"), key, false)
	require.NoError(t, err)

	memoryStore := store.NewMemoryNonceStore(
		store.WithTTL(1*time.Hour),
		store.WithMaxSize(1000),
	)

	keyProvider := NewSingleKeyProvider("", credential)
	provider := NewHmacProvider(keyProvider,
		WithNonceStore(memoryStore),
		WithKeyInterval(5*time.Minute),
	)

	testData := "integration test data"

	// Test complete sign and verify cycle
	hash, timestamp, nonce, err := provider.Sign256("", strings.NewReader(testData))
	require.NoError(t, err)

	userId, valid, err := provider.Verify256(strings.NewReader(testData), hash, timestamp, nonce)
	assert.NoError(t, err)
	assert.True(t, valid)
	assert.Equal(t, "", string(userId))
	// Test replay protection
	userId, valid, err = provider.Verify256(strings.NewReader(testData), hash, timestamp, nonce)
	assert.Error(t, err)
	assert.False(t, valid)
}

func TestHMACProviderWithKVStore(t *testing.T) {
	// Create provider with KV store
	key, err := secure.GenerateKey()
	require.NoError(t, err)

	credential, err := secure.NewCredential([]byte("integration-test-secret"), key, false)
	require.NoError(t, err)

	memKV := kv.NewMemoryKV()
	kvStore := store.NewKvStore(memKV, 1*time.Hour)

	user := "api-key"
	keyProvider := NewSingleKeyProvider(user, credential)
	provider := NewHmacProvider(keyProvider,
		WithNonceStore(kvStore),
		WithKeyInterval(5*time.Minute),
	)

	testData := "kv integration test data"

	// Test complete sign and verify cycle
	hash, timestamp, nonce, err := provider.Sign256(user, strings.NewReader(testData))
	require.NoError(t, err)
	parts := strings.Split(string(hash), ".")
	assert.Equal(t, user, parts[0])
	assert.Len(t, parts, 2)

	userId, valid, err := provider.Verify256(strings.NewReader(testData), hash, timestamp, nonce)
	assert.NoError(t, err)
	assert.True(t, valid)
	assert.Equal(t, user, userId)

	// Verify nonce was stored in KV
	value, err := memKV.Get(nonce)
	assert.NoError(t, err)
	assert.Equal(t, []byte("1"), value)

	// Test replay protection
	userId, valid, err = provider.Verify256(strings.NewReader(testData), hash, timestamp, nonce)
	assert.Error(t, err)
	assert.False(t, valid)
}

func TestHMACProviderSignVerifyRoundtrip(t *testing.T) {
	testCases := []struct {
		name   string
		data   string
		userId string
	}{
		{"empty data", "", ""},
		{"empty data", "", "someUser"},
		{"short data", "test", ""},
		{"short data", "test", "otherUser"},
		{"long data", strings.Repeat("long test data ", 100), ""},
		{"long data", strings.Repeat("long test data ", 100), "someUser"},
		{"unicode data", "Hello 世界 🌍", ""},
		{"unicode data", "Hello 世界 🌍", "世界"},
		{"binary-like data", "\x00\x01\x02\xff\xfe\xfd", "01010"},
		{"binary-like data", "\x00\x01\x02\xff\xfe\xfd", ""},
	}

	for _, tc := range testCases {
		provider := createTestHMACProvider(t, tc.userId)
		t.Run(tc.name, func(t *testing.T) {
			// Test SHA256 methods
			hash, err := provider.SHA256Sign(tc.userId, strings.NewReader(tc.data))
			require.NoError(t, err)

			userId, valid, err := provider.SHA256Verify(strings.NewReader(tc.data), hash)
			assert.NoError(t, err)
			assert.True(t, valid)
			assert.Equal(t, tc.userId, userId)

			// Test Sign256/Verify256 methods
			hash256, timestamp, nonce, err := provider.Sign256(tc.userId, strings.NewReader(tc.data))
			require.NoError(t, err)

			userId, valid, err = provider.Verify256(strings.NewReader(tc.data), hash256, timestamp, nonce)
			assert.NoError(t, err)
			assert.True(t, valid)
			assert.Equal(t, tc.userId, userId)
		})
	}
}

func TestHMACProviderMultipleNonces(t *testing.T) {
	for _, user := range userNames {
		provider := createTestHMACProvider(t, user)

		testData := "test data for multiple nonces"
		const numRequests = 10

		// Generate multiple signatures (each with unique nonce)
		signatures := make([]struct{ hash, timestamp, nonce string }, numRequests)

		for i := 0; i < numRequests; i++ {
			hash, timestamp, nonce, err := provider.Sign256(user, strings.NewReader(testData))
			require.NoError(t, err)
			signatures[i] = struct{ hash, timestamp, nonce string }{hash, timestamp, nonce}
		}

		// All signatures should verify successfully (different nonces)
		for i, sig := range signatures {
			userId, valid, err := provider.Verify256(strings.NewReader(testData), sig.hash, sig.timestamp, sig.nonce)
			assert.NoError(t, err, "Verification %d should succeed", i)
			assert.True(t, valid, "Signature %d should be valid", i)
			assert.Equal(t, user, userId, "Verification %d should match", i)
		}

		// None should verify a second time (replay protection)
		for i, sig := range signatures {
			_, valid, err := provider.Verify256(strings.NewReader(testData), sig.hash, sig.timestamp, sig.nonce)
			assert.Error(t, err, "Second verification %d should fail", i)
			assert.False(t, valid, "Second verification %d should be invalid", i)
		}
	}
}

func TestHMACProviderTimeWindow(t *testing.T) {
	for _, user := range userNames {
		shortInterval := 1 * time.Second // Longer interval to allow verification
		provider := createTestHMACProvider(t, user, WithKeyInterval(shortInterval))

		testData := "time window test data"

		// Generate signature
		hash, timestamp, nonce, err := provider.Sign256(user, strings.NewReader(testData))
		require.NoError(t, err)
		if user != "" {
			assert.Len(t, strings.Split(hash, "."), 2)
		} else {
			assert.Len(t, strings.Split(timestamp, "."), 1)
		}

		// Should verify immediately
		userId, valid, err := provider.Verify256(strings.NewReader(testData), hash, timestamp, nonce)
		require.NoError(t, err)
		require.True(t, valid)
		require.Equal(t, user, userId, "Verification %d should match", 1)

		// Generate another signature and wait for it to expire
		hash2, timestamp2, nonce2, err := provider.Sign256(userId, strings.NewReader(testData))
		require.NoError(t, err)
		if user != "" {
			assert.Len(t, strings.Split(hash, "."), 2)
		} else {
			assert.Len(t, strings.Split(timestamp, "."), 1)
		}

		// Wait for timestamp to expire
		time.Sleep(shortInterval + 100*time.Millisecond)

		// Should fail due to expired timestamp
		userId, valid, err = provider.Verify256(strings.NewReader(testData), hash2, timestamp2, nonce2)
		assert.Error(t, err)
		assert.False(t, valid)
	}
}

func TestHMACProviderDifferentSecrets(t *testing.T) {
	for _, user := range userNames {

		// Create two providers with different secrets
		key1, err := secure.GenerateKey()
		require.NoError(t, err)
		credential1, err := secure.NewCredential([]byte("secret1"), key1, false)
		require.NoError(t, err)
		provider1 := NewHmacProvider(NewSingleKeyProvider(user, credential1))

		key2, err := secure.GenerateKey()
		require.NoError(t, err)
		credential2, err := secure.NewCredential([]byte("secret2"), key2, false)
		require.NoError(t, err)
		provider2 := NewHmacProvider(NewSingleKeyProvider(user, credential2))

		testData := "cross-provider test data"

		// Sign with provider1
		hash, timestamp, nonce, err := provider1.Sign256(user, strings.NewReader(testData))
		require.NoError(t, err)

		// Should verify with provider1
		userId, valid, err := provider1.Verify256(strings.NewReader(testData), hash, timestamp, nonce)
		assert.NoError(t, err)
		assert.True(t, valid)
		assert.Equal(t, user, userId, "Verification %d should match", 1)

		// Should NOT verify with provider2 (different secret)
		_, valid, err = provider2.Verify256(strings.NewReader(testData), hash, timestamp, nonce)
		assert.Error(t, err)
		assert.False(t, valid)
	}
}

func TestHMACProviderLargeDataHandling(t *testing.T) {
	for _, user := range userNames {
		// Test with custom max size
		maxSize := 1024
		provider := createTestHMACProvider(t, user, WithMaxInputSize(maxSize))

		// Data exactly at limit should work
		exactData := strings.Repeat("a", maxSize)
		hash, err := provider.SHA256Sign(user, strings.NewReader(exactData))
		assert.NoError(t, err)
		assert.NotEmpty(t, hash)

		userId, valid, err := provider.SHA256Verify(strings.NewReader(exactData), hash)
		assert.NoError(t, err)
		assert.True(t, valid)
		assert.Equal(t, user, userId, "Verification %d should match", 1)

		// Data over limit should fail
		overData := strings.Repeat("a", maxSize+1)
		_, err = provider.SHA256Sign(user, strings.NewReader(overData))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "input too large")

		// Verify also should fail with large data
		_, valid, err = provider.SHA256Verify(strings.NewReader(overData), "deadbeef")
		assert.Error(t, err)
		assert.False(t, valid)
	}
}

func TestHMACProviderErrorPropagation(t *testing.T) {
	for _, user := range userNames {
		// Test error handling throughout the system

		// Create provider with failing nonce store
		mockStore := newMockNonceStore()
		provider := createTestHMACProvider(t, user, WithNonceStore(mockStore))

		testData := "error propagation test"

		// Generate signature (should work)
		mockStore.setFail(false)
		hash, timestamp, nonce, err := provider.Sign256(user, strings.NewReader(testData))
		require.NoError(t, err)

		// Make nonce store fail
		mockStore.setFail(true)

		// Verification should fail safely
		_, valid, err := provider.Verify256(strings.NewReader(testData), hash, timestamp, nonce)
		assert.Error(t, err)
		assert.False(t, valid)
		assert.Contains(t, err.Error(), "invalid request")
	}
}

func TestHMACProviderConcurrentAccess(t *testing.T) {
	for _, user := range userNames {
		provider := createTestHMACProvider(t, user)

		const numGoroutines = 50
		const operationsPerGoroutine = 20

		results := make(chan bool, numGoroutines*operationsPerGoroutine)

		// Launch multiple goroutines performing operations
		for i := 0; i < numGoroutines; i++ {
			go func(goroutineID int) {
				for j := 0; j < operationsPerGoroutine; j++ {
					testData := fmt.Sprintf("concurrent-test-%d-%d", goroutineID, j)

					// Sign and verify
					hash, timestamp, nonce, err := provider.Sign256(user, strings.NewReader(testData))
					if err != nil {
						results <- false
						continue
					}

					userId, valid, err := provider.Verify256(strings.NewReader(testData), hash, timestamp, nonce)
					if err != nil || !valid || userId != user {
						results <- false
						continue
					}

					results <- true
				}
			}(i)
		}

		// Wait for all operations to complete
		successCount := 0
		for i := 0; i < numGoroutines*operationsPerGoroutine; i++ {
			if <-results {
				successCount++
			}
		}

		// All operations should succeed
		expected := numGoroutines * operationsPerGoroutine
		assert.Equal(t, expected, successCount, "All concurrent operations should succeed")
	}
}

// Benchmark integration scenarios
func BenchmarkHMACProviderFullCycle(b *testing.B) {
	for _, user := range userNames {
		provider := createTestHMACProvider(&testing.T{}, user)
		testData := "benchmark integration test data"

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// Sign
			hash, timestamp, nonce, err := provider.Sign256(user, strings.NewReader(testData))
			if err != nil {
				b.Fatal(err)
			}

			// Verify
			userId, valid, err := provider.Verify256(strings.NewReader(testData), hash, timestamp, nonce)
			if err != nil || !valid || userId != user {
				b.Fatal("verification failed")
			}
		}
	}
}
