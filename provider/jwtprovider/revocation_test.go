package jwtprovider

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// Mock RevocationBackend for testing
type mockRevocationBackend struct {
	revokedTokens      map[string]*RevokedToken
	revokeTokenError   error
	isTokenRevokedFunc func(tokenID string) bool
	cleanupError       error
	closeError         error
}

func newMockRevocationBackend() *mockRevocationBackend {
	return &mockRevocationBackend{
		revokedTokens: make(map[string]*RevokedToken),
	}
}

func (m *mockRevocationBackend) RevokeToken(tokenID string, expiresAt time.Time) error {
	if m.revokeTokenError != nil {
		return m.revokeTokenError
	}
	m.revokedTokens[tokenID] = &RevokedToken{
		TokenID:   tokenID,
		RevokedAt: time.Now(),
		ExpiresAt: expiresAt,
	}
	return nil
}

func (m *mockRevocationBackend) IsTokenRevoked(tokenID string) bool {
	if m.isTokenRevokedFunc != nil {
		return m.isTokenRevokedFunc(tokenID)
	}
	_, exists := m.revokedTokens[tokenID]
	return exists
}

func (m *mockRevocationBackend) RevokeAllUserTokens(userID string, issuedBefore time.Time) error {
	// Mock implementation
	return nil
}

func (m *mockRevocationBackend) GetRevokedTokens() ([]RevokedToken, error) {
	tokens := make([]RevokedToken, 0, len(m.revokedTokens))
	for _, token := range m.revokedTokens {
		tokens = append(tokens, *token)
	}
	return tokens, nil
}

func (m *mockRevocationBackend) CleanupExpired() error {
	if m.cleanupError != nil {
		return m.cleanupError
	}
	// Mock cleanup
	now := time.Now()
	for tokenID, token := range m.revokedTokens {
		if now.After(token.ExpiresAt) {
			delete(m.revokedTokens, tokenID)
		}
	}
	return nil
}

func (m *mockRevocationBackend) Close() error {
	if m.closeError != nil {
		return m.closeError
	}
	return nil
}

// Test NewRevocationManager
func TestNewRevocationManager(t *testing.T) {
	t.Run("with backend", func(t *testing.T) {
		backend := newMockRevocationBackend()
		manager := NewRevocationManager(backend)
		
		assert.NotNil(t, manager)
		assert.Equal(t, backend, manager.backend)
	})
	
	t.Run("with nil backend", func(t *testing.T) {
		manager := NewRevocationManager(nil)
		
		assert.NotNil(t, manager)
		assert.NotNil(t, manager.backend)
		// Should create a default memory backend
		_, isMemoryBackend := manager.backend.(*MemoryRevocationBackend)
		assert.True(t, isMemoryBackend)
	})
}

// Test RevokeToken
func TestRevocationManager_RevokeToken(t *testing.T) {
	tests := []struct {
		name        string
		tokenID     string
		expiresAt   time.Time
		setupBackend func() *mockRevocationBackend
		wantErr     error
	}{
		{
			name:      "successful revocation",
			tokenID:   "test-token-123",
			expiresAt: time.Now().Add(time.Hour),
			setupBackend: func() *mockRevocationBackend {
				return newMockRevocationBackend()
			},
			wantErr: nil,
		},
		{
			name:      "empty token ID",
			tokenID:   "",
			expiresAt: time.Now().Add(time.Hour),
			setupBackend: func() *mockRevocationBackend {
				return newMockRevocationBackend()
			},
			wantErr: ErrInvalidTokenID,
		},
		{
			name:      "already revoked token",
			tokenID:   "already-revoked",
			expiresAt: time.Now().Add(time.Hour),
			setupBackend: func() *mockRevocationBackend {
				backend := newMockRevocationBackend()
				backend.revokedTokens["already-revoked"] = &RevokedToken{
					TokenID:   "already-revoked",
					RevokedAt: time.Now(),
					ExpiresAt: time.Now().Add(time.Hour),
				}
				return backend
			},
			wantErr: ErrTokenAlreadyRevoked,
		},
		{
			name:      "backend error",
			tokenID:   "test-token",
			expiresAt: time.Now().Add(time.Hour),
			setupBackend: func() *mockRevocationBackend {
				backend := newMockRevocationBackend()
				backend.revokeTokenError = ErrRevocationFailed
				return backend
			},
			wantErr: ErrRevocationFailed,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backend := tt.setupBackend()
			manager := NewRevocationManager(backend)
			
			err := manager.RevokeToken(tt.tokenID, tt.expiresAt)
			
			if tt.wantErr != nil {
				assert.Error(t, err)
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
				// Verify token was added to backend
				assert.True(t, backend.IsTokenRevoked(tt.tokenID))
			}
		})
	}
}

// Test IsTokenRevoked
func TestRevocationManager_IsTokenRevoked(t *testing.T) {
	tests := []struct {
		name     string
		tokenID  string
		setup    func() *mockRevocationBackend
		expected bool
	}{
		{
			name:    "revoked token",
			tokenID: "revoked-token",
			setup: func() *mockRevocationBackend {
				backend := newMockRevocationBackend()
				backend.revokedTokens["revoked-token"] = &RevokedToken{
					TokenID:   "revoked-token",
					RevokedAt: time.Now(),
					ExpiresAt: time.Now().Add(time.Hour),
				}
				return backend
			},
			expected: true,
		},
		{
			name:    "non-revoked token",
			tokenID: "valid-token",
			setup: func() *mockRevocationBackend {
				return newMockRevocationBackend()
			},
			expected: false,
		},
		{
			name:    "empty token ID",
			tokenID: "",
			setup: func() *mockRevocationBackend {
				return newMockRevocationBackend()
			},
			expected: false,
		},
		{
			name:    "custom revocation check",
			tokenID: "custom-check",
			setup: func() *mockRevocationBackend {
				backend := newMockRevocationBackend()
				backend.isTokenRevokedFunc = func(tokenID string) bool {
					return tokenID == "custom-check"
				}
				return backend
			},
			expected: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backend := tt.setup()
			manager := NewRevocationManager(backend)
			
			result := manager.IsTokenRevoked(tt.tokenID)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Test RevokeAllUserTokens
func TestRevocationManager_RevokeAllUserTokens(t *testing.T) {
	tests := []struct {
		name         string
		userID       string
		issuedBefore time.Time
		wantErr      error
	}{
		{
			name:         "successful revocation",
			userID:       "user-123",
			issuedBefore: time.Now(),
			wantErr:      nil,
		},
		{
			name:         "empty user ID",
			userID:       "",
			issuedBefore: time.Now(),
			wantErr:      ErrInvalidTokenID,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backend := newMockRevocationBackend()
			manager := NewRevocationManager(backend)
			
			err := manager.RevokeAllUserTokens(tt.userID, tt.issuedBefore)
			
			if tt.wantErr != nil {
				assert.Error(t, err)
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// Test CleanupExpired
func TestRevocationManager_CleanupExpired(t *testing.T) {
	t.Run("successful cleanup", func(t *testing.T) {
		backend := newMockRevocationBackend()
		manager := NewRevocationManager(backend)
		
		err := manager.CleanupExpired()
		assert.NoError(t, err)
	})
	
	t.Run("cleanup error", func(t *testing.T) {
		backend := newMockRevocationBackend()
		backend.cleanupError = ErrRevocationFailed
		manager := NewRevocationManager(backend)
		
		err := manager.CleanupExpired()
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrRevocationFailed)
	})
}

// Test GetRevokedTokens
func TestRevocationManager_GetRevokedTokens(t *testing.T) {
	backend := newMockRevocationBackend()
	
	// Add some revoked tokens
	now := time.Now()
	backend.revokedTokens["token1"] = &RevokedToken{
		TokenID:   "token1",
		UserID:    "user1",
		RevokedAt: now,
		ExpiresAt: now.Add(time.Hour),
	}
	backend.revokedTokens["token2"] = &RevokedToken{
		TokenID:   "token2",
		UserID:    "user2",
		RevokedAt: now.Add(-time.Minute),
		ExpiresAt: now.Add(2 * time.Hour),
	}
	
	manager := NewRevocationManager(backend)
	
	tokens, err := manager.GetRevokedTokens()
	assert.NoError(t, err)
	assert.Len(t, tokens, 2)
	
	// Verify tokens are returned
	tokenIDs := make(map[string]bool)
	for _, token := range tokens {
		tokenIDs[token.TokenID] = true
	}
	assert.True(t, tokenIDs["token1"])
	assert.True(t, tokenIDs["token2"])
}

// Test Close
func TestRevocationManager_Close(t *testing.T) {
	t.Run("successful close", func(t *testing.T) {
		backend := newMockRevocationBackend()
		manager := NewRevocationManager(backend)
		
		err := manager.Close()
		assert.NoError(t, err)
	})
	
	t.Run("close error", func(t *testing.T) {
		backend := newMockRevocationBackend()
		backend.closeError = ErrRevocationFailed
		manager := NewRevocationManager(backend)
		
		err := manager.Close()
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrRevocationFailed)
	})
	
	t.Run("nil backend", func(t *testing.T) {
		manager := &RevocationManager{backend: nil}
		
		err := manager.Close()
		assert.NoError(t, err)
	})
}

// Test MemoryRevocationBackend
func TestMemoryRevocationBackend(t *testing.T) {
	t.Run("NewMemoryRevocationBackend", func(t *testing.T) {
		backend := NewMemoryRevocationBackend()
		
		assert.NotNil(t, backend)
		// Verify the backend is functional by testing basic operations
		assert.False(t, backend.IsTokenRevoked("non-existent"))
		assert.Equal(t, 0, backend.getRevokedTokenCountForTest())
		
		// Clean up
		err := backend.Close()
		assert.NoError(t, err)
	})
	
	t.Run("RevokeToken", func(t *testing.T) {
		backend := NewMemoryRevocationBackend()
		defer backend.Close()
		
		tokenID := "test-token"
		expiresAt := time.Now().Add(time.Hour)
		
		err := backend.RevokeToken(tokenID, expiresAt)
		assert.NoError(t, err)
		
		// Verify token is revoked
		assert.True(t, backend.IsTokenRevoked(tokenID))
		
		// Check internal state
		revokedToken, exists := backend.getRevokedTokenForTest(tokenID)
		assert.True(t, exists)
		assert.Equal(t, tokenID, revokedToken.TokenID)
		assert.Equal(t, expiresAt, revokedToken.ExpiresAt)
		assert.True(t, revokedToken.RevokedAt.Before(time.Now().Add(time.Second)))
	})
	
	t.Run("IsTokenRevoked with expired token", func(t *testing.T) {
		backend := NewMemoryRevocationBackend()
		defer backend.Close()
		
		tokenID := "expired-token"
		// Revoke with past expiration
		expiresAt := time.Now().Add(-time.Hour)
		
		backend.addRevokedTokenForTest(tokenID, &RevokedToken{
			TokenID:   tokenID,
			RevokedAt: time.Now().Add(-2 * time.Hour),
			ExpiresAt: expiresAt,
		})
		
		// Should return false for expired revocation
		assert.False(t, backend.IsTokenRevoked(tokenID))
		
		// Give goroutine time to clean up
		time.Sleep(100 * time.Millisecond)
		
		// Token should be removed from map
		assert.False(t, backend.hasRevokedToken(tokenID))
	})
	
	t.Run("RevokeAllUserTokens", func(t *testing.T) {
		backend := NewMemoryRevocationBackend()
		defer backend.Close()
		
		userID := "user-123"
		
		// Track some tokens for the user
		backend.TrackUserToken(userID, "token1")
		backend.TrackUserToken(userID, "token2")
		backend.TrackUserToken(userID, "token3")
		
		// Revoke some tokens individually first
		backend.RevokeToken("token1", time.Now().Add(time.Hour))
		
		// Revoke all user tokens
		issuedBefore := time.Now()
		err := backend.RevokeAllUserTokens(userID, issuedBefore)
		assert.NoError(t, err)
		
		// All tokens should be revoked
		assert.True(t, backend.IsTokenRevoked("token1"))
		assert.True(t, backend.IsTokenRevoked("token2"))
		assert.True(t, backend.IsTokenRevoked("token3"))
	})
	
	t.Run("RevokeAllUserTokens with no tokens", func(t *testing.T) {
		backend := NewMemoryRevocationBackend()
		defer backend.Close()
		
		// Should not error for non-existent user
		err := backend.RevokeAllUserTokens("unknown-user", time.Now())
		assert.NoError(t, err)
	})
	
	t.Run("GetRevokedTokens", func(t *testing.T) {
		backend := NewMemoryRevocationBackend()
		defer backend.Close()
		
		// Add some tokens
		now := time.Now()
		backend.addRevokedTokenForTest("token1", &RevokedToken{
			TokenID:   "token1",
			RevokedAt: now,
			ExpiresAt: now.Add(time.Hour),
		})
		backend.addRevokedTokenForTest("token2", &RevokedToken{
			TokenID:   "token2",
			RevokedAt: now,
			ExpiresAt: now.Add(2 * time.Hour),
		})
		// Add expired token (should not be returned)
		backend.addRevokedTokenForTest("expired", &RevokedToken{
			TokenID:   "expired",
			RevokedAt: now.Add(-2 * time.Hour),
			ExpiresAt: now.Add(-time.Hour),
		})
		
		tokens, err := backend.GetRevokedTokens()
		assert.NoError(t, err)
		assert.Len(t, tokens, 2)
		
		// Verify only non-expired tokens are returned
		for _, token := range tokens {
			assert.NotEqual(t, "expired", token.TokenID)
		}
	})
	
	t.Run("CleanupExpired", func(t *testing.T) {
		backend := NewMemoryRevocationBackend()
		defer backend.Close()
		
		now := time.Now()
		
		// Add mixed tokens
		backend.addRevokedTokenForTest("active", &RevokedToken{
			TokenID:   "active",
			RevokedAt: now,
			ExpiresAt: now.Add(time.Hour),
		})
		backend.addRevokedTokenForTest("expired1", &RevokedToken{
			TokenID:   "expired1",
			UserID:    "user1",
			RevokedAt: now.Add(-2 * time.Hour),
			ExpiresAt: now.Add(-time.Hour),
		})
		backend.addRevokedTokenForTest("expired2", &RevokedToken{
			TokenID:   "expired2",
			RevokedAt: now.Add(-3 * time.Hour),
			ExpiresAt: now.Add(-2 * time.Hour),
		})
		
		// Track user tokens
		backend.setUserTokensForTest("user1", []string{"expired1", "other-token"})
		
		err := backend.CleanupExpired()
		assert.NoError(t, err)
		
		// Verify cleanup
		assert.Equal(t, 1, backend.getRevokedTokenCountForTest())
		assert.True(t, backend.containsRevokedTokenForTest("active"))
		assert.False(t, backend.containsRevokedTokenForTest("expired1"))
		assert.False(t, backend.containsRevokedTokenForTest("expired2"))
		
		// Verify user token list was cleaned
		userTokens := backend.getUserTokensForTest("user1")
		assert.Len(t, userTokens, 1)
		assert.Equal(t, "other-token", userTokens[0])
	})
	
	t.Run("TrackUserToken", func(t *testing.T) {
		backend := NewMemoryRevocationBackend()
		defer backend.Close()
		
		// Track tokens
		backend.TrackUserToken("user1", "token1")
		backend.TrackUserToken("user1", "token2")
		backend.TrackUserToken("user2", "token3")
		
		// Verify tracking
		assert.Len(t, backend.userTokens["user1"], 2)
		assert.Contains(t, backend.userTokens["user1"], "token1")
		assert.Contains(t, backend.userTokens["user1"], "token2")
		assert.Len(t, backend.userTokens["user2"], 1)
		assert.Contains(t, backend.userTokens["user2"], "token3")
		
		// Track duplicate token (should not add)
		backend.TrackUserToken("user1", "token1")
		assert.Len(t, backend.userTokens["user1"], 2)
		
		// Track with empty values (should not add)
		backend.TrackUserToken("", "token4")
		backend.TrackUserToken("user3", "")
		assert.NotContains(t, backend.userTokens, "")
		assert.NotContains(t, backend.userTokens, "user3")
	})
	
	t.Run("Close", func(t *testing.T) {
		backend := NewMemoryRevocationBackend()
		
		// Add some data
		backend.RevokeToken("token1", time.Now().Add(time.Hour))
		backend.TrackUserToken("user1", "token1")
		
		err := backend.Close()
		assert.NoError(t, err)
		assert.False(t, backend.cleanupRunning)
		assert.Empty(t, backend.revokedTokens)
		assert.Empty(t, backend.userTokens)
		
		// Close again should not error
		err = backend.Close()
		assert.NoError(t, err)
	})
	
	t.Run("Concurrent operations", func(t *testing.T) {
		backend := NewMemoryRevocationBackend()
		defer backend.Close()
		
		// Run concurrent operations
		done := make(chan bool)
		
		// Writer goroutines
		for i := 0; i < 5; i++ {
			go func(id int) {
				for j := 0; j < 100; j++ {
					tokenID := fmt.Sprintf("token-%d-%d", id, j)
					backend.RevokeToken(tokenID, time.Now().Add(time.Hour))
				}
				done <- true
			}(i)
		}
		
		// Reader goroutines
		for i := 0; i < 5; i++ {
			go func(id int) {
				for j := 0; j < 100; j++ {
					tokenID := fmt.Sprintf("token-%d-%d", id, j)
					backend.IsTokenRevoked(tokenID)
				}
				done <- true
			}(i)
		}
		
		// Wait for all goroutines
		for i := 0; i < 10; i++ {
			<-done
		}
		
		// Verify some tokens were added
		tokens, err := backend.GetRevokedTokens()
		assert.NoError(t, err)
		assert.NotEmpty(t, tokens)
	})
}

// Test automatic cleanup
func TestMemoryRevocationBackend_AutoCleanup(t *testing.T) {
	// This test is tricky because we need to wait for the cleanup ticker
	// For testing purposes, we'll verify the cleanup mechanism is started
	backend := NewMemoryRevocationBackend()
	
	assert.True(t, backend.cleanupRunning)
	assert.NotNil(t, backend.cleanupTicker)
	
	// Verify cleanup doesn't start twice
	backend.startCleanup()
	assert.True(t, backend.cleanupRunning)
	
	err := backend.Close()
	assert.NoError(t, err)
}

// Test error constants
func TestRevocationErrorConstants(t *testing.T) {
	assert.Equal(t, "token is already revoked", ErrTokenAlreadyRevoked.Error())
	assert.Equal(t, "invalid token ID", ErrInvalidTokenID.Error())
	assert.Equal(t, "token revocation failed", ErrRevocationFailed.Error())
}

