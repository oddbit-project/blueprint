package secure

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"sync"
)

var (
	ErrEncryption      = errors.New("encryption error")
	ErrDecryption      = errors.New("decryption error")
	ErrInvalidKey      = errors.New("invalid encryption key")
	ErrEmptyCredential = errors.New("empty credential")
)

// SecureCredential stores sensitive information (like passwords)
// in encrypted form in memory
type SecureCredential struct {
	encryptedData []byte
	nonce         []byte
	key           []byte
	mu            sync.RWMutex
}

// NewSecureCredential creates a new secure credential container
// The encryption key should be unique per application instance
// You can use env variables, hardware tokens, etc. as the source
// of the encryption key
func NewSecureCredential(plaintext string, encryptionKey []byte) (*SecureCredential, error) {
	if len(encryptionKey) != 32 {
		return nil, ErrInvalidKey
	}
	
	if plaintext == "" {
		return nil, ErrEmptyCredential
	}
	
	sc := &SecureCredential{
		key: make([]byte, len(encryptionKey)),
	}
	
	// Copy the key to avoid using the original reference
	copy(sc.key, encryptionKey)
	
	// Encrypt the credential
	var err error
	sc.encryptedData, sc.nonce, err = encrypt([]byte(plaintext), sc.key)
	if err != nil {
		return nil, err
	}
	
	return sc, nil
}

// Get decrypts and returns the plaintext credential
// This should be called only when needed to minimize
// exposure of the sensitive data in memory
func (sc *SecureCredential) Get() (string, error) {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	
	if sc.encryptedData == nil || sc.nonce == nil {
		return "", ErrEmptyCredential
	}
	
	plaintext, err := decrypt(sc.encryptedData, sc.nonce, sc.key)
	if err != nil {
		return "", err
	}
	
	return string(plaintext), nil
}

// Update updates the credential with a new plaintext value
func (sc *SecureCredential) Update(plaintext string) error {
	if plaintext == "" {
		return ErrEmptyCredential
	}
	
	sc.mu.Lock()
	defer sc.mu.Unlock()
	
	var err error
	sc.encryptedData, sc.nonce, err = encrypt([]byte(plaintext), sc.key)
	return err
}

// Clear zeroes out all sensitive data
func (sc *SecureCredential) Clear() {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	
	if sc.encryptedData != nil {
		for i := range sc.encryptedData {
			sc.encryptedData[i] = 0
		}
		sc.encryptedData = nil
	}
	
	if sc.nonce != nil {
		for i := range sc.nonce {
			sc.nonce[i] = 0
		}
		sc.nonce = nil
	}
	
	if sc.key != nil {
		for i := range sc.key {
			sc.key[i] = 0
		}
		sc.key = nil
	}
}

// encrypt encrypts plaintext using AES-GCM with the provided key
// and returns the ciphertext and nonce
func encrypt(plaintext, key []byte) (ciphertext, nonce []byte, err error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, ErrEncryption
	}
	
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, ErrEncryption
	}
	
	nonce = make([]byte, aesGCM.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, ErrEncryption
	}
	
	ciphertext = aesGCM.Seal(nil, nonce, plaintext, nil)
	return ciphertext, nonce, nil
}

// decrypt decrypts ciphertext using AES-GCM with the provided key and nonce
func decrypt(ciphertext, nonce, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, ErrDecryption
	}
	
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, ErrDecryption
	}
	
	plaintext, err := aesGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, ErrDecryption
	}
	
	return plaintext, nil
}

// FromEnv creates a SecureCredential from an environment variable
func FromEnv(envName string, encryptionKey []byte) (*SecureCredential, error) {
	value := GetEnvVar(envName)
	if value == "" {
		return nil, ErrEmptyCredential
	}
	
	return NewSecureCredential(value, encryptionKey)
}

// GenerateKey generates a random 32-byte key for AES-256
func GenerateKey() ([]byte, error) {
	key := make([]byte, 32)
	_, err := io.ReadFull(rand.Reader, key)
	if err != nil {
		return nil, err
	}
	return key, nil
}

// EncodeKey encodes a key as a base64 string for storage
func EncodeKey(key []byte) string {
	return base64.StdEncoding.EncodeToString(key)
}

// DecodeKey decodes a base64 encoded key
func DecodeKey(encodedKey string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(encodedKey)
}