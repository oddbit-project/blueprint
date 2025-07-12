package jwtprovider

import (
	"time"

	"github.com/oddbit-project/blueprint/utils"
)

const (
	// Revocation-related errors
	ErrTokenAlreadyRevoked = utils.Error("token is already revoked")
	ErrInvalidTokenID      = utils.Error("invalid token ID")
	ErrRevocationFailed    = utils.Error("token revocation failed")
)

// RevocationBackend defines the interface for token revocation storage
type RevocationBackend interface {
	// RevokeToken revokes a token by its ID with an optional expiration time
	RevokeToken(tokenID string, expiresAt time.Time) error

	// IsTokenRevoked checks if a token is revoked
	IsTokenRevoked(tokenID string) bool

	// RevokeAllUserTokens revokes all tokens for a specific user
	RevokeAllUserTokens(userID string, issuedBefore time.Time) error

	// GetRevokedTokens returns all revoked tokens (for admin purposes)
	GetRevokedTokens() ([]RevokedToken, error)

	// CleanupExpired removes expired revocation entries
	CleanupExpired() error

	// Close closes the backend and releases resources
	Close() error
}

// RevokedToken represents a revoked token
type RevokedToken struct {
	TokenID   string    `json:"tokenId"`
	UserID    string    `json:"userId,omitempty"`
	RevokedAt time.Time `json:"revokedAt"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// RevocationManager manages token revocation
type RevocationManager struct {
	backend RevocationBackend
}

// NewRevocationManager creates a new revocation manager
func NewRevocationManager(backend RevocationBackend) *RevocationManager {
	if backend == nil {
		backend = NewMemoryRevocationBackend()
	}
	return &RevocationManager{
		backend: backend,
	}
}

// RevokeToken revokes a specific token
func (rm *RevocationManager) RevokeToken(tokenID string, expiresAt time.Time) error {
	if tokenID == "" {
		return ErrInvalidTokenID
	}

	// Check if already revoked
	if rm.backend.IsTokenRevoked(tokenID) {
		return ErrTokenAlreadyRevoked
	}

	return rm.backend.RevokeToken(tokenID, expiresAt)
}

// IsTokenRevoked checks if a token is revoked
func (rm *RevocationManager) IsTokenRevoked(tokenID string) bool {
	if tokenID == "" {
		return false
	}
	return rm.backend.IsTokenRevoked(tokenID)
}

// RevokeAllUserTokens revokes all tokens for a user issued before a specific time
func (rm *RevocationManager) RevokeAllUserTokens(userID string, issuedBefore time.Time) error {
	if userID == "" {
		return ErrInvalidTokenID
	}
	return rm.backend.RevokeAllUserTokens(userID, issuedBefore)
}

// CleanupExpired removes expired revocation entries
func (rm *RevocationManager) CleanupExpired() error {
	return rm.backend.CleanupExpired()
}

// GetRevokedTokens returns all revoked tokens for admin purposes
func (rm *RevocationManager) GetRevokedTokens() ([]RevokedToken, error) {
	return rm.backend.GetRevokedTokens()
}

// Close closes the revocation manager and backend
func (rm *RevocationManager) Close() error {
	if rm.backend != nil {
		return rm.backend.Close()
	}
	return nil
}
