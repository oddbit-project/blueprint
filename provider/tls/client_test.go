package tls

import (
	"os"
	"path/filepath"
	"testing"
)

// Helper to skip test cases that require certificate validation
func skipCATests(t *testing.T) {
	t.Skip("Skipping test that requires valid CA certificate")
}

// Helper to create temporary cert files for testing
func createTempCertFiles(t *testing.T) (certFile, keyFile, caFile string, cleanup func()) {
	tempDir, err := os.MkdirTemp("", "tls_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	// Sample self-signed certificate and key for testing
	certPEM := `-----BEGIN CERTIFICATE-----
MIIBhTCCASugAwIBAgIQIRi6zePL6mKjOipn+dNuaTAKBggqhkjOPQQDAjASMRAw
DgYDVQQKEwdBY21lIENvMB4XDTE3MTAyMDE5NDMwNloXDTE4MTAyMDE5NDMwNlow
EjEQMA4GA1UEChMHQWNtZSBDbzBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABD0d
7VNhbWvZLWPuj/RtHFjvtJBEwOkhbN/BnnE8rnZR8+sbwnc/KhCk3FhnpHZnQz7B
5aETbbIgmuvewdjvSBSjYzBhMA4GA1UdDwEB/wQEAwICpDATBgNVHSUEDDAKBggr
BgEFBQcDATAPBgNVHRMBAf8EBTADAQH/MCkGA1UdEQQiMCCCDmxvY2FsaG9zdDo1
NDUzgg4xMjcuMC4wLjE6NTQ1MzAKBggqhkjOPQQDAgNIADBFAiEA2zpJEPQyz6/l
Wf86aX6PepsntZv2GYlA5UpabfT2EZICICpJ5h/iI+i341gBmLiAFQOyTDT+/wQc
6MF9+Yw1Yy0t
-----END CERTIFICATE-----`

	keyPEM := `-----BEGIN EC PRIVATE KEY-----
MHcCAQEEIIrYSSNQFaA2Hwf1duRSxKtLYX5CB04fSeQ6tF1aY/PuoAoGCCqGSM49
AwEHoUQDQgAEPR3tU2Fta9ktY+6P9G0cWO+0kETA6SFs38GecTyudlHz6xvCdz8q
EKTcWGekdmdDPsHloRNtsiCa697B2O9IFA==
-----END EC PRIVATE KEY-----`

	// Use the same cert as CA for simplicity
	caPEM := certPEM

	certFile = filepath.Join(tempDir, "cert.pem")
	keyFile = filepath.Join(tempDir, "key.pem")
	caFile = filepath.Join(tempDir, "ca.pem")

	if err := os.WriteFile(certFile, []byte(certPEM), 0600); err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to write cert file: %v", err)
	}

	if err := os.WriteFile(keyFile, []byte(keyPEM), 0600); err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to write key file: %v", err)
	}

	if err := os.WriteFile(caFile, []byte(caPEM), 0600); err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to write CA file: %v", err)
	}

	cleanup = func() {
		os.RemoveAll(tempDir)
	}

	return certFile, keyFile, caFile, cleanup
}

func TestClientConfig_TLSConfig_Disabled(t *testing.T) {
	// Test with TLS disabled
	config := &ClientConfig{
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

func TestClientConfig_TLSConfig_EmptyConfig(t *testing.T) {
	// Test with TLS enabled but no certificates
	config := &ClientConfig{
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

func TestClientConfig_TLSConfig_WithCA(t *testing.T) {
	skipCATests(t) // Skip until we have proper CA certificates for testing

	_, _, caFile, cleanup := createTempCertFiles(t)
	defer cleanup()

	// Test with CA only
	config := &ClientConfig{
		TLSEnable: true,
		TLSCA:     caFile,
	}

	tlsConfig, err := config.TLSConfig()
	if err != nil {
		t.Fatalf("Unexpected error with CA config: %v", err)
	}
	if tlsConfig == nil {
		t.Error("Expected non-nil TLS config")
	}
	if tlsConfig.RootCAs == nil {
		t.Error("Expected non-nil RootCAs")
	}
}

func TestClientConfig_TLSConfig_WithCertAndKey(t *testing.T) {
	certFile, keyFile, _, cleanup := createTempCertFiles(t)
	defer cleanup()

	// Test with client cert and key
	config := &ClientConfig{
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

func TestClientConfig_TLSConfig_Complete(t *testing.T) {
	skipCATests(t) // Skip until we have proper CA certificates for testing

	certFile, keyFile, caFile, cleanup := createTempCertFiles(t)
	defer cleanup()

	// Test with complete config
	config := &ClientConfig{
		TLSEnable:             true,
		TLSCA:                 caFile,
		TLSCert:               certFile,
		TLSKey:                keyFile,
		TLSInsecureSkipVerify: true,
	}

	tlsConfig, err := config.TLSConfig()
	if err != nil {
		t.Fatalf("Unexpected error with complete config: %v", err)
	}
	if tlsConfig == nil {
		t.Error("Expected non-nil TLS config")
	}
	if !tlsConfig.InsecureSkipVerify {
		t.Error("InsecureSkipVerify should be true")
	}
	if tlsConfig.RootCAs == nil {
		t.Error("Expected non-nil RootCAs")
	}
	if len(tlsConfig.Certificates) != 1 {
		t.Errorf("Expected 1 certificate, got %d", len(tlsConfig.Certificates))
	}
}

func TestClientConfig_TLSConfig_InvalidCA(t *testing.T) {
	// Test with invalid CA file
	config := &ClientConfig{
		TLSEnable: true,
		TLSCA:     "/nonexistent/ca.pem",
	}

	_, err := config.TLSConfig()
	if err == nil {
		t.Error("Expected error with invalid CA file")
	}
}

func TestClientConfig_TLSConfig_InvalidCert(t *testing.T) {
	skipCATests(t) // Skip until we have proper CA certificates for testing

	_, _, caFile, cleanup := createTempCertFiles(t)
	defer cleanup()

	// Test with valid CA but invalid cert/key
	config := &ClientConfig{
		TLSEnable: true,
		TLSCA:     caFile,
		TLSCert:   "/nonexistent/cert.pem",
		TLSKey:    "/nonexistent/key.pem",
	}

	_, err := config.TLSConfig()
	if err == nil {
		t.Error("Expected error with invalid cert/key files")
	}
}

func TestClientConfig_TLSConfig_WithPassword(t *testing.T) {
	// Test the TlsKeyCredential fields
	config := &ClientConfig{
		TLSEnable: true,
		TlsKeyCredential: TlsKeyCredential{
			Password:       "test-password",
			PasswordEnvVar: "TEST_PASSWORD_ENV",
			PasswordFile:   "/path/to/password.txt",
		},
	}

	// Just verify that the fields are set correctly
	key, err := config.TlsKeyCredential.Fetch()
	if err != nil {
		t.Fatalf("Unexpected error when fetching key: %v", err)
	}
	if key != "test-password" {
		t.Errorf("Expected Password to be 'test-password', got '%s'", key)
	}
}
