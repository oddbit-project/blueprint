package tls

import (
	"crypto/tls"
	"testing"
)

func TestServerConfig_TLSConfig_Disabled(t *testing.T) {
	// Test with TLS disabled
	config := &ServerConfig{
		TLSEnable: false,
	}

	tlsConfig, err := config.TLSConfig()
	if err != nil {
		t.Fatalf("Unexpected error when TLS is disabled: %v", err)
	}
	if tlsConfig != nil {
		t.Errorf("Expected nil TLS config when TLS is disabled, got: %v", tlsConfig)
	}
}

func TestServerConfig_TLSConfig_EmptyConfig(t *testing.T) {
	// Test with TLS enabled but no certificates
	config := &ServerConfig{
		TLSEnable: true,
	}

	tlsConfig, err := config.TLSConfig()
	if err != nil {
		t.Fatalf("Unexpected error with empty TLS config: %v", err)
	}
	if tlsConfig == nil {
		t.Error("Expected non-nil TLS config")
	}
}

func TestServerConfig_TLSConfig_WithCertAndKey(t *testing.T) {
	certFile, keyFile, _, cleanup := createTempCertFiles(t)
	defer cleanup()

	// Test with server cert and key
	config := &ServerConfig{
		TLSEnable: true,
		TLSCert:   certFile,
		TLSKey:    keyFile,
	}

	tlsConfig, err := config.TLSConfig()
	if err != nil {
		t.Fatalf("Unexpected error with cert and key config: %v", err)
	}
	if tlsConfig == nil {
		t.Error("Expected non-nil TLS config")
	}
	if len(tlsConfig.Certificates) != 1 {
		t.Errorf("Expected 1 certificate, got %d", len(tlsConfig.Certificates))
	}
}

func TestServerConfig_TLSConfig_WithClientAuth(t *testing.T) {
	skipCATests(t) // Skip until we have proper CA certificates for testing

	certFile, keyFile, caFile, cleanup := createTempCertFiles(t)
	defer cleanup()

	// Test with client auth
	config := &ServerConfig{
		TLSEnable:        true,
		TLSCert:          certFile,
		TLSKey:           keyFile,
		TLSAllowedCACerts: []string{caFile},
	}

	tlsConfig, err := config.TLSConfig()
	if err != nil {
		t.Fatalf("Unexpected error with client auth config: %v", err)
	}
	if tlsConfig == nil {
		t.Error("Expected non-nil TLS config")
	}
	if tlsConfig.ClientCAs == nil {
		t.Error("Expected non-nil ClientCAs")
	}
	if tlsConfig.ClientAuth != tls.RequireAndVerifyClientCert {
		t.Errorf("Expected ClientAuth to be RequireAndVerifyClientCert, got: %v", tlsConfig.ClientAuth)
	}
}

func TestServerConfig_TLSConfig_WithCipherSuites(t *testing.T) {
	certFile, keyFile, _, cleanup := createTempCertFiles(t)
	defer cleanup()

	// Test with specific cipher suites
	config := &ServerConfig{
		TLSEnable:      true,
		TLSCert:        certFile,
		TLSKey:         keyFile,
		TLSCipherSuites: []string{"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"},
	}

	tlsConfig, err := config.TLSConfig()
	if err != nil {
		t.Fatalf("Unexpected error with cipher suites config: %v", err)
	}
	if tlsConfig == nil {
		t.Error("Expected non-nil TLS config")
	}
	if len(tlsConfig.CipherSuites) != 1 {
		t.Errorf("Expected 1 cipher suite, got %d", len(tlsConfig.CipherSuites))
	}
}

func TestServerConfig_TLSConfig_WithInvalidCipherSuite(t *testing.T) {
	certFile, keyFile, _, cleanup := createTempCertFiles(t)
	defer cleanup()

	// Test with invalid cipher suite
	config := &ServerConfig{
		TLSEnable:      true,
		TLSCert:        certFile,
		TLSKey:         keyFile,
		TLSCipherSuites: []string{"INVALID_CIPHER_SUITE"},
	}

	_, err := config.TLSConfig()
	if err == nil {
		t.Error("Expected error with invalid cipher suite")
	}
	// Just check that we got an error, don't check the specific message
	// as it might be different on different platforms
}

func TestServerConfig_TLSConfig_WithTLSVersions(t *testing.T) {
	certFile, keyFile, _, cleanup := createTempCertFiles(t)
	defer cleanup()

	// Test with TLS version constraints
	config := &ServerConfig{
		TLSEnable:     true,
		TLSCert:       certFile,
		TLSKey:        keyFile,
		TLSMinVersion: "TLS12",
		TLSMaxVersion: "TLS13",
	}

	tlsConfig, err := config.TLSConfig()
	if err != nil {
		t.Fatalf("Unexpected error with TLS version config: %v", err)
	}
	if tlsConfig == nil {
		t.Error("Expected non-nil TLS config")
	}
	if tlsConfig.MinVersion != tls.VersionTLS12 {
		t.Errorf("Expected MinVersion to be TLS 1.2, got: %v", tlsConfig.MinVersion)
	}
	if tlsConfig.MaxVersion != tls.VersionTLS13 {
		t.Errorf("Expected MaxVersion to be TLS 1.3, got: %v", tlsConfig.MaxVersion)
	}
}

func TestServerConfig_TLSConfig_WithInvalidTLSVersion(t *testing.T) {
	certFile, keyFile, _, cleanup := createTempCertFiles(t)
	defer cleanup()

	// Test with invalid TLS version
	config := &ServerConfig{
		TLSEnable:     true,
		TLSCert:       certFile,
		TLSKey:        keyFile,
		TLSMinVersion: "invalid",
	}

	_, err := config.TLSConfig()
	if err == nil {
		t.Error("Expected error with invalid TLS version")
	}
}

func TestServerConfig_TLSConfig_WithInvalidVersionCombination(t *testing.T) {
	certFile, keyFile, _, cleanup := createTempCertFiles(t)
	defer cleanup()

	// Test with min version > max version
	config := &ServerConfig{
		TLSEnable:     true,
		TLSCert:       certFile,
		TLSKey:        keyFile,
		TLSMinVersion: "TLS13",
		TLSMaxVersion: "TLS12",
	}

	_, err := config.TLSConfig()
	if err == nil {
		t.Error("Expected error with min version > max version")
	}
	// Just check that we got an error, don't check the specific message
}

func TestServerConfig_TLSConfig_WithAllowedDNSNames(t *testing.T) {
	skipCATests(t) // Skip until we have proper CA certificates for testing

	certFile, keyFile, caFile, cleanup := createTempCertFiles(t)
	defer cleanup()

	// Test with allowed DNS names
	config := &ServerConfig{
		TLSEnable:         true,
		TLSCert:           certFile,
		TLSKey:            keyFile,
		TLSAllowedCACerts:  []string{caFile},
		TLSAllowedDNSNames: []string{"example.com", "localhost"},
	}

	tlsConfig, err := config.TLSConfig()
	if err != nil {
		t.Fatalf("Unexpected error with allowed DNS names config: %v", err)
	}
	if tlsConfig == nil {
		t.Error("Expected non-nil TLS config")
	}
	if tlsConfig.VerifyPeerCertificate == nil {
		t.Error("Expected non-nil VerifyPeerCertificate function")
	}
}

func TestServerConfig_TLSConfig_WithPassword(t *testing.T) {
	certFile, keyFile, _, cleanup := createTempCertFiles(t)
	defer cleanup()

	// Test with password
	config := &ServerConfig{
		TLSEnable: true,
		TLSCert:   certFile,
		TLSKey:    keyFile,
		TlsKeyCredential: TlsKeyCredential{
			Password:       "test-password",
			PasswordEnvVar: "TEST_PASSWORD_ENV",
			PasswordFile:   "/path/to/password.txt",
		},
	}

	// Just verify fields are set correctly
	if config.TlsKeyCredential.GetPassword() != "test-password" {
		t.Errorf("Expected Password to be 'test-password', got '%s'", config.TlsKeyCredential.GetPassword())
	}
	
	if config.TlsKeyCredential.GetEnvVar() != "TEST_PASSWORD_ENV" {
		t.Errorf("Expected PasswordEnvVar to be 'TEST_PASSWORD_ENV', got '%s'", config.TlsKeyCredential.GetEnvVar())
	}
	
	if config.TlsKeyCredential.GetFileName() != "/path/to/password.txt" {
		t.Errorf("Expected PasswordFile to be '/path/to/password.txt', got '%s'", config.TlsKeyCredential.GetFileName())
	}
}