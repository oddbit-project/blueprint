package hmacprovider

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/oddbit-project/blueprint/utils"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/oddbit-project/blueprint/provider/hmacprovider/store"
)

const (
	ErrInvalidKeyId      = utils.Error("invalid key id")
	ErrInvalidHashFormat = utils.Error("invalid hash format")
	ErrInvalidRequest    = utils.Error("invalid request")

	DefaultKeyInterval = 5 * time.Minute
	MaxInputSize       = 32 * 1024 * 1024 // 32MB
)

type HMACProvider struct {
	secretProvider HMACKeyProvider
	nonceStore     store.NonceStore // nonce storage (defaults to memory)
	interval       time.Duration    // allowed timestamp deviation into the past or the future
	maxInputSize   int
}

type HMACProviderOption func(*HMACProvider)

func WithNonceStore(nonceStore store.NonceStore) HMACProviderOption {
	return func(hp *HMACProvider) {
		hp.nonceStore = nonceStore
	}
}
func WithKeyInterval(interval time.Duration) HMACProviderOption {
	return func(hp *HMACProvider) {
		hp.interval = interval
	}
}

func WithMaxInputSize(maxInputSize int) HMACProviderOption {
	return func(hp *HMACProvider) {
		hp.maxInputSize = maxInputSize
	}
}

func NewHmacProvider(secretProvider HMACKeyProvider, opts ...HMACProviderOption) *HMACProvider {
	result := &HMACProvider{
		secretProvider: secretProvider,
		nonceStore:     nil,
		interval:       DefaultKeyInterval,
		maxInputSize:   MaxInputSize,
	}
	for _, opt := range opts {
		opt(result)
	}

	// if no nounce storage, use mem
	if result.nonceStore == nil {
		result.nonceStore = store.NewMemoryNonceStore()
	}
	return result
}

// SHA256Sign generate a simple SHA256 HMAC, no nounce, no timestamp
func (h *HMACProvider) SHA256Sign(keyId string, data io.Reader) (string, error) {
	secret, err := h.secretProvider.FetchSecret(keyId)
	if err != nil {
		return "", err
	}
	if secret == nil {
		return "", ErrInvalidKeyId
	}

	// Limit input size to prevent DoS
	limitedReader := io.LimitReader(data, int64(h.maxInputSize))
	content, err := io.ReadAll(limitedReader)
	if err != nil {
		return "", err
	}

	// Check if we hit the limit
	if len(content) == h.maxInputSize {
		// Try to read one more byte to see if there's more data
		if _, err := data.Read(make([]byte, 1)); err != io.EOF {
			return "", errors.New("input too large")
		}
	}

	key, err := secret.GetBytes()
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, key)
	mac.Write(content)
	hash := hex.EncodeToString(mac.Sum(nil))
	if keyId != "" {
		return fmt.Sprintf("%s.%s", keyId, hash), nil
	}
	return hash, nil
}

// SHA256Verify verify a simple SHA256 HMAC, no nounce, no timestamp
// the hash must be a hex-encoded sha256 hash
// returns the keyId (if any), true if is valid, and an optional error status
func (h *HMACProvider) SHA256Verify(data io.Reader, hash string) (string, bool, error) {
	parts := strings.Split(hash, ".")
	if len(parts) > 2 {
		return "", false, ErrInvalidHashFormat
	}
	keyId := ""
	if len(parts) == 2 {
		keyId = parts[0]
		hash = parts[1]
	}
	secret, err := h.secretProvider.FetchSecret(keyId)
	if err != nil {
		return "", false, err
	}
	if secret == nil {
		return "", false, ErrInvalidKeyId
	}

	// Decode hex string first to prevent timing attacks
	providedMAC, err := hex.DecodeString(hash)
	if err != nil {
		return "", false, ErrInvalidHashFormat
	}

	// Limit input size to prevent DoS
	limitedReader := io.LimitReader(data, int64(h.maxInputSize))
	content, err := io.ReadAll(limitedReader)
	if err != nil {
		return "", false, err
	}

	// Check if we hit the limit
	if len(content) == h.maxInputSize {
		// Try to read one more byte to see if there's more data
		if _, err := data.Read(make([]byte, 1)); err != io.EOF {
			return "", false, errors.New("input too large")
		}
	}

	var key []byte
	key, err = secret.GetBytes()
	if err != nil {
		return "", false, err
	}
	mac := hmac.New(sha256.New, key)
	mac.Write(content)
	expectedMAC := mac.Sum(nil)

	// Constant-time comparison
	return keyId, hmac.Equal(expectedMAC, providedMAC), nil
}

// Sign256 generates a HMAC256 signature using timestamp and nonce
func (h *HMACProvider) Sign256(keyId string, data io.Reader) (hash string, timestamp string, nonce string, err error) {
	secret, err := h.secretProvider.FetchSecret(keyId)
	if err != nil {
		return "", "", "", err
	}
	if secret == nil {
		return "", "", "", ErrInvalidKeyId
	}

	timestamp = time.Now().UTC().Format(time.RFC3339)
	nonce = uuid.New().String()

	// Limit input size to prevent DoS
	limitedReader := io.LimitReader(data, int64(h.maxInputSize))
	content, err := io.ReadAll(limitedReader)
	if err != nil {
		return
	}

	// Check if we hit the limit
	if len(content) == h.maxInputSize {
		// Try to read one more byte to see if there's more data
		if _, err1 := data.Read(make([]byte, 1)); err1 != io.EOF {
			err = errors.New("input too large")
			return
		}
	}

	key, err := secret.GetBytes()
	if err != nil {
		return
	}

	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(timestamp))
	mac.Write([]byte(":"))
	mac.Write([]byte(nonce))
	mac.Write([]byte(":"))
	mac.Write(content)
	hash = hex.EncodeToString(mac.Sum(nil))
	if keyId != "" {
		hash = fmt.Sprintf("%s.%s", keyId, hash)
	}

	return hash, timestamp, nonce, nil
}

func (h *HMACProvider) verifyTimestamp(ts string) bool {
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return false
	}
	now := time.Now().UTC()
	diff := now.Sub(t)
	return diff < h.interval && diff > -h.interval
}

// Verify256 verifies a HMAC256 signature using timestamp and nonce
// Returns keyId(if any), true if success, and an optional error code
func (h *HMACProvider) Verify256(data io.Reader, hash string, timestamp string, nonce string) (string, bool, error) {
	// Validate inputs first
	if hash == "" || timestamp == "" || nonce == "" {
		return "", false, errors.New("invalid request")
	}

	parts := strings.Split(hash, ".")
	if len(parts) > 2 {
		return "", false, ErrInvalidHashFormat
	}
	keyId := ""
	if len(parts) == 2 {
		keyId = parts[0]
		hash = parts[1]
	}
	secret, err := h.secretProvider.FetchSecret(keyId)
	if err != nil {
		return "", false, err
	}
	if secret == nil {
		return "", false, ErrInvalidKeyId
	}

	// Check timestamp BEFORE consuming nonce
	if !h.verifyTimestamp(timestamp) {
		return "", false, ErrInvalidRequest
	}

	// Decode hash to prevent timing attacks
	providedMAC, err := hex.DecodeString(hash)
	if err != nil {
		return "", false, ErrInvalidRequest
	}

	// Limit input size to prevent DoS
	limitedReader := io.LimitReader(data, int64(h.maxInputSize))
	content, err := io.ReadAll(limitedReader)
	if err != nil {
		return "", false, err
	}

	// Check if we hit the limit
	if len(content) == h.maxInputSize {
		// Try to read one more byte to see if there's more data
		if _, err := data.Read(make([]byte, 1)); err != io.EOF {
			return "", false, ErrInvalidRequest
		}
	}

	// Compute HMAC
	key, err := secret.GetBytes()
	if err != nil {
		return "", false, err
	}

	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(timestamp))
	mac.Write([]byte(":"))
	mac.Write([]byte(nonce))
	mac.Write([]byte(":"))
	mac.Write(content)
	expectedMAC := mac.Sum(nil)

	// Verify HMAC first
	if !hmac.Equal(expectedMAC, providedMAC) {
		return "", false, ErrInvalidRequest
	}

	// Only consume nonce after successful validation
	if h.nonceStore != nil {
		if !h.nonceStore.AddIfNotExists(nonce) {
			return "", false, ErrInvalidRequest
		}
	}

	return keyId, true, nil
}
