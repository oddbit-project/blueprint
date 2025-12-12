package pin

import (
	"crypto/rand"
	"crypto/subtle"
	"errors"
	"math/big"
	"strings"
)

const (
	numericCharset      = "0123456789"
	alphanumericCharset = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"
)

const groupSize = 3

var (
	ErrInvalidLength = errors.New("pin length must be greater than 0")
)

// GenerateNumeric generates a cryptographically secure numeric PIN of the specified length.
func GenerateNumeric(length int) (string, error) {
	return generate(length, numericCharset)
}

// GenerateAlphanumeric generates a cryptographically secure alphanumeric PIN of the specified length.
// The generated PIN contains uppercase letters and digits.
func GenerateAlphanumeric(length int) (string, error) {
	return generate(length, alphanumericCharset)
}

// generate creates a cryptographically secure random string from the given charset.
func generate(length int, charset string) (string, error) {
	if length <= 0 {
		return "", ErrInvalidLength
	}

	result := make([]byte, length)
	charsetLen := big.NewInt(int64(len(charset)))

	for i := 0; i < length; i++ {
		idx, err := rand.Int(rand.Reader, charsetLen)
		if err != nil {
			return "", err
		}
		result[i] = charset[idx.Int64()]
	}

	return formatWithDashes(string(result)), nil
}

// formatWithDashes inserts dashes every 3 characters for readability.
// Example: "123456789" becomes "123-456-789"
func formatWithDashes(s string) string {
	if len(s) <= groupSize {
		return s
	}

	var builder strings.Builder
	for i, char := range s {
		if i > 0 && i%groupSize == 0 {
			builder.WriteByte('-')
		}
		builder.WriteRune(char)
	}
	return builder.String()
}

// stripDashes removes all dashes from a PIN for comparison.
func stripDashes(s string) string {
	return strings.ReplaceAll(s, "-", "")
}

// CompareNumeric performs a constant-time comparison of two numeric PINs.
// Dashes are stripped before comparison, so "123-456" matches "123456".
// Returns true if they match, false otherwise.
func CompareNumeric(pin1, pin2 string) bool {
	s1 := stripDashes(pin1)
	s2 := stripDashes(pin2)
	return subtle.ConstantTimeCompare([]byte(s1), []byte(s2)) == 1
}

// CompareAlphanumeric performs a constant-time, case-insensitive comparison of two alphanumeric PINs.
// Dashes are stripped before comparison, so "ABC-123" matches "abc123".
// Returns true if they match, false otherwise.
func CompareAlphanumeric(pin1, pin2 string) bool {
	// Strip dashes and convert to uppercase for case-insensitive comparison
	upper1 := strings.ToUpper(stripDashes(pin1))
	upper2 := strings.ToUpper(stripDashes(pin2))
	return subtle.ConstantTimeCompare([]byte(upper1), []byte(upper2)) == 1
}
