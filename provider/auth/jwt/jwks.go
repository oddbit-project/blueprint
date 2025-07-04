package jwt

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/oddbit-project/blueprint/log"
	"github.com/oddbit-project/blueprint/utils"
)

const (
	ErrJWKSNotSupported    = utils.Error("JWKS not supported for this signing algorithm")
	ErrJWKSKeyNotAvailable = utils.Error("public key not available for JWKS")
)

// JWKSConfig holds configuration for JWKS endpoint
type JWKSConfig struct {
	Enabled  bool   `json:"enabled"`  // Enable JWKS endpoint
	Endpoint string `json:"endpoint"` // JWKS endpoint path (default: /.well-known/jwks.json)
}

// NewJWKSConfig creates a default JWKS configuration
func NewJWKSConfig() *JWKSConfig {
	return &JWKSConfig{
		Enabled:  false, // Disabled by default
		Endpoint: "/.well-known/jwks.json",
	}
}

// JWK represents a JSON Web Key
type JWK struct {
	KeyType   string `json:"kty"`           // Key Type
	KeyID     string `json:"kid,omitempty"` // Key ID
	Use       string `json:"use,omitempty"` // Public Key Use
	Algorithm string `json:"alg,omitempty"` // Algorithm
	
	// RSA-specific fields
	Modulus  string `json:"n,omitempty"` // RSA modulus
	Exponent string `json:"e,omitempty"` // RSA exponent
	
	// ECDSA-specific fields
	Curve string `json:"crv,omitempty"` // Curve
	X     string `json:"x,omitempty"`   // X coordinate
	Y     string `json:"y,omitempty"`   // Y coordinate
	
	// EdDSA-specific fields
	KeyValue string `json:"x,omitempty"` // Key value for EdDSA (reuses x field)
}

// JWKS represents a JSON Web Key Set
type JWKS struct {
	Keys []JWK `json:"keys"`
}

// JWKSManager manages JWKS functionality
type JWKSManager struct {
	config *JWKSConfig
	logger *log.Logger
}

// NewJWKSManager creates a new JWKS manager
func NewJWKSManager(config *JWKSConfig, logger *log.Logger) *JWKSManager {
	if config == nil {
		config = NewJWKSConfig()
	}
	
	return &JWKSManager{
		config: config,
		logger: logger,
	}
}

// GenerateJWKS generates a JWKS from a JWT configuration
func (jm *JWKSManager) GenerateJWKS(jwtConfig *JWTConfig) (*JWKS, error) {
	if !jm.config.Enabled {
		return nil, fmt.Errorf("JWKS is disabled")
	}
	
	jwk, err := jm.generateJWKFromConfig(jwtConfig)
	if err != nil {
		return nil, err
	}
	
	return &JWKS{
		Keys: []JWK{*jwk},
	}, nil
}

// generateJWKFromConfig creates a JWK from JWT configuration
func (jm *JWKSManager) generateJWKFromConfig(config *JWTConfig) (*JWK, error) {
	jwk := &JWK{
		KeyID:     config.KeyID,
		Use:       "sig", // Signature use
		Algorithm: config.SigningAlgorithm,
	}
	
	switch config.SigningAlgorithm {
	case "RS256", "RS384", "RS512":
		return jm.generateRSAJWK(jwk, config)
	case "ES256", "ES384", "ES512":
		return jm.generateECDSAJWK(jwk, config)
	case "EdDSA":
		return jm.generateEdDSAJWK(jwk, config)
	case "HS256", "HS384", "HS512":
		// HMAC keys should not be exposed in JWKS
		return nil, ErrJWKSNotSupported
	default:
		return nil, ErrJWKSNotSupported
	}
}

// generateRSAJWK creates a JWK for RSA public keys
func (jm *JWKSManager) generateRSAJWK(jwk *JWK, config *JWTConfig) (*JWK, error) {
	var publicKey *rsa.PublicKey
	
	if config.PublicKey != nil {
		var ok bool
		publicKey, ok = config.PublicKey.(*rsa.PublicKey)
		if !ok {
			return nil, ErrJWKSKeyNotAvailable
		}
	} else if config.PrivateKey != nil {
		if privKey, ok := config.PrivateKey.(*rsa.PrivateKey); ok {
			publicKey = &privKey.PublicKey
		} else {
			return nil, ErrJWKSKeyNotAvailable
		}
	} else {
		return nil, ErrJWKSKeyNotAvailable
	}
	
	jwk.KeyType = "RSA"
	jwk.Modulus = base64.RawURLEncoding.EncodeToString(publicKey.N.Bytes())
	jwk.Exponent = base64.RawURLEncoding.EncodeToString(big.NewInt(int64(publicKey.E)).Bytes())
	
	return jwk, nil
}

// generateECDSAJWK creates a JWK for ECDSA public keys
func (jm *JWKSManager) generateECDSAJWK(jwk *JWK, config *JWTConfig) (*JWK, error) {
	var publicKey *ecdsa.PublicKey
	
	if config.PublicKey != nil {
		var ok bool
		publicKey, ok = config.PublicKey.(*ecdsa.PublicKey)
		if !ok {
			return nil, ErrJWKSKeyNotAvailable
		}
	} else if config.PrivateKey != nil {
		if privKey, ok := config.PrivateKey.(*ecdsa.PrivateKey); ok {
			publicKey = &privKey.PublicKey
		} else {
			return nil, ErrJWKSKeyNotAvailable
		}
	} else {
		return nil, ErrJWKSKeyNotAvailable
	}
	
	jwk.KeyType = "EC"
	
	// Set curve name
	switch publicKey.Curve {
	case elliptic.P256():
		jwk.Curve = "P-256"
	case elliptic.P384():
		jwk.Curve = "P-384"
	case elliptic.P521():
		jwk.Curve = "P-521"
	default:
		return nil, fmt.Errorf("unsupported elliptic curve")
	}
	
	// Encode coordinates
	keySize := (publicKey.Curve.Params().BitSize + 7) / 8
	jwk.X = base64.RawURLEncoding.EncodeToString(leftPad(publicKey.X.Bytes(), keySize))
	jwk.Y = base64.RawURLEncoding.EncodeToString(leftPad(publicKey.Y.Bytes(), keySize))
	
	return jwk, nil
}

// generateEdDSAJWK creates a JWK for EdDSA public keys
func (jm *JWKSManager) generateEdDSAJWK(jwk *JWK, config *JWTConfig) (*JWK, error) {
	var publicKey ed25519.PublicKey
	
	if config.PublicKey != nil {
		var ok bool
		publicKey, ok = config.PublicKey.(ed25519.PublicKey)
		if !ok {
			return nil, ErrJWKSKeyNotAvailable
		}
	} else if config.PrivateKey != nil {
		if privKey, ok := config.PrivateKey.(ed25519.PrivateKey); ok {
			publicKey = ed25519.PublicKey(privKey[32:])
		} else {
			return nil, ErrJWKSKeyNotAvailable
		}
	} else {
		return nil, ErrJWKSKeyNotAvailable
	}
	
	jwk.KeyType = "OKP"
	jwk.Curve = "Ed25519"
	jwk.KeyValue = base64.RawURLEncoding.EncodeToString(publicKey)
	
	return jwk, nil
}

// CreateJWKSHandler creates a Gin handler for the JWKS endpoint
func (jm *JWKSManager) CreateJWKSHandler(jwtConfig *JWTConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !jm.config.Enabled {
			c.JSON(http.StatusNotFound, gin.H{"error": "JWKS endpoint is disabled"})
			return
		}
		
		jwks, err := jm.GenerateJWKS(jwtConfig)
		if err != nil {
			if jm.logger != nil {
				jm.logger.Error(err, "Failed to generate JWKS")
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate JWKS"})
			return
		}
		
		// Set appropriate headers
		c.Header("Content-Type", "application/json")
		c.Header("Cache-Control", "public, max-age=3600") // Cache for 1 hour
		
		c.JSON(http.StatusOK, jwks)
	}
}

// RegisterJWKSEndpoint registers the JWKS endpoint with a Gin router
func (jm *JWKSManager) RegisterJWKSEndpoint(router *gin.Engine, jwtConfig *JWTConfig) {
	if jm.config.Enabled {
		router.GET(jm.config.Endpoint, jm.CreateJWKSHandler(jwtConfig))
		
		if jm.logger != nil {
			jm.logger.Info("JWKS endpoint registered", map[string]interface{}{
				"endpoint": jm.config.Endpoint,
			})
		}
	}
}

// leftPad pads a byte slice to the specified length with leading zeros
func leftPad(data []byte, length int) []byte {
	if len(data) >= length {
		return data
	}
	
	padded := make([]byte, length)
	copy(padded[length-len(data):], data)
	return padded
}

// ValidateJWKS validates a JWKS structure
func ValidateJWKS(jwks *JWKS) error {
	if jwks == nil {
		return fmt.Errorf("JWKS cannot be nil")
	}
	
	if len(jwks.Keys) == 0 {
		return fmt.Errorf("JWKS must contain at least one key")
	}
	
	for i, key := range jwks.Keys {
		if err := ValidateJWK(&key); err != nil {
			return fmt.Errorf("invalid JWK at index %d: %w", i, err)
		}
	}
	
	return nil
}

// ValidateJWK validates a JWK structure
func ValidateJWK(jwk *JWK) error {
	if jwk == nil {
		return fmt.Errorf("JWK cannot be nil")
	}
	
	if jwk.KeyType == "" {
		return fmt.Errorf("JWK must have a key type")
	}
	
	switch jwk.KeyType {
	case "RSA":
		if jwk.Modulus == "" || jwk.Exponent == "" {
			return fmt.Errorf("RSA JWK must have modulus and exponent")
		}
	case "EC":
		if jwk.Curve == "" || jwk.X == "" || jwk.Y == "" {
			return fmt.Errorf("EC JWK must have curve, x, and y coordinates")
		}
	case "OKP":
		if jwk.Curve == "" || jwk.KeyValue == "" {
			return fmt.Errorf("OKP JWK must have curve and key value")
		}
	default:
		return fmt.Errorf("unsupported key type: %s", jwk.KeyType)
	}
	
	return nil
}

// JWKSFromJSON parses a JWKS from JSON bytes
func JWKSFromJSON(data []byte) (*JWKS, error) {
	var jwks JWKS
	if err := json.Unmarshal(data, &jwks); err != nil {
		return nil, fmt.Errorf("failed to parse JWKS JSON: %w", err)
	}
	
	if err := ValidateJWKS(&jwks); err != nil {
		return nil, fmt.Errorf("invalid JWKS: %w", err)
	}
	
	return &jwks, nil
}

// ToJSON converts a JWKS to JSON bytes
func (jwks *JWKS) ToJSON() ([]byte, error) {
	return json.MarshalIndent(jwks, "", "  ")
}