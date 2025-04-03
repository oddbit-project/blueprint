package session

import (
	"crypto/rand"
	"errors"
	"fmt"
	"github.com/golang-jwt/jwt/v5"
	"github.com/oddbit-project/blueprint/log"
	"github.com/oddbit-project/blueprint/utils"
	"time"
)

const (
	ErrJWTSigningKey           = utils.Error("JWT signing key is required")
	ErrInvalidSigningAlgorithm = utils.Error("JWT signing algorithm is invalid")
)

// JWT-related errors
var (
	ErrJWTInvalid  = errors.New("invalid JWT token")
	ErrJWTExpired  = errors.New("JWT token expired")
	ErrJWTNotFound = errors.New("JWT token not found")
)

// JWTConfig holds configuration for JWT tokens
type JWTConfig struct {
	SigningKey        []byte            `json:"signingKey"`        // SigningKey is the key used to sign JWT tokens; if json, base64-encoded key
	SigningAlgorithm  string            `json:"signingAlgorithm"`  // SigningAlgorithm, one of HS256, HS384, HS512
	ExpirationSeconds int               `json:"expirationSeconds"` // ExpirationSeconds
	Issuer            string            `json:"issuer"`            // Issuer is the issuer of the token
	Audience          string            `json:"audience"`          // Audience is the audience of the token
	SigningMethod     jwt.SigningMethod `json:"-"`                 // SigningMethod is the method used to sign the token; filled on Validate()
	Expiration        time.Duration     `json:"-"`                 // Expiration is the expiration time for tokens; filled on Validate()
}

// JWTManager manages JWT tokens
type JWTManager struct {
	config *JWTConfig
	logger *log.Logger
}

// Claims is a custom JWT claims type
type Claims struct {
	jwt.RegisteredClaims
	Data map[string]interface{} `json:"data,omitempty"`
}

// NewJWTConfig returns a default JWT configuration
func NewJWTConfig() *JWTConfig {
	// random signing key, should be overriden by user
	buf := make([]byte, 128)
	_, err := rand.Read(buf)
	if err != nil {
		panic(err)
	}
	return &JWTConfig{
		SigningKey:        buf, // Must be set by the user
		SigningAlgorithm:  "HS256",
		SigningMethod:     jwt.SigningMethodHS256,
		ExpirationSeconds: 86400,
		Expiration:        time.Second * 86400, // 24 hours
		Issuer:            "blueprint",
		Audience:          "api",
	}
}

// Validate the JWT configuration
func (c *JWTConfig) Validate() error {
	if len(c.SigningKey) == 0 {

		return ErrJWTSigningKey
	}

	c.SigningMethod = jwt.GetSigningMethod(c.SigningAlgorithm)
	if c.SigningMethod == nil {
		return ErrInvalidSigningAlgorithm
	}
	if c.ExpirationSeconds <= 0 {
		return ErrInvalidExpirationSeconds
	}
	c.Expiration = time.Second * time.Duration(c.ExpirationSeconds)

	return nil
}

// NewJWTManager creates a new JWT manager
func NewJWTManager(config *JWTConfig, logger *log.Logger) (*JWTManager, error) {
	if config == nil {
		config = NewJWTConfig()
	}

	if err := config.Validate(); err != nil {
		return nil, err
	}

	return &JWTManager{
		config: config,
		logger: logger,
	}, nil
}

// Generate creates a new JWT token with the given claims
func (m *JWTManager) Generate(sessionID string, sessionData *SessionData) (string, error) {
	now := time.Now()
	expiresAt := now.Add(m.config.Expiration)

	// Create claims
	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.config.Issuer,
			Subject:   sessionID,
			Audience:  jwt.ClaimStrings{m.config.Audience},
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			NotBefore: jwt.NewNumericDate(now),
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        sessionID,
		},
		Data: make(map[string]interface{}),
	}

	// Add session data to claims
	for k, v := range sessionData.Values {
		claims.Data[k] = v
	}

	// Create token
	token := jwt.NewWithClaims(m.config.SigningMethod, claims)

	// Sign and get the complete encoded token
	tokenString, err := token.SignedString(m.config.SigningKey)
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
		return m.config.SigningKey, nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrJWTExpired
		}
		return nil, ErrJWTInvalid
	}

	// Get claims
	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, ErrJWTInvalid
}

// Refresh refreshes a JWT token
func (m *JWTManager) Refresh(tokenString string) (string, error) {
	// Validate existing token
	claims, err := m.Validate(tokenString)
	if err != nil {
		return "", err
	}

	// Create a new session data from the claims
	sessionData := &SessionData{
		Values:       claims.Data,
		LastAccessed: time.Now(),
		Created:      time.Now(),
		ID:           claims.ID,
	}

	// Add a refresh token indicator to ensure the token is different
	sessionData.Values["_refreshed_at"] = time.Now().UnixNano()

	// Generate new token
	return m.Generate(claims.ID, sessionData)
}

// Get retrieves a session from a JWT token
func (m *JWTManager) Get(tokenString string) (*SessionData, error) {
	if tokenString == "" {
		return nil, ErrJWTNotFound
	}

	// Validate token
	claims, err := m.Validate(tokenString)
	if err != nil {
		return nil, err
	}

	// Convert claims to session data
	session := SessionDataFromClaims(claims)
	return session, nil
}

// Set generates a new JWT token for the session data
// Note: The returned error contains the token string
// This is necessary because there's no persistent storage with JWT
func (m *JWTManager) Set(id string, session *SessionData) error {
	tokenString, err := m.Generate(id, session)
	if err != nil {
		return err
	}

	// Store the token string in the session itself
	// This is a hack since JWTStore doesn't have persistent storage
	session.Values["_jwt_token"] = tokenString
	return nil
}

// Generate creates a new session and ID
func (m *JWTManager) NewSession() (*SessionData, string) {
	id := generateSessionID()

	session := &SessionData{
		Values:       make(map[string]interface{}),
		LastAccessed: time.Now(),
		Created:      time.Now(),
		ID:           id,
	}

	return session, id
}

// SessionDataFromClaims converts JWT claims to a SessionData object
func SessionDataFromClaims(claims *Claims) *SessionData {
	return &SessionData{
		Values:       claims.Data,
		LastAccessed: time.Now(),
		Created:      claims.IssuedAt.Time,
		ID:           claims.ID,
	}
}
