package s3

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateBucketName(t *testing.T) {
	testCases := []struct {
		name        string
		bucketName  string
		expectError bool
	}{
		// Valid bucket names
		{"valid lowercase", "my-bucket", false},
		{"valid with numbers", "my-bucket-123", false},
		{"valid with periods", "my.bucket", false},
		{"valid min length", "abc", false},
		{"valid max length", strings.Repeat("a", 63), false},

		// Invalid bucket names
		{"too short", "ab", true},
		{"too long", strings.Repeat("a", 64), true},
		{"uppercase", "My-Bucket", true},
		{"starts with hyphen", "-my-bucket", true},
		{"ends with hyphen", "my-bucket-", true},
		{"starts with period", ".my-bucket", true},
		{"ends with period", "my-bucket.", true},
		{"consecutive periods", "my..bucket", true},
		{"period next to hyphen", "my-.bucket", true},
		{"hyphen next to period", "my.-bucket", true},
		{"ip address format", "192.168.1.1", true},
		{"starts with xn--", "xn--my-bucket", true},
		{"ends with -s3alias", "my-bucket-s3alias", true},
		{"ends with --ol-s3", "my-bucket--ol-s3", true},
		{"contains space", "my bucket", true},
		{"contains underscore", "my_bucket", true},
		{"contains uppercase", "myBucket", true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateBucketName(tc.bucketName)
			if tc.expectError {
				assert.Error(t, err)
				assert.Equal(t, ErrInvalidBucketName, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateObjectKey(t *testing.T) {
	testCases := []struct {
		name        string
		objectKey   string
		expectError bool
	}{
		// Valid object keys
		{"simple key", "my-object", false},
		{"with path", "folder/my-object.txt", false},
		{"with spaces", "my object.txt", false},
		{"with special chars", "my-object_123.txt", false},
		{"unicode characters", "мой-объект.txt", false},
		{"min length", "a", false},
		{"max length", strings.Repeat("a", 1024), false},

		// Invalid object keys
		{"empty key", "", true},
		{"too long", strings.Repeat("a", 1025), true},
		{"contains null byte", "my-object\x00", true},
		{"contains control char", "my-object\x01", true},
		{"contains DEL char", "my-object\x7F", true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateObjectName(tc.objectKey)
			if tc.expectError {
				assert.Error(t, err)
				assert.Equal(t, ErrInvalidObjectKey, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSanitizeBucketName(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{"uppercase to lowercase", "My-Bucket", "my-bucket"},
		{"spaces to hyphens", "my bucket bucketName", "my-bucket-bucketName"},
		{"underscores to hyphens", "my_bucket_name", "my-bucket-bucketName"},
		{"remove consecutive hyphens", "my---bucket", "my-bucket"},
		{"trim hyphens", "-my-bucket-", "my-bucket"},
		{"short bucketName padding", "ab", "abx"},
		{"long bucketName truncation", strings.Repeat("a", 70), strings.Repeat("a", 63)},
		{"special chars", "My@Bucket#Name!", "my-bucket-bucketName"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := SanitizeBucketName(tc.input)
			assert.True(t, len(result) >= minBucketNameLength)
			assert.True(t, len(result) <= maxBucketNameLength)
			assert.NoError(t, ValidateBucketName(result))

			if tc.expected != "" {
				// Some test cases expect specific results
				if !strings.Contains(tc.expected, "x") { // Skip padding tests
					assert.Equal(t, tc.expected, result)
				}
			}
		})
	}
}

func TestSanitizeObjectKey(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{"backslashes to forward slashes", "folder\\file.txt", "folder/file.txt"},
		{"double quotes to single quotes", "file\"bucketName.txt", "file'bucketName.txt"},
		{"remove control characters", "file\x00\x01name.txt", "filename.txt"},
		{"remove leading slashes", "/folder/file.txt", "folder/file.txt"},
		{"preserve unicode", "файл.txt", "файл.txt"},
		{"long key truncation", strings.Repeat("a", 1100) + ".txt", strings.Repeat("a", 1020) + ".txt"},
		{"empty after sanitization", "\x00\x01\x02", "object-123456"}, // placeholder suffix
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := SanitizeObjectKey(tc.input)
			assert.True(t, len(result) >= minObjectKeyLength)
			assert.True(t, len(result) <= maxObjectKeyLength)
			assert.NoError(t, ValidateObjectName(result))

			// Skip tests with placeholder random suffix
			if !strings.Contains(tc.expected, "123456") {
				assert.Equal(t, tc.expected, result)
			}
		})
	}
}

func TestIsBucketNameValid(t *testing.T) {
	assert.True(t, IsBucketNameValid("valid-bucket"))
	assert.False(t, IsBucketNameValid("Invalid-Bucket"))
	assert.False(t, IsBucketNameValid(""))
}

func TestIsObjectKeyValid(t *testing.T) {
	assert.True(t, IsObjectKeyValid("valid-object.txt"))
	assert.False(t, IsObjectKeyValid(""))
	assert.False(t, IsObjectKeyValid(strings.Repeat("a", 1025)))
}

func TestBucketNameValidationRules(t *testing.T) {
	rules := BucketNameValidationRules()
	assert.NotEmpty(t, rules)
	assert.Contains(t, rules, "Must be 3-63 characters long")
}

func TestObjectKeyValidationRules(t *testing.T) {
	rules := ObjectKeyValidationRules()
	assert.NotEmpty(t, rules)
	assert.Contains(t, rules, "Must be 1-1024 characters long")
}
