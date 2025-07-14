package htpasswd

import (
	"strings"
	"testing"
)

func TestHashPassword(t *testing.T) {
	password := "testpass"

	testCases := []struct {
		name     string
		hashType HashType
		prefix   string
	}{
		{"bcrypt", HashTypeBcrypt, "$2"},
		{"apache_md5", HashTypeApacheMD5, "$apr1$"},
		{"sha1", HashTypeSHA1, "{SHA}"},
		{"sha256", HashTypeSHA256, "{SHA256}"},
		{"sha512", HashTypeSHA512, "{SHA512}"},
		{"crypt", HashTypeCrypt, ""},
		{"plain", HashTypePlain, ""},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			hash, err := HashPassword(password, tc.hashType)
			if err != nil {
				t.Fatalf("HashPassword failed for %s: %v", tc.name, err)
			}

			if tc.prefix != "" && hash[:len(tc.prefix)] != tc.prefix {
				t.Errorf("Hash for %s should start with %s, got: %s", tc.name, tc.prefix, hash)
			}

			// Verify the hash works
			if !VerifyPassword(password, hash) {
				t.Errorf("Password verification failed for %s hash: %s", tc.name, hash)
			}

			// Verify wrong password fails
			if VerifyPassword("wrongpass", hash) {
				t.Errorf("Password verification should fail for wrong password with %s hash", tc.name)
			}
		})
	}
}

func TestDetectHashType(t *testing.T) {
	testCases := []struct {
		hash     string
		expected HashType
	}{
		{"$2y$10$abcdefghijklmnopqrstuv", HashTypeBcrypt},
		{"$2a$10$abcdefghijklmnopqrstuv", HashTypeBcrypt},
		{"$apr1$salt$hash", HashTypeApacheMD5},
		{"{SHA}hashhashhashhash", HashTypeSHA1},
		{"{SHA256}hashhashhashhash", HashTypeSHA256},
		{"{SHA512}hashhashhashhash", HashTypeSHA512},
		{"abcdefghijklm", HashTypeCrypt},
		{"plaintext", HashTypePlain},
	}

	for _, tc := range testCases {
		t.Run(tc.hash, func(t *testing.T) {
			detected := DetectHashType(tc.hash)
			if detected != tc.expected {
				t.Errorf("Expected %v, got %v for hash: %s", tc.expected, detected, tc.hash)
			}
		})
	}
}

func TestApacheMD5Specific(t *testing.T) {
	password := "testpass"
	salt := "testsalt"

	// Test that the same password and salt produce consistent results
	hash1 := apacheMD5Hash(password, salt)
	hash2 := apacheMD5Hash(password, salt)

	if hash1 != hash2 {
		t.Errorf("Apache MD5 should be deterministic: %s != %s", hash1, hash2)
	}

	// Test that it follows the correct format
	expected_prefix := "$apr1$" + salt + "$"
	if hash1[:len(expected_prefix)] != expected_prefix {
		t.Errorf("Apache MD5 hash should start with %s, got: %s", expected_prefix, hash1)
	}

	// Test that verification works
	if !VerifyApacheMD5(password, hash1) {
		t.Errorf("Apache MD5 verification failed for hash: %s", hash1)
	}

	// Test that wrong password fails
	if VerifyApacheMD5("wrongpass", hash1) {
		t.Errorf("Apache MD5 verification should fail for wrong password")
	}
}

func TestCryptSpecific(t *testing.T) {
	password := "testpass"

	// Test that crypt hash is 13 characters
	hash, err := HashCrypt(password)
	if err != nil {
		t.Fatalf("hashCrypt failed: %v", err)
	}

	if len(hash) != 13 {
		t.Errorf("Crypt hash should be 13 characters, got %d: %s", len(hash), hash)
	}

	// Test that verification works
	if !VerifyCrypt(password, hash) {
		t.Errorf("Crypt verification failed for hash: %s", hash)
	}

	// Test that wrong password fails
	if VerifyCrypt("wrongpass", hash) {
		t.Errorf("Crypt verification should fail for wrong password")
	}
}

func TestSHA256Specific(t *testing.T) {
	password := "testpass"

	// Test hash generation
	hash, err := HashSHA256(password)
	if err != nil {
		t.Fatalf("hashSHA256 failed: %v", err)
	}

	// Test format
	if !strings.HasPrefix(hash, "{SHA256}") {
		t.Errorf("SHA256 hash should start with {SHA256}, got: %s", hash)
	}

	// Test that the same password produces the same hash
	hash2, err := HashSHA256(password)
	if err != nil {
		t.Fatalf("hashSHA256 failed on second call: %v", err)
	}

	if hash != hash2 {
		t.Errorf("SHA256 should be deterministic: %s != %s", hash, hash2)
	}

	// Test verification works
	if !VerifySHA256(password, hash) {
		t.Errorf("SHA256 verification failed for hash: %s", hash)
	}

	// Test that wrong password fails
	if VerifySHA256("wrongpass", hash) {
		t.Errorf("SHA256 verification should fail for wrong password")
	}
}

func TestSHA512Specific(t *testing.T) {
	password := "testpass"

	// Test hash generation
	hash, err := HashSHA512(password)
	if err != nil {
		t.Fatalf("hashSHA512 failed: %v", err)
	}

	// Test format
	if !strings.HasPrefix(hash, "{SHA512}") {
		t.Errorf("SHA512 hash should start with {SHA512}, got: %s", hash)
	}

	// Test that the same password produces the same hash
	hash2, err := HashSHA512(password)
	if err != nil {
		t.Fatalf("hashSHA512 failed on second call: %v", err)
	}

	if hash != hash2 {
		t.Errorf("SHA512 should be deterministic: %s != %s", hash, hash2)
	}

	// Test verification works
	if !VerifySHA512(password, hash) {
		t.Errorf("SHA512 verification failed for hash: %s", hash)
	}

	// Test that wrong password fails
	if VerifySHA512("wrongpass", hash) {
		t.Errorf("SHA512 verification should fail for wrong password")
	}
}
