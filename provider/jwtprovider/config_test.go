package jwtprovider

import (
	"crypto/elliptic"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/oddbit-project/blueprint/crypt/secure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test NewJWTConfig
func TestNewJWTConfig(t *testing.T) {
	cfg := NewJWTConfig()
	
	assert.NotNil(t, cfg)
	assert.Nil(t, cfg.CfgSigningKey)
	assert.Nil(t, cfg.CfgPrivateKey)
	assert.Nil(t, cfg.CfgPublicKey)
	assert.Equal(t, HS256, cfg.SigningAlgorithm)
	assert.Equal(t, int(DefaultTTL.Seconds()), cfg.ExpirationSeconds)
	assert.Equal(t, DefaultTTL, cfg.expiration)
	assert.Equal(t, DefaultIssuer, cfg.Issuer)
	assert.Equal(t, DefaultAudience, cfg.Audience)
	assert.Equal(t, "default", cfg.KeyID)
	assert.True(t, cfg.RequireIssuer)
	assert.True(t, cfg.RequireAudience)
	assert.False(t, cfg.validated)
	assert.Nil(t, cfg.signingKey)
	assert.Nil(t, cfg.privateKey)
	assert.Nil(t, cfg.publicKey)
}

// Test NewJWTConfigWithKey
func TestNewJWTConfigWithKey(t *testing.T) {
	testKey := []byte("test-signing-key-for-jwt-tokens")
	
	cfg, err := NewJWTConfigWithKey(testKey)
	require.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.NotNil(t, cfg.signingKey)
	
	// Verify the key was properly stored
	storedKey, err := cfg.signingKey.GetBytes()
	require.NoError(t, err)
	assert.Equal(t, testKey, storedKey)
	
	// Test with empty key
	_, err = NewJWTConfigWithKey([]byte{})
	assert.Error(t, err)
}

// Test Validate with HMAC algorithms
func TestValidate_HMAC(t *testing.T) {
	algorithms := []string{HS256, HS384, HS512}
	
	for _, alg := range algorithms {
		t.Run(alg, func(t *testing.T) {
			// Valid configuration with password
			cfg := NewJWTConfig()
			cfg.SigningAlgorithm = alg
			cfg.CfgSigningKey = &secure.DefaultCredentialConfig{
				Password: "test-secret-key",
			}
			
			err := cfg.Validate()
			assert.NoError(t, err)
			assert.True(t, cfg.validated)
			assert.NotNil(t, cfg.signingMethod)
			assert.Equal(t, alg, cfg.signingMethod.Alg())
			
			// Valid configuration with env var
			envVar := "TEST_JWT_SECRET_" + alg // Make unique for each algorithm
			os.Setenv(envVar, "env-secret-key")
			defer os.Unsetenv(envVar)
			
			cfg2 := NewJWTConfig()
			cfg2.SigningAlgorithm = alg
			cfg2.CfgSigningKey = &secure.DefaultCredentialConfig{
				PasswordEnvVar: envVar,
			}
			
			err = cfg2.Validate()
			assert.NoError(t, err)
			
			// Valid configuration with file
			tempDir := t.TempDir()
			secretFile := filepath.Join(tempDir, "secret.txt")
			err = os.WriteFile(secretFile, []byte("file-secret-key"), 0600)
			require.NoError(t, err)
			
			cfg3 := NewJWTConfig()
			cfg3.SigningAlgorithm = alg
			cfg3.CfgSigningKey = &secure.DefaultCredentialConfig{
				PasswordFile: secretFile,
			}
			
			err = cfg3.Validate()
			assert.NoError(t, err)
			
			// Invalid configuration - no signing key
			cfg4 := NewJWTConfig()
			cfg4.SigningAlgorithm = alg
			
			err = cfg4.Validate()
			assert.Error(t, err)
			assert.ErrorIs(t, err, ErrSigningKeyRequired)
			
			// Invalid configuration - empty signing key
			cfg5 := NewJWTConfig()
			cfg5.SigningAlgorithm = alg
			cfg5.CfgSigningKey = &secure.DefaultCredentialConfig{}
			
			err = cfg5.Validate()
			assert.Error(t, err)
			assert.ErrorIs(t, err, ErrSigningKeyRequired)
		})
	}
}

// Test Validate with RSA algorithms
func TestValidate_RSA(t *testing.T) {
	algorithms := []string{RS256, RS384, RS512}
	
	// Generate test keys
	privateKeyPEM, publicKeyPEM := generateRSAKeyPair(t)
	
	for _, alg := range algorithms {
		t.Run(alg, func(t *testing.T) {
			// Valid configuration
			cfg := NewJWTConfig()
			cfg.SigningAlgorithm = alg
			cfg.CfgPrivateKey = &secure.KeyConfig{
				Key: string(privateKeyPEM),
			}
			cfg.CfgPublicKey = &secure.KeyConfig{
				Key: string(publicKeyPEM),
			}
			
			err := cfg.Validate()
			assert.NoError(t, err)
			assert.True(t, cfg.validated)
			assert.NotNil(t, cfg.signingMethod)
			assert.Equal(t, alg, cfg.signingMethod.Alg())
			
			// Invalid configuration - no private key
			cfg2 := NewJWTConfig()
			cfg2.SigningAlgorithm = alg
			cfg2.CfgPublicKey = &secure.KeyConfig{
				Key: string(publicKeyPEM),
			}
			
			err = cfg2.Validate()
			assert.Error(t, err)
			assert.ErrorIs(t, err, ErrPrivateKeyRequired)
			
			// Invalid configuration - no public key
			cfg3 := NewJWTConfig()
			cfg3.SigningAlgorithm = alg
			cfg3.CfgPrivateKey = &secure.KeyConfig{
				Key: string(privateKeyPEM),
			}
			
			err = cfg3.Validate()
			assert.Error(t, err)
			assert.ErrorIs(t, err, ErrPublicKeyRequired)
			
			// Invalid configuration - empty private key
			cfg4 := NewJWTConfig()
			cfg4.SigningAlgorithm = alg
			cfg4.CfgPrivateKey = &secure.KeyConfig{}
			cfg4.CfgPublicKey = &secure.KeyConfig{
				Key: string(publicKeyPEM),
			}
			
			err = cfg4.Validate()
			assert.Error(t, err)
			assert.ErrorIs(t, err, ErrPrivateKeyRequired)
			
			// Invalid configuration - invalid private key format
			cfg5 := NewJWTConfig()
			cfg5.SigningAlgorithm = alg
			cfg5.CfgPrivateKey = &secure.KeyConfig{
				Key: "invalid-private-key",
			}
			cfg5.CfgPublicKey = &secure.KeyConfig{
				Key: string(publicKeyPEM),
			}
			
			err = cfg5.Validate()
			assert.Error(t, err)
			
			// Invalid configuration - invalid public key format
			cfg6 := NewJWTConfig()
			cfg6.SigningAlgorithm = alg
			cfg6.CfgPrivateKey = &secure.KeyConfig{
				Key: string(privateKeyPEM),
			}
			cfg6.CfgPublicKey = &secure.KeyConfig{
				Key: "invalid-public-key",
			}
			
			err = cfg6.Validate()
			assert.Error(t, err)
		})
	}
}

// Test Validate with ECDSA algorithms
func TestValidate_ECDSA(t *testing.T) {
	algorithms := []struct {
		name  string
		curve elliptic.Curve
	}{
		{ES256, elliptic.P256()},
		{ES384, elliptic.P384()},
		{ES512, elliptic.P521()},
	}
	
	for _, alg := range algorithms {
		t.Run(alg.name, func(t *testing.T) {
			// Generate appropriate keys for this algorithm
			privateKeyPEM, publicKeyPEM := generateECDSAKeyPairForCurve(t, alg.curve)
			
			// Valid configuration with both keys
			cfg := NewJWTConfig()
			cfg.SigningAlgorithm = alg.name
			cfg.CfgPrivateKey = &secure.KeyConfig{
				Key: string(privateKeyPEM),
			}
			cfg.CfgPublicKey = &secure.KeyConfig{
				Key: string(publicKeyPEM),
			}
			
			err := cfg.Validate()
			assert.NoError(t, err)
			assert.True(t, cfg.validated)
			assert.NotNil(t, cfg.signingMethod)
			assert.Equal(t, alg.name, cfg.signingMethod.Alg())
			
			// Valid configuration with only private key (public key derived)
			cfg2 := NewJWTConfig()
			cfg2.SigningAlgorithm = alg.name
			cfg2.CfgPrivateKey = &secure.KeyConfig{
				Key: string(privateKeyPEM),
			}
			
			err = cfg2.Validate()
			assert.NoError(t, err)
			assert.True(t, cfg2.validated)
			assert.NotNil(t, cfg2.publicKey)
			
			// Invalid configuration - no private key
			cfg3 := NewJWTConfig()
			cfg3.SigningAlgorithm = alg.name
			
			err = cfg3.Validate()
			assert.Error(t, err)
			assert.ErrorIs(t, err, ErrPrivateKeyRequired)
			
			// Invalid configuration - empty private key
			cfg4 := NewJWTConfig()
			cfg4.SigningAlgorithm = alg.name
			cfg4.CfgPrivateKey = &secure.KeyConfig{}
			
			err = cfg4.Validate()
			assert.Error(t, err)
			assert.ErrorIs(t, err, ErrPrivateKeyRequired)
			
			// Invalid configuration - invalid private key format
			cfg5 := NewJWTConfig()
			cfg5.SigningAlgorithm = alg.name
			cfg5.CfgPrivateKey = &secure.KeyConfig{
				Key: "invalid-private-key",
			}
			
			err = cfg5.Validate()
			assert.Error(t, err)
		})
	}
}

// Test Validate with EdDSA algorithm
func TestValidate_EdDSA(t *testing.T) {
	// Generate test keys
	privateKeyPEM, publicKeyPEM := generateEdDSAKeyPair(t)
	
	// Valid configuration with both keys
	cfg := NewJWTConfig()
	cfg.SigningAlgorithm = EdDSA
	cfg.CfgPrivateKey = &secure.KeyConfig{
		Key: string(privateKeyPEM),
	}
	cfg.CfgPublicKey = &secure.KeyConfig{
		Key: string(publicKeyPEM),
	}
	
	err := cfg.Validate()
	assert.NoError(t, err)
	assert.True(t, cfg.validated)
	assert.NotNil(t, cfg.signingMethod)
	assert.Equal(t, EdDSA, cfg.signingMethod.Alg())
	
	// Valid configuration with only private key (public key derived)
	cfg2 := NewJWTConfig()
	cfg2.SigningAlgorithm = EdDSA
	cfg2.CfgPrivateKey = &secure.KeyConfig{
		Key: string(privateKeyPEM),
	}
	
	err = cfg2.Validate()
	assert.NoError(t, err)
	assert.True(t, cfg2.validated)
	assert.NotNil(t, cfg2.publicKey)
	
	// Invalid configuration - no private key
	cfg3 := NewJWTConfig()
	cfg3.SigningAlgorithm = EdDSA
	
	err = cfg3.Validate()
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrPrivateKeyRequired)
	
	// Invalid configuration - empty private key
	cfg4 := NewJWTConfig()
	cfg4.SigningAlgorithm = EdDSA
	cfg4.CfgPrivateKey = &secure.KeyConfig{}
	
	err = cfg4.Validate()
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrPrivateKeyRequired)
	
	// Invalid configuration - invalid private key format
	cfg5 := NewJWTConfig()
	cfg5.SigningAlgorithm = EdDSA
	cfg5.CfgPrivateKey = &secure.KeyConfig{
		Key: "invalid-private-key",
	}
	
	err = cfg5.Validate()
	assert.Error(t, err)
}

// Test Validate with invalid configurations
func TestValidate_Invalid(t *testing.T) {
	tests := []struct {
		name        string
		setupCfg    func() *JWTConfig
		wantErr     error
		errContains string
	}{
		{
			name: "invalid signing algorithm",
			setupCfg: func() *JWTConfig {
				cfg := NewJWTConfig()
				cfg.SigningAlgorithm = "INVALID"
				return cfg
			},
			wantErr: ErrInvalidSigningAlgorithm,
		},
		{
			name: "unsupported signing algorithm",
			setupCfg: func() *JWTConfig {
				cfg := NewJWTConfig()
				cfg.SigningAlgorithm = "PS256" // Valid JWT alg but not supported
				return cfg
			},
			wantErr: ErrInvalidSigningAlgorithm,
		},
		{
			name: "zero expiration seconds",
			setupCfg: func() *JWTConfig {
				cfg := NewJWTConfig()
				cfg.SigningAlgorithm = HS256
				cfg.CfgSigningKey = &secure.DefaultCredentialConfig{
					Password: "test-secret",
				}
				cfg.ExpirationSeconds = 0
				return cfg
			},
			wantErr: ErrInvalidDuration,
		},
		{
			name: "negative expiration seconds",
			setupCfg: func() *JWTConfig {
				cfg := NewJWTConfig()
				cfg.SigningAlgorithm = HS256
				cfg.CfgSigningKey = &secure.DefaultCredentialConfig{
					Password: "test-secret",
				}
				cfg.ExpirationSeconds = -100
				return cfg
			},
			wantErr: ErrInvalidDuration,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.setupCfg()
			err := cfg.Validate()
			
			assert.Error(t, err)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			}
			if tt.errContains != "" {
				assert.Contains(t, err.Error(), tt.errContains)
			}
			assert.False(t, cfg.validated)
		})
	}
}

// Test Validate idempotency
func TestValidate_Idempotent(t *testing.T) {
	cfg := NewJWTConfig()
	cfg.SigningAlgorithm = HS256
	cfg.CfgSigningKey = &secure.DefaultCredentialConfig{
		Password: "test-secret",
	}
	
	// First validation
	err := cfg.Validate()
	assert.NoError(t, err)
	assert.True(t, cfg.validated)
	
	// Store the signing method reference
	signingMethod := cfg.signingMethod
	
	// Second validation should return immediately
	err = cfg.Validate()
	assert.NoError(t, err)
	assert.True(t, cfg.validated)
	assert.Same(t, signingMethod, cfg.signingMethod)
}

// Test key configuration with files
func TestKeyConfiguration_Files(t *testing.T) {
	tempDir := t.TempDir()
	
	// Create test key files
	signingKeyFile := filepath.Join(tempDir, "signing.key")
	err := os.WriteFile(signingKeyFile, []byte("file-signing-key"), 0600)
	require.NoError(t, err)
	
	privateKeyPEM, publicKeyPEM := generateRSAKeyPair(t)
	privateKeyFile := filepath.Join(tempDir, "private.key")
	err = os.WriteFile(privateKeyFile, privateKeyPEM, 0600)
	require.NoError(t, err)
	
	publicKeyFile := filepath.Join(tempDir, "public.key")
	err = os.WriteFile(publicKeyFile, publicKeyPEM, 0600)
	require.NoError(t, err)
	
	t.Run("HMAC with file", func(t *testing.T) {
		cfg := NewJWTConfig()
		cfg.SigningAlgorithm = HS256
		cfg.CfgSigningKey = &secure.DefaultCredentialConfig{
			PasswordFile: signingKeyFile,
		}
		
		err := cfg.Validate()
		assert.NoError(t, err)
		assert.NotNil(t, cfg.signingKey)
	})
	
	t.Run("RSA with files", func(t *testing.T) {
		cfg := NewJWTConfig()
		cfg.SigningAlgorithm = RS256
		cfg.CfgPrivateKey = &secure.KeyConfig{
			KeyFile: privateKeyFile,
		}
		cfg.CfgPublicKey = &secure.KeyConfig{
			KeyFile: publicKeyFile,
		}
		
		err := cfg.Validate()
		assert.NoError(t, err)
		assert.NotNil(t, cfg.privateKey)
		assert.NotNil(t, cfg.publicKey)
	})
}

// Test key configuration with environment variables
func TestKeyConfiguration_EnvVars(t *testing.T) {
	// Set up environment variables
	os.Setenv("TEST_SIGNING_KEY", "env-signing-key")
	defer os.Unsetenv("TEST_SIGNING_KEY")
	
	privateKeyPEM, publicKeyPEM := generateRSAKeyPair(t)
	os.Setenv("TEST_PRIVATE_KEY", string(privateKeyPEM))
	defer os.Unsetenv("TEST_PRIVATE_KEY")
	
	os.Setenv("TEST_PUBLIC_KEY", string(publicKeyPEM))
	defer os.Unsetenv("TEST_PUBLIC_KEY")
	
	t.Run("HMAC with env var", func(t *testing.T) {
		cfg := NewJWTConfig()
		cfg.SigningAlgorithm = HS256
		cfg.CfgSigningKey = &secure.DefaultCredentialConfig{
			PasswordEnvVar: "TEST_SIGNING_KEY",
		}
		
		err := cfg.Validate()
		assert.NoError(t, err)
		assert.NotNil(t, cfg.signingKey)
	})
	
	t.Run("RSA with env vars", func(t *testing.T) {
		cfg := NewJWTConfig()
		cfg.SigningAlgorithm = RS256
		cfg.CfgPrivateKey = &secure.KeyConfig{
			KeyEnvVar: "TEST_PRIVATE_KEY",
		}
		cfg.CfgPublicKey = &secure.KeyConfig{
			KeyEnvVar: "TEST_PUBLIC_KEY",
		}
		
		err := cfg.Validate()
		assert.NoError(t, err)
		assert.NotNil(t, cfg.privateKey)
		assert.NotNil(t, cfg.publicKey)
	})
}

// Test requireSigningKey
func TestRequireSigningKey(t *testing.T) {
	t.Run("with pre-set signingKey", func(t *testing.T) {
		cfg := NewJWTConfig()
		key, err := secure.NewCredential([]byte("preset-key"), secure.RandomKey32(), false)
		require.NoError(t, err)
		cfg.signingKey = key
		
		err = cfg.requireSigningKey()
		assert.NoError(t, err)
		assert.Same(t, key, cfg.signingKey)
	})
	
	t.Run("with CfgSigningKey", func(t *testing.T) {
		cfg := NewJWTConfig()
		cfg.CfgSigningKey = &secure.DefaultCredentialConfig{
			Password: "config-key",
		}
		
		err := cfg.requireSigningKey()
		assert.NoError(t, err)
		assert.NotNil(t, cfg.signingKey)
		
		// Verify the key value
		storedKey, err := cfg.signingKey.GetBytes()
		require.NoError(t, err)
		assert.Equal(t, []byte("config-key"), storedKey)
	})
	
	t.Run("error from CredentialFromConfig", func(t *testing.T) {
		cfg := NewJWTConfig()
		cfg.CfgSigningKey = &secure.DefaultCredentialConfig{
			PasswordFile: "/nonexistent/file",
		}
		
		err := cfg.requireSigningKey()
		assert.Error(t, err)
	})
}

// Test edge cases for key derivation
func TestKeyDerivation_EdgeCases(t *testing.T) {
	t.Run("ECDSA public key derivation error", func(t *testing.T) {
		cfg := NewJWTConfig()
		cfg.SigningAlgorithm = ES256
		
		// Create a mock private key that will fail to parse
		invalidKey, err := secure.NewCredential([]byte("invalid-ecdsa-key"), secure.RandomKey32(), false)
		require.NoError(t, err)
		cfg.privateKey = invalidKey
		
		err = cfg.requireECDSA()
		assert.Error(t, err)
	})
	
	t.Run("EdDSA public key derivation with short key", func(t *testing.T) {
		cfg := NewJWTConfig()
		cfg.SigningAlgorithm = EdDSA
		
		// Create a key that's too short for EdDSA (less than 64 bytes)
		shortKey, err := secure.NewCredential([]byte("short"), secure.RandomKey32(), false)
		require.NoError(t, err)
		cfg.privateKey = shortKey
		
		// This should error when trying to derive public key due to slice bounds
		func() {
			defer func() {
				if r := recover(); r != nil {
					// Expected panic due to slice bounds
					assert.Contains(t, fmt.Sprintf("%v", r), "slice bounds out of range")
				}
			}()
			err = cfg.requireEdDSA()
			// If no panic, should be an error
			if err == nil {
				t.Error("Expected error for short EdDSA key")
			}
		}()
	})
}

// Test all signing algorithm constants
func TestSigningAlgorithmConstants(t *testing.T) {
	algorithms := []struct {
		constant string
		expected string
	}{
		{HS256, "HS256"},
		{HS384, "HS384"},
		{HS512, "HS512"},
		{RS256, "RS256"},
		{RS384, "RS384"},
		{RS512, "RS512"},
		{ES256, "ES256"},
		{ES384, "ES384"},
		{ES512, "ES512"},
		{EdDSA, "EdDSA"},
	}
	
	for _, alg := range algorithms {
		t.Run(alg.constant, func(t *testing.T) {
			assert.Equal(t, alg.expected, alg.constant)
			
			// Verify JWT library recognizes the algorithm
			signingMethod := jwt.GetSigningMethod(alg.constant)
			assert.NotNil(t, signingMethod, "JWT library should recognize %s", alg.constant)
		})
	}
}

// Test default constants
func TestDefaultConstants(t *testing.T) {
	assert.Equal(t, time.Second*86400, DefaultTTL)
	assert.Equal(t, "blueprint", DefaultIssuer)
	assert.Equal(t, "api", DefaultAudience)
}

// Test custom expiration durations
func TestCustomExpirationDurations(t *testing.T) {
	tests := []struct {
		name              string
		expirationSeconds int
		expectedDuration  time.Duration
	}{
		{"1 minute", 60, time.Minute},
		{"5 minutes", 300, 5 * time.Minute},
		{"1 hour", 3600, time.Hour},
		{"24 hours", 86400, 24 * time.Hour},
		{"7 days", 604800, 7 * 24 * time.Hour},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := NewJWTConfig()
			cfg.SigningAlgorithm = HS256
			cfg.CfgSigningKey = &secure.DefaultCredentialConfig{
				Password: "test-secret",
			}
			cfg.ExpirationSeconds = tt.expirationSeconds
			
			err := cfg.Validate()
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedDuration, cfg.expiration)
		})
	}
}