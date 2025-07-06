package secure

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"github.com/oddbit-project/blueprint/utils/env"
	"github.com/oddbit-project/blueprint/utils/fs"
	"io"
	"sync"
)

type CredentialConfig interface {
	Fetch() (string, error)
	IsEmpty() bool
}

var (
	ErrEncryption          = errors.New("encryption error")
	ErrDecryption          = errors.New("decryption error")
	ErrInvalidKey          = errors.New("invalid encryption key")
	ErrEmptyCredential     = errors.New("empty credential")
	ErrSecretsFileNotFound = errors.New("secrets file not found")
)

type Secret interface {
	GetBytes() ([]byte, error)
}

// Credential stores sensitive information (like passwords)
// in encrypted form in memory
type Credential struct {
	empty         bool
	encryptedData []byte
	nonce         []byte
	key           []byte
	mu            sync.RWMutex
}

// NewCredential creates a new secure credential container
// The encryption key should be unique per application instance
// You can use env variables, hardware tokens, etc. as the source
// of the encryption key
func NewCredential(data []byte, encryptionKey []byte, allowEmpty bool) (*Credential, error) {
	if len(data) == 0 {
		if allowEmpty {
			return &Credential{
				empty: true,
			}, nil
		}
		return nil, ErrEmptyCredential
	}
	if len(encryptionKey) != 32 {
		return nil, ErrInvalidKey
	}

	sc := &Credential{
		key: make([]byte, len(encryptionKey)),
	}

	// Copy the key to avoid using the original reference
	copy(sc.key, encryptionKey)

	// Encrypt the credential
	var err error
	sc.encryptedData, sc.nonce, err = encrypt(data, sc.key)
	if err != nil {
		return nil, err
	}

	return sc, nil
}

// Get decrypts and returns the plaintext credential
func (sc *Credential) Get() (string, error) {
	buf, err := sc.GetBytes()
	if err != nil {
		return "", err
	}
	if len(buf) == 0 {
		return "", nil
	}
	return string(buf), nil
}

// GetBytes decrypts and returns the raw credential
// This should be called only when needed to minimize
// exposure of the sensitive data in memory
func (sc *Credential) GetBytes() ([]byte, error) {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	if sc.empty {
		return nil, nil
	}
	if sc.encryptedData == nil || sc.nonce == nil {
		return nil, ErrEmptyCredential
	}

	buf, err := decrypt(sc.encryptedData, sc.nonce, sc.key)
	if err != nil {
		return nil, err
	}

	return buf, nil
}

// Update updates the credential with a new plaintext value
func (sc *Credential) Update(plaintext string) error {
	return sc.UpdateBytes([]byte(plaintext))
}

// UpdateBytes updates the credential with a new value
func (sc *Credential) UpdateBytes(data []byte) error {
	if len(data) == 0 {
		sc.Clear()
		return nil
	}

	sc.mu.Lock()
	defer sc.mu.Unlock()

	var err error
	sc.encryptedData, sc.nonce, err = encrypt(data, sc.key)
	return err
}

// Clear zeroes out all sensitive data
func (sc *Credential) Clear() {
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
	sc.empty = true
}

// IsEmpty returns true if credentials is empty
func (sc *Credential) IsEmpty() bool {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	return sc.empty
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

// CredentialFromEnv creates a Credential from an environment variable
func CredentialFromEnv(envName string, encryptionKey []byte, allowEmpty bool) (*Credential, error) {
	value := env.GetEnvVar(envName)
	if value == "" {
		return nil, ErrEmptyCredential
	}

	return NewCredential([]byte(value), encryptionKey, allowEmpty)
}

// CredentialFromFile creates a Credential from a secrets file
func CredentialFromFile(filename string, encryptionKey []byte, allowEmpty bool) (*Credential, error) {
	if !fs.FileExists(filename) {
		return nil, ErrSecretsFileNotFound
	}
	value, err := fs.ReadString(filename)
	if err != nil {
		return nil, ErrSecretsFileNotFound
	}

	if value == "" {
		return nil, ErrEmptyCredential
	}

	return NewCredential([]byte(value), encryptionKey, allowEmpty)
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

// CredentialFromConfig attempts to parse credentials from a CredentialConfig struct
// if no valid credentials found, returns error; if environment var is used, it is read only once and
// then overwritten with an empty value
func CredentialFromConfig(cfg CredentialConfig, encryptionKey []byte, allowEmpty bool) (*Credential, error) {
	cred, err := cfg.Fetch()
	if err != nil {
		return nil, err
	}
	if len(cred) > 0 || (allowEmpty && len(cred) == 0) {
		return NewCredential([]byte(cred), encryptionKey, allowEmpty)
	}
	return nil, ErrEmptyCredential
}

// RandomKey32 generate a random key
func RandomKey32() []byte {
	key := make([]byte, 32)
	_, err := io.ReadFull(rand.Reader, key)
	if err != nil {
		panic(err)
	}
	return key
}

// RandomCredential create a secure credential using random bytes
func RandomCredential(l int) (*Credential, error) {
	secret := make([]byte, l)
	_, err := io.ReadFull(rand.Reader, secret)
	if err != nil {
		panic(err)
	}
	return NewCredential(secret, RandomKey32(), false)
}
