package jwtprovider

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/oddbit-project/blueprint/crypt/secure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test helper functions

func generateRSAKeyPair(t *testing.T) (privateKeyPEM, publicKeyPEM []byte) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	// Encode private key
	privateKeyBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	require.NoError(t, err)
	privateKeyPEM = pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privateKeyBytes,
	})

	// Encode public key
	publicKeyBytes, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	require.NoError(t, err)
	publicKeyPEM = pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyBytes,
	})

	return privateKeyPEM, publicKeyPEM
}

func generateECDSAKeyPair(t *testing.T) (privateKeyPEM, publicKeyPEM []byte) {
	return generateECDSAKeyPairForCurve(t, elliptic.P256())
}

func generateECDSAKeyPairForCurve(t *testing.T, curve elliptic.Curve) (privateKeyPEM, publicKeyPEM []byte) {
	privateKey, err := ecdsa.GenerateKey(curve, rand.Reader)
	require.NoError(t, err)

	// Encode private key
	privateKeyBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	require.NoError(t, err)
	privateKeyPEM = pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privateKeyBytes,
	})

	// Encode public key
	publicKeyBytes, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	require.NoError(t, err)
	publicKeyPEM = pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyBytes,
	})

	return privateKeyPEM, publicKeyPEM
}

func generateEdDSAKeyPair(t *testing.T) (privateKeyPEM, publicKeyPEM []byte) {
	// ed25519.GenerateKey returns (publicKey, privateKey, error)
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	// For EdDSA, we need to ensure the PKCS8/PKIX encoding works correctly
	// Encode private key using PKCS8 format
	privateKeyBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	require.NoError(t, err)
	privateKeyPEM = pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privateKeyBytes,
	})

	// Encode public key using PKIX format
	publicKeyBytes, err := x509.MarshalPKIXPublicKey(publicKey)
	require.NoError(t, err)
	publicKeyPEM = pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyBytes,
	})

	// Verify that we can decode them back correctly
	block, _ := pem.Decode(privateKeyPEM)
	require.NotNil(t, block, "Failed to decode private key PEM")
	parsedPrivKey, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	require.NoError(t, err, "Failed to parse PKCS8 private key")
	require.IsType(t, ed25519.PrivateKey{}, parsedPrivKey, "Private key should be ed25519.PrivateKey")

	pubBlock, _ := pem.Decode(publicKeyPEM)
	require.NotNil(t, pubBlock, "Failed to decode public key PEM")
	parsedPubKey, err := x509.ParsePKIXPublicKey(pubBlock.Bytes)
	require.NoError(t, err, "Failed to parse PKIX public key")
	require.IsType(t, ed25519.PublicKey{}, parsedPubKey, "Public key should be ed25519.PublicKey")

	return privateKeyPEM, publicKeyPEM
}

// Mock types for testing
type mockSecret struct {
	data []byte
	err  error
}

func (m *mockSecret) GetBytes() ([]byte, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.data, nil
}

func (m *mockSecret) Get() (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return string(m.data), nil
}

// Tests for ProviderOpts
func TestWithRevocationManager(t *testing.T) {
	backend := NewMemoryRevocationBackend()
	revManager := NewRevocationManager(backend)
	
	cfg := NewJWTConfig()
	cfg.SigningAlgorithm = HS256
	cfg.CfgSigningKey = &secure.DefaultCredentialConfig{Password: "test-secret-key-for-signing-jwt"}
	
	provider, err := NewProvider(cfg, WithRevocationManager(revManager))
	require.NoError(t, err)
	
	p := provider.(*jwtProvider)
	assert.Equal(t, revManager, p.revocationManager)
	assert.NotNil(t, provider.GetRevocationManager())
}

// Tests for NewProvider
func TestNewProvider(t *testing.T) {
	tests := []struct {
		name      string
		setupCfg  func() *JWTConfig
		wantErr   bool
		errString string
	}{
		{
			name: "valid HS256 config",
			setupCfg: func() *JWTConfig {
				cfg := NewJWTConfig()
				cfg.SigningAlgorithm = HS256
				cfg.CfgSigningKey = &secure.DefaultCredentialConfig{Password: "test-secret-key-for-signing-jwt"}
				return cfg
			},
			wantErr: false,
		},
		{
			name: "invalid config - no signing key",
			setupCfg: func() *JWTConfig {
				cfg := NewJWTConfig()
				cfg.SigningAlgorithm = HS256
				return cfg
			},
			wantErr:   true,
			errString: "signing key is required",
		},
		{
			name: "invalid signing algorithm",
			setupCfg: func() *JWTConfig {
				cfg := NewJWTConfig()
				cfg.SigningAlgorithm = "INVALID"
				return cfg
			},
			wantErr:   true,
			errString: "JWT signing algorithm is invalid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.setupCfg()
			provider, err := NewProvider(cfg)
			
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errString != "" {
					assert.Contains(t, err.Error(), tt.errString)
				}
				assert.Nil(t, provider)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, provider)
			}
		})
	}
}

// Tests for GenerateToken and ParseToken with all algorithms
func TestGenerateAndParseToken_AllAlgorithms(t *testing.T) {
	testSubject := "test-user"
	testData := map[string]any{
		"user_id": "123",
		"role":    "admin",
		"email":   "test@example.com",
	}

	tests := []struct {
		name      string
		algorithm string
		setupCfg  func() *JWTConfig
	}{
		// HMAC algorithms
		{
			name:      "HS256",
			algorithm: HS256,
			setupCfg: func() *JWTConfig {
				cfg := NewJWTConfig()
				cfg.SigningAlgorithm = HS256
				cfg.CfgSigningKey = &secure.DefaultCredentialConfig{Password: "test-secret-key-for-hs256-signing"}
				return cfg
			},
		},
		{
			name:      "HS384",
			algorithm: HS384,
			setupCfg: func() *JWTConfig {
				cfg := NewJWTConfig()
				cfg.SigningAlgorithm = HS384
				cfg.CfgSigningKey = &secure.DefaultCredentialConfig{Password: "test-secret-key-for-hs384-signing"}
				return cfg
			},
		},
		{
			name:      "HS512",
			algorithm: HS512,
			setupCfg: func() *JWTConfig {
				cfg := NewJWTConfig()
				cfg.SigningAlgorithm = HS512
				cfg.CfgSigningKey = &secure.DefaultCredentialConfig{Password: "test-secret-key-for-hs512-signing"}
				return cfg
			},
		},
		// RSA algorithms
		{
			name:      "RS256",
			algorithm: RS256,
			setupCfg: func() *JWTConfig {
				privateKey, publicKey := generateRSAKeyPair(t)
				cfg := NewJWTConfig()
				cfg.SigningAlgorithm = RS256
				cfg.CfgPrivateKey = &secure.KeyConfig{Key: string(privateKey)}
				cfg.CfgPublicKey = &secure.KeyConfig{Key: string(publicKey)}
				return cfg
			},
		},
		{
			name:      "RS384",
			algorithm: RS384,
			setupCfg: func() *JWTConfig {
				privateKey, publicKey := generateRSAKeyPair(t)
				cfg := NewJWTConfig()
				cfg.SigningAlgorithm = RS384
				cfg.CfgPrivateKey = &secure.KeyConfig{Key: string(privateKey)}
				cfg.CfgPublicKey = &secure.KeyConfig{Key: string(publicKey)}
				return cfg
			},
		},
		{
			name:      "RS512",
			algorithm: RS512,
			setupCfg: func() *JWTConfig {
				privateKey, publicKey := generateRSAKeyPair(t)
				cfg := NewJWTConfig()
				cfg.SigningAlgorithm = RS512
				cfg.CfgPrivateKey = &secure.KeyConfig{Key: string(privateKey)}
				cfg.CfgPublicKey = &secure.KeyConfig{Key: string(publicKey)}
				return cfg
			},
		},
		// ECDSA algorithms
		{
			name:      "ES256",
			algorithm: ES256,
			setupCfg: func() *JWTConfig {
				privateKey, publicKey := generateECDSAKeyPairForCurve(t, elliptic.P256())
				cfg := NewJWTConfig()
				cfg.SigningAlgorithm = ES256
				cfg.CfgPrivateKey = &secure.KeyConfig{Key: string(privateKey)}
				cfg.CfgPublicKey = &secure.KeyConfig{Key: string(publicKey)}
				return cfg
			},
		},
		{
			name:      "ES384",
			algorithm: ES384,
			setupCfg: func() *JWTConfig {
				privateKey, publicKey := generateECDSAKeyPairForCurve(t, elliptic.P384())
				cfg := NewJWTConfig()
				cfg.SigningAlgorithm = ES384
				cfg.CfgPrivateKey = &secure.KeyConfig{Key: string(privateKey)}
				cfg.CfgPublicKey = &secure.KeyConfig{Key: string(publicKey)}
				return cfg
			},
		},
		{
			name:      "ES512",
			algorithm: ES512,
			setupCfg: func() *JWTConfig {
				privateKey, publicKey := generateECDSAKeyPairForCurve(t, elliptic.P521())
				cfg := NewJWTConfig()
				cfg.SigningAlgorithm = ES512
				cfg.CfgPrivateKey = &secure.KeyConfig{Key: string(privateKey)}
				cfg.CfgPublicKey = &secure.KeyConfig{Key: string(publicKey)}
				return cfg
			},
		},
		// EdDSA algorithm
		{
			name:      "EdDSA",
			algorithm: EdDSA,
			setupCfg: func() *JWTConfig {
				privateKey, publicKey := generateEdDSAKeyPair(t)
				cfg := NewJWTConfig()
				cfg.SigningAlgorithm = EdDSA
				cfg.CfgPrivateKey = &secure.KeyConfig{Key: string(privateKey)}
				cfg.CfgPublicKey = &secure.KeyConfig{Key: string(publicKey)}
				return cfg
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.setupCfg()
			provider, err := NewProvider(cfg)
			require.NoError(t, err)

			// Generate token
			token, err := provider.GenerateToken(testSubject, testData)
			require.NoError(t, err)
			assert.NotEmpty(t, token)

			// Parse token
			claims, err := provider.ParseToken(token)
			require.NoError(t, err)
			assert.NotNil(t, claims)

			// Verify claims
			assert.Equal(t, testSubject, claims.Subject)
			assert.Equal(t, cfg.Issuer, claims.Issuer)
			assert.Contains(t, claims.Audience, cfg.Audience)
			assert.NotEmpty(t, claims.ID)

			// Verify custom data
			assert.Equal(t, testData["user_id"], claims.Data["user_id"])
			assert.Equal(t, testData["role"], claims.Data["role"])
			assert.Equal(t, testData["email"], claims.Data["email"])

			// Verify timing claims
			assert.True(t, claims.IssuedAt.Time.Before(time.Now().Add(time.Second)))
			assert.True(t, claims.NotBefore.Time.Before(time.Now().Add(time.Second)))
			assert.True(t, claims.ExpiresAt.Time.After(time.Now()))
		})
	}
}

// Test GenerateToken error cases
func TestGenerateToken_ErrorCases(t *testing.T) {
	tests := []struct {
		name     string
		setupCfg func() (*JWTConfig, JWTProvider)
		subject  string
		data     map[string]any
		wantErr  bool
	}{
		{
			name: "signing key GetBytes error",
			setupCfg: func() (*JWTConfig, JWTProvider) {
				cfg := NewJWTConfig()
				cfg.SigningAlgorithm = HS256
				cfg.validated = true
				cfg.signingMethod = jwt.GetSigningMethod(HS256)
				cfg.expiration = time.Hour
				cfg.signingKey = &mockSecret{err: errors.New("key error")}
				
				provider := &jwtProvider{cfg: cfg}
				return cfg, provider
			},
			subject: "test",
			wantErr: true,
		},
		{
			name: "private key GetBytes error for RSA",
			setupCfg: func() (*JWTConfig, JWTProvider) {
				cfg := NewJWTConfig()
				cfg.SigningAlgorithm = RS256
				cfg.validated = true
				cfg.signingMethod = jwt.GetSigningMethod(RS256)
				cfg.expiration = time.Hour
				cfg.privateKey = &mockSecret{err: errors.New("key error")}
				
				provider := &jwtProvider{cfg: cfg}
				return cfg, provider
			},
			subject: "test",
			wantErr: true,
		},
		{
			name: "invalid private key data for RSA",
			setupCfg: func() (*JWTConfig, JWTProvider) {
				cfg := NewJWTConfig()
				cfg.SigningAlgorithm = RS256
				cfg.validated = true
				cfg.signingMethod = jwt.GetSigningMethod(RS256)
				cfg.expiration = time.Hour
				cfg.privateKey = &mockSecret{data: []byte("invalid key data")}
				
				provider := &jwtProvider{cfg: cfg}
				return cfg, provider
			},
			subject: "test",
			wantErr: true,
		},
		{
			name: "EdDSA key error",
			setupCfg: func() (*JWTConfig, JWTProvider) {
				cfg := NewJWTConfig()
				cfg.SigningAlgorithm = EdDSA
				cfg.validated = true
				cfg.signingMethod = jwt.GetSigningMethod(EdDSA)
				cfg.expiration = time.Hour
				cfg.privateKey = &mockSecret{err: errors.New("key error")}
				
				provider := &jwtProvider{cfg: cfg}
				return cfg, provider
			},
			subject: "test",
			wantErr: true,
		},
		{
			name: "unsupported algorithm",
			setupCfg: func() (*JWTConfig, JWTProvider) {
				cfg := NewJWTConfig()
				cfg.SigningAlgorithm = "UNSUPPORTED"
				cfg.validated = true
				cfg.signingMethod = jwt.GetSigningMethod(HS256) // Won't match
				cfg.expiration = time.Hour
				
				provider := &jwtProvider{cfg: cfg}
				return cfg, provider
			},
			subject: "test",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, provider := tt.setupCfg()
			
			token, err := provider.GenerateToken(tt.subject, tt.data)
			
			if tt.wantErr {
				assert.Error(t, err)
				assert.Empty(t, token)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, token)
			}
		})
	}
}

// Test ParseToken error cases
func TestParseToken_ErrorCases(t *testing.T) {
	// Setup a valid provider for generating test tokens
	cfg := NewJWTConfig()
	cfg.SigningAlgorithm = HS256
	cfg.CfgSigningKey = &secure.DefaultCredentialConfig{Password: "test-secret"}
	validProvider, err := NewProvider(cfg)
	require.NoError(t, err)

	// Generate a valid token
	validToken, err := validProvider.GenerateToken("test", nil)
	require.NoError(t, err)

	// Generate expired token
	expiredCfg := NewJWTConfig()
	expiredCfg.SigningAlgorithm = HS256
	expiredCfg.CfgSigningKey = &secure.DefaultCredentialConfig{Password: "test-secret"}
	expiredCfg.ExpirationSeconds = 1
	expiredProvider, err := NewProvider(expiredCfg)
	require.NoError(t, err)
	expiredToken, err := expiredProvider.GenerateToken("test", nil)
	require.NoError(t, err)
	time.Sleep(2 * time.Second)

	tests := []struct {
		name        string
		token       string
		setupCfg    func() (*JWTConfig, JWTProvider)
		wantErr     bool
		errContains string
	}{
		{
			name:  "invalid token format",
			token: "invalid.token.format",
			setupCfg: func() (*JWTConfig, JWTProvider) {
				return cfg, validProvider
			},
			wantErr:     true,
			errContains: "invalid token",
		},
		{
			name:  "empty token",
			token: "",
			setupCfg: func() (*JWTConfig, JWTProvider) {
				return cfg, validProvider
			},
			wantErr:     true,
			errContains: "invalid token",
		},
		{
			name:  "expired token",
			token: expiredToken,
			setupCfg: func() (*JWTConfig, JWTProvider) {
				return expiredCfg, expiredProvider
			},
			wantErr:     true,
			errContains: "token has expired",
		},
		{
			name:  "wrong signing method",
			token: validToken,
			setupCfg: func() (*JWTConfig, JWTProvider) {
				wrongCfg := NewJWTConfig()
				wrongCfg.SigningAlgorithm = RS256
				privateKey, publicKey := generateRSAKeyPair(t)
				wrongCfg.CfgPrivateKey = &secure.KeyConfig{Key: string(privateKey)}
				wrongCfg.CfgPublicKey = &secure.KeyConfig{Key: string(publicKey)}
				wrongProvider, _ := NewProvider(wrongCfg)
				return wrongCfg, wrongProvider
			},
			wantErr:     true,
			errContains: "invalid token",
		},
		{
			name:  "signing key error",
			token: validToken,
			setupCfg: func() (*JWTConfig, JWTProvider) {
				errorCfg := NewJWTConfig()
				errorCfg.SigningAlgorithm = HS256
				errorCfg.validated = true
				errorCfg.signingMethod = jwt.GetSigningMethod(HS256)
				errorCfg.signingKey = &mockSecret{err: errors.New("key error")}
				
				errorProvider := &jwtProvider{cfg: errorCfg}
				return errorCfg, errorProvider
			},
			wantErr:     true,
			errContains: "invalid token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, provider := tt.setupCfg()
			
			claims, err := provider.ParseToken(tt.token)
			
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, claims)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, claims)
			}
		})
	}
}

// Test mandatory claims validation
func TestValidateMandatoryClaims(t *testing.T) {
	cfg := NewJWTConfig()
	cfg.SigningAlgorithm = HS256
	cfg.CfgSigningKey = &secure.DefaultCredentialConfig{Password: "test-secret"}
	cfg.Issuer = "test-issuer"
	cfg.Audience = "test-audience"
	cfg.RequireIssuer = true
	cfg.RequireAudience = true
	
	provider, err := NewProvider(cfg)
	require.NoError(t, err)
	p := provider.(*jwtProvider)

	tests := []struct {
		name    string
		claims  *Claims
		wantErr bool
		errType error
	}{
		{
			name: "valid claims",
			claims: &Claims{
				RegisteredClaims: jwt.RegisteredClaims{
					Issuer:   "test-issuer",
					Audience: jwt.ClaimStrings{"test-audience"},
				},
			},
			wantErr: false,
		},
		{
			name: "missing issuer",
			claims: &Claims{
				RegisteredClaims: jwt.RegisteredClaims{
					Audience: jwt.ClaimStrings{"test-audience"},
				},
			},
			wantErr: true,
			errType: ErrMissingIssuer,
		},
		{
			name: "wrong issuer",
			claims: &Claims{
				RegisteredClaims: jwt.RegisteredClaims{
					Issuer:   "wrong-issuer",
					Audience: jwt.ClaimStrings{"test-audience"},
				},
			},
			wantErr: true,
			errType: ErrMissingIssuer,
		},
		{
			name: "missing audience",
			claims: &Claims{
				RegisteredClaims: jwt.RegisteredClaims{
					Issuer: "test-issuer",
				},
			},
			wantErr: true,
			errType: ErrMissingAudience,
		},
		{
			name: "wrong audience",
			claims: &Claims{
				RegisteredClaims: jwt.RegisteredClaims{
					Issuer:   "test-issuer",
					Audience: jwt.ClaimStrings{"wrong-audience"},
				},
			},
			wantErr: true,
			errType: ErrMissingAudience,
		},
		{
			name: "audience in list",
			claims: &Claims{
				RegisteredClaims: jwt.RegisteredClaims{
					Issuer:   "test-issuer",
					Audience: jwt.ClaimStrings{"other-audience", "test-audience", "another-audience"},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := p.validateMandatoryClaims(tt.claims)
			
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errType != nil {
					assert.ErrorIs(t, err, tt.errType)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// Test optional validation
func TestValidateMandatoryClaims_Optional(t *testing.T) {
	cfg := NewJWTConfig()
	cfg.SigningAlgorithm = HS256
	cfg.CfgSigningKey = &secure.DefaultCredentialConfig{Password: "test-secret"}
	cfg.Issuer = "test-issuer"
	cfg.Audience = "test-audience"
	cfg.RequireIssuer = false
	cfg.RequireAudience = false
	
	provider, err := NewProvider(cfg)
	require.NoError(t, err)
	p := provider.(*jwtProvider)

	// Should not fail with missing claims when not required
	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{},
	}
	
	err = p.validateMandatoryClaims(claims)
	assert.NoError(t, err)
}

// Test Refresh functionality
func TestRefresh(t *testing.T) {
	cfg := NewJWTConfig()
	cfg.SigningAlgorithm = HS256
	cfg.CfgSigningKey = &secure.DefaultCredentialConfig{Password: "test-secret"}
	
	provider, err := NewProvider(cfg)
	require.NoError(t, err)
	p := provider.(*jwtProvider)

	// Generate initial token
	initialData := map[string]any{
		"user_id": "123",
		"role":    "admin",
	}
	initialToken, err := provider.GenerateToken("test-user", initialData)
	require.NoError(t, err)

	// Refresh the token
	refreshedToken, err := p.Refresh(initialToken)
	require.NoError(t, err)
	assert.NotEqual(t, initialToken, refreshedToken)

	// Parse refreshed token
	refreshedClaims, err := provider.ParseToken(refreshedToken)
	require.NoError(t, err)

	// Verify original data is preserved
	assert.Equal(t, "123", refreshedClaims.Data["user_id"])
	assert.Equal(t, "admin", refreshedClaims.Data["role"])

	// Verify rotation metadata
	assert.NotNil(t, refreshedClaims.Data["_rotated_at"])
	assert.Equal(t, float64(1), refreshedClaims.Data["_rotation_count"])

	// Refresh again to test counter increment
	secondRefresh, err := p.Refresh(refreshedToken)
	require.NoError(t, err)
	
	secondClaims, err := provider.ParseToken(secondRefresh)
	require.NoError(t, err)
	assert.Equal(t, float64(2), secondClaims.Data["_rotation_count"])
}

// Test getRotationCount helper
func TestGetRotationCount(t *testing.T) {
	tests := []struct {
		name     string
		data     map[string]any
		expected int
	}{
		{
			name:     "no rotation count",
			data:     map[string]any{},
			expected: 0,
		},
		{
			name:     "int rotation count",
			data:     map[string]any{"_rotation_count": 5},
			expected: 5,
		},
		{
			name:     "float64 rotation count (from JSON)",
			data:     map[string]any{"_rotation_count": float64(10)},
			expected: 10,
		},
		{
			name:     "invalid type",
			data:     map[string]any{"_rotation_count": "invalid"},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count := getRotationCount(tt.data)
			assert.Equal(t, tt.expected, count)
		})
	}
}

// Test revocation functionality
func TestRevocation(t *testing.T) {
	backend := NewMemoryRevocationBackend()
	revManager := NewRevocationManager(backend)
	
	cfg := NewJWTConfig()
	cfg.SigningAlgorithm = HS256
	cfg.CfgSigningKey = &secure.DefaultCredentialConfig{Password: "test-secret"}
	
	provider, err := NewProvider(cfg, WithRevocationManager(revManager))
	require.NoError(t, err)

	// Generate token
	token, err := provider.GenerateToken("test-user", nil)
	require.NoError(t, err)

	// Parse to get claims
	claims, err := provider.ParseToken(token)
	require.NoError(t, err)
	tokenID := claims.ID

	// Token should not be revoked initially
	assert.False(t, provider.IsTokenRevoked(tokenID))

	// Revoke token
	err = provider.RevokeToken(token)
	require.NoError(t, err)

	// Token should now be revoked
	assert.True(t, provider.IsTokenRevoked(tokenID))

	// Parsing revoked token should fail
	_, err = provider.ParseToken(token)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrTokenAlreadyRevoked)
}

// Test RevokeTokenByID
func TestRevokeTokenByID(t *testing.T) {
	backend := NewMemoryRevocationBackend()
	revManager := NewRevocationManager(backend)
	
	cfg := NewJWTConfig()
	cfg.SigningAlgorithm = HS256
	cfg.CfgSigningKey = &secure.DefaultCredentialConfig{Password: "test-secret"}
	
	provider, err := NewProvider(cfg, WithRevocationManager(revManager))
	require.NoError(t, err)

	// Revoke by ID
	tokenID := "test-token-id"
	expiresAt := time.Now().Add(time.Hour)
	
	err = provider.RevokeTokenByID(tokenID, expiresAt)
	require.NoError(t, err)

	// Check if revoked
	assert.True(t, provider.IsTokenRevoked(tokenID))
}

// Test revocation without manager
func TestRevocation_NoManager(t *testing.T) {
	cfg := NewJWTConfig()
	cfg.SigningAlgorithm = HS256
	cfg.CfgSigningKey = &secure.DefaultCredentialConfig{Password: "test-secret"}
	
	provider, err := NewProvider(cfg)
	require.NoError(t, err)

	// Generate token
	token, err := provider.GenerateToken("test-user", nil)
	require.NoError(t, err)

	// Revoke should fail without manager
	err = provider.RevokeToken(token)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrNoRevocationManager)

	// RevokeByID should fail without manager
	err = provider.RevokeTokenByID("test-id", time.Now())
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrNoRevocationManager)

	// IsTokenRevoked should return false without manager
	assert.False(t, provider.IsTokenRevoked("test-id"))
}

// Test with different key configurations
func TestKeyConfigurations(t *testing.T) {
	t.Run("KeyID header", func(t *testing.T) {
		cfg := NewJWTConfig()
		cfg.SigningAlgorithm = HS256
		cfg.CfgSigningKey = &secure.DefaultCredentialConfig{Password: "test-secret"}
		cfg.KeyID = "test-key-id"
		
		provider, err := NewProvider(cfg)
		require.NoError(t, err)

		token, err := provider.GenerateToken("test", nil)
		require.NoError(t, err)

		// Parse raw token to check header
		parsedToken, _, err := jwt.NewParser().ParseUnverified(token, &Claims{})
		require.NoError(t, err)
		
		assert.Equal(t, "test-key-id", parsedToken.Header["kid"])
	})

	t.Run("No KeyID", func(t *testing.T) {
		cfg := NewJWTConfig()
		cfg.SigningAlgorithm = HS256
		cfg.CfgSigningKey = &secure.DefaultCredentialConfig{Password: "test-secret"}
		cfg.KeyID = ""
		
		provider, err := NewProvider(cfg)
		require.NoError(t, err)

		token, err := provider.GenerateToken("test", nil)
		require.NoError(t, err)

		// Parse raw token to check header
		parsedToken, _, err := jwt.NewParser().ParseUnverified(token, &Claims{})
		require.NoError(t, err)
		
		_, hasKID := parsedToken.Header["kid"]
		assert.False(t, hasKID)
	})
}

// Test edge cases
func TestEdgeCases(t *testing.T) {
	t.Run("nil custom data", func(t *testing.T) {
		cfg := NewJWTConfig()
		cfg.SigningAlgorithm = HS256
		cfg.CfgSigningKey = &secure.DefaultCredentialConfig{Password: "test-secret"}
		
		provider, err := NewProvider(cfg)
		require.NoError(t, err)

		token, err := provider.GenerateToken("test", nil)
		require.NoError(t, err)

		claims, err := provider.ParseToken(token)
		require.NoError(t, err)
		// Data can be nil or empty when no custom data is provided
		if claims.Data != nil {
			assert.Empty(t, claims.Data)
		}
	})

	t.Run("empty token ID in revocation", func(t *testing.T) {
		backend := NewMemoryRevocationBackend()
		revManager := NewRevocationManager(backend)
		
		cfg := NewJWTConfig()
		cfg.SigningAlgorithm = HS256
		cfg.CfgSigningKey = &secure.DefaultCredentialConfig{Password: "test-secret"}
		
		provider, err := NewProvider(cfg, WithRevocationManager(revManager))
		require.NoError(t, err)
		p := provider.(*jwtProvider)

		p.cfg.validated = true
		
		// This should fail due to parse error (mock-token is not valid JWT)
		err = p.RevokeToken("mock-token")
		assert.Error(t, err)
	})

	t.Run("zero expiration time in revocation", func(t *testing.T) {
		backend := NewMemoryRevocationBackend()
		revManager := NewRevocationManager(backend)
		
		cfg := NewJWTConfig()
		cfg.SigningAlgorithm = HS256
		cfg.CfgSigningKey = &secure.DefaultCredentialConfig{Password: "test-secret"}
		cfg.ExpirationSeconds = 3600
		
		provider, err := NewProvider(cfg, WithRevocationManager(revManager))
		require.NoError(t, err)

		// Generate token
		token, err := provider.GenerateToken("test", nil)
		require.NoError(t, err)

		// Parse to get claims and manually set zero expiration
		originalClaims, err := provider.ParseToken(token)
		require.NoError(t, err)
		
		// Create new claims with zero expiration
		claims := &Claims{
			RegisteredClaims: jwt.RegisteredClaims{
				Issuer:    originalClaims.Issuer,
				Subject:   originalClaims.Subject,
				Audience:  originalClaims.Audience,
				IssuedAt:  originalClaims.IssuedAt,
				NotBefore: originalClaims.NotBefore,
				ID:        originalClaims.ID,
				// ExpiresAt is intentionally nil
			},
			Data: originalClaims.Data,
		}
		
		// Create new token with modified claims
		newToken := jwt.NewWithClaims(jwt.GetSigningMethod(HS256), claims)
		key, _ := cfg.signingKey.GetBytes()
		tokenString, _ := newToken.SignedString(key)

		// Revoke should still work with far future date
		err = provider.RevokeToken(tokenString)
		assert.NoError(t, err)
	})
}

// Mock JWT provider for testing edge cases
type mockJWTProvider struct {
	claims *Claims
	err    error
}

func (m *mockJWTProvider) ParseToken(tokenString string) (*Claims, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.claims, nil
}

func (m *mockJWTProvider) GenerateToken(subject string, data map[string]any) (string, error) {
	return "", nil
}

func (m *mockJWTProvider) RevokeToken(tokenString string) error {
	return nil
}

func (m *mockJWTProvider) RevokeTokenByID(tokenID string, expiresAt time.Time) error {
	return nil
}

func (m *mockJWTProvider) IsTokenRevoked(tokenID string) bool {
	return false
}

func (m *mockJWTProvider) GetRevocationManager() *RevocationManager {
	return nil
}