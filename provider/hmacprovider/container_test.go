package hmacprovider

import (
	"encoding/hex"
	"strings"
	"testing"
	"time"

	"github.com/oddbit-project/blueprint/crypt/secure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockNonceStore for testing
type mockNonceStore struct {
	nonces map[string]bool
	fail   bool
}

func newMockNonceStore() *mockNonceStore {
	return &mockNonceStore{
		nonces: make(map[string]bool),
		fail:   false,
	}
}

func (m *mockNonceStore) AddIfNotExists(nonce string) bool {
	if m.fail {
		return false
	}
	if m.nonces[nonce] {
		return false
	}
	m.nonces[nonce] = true
	return true
}

func (m *mockNonceStore) setFail(fail bool) {
	m.fail = fail
}

func (m *mockNonceStore) Close() {
	// No-op for mock
}

// Test helper to create a test HMAC provider
func createTestHMACProvider(t *testing.T, opts ...HMACProviderOption) *HMACProvider {
	key, err := secure.GenerateKey()
	require.NoError(t, err)
	
	credential, err := secure.NewCredential([]byte("test-secret"), key, false)
	require.NoError(t, err)
	
	return NewHmacProvider(credential, opts...)
}

func TestNewHmacProvider(t *testing.T) {
	provider := createTestHMACProvider(t)
	
	assert.NotNil(t, provider)
	assert.NotNil(t, provider.secret)
	assert.NotNil(t, provider.nonceStore)
	assert.Equal(t, DefaultKeyInterval, provider.interval)
	assert.Equal(t, MaxInputSize, provider.maxInputSize)
}

func TestNewHmacProviderWithOptions(t *testing.T) {
	mockStore := newMockNonceStore()
	customInterval := 10 * time.Minute
	customMaxSize := 1024 * 1024
	
	provider := createTestHMACProvider(t,
		WithNonceStore(mockStore),
		WithKeyInterval(customInterval),
		WithMaxInputSize(customMaxSize),
	)
	
	assert.Equal(t, mockStore, provider.nonceStore)
	assert.Equal(t, customInterval, provider.interval)
	assert.Equal(t, customMaxSize, provider.maxInputSize)
}

func TestSHA256Sign(t *testing.T) {
	provider := createTestHMACProvider(t)
	
	testData := "Hello, World!"
	reader := strings.NewReader(testData)
	
	hash, err := provider.SHA256Sign(reader)
	
	assert.NoError(t, err)
	assert.NotEmpty(t, hash)
	
	// Verify it's valid hex
	_, err = hex.DecodeString(hash)
	assert.NoError(t, err)
	
	// Verify hash length (SHA256 = 32 bytes = 64 hex chars)
	assert.Equal(t, 64, len(hash))
}

func TestSHA256SignConsistency(t *testing.T) {
	provider := createTestHMACProvider(t)
	
	testData := "test data"
	
	// Sign the same data multiple times
	hash1, err1 := provider.SHA256Sign(strings.NewReader(testData))
	hash2, err2 := provider.SHA256Sign(strings.NewReader(testData))
	
	assert.NoError(t, err1)
	assert.NoError(t, err2)
	assert.Equal(t, hash1, hash2, "Same data should produce same hash")
}

func TestSHA256Verify(t *testing.T) {
	provider := createTestHMACProvider(t)
	
	testData := "Hello, World!"
	reader := strings.NewReader(testData)
	
	// Sign the data
	hash, err := provider.SHA256Sign(reader)
	require.NoError(t, err)
	
	// Verify with correct data and hash
	valid, err := provider.SHA256Verify(strings.NewReader(testData), hash)
	assert.NoError(t, err)
	assert.True(t, valid)
	
	// Verify with wrong data
	valid, err = provider.SHA256Verify(strings.NewReader("Wrong data"), hash)
	assert.NoError(t, err)
	assert.False(t, valid)
	
	// Verify with wrong hash (but valid hex)
	valid, err = provider.SHA256Verify(strings.NewReader(testData), "deadbeef")
	assert.NoError(t, err) // Valid hex, should decode fine
	assert.False(t, valid) // But HMAC should not match
}

func TestSHA256VerifyInvalidHex(t *testing.T) {
	provider := createTestHMACProvider(t)
	
	testData := "test"
	
	// Test with invalid hex characters
	valid, err := provider.SHA256Verify(strings.NewReader(testData), "not-hex")
	assert.Error(t, err)
	assert.False(t, valid)
	assert.Contains(t, err.Error(), "invalid hash format")
}

func TestSHA256SignLargeInput(t *testing.T) {
	provider := createTestHMACProvider(t, WithMaxInputSize(1024))
	
	// Create input larger than max size
	largeData := strings.Repeat("a", 1025)
	reader := strings.NewReader(largeData)
	
	_, err := provider.SHA256Sign(reader)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "input too large")
}

func TestSHA256VerifyLargeInput(t *testing.T) {
	provider := createTestHMACProvider(t, WithMaxInputSize(1024))
	
	// Create input larger than max size
	largeData := strings.Repeat("a", 1025)
	reader := strings.NewReader(largeData)
	
	valid, err := provider.SHA256Verify(reader, "deadbeef")
	assert.Error(t, err)
	assert.False(t, valid)
	assert.Contains(t, err.Error(), "input too large") // Should fail on input size first
}

func TestSign256(t *testing.T) {
	provider := createTestHMACProvider(t)
	
	testData := "test data"
	reader := strings.NewReader(testData)
	
	hash, timestamp, nonce, err := provider.Sign256(reader)
	
	assert.NoError(t, err)
	assert.NotEmpty(t, hash)
	assert.NotEmpty(t, timestamp)
	assert.NotEmpty(t, nonce)
	
	// Verify hash is valid hex
	_, err = hex.DecodeString(hash)
	assert.NoError(t, err)
	
	// Verify timestamp is valid RFC3339
	_, err = time.Parse(time.RFC3339, timestamp)
	assert.NoError(t, err)
	
	// Verify nonce is UUID format (36 characters)
	assert.Equal(t, 36, len(nonce))
}

func TestSign256LargeInput(t *testing.T) {
	provider := createTestHMACProvider(t, WithMaxInputSize(1024))
	
	largeData := strings.Repeat("a", 1025)
	reader := strings.NewReader(largeData)
	
	_, _, _, err := provider.Sign256(reader)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "input too large")
}

func TestVerifyTimestamp(t *testing.T) {
	provider := createTestHMACProvider(t, WithKeyInterval(5*time.Minute))
	
	now := time.Now().UTC()
	
	tests := []struct {
		name      string
		timestamp string
		expected  bool
	}{
		{
			name:      "current time",
			timestamp: now.Format(time.RFC3339),
			expected:  true,
		},
		{
			name:      "4 minutes ago",
			timestamp: now.Add(-4 * time.Minute).Format(time.RFC3339),
			expected:  true,
		},
		{
			name:      "4 minutes future",
			timestamp: now.Add(4 * time.Minute).Format(time.RFC3339),
			expected:  true,
		},
		{
			name:      "6 minutes ago",
			timestamp: now.Add(-6 * time.Minute).Format(time.RFC3339),
			expected:  false,
		},
		{
			name:      "6 minutes future",
			timestamp: now.Add(6 * time.Minute).Format(time.RFC3339),
			expected:  false,
		},
		{
			name:      "invalid format",
			timestamp: "not-a-timestamp",
			expected:  false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := provider.verifyTimestamp(tt.timestamp)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestVerify256Success(t *testing.T) {
	mockStore := newMockNonceStore()
	provider := createTestHMACProvider(t, WithNonceStore(mockStore))
	
	testData := "test data"
	
	// Generate signature
	hash, timestamp, nonce, err := provider.Sign256(strings.NewReader(testData))
	require.NoError(t, err)
	
	// Verify signature
	valid, err := provider.Verify256(strings.NewReader(testData), hash, timestamp, nonce)
	assert.NoError(t, err)
	assert.True(t, valid)
	
	// Verify nonce was consumed
	assert.True(t, mockStore.nonces[nonce])
}

func TestVerify256InvalidParameters(t *testing.T) {
	provider := createTestHMACProvider(t)
	
	testData := "test data"
	
	tests := []struct {
		name      string
		hash      string
		timestamp string
		nonce     string
	}{
		{"empty hash", "", "2023-01-01T00:00:00Z", "nonce"},
		{"empty timestamp", "hash", "", "nonce"},
		{"empty nonce", "hash", "2023-01-01T00:00:00Z", ""},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid, err := provider.Verify256(strings.NewReader(testData), tt.hash, tt.timestamp, tt.nonce)
			assert.Error(t, err)
			assert.False(t, valid)
			assert.Contains(t, err.Error(), "invalid request")
		})
	}
}

func TestVerify256InvalidTimestamp(t *testing.T) {
	provider := createTestHMACProvider(t, WithKeyInterval(1*time.Minute))
	
	testData := "test data"
	
	// Use old timestamp
	oldTimestamp := time.Now().Add(-2 * time.Minute).Format(time.RFC3339)
	
	valid, err := provider.Verify256(strings.NewReader(testData), "deadbeef", oldTimestamp, "nonce")
	assert.Error(t, err)
	assert.False(t, valid)
	assert.Contains(t, err.Error(), "invalid request")
}

func TestVerify256InvalidHash(t *testing.T) {
	provider := createTestHMACProvider(t)
	
	testData := "test data"
	timestamp := time.Now().Format(time.RFC3339)
	
	valid, err := provider.Verify256(strings.NewReader(testData), "not-hex", timestamp, "nonce")
	assert.Error(t, err)
	assert.False(t, valid)
	assert.Contains(t, err.Error(), "invalid request")
}

func TestVerify256WrongSignature(t *testing.T) {
	mockStore := newMockNonceStore()
	provider := createTestHMACProvider(t, WithNonceStore(mockStore))
	
	testData := "test data"
	
	// Generate signature for different data
	hash, timestamp, nonce, err := provider.Sign256(strings.NewReader("different data"))
	require.NoError(t, err)
	
	// Try to verify with original data
	valid, err := provider.Verify256(strings.NewReader(testData), hash, timestamp, nonce)
	assert.Error(t, err)
	assert.False(t, valid)
	assert.Contains(t, err.Error(), "invalid request")
	
	// Verify nonce was NOT consumed
	assert.False(t, mockStore.nonces[nonce])
}

func TestVerify256ReplayAttack(t *testing.T) {
	mockStore := newMockNonceStore()
	provider := createTestHMACProvider(t, WithNonceStore(mockStore))
	
	testData := "test data"
	
	// Generate signature
	hash, timestamp, nonce, err := provider.Sign256(strings.NewReader(testData))
	require.NoError(t, err)
	
	// First verification should succeed
	valid, err := provider.Verify256(strings.NewReader(testData), hash, timestamp, nonce)
	assert.NoError(t, err)
	assert.True(t, valid)
	
	// Second verification should fail (replay attack)
	valid, err = provider.Verify256(strings.NewReader(testData), hash, timestamp, nonce)
	assert.Error(t, err)
	assert.False(t, valid)
	assert.Contains(t, err.Error(), "invalid request")
}

func TestVerify256NonceStoreFailure(t *testing.T) {
	mockStore := newMockNonceStore()
	mockStore.setFail(true) // Simulate nonce store failure
	provider := createTestHMACProvider(t, WithNonceStore(mockStore))
	
	testData := "test data"
	
	// Generate signature with working store
	mockStore.setFail(false)
	hash, timestamp, nonce, err := provider.Sign256(strings.NewReader(testData))
	require.NoError(t, err)
	
	// Fail the store for verification
	mockStore.setFail(true)
	
	valid, err := provider.Verify256(strings.NewReader(testData), hash, timestamp, nonce)
	assert.Error(t, err)
	assert.False(t, valid)
	assert.Contains(t, err.Error(), "invalid request")
}

func TestVerify256LargeInput(t *testing.T) {
	provider := createTestHMACProvider(t, WithMaxInputSize(1024))
	
	largeData := strings.Repeat("a", 1025)
	timestamp := time.Now().Format(time.RFC3339)
	
	valid, err := provider.Verify256(strings.NewReader(largeData), "deadbeef", timestamp, "nonce")
	assert.Error(t, err)
	assert.False(t, valid)
	assert.Contains(t, err.Error(), "invalid request")
}

func TestVerify256OperationOrder(t *testing.T) {
	mockStore := newMockNonceStore()
	provider := createTestHMACProvider(t, WithNonceStore(mockStore), WithKeyInterval(1*time.Minute))
	
	testData := "test data"
	
	// Use invalid timestamp (should fail before nonce is consumed)
	oldTimestamp := time.Now().Add(-2 * time.Minute).Format(time.RFC3339)
	validHex := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	
	valid, err := provider.Verify256(strings.NewReader(testData), validHex, oldTimestamp, "test-nonce")
	assert.Error(t, err)
	assert.False(t, valid)
	
	// Verify nonce was NOT consumed (operation order is correct)
	assert.False(t, mockStore.nonces["test-nonce"])
}

// Benchmark tests
func BenchmarkSHA256Sign(b *testing.B) {
	provider := createTestHMACProvider(&testing.T{})
	testData := "benchmark test data"
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := provider.SHA256Sign(strings.NewReader(testData))
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSHA256Verify(b *testing.B) {
	provider := createTestHMACProvider(&testing.T{})
	testData := "benchmark test data"
	
	hash, err := provider.SHA256Sign(strings.NewReader(testData))
	if err != nil {
		b.Fatal(err)
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := provider.SHA256Verify(strings.NewReader(testData), hash)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSign256(b *testing.B) {
	provider := createTestHMACProvider(&testing.T{})
	testData := "benchmark test data"
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _, err := provider.Sign256(strings.NewReader(testData))
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkVerify256(b *testing.B) {
	provider := createTestHMACProvider(&testing.T{})
	testData := "benchmark test data"
	
	// Pre-generate signatures
	signatures := make([]struct{ hash, timestamp, nonce string }, b.N)
	for i := 0; i < b.N; i++ {
		hash, timestamp, nonce, err := provider.Sign256(strings.NewReader(testData))
		if err != nil {
			b.Fatal(err)
		}
		signatures[i] = struct{ hash, timestamp, nonce string }{hash, timestamp, nonce}
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sig := signatures[i]
		_, err := provider.Verify256(strings.NewReader(testData), sig.hash, sig.timestamp, sig.nonce)
		if err != nil {
			b.Fatal(err)
		}
	}
}