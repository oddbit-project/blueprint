package jwt

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/oddbit-project/blueprint/log"
	"github.com/oddbit-project/blueprint/provider/httpserver/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRandomJWTKey(t *testing.T) {
	key1 := RandomJWTKey()
	key2 := RandomJWTKey()

	assert.Len(t, key1, 128, "JWT key should be 128 bytes")
	assert.Len(t, key2, 128, "JWT key should be 128 bytes")
	assert.NotEqual(t, key1, key2, "Random keys should be different")
}

func TestNewJWTConfig(t *testing.T) {
	key := []byte("test-signing-key-32-bytes-long!!")
	config := NewJWTConfig(key)

	assert.Equal(t, key, config.SigningKey)
	assert.Equal(t, "HS256", config.SigningAlgorithm)
	assert.Equal(t, 86400, config.ExpirationSeconds)
	assert.Equal(t, "blueprint", config.Issuer)
	assert.Equal(t, "api", config.Audience)
	assert.Equal(t, "default", config.KeyID)
	assert.True(t, config.RequireIssuer)
	assert.True(t, config.RequireAudience)
}

func TestJWTConfigValidate(t *testing.T) {
	t.Run("Valid HMAC config", func(t *testing.T) {
		config := NewJWTConfig([]byte("test-key-32-bytes-long-enough!!"))
		err := config.Validate()
		assert.NoError(t, err)
		assert.Equal(t, jwt.SigningMethodHS256, config.SigningMethod)
		assert.Equal(t, 86400*time.Second, config.Expiration)
	})

	t.Run("Missing HMAC signing key", func(t *testing.T) {
		config := NewJWTConfig(nil)
		err := config.Validate()
		assert.Error(t, err)
		assert.Equal(t, ErrJWTSigningKey, err)
	})

	t.Run("Invalid signing algorithm", func(t *testing.T) {
		config := NewJWTConfig([]byte("test-key"))
		config.SigningAlgorithm = "INVALID"
		err := config.Validate()
		assert.Error(t, err)
		assert.Equal(t, ErrInvalidSigningAlgorithm, err)
	})

	t.Run("Zero expiration", func(t *testing.T) {
		config := NewJWTConfig([]byte("test-key-32-bytes-long-enough!!"))
		config.ExpirationSeconds = 0
		err := config.Validate()
		assert.Error(t, err)
	})

	t.Run("Valid RSA config", func(t *testing.T) {
		privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
		require.NoError(t, err)

		config := NewJWTConfig(nil)
		config.SigningAlgorithm = "RS256"
		config.PrivateKey = privateKey
		err = config.Validate()
		assert.NoError(t, err)
		assert.Equal(t, &privateKey.PublicKey, config.PublicKey)
	})

	t.Run("RSA missing private key", func(t *testing.T) {
		config := NewJWTConfig(nil)
		config.SigningAlgorithm = "RS256"
		err := config.Validate()
		assert.Error(t, err)
		assert.Equal(t, ErrJWTSigningKey, err)
	})

	t.Run("RSA invalid key type", func(t *testing.T) {
		config := NewJWTConfig(nil)
		config.SigningAlgorithm = "RS256"
		config.PrivateKey = "not-a-key"
		err := config.Validate()
		assert.Error(t, err)
		assert.Equal(t, ErrInvalidKeyType, err)
	})

	t.Run("Valid ECDSA config", func(t *testing.T) {
		privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		require.NoError(t, err)

		config := NewJWTConfig(nil)
		config.SigningAlgorithm = "ES256"
		config.PrivateKey = privateKey
		err = config.Validate()
		assert.NoError(t, err)
		assert.Equal(t, &privateKey.PublicKey, config.PublicKey)
	})

	t.Run("Valid EdDSA config", func(t *testing.T) {
		_, privateKey, err := ed25519.GenerateKey(rand.Reader)
		require.NoError(t, err)

		config := NewJWTConfig(nil)
		config.SigningAlgorithm = "EdDSA"
		config.PrivateKey = privateKey
		err = config.Validate()
		assert.NoError(t, err)
		assert.Equal(t, ed25519.PublicKey(privateKey[32:]), config.PublicKey)
	})
}

func TestNewJWTManager(t *testing.T) {
	logger := log.New("test")

	t.Run("With valid config", func(t *testing.T) {
		config := NewJWTConfig([]byte("test-key-32-bytes-long-enough!!"))
		manager, err := NewJWTManager(config, logger)
		assert.NoError(t, err)
		assert.NotNil(t, manager)
		assert.Equal(t, config, manager.config)
		assert.Equal(t, logger, manager.logger)
		assert.NotNil(t, manager.revocationManager)
		assert.NotNil(t, manager.jwksManager)
	})

	t.Run("With nil config", func(t *testing.T) {
		manager, err := NewJWTManager(nil, logger)
		assert.NoError(t, err)
		assert.NotNil(t, manager)
		assert.NotNil(t, manager.config)
	})

	t.Run("With invalid config", func(t *testing.T) {
		config := NewJWTConfig(nil) // Invalid - no signing key
		manager, err := NewJWTManager(config, logger)
		assert.Error(t, err)
		assert.Nil(t, manager)
	})
}

func TestNewJWTManagerWithRevocation(t *testing.T) {
	logger := log.New("test")
	config := NewJWTConfig([]byte("test-key-32-bytes-long-enough!!"))
	revocationManager := NewRevocationManager(NewMemoryRevocationBackend())

	manager, err := NewJWTManagerWithRevocation(config, logger, revocationManager)
	assert.NoError(t, err)
	assert.NotNil(t, manager)
	assert.Equal(t, revocationManager, manager.revocationManager)
}

func TestJWTManagerGenerate(t *testing.T) {
	logger := log.New("test")
	config := NewJWTConfig([]byte("test-key-32-bytes-long-enough!!"))
	manager, err := NewJWTManager(config, logger)
	require.NoError(t, err)

	sessionData := &session.SessionData{
		Values: map[string]interface{}{
			"user_id": "test-user",
			"role":    "admin",
		},
		LastAccessed: time.Now(),
		Created:      time.Now(),
		ID:           "session-123",
	}

	token, err := manager.Generate("session-123", sessionData)
	assert.NoError(t, err)
	assert.NotEmpty(t, token)

	// Verify we can parse the token
	claims, err := manager.Validate(token)
	assert.NoError(t, err)
	assert.Equal(t, "session-123", claims.Subject)
	assert.Equal(t, "test-user", claims.Data["user_id"])
	assert.Equal(t, "admin", claims.Data["role"])
	assert.Equal(t, "blueprint", claims.Issuer)
	assert.Contains(t, claims.Audience, "api")
	assert.NotEmpty(t, claims.ID)
}

func TestJWTManagerValidate(t *testing.T) {
	logger := log.New("test")
	config := NewJWTConfig([]byte("test-key-32-bytes-long-enough!!"))
	manager, err := NewJWTManager(config, logger)
	require.NoError(t, err)

	sessionData := &session.SessionData{
		Values: map[string]interface{}{"user_id": "test-user"},
		ID:     "session-123",
	}

	t.Run("Valid token", func(t *testing.T) {
		token, err := manager.Generate("session-123", sessionData)
		require.NoError(t, err)

		claims, err := manager.Validate(token)
		assert.NoError(t, err)
		assert.Equal(t, "session-123", claims.Subject)
	})

	t.Run("Invalid token", func(t *testing.T) {
		claims, err := manager.Validate("invalid.token.here")
		assert.Error(t, err)
		assert.Nil(t, claims)
		assert.Equal(t, ErrJWTInvalid, err)
	})

	t.Run("Token with wrong signing key", func(t *testing.T) {
		wrongConfig := NewJWTConfig([]byte("wrong-key-32-bytes-long-enough!"))
		wrongManager, err := NewJWTManager(wrongConfig, logger)
		require.NoError(t, err)

		token, err := wrongManager.Generate("session-123", sessionData)
		require.NoError(t, err)

		// Try to validate with original manager
		claims, err := manager.Validate(token)
		assert.Error(t, err)
		assert.Nil(t, claims)
		assert.Equal(t, ErrJWTInvalid, err)
	})

	t.Run("Expired token", func(t *testing.T) {
		// Create config with very short expiration
		shortConfig := NewJWTConfig([]byte("test-key-32-bytes-long-enough!!"))
		shortConfig.ExpirationSeconds = 1
		shortManager, err := NewJWTManager(shortConfig, logger)
		require.NoError(t, err)

		token, err := shortManager.Generate("session-123", sessionData)
		require.NoError(t, err)

		// Wait for expiration
		time.Sleep(2 * time.Second)

		claims, err := shortManager.Validate(token)
		assert.Error(t, err)
		assert.Nil(t, claims)
		assert.Equal(t, ErrJWTExpired, err)
	})
}

func TestJWTManagerValidateMandatoryClaims(t *testing.T) {
	logger := log.New("test")

	t.Run("Missing issuer", func(t *testing.T) {
		config := NewJWTConfig([]byte("test-key-32-bytes-long-enough!!"))
		config.RequireIssuer = true
		config.Issuer = "expected-issuer"
		manager, err := NewJWTManager(config, logger)
		require.NoError(t, err)

		claims := &Claims{
			RegisteredClaims: jwt.RegisteredClaims{
				Issuer: "", // Missing issuer
			},
		}

		err = manager.validateMandatoryClaims(claims)
		assert.Error(t, err)
		assert.Equal(t, ErrMissingIssuer, err)
	})

	t.Run("Wrong issuer", func(t *testing.T) {
		config := NewJWTConfig([]byte("test-key-32-bytes-long-enough!!"))
		config.RequireIssuer = true
		config.Issuer = "expected-issuer"
		manager, err := NewJWTManager(config, logger)
		require.NoError(t, err)

		claims := &Claims{
			RegisteredClaims: jwt.RegisteredClaims{
				Issuer: "wrong-issuer",
			},
		}

		err = manager.validateMandatoryClaims(claims)
		assert.Error(t, err)
		assert.Equal(t, ErrMissingIssuer, err)
	})

	t.Run("Missing audience", func(t *testing.T) {
		config := NewJWTConfig([]byte("test-key-32-bytes-long-enough!!"))
		config.RequireIssuer = false // Disable issuer validation to test audience
		config.RequireAudience = true
		config.Audience = "expected-audience"
		manager, err := NewJWTManager(config, logger)
		require.NoError(t, err)

		claims := &Claims{
			RegisteredClaims: jwt.RegisteredClaims{
				Issuer:   "expected-issuer",
				Audience: nil, // Missing audience
			},
		}

		err = manager.validateMandatoryClaims(claims)
		assert.Error(t, err)
		assert.Equal(t, ErrMissingAudience, err)
	})

	t.Run("Wrong audience", func(t *testing.T) {
		config := NewJWTConfig([]byte("test-key-32-bytes-long-enough!!"))
		config.RequireIssuer = false // Disable issuer validation to test audience
		config.RequireAudience = true
		config.Audience = "expected-audience"
		manager, err := NewJWTManager(config, logger)
		require.NoError(t, err)

		claims := &Claims{
			RegisteredClaims: jwt.RegisteredClaims{
				Issuer:   "expected-issuer",
				Audience: jwt.ClaimStrings{"wrong-audience"},
			},
		}

		err = manager.validateMandatoryClaims(claims)
		assert.Error(t, err)
		assert.Equal(t, ErrMissingAudience, err)
	})

	t.Run("Valid claims", func(t *testing.T) {
		config := NewJWTConfig([]byte("test-key-32-bytes-long-enough!!"))
		config.RequireIssuer = true
		config.RequireAudience = true
		config.Issuer = "expected-issuer"
		config.Audience = "expected-audience"
		manager, err := NewJWTManager(config, logger)
		require.NoError(t, err)

		claims := &Claims{
			RegisteredClaims: jwt.RegisteredClaims{
				Issuer:   "expected-issuer",
				Audience: jwt.ClaimStrings{"expected-audience", "other-audience"},
			},
		}

		err = manager.validateMandatoryClaims(claims)
		assert.NoError(t, err)
	})
}

func TestJWTManagerRefresh(t *testing.T) {
	logger := log.New("test")
	config := NewJWTConfig([]byte("test-key-32-bytes-long-enough!!"))
	manager, err := NewJWTManager(config, logger)
	require.NoError(t, err)

	sessionData := &session.SessionData{
		Values: map[string]interface{}{"user_id": "test-user"},
		ID:     "session-123",
	}

	originalToken, err := manager.Generate("session-123", sessionData)
	require.NoError(t, err)

	t.Run("Valid refresh", func(t *testing.T) {
		newToken, err := manager.Refresh(originalToken)
		assert.NoError(t, err)
		assert.NotEmpty(t, newToken)
		assert.NotEqual(t, originalToken, newToken)

		// Verify new token is valid
		claims, err := manager.Validate(newToken)
		assert.NoError(t, err)
		assert.Equal(t, "session-123", claims.Subject)

		// Check rotation metadata
		assert.Contains(t, claims.Data, "_rotation_count")
		assert.Contains(t, claims.Data, "_rotated_at")
	})

	t.Run("Refresh invalid token", func(t *testing.T) {
		newToken, err := manager.Refresh("invalid.token.here")
		assert.Error(t, err)
		assert.Empty(t, newToken)
	})
}

func TestJWTManagerGet(t *testing.T) {
	logger := log.New("test")
	config := NewJWTConfig([]byte("test-key-32-bytes-long-enough!!"))
	manager, err := NewJWTManager(config, logger)
	require.NoError(t, err)

	sessionData := &session.SessionData{
		Values: map[string]interface{}{"user_id": "test-user"},
		ID:     "session-123",
	}

	token, err := manager.Generate("session-123", sessionData)
	require.NoError(t, err)

	t.Run("Get valid token", func(t *testing.T) {
		retrievedData, err := manager.Get(token)
		assert.NoError(t, err)
		assert.NotNil(t, retrievedData)
		assert.Equal(t, "session-123", retrievedData.ID)
		assert.Equal(t, "test-user", retrievedData.Values["user_id"])
	})

	t.Run("Get empty token", func(t *testing.T) {
		retrievedData, err := manager.Get("")
		assert.Error(t, err)
		assert.Nil(t, retrievedData)
		assert.Equal(t, ErrJWTNotFound, err)
	})

	t.Run("Get invalid token", func(t *testing.T) {
		retrievedData, err := manager.Get("invalid.token.here")
		assert.Error(t, err)
		assert.Nil(t, retrievedData)
	})
}

func TestJWTManagerSet(t *testing.T) {
	logger := log.New("test")
	config := NewJWTConfig([]byte("test-key-32-bytes-long-enough!!"))
	manager, err := NewJWTManager(config, logger)
	require.NoError(t, err)

	sessionData := &session.SessionData{
		Values:       map[string]interface{}{"user_id": "test-user"},
		LastAccessed: time.Now().Add(-time.Hour),
		ID:           "session-123",
	}

	err = manager.Set("session-123", sessionData)
	assert.NoError(t, err)
	// LastAccessed should be updated
	assert.True(t, sessionData.LastAccessed.After(time.Now().Add(-time.Minute)))
}

func TestJWTManagerNewSession(t *testing.T) {
	logger := log.New("test")
	config := NewJWTConfig([]byte("test-key-32-bytes-long-enough!!"))
	manager, err := NewJWTManager(config, logger)
	require.NoError(t, err)

	sessionData, sessionID := manager.NewSession()
	assert.NotNil(t, sessionData)
	assert.NotEmpty(t, sessionID)
	assert.Equal(t, sessionID, sessionData.ID)
	assert.NotNil(t, sessionData.Values)
	assert.True(t, sessionData.Created.After(time.Now().Add(-time.Minute)))
	assert.True(t, sessionData.LastAccessed.After(time.Now().Add(-time.Minute)))
}

func TestSessionDataFromClaims(t *testing.T) {
	now := time.Now()
	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:  "session-123",
			IssuedAt: jwt.NewNumericDate(now),
		},
		Data: map[string]interface{}{
			"user_id": "test-user",
			"role":    "admin",
		},
	}

	sessionData := SessionDataFromClaims(claims)
	assert.Equal(t, "session-123", sessionData.ID)
	assert.Equal(t, "test-user", sessionData.Values["user_id"])
	assert.Equal(t, "admin", sessionData.Values["role"])
	
	// Check created time with some tolerance for precision loss
	assert.WithinDuration(t, now, sessionData.Created, time.Second)
	assert.True(t, sessionData.LastAccessed.After(now.Add(-time.Minute)))
}

func TestGetRotationCount(t *testing.T) {
	tests := []struct {
		name     string
		data     map[string]interface{}
		expected int
	}{
		{
			name:     "No rotation count",
			data:     map[string]interface{}{},
			expected: 0,
		},
		{
			name:     "Int rotation count",
			data:     map[string]interface{}{"_rotation_count": 5},
			expected: 5,
		},
		{
			name:     "Float64 rotation count",
			data:     map[string]interface{}{"_rotation_count": float64(3)},
			expected: 3,
		},
		{
			name:     "Invalid rotation count",
			data:     map[string]interface{}{"_rotation_count": "invalid"},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getRotationCount(tt.data)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGenerateKeyPairs(t *testing.T) {
	t.Run("RSA key pair generation", func(t *testing.T) {
		privateKey, publicKey, err := GenerateRSAKeyPair(2048)
		assert.NoError(t, err)
		assert.NotNil(t, privateKey)
		assert.NotNil(t, publicKey)
		assert.Equal(t, 2048, privateKey.N.BitLen())
		assert.Equal(t, &privateKey.PublicKey, publicKey)
	})

	t.Run("RSA key pair with small size", func(t *testing.T) {
		privateKey, publicKey, err := GenerateRSAKeyPair(1024) // Should be upgraded to 2048
		assert.NoError(t, err)
		assert.NotNil(t, privateKey)
		assert.NotNil(t, publicKey)
		assert.Equal(t, 2048, privateKey.N.BitLen()) // Should be 2048, not 1024
	})

	t.Run("ECDSA key pair generation", func(t *testing.T) {
		privateKey, publicKey, err := GenerateECDSAKeyPair(elliptic.P256())
		assert.NoError(t, err)
		assert.NotNil(t, privateKey)
		assert.NotNil(t, publicKey)
		assert.Equal(t, elliptic.P256(), privateKey.Curve)
		assert.Equal(t, &privateKey.PublicKey, publicKey)
	})

	t.Run("ECDSA key pair with nil curve", func(t *testing.T) {
		privateKey, publicKey, err := GenerateECDSAKeyPair(nil) // Should default to P256
		assert.NoError(t, err)
		assert.NotNil(t, privateKey)
		assert.NotNil(t, publicKey)
		assert.Equal(t, elliptic.P256(), privateKey.Curve)
	})

	t.Run("Ed25519 key pair generation", func(t *testing.T) {
		privateKey, publicKey, err := GenerateEd25519KeyPair()
		assert.NoError(t, err)
		assert.NotNil(t, privateKey)
		assert.NotNil(t, publicKey)
		assert.Len(t, privateKey, ed25519.PrivateKeySize)
		assert.Len(t, publicKey, ed25519.PublicKeySize)
	})
}

func TestNewJWTConfigWithAlgorithms(t *testing.T) {
	t.Run("RSA config creation", func(t *testing.T) {
		config, err := NewJWTConfigWithRSA("RS256", 2048)
		assert.NoError(t, err)
		assert.NotNil(t, config)
		assert.Equal(t, "RS256", config.SigningAlgorithm)
		assert.NotNil(t, config.PrivateKey)
		assert.NotNil(t, config.PublicKey)
		assert.Nil(t, config.SigningKey)

		err = config.Validate()
		assert.NoError(t, err)
	})

	t.Run("RSA config with invalid algorithm", func(t *testing.T) {
		config, err := NewJWTConfigWithRSA("HS256", 2048)
		assert.Error(t, err)
		assert.Nil(t, config)
	})

	t.Run("ECDSA config creation", func(t *testing.T) {
		config, err := NewJWTConfigWithECDSA("ES256")
		assert.NoError(t, err)
		assert.NotNil(t, config)
		assert.Equal(t, "ES256", config.SigningAlgorithm)
		assert.NotNil(t, config.PrivateKey)
		assert.NotNil(t, config.PublicKey)
		assert.Nil(t, config.SigningKey)

		err = config.Validate()
		assert.NoError(t, err)
	})

	t.Run("ECDSA config with invalid algorithm", func(t *testing.T) {
		config, err := NewJWTConfigWithECDSA("RS256")
		assert.Error(t, err)
		assert.Nil(t, config)
	})

	t.Run("EdDSA config creation", func(t *testing.T) {
		config, err := NewJWTConfigWithEd25519()
		assert.NoError(t, err)
		assert.NotNil(t, config)
		assert.Equal(t, "EdDSA", config.SigningAlgorithm)
		assert.NotNil(t, config.PrivateKey)
		assert.NotNil(t, config.PublicKey)
		assert.Nil(t, config.SigningKey)

		err = config.Validate()
		assert.NoError(t, err)
	})
}