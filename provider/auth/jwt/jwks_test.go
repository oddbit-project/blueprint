package jwt

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/oddbit-project/blueprint/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewJWKSConfig(t *testing.T) {
	config := NewJWKSConfig()
	assert.False(t, config.Enabled)
	assert.Equal(t, "/.well-known/jwks.json", config.Endpoint)
}

func TestNewJWKSManager(t *testing.T) {
	logger := log.New("test")

	t.Run("With config", func(t *testing.T) {
		config := &JWKSConfig{Enabled: true}
		manager := NewJWKSManager(config, logger)
		assert.NotNil(t, manager)
		assert.Equal(t, config, manager.config)
		assert.Equal(t, logger, manager.logger)
	})

	t.Run("With nil config", func(t *testing.T) {
		manager := NewJWKSManager(nil, logger)
		assert.NotNil(t, manager)
		assert.NotNil(t, manager.config)
		assert.False(t, manager.config.Enabled) // Default is disabled
	})
}

func TestJWKSManagerGenerateJWKS(t *testing.T) {
	logger := log.New("test")
	
	t.Run("JWKS disabled", func(t *testing.T) {
		config := &JWKSConfig{Enabled: false}
		manager := NewJWKSManager(config, logger)
		
		jwtConfig := NewJWTConfig([]byte("test-key"))
		jwks, err := manager.GenerateJWKS(jwtConfig)
		assert.Error(t, err)
		assert.Nil(t, jwks)
	})

	t.Run("RSA JWKS generation", func(t *testing.T) {
		config := &JWKSConfig{Enabled: true}
		manager := NewJWKSManager(config, logger)

		jwtConfig, err := NewJWTConfigWithRSA("RS256", 2048)
		require.NoError(t, err)
		jwtConfig.KeyID = "rsa-key-1"

		jwks, err := manager.GenerateJWKS(jwtConfig)
		assert.NoError(t, err)
		assert.NotNil(t, jwks)
		assert.Len(t, jwks.Keys, 1)

		key := jwks.Keys[0]
		assert.Equal(t, "RSA", key.KeyType)
		assert.Equal(t, "rsa-key-1", key.KeyID)
		assert.Equal(t, "sig", key.Use)
		assert.Equal(t, "RS256", key.Algorithm)
		assert.NotEmpty(t, key.Modulus)
		assert.NotEmpty(t, key.Exponent)
	})

	t.Run("ECDSA JWKS generation", func(t *testing.T) {
		config := &JWKSConfig{Enabled: true}
		manager := NewJWKSManager(config, logger)

		jwtConfig, err := NewJWTConfigWithECDSA("ES256")
		require.NoError(t, err)
		jwtConfig.KeyID = "ec-key-1"

		jwks, err := manager.GenerateJWKS(jwtConfig)
		assert.NoError(t, err)
		assert.NotNil(t, jwks)
		assert.Len(t, jwks.Keys, 1)

		key := jwks.Keys[0]
		assert.Equal(t, "EC", key.KeyType)
		assert.Equal(t, "ec-key-1", key.KeyID)
		assert.Equal(t, "sig", key.Use)
		assert.Equal(t, "ES256", key.Algorithm)
		assert.Equal(t, "P-256", key.Curve)
		assert.NotEmpty(t, key.X)
		assert.NotEmpty(t, key.Y)
	})

	t.Run("EdDSA JWKS generation", func(t *testing.T) {
		config := &JWKSConfig{Enabled: true}
		manager := NewJWKSManager(config, logger)

		jwtConfig, err := NewJWTConfigWithEd25519()
		require.NoError(t, err)
		jwtConfig.KeyID = "ed-key-1"

		jwks, err := manager.GenerateJWKS(jwtConfig)
		assert.NoError(t, err)
		assert.NotNil(t, jwks)
		assert.Len(t, jwks.Keys, 1)

		key := jwks.Keys[0]
		assert.Equal(t, "OKP", key.KeyType)
		assert.Equal(t, "ed-key-1", key.KeyID)
		assert.Equal(t, "sig", key.Use)
		assert.Equal(t, "EdDSA", key.Algorithm)
		assert.Equal(t, "Ed25519", key.Curve)
		assert.NotEmpty(t, key.KeyValue)
	})

	t.Run("HMAC not supported", func(t *testing.T) {
		config := &JWKSConfig{Enabled: true}
		manager := NewJWKSManager(config, logger)

		jwtConfig := NewJWTConfig([]byte("test-key"))
		jwtConfig.SigningAlgorithm = "HS256"

		jwks, err := manager.GenerateJWKS(jwtConfig)
		assert.Error(t, err)
		assert.Nil(t, jwks)
		assert.Equal(t, ErrJWKSNotSupported, err)
	})
}

func TestJWKSManagerGenerateRSAJWK(t *testing.T) {
	logger := log.New("test")
	config := &JWKSConfig{Enabled: true}
	manager := NewJWKSManager(config, logger)

	t.Run("With public key", func(t *testing.T) {
		privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
		require.NoError(t, err)

		jwtConfig := &JWTConfig{
			SigningAlgorithm: "RS256",
			KeyID:           "test-key",
			PublicKey:       &privateKey.PublicKey,
		}

		jwk := &JWK{KeyID: "test-key"}
		result, err := manager.generateRSAJWK(jwk, jwtConfig)
		assert.NoError(t, err)
		assert.Equal(t, "RSA", result.KeyType)
		assert.NotEmpty(t, result.Modulus)
		assert.NotEmpty(t, result.Exponent)
	})

	t.Run("With private key only", func(t *testing.T) {
		privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
		require.NoError(t, err)

		jwtConfig := &JWTConfig{
			SigningAlgorithm: "RS256",
			KeyID:           "test-key",
			PrivateKey:      privateKey,
		}

		jwk := &JWK{KeyID: "test-key"}
		result, err := manager.generateRSAJWK(jwk, jwtConfig)
		assert.NoError(t, err)
		assert.Equal(t, "RSA", result.KeyType)
	})

	t.Run("No key available", func(t *testing.T) {
		jwtConfig := &JWTConfig{
			SigningAlgorithm: "RS256",
			KeyID:           "test-key",
		}

		jwk := &JWK{KeyID: "test-key"}
		result, err := manager.generateRSAJWK(jwk, jwtConfig)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Equal(t, ErrJWKSKeyNotAvailable, err)
	})

	t.Run("Wrong key type", func(t *testing.T) {
		jwtConfig := &JWTConfig{
			SigningAlgorithm: "RS256",
			KeyID:           "test-key",
			PublicKey:       "not-a-key",
		}

		jwk := &JWK{KeyID: "test-key"}
		result, err := manager.generateRSAJWK(jwk, jwtConfig)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Equal(t, ErrJWKSKeyNotAvailable, err)
	})
}

func TestJWKSManagerGenerateECDSAJWK(t *testing.T) {
	logger := log.New("test")
	config := &JWKSConfig{Enabled: true}
	manager := NewJWKSManager(config, logger)

	curves := []struct {
		curve elliptic.Curve
		name  string
	}{
		{elliptic.P256(), "P-256"},
		{elliptic.P384(), "P-384"},
		{elliptic.P521(), "P-521"},
	}

	for _, tc := range curves {
		t.Run("Curve "+tc.name, func(t *testing.T) {
			privateKey, err := ecdsa.GenerateKey(tc.curve, rand.Reader)
			require.NoError(t, err)

			jwtConfig := &JWTConfig{
				SigningAlgorithm: "ES256",
				KeyID:           "test-key",
				PublicKey:       &privateKey.PublicKey,
			}

			jwk := &JWK{KeyID: "test-key"}
			result, err := manager.generateECDSAJWK(jwk, jwtConfig)
			assert.NoError(t, err)
			assert.Equal(t, "EC", result.KeyType)
			assert.Equal(t, tc.name, result.Curve)
			assert.NotEmpty(t, result.X)
			assert.NotEmpty(t, result.Y)
		})
	}

	t.Run("No key available", func(t *testing.T) {
		jwtConfig := &JWTConfig{
			SigningAlgorithm: "ES256",
			KeyID:           "test-key",
		}

		jwk := &JWK{KeyID: "test-key"}
		result, err := manager.generateECDSAJWK(jwk, jwtConfig)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Equal(t, ErrJWKSKeyNotAvailable, err)
	})
}

func TestJWKSManagerGenerateEdDSAJWK(t *testing.T) {
	logger := log.New("test")
	config := &JWKSConfig{Enabled: true}
	manager := NewJWKSManager(config, logger)

	t.Run("With public key", func(t *testing.T) {
		publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
		require.NoError(t, err)

		jwtConfig := &JWTConfig{
			SigningAlgorithm: "EdDSA",
			KeyID:           "test-key",
			PublicKey:       publicKey,
			PrivateKey:      privateKey,
		}

		jwk := &JWK{KeyID: "test-key"}
		result, err := manager.generateEdDSAJWK(jwk, jwtConfig)
		assert.NoError(t, err)
		assert.Equal(t, "OKP", result.KeyType)
		assert.Equal(t, "Ed25519", result.Curve)
		assert.NotEmpty(t, result.KeyValue)
	})

	t.Run("With private key only", func(t *testing.T) {
		_, privateKey, err := ed25519.GenerateKey(rand.Reader)
		require.NoError(t, err)

		jwtConfig := &JWTConfig{
			SigningAlgorithm: "EdDSA",
			KeyID:           "test-key",
			PrivateKey:      privateKey,
		}

		jwk := &JWK{KeyID: "test-key"}
		result, err := manager.generateEdDSAJWK(jwk, jwtConfig)
		assert.NoError(t, err)
		assert.Equal(t, "OKP", result.KeyType)
		assert.Equal(t, "Ed25519", result.Curve)
		assert.NotEmpty(t, result.KeyValue)
	})
}

func TestJWKSManagerCreateJWKSHandler(t *testing.T) {
	logger := log.New("test")

	t.Run("JWKS disabled", func(t *testing.T) {
		config := &JWKSConfig{Enabled: false}
		manager := NewJWKSManager(config, logger)

		jwtConfig := NewJWTConfig([]byte("test-key"))
		handler := manager.CreateJWKSHandler(jwtConfig)

		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.GET("/jwks", handler)

		req := httptest.NewRequest("GET", "/jwks", nil)
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)

		assert.Equal(t, http.StatusNotFound, resp.Code)
	})

	t.Run("RSA JWKS endpoint", func(t *testing.T) {
		config := &JWKSConfig{Enabled: true}
		manager := NewJWKSManager(config, logger)

		jwtConfig, err := NewJWTConfigWithRSA("RS256", 2048)
		require.NoError(t, err)

		handler := manager.CreateJWKSHandler(jwtConfig)

		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.GET("/jwks", handler)

		req := httptest.NewRequest("GET", "/jwks", nil)
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)

		assert.Equal(t, http.StatusOK, resp.Code)
		assert.Equal(t, "application/json", resp.Header().Get("Content-Type"))
		assert.Contains(t, resp.Header().Get("Cache-Control"), "public")

		var jwks JWKS
		err = json.Unmarshal(resp.Body.Bytes(), &jwks)
		assert.NoError(t, err)
		assert.Len(t, jwks.Keys, 1)
		assert.Equal(t, "RSA", jwks.Keys[0].KeyType)
	})

	t.Run("HMAC JWKS endpoint error", func(t *testing.T) {
		config := &JWKSConfig{Enabled: true}
		manager := NewJWKSManager(config, logger)

		jwtConfig := NewJWTConfig([]byte("test-key"))
		handler := manager.CreateJWKSHandler(jwtConfig)

		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.GET("/jwks", handler)

		req := httptest.NewRequest("GET", "/jwks", nil)
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)

		assert.Equal(t, http.StatusInternalServerError, resp.Code)
	})
}

func TestJWKSManagerRegisterJWKSEndpoint(t *testing.T) {
	logger := log.New("test")
	
	t.Run("JWKS enabled", func(t *testing.T) {
		config := &JWKSConfig{
			Enabled:  true,
			Endpoint: "/test-jwks",
		}
		manager := NewJWKSManager(config, logger)

		jwtConfig, err := NewJWTConfigWithRSA("RS256", 2048)
		require.NoError(t, err)

		gin.SetMode(gin.TestMode)
		router := gin.New()
		manager.RegisterJWKSEndpoint(router, jwtConfig)

		req := httptest.NewRequest("GET", "/test-jwks", nil)
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)

		assert.Equal(t, http.StatusOK, resp.Code)
	})

	t.Run("JWKS disabled", func(t *testing.T) {
		config := &JWKSConfig{Enabled: false}
		manager := NewJWKSManager(config, logger)

		jwtConfig := NewJWTConfig([]byte("test-key"))

		gin.SetMode(gin.TestMode)
		router := gin.New()
		manager.RegisterJWKSEndpoint(router, jwtConfig)

		// Should not register any routes when disabled
		req := httptest.NewRequest("GET", "/.well-known/jwks.json", nil)
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)

		assert.Equal(t, http.StatusNotFound, resp.Code)
	})
}

func TestValidateJWKS(t *testing.T) {
	t.Run("Valid JWKS", func(t *testing.T) {
		jwks := &JWKS{
			Keys: []JWK{
				{
					KeyType:  "RSA",
					KeyID:    "key-1",
					Use:      "sig",
					Modulus:  "test-modulus",
					Exponent: "test-exponent",
				},
			},
		}

		err := ValidateJWKS(jwks)
		assert.NoError(t, err)
	})

	t.Run("Nil JWKS", func(t *testing.T) {
		err := ValidateJWKS(nil)
		assert.Error(t, err)
	})

	t.Run("Empty JWKS", func(t *testing.T) {
		jwks := &JWKS{Keys: []JWK{}}
		err := ValidateJWKS(jwks)
		assert.Error(t, err)
	})

	t.Run("Invalid JWK", func(t *testing.T) {
		jwks := &JWKS{
			Keys: []JWK{
				{
					KeyType: "", // Invalid - empty key type
				},
			},
		}

		err := ValidateJWKS(jwks)
		assert.Error(t, err)
	})
}

func TestValidateJWK(t *testing.T) {
	t.Run("Nil JWK", func(t *testing.T) {
		err := ValidateJWK(nil)
		assert.Error(t, err)
	})

	t.Run("Empty key type", func(t *testing.T) {
		jwk := &JWK{}
		err := ValidateJWK(jwk)
		assert.Error(t, err)
	})

	t.Run("Valid RSA JWK", func(t *testing.T) {
		jwk := &JWK{
			KeyType:  "RSA",
			Modulus:  "test-modulus",
			Exponent: "test-exponent",
		}
		err := ValidateJWK(jwk)
		assert.NoError(t, err)
	})

	t.Run("Invalid RSA JWK", func(t *testing.T) {
		jwk := &JWK{
			KeyType: "RSA",
			// Missing modulus and exponent
		}
		err := ValidateJWK(jwk)
		assert.Error(t, err)
	})

	t.Run("Valid EC JWK", func(t *testing.T) {
		jwk := &JWK{
			KeyType: "EC",
			Curve:   "P-256",
			X:       "test-x",
			Y:       "test-y",
		}
		err := ValidateJWK(jwk)
		assert.NoError(t, err)
	})

	t.Run("Invalid EC JWK", func(t *testing.T) {
		jwk := &JWK{
			KeyType: "EC",
			// Missing curve, x, y
		}
		err := ValidateJWK(jwk)
		assert.Error(t, err)
	})

	t.Run("Valid OKP JWK", func(t *testing.T) {
		jwk := &JWK{
			KeyType:  "OKP",
			Curve:    "Ed25519",
			KeyValue: "test-key-value",
		}
		err := ValidateJWK(jwk)
		assert.NoError(t, err)
	})

	t.Run("Invalid OKP JWK", func(t *testing.T) {
		jwk := &JWK{
			KeyType: "OKP",
			// Missing curve and key value
		}
		err := ValidateJWK(jwk)
		assert.Error(t, err)
	})

	t.Run("Unsupported key type", func(t *testing.T) {
		jwk := &JWK{
			KeyType: "UNKNOWN",
		}
		err := ValidateJWK(jwk)
		assert.Error(t, err)
	})
}

func TestJWKSFromJSON(t *testing.T) {
	t.Run("Valid JSON", func(t *testing.T) {
		jsonData := `{
			"keys": [
				{
					"kty": "RSA",
					"kid": "key-1",
					"use": "sig",
					"n": "test-modulus",
					"e": "test-exponent"
				}
			]
		}`

		jwks, err := JWKSFromJSON([]byte(jsonData))
		assert.NoError(t, err)
		assert.NotNil(t, jwks)
		assert.Len(t, jwks.Keys, 1)
		assert.Equal(t, "RSA", jwks.Keys[0].KeyType)
	})

	t.Run("Invalid JSON", func(t *testing.T) {
		jwks, err := JWKSFromJSON([]byte("invalid json"))
		assert.Error(t, err)
		assert.Nil(t, jwks)
	})

	t.Run("Invalid JWKS", func(t *testing.T) {
		jsonData := `{"keys": []}`

		jwks, err := JWKSFromJSON([]byte(jsonData))
		assert.Error(t, err)
		assert.Nil(t, jwks)
	})
}

func TestJWKSToJSON(t *testing.T) {
	jwks := &JWKS{
		Keys: []JWK{
			{
				KeyType:  "RSA",
				KeyID:    "key-1",
				Use:      "sig",
				Modulus:  "test-modulus",
				Exponent: "test-exponent",
			},
		},
	}

	jsonData, err := jwks.ToJSON()
	assert.NoError(t, err)
	assert.NotEmpty(t, jsonData)

	// Verify we can parse it back
	parsedJWKS, err := JWKSFromJSON(jsonData)
	assert.NoError(t, err)
	assert.Equal(t, jwks.Keys[0].KeyType, parsedJWKS.Keys[0].KeyType)
	assert.Equal(t, jwks.Keys[0].KeyID, parsedJWKS.Keys[0].KeyID)
}

func TestLeftPad(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		length   int
		expected []byte
	}{
		{
			name:     "No padding needed",
			data:     []byte{1, 2, 3, 4},
			length:   4,
			expected: []byte{1, 2, 3, 4},
		},
		{
			name:     "Padding needed",
			data:     []byte{1, 2},
			length:   4,
			expected: []byte{0, 0, 1, 2},
		},
		{
			name:     "Data longer than length",
			data:     []byte{1, 2, 3, 4, 5},
			length:   3,
			expected: []byte{1, 2, 3, 4, 5},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := leftPad(tt.data, tt.length)
			assert.Equal(t, tt.expected, result)
		})
	}
}