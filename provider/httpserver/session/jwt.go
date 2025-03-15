package session

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/golang-jwt/jwt/v5"
	"github.com/oddbit-project/blueprint/log"
	"time"
)

// JWT-related errors
var (
	ErrJWTInvalid    = errors.New("invalid JWT token")
	ErrJWTExpired    = errors.New("JWT token expired")
	ErrJWTNotFound   = errors.New("JWT token not found")
	ErrJWTSigningKey = errors.New("JWT signing key is required")
)

// JWTConfig holds configuration for JWT tokens
type JWTConfig struct {
	// SigningKey is the key used to sign JWT tokens
	SigningKey []byte

	// SigningMethod is the method used to sign the token
	SigningMethod jwt.SigningMethod

	// Expiration is the expiration time for tokens
	Expiration time.Duration

	// Issuer is the issuer of the token
	Issuer string

	// Audience is the audience of the token
	Audience string

	// Logger for operations
	Logger *log.Logger
}

// DefaultJWTConfig returns a default JWT configuration
func DefaultJWTConfig() *JWTConfig {
	return &JWTConfig{
		SigningKey:    nil, // Must be set by the user
		SigningMethod: jwt.SigningMethodHS256,
		Expiration:    time.Hour * 24, // 24 hours
		Issuer:        "blueprint",
		Audience:      "api",
		Logger:        nil,
	}
}

// JWTManager manages JWT tokens
type JWTManager struct {
	config *JWTConfig
}

// NewJWTManager creates a new JWT manager
func NewJWTManager(config *JWTConfig) (*JWTManager, error) {
	if config == nil {
		config = DefaultJWTConfig()
	}

	if len(config.SigningKey) == 0 {
		return nil, ErrJWTSigningKey
	}

	return &JWTManager{
		config: config,
	}, nil
}

// Claims is a custom JWT claims type
type Claims struct {
	jwt.RegisteredClaims
	Data map[string]interface{} `json:"data,omitempty"`
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
		if m.config.Logger != nil {
			m.config.Logger.Error(err, "Failed to sign JWT token")
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

// SessionDataFromClaims converts JWT claims to a SessionData object
func SessionDataFromClaims(claims *Claims) *SessionData {
	return &SessionData{
		Values:       claims.Data,
		LastAccessed: time.Now(),
		Created:      claims.IssuedAt.Time,
		ID:           claims.ID,
	}
}

// JWTStore is a JWT token-based store implementation
type JWTStore struct {
	manager *JWTManager
	config  *SessionConfig
	logger  *log.Logger
}

// NewJWTStore creates a new JWT-based store
func NewJWTStore(jwtManager *JWTManager, sessionConfig *SessionConfig) *JWTStore {
	if sessionConfig == nil {
		sessionConfig = DefaultSessionConfig()
	}

	return &JWTStore{
		manager: jwtManager,
		config:  sessionConfig,
		logger:  sessionConfig.Logger,
	}
}

// Get retrieves a session from a JWT token
func (s *JWTStore) Get(tokenString string) (*SessionData, error) {
	if tokenString == "" {
		return nil, ErrJWTNotFound
	}

	// Validate token
	claims, err := s.manager.Validate(tokenString)
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
func (s *JWTStore) Set(id string, session *SessionData) error {
	tokenString, err := s.manager.Generate(id, session)
	if err != nil {
		return err
	}

	// Store the token string in the session itself
	// This is a hack since JWTStore doesn't have persistent storage
	session.Values["_jwt_token"] = tokenString
	return nil
}

// Delete is a no-op for JWT store since tokens are stateless
func (s *JWTStore) Delete(id string) error {
	// No-op: JWT tokens are stateless and can't be deleted
	// They will expire based on their expiration time
	return nil
}

// Generate creates a new session and ID
func (s *JWTStore) Generate() (*SessionData, string) {
	id := generateSessionID()
	
	session := &SessionData{
		Values:       make(map[string]interface{}),
		LastAccessed: time.Now(),
		Created:      time.Now(),
		ID:           id,
	}

	return session, id
}

// StartCleanup is a no-op for JWT store
func (s *JWTStore) StartCleanup() {
	// No-op: JWT tokens are stateless and don't need cleanup
}

// StopCleanup is a no-op for JWT store
func (s *JWTStore) StopCleanup() {
	// No-op: JWT tokens are stateless and don't need cleanup
}

// MarshallSessionData converts a session data object to JSON
func MarshallSessionData(session *SessionData) (string, error) {
	data, err := json.Marshal(session)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// UnmarshallSessionData converts JSON to a session data object
func UnmarshallSessionData(data string) (*SessionData, error) {
	var session SessionData
	err := json.Unmarshal([]byte(data), &session)
	if err != nil {
		return nil, err
	}
	return &session, nil
}