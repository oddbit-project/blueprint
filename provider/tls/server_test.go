package tls

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		TLSEnable:         true,
		TLSCert:           certFile,
		TLSKey:            keyFile,
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
		TLSEnable:       true,
		TLSCert:         certFile,
		TLSKey:          keyFile,
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
		TLSEnable:       true,
		TLSCert:         certFile,
		TLSKey:          keyFile,
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
		TLSEnable:          true,
		TLSCert:            certFile,
		TLSKey:             keyFile,
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
	key, err := config.Fetch()
	if err != nil {
		t.Fatalf("Unexpected error with password config: %v", err)
	}
	if key != "test-password" {
		t.Errorf("Expected Password to be 'test-password', got '%s'", key)
	}

}

func TestServerConfig_TLSConfig_NoCert_AppliesVersionsAndCiphers(t *testing.T) {
	tests := []struct {
		name    string
		config  ServerConfig
		wantErr error
		wantMin uint16
		wantMax uint16
		ciphers []uint16
	}{
		{
			name:    "defaults",
			config:  ServerConfig{TLSEnable: true},
			wantMin: TLSMinVersionDefault,
			ciphers: defaultCipherSuites,
		},
		{
			name: "explicit versions and ciphers",
			config: ServerConfig{
				TLSEnable:       true,
				TLSMinVersion:   "TLS12",
				TLSMaxVersion:   "TLS13",
				TLSCipherSuites: []string{"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"},
			},
			wantMin: tls.VersionTLS12,
			wantMax: tls.VersionTLS13,
			ciphers: []uint16{tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256},
		},
		{
			name:    "invalid min version",
			config:  ServerConfig{TLSEnable: true, TLSMinVersion: "1.3"},
			wantErr: ErrInvalidTlsVersion,
		},
		{
			name:    "invalid max version",
			config:  ServerConfig{TLSEnable: true, TLSMaxVersion: "1.2"},
			wantErr: ErrInvalidTlsVersion,
		},
		{
			name:    "min greater than max",
			config:  ServerConfig{TLSEnable: true, TLSMinVersion: "TLS13", TLSMaxVersion: "TLS12"},
			wantErr: ErrInvalidTlsVersion,
		},
		{
			name:    "invalid cipher",
			config:  ServerConfig{TLSEnable: true, TLSCipherSuites: []string{"NOT_A_CIPHER"}},
			wantErr: ErrInvalidCipher,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := tt.config.TLSConfig()
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				assert.Nil(t, cfg)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, cfg)
			assert.Equal(t, tt.wantMin, cfg.MinVersion)
			assert.Equal(t, tt.wantMax, cfg.MaxVersion)
			assert.Equal(t, tt.ciphers, cfg.CipherSuites)
		})
	}
}

func TestServerConfig_VerifyPeerCertificate_AllowedDNSNames(t *testing.T) {
	newCert := func(t *testing.T, dnsNames ...string) []byte {
		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		require.NoError(t, err)
		tpl := &x509.Certificate{
			SerialNumber: big.NewInt(1),
			NotBefore:    time.Now().Add(-time.Hour),
			NotAfter:     time.Now().Add(time.Hour),
			DNSNames:     dnsNames,
		}
		der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &key.PublicKey, key)
		require.NoError(t, err)
		return der
	}

	config := &ServerConfig{TLSAllowedDNSNames: []string{"allowed.example.com"}}

	tests := []struct {
		name     string
		dnsNames []string
		wantErr  error
	}{
		{"allowed name", []string{"allowed.example.com"}, nil},
		{"allowed name among others", []string{"other.example.com", "allowed.example.com"}, nil},
		{"name not in list", []string{"evil.example.com"}, ErrForbiddenDNS},
		{"no names", nil, ErrForbiddenDNS},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := config.verifyPeerCertificate([][]byte{newCert(t, tt.dnsNames...)}, nil)
			assert.ErrorIs(t, err, tt.wantErr)
		})
	}
}
