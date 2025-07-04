package jwt

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/oddbit-project/blueprint/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJWTManagerJWKSIntegration(t *testing.T) {
	logger := log.New("test")

	t.Run("RSA JWT Manager with JWKS", func(t *testing.T) {
		// Create RSA JWT config with JWKS enabled
		config, err := NewJWTConfigWithRSA("RS256", 2048)
		require.NoError(t, err)
		config.KeyID = "rsa-test-key"
		config.JWKSConfig = &JWKSConfig{
			Enabled:  true,
			Endpoint: "/keys",
		}

		manager, err := NewJWTManager(config, logger)
		require.NoError(t, err)

		// Test JWKS generation
		jwks, err := manager.GenerateJWKS()
		assert.NoError(t, err)
		assert.NotNil(t, jwks)
		assert.Len(t, jwks.Keys, 1)
		assert.Equal(t, "RSA", jwks.Keys[0].KeyType)
		assert.Equal(t, "rsa-test-key", jwks.Keys[0].KeyID)
	})

	t.Run("ECDSA JWT Manager with JWKS", func(t *testing.T) {
		config, err := NewJWTConfigWithECDSA("ES256")
		require.NoError(t, err)
		config.KeyID = "ec-test-key"
		config.JWKSConfig = &JWKSConfig{
			Enabled:  true,
			Endpoint: "/keys",
		}

		manager, err := NewJWTManager(config, logger)
		require.NoError(t, err)

		jwks, err := manager.GenerateJWKS()
		assert.NoError(t, err)
		assert.NotNil(t, jwks)
		assert.Len(t, jwks.Keys, 1)
		assert.Equal(t, "EC", jwks.Keys[0].KeyType)
		assert.Equal(t, "P-256", jwks.Keys[0].Curve)
	})

	t.Run("EdDSA JWT Manager with JWKS", func(t *testing.T) {
		config, err := NewJWTConfigWithEd25519()
		require.NoError(t, err)
		config.KeyID = "ed-test-key"
		config.JWKSConfig = &JWKSConfig{
			Enabled:  true,
			Endpoint: "/keys",
		}

		manager, err := NewJWTManager(config, logger)
		require.NoError(t, err)

		jwks, err := manager.GenerateJWKS()
		assert.NoError(t, err)
		assert.NotNil(t, jwks)
		assert.Len(t, jwks.Keys, 1)
		assert.Equal(t, "OKP", jwks.Keys[0].KeyType)
		assert.Equal(t, "Ed25519", jwks.Keys[0].Curve)
	})
}

func TestJWTManagerGetJWKSManager(t *testing.T) {
	logger := log.New("test")
	config := NewJWTConfig([]byte("test-key-32-bytes-long-enough!!"))
	manager, err := NewJWTManager(config, logger)
	require.NoError(t, err)

	jwksManager := manager.GetJWKSManager()
	assert.NotNil(t, jwksManager)
	assert.Equal(t, manager.jwksManager, jwksManager)
}

func TestJWTManagerCreateJWKSHandler(t *testing.T) {
	logger := log.New("test")

	t.Run("RSA JWKS handler", func(t *testing.T) {
		config, err := NewJWTConfigWithRSA("RS256", 2048)
		require.NoError(t, err)
		config.JWKSConfig = &JWKSConfig{Enabled: true}

		manager, err := NewJWTManager(config, logger)
		require.NoError(t, err)

		handler := manager.CreateJWKSHandler()
		assert.NotNil(t, handler)

		// Test the handler
		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.GET("/jwks", handler)

		req := httptest.NewRequest("GET", "/jwks", nil)
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)

		assert.Equal(t, http.StatusOK, resp.Code)
		assert.Equal(t, "application/json", resp.Header().Get("Content-Type"))

		var jwks JWKS
		err = json.Unmarshal(resp.Body.Bytes(), &jwks)
		assert.NoError(t, err)
		assert.Len(t, jwks.Keys, 1)
		assert.Equal(t, "RSA", jwks.Keys[0].KeyType)
	})

	t.Run("HMAC JWKS handler (should fail)", func(t *testing.T) {
		config := NewJWTConfig([]byte("test-key-32-bytes-long-enough!!"))
		config.JWKSConfig = &JWKSConfig{Enabled: true}

		manager, err := NewJWTManager(config, logger)
		require.NoError(t, err)

		handler := manager.CreateJWKSHandler()
		assert.NotNil(t, handler)

		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.GET("/jwks", handler)

		req := httptest.NewRequest("GET", "/jwks", nil)
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)

		assert.Equal(t, http.StatusInternalServerError, resp.Code)
	})

	t.Run("JWKS disabled handler", func(t *testing.T) {
		config := NewJWTConfig([]byte("test-key-32-bytes-long-enough!!"))
		config.JWKSConfig = &JWKSConfig{Enabled: false}

		manager, err := NewJWTManager(config, logger)
		require.NoError(t, err)

		handler := manager.CreateJWKSHandler()
		assert.NotNil(t, handler)

		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.GET("/jwks", handler)

		req := httptest.NewRequest("GET", "/jwks", nil)
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)

		assert.Equal(t, http.StatusNotFound, resp.Code)
	})
}

func TestJWTManagerRegisterJWKSEndpoint(t *testing.T) {
	logger := log.New("test")

	t.Run("Register JWKS endpoint", func(t *testing.T) {
		config, err := NewJWTConfigWithRSA("RS256", 2048)
		require.NoError(t, err)
		config.JWKSConfig = &JWKSConfig{
			Enabled:  true,
			Endpoint: "/custom-jwks",
		}

		manager, err := NewJWTManager(config, logger)
		require.NoError(t, err)

		gin.SetMode(gin.TestMode)
		router := gin.New()
		manager.RegisterJWKSEndpoint(router)

		// Test that the endpoint was registered
		req := httptest.NewRequest("GET", "/custom-jwks", nil)
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)

		assert.Equal(t, http.StatusOK, resp.Code)
		assert.Equal(t, "application/json", resp.Header().Get("Content-Type"))
	})

	t.Run("JWKS disabled - no endpoint registered", func(t *testing.T) {
		config := NewJWTConfig([]byte("test-key-32-bytes-long-enough!!"))
		config.JWKSConfig = &JWKSConfig{Enabled: false}

		manager, err := NewJWTManager(config, logger)
		require.NoError(t, err)

		gin.SetMode(gin.TestMode)
		router := gin.New()
		manager.RegisterJWKSEndpoint(router)

		// Should not register endpoint when disabled
		req := httptest.NewRequest("GET", "/.well-known/jwks.json", nil)
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)

		assert.Equal(t, http.StatusNotFound, resp.Code)
	})
}

func TestJWTManagerJWKSWithNilConfig(t *testing.T) {
	logger := log.New("test")
	config := NewJWTConfig([]byte("test-key-32-bytes-long-enough!!"))
	// Don't set JWKSConfig (should be nil)

	manager, err := NewJWTManager(config, logger)
	require.NoError(t, err)

	t.Run("JWKS generation with nil config", func(t *testing.T) {
		jwks, err := manager.GenerateJWKS()
		assert.Error(t, err)
		assert.Nil(t, jwks)
		assert.Contains(t, err.Error(), "JWKS is disabled")
	})

	t.Run("JWKS handler with nil config", func(t *testing.T) {
		handler := manager.CreateJWKSHandler()
		assert.NotNil(t, handler)

		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.GET("/jwks", handler)

		req := httptest.NewRequest("GET", "/jwks", nil)
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)

		assert.Equal(t, http.StatusNotFound, resp.Code)
	})
}

func TestJWTManagerJWKSAlgorithmCoverage(t *testing.T) {
	logger := log.New("test")

	algorithms := []struct {
		name         string
		createConfig func() (*JWTConfig, error)
		expectedType string
	}{
		{
			name: "RS384",
			createConfig: func() (*JWTConfig, error) {
				return NewJWTConfigWithRSA("RS384", 2048)
			},
			expectedType: "RSA",
		},
		{
			name: "RS512",
			createConfig: func() (*JWTConfig, error) {
				return NewJWTConfigWithRSA("RS512", 2048)
			},
			expectedType: "RSA",
		},
		{
			name: "ES384",
			createConfig: func() (*JWTConfig, error) {
				return NewJWTConfigWithECDSA("ES384")
			},
			expectedType: "EC",
		},
		{
			name: "ES512",
			createConfig: func() (*JWTConfig, error) {
				return NewJWTConfigWithECDSA("ES512")
			},
			expectedType: "EC",
		},
	}

	for _, alg := range algorithms {
		t.Run(alg.name, func(t *testing.T) {
			config, err := alg.createConfig()
			require.NoError(t, err)
			config.JWKSConfig = &JWKSConfig{Enabled: true}

			manager, err := NewJWTManager(config, logger)
			require.NoError(t, err)

			jwks, err := manager.GenerateJWKS()
			assert.NoError(t, err)
			assert.NotNil(t, jwks)
			assert.Len(t, jwks.Keys, 1)
			assert.Equal(t, alg.expectedType, jwks.Keys[0].KeyType)
		})
	}
}

func TestJWTManagerJWKSHTTPHeaders(t *testing.T) {
	logger := log.New("test")
	config, err := NewJWTConfigWithRSA("RS256", 2048)
	require.NoError(t, err)
	config.JWKSConfig = &JWKSConfig{Enabled: true}

	manager, err := NewJWTManager(config, logger)
	require.NoError(t, err)

	handler := manager.CreateJWKSHandler()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/jwks", handler)

	req := httptest.NewRequest("GET", "/jwks", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
	assert.Equal(t, "application/json", resp.Header().Get("Content-Type"))
	
	// Check cache control header
	cacheControl := resp.Header().Get("Cache-Control")
	assert.Contains(t, cacheControl, "public")
	assert.Contains(t, cacheControl, "max-age=3600")
}

func TestJWTManagerJWKSEndpointCustomization(t *testing.T) {
	logger := log.New("test")

	customEndpoints := []string{
		"/keys",
		"/auth/jwks",
		"/.well-known/keys",
		"/api/v1/jwks.json",
	}

	for _, endpoint := range customEndpoints {
		t.Run("Custom endpoint: "+endpoint, func(t *testing.T) {
			config, err := NewJWTConfigWithRSA("RS256", 2048)
			require.NoError(t, err)
			config.JWKSConfig = &JWKSConfig{
				Enabled:  true,
				Endpoint: endpoint,
			}

			manager, err := NewJWTManager(config, logger)
			require.NoError(t, err)

			gin.SetMode(gin.TestMode)
			router := gin.New()
			manager.RegisterJWKSEndpoint(router)

			req := httptest.NewRequest("GET", endpoint, nil)
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			assert.Equal(t, http.StatusOK, resp.Code)
		})
	}
}

func TestJWTManagerJWKSErrorHandling(t *testing.T) {
	logger := log.New("test")

	t.Run("Unsupported algorithm", func(t *testing.T) {
		config := NewJWTConfig([]byte("test-key-32-bytes-long-enough!!"))
		config.SigningAlgorithm = "UNSUPPORTED"
		config.JWKSConfig = &JWKSConfig{Enabled: true}

		manager, err := NewJWTManager(config, logger)
		// This should fail during config validation
		assert.Error(t, err)
		assert.Nil(t, manager)
	})

	t.Run("Missing keys for asymmetric algorithm", func(t *testing.T) {
		config := NewJWTConfig(nil)
		config.SigningAlgorithm = "RS256"
		config.JWKSConfig = &JWKSConfig{Enabled: true}
		// Don't set private/public keys

		manager, err := NewJWTManager(config, logger)
		// Should fail during validation
		assert.Error(t, err)
		assert.Nil(t, manager)
	})
}

func TestJWTManagerJWKSManagerCreation(t *testing.T) {
	logger := log.New("test")

	t.Run("JWKS manager created with config", func(t *testing.T) {
		jwksConfig := &JWKSConfig{
			Enabled:  true,
			Endpoint: "/test-keys",
		}
		config := NewJWTConfig([]byte("test-key-32-bytes-long-enough!!"))
		config.JWKSConfig = jwksConfig

		manager, err := NewJWTManager(config, logger)
		require.NoError(t, err)

		jwksManager := manager.GetJWKSManager()
		assert.NotNil(t, jwksManager)
		assert.Equal(t, jwksConfig, jwksManager.config)
	})

	t.Run("JWKS manager created with nil config", func(t *testing.T) {
		config := NewJWTConfig([]byte("test-key-32-bytes-long-enough!!"))
		// JWKSConfig is nil

		manager, err := NewJWTManager(config, logger)
		require.NoError(t, err)

		jwksManager := manager.GetJWKSManager()
		assert.NotNil(t, jwksManager)
		assert.NotNil(t, jwksManager.config)
		assert.False(t, jwksManager.config.Enabled) // Should be disabled by default
	})
}