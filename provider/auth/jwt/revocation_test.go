package jwt

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRevocationManager(t *testing.T) {
	t.Run("With backend", func(t *testing.T) {
		backend := NewMemoryRevocationBackend()
		manager := NewRevocationManager(backend)
		assert.NotNil(t, manager)
		assert.Equal(t, backend, manager.backend)
	})

	t.Run("With nil backend", func(t *testing.T) {
		manager := NewRevocationManager(nil)
		assert.NotNil(t, manager)
		assert.NotNil(t, manager.backend)
	})
}

func TestRevocationManagerRevokeToken(t *testing.T) {
	manager := NewRevocationManager(NewMemoryRevocationBackend())
	expiresAt := time.Now().Add(time.Hour)

	t.Run("Valid revocation", func(t *testing.T) {
		err := manager.RevokeToken("token-123", expiresAt)
		assert.NoError(t, err)

		isRevoked := manager.IsTokenRevoked("token-123")
		assert.True(t, isRevoked)
	})

	t.Run("Empty token ID", func(t *testing.T) {
		err := manager.RevokeToken("", expiresAt)
		assert.Error(t, err)
		assert.Equal(t, ErrInvalidTokenID, err)
	})

	t.Run("Already revoked token", func(t *testing.T) {
		tokenID := "already-revoked-token"
		err := manager.RevokeToken(tokenID, expiresAt)
		require.NoError(t, err)

		err = manager.RevokeToken(tokenID, expiresAt)
		assert.Error(t, err)
		assert.Equal(t, ErrTokenAlreadyRevoked, err)
	})
}

func TestRevocationManagerRevokeTokenWithReason(t *testing.T) {
	manager := NewRevocationManager(NewMemoryRevocationBackend())
	expiresAt := time.Now().Add(time.Hour)

	err := manager.RevokeTokenWithReason("token-123", "user-123", expiresAt, RevocationReasonLogout, "admin")
	assert.NoError(t, err)

	isRevoked := manager.IsTokenRevoked("token-123")
	assert.True(t, isRevoked)
}

func TestRevocationManagerIsTokenRevoked(t *testing.T) {
	manager := NewRevocationManager(NewMemoryRevocationBackend())

	t.Run("Empty token ID", func(t *testing.T) {
		isRevoked := manager.IsTokenRevoked("")
		assert.False(t, isRevoked)
	})

	t.Run("Non-revoked token", func(t *testing.T) {
		isRevoked := manager.IsTokenRevoked("non-revoked-token")
		assert.False(t, isRevoked)
	})

	t.Run("Revoked token", func(t *testing.T) {
		tokenID := "revoked-token"
		expiresAt := time.Now().Add(time.Hour)
		err := manager.RevokeToken(tokenID, expiresAt)
		require.NoError(t, err)

		isRevoked := manager.IsTokenRevoked(tokenID)
		assert.True(t, isRevoked)
	})
}

func TestRevocationManagerRevokeAllUserTokens(t *testing.T) {
	manager := NewRevocationManager(NewMemoryRevocationBackend())
	issuedBefore := time.Now()

	t.Run("Valid user ID", func(t *testing.T) {
		err := manager.RevokeAllUserTokens("user-123", issuedBefore)
		assert.NoError(t, err)
	})

	t.Run("Empty user ID", func(t *testing.T) {
		err := manager.RevokeAllUserTokens("", issuedBefore)
		assert.Error(t, err)
		assert.Equal(t, ErrInvalidTokenID, err)
	})
}

func TestRevocationManagerCleanupExpired(t *testing.T) {
	manager := NewRevocationManager(NewMemoryRevocationBackend())

	err := manager.CleanupExpired()
	assert.NoError(t, err)
}

func TestRevocationManagerGetRevokedTokens(t *testing.T) {
	manager := NewRevocationManager(NewMemoryRevocationBackend())
	expiresAt := time.Now().Add(time.Hour)

	// Initially empty
	tokens, err := manager.GetRevokedTokens()
	assert.NoError(t, err)
	assert.Empty(t, tokens)

	// Add some revoked tokens
	err = manager.RevokeToken("token-1", expiresAt)
	require.NoError(t, err)
	err = manager.RevokeToken("token-2", expiresAt)
	require.NoError(t, err)

	tokens, err = manager.GetRevokedTokens()
	assert.NoError(t, err)
	assert.Len(t, tokens, 2)
}

func TestRevocationManagerClose(t *testing.T) {
	manager := NewRevocationManager(NewMemoryRevocationBackend())

	err := manager.Close()
	assert.NoError(t, err)

	// Test with nil backend
	manager.backend = nil
	err = manager.Close()
	assert.NoError(t, err)
}

func TestMemoryRevocationBackend(t *testing.T) {
	backend := NewMemoryRevocationBackend()
	defer backend.Close()

	t.Run("RevokeToken", func(t *testing.T) {
		expiresAt := time.Now().Add(time.Hour)
		err := backend.RevokeToken("token-123", expiresAt)
		assert.NoError(t, err)
	})

	t.Run("IsTokenRevoked", func(t *testing.T) {
		expiresAt := time.Now().Add(time.Hour)
		
		// Not revoked initially
		isRevoked := backend.IsTokenRevoked("token-456")
		assert.False(t, isRevoked)

		// Revoke token
		err := backend.RevokeToken("token-456", expiresAt)
		require.NoError(t, err)

		// Should be revoked now
		isRevoked = backend.IsTokenRevoked("token-456")
		assert.True(t, isRevoked)
	})

	t.Run("IsTokenRevoked with expired revocation", func(t *testing.T) {
		expiresAt := time.Now().Add(-time.Hour) // Already expired
		err := backend.RevokeToken("token-expired", expiresAt)
		require.NoError(t, err)

		// Should not be considered revoked since revocation expired
		isRevoked := backend.IsTokenRevoked("token-expired")
		assert.False(t, isRevoked)
	})

	t.Run("RevokeAllUserTokens", func(t *testing.T) {
		issuedBefore := time.Now()
		err := backend.RevokeAllUserTokens("user-123", issuedBefore)
		assert.NoError(t, err)
	})

	t.Run("GetRevokedTokens", func(t *testing.T) {
		tokens, err := backend.GetRevokedTokens()
		assert.NoError(t, err)
		assert.NotNil(t, tokens)
	})

	t.Run("CleanupExpired", func(t *testing.T) {
		// Add an expired revocation
		expiredTime := time.Now().Add(-time.Hour)
		err := backend.RevokeToken("expired-token", expiredTime)
		require.NoError(t, err)

		// Add a valid revocation
		validTime := time.Now().Add(time.Hour)
		err = backend.RevokeToken("valid-token", validTime)
		require.NoError(t, err)

		// Cleanup expired
		err = backend.CleanupExpired()
		assert.NoError(t, err)

		// Expired token should not be revoked anymore
		isRevoked := backend.IsTokenRevoked("expired-token")
		assert.False(t, isRevoked)

		// Valid token should still be revoked
		isRevoked = backend.IsTokenRevoked("valid-token")
		assert.True(t, isRevoked)
	})

	t.Run("TrackUserToken", func(t *testing.T) {
		backend.TrackUserToken("user-123", "token-1")
		backend.TrackUserToken("user-123", "token-2")
		backend.TrackUserToken("user-456", "token-3")

		// Test duplicate tracking
		backend.TrackUserToken("user-123", "token-1") // Should not add duplicate

		// Test with empty values
		backend.TrackUserToken("", "token-4") // Should be ignored
		backend.TrackUserToken("user-123", "") // Should be ignored
	})

	t.Run("Close", func(t *testing.T) {
		testBackend := NewMemoryRevocationBackend()
		err := testBackend.Close()
		assert.NoError(t, err)

		// Verify cleanup was called
		assert.Empty(t, testBackend.revokedTokens)
		assert.Empty(t, testBackend.userTokens)
	})
}

func TestMemoryRevocationBackendConcurrency(t *testing.T) {
	backend := NewMemoryRevocationBackend()
	defer backend.Close()

	// Test concurrent operations
	done := make(chan bool, 10)
	
	// Concurrent revocations
	for i := 0; i < 5; i++ {
		go func(id int) {
			tokenID := fmt.Sprintf("token-%d", id)
			expiresAt := time.Now().Add(time.Hour)
			err := backend.RevokeToken(tokenID, expiresAt)
			assert.NoError(t, err)
			done <- true
		}(i)
	}

	// Concurrent checks
	for i := 0; i < 5; i++ {
		go func(id int) {
			tokenID := fmt.Sprintf("token-%d", id)
			backend.IsTokenRevoked(tokenID)
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestRevokedToken(t *testing.T) {
	token := RevokedToken{
		TokenID:   "token-123",
		UserID:    "user-456",
		RevokedAt: time.Now(),
		ExpiresAt: time.Now().Add(time.Hour),
		Reason:    string(RevocationReasonLogout),
		RevokedBy: "admin",
	}

	assert.Equal(t, "token-123", token.TokenID)
	assert.Equal(t, "user-456", token.UserID)
	assert.Equal(t, string(RevocationReasonLogout), token.Reason)
	assert.Equal(t, "admin", token.RevokedBy)
}

func TestRevocationReasons(t *testing.T) {
	reasons := []RevocationReason{
		RevocationReasonLogout,
		RevocationReasonPasswordChange,
		RevocationReasonSuspiciousActivity,
		RevocationReasonAdminAction,
		RevocationReasonExpired,
		RevocationReasonSecurityBreach,
	}

	for _, reason := range reasons {
		assert.NotEmpty(t, string(reason))
	}
}