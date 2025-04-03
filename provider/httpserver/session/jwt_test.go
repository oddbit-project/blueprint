package session

import (
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestJWTManager(t *testing.T) {
	// Create JWT config
	config := NewJWTConfig()
	config.SigningKey = []byte("test-signing-key-for-jwt-tests-only")

	// Create manager
	manager, err := NewJWTManager(config, nil)
	assert.NoError(t, err)
	assert.NotNil(t, manager)

	// Create session
	session := &SessionData{
		Values: map[string]interface{}{
			"user_id":  123,
			"username": "testuser",
			"roles":    []string{"admin", "user"},
		},
		LastAccessed: time.Now(),
		Created:      time.Now(),
		ID:           "test-session-id",
	}

	// Generate token
	token, err := manager.Generate(session.ID, session)
	assert.NoError(t, err)
	assert.NotEmpty(t, token)

	// Validate token
	claims, err := manager.Validate(token)
	assert.NoError(t, err)
	assert.NotNil(t, claims)

	// Check claims
	assert.Equal(t, session.ID, claims.ID)
	assert.Equal(t, float64(123), claims.Data["user_id"])
	assert.Equal(t, "testuser", claims.Data["username"])
}

func TestJWTExpiration(t *testing.T) {
	// Create a fake expired token for testing
	// This approach skips waiting for actual expiration, which can be timing sensitive in tests

	// Create manager with standard config
	config := NewJWTConfig()
	config.SigningKey = []byte("test-signing-key-for-jwt-tests-only")
	
	manager, err := NewJWTManager(config, nil)
	assert.NoError(t, err)

	// Create a token with expiry time in the past
	now := time.Now()
	pastTime := now.Add(-time.Hour) // Time in the past
	
	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    config.Issuer,
			Subject:   "test-expired",
			Audience:  jwt.ClaimStrings{config.Audience},
			ExpiresAt: jwt.NewNumericDate(pastTime), // Expired time!
			NotBefore: jwt.NewNumericDate(pastTime),
			IssuedAt:  jwt.NewNumericDate(pastTime),
			ID:        "test-expired-id",
		},
		Data: map[string]interface{}{"test": "value"},
	}
	
	token := jwt.NewWithClaims(config.SigningMethod, claims)
	tokenString, err := token.SignedString(config.SigningKey)
	assert.NoError(t, err)
	
	// Validate token (should fail with expiration error)
	_, err = manager.Validate(tokenString)
	assert.Error(t, err)
	assert.Equal(t, ErrJWTExpired, err)
}

func TestJWTRefresh(t *testing.T) {
	// Create JWT config
	config := NewJWTConfig()
	config.SigningKey = []byte("test-signing-key-for-jwt-tests-only")

	// Create manager
	manager, err := NewJWTManager(config, nil)
	assert.NoError(t, err)

	// Create session
	session := &SessionData{
		Values: map[string]interface{}{
			"user_id": 123,
		},
		LastAccessed: time.Now(),
		Created:      time.Now(),
		ID:           "test-session-id",
	}

	// Generate token
	token, err := manager.Generate(session.ID, session)
	assert.NoError(t, err)

	// Wait a tiny bit to ensure timestamps are different
	time.Sleep(time.Millisecond * 5)

	// Refresh token
	newToken, err := manager.Refresh(token)
	assert.NoError(t, err)
	assert.NotEqual(t, token, newToken) // Should be different

	// Validate new token
	claims, err := manager.Validate(newToken)
	assert.NoError(t, err)
	assert.Equal(t, float64(123), claims.Data["user_id"])
}

func TestJWTStore(t *testing.T) {
	// Create JWT config
	jwtConfig := NewJWTConfig()
	jwtConfig.SigningKey = []byte("test-signing-key-for-jwt-tests-only")

	// Create manager
	jwtManager, err := NewJWTManager(jwtConfig, nil)
	assert.NoError(t, err)

	// Generate session
	session, id := jwtManager.NewSession()
	assert.NotNil(t, session)
	assert.NotEmpty(t, id)

	// Add data to session
	session.Values["user_id"] = 123

	// Set session
	err = jwtManager.Set(id, session)
	assert.NoError(t, err)

	// Verify token was stored in session values
	tokenStr, ok := session.Values["_jwt_token"].(string)
	assert.True(t, ok)
	assert.NotEmpty(t, tokenStr)

	// Get session using token
	retrievedSession, err := jwtManager.Get(tokenStr)
	assert.NoError(t, err)
	assert.NotNil(t, retrievedSession)

	// Verify data
	assert.Equal(t, float64(123), retrievedSession.Values["user_id"])
}
