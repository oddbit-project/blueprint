package secure

import (
	"encoding/binary"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAES256GCM_BasicEncryptDecrypt(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	gcm, err := NewAES256GCM(key)
	require.NoError(t, err)
	defer gcm.Clear()

	plaintext := []byte("Hello, World!")
	ciphertext, err := gcm.Encrypt(plaintext)
	require.NoError(t, err)

	decrypted, err := gcm.Decrypt(ciphertext)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}

func TestAES256GCM_NonceUniqueness(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	gcm, err := NewAES256GCM(key)
	require.NoError(t, err)
	defer gcm.Clear()

	// Encrypt multiple times and check that nonces are unique
	nonces := make(map[string]bool)
	plaintext := []byte("Test message")

	for i := 0; i < 100; i++ {
		ciphertext, err := gcm.Encrypt(plaintext)
		require.NoError(t, err)

		// Extract nonce (first 12 bytes)
		nonce := ciphertext[:12]
		nonceStr := string(nonce)

		// Check that we haven't seen this nonce before
		assert.False(t, nonces[nonceStr], "Nonce reuse detected at iteration %d", i)
		nonces[nonceStr] = true

		// Verify counter is incrementing (last 8 bytes of nonce)
		counter := binary.BigEndian.Uint64(nonce[4:])
		assert.Equal(t, uint64(i), counter, "Counter mismatch at iteration %d", i)
	}
}

func TestAES256GCM_CounterExhaustion(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	gcm, err := NewAES256GCM(key)
	require.NoError(t, err)
	defer gcm.Clear()

	// Set counter to max value - 1
	impl := gcm.(*aes256Gcm)
	impl.counter = math.MaxUint64 - 1

	// This should succeed
	_, err = gcm.Encrypt([]byte("test"))
	require.NoError(t, err)

	// This should fail with ErrNonceExhausted
	_, err = gcm.Encrypt([]byte("test"))
	assert.ErrorIs(t, err, ErrNonceExhausted)
}

func TestAES256GCM_ThreadSafety(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	gcm, err := NewAES256GCM(key)
	require.NoError(t, err)
	defer gcm.Clear()

	// Run concurrent encryptions
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				_, err := gcm.Encrypt([]byte("concurrent test"))
				assert.NoError(t, err)
			}
			done <- true
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Check that counter is at expected value
	impl := gcm.(*aes256Gcm)
	assert.Equal(t, uint64(1000), impl.counter)
}

func TestAES256GCM_InvalidKeyLength(t *testing.T) {
	// Test with wrong key length
	shortKey := make([]byte, 16)
	_, err := NewAES256GCM(shortKey)
	assert.ErrorIs(t, err, ErrInvalidKeyLength)

	longKey := make([]byte, 64)
	_, err = NewAES256GCM(longKey)
	assert.ErrorIs(t, err, ErrInvalidKeyLength)
}

func TestAES256GCM_Clear(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	gcm, err := NewAES256GCM(key)
	require.NoError(t, err)

	impl := gcm.(*aes256Gcm)
	gcm.Clear()

	// Verify key is cleared
	assert.Nil(t, impl.key)
	assert.Equal(t, uint64(0), impl.counter)
}

func TestConstantTimeKeyLengthCheck(t *testing.T) {
	tests := []struct {
		name     string
		keyLen   int
		expected bool
	}{
		{"valid 32-byte key", 32, true},
		{"short 16-byte key", 16, false},
		{"long 64-byte key", 64, false},
		{"empty key", 0, false},
		{"oversized key", 512, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := make([]byte, tt.keyLen)
			result := constantTimeKeyLengthCheck(key)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConstantTimeMinLength(t *testing.T) {
	tests := []struct {
		name     string
		dataLen  int
		minLen   int
		expected bool
	}{
		{"exact length", 32, 32, true},
		{"longer data", 64, 32, true},
		{"shorter data", 16, 32, false},
		{"zero length", 0, 16, false},
		{"negative minLen", 16, -1, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := make([]byte, tt.dataLen)
			result := constantTimeMinLength(data, tt.minLen)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConstantTimeLessOrEq(t *testing.T) {
	tests := []struct {
		name     string
		x, y     int
		expected int
	}{
		{"x < y", 5, 10, 1},
		{"x = y", 10, 10, 1},
		{"x > y", 15, 10, 0},
		{"negative numbers", -5, 5, 1},
		{"both negative", -10, -5, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := constantTimeLessOrEq(tt.x, tt.y)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractNonceConstantTime(t *testing.T) {
	data := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}

	// Test normal case
	nonce := extractNonceConstantTime(data, 12)
	expected := data[:12]
	assert.Equal(t, expected, nonce)

	// Test short data case
	shortData := []byte{1, 2, 3}
	nonce = extractNonceConstantTime(shortData, 12)
	assert.Len(t, nonce, 12)
	// First 3 bytes should match
	assert.Equal(t, shortData[0], nonce[0])
	assert.Equal(t, shortData[1], nonce[1])
	assert.Equal(t, shortData[2], nonce[2])
}

func TestAES256GCM_ConstantTimeDecryptErrors(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	gcm, err := NewAES256GCM(key)
	require.NoError(t, err)
	defer gcm.Clear()

	// Test with data too short
	shortData := []byte{1, 2, 3}
	_, err = gcm.Decrypt(shortData)
	assert.ErrorIs(t, err, ErrDataTooShort)

	// Test with minimum length but invalid ciphertext
	minData := make([]byte, 28) // 12 (nonce) + 16 (minimum for GCM tag)
	_, err = gcm.Decrypt(minData)
	assert.ErrorIs(t, err, ErrAuthenticationFailed)
}
