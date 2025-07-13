package hmacprovider

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"github.com/google/uuid"
	"github.com/oddbit-project/blueprint/crypt/secure"
	"github.com/oddbit-project/blueprint/provider/hmacprovider/store"
	"io"
	"time"
)

const (
	DefaultKeyInterval = 5 * time.Minute
	MaxInputSize       = 32 * 1024 * 1024 // 32MB
)

type HMACProvider struct {
	secret       *secure.Credential
	nonceStore   store.NonceStore // nonce storage (defaults to memory)
	interval     time.Duration    // allowed timestamp deviation into the past or the future
	maxInputSize int
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

func NewHmacProvider(secretKey *secure.Credential, opts ...HMACProviderOption) *HMACProvider {
	result := &HMACProvider{
		secret:       secretKey,
		nonceStore:   nil,
		interval:     DefaultKeyInterval,
		maxInputSize: MaxInputSize,
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
func (h *HMACProvider) SHA256Sign(data io.Reader) (string, error) {
	content, err := io.ReadAll(data)
	if err != nil {
		return "", err
	}
	secret, err := h.secret.GetBytes()
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, secret)
	mac.Write(content)
	hash := mac.Sum(nil)
	return hex.EncodeToString(hash), nil
}

// SHA256Verify verify a simple SHA256 HMAC, no nounce, no timestamp
// the hash must be a hex-encoded sha256 hash
func (h *HMACProvider) SHA256Verify(data io.Reader, hash string) (bool, error) {
	// Decode hex string first to prevent timing attacks
	providedMAC, err := hex.DecodeString(hash)
	if err != nil {
		return false, errors.New("invalid hash format")
	}

	// Limit input size to prevent DoS
	limitedReader := io.LimitReader(data, int64(h.maxInputSize))
	content, err := io.ReadAll(limitedReader)
	if err != nil {
		return false, err
	}

	// Check if we hit the limit
	if len(content) == h.maxInputSize {
		// Try to read one more byte to see if there's more data
		if _, err := data.Read(make([]byte, 1)); err != io.EOF {
			return false, errors.New("input too large")
		}
	}

	secret, err := h.secret.GetBytes()
	if err != nil {
		return false, err
	}
	mac := hmac.New(sha256.New, secret)
	mac.Write(content)
	expectedMAC := mac.Sum(nil)

	// Constant-time comparison
	return hmac.Equal(expectedMAC, providedMAC), nil
}

// Sign256 generates a HMAC256 signature using timestamp and nonce
func (h *HMACProvider) Sign256(data io.Reader) (hash string, timestamp string, nonce string, err error) {
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

	secret, err := h.secret.GetBytes()
	if err != nil {
		return
	}

	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(timestamp))
	mac.Write([]byte(":"))
	mac.Write([]byte(nonce))
	mac.Write([]byte(":"))
	mac.Write(content)
	hash = hex.EncodeToString(mac.Sum(nil))
	return
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
func (h *HMACProvider) Verify256(data io.Reader, hash string, timestamp string, nonce string) (bool, error) {
	// Validate inputs first
	if hash == "" || timestamp == "" || nonce == "" {
		return false, errors.New("invalid request")
	}

	// Check timestamp BEFORE consuming nonce
	if !h.verifyTimestamp(timestamp) {
		return false, errors.New("invalid request")
	}

	// Decode hash to prevent timing attacks
	providedMAC, err := hex.DecodeString(hash)
	if err != nil {
		return false, errors.New("invalid request")
	}

	// Limit input size to prevent DoS
	limitedReader := io.LimitReader(data, int64(h.maxInputSize))
	content, err := io.ReadAll(limitedReader)
	if err != nil {
		return false, err
	}

	// Check if we hit the limit
	if len(content) == h.maxInputSize {
		// Try to read one more byte to see if there's more data
		if _, err := data.Read(make([]byte, 1)); err != io.EOF {
			return false, errors.New("invalid request")
		}
	}

	// Compute HMAC
	secret, err := h.secret.GetBytes()
	if err != nil {
		return false, err
	}

	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(timestamp))
	mac.Write([]byte(":"))
	mac.Write([]byte(nonce))
	mac.Write([]byte(":"))
	mac.Write(content)
	expectedMAC := mac.Sum(nil)

	// Verify HMAC first
	if !hmac.Equal(expectedMAC, providedMAC) {
		return false, errors.New("invalid request")
	}

	// Only consume nonce after successful validation
	if h.nonceStore != nil {
		if !h.nonceStore.AddIfNotExists(nonce) {
			return false, errors.New("invalid request")
		}
	}

	return true, nil
}
