package secure

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// TestRandomKey32 tests the RandomKey32 function
func TestRandomKey32(t *testing.T) {
	// Test key generation
	key1 := RandomKey32()
	if len(key1) != 32 {
		t.Errorf("RandomKey32 should return 32 bytes, got %d", len(key1))
	}

	// Test that multiple calls return different keys
	key2 := RandomKey32()
	if len(key2) != 32 {
		t.Errorf("RandomKey32 should return 32 bytes, got %d", len(key2))
	}

	// Keys should be different (extremely unlikely to be the same)
	identical := true
	for i := 0; i < 32; i++ {
		if key1[i] != key2[i] {
			identical = false
			break
		}
	}
	if identical {
		t.Error("RandomKey32 should generate different keys on successive calls")
	}

	// Test that generated key is not all zeros
	allZeros := true
	for _, b := range key1 {
		if b != 0 {
			allZeros = false
			break
		}
	}
	if allZeros {
		t.Error("RandomKey32 should not generate all-zero key")
	}
}

// TestRandomCredential tests the RandomCredential function
func TestRandomCredential(t *testing.T) {
	// Test with valid length
	cred, err := RandomCredential(64)
	if err != nil {
		t.Fatalf("RandomCredential should not return error: %v", err)
	}
	if cred == nil {
		t.Fatal("RandomCredential should return non-nil credential")
	}
	if cred.IsEmpty() {
		t.Error("RandomCredential should not return empty credential")
	}

	// Test that credential can be retrieved
	value, err := cred.GetBytes()
	if err != nil {
		t.Errorf("Failed to get bytes from random credential: %v", err)
	}
	if len(value) != 64 {
		t.Errorf("Random credential should contain 64 bytes, got %d", len(value))
	}

	// Test with different length
	cred2, err := RandomCredential(128)
	if err != nil {
		t.Fatalf("RandomCredential should not return error: %v", err)
	}

	value2, err := cred2.GetBytes()
	if err != nil {
		t.Errorf("Failed to get bytes from random credential: %v", err)
	}
	if len(value2) != 128 {
		t.Errorf("Random credential should contain 128 bytes, got %d", len(value2))
	}

	// Test with zero length (should handle gracefully)
	// This will fail with ErrEmptyCredential since allowEmpty=false in RandomCredential
	cred3, err := RandomCredential(0)
	if err != ErrEmptyCredential {
		t.Errorf("RandomCredential with zero length should return ErrEmptyCredential, got: %v", err)
	}
	if cred3 != nil {
		t.Error("RandomCredential with zero length should return nil credential")
	}

	// Test that different calls return different credentials
	cred4, err := RandomCredential(32)
	if err != nil {
		t.Fatalf("RandomCredential should not return error: %v", err)
	}

	cred5, err := RandomCredential(32)
	if err != nil {
		t.Fatalf("RandomCredential should not return error: %v", err)
	}

	value4, _ := cred4.GetBytes()
	value5, _ := cred5.GetBytes()

	identical := true
	for i := 0; i < 32; i++ {
		if value4[i] != value5[i] {
			identical = false
			break
		}
	}
	if identical {
		t.Error("RandomCredential should generate different credentials on successive calls")
	}
}

// TestDefaultCredentialConfig_IsEmpty tests the IsEmpty method
func TestDefaultCredentialConfig_IsEmpty(t *testing.T) {
	tests := []struct {
		name     string
		config   DefaultCredentialConfig
		expected bool
	}{
		{
			name: "empty config",
			config: DefaultCredentialConfig{
				Password:       "",
				PasswordEnvVar: "",
				PasswordFile:   "",
			},
			expected: true,
		},
		{
			name: "config with password",
			config: DefaultCredentialConfig{
				Password:       "test-password",
				PasswordEnvVar: "",
				PasswordFile:   "",
			},
			expected: false,
		},
		{
			name: "config with env var",
			config: DefaultCredentialConfig{
				Password:       "",
				PasswordEnvVar: "TEST_ENV_VAR",
				PasswordFile:   "",
			},
			expected: false,
		},
		{
			name: "config with file",
			config: DefaultCredentialConfig{
				Password:       "",
				PasswordEnvVar: "",
				PasswordFile:   "/path/to/file",
			},
			expected: false,
		},
		{
			name: "config with whitespace only",
			config: DefaultCredentialConfig{
				Password:       "  \t\n  ",
				PasswordEnvVar: "  ",
				PasswordFile:   "\t\n",
			},
			expected: true,
		},
		{
			name: "config with multiple fields",
			config: DefaultCredentialConfig{
				Password:       "password",
				PasswordEnvVar: "ENV_VAR",
				PasswordFile:   "/path/to/file",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.config.IsEmpty()
			if result != tt.expected {
				t.Errorf("IsEmpty() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

// TestKeyConfig_IsEmpty tests the KeyConfig IsEmpty method
func TestKeyConfig_IsEmpty(t *testing.T) {
	tests := []struct {
		name     string
		config   KeyConfig
		expected bool
	}{
		{
			name: "empty config",
			config: KeyConfig{
				Key:       "",
				KeyEnvVar: "",
				KeyFile:   "",
			},
			expected: true,
		},
		{
			name: "config with key",
			config: KeyConfig{
				Key:       "test-key",
				KeyEnvVar: "",
				KeyFile:   "",
			},
			expected: false,
		},
		{
			name: "config with env var",
			config: KeyConfig{
				Key:       "",
				KeyEnvVar: "TEST_KEY_ENV_VAR",
				KeyFile:   "",
			},
			expected: false,
		},
		{
			name: "config with file",
			config: KeyConfig{
				Key:       "",
				KeyEnvVar: "",
				KeyFile:   "/path/to/keyfile",
			},
			expected: false,
		},
		{
			name: "config with whitespace only",
			config: KeyConfig{
				Key:       "  \t\n  ",
				KeyEnvVar: "  ",
				KeyFile:   "\t\n",
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.config.IsEmpty()
			if result != tt.expected {
				t.Errorf("IsEmpty() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

// TestKeyConfig_Fetch tests the KeyConfig Fetch method
func TestKeyConfig_Fetch(t *testing.T) {
	// Test with direct key
	config := KeyConfig{
		Key:       "direct-key",
		KeyEnvVar: "",
		KeyFile:   "",
	}

	result, err := config.Fetch()
	if err != nil {
		t.Errorf("Fetch() with direct key should not return error: %v", err)
	}
	if result != "direct-key" {
		t.Errorf("Fetch() = %s, expected 'direct-key'", result)
	}

	// Test with environment variable
	envVarName := "TEST_KEY_CONFIG_ENV_VAR"
	os.Setenv(envVarName, "env-var-key")
	defer os.Unsetenv(envVarName)

	config = KeyConfig{
		Key:       "",
		KeyEnvVar: envVarName,
		KeyFile:   "",
	}

	result, err = config.Fetch()
	if err != nil {
		t.Errorf("Fetch() with env var should not return error: %v", err)
	}
	if result != "env-var-key" {
		t.Errorf("Fetch() = %s, expected 'env-var-key'", result)
	}

	// Check that env var was cleared
	if os.Getenv(envVarName) != "" {
		t.Error("Environment variable should be cleared after fetch")
	}

	// Test with file
	tempDir, err := os.MkdirTemp("", "key_config_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	keyFile := filepath.Join(tempDir, "key.txt")
	err = os.WriteFile(keyFile, []byte("file-key"), 0600)
	if err != nil {
		t.Fatalf("Failed to create key file: %v", err)
	}

	config = KeyConfig{
		Key:       "",
		KeyEnvVar: "",
		KeyFile:   keyFile,
	}

	result, err = config.Fetch()
	if err != nil {
		t.Errorf("Fetch() with file should not return error: %v", err)
	}
	if result != "file-key" {
		t.Errorf("Fetch() = %s, expected 'file-key'", result)
	}

	// Test with empty config
	config = KeyConfig{
		Key:       "",
		KeyEnvVar: "",
		KeyFile:   "",
	}

	result, err = config.Fetch()
	if err != nil {
		t.Errorf("Fetch() with empty config should not return error: %v", err)
	}
	if result != "" {
		t.Errorf("Fetch() = %s, expected empty string", result)
	}

	// Test with whitespace trimming
	config = KeyConfig{
		Key:       "  trimmed-key  ",
		KeyEnvVar: "",
		KeyFile:   "",
	}

	result, err = config.Fetch()
	if err != nil {
		t.Errorf("Fetch() should not return error: %v", err)
	}
	if result != "trimmed-key" {
		t.Errorf("Fetch() = %s, expected 'trimmed-key'", result)
	}

	// Test with non-existent file
	config = KeyConfig{
		Key:       "",
		KeyEnvVar: "",
		KeyFile:   "/nonexistent/file.txt",
	}

	_, err = config.Fetch()
	if err == nil {
		t.Error("Fetch() with non-existent file should return error")
	}

	// Test priority: Key over EnvVar over File
	os.Setenv("TEST_KEY_PRIORITY", "env-key")
	defer os.Unsetenv("TEST_KEY_PRIORITY")

	config = KeyConfig{
		Key:       "direct-key",
		KeyEnvVar: "TEST_KEY_PRIORITY",
		KeyFile:   keyFile,
	}

	result, err = config.Fetch()
	if err != nil {
		t.Errorf("Fetch() should not return error: %v", err)
	}
	if result != "direct-key" {
		t.Errorf("Fetch() = %s, expected 'direct-key' (priority test)", result)
	}
}

// TestEdgeCases tests various edge cases for better coverage
func TestEdgeCases(t *testing.T) {
	// Test NewCredential with nil data and allowEmpty=true
	key := RandomKey32()
	cred, err := NewCredential(nil, key, true)
	if err != nil {
		t.Errorf("NewCredential with nil data and allowEmpty=true should not error: %v", err)
	}
	if cred == nil {
		t.Error("NewCredential should return non-nil credential")
	}
	if !cred.IsEmpty() {
		t.Error("Credential should be empty")
	}

	// Test Get() on empty credential
	value, err := cred.Get()
	if err != nil {
		t.Errorf("Get() on empty credential should not error: %v", err)
	}
	if value != "" {
		t.Errorf("Get() on empty credential should return empty string, got %s", value)
	}

	// Test GetBytes() on empty credential
	bytes, err := cred.GetBytes()
	if err != nil {
		t.Errorf("GetBytes() on empty credential should not error: %v", err)
	}
	if bytes != nil {
		t.Errorf("GetBytes() on empty credential should return nil, got %v", bytes)
	}

	// Test UpdateBytes with empty slice
	cred2, err := NewCredential([]byte("test"), key, false)
	if err != nil {
		t.Fatalf("Failed to create credential: %v", err)
	}

	err = cred2.UpdateBytes([]byte{})
	if err != nil {
		t.Errorf("UpdateBytes with empty slice should not error: %v", err)
	}
	if !cred2.IsEmpty() {
		t.Error("Credential should be empty after UpdateBytes with empty slice")
	}

	// Test GenerateKey error handling (hard to test actual error from crypto/rand)
	// This would require mocking crypto/rand which is complex
	key, err = GenerateKey()
	if err != nil {
		t.Errorf("GenerateKey should not error in normal conditions: %v", err)
	}
	if len(key) != 32 {
		t.Errorf("GenerateKey should return 32 bytes, got %d", len(key))
	}
}

// MockCredentialConfig implements CredentialConfig for testing error conditions
type MockCredentialConfig struct {
	fetchError error
	isEmpty    bool
}

func (m MockCredentialConfig) Fetch() (string, error) {
	if m.fetchError != nil {
		return "", m.fetchError
	}
	return "mock-credential", nil
}

func (m MockCredentialConfig) IsEmpty() bool {
	return m.isEmpty
}

// TestCredentialFromConfig_ErrorConditions tests error conditions in CredentialFromConfig
func TestCredentialFromConfig_ErrorConditions(t *testing.T) {
	key := RandomKey32()

	// Test with config that returns error
	mockConfig := MockCredentialConfig{
		fetchError: errors.New("fetch error"),
		isEmpty:    false,
	}

	_, err := CredentialFromConfig(mockConfig, key, false)
	if err == nil {
		t.Error("CredentialFromConfig should return error when config.Fetch() returns error")
	}

	// Test with empty config and allowEmpty=true
	mockConfigEmpty := &MockCredentialConfigEmpty{}

	cred, err := CredentialFromConfig(mockConfigEmpty, key, true)
	if err != nil {
		t.Errorf("CredentialFromConfig with empty config and allowEmpty=true should not error: %v", err)
	}
	if cred == nil {
		t.Error("CredentialFromConfig should return non-nil credential")
	}
	if !cred.IsEmpty() {
		t.Error("Credential should be empty")
	}
}

// MockCredentialConfigEmpty for testing empty credentials
type MockCredentialConfigEmpty struct{}

func (m *MockCredentialConfigEmpty) Fetch() (string, error) {
	return "", nil
}

func (m *MockCredentialConfigEmpty) IsEmpty() bool {
	return true
}
