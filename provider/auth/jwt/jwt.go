package jwt

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/oddbit-project/blueprint/log"
	"github.com/oddbit-project/blueprint/provider/httpserver/session"
	"github.com/oddbit-project/blueprint/utils"
)

const (
	ErrJWTSigningKey           = utils.Error("JWT signing key is required")
	ErrInvalidSigningAlgorithm = utils.Error("JWT signing algorithm is invalid")
	ErrInvalidKeyType          = utils.Error("invalid key type for selected algorithm")
	ErrMissingIssuer           = utils.Error("issuer validation failed")
	ErrMissingAudience         = utils.Error("audience validation failed")
)

// JWT-related errors
var (
	ErrJWTInvalid  = errors.New("invalid JWT token")
	ErrJWTExpired  = errors.New("JWT token expired")
	ErrJWTNotFound = errors.New("JWT token not found")
)

// JWTConfig holds configuration for JWT tokens
type JWTConfig struct {
	SigningKey        []byte            `json:"signingKey"`        // SigningKey is the key used to sign JWT tokens; if json, base64-encoded key (for HMAC)
	PrivateKey        interface{}       `json:"-"`                 // PrivateKey for asymmetric algorithms (RSA, ECDSA, EdDSA)
	PublicKey         interface{}       `json:"-"`                 // PublicKey for asymmetric algorithms (RSA, ECDSA, EdDSA)
	SigningAlgorithm  string            `json:"signingAlgorithm"`  // SigningAlgorithm: HS256/HS384/HS512, RS256/RS384/RS512, ES256/ES384/ES512, EdDSA
	ExpirationSeconds int               `json:"expirationSeconds"` // ExpirationSeconds
	Issuer            string            `json:"issuer"`            // Issuer is the issuer of the token
	Audience          string            `json:"audience"`          // Audience is the audience of the token
	KeyID             string            `json:"keyID"`             // KeyID for JWKS support
	SigningMethod     jwt.SigningMethod `json:"-"`                 // SigningMethod is the method used to sign the token; filled on Validate()
	Expiration        time.Duration     `json:"-"`                 // Expiration is the expiration time for tokens; filled on Validate()

	// Enhanced validation flags
	RequireIssuer   bool `json:"requireIssuer"`   // Mandatory issuer validation
	RequireAudience bool `json:"requireAudience"` // Mandatory audience validation

	// JWKS configuration
	JWKSConfig *JWKSConfig `json:"jwksConfig,omitempty"` // JWKS endpoint configuration
}

// JWTManager manages JWT tokens
type JWTManager struct {
	config            *JWTConfig
	logger            *log.Logger
	revocationManager *RevocationManager
	jwksManager       *JWKSManager
}

// Claims is a custom JWT claims type
type Claims struct {
	jwt.RegisteredClaims
	Data map[string]interface{} `json:"data,omitempty"`
}

// RandomJWTKey generate a random signing key
func RandomJWTKey() []byte {
	buf := make([]byte, 128)
	_, err := rand.Read(buf)
	if err != nil {
		panic(err)
	}
	return buf
}

// NewJWTConfig returns a default JWT configuration
func NewJWTConfig(signingKey []byte) *JWTConfig {
	return &JWTConfig{
		SigningKey:        signingKey,
		SigningAlgorithm:  "HS256",
		SigningMethod:     jwt.SigningMethodHS256,
		ExpirationSeconds: 86400,
		Expiration:        time.Second * 86400, // 24 hours
		Issuer:            "blueprint",
		Audience:          "api",
		KeyID:             "default",
		RequireIssuer:     true, // Enable by default for security
		RequireAudience:   true, // Enable by default for security
	}
}

// Validate the JWT configuration
func (c *JWTConfig) Validate() error {
	// Get signing method
	c.SigningMethod = jwt.GetSigningMethod(c.SigningAlgorithm)
	if c.SigningMethod == nil {
		return ErrInvalidSigningAlgorithm
	}

	// Validate keys based on algorithm type
	switch c.SigningAlgorithm {
	case "HS256", "HS384", "HS512":
		// HMAC algorithms require signing key
		if len(c.SigningKey) == 0 {
			return ErrJWTSigningKey
		}
	case "RS256", "RS384", "RS512":
		// RSA algorithms require private key for signing, public key for verification
		if c.PrivateKey == nil {
			return ErrJWTSigningKey
		}
		if _, ok := c.PrivateKey.(*rsa.PrivateKey); !ok {
			return ErrInvalidKeyType
		}
		// For verification, we need either public key or can derive from private key
		if c.PublicKey == nil {
			if privKey, ok := c.PrivateKey.(*rsa.PrivateKey); ok {
				c.PublicKey = &privKey.PublicKey
			}
		}
	case "ES256", "ES384", "ES512":
		// ECDSA algorithms require private key for signing, public key for verification
		if c.PrivateKey == nil {
			return ErrJWTSigningKey
		}
		if _, ok := c.PrivateKey.(*ecdsa.PrivateKey); !ok {
			return ErrInvalidKeyType
		}
		// For verification, we need either public key or can derive from private key
		if c.PublicKey == nil {
			if privKey, ok := c.PrivateKey.(*ecdsa.PrivateKey); ok {
				c.PublicKey = &privKey.PublicKey
			}
		}
	case "EdDSA":
		// EdDSA algorithms require private key for signing, public key for verification
		if c.PrivateKey == nil {
			return ErrJWTSigningKey
		}
		if _, ok := c.PrivateKey.(ed25519.PrivateKey); !ok {
			return ErrInvalidKeyType
		}
		// For verification, we need either public key or can derive from private key
		if c.PublicKey == nil {
			if privKey, ok := c.PrivateKey.(ed25519.PrivateKey); ok {
				c.PublicKey = ed25519.PublicKey(privKey[32:])
			}
		}
	default:
		return ErrInvalidSigningAlgorithm
	}

	if c.ExpirationSeconds <= 0 {
		return session.ErrInvalidExpirationSeconds
	}
	c.Expiration = time.Second * time.Duration(c.ExpirationSeconds)

	return nil
}

// NewJWTManager creates a new JWT manager
func NewJWTManager(config *JWTConfig, logger *log.Logger) (*JWTManager, error) {
	return NewJWTManagerWithRevocation(config, logger, nil)
}

// NewJWTManagerWithRevocation creates a new JWT manager with optional revocation support
func NewJWTManagerWithRevocation(config *JWTConfig, logger *log.Logger, revocationManager *RevocationManager) (*JWTManager, error) {
	if config == nil {
		config = NewJWTConfig(RandomJWTKey())
	}

	if err := config.Validate(); err != nil {
		return nil, err
	}

	// Create default revocation manager if none provided
	if revocationManager == nil {
		revocationManager = NewRevocationManager(NewMemoryRevocationBackend())
	}

	// Create JWKS manager
	jwksManager := NewJWKSManager(config.JWKSConfig, logger)

	return &JWTManager{
		config:            config,
		logger:            logger,
		revocationManager: revocationManager,
		jwksManager:       jwksManager,
	}, nil
}

// Generate creates a new JWT token with the given claims
func (m *JWTManager) Generate(sessionID string, sessionData *session.SessionData) (string, error) {
	now := time.Now()
	expiresAt := now.Add(m.config.Expiration)

	// Generate a unique JWT ID separate from session ID for security
	jwtID := session.GenerateSessionID()

	// Create claims
	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.config.Issuer,
			Subject:   sessionID,
			Audience:  jwt.ClaimStrings{m.config.Audience},
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			NotBefore: jwt.NewNumericDate(now),
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        jwtID, // Use separate JWT ID, not session ID
		},
		Data: make(map[string]interface{}),
	}

	// Add session data to claims, excluding internal JWT token storage
	for k, v := range sessionData.Values {
		if k != "_jwt_token" && k != "_jwt_id" {
			claims.Data[k] = v
		}
	}

	// Create token with key ID header for JWKS support
	token := jwt.NewWithClaims(m.config.SigningMethod, claims)
	if m.config.KeyID != "" {
		token.Header["kid"] = m.config.KeyID
	}

	// Get the appropriate signing key for the algorithm
	var signingKey interface{}
	switch m.config.SigningAlgorithm {
	case "HS256", "HS384", "HS512":
		signingKey = m.config.SigningKey
	case "RS256", "RS384", "RS512", "ES256", "ES384", "ES512", "EdDSA":
		signingKey = m.config.PrivateKey
	default:
		return "", ErrInvalidSigningAlgorithm
	}

	// Sign and get the complete encoded token
	tokenString, err := token.SignedString(signingKey)
	if err != nil {
		if m.logger != nil {
			m.logger.Error(err, "Failed to sign JWT token")
		}
		return "", err
	}

	return tokenString, nil
}

// Validate validates a JWT token and returns the claims
func (m *JWTManager) Validate(tokenString string) (*Claims, error) {
	// Parse token
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// Validate signing method
		if token.Method.Alg() != m.config.SigningMethod.Alg() {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		// Return the appropriate verification key for the algorithm
		switch m.config.SigningAlgorithm {
		case "HS256", "HS384", "HS512":
			return m.config.SigningKey, nil
		case "RS256", "RS384", "RS512", "ES256", "ES384", "ES512", "EdDSA":
			if m.config.PublicKey != nil {
				return m.config.PublicKey, nil
			}
			// If no public key is set, derive from private key
			switch key := m.config.PrivateKey.(type) {
			case *rsa.PrivateKey:
				return &key.PublicKey, nil
			case *ecdsa.PrivateKey:
				return &key.PublicKey, nil
			case ed25519.PrivateKey:
				return ed25519.PublicKey(key[32:]), nil
			default:
				return nil, ErrInvalidKeyType
			}
		default:
			return nil, ErrInvalidSigningAlgorithm
		}
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrJWTExpired
		}
		return nil, ErrJWTInvalid
	}

	// Get claims
	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		// Check if token is revoked
		if m.revocationManager != nil && m.revocationManager.IsTokenRevoked(claims.ID) {
			return nil, ErrTokenAlreadyRevoked
		}
		
		// Perform mandatory claim validation
		if err := m.validateMandatoryClaims(claims); err != nil {
			return nil, err
		}
		return claims, nil
	}

	return nil, ErrJWTInvalid
}

// validateMandatoryClaims performs mandatory issuer and audience validation
func (m *JWTManager) validateMandatoryClaims(claims *Claims) error {
	// Validate issuer if required
	if m.config.RequireIssuer {
		if claims.Issuer == "" {
			return ErrMissingIssuer
		}
		if claims.Issuer != m.config.Issuer {
			return ErrMissingIssuer
		}
	}

	// Validate audience if required
	if m.config.RequireAudience {
		if len(claims.Audience) == 0 {
			return ErrMissingAudience
		}
		// Check if our audience is in the token's audience list
		audienceValid := false
		for _, aud := range claims.Audience {
			if aud == m.config.Audience {
				audienceValid = true
				break
			}
		}
		if !audienceValid {
			return ErrMissingAudience
		}
	}

	return nil
}

// Refresh refreshes a JWT token with token rotation
func (m *JWTManager) Refresh(tokenString string) (string, error) {
	// Validate existing token
	claims, err := m.Validate(tokenString)
	if err != nil {
		return "", err
	}

	// Create a new session data from the claims using the Subject (session ID)
	sessionData := &session.SessionData{
		Values:       claims.Data,
		LastAccessed: time.Now(),
		Created:      claims.IssuedAt.Time,
		ID:           claims.Subject, // Use Subject as session ID, not JWT ID
	}

	// Add rotation metadata to ensure token uniqueness and track rotations
	sessionData.Values["_rotated_at"] = time.Now().UnixNano()
	sessionData.Values["_rotation_count"] = getRotationCount(claims.Data) + 1

	// Generate new token with same session ID but new JWT ID
	return m.Generate(claims.Subject, sessionData)
}

// getRotationCount safely gets the rotation count from claims data
func getRotationCount(data map[string]interface{}) int {
	if count, ok := data["_rotation_count"]; ok {
		if intCount, ok := count.(int); ok {
			return intCount
		}
		// Handle float64 from JSON unmarshaling
		if floatCount, ok := count.(float64); ok {
			return int(floatCount)
		}
	}
	return 0
}

// Get retrieves a session from a JWT token
func (m *JWTManager) Get(tokenString string) (*session.SessionData, error) {
	if tokenString == "" {
		return nil, ErrJWTNotFound
	}

	// Validate token
	claims, err := m.Validate(tokenString)
	if err != nil {
		return nil, err
	}

	// Convert claims to session data
	sessionData := SessionDataFromClaims(claims)
	return sessionData, nil
}

// Set generates a new JWT token for the session data
// Note: JWT tokens are stateless, so this doesn't store anything persistently
func (m *JWTManager) Set(id string, sessionData *session.SessionData) error {
	// Generate token but don't store it in session data for security
	_, err := m.Generate(id, sessionData)
	if err != nil {
		return err
	}

	// Update session metadata only
	sessionData.LastAccessed = time.Now()
	return nil
}

// NewSession creates a new session and ID
func (m *JWTManager) NewSession() (*session.SessionData, string) {
	id := session.GenerateSessionID()

	sessionData := &session.SessionData{
		Values:       make(map[string]interface{}),
		LastAccessed: time.Now(),
		Created:      time.Now(),
		ID:           id,
	}

	return sessionData, id
}

// SessionDataFromClaims converts JWT claims to a SessionData object
func SessionDataFromClaims(claims *Claims) *session.SessionData {
	return &session.SessionData{
		Values:       claims.Data,
		LastAccessed: time.Now(),
		Created:      claims.IssuedAt.Time,
		ID:           claims.Subject, // Use Subject as session ID, not JWT ID
	}
}

// RevokeToken revokes a specific JWT token by extracting its ID and expiration
func (m *JWTManager) RevokeToken(tokenString string) error {
	// Parse token to get claims using our own verification key
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// Use the same key resolution logic as in Validate
		switch m.config.SigningAlgorithm {
		case "HS256", "HS384", "HS512":
			return m.config.SigningKey, nil
		case "RS256", "RS384", "RS512", "ES256", "ES384", "ES512", "EdDSA":
			if m.config.PublicKey != nil {
				return m.config.PublicKey, nil
			}
			// If no public key is set, derive from private key
			switch key := m.config.PrivateKey.(type) {
			case *rsa.PrivateKey:
				return &key.PublicKey, nil
			case *ecdsa.PrivateKey:
				return &key.PublicKey, nil
			case ed25519.PrivateKey:
				return ed25519.PublicKey(key[32:]), nil
			default:
				return nil, ErrInvalidKeyType
			}
		default:
			return nil, ErrInvalidSigningAlgorithm
		}
	})
	
	if err != nil {
		// If we can't parse the token, we can't revoke it
		return fmt.Errorf("cannot parse token for revocation: %w", err)
	}
	
	if claims, ok := token.Claims.(*Claims); ok {
		if claims.ID == "" {
			return ErrInvalidTokenID
		}
		
		// Use token's expiration time for revocation expiry
		expiresAt := claims.ExpiresAt.Time
		if expiresAt.IsZero() {
			// If no expiration set, use a far future date
			expiresAt = time.Now().Add(24 * time.Hour * 365)
		}
		
		return m.revocationManager.RevokeToken(claims.ID, expiresAt)
	}
	
	return ErrJWTInvalid
}

// RevokeTokenByID revokes a token by its JWT ID
func (m *JWTManager) RevokeTokenByID(tokenID string, expiresAt time.Time) error {
	if m.revocationManager == nil {
		return fmt.Errorf("revocation manager not available")
	}
	return m.revocationManager.RevokeToken(tokenID, expiresAt)
}

// IsTokenRevoked checks if a token is revoked by its JWT ID
func (m *JWTManager) IsTokenRevoked(tokenID string) bool {
	if m.revocationManager == nil {
		return false
	}
	return m.revocationManager.IsTokenRevoked(tokenID)
}

// RevokeAllUserTokens revokes all tokens for a user issued before a specific time
func (m *JWTManager) RevokeAllUserTokens(userID string, issuedBefore time.Time) error {
	if m.revocationManager == nil {
		return fmt.Errorf("revocation manager not available")
	}
	return m.revocationManager.RevokeAllUserTokens(userID, issuedBefore)
}

// GetRevocationManager returns the revocation manager for advanced operations
func (m *JWTManager) GetRevocationManager() *RevocationManager {
	return m.revocationManager
}

// GetJWKSManager returns the JWKS manager for key distribution
func (m *JWTManager) GetJWKSManager() *JWKSManager {
	return m.jwksManager
}

// GenerateJWKS generates a JWKS for this JWT manager's configuration
func (m *JWTManager) GenerateJWKS() (*JWKS, error) {
	return m.jwksManager.GenerateJWKS(m.config)
}

// CreateJWKSHandler creates a Gin handler for the JWKS endpoint
func (m *JWTManager) CreateJWKSHandler() gin.HandlerFunc {
	return m.jwksManager.CreateJWKSHandler(m.config)
}

// RegisterJWKSEndpoint registers the JWKS endpoint with a Gin router
func (m *JWTManager) RegisterJWKSEndpoint(router *gin.Engine) {
	m.jwksManager.RegisterJWKSEndpoint(router, m.config)
}

// GenerateRSAKeyPair generates an RSA key pair for RS256/RS384/RS512 algorithms
func GenerateRSAKeyPair(bits int) (*rsa.PrivateKey, *rsa.PublicKey, error) {
	if bits < 2048 {
		bits = 2048 // Minimum secure key size
	}
	privateKey, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		return nil, nil, err
	}
	return privateKey, &privateKey.PublicKey, nil
}

// GenerateECDSAKeyPair generates an ECDSA key pair for ES256/ES384/ES512 algorithms
func GenerateECDSAKeyPair(curve elliptic.Curve) (*ecdsa.PrivateKey, *ecdsa.PublicKey, error) {
	if curve == nil {
		curve = elliptic.P256() // Default to P-256 for ES256
	}
	privateKey, err := ecdsa.GenerateKey(curve, rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	return privateKey, &privateKey.PublicKey, nil
}

// GenerateEd25519KeyPair generates an Ed25519 key pair for EdDSA algorithm
func GenerateEd25519KeyPair() (ed25519.PrivateKey, ed25519.PublicKey, error) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	return privateKey, publicKey, nil
}

// NewJWTConfigWithRSA creates a JWT config with RSA keys
func NewJWTConfigWithRSA(algorithm string, bits int) (*JWTConfig, error) {
	if algorithm != "RS256" && algorithm != "RS384" && algorithm != "RS512" {
		return nil, fmt.Errorf("invalid RSA algorithm: %s", algorithm)
	}

	privateKey, publicKey, err := GenerateRSAKeyPair(bits)
	if err != nil {
		return nil, err
	}

	config := NewJWTConfig(nil) // No signing key for asymmetric
	config.SigningAlgorithm = algorithm
	config.SigningMethod = jwt.GetSigningMethod(algorithm)
	config.PrivateKey = privateKey
	config.PublicKey = publicKey

	return config, nil
}

// NewJWTConfigWithECDSA creates a JWT config with ECDSA keys
func NewJWTConfigWithECDSA(algorithm string) (*JWTConfig, error) {
	var curve elliptic.Curve
	switch algorithm {
	case "ES256":
		curve = elliptic.P256()
	case "ES384":
		curve = elliptic.P384()
	case "ES512":
		curve = elliptic.P521()
	default:
		return nil, fmt.Errorf("invalid ECDSA algorithm: %s", algorithm)
	}

	privateKey, publicKey, err := GenerateECDSAKeyPair(curve)
	if err != nil {
		return nil, err
	}

	config := NewJWTConfig(nil) // No signing key for asymmetric
	config.SigningAlgorithm = algorithm
	config.SigningMethod = jwt.GetSigningMethod(algorithm)
	config.PrivateKey = privateKey
	config.PublicKey = publicKey

	return config, nil
}

// NewJWTConfigWithEd25519 creates a JWT config with Ed25519 keys
func NewJWTConfigWithEd25519() (*JWTConfig, error) {
	privateKey, publicKey, err := GenerateEd25519KeyPair()
	if err != nil {
		return nil, err
	}

	config := NewJWTConfig(nil) // No signing key for asymmetric
	config.SigningAlgorithm = "EdDSA"
	config.SigningMethod = jwt.GetSigningMethod("EdDSA")
	config.PrivateKey = privateKey
	config.PublicKey = publicKey

	return config, nil
}
