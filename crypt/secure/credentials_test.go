package secure

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCredential_Lifecycle(t *testing.T) {
	// Generate a key
	key, err := GenerateKey()
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}
	if len(key) != 32 {
		t.Fatalf("Generated key has incorrect length: expected 32, got %d", len(key))
	}

	// Test empty credential with allowEmpty=true
	emptyCredential, err := NewCredential([]byte{}, key, true)
	if err != nil {
		t.Fatalf("Failed to create empty credential: %v", err)
	}
	if !emptyCredential.IsEmpty() {
		t.Errorf("Credential should be empty")
	}
	value, err := emptyCredential.Get()
	if err != nil {
		t.Errorf("Failed to get empty credential: %v", err)
	}
	if value != "" {
		t.Errorf("Empty credential returned non-empty value: %s", value)
	}

	// Test empty credential with allowEmpty=false
	_, err = NewCredential([]byte{}, key, false)
	if err != ErrEmptyCredential {
		t.Errorf("Expected ErrEmptyCredential, got: %v", err)
	}

	_, err = NewCredential(nil, key, false)
	if err != ErrEmptyCredential {
		t.Errorf("Expected ErrEmptyCredential, got: %v", err)
	}

	// Test credential with invalid key length
	_, err = NewCredential([]byte("password"), []byte("too-short-key"), false)
	if err != ErrInvalidKey {
		t.Errorf("Expected ErrInvalidKey, got: %v", err)
	}

	// Create a valid credential
	testPassword := []byte("secure-test-password")
	credential, err := NewCredential(testPassword, key, false)
	if err != nil {
		t.Fatalf("Failed to create credential: %v", err)
	}
	if credential.IsEmpty() {
		t.Errorf("Credential should not be empty")
	}

	// Test Get() returns the correct value
	retrievedPassword, err := credential.GetBytes()
	if err != nil {
		t.Errorf("Failed to get credential: %v", err)
	}
	if string(retrievedPassword) != string(testPassword) {
		t.Errorf("Retrieved password doesn't match original: expected %s, got %s", testPassword, retrievedPassword)
	}

	// Test Update()
	newPassword := "updated-password"
	err = credential.Update(newPassword)
	if err != nil {
		t.Errorf("Failed to update credential: %v", err)
	}

	retrievedPassword, err = credential.GetBytes()
	if err != nil {
		t.Errorf("Failed to get updated credential: %v", err)
	}
	if string(retrievedPassword) != newPassword {
		t.Errorf("Retrieved updated password doesn't match: expected %s, got %s", newPassword, retrievedPassword)
	}

	// Test Clear()
	credential.Clear()
	if !credential.IsEmpty() {
		t.Errorf("Credential should be empty after Clear()")
	}
	_, err = credential.Get()
	if err != nil {
		t.Errorf("Get() after Clear() should return empty string without error: %v", err)
	}

	// Test Update() with empty value
	err = credential.Update("")
	if err != nil {
		t.Errorf("Update with empty value should not return error: %v", err)
	}
	if !credential.IsEmpty() {
		t.Errorf("Credential should be empty after Update with empty value")
	}
}

func TestEncryptDecrypt(t *testing.T) {
	// Generate a key
	key, err := GenerateKey()
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	// Test successful encryption/decryption
	plaintext := []byte("test-data-to-encrypt")
	ciphertext, nonce, err := encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("Encryption failed: %v", err)
	}
	if len(ciphertext) == 0 {
		t.Errorf("Ciphertext is empty")
	}
	if len(nonce) == 0 {
		t.Errorf("Nonce is empty")
	}

	// Test successful decryption
	decrypted, err := decrypt(ciphertext, nonce, key)
	if err != nil {
		t.Fatalf("Decryption failed: %v", err)
	}
	if string(decrypted) != string(plaintext) {
		t.Errorf("Decrypted text doesn't match original: expected %s, got %s", plaintext, decrypted)
	}

	// Test decryption with wrong key
	wrongKey, _ := GenerateKey()
	_, err = decrypt(ciphertext, nonce, wrongKey)
	if err != ErrDecryption {
		t.Errorf("Expected ErrDecryption with wrong key, got: %v", err)
	}

	// Test decryption with wrong nonce
	wrongNonce := make([]byte, len(nonce))
	copy(wrongNonce, nonce)
	wrongNonce[0] = wrongNonce[0] ^ 0xFF // Flip some bits
	_, err = decrypt(ciphertext, wrongNonce, key)
	if err != ErrDecryption {
		t.Errorf("Expected ErrDecryption with wrong nonce, got: %v", err)
	}
}

func TestCredentialFromEnv(t *testing.T) {
	// Generate a key
	key, err := GenerateKey()
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	// Test with non-existent environment variable
	_, err = CredentialFromEnv("TEST_ENV_VAR_DOES_NOT_EXIST", key, false)
	if err != ErrEmptyCredential {
		t.Errorf("Expected ErrEmptyCredential, got: %v", err)
	}

	// Test with existing environment variable
	testValue := "test-env-value"
	os.Setenv("TEST_CREDENTIAL_ENV_VAR", testValue)
	defer os.Unsetenv("TEST_CREDENTIAL_ENV_VAR")

	credential, err := CredentialFromEnv("TEST_CREDENTIAL_ENV_VAR", key, false)
	if err != nil {
		t.Fatalf("Failed to create credential from env: %v", err)
	}

	retrievedValue, err := credential.Get()
	if err != nil {
		t.Errorf("Failed to get credential value: %v", err)
	}
	if retrievedValue != testValue {
		t.Errorf("Retrieved value doesn't match: expected %s, got %s", testValue, retrievedValue)
	}

	// Test with empty environment variable and allowEmpty=true
	os.Setenv("TEST_CREDENTIAL_EMPTY_ENV_VAR", "")
	defer os.Unsetenv("TEST_CREDENTIAL_EMPTY_ENV_VAR")

	_, err = CredentialFromEnv("TEST_CREDENTIAL_EMPTY_ENV_VAR", key, false)
	if err != ErrEmptyCredential {
		t.Errorf("Expected ErrEmptyCredential for empty env var, got: %v", err)
	}
}

// Since we can't directly replace package functions in Go, we need to create a wrapper
// for testing the file-based credential functions
func TestCredentialFromFile(t *testing.T) {
	// Generate a key
	key, err := GenerateKey()
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	// Create a temporary test file
	tempDir, err := os.MkdirTemp("", "credential_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Test with non-existent file
	nonExistentFile := filepath.Join(tempDir, "nonexistent.txt")
	_, err = CredentialFromFile(nonExistentFile, key, false)
	if err != ErrSecretsFileNotFound {
		t.Errorf("Expected ErrSecretsFileNotFound, got: %v", err)
	}

	// Test with empty file
	emptyFile := filepath.Join(tempDir, "empty.txt")
	err = os.WriteFile(emptyFile, []byte(""), 0600)
	if err != nil {
		t.Fatalf("Failed to create empty file: %v", err)
	}

	_, err = CredentialFromFile(emptyFile, key, false)
	if err != ErrEmptyCredential {
		t.Errorf("Expected ErrEmptyCredential, got: %v", err)
	}

	// Test with valid file
	testValue := "test-file-value"
	validFile := filepath.Join(tempDir, "valid.txt")
	err = os.WriteFile(validFile, []byte(testValue), 0600)
	if err != nil {
		t.Fatalf("Failed to create valid file: %v", err)
	}

	credential, err := CredentialFromFile(validFile, key, false)
	if err != nil {
		t.Fatalf("Failed to create credential from file: %v", err)
	}

	retrievedValue, err := credential.Get()
	if err != nil {
		t.Errorf("Failed to get credential value: %v", err)
	}
	if retrievedValue != testValue {
		t.Errorf("Retrieved value doesn't match: expected %s, got %s", testValue, retrievedValue)
	}
}

func TestKeyEncoding(t *testing.T) {
	// Generate a key
	key, err := GenerateKey()
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	// Test encoding/decoding
	encoded := EncodeKey(key)
	decoded, err := DecodeKey(encoded)
	if err != nil {
		t.Fatalf("Failed to decode key: %v", err)
	}

	if len(decoded) != len(key) {
		t.Errorf("Decoded key length mismatch: expected %d, got %d", len(key), len(decoded))
	}

	for i := range key {
		if decoded[i] != key[i] {
			t.Errorf("Decoded key mismatch at index %d: expected %d, got %d", i, key[i], decoded[i])
		}
	}

	// Test decoding invalid base64
	_, err = DecodeKey("invalid-base64!@#$")
	if err == nil {
		t.Errorf("Expected error when decoding invalid base64, got nil")
	}
}

// Mock implementation of CredentialConfig for testing
type mockCredentialConfig struct {
	password string
	envVar   string
	fileName string
}

func (m mockCredentialConfig) GetPassword() string {
	return m.password
}

func (m mockCredentialConfig) GetEnvVar() string {
	return m.envVar
}

func (m mockCredentialConfig) GetFileName() string {
	return m.fileName
}

func TestCredentialFromConfig(t *testing.T) {
	// Generate a key
	key, err := GenerateKey()
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	// Test with direct password
	config := mockCredentialConfig{
		password: "direct-password",
		envVar:   "",
		fileName: "",
	}

	credential, err := CredentialFromConfig(config, key, false)
	if err != nil {
		t.Fatalf("Failed to create credential from config with password: %v", err)
	}

	value, err := credential.Get()
	if err != nil {
		t.Errorf("Failed to get credential value: %v", err)
	}
	if value != "direct-password" {
		t.Errorf("Retrieved value doesn't match: expected %s, got %s", "direct-password", value)
	}

	// Test with env var
	envVarName := "TEST_CREDENTIAL_ENV_VAR_CONFIG"
	os.Setenv(envVarName, "env-var-password")
	defer os.Unsetenv(envVarName)

	config = mockCredentialConfig{
		password: "",
		envVar:   envVarName,
		fileName: "",
	}

	credential, err = CredentialFromConfig(config, key, false)
	if err != nil {
		t.Fatalf("Failed to create credential from config with env var: %v", err)
	}

	value, err = credential.Get()
	if err != nil {
		t.Errorf("Failed to get credential value: %v", err)
	}
	if value != "env-var-password" {
		t.Errorf("Retrieved value doesn't match: expected %s, got %s", "env-var-password", value)
	}

	// Test with file
	// Create a temporary file with a password
	tempDir, err := os.MkdirTemp("", "credential_config_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	secretFile := filepath.Join(tempDir, "secret.txt")
	err = os.WriteFile(secretFile, []byte("file-password"), 0600)
	if err != nil {
		t.Fatalf("Failed to create secret file: %v", err)
	}

	config = mockCredentialConfig{
		password: "",
		envVar:   "",
		fileName: secretFile,
	}

	credential, err = CredentialFromConfig(config, key, false)
	if err != nil {
		t.Fatalf("Failed to create credential from config with file: %v", err)
	}

	value, err = credential.Get()
	if err != nil {
		t.Errorf("Failed to get credential value: %v", err)
	}
	if value != "file-password" {
		t.Errorf("Retrieved value doesn't match: expected %s, got %s", "file-password", value)
	}

	// Test with empty config and allowEmpty=false
	config = mockCredentialConfig{
		password: "",
		envVar:   "",
		fileName: "",
	}

	_, err = CredentialFromConfig(config, key, false)
	if err != ErrEmptyCredential {
		t.Errorf("Expected ErrEmptyCredential, got: %v", err)
	}

	// Test with empty config and allowEmpty=true
	credential, err = CredentialFromConfig(config, key, true)
	if err != nil {
		t.Fatalf("Failed to create empty credential from config: %v", err)
	}
	if !credential.IsEmpty() {
		t.Errorf("Credential should be empty")
	}
}

func TestDefaultCredentialConfig(t *testing.T) {
	cfg := DefaultCredentialConfig{
		Password:       "test-password",
		PasswordEnvVar: "TEST_ENV_VAR",
		PasswordFile:   "/path/to/secret/file",
	}

	if cfg.GetPassword() != "test-password" {
		t.Errorf("GetPassword returned incorrect value: expected %s, got %s", "test-password", cfg.GetPassword())
	}

	if cfg.GetEnvVar() != "TEST_ENV_VAR" {
		t.Errorf("GetEnvVar returned incorrect value: expected %s, got %s", "TEST_ENV_VAR", cfg.GetEnvVar())
	}

	if cfg.GetFileName() != "/path/to/secret/file" {
		t.Errorf("GetFileName returned incorrect value: expected %s, got %s", "/path/to/secret/file", cfg.GetFileName())
	}
}
