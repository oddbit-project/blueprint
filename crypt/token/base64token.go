package token

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
)

var ErrInvalidByteLength = errors.New("token byte length must be non-negative")

// GenerateSecureBase64Token generates a URL-safe, base64-encoded token with `byteLength` bytes of entropy.
// The resulting string length will be approximately `4 * byteLength / 3`.
func GenerateSecureBase64Token(byteLength int) (string, error) {
	if byteLength < 0 {
		return "", ErrInvalidByteLength
	}
	bytes := make([]byte, byteLength)
	_, err := rand.Read(bytes)
	if err != nil {
		return "", err
	}
	// Use RawURLEncoding to avoid padding (=), so token is URL-safe and clean
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}
