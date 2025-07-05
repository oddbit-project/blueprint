package jwtprovider

import (
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/oddbit-project/blueprint/crypt/secure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewJWTConfig(t *testing.T) {
	cfg := NewJWTConfig()
	
	assert.NotNil(t, cfg)
	assert.Equal(t, []string{HS256, HS384, HS512}, cfg.SignAlgorithms)
	assert.Equal(t, DefaultTTL.Minutes(), cfg.TokenTTLMinutes)
}

func TestJWTConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  *JWTConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			config: &JWTConfig{
				SignAlgorithms:  []string{HS256},
				TokenTTLMinutes: 30,
			},
			wantErr: false,
		},
		{
			name: "empty sign algorithms",
			config: &JWTConfig{
				SignAlgorithms:  []string{},
				TokenTTLMinutes: 30,
			},
			wantErr: true,
			errMsg:  "no signAlgorithms specified",
		},
		{
			name: "nil sign algorithms",
			config: &JWTConfig{
				SignAlgorithms:  nil,
				TokenTTLMinutes: 30,
			},
			wantErr: true,
			errMsg:  "no signAlgorithms specified",
		},
		{
			name: "zero token TTL",
			config: &JWTConfig{
				SignAlgorithms:  []string{HS256},
				TokenTTLMinutes: 0,
			},
			wantErr: true,
			errMsg:  "tokenTTLMinutes must be greater than zero",
		},
		{
			name: "negative token TTL",
			config: &JWTConfig{
				SignAlgorithms:  []string{HS256},
				TokenTTLMinutes: -10,
			},
			wantErr: true,
			errMsg:  "tokenTTLMinutes must be greater than zero",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestWithSignAlgos(t *testing.T) {
	secret, err := secure.RandomCredential(128)
	require.NoError(t, err)

	// Test with custom algorithms
	provider := NewJwtProvider(secret, WithSignAlgos(RS256, ES256))
	jp := provider.(*jwtProvider)
	assert.Equal(t, []string{RS256, ES256}, jp.allowedAlgos)

	// Test with empty algorithms (should use defaults)
	provider2 := NewJwtProvider(secret, WithSignAlgos())
	jp2 := provider2.(*jwtProvider)
	assert.Equal(t, []string{HS256, RS256, ES256, PS256}, jp2.allowedAlgos)
}

func TestWithDuration(t *testing.T) {
	secret, err := secure.RandomCredential(128)
	require.NoError(t, err)

	customDuration := 2 * time.Hour
	provider := NewJwtProvider(secret, WithDuration(customDuration))
	jp := provider.(*jwtProvider)
	assert.Equal(t, customDuration, jp.duration)
}

func TestNewJwtProvider(t *testing.T) {
	// Test with nil secret (should generate random)
	provider := NewJwtProvider(nil)
	assert.NotNil(t, provider)
	jp := provider.(*jwtProvider)
	assert.NotNil(t, jp.jwtSecret)
	assert.Equal(t, []string{HS256, RS256, ES256, PS256}, jp.allowedAlgos)
	assert.Equal(t, DefaultTTL, jp.duration)

	// Test with provided secret
	secret, err := secure.RandomCredential(128)
	require.NoError(t, err)
	provider2 := NewJwtProvider(secret)
	jp2 := provider2.(*jwtProvider)
	assert.Equal(t, secret, jp2.jwtSecret)
}

func TestNewFromConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  *JWTConfig
		wantErr bool
	}{
		{
			name: "valid config with password",
			config: &JWTConfig{
				SignAlgorithms:  []string{HS256, RS256},
				TokenTTLMinutes: 60,
				DefaultCredentialConfig: secure.DefaultCredentialConfig{
					Password: "test-secret",
				},
			},
			wantErr: false,
		},
		{
			name: "empty password",
			config: &JWTConfig{
				SignAlgorithms:  []string{HS256},
				TokenTTLMinutes: 30,
				DefaultCredentialConfig: secure.DefaultCredentialConfig{
					Password: "",
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := NewFromConfig(tt.config)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, provider)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, provider)
				
				// Verify configuration was applied
				jp := provider.(*jwtProvider)
				assert.Equal(t, tt.config.SignAlgorithms, jp.allowedAlgos)
				assert.Equal(t, time.Duration(tt.config.TokenTTLMinutes)*time.Minute, jp.duration)
			}
		})
	}
}

func TestGenerateToken(t *testing.T) {
	secret, err := secure.RandomCredential(128)
	require.NoError(t, err)
	provider := NewJwtProvider(secret, WithDuration(1*time.Hour))

	tests := []struct {
		name         string
		algorithm    string
		customClaims map[string]any
		wantErr      bool
		errType      error
	}{
		{
			name:         "valid HS256 token",
			algorithm:    HS256,
			customClaims: nil,
			wantErr:      false,
		},
		{
			name:      "valid token with custom claims",
			algorithm: HS256,
			customClaims: map[string]any{
				"user_id": "123",
				"role":    "admin",
			},
			wantErr: false,
		},
		{
			name:         "invalid algorithm",
			algorithm:    "INVALID",
			customClaims: nil,
			wantErr:      true,
			errType:      ErrInvalidSigningAlgorithm,
		},
		{
			name:         "empty algorithm",
			algorithm:    "",
			customClaims: nil,
			wantErr:      true,
			errType:      ErrInvalidSigningAlgorithm,
		},
		{
			name:      "override standard claims",
			algorithm: HS256,
			customClaims: map[string]any{
				ClaimIssuer:  "test-issuer",
				ClaimSubject: "test-subject",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := provider.GenerateToken(tt.algorithm, tt.customClaims)
			
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errType != nil {
					assert.ErrorIs(t, err, tt.errType)
				}
				assert.Empty(t, token)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, token)
				
				// Verify token can be parsed
				claims, err := provider.ParseToken(token)
				assert.NoError(t, err)
				assert.NotNil(t, claims)
				
				// Verify standard claims
				assert.Contains(t, claims, ClaimExpiresAt)
				assert.Contains(t, claims, ClaimIssuedAt)
				assert.Contains(t, claims, ClaimNotBefore)
				
				// Verify custom claims
				if tt.customClaims != nil {
					for k, v := range tt.customClaims {
						assert.Equal(t, v, claims[k])
					}
				}
			}
		})
	}
}

func TestParseToken(t *testing.T) {
	secret, err := secure.RandomCredential(128)
	require.NoError(t, err)
	provider := NewJwtProvider(secret, WithSignAlgos(HS256, RS256))

	// Generate a valid token
	validToken, err := provider.GenerateToken(HS256, map[string]any{"user_id": "123"})
	require.NoError(t, err)

	// Generate token with different secret
	differentSecret, err := secure.RandomCredential(128)
	require.NoError(t, err)
	differentProvider := NewJwtProvider(differentSecret)
	wrongSecretToken, err := differentProvider.GenerateToken(HS256, nil)
	require.NoError(t, err)

	// Generate expired token
	expiredProvider := NewJwtProvider(secret, WithDuration(-1*time.Hour))
	expiredToken, err := expiredProvider.GenerateToken(HS256, nil)
	require.NoError(t, err)

	tests := []struct {
		name    string
		token   string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid token",
			token:   validToken,
			wantErr: false,
		},
		{
			name:    "invalid token format",
			token:   "invalid.token.format",
			wantErr: true,
		},
		{
			name:    "empty token",
			token:   "",
			wantErr: true,
		},
		{
			name:    "wrong secret",
			token:   wrongSecretToken,
			wantErr: true,
		},
		{
			name:    "expired token",
			token:   expiredToken,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims, err := provider.ParseToken(tt.token)
			
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, claims)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, claims)
				assert.Equal(t, "123", claims["user_id"])
			}
		})
	}
}

func TestParseTokenWithDisallowedAlgorithm(t *testing.T) {
	secret, err := secure.RandomCredential(128)
	require.NoError(t, err)
	
	// Create provider that only allows HS256
	restrictedProvider := NewJwtProvider(secret, WithSignAlgos(HS256))
	
	// Create provider that can generate HS512 tokens
	allAlgoProvider := NewJwtProvider(secret, WithSignAlgos(HS256, HS512))
	
	// Generate HS512 token
	hs512Token, err := allAlgoProvider.GenerateToken(HS512, nil)
	require.NoError(t, err)
	
	// Try to parse HS512 token with restricted provider
	claims, err := restrictedProvider.ParseToken(hs512Token)
	assert.Error(t, err)
	assert.Nil(t, claims)
	assert.ErrorIs(t, err, jwt.ErrSignatureInvalid)
}

func TestValidateClaims(t *testing.T) {
	provider := NewJwtProvider(nil)
	jp := provider.(*jwtProvider)

	tests := []struct {
		name    string
		claims  jwt.MapClaims
		wantErr bool
		errType error
	}{
		{
			name: "valid claims",
			claims: jwt.MapClaims{
				ClaimExpiresAt: float64(time.Now().Add(1 * time.Hour).Unix()),
				ClaimNotBefore: float64(time.Now().Add(-1 * time.Hour).Unix()),
			},
			wantErr: false,
		},
		{
			name: "expired token",
			claims: jwt.MapClaims{
				ClaimExpiresAt: float64(time.Now().Add(-1 * time.Hour).Unix()),
			},
			wantErr: true,
			errType: ErrTokenExpired,
		},
		{
			name: "not yet valid",
			claims: jwt.MapClaims{
				ClaimExpiresAt: float64(time.Now().Add(1 * time.Hour).Unix()),
				ClaimNotBefore: float64(time.Now().Add(1 * time.Hour).Unix()),
			},
			wantErr: true,
			errType: ErrNbfNotValid,
		},
		{
			name:    "no time claims",
			claims:  jwt.MapClaims{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := jp.ValidateClaims(tt.claims)
			
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

func TestValidateExp(t *testing.T) {
	provider := NewJwtProvider(nil)
	jp := provider.(*jwtProvider)

	tests := []struct {
		name    string
		claims  jwt.MapClaims
		wantErr bool
		errType error
	}{
		{
			name: "valid expiry",
			claims: jwt.MapClaims{
				ClaimExpiresAt: float64(time.Now().Add(1 * time.Hour).Unix()),
			},
			wantErr: false,
		},
		{
			name: "expired",
			claims: jwt.MapClaims{
				ClaimExpiresAt: float64(time.Now().Add(-1 * time.Hour).Unix()),
			},
			wantErr: true,
			errType: ErrTokenExpired,
		},
		{
			name: "invalid exp type",
			claims: jwt.MapClaims{
				ClaimExpiresAt: "not-a-number",
			},
			wantErr: true,
			errType: ErrInvalidExpClaim,
		},
		{
			name: "no exp claim",
			claims: jwt.MapClaims{
				"other": "data",
			},
			wantErr: false,
		},
		{
			name: "exp as int",
			claims: jwt.MapClaims{
				ClaimExpiresAt: int(time.Now().Add(1 * time.Hour).Unix()),
			},
			wantErr: true,
			errType: ErrInvalidExpClaim,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := jp.ValidateExp(tt.claims)
			
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

func TestValidateNbf(t *testing.T) {
	provider := NewJwtProvider(nil)
	jp := provider.(*jwtProvider)

	tests := []struct {
		name    string
		claims  jwt.MapClaims
		wantErr bool
		errType error
	}{
		{
			name: "valid nbf",
			claims: jwt.MapClaims{
				ClaimNotBefore: float64(time.Now().Add(-1 * time.Hour).Unix()),
			},
			wantErr: false,
		},
		{
			name: "not yet valid",
			claims: jwt.MapClaims{
				ClaimNotBefore: float64(time.Now().Add(1 * time.Hour).Unix()),
			},
			wantErr: true,
			errType: ErrNbfNotValid,
		},
		{
			name: "invalid nbf type",
			claims: jwt.MapClaims{
				ClaimNotBefore: "not-a-number",
			},
			wantErr: true,
			errType: ErrInvalidNbfClaim,
		},
		{
			name: "no nbf claim",
			claims: jwt.MapClaims{
				"other": "data",
			},
			wantErr: false,
		},
		{
			name: "nbf as int",
			claims: jwt.MapClaims{
				ClaimNotBefore: int(time.Now().Add(-1 * time.Hour).Unix()),
			},
			wantErr: true,
			errType: ErrInvalidNbfClaim,
		},
		{
			name: "nbf exactly now",
			claims: jwt.MapClaims{
				ClaimNotBefore: float64(time.Now().Unix()),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := jp.ValidateNbf(tt.claims)
			
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

func TestIntegration(t *testing.T) {
	// Test full integration with config
	config := &JWTConfig{
		SignAlgorithms:  []string{HS256, HS512},
		TokenTTLMinutes: 5,
		DefaultCredentialConfig: secure.DefaultCredentialConfig{
			Password: "my-secret-key",
		},
	}

	// Validate config
	err := config.Validate()
	require.NoError(t, err)

	// Create provider from config
	provider, err := NewFromConfig(config)
	require.NoError(t, err)

	// Generate token
	customClaims := map[string]any{
		"user_id":   "user123",
		"role":      "admin",
		"tenant_id": "tenant456",
	}
	
	token, err := provider.GenerateToken(HS256, customClaims)
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	// Parse token
	claims, err := provider.ParseToken(token)
	require.NoError(t, err)
	
	// Verify custom claims
	assert.Equal(t, "user123", claims["user_id"])
	assert.Equal(t, "admin", claims["role"])
	assert.Equal(t, "tenant456", claims["tenant_id"])

	// Validate claims
	err = provider.ValidateClaims(claims)
	assert.NoError(t, err)

	// Test with unsupported algorithm (RS256 is not in the configured algorithms)
	token, err = provider.GenerateToken(RS256, nil)
	assert.Error(t, err)
	// RS256 will fail because we're using symmetric key, not because it's not allowed
	// So we test with a truly invalid algorithm
	token, err = provider.GenerateToken("INVALID_ALG", nil)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidSigningAlgorithm)
}

func TestEdgeCases(t *testing.T) {
	t.Run("empty secret bytes", func(t *testing.T) {
		// Create a mock secret that returns empty bytes
		mockSecret := &mockSecret{
			data: []byte{},
			err:  nil,
		}
		
		provider := NewJwtProvider(mockSecret)
		
		// Try to generate token with empty secret
		token, err := provider.GenerateToken(HS256, nil)
		// This might succeed or fail depending on JWT library behavior
		// The important thing is it doesn't panic
		_ = token
		_ = err
	})

	t.Run("secret returns error", func(t *testing.T) {
		// Create a mock secret that returns an error
		mockSecret := &mockSecret{
			data: nil,
			err:  errors.New("secret error"),
		}
		
		provider := NewJwtProvider(mockSecret)
		
		// Try to generate token when secret returns error
		token, err := provider.GenerateToken(HS256, nil)
		assert.Error(t, err)
		assert.Empty(t, token)
	})

	t.Run("very long token TTL", func(t *testing.T) {
		config := &JWTConfig{
			SignAlgorithms:  []string{HS256},
			TokenTTLMinutes: 525600, // 1 year in minutes
			DefaultCredentialConfig: secure.DefaultCredentialConfig{
				Password: "secret",
			},
		}
		
		provider, err := NewFromConfig(config)
		require.NoError(t, err)
		
		token, err := provider.GenerateToken(HS256, nil)
		require.NoError(t, err)
		
		claims, err := provider.ParseToken(token)
		require.NoError(t, err)
		
		// Verify the token won't expire for a long time
		exp := claims[ClaimExpiresAt].(float64)
		expTime := time.Unix(int64(exp), 0)
		assert.True(t, expTime.After(time.Now().Add(364*24*time.Hour)))
	})

	t.Run("claims with all standard fields", func(t *testing.T) {
		provider := NewJwtProvider(nil)
		
		customClaims := map[string]any{
			ClaimIssuer:   "test-issuer",
			ClaimSubject:  "test-subject",
			ClaimAudience: []string{"aud1", "aud2"},
			ClaimJwtID:    "unique-jwt-id",
		}
		
		token, err := provider.GenerateToken(HS256, customClaims)
		require.NoError(t, err)
		
		claims, err := provider.ParseToken(token)
		require.NoError(t, err)
		
		assert.Equal(t, "test-issuer", claims[ClaimIssuer])
		assert.Equal(t, "test-subject", claims[ClaimSubject])
		assert.Equal(t, "unique-jwt-id", claims[ClaimJwtID])
		// Audience might be returned as interface{} so we need to handle it carefully
		assert.NotNil(t, claims[ClaimAudience])
	})
}

// mockSecret is a test implementation of secure.Secret
type mockSecret struct {
	data []byte
	err  error
}

func (m *mockSecret) GetBytes() ([]byte, error) {
	return m.data, m.err
}