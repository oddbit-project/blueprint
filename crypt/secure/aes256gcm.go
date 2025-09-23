package secure

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/subtle"
	"encoding/binary"
	"github.com/oddbit-project/blueprint/utils"
	"io"
	"math"
	"sync"
)

const (
	ErrInvalidKeyLength     = utils.Error("key length must be 32 bytes")
	ErrDataTooShort         = utils.Error("data too short")
	ErrNonceExhausted       = utils.Error("nonce counter exhausted, key rotation required")
	ErrAuthenticationFailed = utils.Error("authentication failed")
)

type AES256GCM interface {
	Encrypt(data []byte) ([]byte, error)
	Decrypt(data []byte) ([]byte, error)
	Clear()
}

type aes256Gcm struct {
	key     []byte
	counter uint64
	mu      sync.Mutex
}

// NewAES256GCM creates a AES256GCM object
func NewAES256GCM(key []byte) (AES256GCM, error) {
	// Constant-time key length validation
	if !constantTimeKeyLengthCheck(key) {
		return nil, ErrInvalidKeyLength
	}

	result := &aes256Gcm{
		key:     make([]byte, len(key)),
		counter: 0,
	}
	copy(result.key, key)
	return result, nil
}

// constantTimeKeyLengthCheck performs constant-time validation of key length
func constantTimeKeyLengthCheck(key []byte) bool {
	// Create a byte slice with expected length
	expectedLen := make([]byte, 1)
	expectedLen[0] = 32
	actualLen := make([]byte, 1)
	if len(key) <= 255 {
		actualLen[0] = byte(len(key))
	} else {
		actualLen[0] = 255 // cap at 255 for byte comparison
	}
	return subtle.ConstantTimeCompare(expectedLen, actualLen) == 1
}

// Clear performs constant-time clearing of sensitive key material
func (a *aes256Gcm) Clear() {
	if a.key != nil {
		// First pass: zero out the key
		for i := range a.key {
			a.key[i] = 0
		}

		// Second pass: use constant-time copy to prevent compiler optimization
		subtle.ConstantTimeCopy(1, a.key, make([]byte, len(a.key)))

		// Third pass: additional zeroing
		for i := range a.key {
			a.key[i] = 0
		}

		a.key = nil
	}
	// Clear counter
	a.counter = 0
}

// isCounterExhausted performs constant-time check for counter exhaustion
func (a *aes256Gcm) isCounterExhausted() bool {
	max := make([]byte, 8)
	binary.BigEndian.PutUint64(max, math.MaxUint64)
	current := make([]byte, 8)
	binary.BigEndian.PutUint64(current, a.counter)
	return subtle.ConstantTimeCompare(max, current) == 1
}

// constantTimeMinLength performs constant-time minimum length validation
func constantTimeMinLength(data []byte, minLen int) bool {
	// Ensure we don't leak length information through timing
	dataLen := len(data)

	// Convert lengths to byte slices for constant-time comparison
	minLenBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(minLenBytes, uint32(minLen))

	dataLenBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(dataLenBytes, uint32(dataLen))

	// Perform constant-time comparison: dataLen >= minLen
	// This is done by checking if dataLen - minLen >= 0
	diff := int32(dataLen) - int32(minLen)
	isValid := 1 - (int(diff>>31) & 1) // 1 if diff >= 0, 0 otherwise

	return isValid == 1
}

// extractNonceConstantTime performs constant-time nonce extraction
func extractNonceConstantTime(data []byte, nonceSize int) []byte {
	nonce := make([]byte, nonceSize)

	// Simple constant-time copy for the nonce
	// If data is shorter than nonceSize, remaining bytes stay zero
	for i := 0; i < nonceSize; i++ {
		if i < len(data) {
			nonce[i] = data[i]
		}
	}

	return nonce
}

// Encrypt encrypt data using AES256-GCM
func (a *aes256Gcm) Encrypt(data []byte) ([]byte, error) {
	// Constant-time key validation
	if !constantTimeKeyLengthCheck(a.key) {
		return nil, ErrInvalidKeyLength
	}

	block, err := aes.NewCipher(a.key)
	if err != nil {
		return nil, err
	}
	// GCM - https://en.wikipedia.org/wiki/Galois/Counter_Mode
	var gcm cipher.AEAD
	gcm, err = cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	// Lock to ensure thread-safe counter increment
	a.mu.Lock()
	defer a.mu.Unlock()

	// Constant-time counter exhaustion check
	if a.isCounterExhausted() {
		return nil, ErrNonceExhausted
	}

	// Create nonce from counter
	nonce := make([]byte, gcm.NonceSize())
	// Use first 4 bytes for random prefix to reduce correlation
	if _, err = io.ReadFull(rand.Reader, nonce[:4]); err != nil {
		return nil, err
	}
	// Use remaining 8 bytes for counter
	binary.BigEndian.PutUint64(nonce[4:], a.counter)
	a.counter++

	// Seal encrypts and authenticates plaintext, authenticates the
	// additional data and appends the result to dst, returning the updated
	// slice. The nonce must be NonceSize() bytes long and unique for all
	// time, for a given key.
	encrypted := gcm.Seal(nil, nonce, data, nil)

	// Prepend nonce
	result := append(nonce, encrypted...)
	return result, nil
}

// Decrypt decrypt data using AES256-GCM
func (a *aes256Gcm) Decrypt(data []byte) ([]byte, error) {
	// Pre-allocate all possible errors for constant-time selection
	errors := []error{
		nil,                     // 0: success
		ErrInvalidKeyLength,     // 1: invalid key
		ErrDataTooShort,         // 2: data too short
		ErrAuthenticationFailed, // 3: authentication failed
	}

	errorIndex := 0
	var result []byte

	// Constant-time key validation
	keyValid := constantTimeKeyLengthCheck(a.key)
	keyValidInt := 0
	if keyValid {
		keyValidInt = 1
	}
	errorIndex = subtle.ConstantTimeSelect(keyValidInt, errorIndex, 1)

	// Early return for invalid key to avoid nil pointer
	if errorIndex != 0 {
		return nil, errors[errorIndex]
	}

	block, err := aes.NewCipher(a.key)
	if err != nil {
		return nil, err
	}

	var gcm cipher.AEAD
	gcm, err = cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	minLen := nonceSize + gcm.Overhead()

	// Constant-time length validation
	lengthValid := constantTimeMinLength(data, minLen)
	lengthValidInt := 0
	if lengthValid {
		lengthValidInt = 1
	}
	errorIndex = subtle.ConstantTimeSelect(lengthValidInt, errorIndex, 2)

	// Process data even if length is invalid (for constant time)
	// but ensure we have enough data to avoid panic
	if len(data) >= minLen {
		// Extract nonce using constant-time operation
		nonce := extractNonceConstantTime(data, nonceSize)
		ciphertext := data[nonceSize:]

		// Attempt decryption
		result, err = gcm.Open(nil, nonce, ciphertext, nil)
		if err != nil {
			successCondition := 0
			if errorIndex == 0 {
				successCondition = 1
			}
			errorIndex = subtle.ConstantTimeSelect(successCondition, 3, errorIndex)
		}
	} else {
		// Create dummy data to process for constant time
		dummyNonce := make([]byte, nonceSize)
		dummyCiphertext := make([]byte, gcm.Overhead())
		_, _ = gcm.Open(nil, dummyNonce, dummyCiphertext, nil)
		errorIndex = subtle.ConstantTimeSelect(lengthValidInt, errorIndex, 2)
	}

	// Return result based on error index
	if errorIndex != 0 {
		return nil, errors[errorIndex]
	}
	return result, nil
}

// constantTimeLessOrEq performs constant-time integer comparison (x <= y)
func constantTimeLessOrEq(x, y int) int {
	// Convert to int32 to avoid issues with bit shifting
	diff := int32(y) - int32(x)
	// If diff >= 0, then x <= y, return 1; otherwise return 0
	return 1 - int((diff>>31)&1)
}
