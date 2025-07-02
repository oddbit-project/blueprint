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

			detectedType := DetectHashType(tc.username)
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
