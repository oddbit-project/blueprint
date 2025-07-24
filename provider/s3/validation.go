package s3

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

// S3 bucket bucketName validation rules
const (
	minBucketNameLength = 3
	maxBucketNameLength = 63
	minObjectKeyLength  = 1
	maxObjectKeyLength  = 1024
)

// Regular expressions for validation
var (
	// Bucket bucketName must be valid DNS bucketName: lowercase letters, numbers, hyphens, periods
	bucketNameRegex = regexp.MustCompile(`^[a-z0-9]([a-z0-9.-]*[a-z0-9])?$`)

	// IP address pattern to reject IP-formatted bucket names
	ipAddressRegex = regexp.MustCompile(`^\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}$`)

	// Invalid sequences in bucket names
	consecutiveDotsRegex = regexp.MustCompile(`\.\.`)
	dashNextToDotRegex   = regexp.MustCompile(`\.\-|\-\.`)
)

// ValidateBucketName validates an S3 bucket bucketName according to AWS rules
func ValidateBucketName(name string) error {
	if len(name) < minBucketNameLength || len(name) > maxBucketNameLength {
		return ErrInvalidBucketName
	}

	// Must contain only lowercase letters, numbers, hyphens, and periods
	if !bucketNameRegex.MatchString(name) {
		return ErrInvalidBucketName
	}

	// Must not be formatted as an IP address
	if ipAddressRegex.MatchString(name) {
		return ErrInvalidBucketName
	}

	// Must not contain consecutive periods
	if consecutiveDotsRegex.MatchString(name) {
		return ErrInvalidBucketName
	}

	// Must not have periods adjacent to hyphens
	if dashNextToDotRegex.MatchString(name) {
		return ErrInvalidBucketName
	}

	// Must not start or end with hyphen or period
	if strings.HasPrefix(name, "-") || strings.HasSuffix(name, "-") ||
		strings.HasPrefix(name, ".") || strings.HasSuffix(name, ".") {
		return ErrInvalidBucketName
	}

	// Additional AWS restrictions
	if strings.HasPrefix(name, "xn--") {
		return ErrInvalidBucketName
	}

	if strings.HasSuffix(name, "-s3alias") {
		return ErrInvalidBucketName
	}

	if strings.HasSuffix(name, "--ol-s3") {
		return ErrInvalidBucketName
	}

	return nil
}

// ValidateObjectName validates an S3 object key according to AWS rules
func ValidateObjectName(key string) error {
	if len(key) < minObjectKeyLength || len(key) > maxObjectKeyLength {
		return ErrInvalidObjectKey
	}

	// Check for valid UTF-8 encoding
	if !utf8.ValidString(key) {
		return ErrInvalidObjectKey
	}

	// Check for prohibited characters
	if containsProhibitedChars(key) {
		return ErrInvalidObjectKey
	}

	// Object keys should not start with slash (though AWS allows it, it's not recommended)
	// This is more of a best practice check
	if strings.HasPrefix(key, "/") {
		// We'll allow it but could log a warning in a real implementation
		// return ErrInvalidObjectKey
	}

	return nil
}

// containsProhibitedChars checks if the key contains characters that should be avoided
func containsProhibitedChars(key string) bool {
	// Characters that can cause issues in URLs or systems
	prohibited := []string{
		"\x00", "\x01", "\x02", "\x03", "\x04", "\x05", "\x06", "\x07",
		"\x08", "\x09", "\x0A", "\x0B", "\x0C", "\x0D", "\x0E", "\x0F",
		"\x10", "\x11", "\x12", "\x13", "\x14", "\x15", "\x16", "\x17",
		"\x18", "\x19", "\x1A", "\x1B", "\x1C", "\x1D", "\x1E", "\x1F",
		"\x7F",
	}

	for _, char := range prohibited {
		if strings.Contains(key, char) {
			return true
		}
	}

	return false
}

// SanitizeBucketName attempts to create a valid bucket bucketName from input
func SanitizeBucketName(name string) string {
	// Convert to lowercase
	name = strings.ToLower(name)

	// Replace invalid characters with hyphens
	result := ""
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			result += string(r)
		} else if r == '.' {
			result += "."
		} else {
			result += "-"
		}
	}

	// Remove consecutive hyphens and periods
	for strings.Contains(result, "--") {
		result = strings.ReplaceAll(result, "--", "-")
	}
	for strings.Contains(result, "..") {
		result = strings.ReplaceAll(result, "..", ".")
	}
	for strings.Contains(result, "-.") {
		result = strings.ReplaceAll(result, "-.", "-")
	}
	for strings.Contains(result, ".-") {
		result = strings.ReplaceAll(result, ".-", "-")
	}

	// Remove leading/trailing hyphens and periods
	result = strings.Trim(result, "-.")

	// Ensure length constraints
	if len(result) < minBucketNameLength {
		result = result + strings.Repeat("x", minBucketNameLength-len(result))
	}
	if len(result) > maxBucketNameLength {
		result = result[:maxBucketNameLength]
		result = strings.Trim(result, "-.")
	}

	// Final validation - if still invalid, use a generic bucketName
	if ValidateBucketName(result) != nil {
		result = "my-bucket-" + generateRandomSuffix()
	}

	return result
}

// SanitizeObjectKey attempts to create a valid object key from input
func SanitizeObjectKey(key string) string {
	// Remove or replace problematic characters
	result := ""
	for _, r := range key {
		// Allow most printable ASCII characters
		if r >= 32 && r <= 126 {
			// Replace some problematic characters
			switch r {
			case '\\':
				result += "/"
			case '"':
				result += "'"
			default:
				result += string(r)
			}
		} else if r > 126 {
			// Keep Unicode characters as they're generally allowed
			result += string(r)
		}
		// Skip control characters (0-31, 127)
	}

	// Remove leading slashes (best practice)
	result = strings.TrimPrefix(result, "/")

	// Ensure length constraints
	if len(result) > maxObjectKeyLength {
		// Try to preserve the file extension if present
		if lastDot := strings.LastIndex(result, "."); lastDot > 0 && lastDot > maxObjectKeyLength-10 {
			extension := result[lastDot:]
			result = result[:maxObjectKeyLength-len(extension)] + extension
		} else {
			result = result[:maxObjectKeyLength]
		}
	}

	// If empty after sanitization, provide a default
	if len(result) == 0 {
		result = "object-" + generateRandomSuffix()
	}

	return result
}

// generateRandomSuffix generates a cryptographically secure random suffix for sanitized names
func generateRandomSuffix() string {
	// Use crypto/rand for secure random generation
	b := make([]byte, 3) // 3 bytes = 6 hex characters
	if _, err := rand.Read(b); err != nil {
		// Fallback to timestamp-based approach if crypto/rand fails
		return fmt.Sprintf("%06d", time.Now().UnixNano()%1000000)
	}
	return hex.EncodeToString(b)
}

// IsBucketNameValid checks if a bucket bucketName is valid without returning an error
func IsBucketNameValid(name string) bool {
	return ValidateBucketName(name) == nil
}

// IsObjectKeyValid checks if an object key is valid without returning an error
func IsObjectKeyValid(key string) bool {
	return ValidateObjectName(key) == nil
}

// BucketNameValidationRules returns a list of bucket bucketName validation rules
func BucketNameValidationRules() []string {
	return []string{
		"Must be 3-63 characters long",
		"Must contain only lowercase letters, numbers, hyphens, and periods",
		"Must start and end with a letter or number",
		"Must not be formatted as an IP address",
		"Must not contain consecutive periods",
		"Must not have periods adjacent to hyphens",
		"Must not start with 'xn--' prefix",
		"Must not end with '-s3alias' or '--ol-s3' suffix",
	}
}

// ObjectKeyValidationRules returns a list of object key validation rules
func ObjectKeyValidationRules() []string {
	return []string{
		"Must be 1-1024 characters long",
		"Must be valid UTF-8",
		"Should not contain control characters",
		"Should avoid characters that require URL encoding",
		"Should not start with forward slash (/) for better compatibility",
	}
}

// ValidateEncryptionOptions validates server-side encryption options
func ValidateEncryptionOptions(sse, kmsKeyId, customerKey, customerAlgorithm string) error {
	// Validate ServerSideEncryption values
	if sse != "" {
		switch sse {
		case SSEAlgorithmAES256, SSEAlgorithmKMS, SSEAlgorithmKMSDSSE:
			// Valid SSE algorithms
		default:
			return fmt.Errorf("invalid server-side encryption algorithm: %s", sse)
		}
	}

	// KMS-specific validations
	if sse == SSEAlgorithmKMS || sse == SSEAlgorithmKMSDSSE {
		if kmsKeyId != "" && len(kmsKeyId) < 1 {
			return fmt.Errorf("KMS key ID cannot be empty when using KMS encryption")
		}
	}

	// Customer-provided encryption validations
	if customerAlgorithm != "" {
		if customerAlgorithm != SSECAlgorithmAES256 {
			return fmt.Errorf("invalid customer encryption algorithm: %s", customerAlgorithm)
		}
		if customerKey == "" {
			return fmt.Errorf("customer encryption key is required when using customer algorithm")
		}
		// Basic length check for base64-encoded 256-bit key (44 characters)
		if len(customerKey) < 44 {
			return fmt.Errorf("customer encryption key appears to be invalid length")
		}
	}

	// Ensure SSE-C and SSE-S3/KMS are not mixed
	if customerAlgorithm != "" && sse != "" {
		return fmt.Errorf("cannot use both server-side encryption and customer-provided encryption")
	}

	return nil
}
