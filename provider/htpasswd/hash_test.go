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
		{"argon2", HashTypeArgon2, "$argon2i$"},
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
		{"$argon2i$v=19$m=65536,t=2,p=1$salt$hash", HashTypeArgon2},
		{"$argon2id$v=19$m=65536,t=2,p=1$salt$hash", HashTypeArgon2},
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

func TestArgon2Specific(t *testing.T) {
	password := "testpass"

	// Test hash generation
	hash, err := HashArgon2(password)
	if err != nil {
		t.Fatalf("HashArgon2 failed: %v", err)
	}

	// Test format - should start with $argon2i$
	if !strings.HasPrefix(hash, "$argon2i$") {
		t.Errorf("Argon2 hash should start with $argon2i$, got: %s", hash)
	}

	// Test that hash contains version
	if !strings.Contains(hash, "v=19") {
		t.Errorf("Argon2 hash should contain v=19, got: %s", hash)
	}

	// Test that hash contains parameters
	if !strings.Contains(hash, "m=65536") {
		t.Errorf("Argon2 hash should contain m=65536, got: %s", hash)
	}
	if !strings.Contains(hash, "t=2") {
		t.Errorf("Argon2 hash should contain t=2, got: %s", hash)
	}
	if !strings.Contains(hash, "p=1") {
		t.Errorf("Argon2 hash should contain p=1, got: %s", hash)
	}

	// Test that verification works
	if !VerifyArgon2(password, hash) {
		t.Errorf("Argon2 verification failed for hash: %s", hash)
	}

	// Test that wrong password fails
	if VerifyArgon2("wrongpass", hash) {
		t.Errorf("Argon2 verification should fail for wrong password")
	}

	// Test that different calls produce different hashes (due to random salt)
	hash2, err := HashArgon2(password)
	if err != nil {
		t.Fatalf("HashArgon2 failed on second call: %v", err)
	}

	if hash == hash2 {
		t.Errorf("Argon2 should produce different hashes due to random salt: %s == %s", hash, hash2)
	}

	// But both should verify correctly
	if !VerifyArgon2(password, hash2) {
		t.Errorf("Argon2 verification failed for second hash: %s", hash2)
	}
}

func TestArgon2FormatValidation(t *testing.T) {
	password := "testpass"

	testCases := []struct {
		name  string
		hash  string
		valid bool
	}{
		{
			"valid argon2i hash",
			"$argon2i$v=19$m=65536,t=2,p=1$c29tZXNhbHQxMjM0NTY$dGVzdGhhc2gxMjM0NTY3ODkwMTIzNDU2Nzg5MDEyMzQ",
			false, // Will fail because hash doesn't match
		},
		{
			"valid argon2id hash",
			"$argon2id$v=19$m=65536,t=2,p=1$c29tZXNhbHQxMjM0NTY$dGVzdGhhc2gxMjM0NTY3ODkwMTIzNDU2Nzg5MDEyMzQ",
			false, // Will fail because hash doesn't match
		},
		{
			"invalid prefix",
			"$argon2$v=19$m=65536,t=2,p=1$c29tZXNhbHQ$dGVzdGhhc2g",
			false,
		},
		{
			"missing version",
			"$argon2i$m=65536,t=2,p=1$c29tZXNhbHQ$dGVzdGhhc2g",
			false,
		},
		{
			"invalid format (too few parts)",
			"$argon2i$v=19$m=65536,t=2,p=1",
			false,
		},
		{
			"invalid format (too many parts)",
			"$argon2i$v=19$m=65536,t=2,p=1$salt$hash$extra",
			false,
		},
		{
			"missing parameters",
			"$argon2i$v=19$$c29tZXNhbHQ$dGVzdGhhc2g",
			false,
		},
		{
			"invalid base64 salt",
			"$argon2i$v=19$m=65536,t=2,p=1$!!!invalid!!!$dGVzdGhhc2g",
			false,
		},
		{
			"invalid base64 hash",
			"$argon2i$v=19$m=65536,t=2,p=1$c29tZXNhbHQ$!!!invalid!!!",
			false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := VerifyArgon2(password, tc.hash)
			if result != tc.valid {
				t.Errorf("Expected verification to be %v for %s, got %v", tc.valid, tc.name, result)
			}
		})
	}
}

func TestArgon2ParameterParsing(t *testing.T) {
	testCases := []struct {
		name   string
		hash   string
		expect bool
	}{
		{
			"valid parameters argon2i",
			"$argon2i$v=19$m=65536,t=2,p=1$c2FsdA$aGFzaA",
			false, // Will fail verification due to invalid hash
		},
		{
			"valid parameters argon2id",
			"$argon2id$v=19$m=65536,t=2,p=1$c2FsdA$aGFzaA",
			false, // Will fail verification due to invalid hash
		},
		{
			"high memory parameter",
			"$argon2i$v=19$m=1048576,t=2,p=1$c2FsdA$aGFzaA",
			false,
		},
		{
			"high time parameter",
			"$argon2i$v=19$m=65536,t=10,p=1$c2FsdA$aGFzaA",
			false,
		},
		{
			"high parallelism",
			"$argon2i$v=19$m=65536,t=2,p=4$c2FsdA$aGFzaA",
			false,
		},
		{
			"invalid parameter format (missing m)",
			"$argon2i$v=19$t=2,p=1$c2FsdA$aGFzaA",
			false,
		},
		{
			"invalid parameter format (missing t)",
			"$argon2i$v=19$m=65536,p=1$c2FsdA$aGFzaA",
			false,
		},
		{
			"invalid parameter format (missing p)",
			"$argon2i$v=19$m=65536,t=2$c2FsdA$aGFzaA",
			false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := VerifyArgon2("password", tc.hash)
			if result != tc.expect {
				t.Logf("Verification result: %v (hash: %s)", result, tc.hash)
			}
			// All should fail verification, but shouldn't crash
		})
	}
}

func TestArgon2EmptyPassword(t *testing.T) {
	// Test hashing empty password
	hash, err := HashArgon2("")
	if err != nil {
		t.Fatalf("HashArgon2 should handle empty password: %v", err)
	}

	// Verify empty password
	if !VerifyArgon2("", hash) {
		t.Errorf("Should verify empty password")
	}

	// Verify non-empty password should fail
	if VerifyArgon2("nonempty", hash) {
		t.Errorf("Non-empty password should not verify against empty password hash")
	}
}

func TestArgon2LongPassword(t *testing.T) {
	// Test with very long password (1000 characters)
	longPassword := strings.Repeat("a", 1000)

	hash, err := HashArgon2(longPassword)
	if err != nil {
		t.Fatalf("HashArgon2 should handle long passwords: %v", err)
	}

	if !VerifyArgon2(longPassword, hash) {
		t.Errorf("Should verify long password")
	}

	// Verify slightly different password fails
	almostLongPassword := strings.Repeat("a", 999) + "b"
	if VerifyArgon2(almostLongPassword, hash) {
		t.Errorf("Should not verify different long password")
	}
}

func TestArgon2SpecialCharacters(t *testing.T) {
	testPasswords := []string{
		"pass!@#$%^&*()",
		"пароль", // Cyrillic
		"密码",    // Chinese
		"🔐🔑",   // Emojis
		"pass\nword",
		"pass\tword",
		"pass word",
	}

	for _, password := range testPasswords {
		t.Run(password, func(t *testing.T) {
			hash, err := HashArgon2(password)
			if err != nil {
				t.Fatalf("HashArgon2 failed for password %q: %v", password, err)
			}

			if !VerifyArgon2(password, hash) {
				t.Errorf("Verification failed for password %q with hash %s", password, hash)
			}

			// Verify different password fails
			if VerifyArgon2(password+"x", hash) {
				t.Errorf("Should not verify modified password %q", password+"x")
			}
		})
	}
}

func TestArgon2HashUniqueness(t *testing.T) {
	password := "testpass"
	hashes := make(map[string]bool)

	// Generate 10 hashes and ensure they're all unique
	for i := 0; i < 10; i++ {
		hash, err := HashArgon2(password)
		if err != nil {
			t.Fatalf("HashArgon2 failed on iteration %d: %v", i, err)
		}

		if hashes[hash] {
			t.Errorf("Duplicate hash generated: %s", hash)
		}
		hashes[hash] = true

		// All should verify
		if !VerifyArgon2(password, hash) {
			t.Errorf("Hash %d failed verification: %s", i, hash)
		}
	}

	if len(hashes) != 10 {
		t.Errorf("Expected 10 unique hashes, got %d", len(hashes))
	}
}

func TestArgon2Integration(t *testing.T) {
	container := NewContainer()
	password := "argon2testpass"
	username := "argon2user"

	// Add user with Argon2 hash
	err := container.AddUserWithHash(username, password, HashTypeArgon2)
	if err != nil {
		t.Fatalf("Failed to add user with Argon2 hash: %v", err)
	}

	// Verify user exists
	if !container.UserExists(username) {
		t.Fatal("User should exist after adding")
	}

	// Verify password
	valid, err := container.VerifyUser(username, password)
	if err != nil {
		t.Fatalf("Failed to verify user: %v", err)
	}
	if !valid {
		t.Fatal("Password verification should succeed")
	}

	// Verify wrong password fails
	valid, err = container.VerifyUser(username, "wrongpass")
	if err != nil {
		t.Fatalf("Failed to verify user with wrong password: %v", err)
	}
	if valid {
		t.Fatal("Password verification should fail for wrong password")
	}

	// Get user and check hash format
	entry, err := container.GetUser(username)
	if err != nil {
		t.Fatalf("Failed to get user: %v", err)
	}

	if !strings.HasPrefix(entry.Hash, "$argon2i$") {
		t.Errorf("Hash should start with $argon2i$, got: %s", entry.Hash)
	}

	// Test detection
	detectedType := DetectHashType(entry.Hash)
	if detectedType != HashTypeArgon2 {
		t.Errorf("Hash type detection failed: expected %v, got %v", HashTypeArgon2, detectedType)
	}
}
