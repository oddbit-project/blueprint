package jwtprovider

import (
	"sync"
	"time"
)

// MemoryRevocationBackend implements RevocationBackend using in-memory storage
type MemoryRevocationBackend struct {
	revokedTokens  map[string]*RevokedToken
	userTokens     map[string][]string // userID -> []tokenID
	mutex          sync.RWMutex
	cleanupTicker  *time.Ticker
	stopCleanup    chan bool
	cleanupRunning bool
}

// NewMemoryRevocationBackend creates a new in-memory revocation backend
func NewMemoryRevocationBackend() *MemoryRevocationBackend {
	backend := &MemoryRevocationBackend{
		revokedTokens: make(map[string]*RevokedToken),
		userTokens:    make(map[string][]string),
		stopCleanup:   make(chan bool),
	}

	// Start automatic cleanup every 10 minutes
	backend.startCleanup()

	return backend
}

// RevokeToken revokes a token by its ID
func (m *MemoryRevocationBackend) RevokeToken(tokenID string, expiresAt time.Time) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Store revoked token
	revokedToken := &RevokedToken{
		TokenID:   tokenID,
		RevokedAt: time.Now(),
		ExpiresAt: expiresAt,
	}

	m.revokedTokens[tokenID] = revokedToken

	return nil
}

// IsTokenRevoked checks if a token is revoked
func (m *MemoryRevocationBackend) IsTokenRevoked(tokenID string) bool {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	revokedToken, exists := m.revokedTokens[tokenID]
	if !exists {
		return false
	}

	// Check if revocation has expired
	if time.Now().After(revokedToken.ExpiresAt) {
		// Clean up expired revocation entry
		go func() {
			m.mutex.Lock()
			delete(m.revokedTokens, tokenID)
			m.mutex.Unlock()
		}()
		return false
	}

	return true
}

// RevokeAllUserTokens revokes all tokens for a specific user
func (m *MemoryRevocationBackend) RevokeAllUserTokens(userID string, issuedBefore time.Time) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Get all tokens for this user
	tokenIDs, exists := m.userTokens[userID]
	if !exists {
		return nil // No tokens to revoke
	}

	// Revoke each token issued before the specified time
	for _, tokenID := range tokenIDs {
		if revokedToken, exists := m.revokedTokens[tokenID]; exists {
			// If token already revoked, check if it was issued before
			if revokedToken.RevokedAt.Before(issuedBefore) {
				continue
			}
		}

		// Create a far-future expiration for user token revocation
		// In practice, you'd want to set this to the original token expiration
		farFuture := time.Now().Add(24 * time.Hour * 365) // 1 year

		m.revokedTokens[tokenID] = &RevokedToken{
			TokenID:   tokenID,
			UserID:    userID,
			RevokedAt: time.Now(),
			ExpiresAt: farFuture,
		}
	}

	return nil
}

// GetRevokedTokens returns all revoked tokens
func (m *MemoryRevocationBackend) GetRevokedTokens() ([]RevokedToken, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	tokens := make([]RevokedToken, 0, len(m.revokedTokens))
	for _, token := range m.revokedTokens {
		// Only return non-expired revocations
		if time.Now().Before(token.ExpiresAt) {
			tokens = append(tokens, *token)
		}
	}

	return tokens, nil
}

// CleanupExpired removes expired revocation entries
func (m *MemoryRevocationBackend) CleanupExpired() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	now := time.Now()
	for tokenID, revokedToken := range m.revokedTokens {
		if now.After(revokedToken.ExpiresAt) {
			delete(m.revokedTokens, tokenID)

			// Also clean up user token mapping
			if revokedToken.UserID != "" {
				if tokenIDs, exists := m.userTokens[revokedToken.UserID]; exists {
					// Remove token from user's token list
					newTokenIDs := make([]string, 0, len(tokenIDs))
					for _, id := range tokenIDs {
						if id != tokenID {
							newTokenIDs = append(newTokenIDs, id)
						}
					}

					if len(newTokenIDs) == 0 {
						delete(m.userTokens, revokedToken.UserID)
					} else {
						m.userTokens[revokedToken.UserID] = newTokenIDs
					}
				}
			}
		}
	}

	return nil
}

// Close stops the cleanup process and releases resources
func (m *MemoryRevocationBackend) Close() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.cleanupRunning {
		m.stopCleanup <- true
		m.cleanupRunning = false
	}

	// Clear all data
	m.revokedTokens = make(map[string]*RevokedToken)
	m.userTokens = make(map[string][]string)

	return nil
}

// startCleanup starts the background cleanup process
func (m *MemoryRevocationBackend) startCleanup() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.cleanupRunning {
		return
	}

	m.cleanupTicker = time.NewTicker(10 * time.Minute)
	m.cleanupRunning = true

	go func() {
		for {
			select {
			case <-m.cleanupTicker.C:
				_ = m.CleanupExpired()
			case <-m.stopCleanup:
				m.cleanupTicker.Stop()
				return
			}
		}
	}()
}

// TrackUserToken associates a token with a user for bulk revocation
func (m *MemoryRevocationBackend) TrackUserToken(userID, tokenID string) {
	if userID == "" || tokenID == "" {
		return
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	if tokenIDs, exists := m.userTokens[userID]; exists {
		// Check if token is already tracked
		for _, id := range tokenIDs {
			if id == tokenID {
				return
			}
		}
		m.userTokens[userID] = append(tokenIDs, tokenID)
	} else {
		m.userTokens[userID] = []string{tokenID}
	}
}
