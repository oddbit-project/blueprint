package secure

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"io"
	"testing"
)

// Original non-constant-time implementation for comparison
type originalAES256GCM struct {
	key []byte
}

func newOriginalAES256GCM(key []byte) *originalAES256GCM {
	result := &originalAES256GCM{
		key: make([]byte, len(key)),
	}
	copy(result.key, key)
	return result
}

func (a *originalAES256GCM) encrypt(data []byte) ([]byte, error) {
	if len(a.key) != 32 {
		return nil, ErrInvalidKeyLength
	}

	block, err := aes.NewCipher(a.key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	encrypted := gcm.Seal(nil, nonce, data, nil)
	result := append(nonce, encrypted...)
	return result, nil
}

func (a *originalAES256GCM) decrypt(data []byte) ([]byte, error) {
	if len(a.key) != 32 {
		return nil, ErrInvalidKeyLength
	}

	block, err := aes.NewCipher(a.key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(data) <= nonceSize {
		return nil, ErrDataTooShort
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}

// Benchmarks
func BenchmarkAES256GCM_Encrypt_Original(b *testing.B) {
	key := make([]byte, 32)
	rand.Read(key)

	gcm := newOriginalAES256GCM(key)
	data := make([]byte, 1024) // 1KB test data
	rand.Read(data)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := gcm.encrypt(data)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkAES256GCM_Encrypt_ConstantTime(b *testing.B) {
	key := make([]byte, 32)
	rand.Read(key)

	gcm, err := NewAES256GCM(key)
	if err != nil {
		b.Fatal(err)
	}
	defer gcm.Clear()

	data := make([]byte, 1024) // 1KB test data
	rand.Read(data)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := gcm.Encrypt(data)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkAES256GCM_Decrypt_Original(b *testing.B) {
	key := make([]byte, 32)
	rand.Read(key)

	gcm := newOriginalAES256GCM(key)
	data := make([]byte, 1024)
	rand.Read(data)

	// Pre-encrypt data for decryption benchmark
	encrypted, err := gcm.encrypt(data)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := gcm.decrypt(encrypted)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkAES256GCM_Decrypt_ConstantTime(b *testing.B) {
	key := make([]byte, 32)
	rand.Read(key)

	gcm, err := NewAES256GCM(key)
	if err != nil {
		b.Fatal(err)
	}
	defer gcm.Clear()

	data := make([]byte, 1024)
	rand.Read(data)

	// Pre-encrypt data for decryption benchmark
	encrypted, err := gcm.Encrypt(data)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := gcm.Decrypt(encrypted)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Benchmark individual constant-time operations
func BenchmarkConstantTimeKeyLengthCheck(b *testing.B) {
	key := make([]byte, 32)
	rand.Read(key)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		constantTimeKeyLengthCheck(key)
	}
}

func BenchmarkConstantTimeMinLength(b *testing.B) {
	data := make([]byte, 1024)
	rand.Read(data)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		constantTimeMinLength(data, 28)
	}
}

func BenchmarkExtractNonceConstantTime(b *testing.B) {
	data := make([]byte, 1024)
	rand.Read(data)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		extractNonceConstantTime(data, 12)
	}
}

func BenchmarkConstantTimeLessOrEq(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		constantTimeLessOrEq(i, 1000000)
	}
}

// Memory allocation benchmarks
func BenchmarkAES256GCM_KeyCreation_Original(b *testing.B) {
	key := make([]byte, 32)
	rand.Read(key)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		gcm := newOriginalAES256GCM(key)
		_ = gcm
	}
}

func BenchmarkAES256GCM_KeyCreation_ConstantTime(b *testing.B) {
	key := make([]byte, 32)
	rand.Read(key)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		gcm, err := NewAES256GCM(key)
		if err != nil {
			b.Fatal(err)
		}
		gcm.Clear()
	}
}

// Different data sizes
func BenchmarkAES256GCM_Encrypt_Small_ConstantTime(b *testing.B) {
	key := make([]byte, 32)
	rand.Read(key)

	gcm, err := NewAES256GCM(key)
	if err != nil {
		b.Fatal(err)
	}
	defer gcm.Clear()

	data := make([]byte, 64) // Small data
	rand.Read(data)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := gcm.Encrypt(data)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkAES256GCM_Encrypt_Large_ConstantTime(b *testing.B) {
	key := make([]byte, 32)
	rand.Read(key)

	gcm, err := NewAES256GCM(key)
	if err != nil {
		b.Fatal(err)
	}
	defer gcm.Clear()

	data := make([]byte, 1024*1024) // 1MB data
	rand.Read(data)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := gcm.Encrypt(data)
		if err != nil {
			b.Fatal(err)
		}
	}
}
