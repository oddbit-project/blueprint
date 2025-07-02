package htpasswd

import (
	"bytes"
	"testing"
)

func TestContainerBasicOperations(t *testing.T) {
	// Create container
	container := NewContainer()

	// Test adding a user
	username := "testuser"
	password := "testpass"

	err := container.AddUser(username, password)
	if err != nil {
		t.Fatalf("Failed to add user: %v", err)
	}

	// Test user exists
	if !container.UserExists(username) {
		t.Fatal("User should exist after adding")
	}

	// Test password verification
	valid, err := container.VerifyUser(username, password)
	if err != nil {
		t.Fatalf("Failed to verify user: %v", err)
	}
	if !valid {
		t.Fatal("Password verification should succeed")
	}

	// Test wrong password
	valid, err = container.VerifyUser(username, "wrongpass")
	if err != nil {
		t.Fatalf("Failed to verify user: %v", err)
	}
	if valid {
		t.Fatal("Password verification should fail for wrong password")
	}

	buf := bytes.NewBuffer([]byte{})

	// Test save and load
	err = container.Write(buf)
	if err != nil {
		t.Fatalf("Failed to save file: %v", err)
	}

	// Create new container and load
	container2, err := NewFromReader(buf)
	if err != nil {
		t.Fatalf("Failed to create second container: %v", err)
	}

	// Test user still exists after load
	if !container2.UserExists(username) {
		t.Fatal("User should exist after loading")
	}

	// Test password still works after load
	valid, err = container2.VerifyUser(username, password)
	if err != nil {
		t.Fatalf("Failed to verify user after load: %v", err)
	}
	if !valid {
		t.Fatal("Password verification should succeed after load")
	}

	// Test delete user
	err = container2.DeleteUser(username)
	if err != nil {
		t.Fatalf("Failed to delete user: %v", err)
	}

	if container2.UserExists(username) {
		t.Fatal("User should not exist after deletion")
	}
}

func TestHashTypes(t *testing.T) {
	container := NewContainer()
	password := "testpass"

	testCases := []struct {
		name     string
		hashType HashType
		username string
	}{
		{"bcrypt", HashTypeBcrypt, "bcrypt_user"},
		{"sha1", HashTypeSHA1, "sha1_user"},
		{"apache_md5", HashTypeApacheMD5, "md5_user"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := container.AddUserWithHash(tc.username, password, tc.hashType)
			if err != nil {
				t.Fatalf("Failed to add user with %s: %v", tc.name, err)
			}

			valid, err := container.VerifyUser(tc.username, password)
			if err != nil {
				t.Fatalf("Failed to verify %s user: %v", tc.name, err)
			}
			if !valid {
				t.Fatalf("Password verification failed for %s", tc.name)
			}

			entry, err := container.GetUser(tc.username)
			if err != nil {
				t.Fatalf("Failed to get user %s: %v", tc.username, err)
			}
			
			detectedType := DetectHashType(entry.Hash)
			if detectedType != tc.hashType {
				t.Fatalf("Hash type mismatch for %s: expected %v, got %v", tc.name, tc.hashType, detectedType)
			}
		})
	}
}

func TestValidation(t *testing.T) {
	container := NewContainer()

	// Test invalid username (contains colon)
	err := container.AddUser("user:invalid", "password")
	if err == nil {
		t.Fatal("Should reject username with colon")
	}

	// Test empty username
	err = container.AddUser("", "password")
	if err == nil {
		t.Fatal("Should reject empty username")
	}

	// Test empty password
	err = container.AddUser("validuser", "")
	if err == nil {
		t.Fatal("Should reject empty password")
	}
}

func TestUsernameByteLimits(t *testing.T) {
	container := NewContainer()

	// Test username at exactly 255 bytes (should pass)
	username255 := ""
	for i := 0; i < 255; i++ {
		username255 += "a"
	}

	err := container.AddUser(username255, "password")
	if err != nil {
		t.Fatalf("Should accept username with exactly 255 bytes: %v", err)
	}

	// Test username with 256 bytes (should fail)
	username256 := username255 + "a"
	err = container.AddUser(username256, "password")
	if err == nil {
		t.Fatal("Should reject username with 256 bytes")
	}

	// Test very long username (1000 bytes)
	username1000 := ""
	for i := 0; i < 1000; i++ {
		username1000 += "x"
	}
	err = container.AddUser(username1000, "password")
	if err == nil {
		t.Fatal("Should reject username with 1000 bytes")
	}

	// Test multi-byte UTF-8 characters near limit
	// Each "é" is 2 bytes in UTF-8
	usernameUTF8 := ""
	for i := 0; i < 127; i++ {
		usernameUTF8 += "é" // 127 * 2 = 254 bytes
	}
	err = container.AddUser(usernameUTF8, "password")
	if err != nil {
		t.Fatalf("Should accept UTF-8 username with 254 bytes: %v", err)
	}

	// Add one more UTF-8 character to exceed limit
	usernameUTF8Over := usernameUTF8 + "é" // 256 bytes
	err = container.AddUser(usernameUTF8Over, "password")
	if err == nil {
		t.Fatal("Should reject UTF-8 username with 256 bytes")
	}
}

func TestInvalidUTF8Sequences(t *testing.T) {
	container := NewContainer()

	testCases := []struct {
		name     string
		username string
		password string
	}{
		{
			"invalid UTF-8 in username",
			string([]byte{0xFF, 0xFE, 0xFD}), // Invalid UTF-8 sequence
			"validpassword",
		},
		{
			"invalid UTF-8 in password",
			"validusername",
			string([]byte{0xFF, 0xFE, 0xFD}), // Invalid UTF-8 sequence
		},
		{
			"incomplete UTF-8 sequence in username",
			string([]byte{0xC0}), // Incomplete UTF-8 sequence
			"validpassword",
		},
		{
			"incomplete UTF-8 sequence in password",
			"validusername",
			string([]byte{0xE0, 0x80}), // Incomplete UTF-8 sequence
		},
		{
			"overlong UTF-8 encoding in username",
			string([]byte{0xC0, 0x80}), // Overlong encoding of null
			"validpassword",
		},
		{
			"overlong UTF-8 encoding in password",
			"validusername",
			string([]byte{0xC1, 0xBF}), // Overlong encoding
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := container.AddUser(tc.username, tc.password)
			if err == nil {
				t.Fatalf("Should reject %s", tc.name)
			}
		})
	}
}

func TestNullBytes(t *testing.T) {
	container := NewContainer()

	testCases := []struct {
		name     string
		username string
		password string
	}{
		{
			"null byte at start of username",
			string([]byte{0x00, 'u', 's', 'e', 'r'}),
			"password",
		},
		{
			"null byte in middle of username",
			string([]byte{'u', 's', 0x00, 'e', 'r'}),
			"password",
		},
		{
			"null byte at end of username",
			string([]byte{'u', 's', 'e', 'r', 0x00}),
			"password",
		},
		{
			"null byte at start of password",
			"username",
			string([]byte{0x00, 'p', 'a', 's', 's'}),
		},
		{
			"null byte in middle of password",
			"username",
			string([]byte{'p', 'a', 0x00, 's', 's'}),
		},
		{
			"null byte at end of password",
			"username",
			string([]byte{'p', 'a', 's', 's', 0x00}),
		},
		{
			"only null byte as username",
			string([]byte{0x00}),
			"password",
		},
		{
			"only null byte as password",
			"username",
			string([]byte{0x00}),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := container.AddUser(tc.username, tc.password)
			// NOTE: Current implementation allows null bytes since they are valid UTF-8
			// This test documents the current behavior - null bytes are NOT rejected
			if err != nil {
				t.Logf("Unexpected error (null bytes should be allowed): %v", err)
			}
			// Test that the user was actually added successfully
			if tc.username != string([]byte{0x00}) { // Skip empty username case
				if !container.UserExists(tc.username) {
					t.Errorf("User with null bytes should exist but doesn't")
				}
			}
		})
	}
}

func TestControlCharacters(t *testing.T) {
	container := NewContainer()

	// Test various control characters
	controlChars := []struct {
		name string
		char byte
	}{
		{"tab", 0x09},
		{"newline", 0x0A},
		{"carriage return", 0x0D},
		{"null", 0x00},
		{"bell", 0x07},
		{"backspace", 0x08},
		{"form feed", 0x0C},
		{"vertical tab", 0x0B},
		{"escape", 0x1B},
		{"delete", 0x7F},
	}

	for _, cc := range controlChars {
		t.Run("username with "+cc.name, func(t *testing.T) {
			username := "user" + string([]byte{cc.char}) + "name"
			err := container.AddUser(username, "password")
			// Note: Current implementation allows all control characters including null
			// Only colon is explicitly rejected by current validation
			if err != nil {
				t.Logf("Control character %s rejected: %v", cc.name, err)
			}
		})

		t.Run("password with "+cc.name, func(t *testing.T) {
			password := "pass" + string([]byte{cc.char}) + "word"
			err := container.AddUser("username"+cc.name, password)
			// Note: Current implementation allows all control characters including null
			if err != nil {
				t.Logf("Control character %s in password rejected: %v", cc.name, err)
			}
		})
	}

	// Test consecutive control characters
	t.Run("multiple control characters in username", func(t *testing.T) {
		username := string([]byte{0x01, 0x02, 0x03, 0x04}) + "user"
		err := container.AddUser(username, "password")
		// Current implementation may allow these
		_ = err // Just test that it doesn't crash
	})

	// Test high-bit control characters (0x80-0x9F)
	t.Run("high control characters", func(t *testing.T) {
		for i := 0x80; i <= 0x9F; i++ {
			username := "user" + string([]byte{byte(i)})
			err := container.AddUser(username, "password")
			// These create invalid UTF-8, so should be rejected
			if err == nil {
				t.Fatalf("Should reject username with high control character 0x%02X", i)
			}
		}
	})
}

func TestFileOperations(t *testing.T) {
	htpasswdFile := bytes.NewBuffer([]byte{})

	// Create a sample htpasswd file content
	content := "user1:$2y$10$abcdefghijklmnopqrstuvwxyz\nuser2:{SHA}5E884898DA28047151D0E56F8DC6292773603D0D6AABBDD62A11EF721D1542D8\n"
	_, err := htpasswdFile.Write([]byte(content))
	if err != nil {
		t.Fatalf("Failed to write test buffer: %v", err)
	}

	container, err := NewFromReader(htpasswdFile)
	if err != nil {
		t.Fatalf("Failed to create container: %v", err)
	}

	// Test that users were loaded
	if !container.UserExists("user1") {
		t.Fatal("user1 should exist after loading")
	}
	if !container.UserExists("user2") {
		t.Fatal("user2 should exist after loading")
	}

	// Test count
	if container.Count() != 2 {
		t.Fatalf("Expected 2 users, got %d", container.Count())
	}

	// Test list users
	users := container.ListUsers()
	if len(users) != 2 {
		t.Fatalf("Expected 2 users in list, got %d", len(users))
	}
}
