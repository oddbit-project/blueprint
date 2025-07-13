package jwtprovider

import (
	"sync"
	"time"
)

// MemoryRevocationBackend implements RevocationBackend using in-memory storage
type MemoryRevocationBackend struct {
	revokedTokens  map[string]*RevokedToken
	userTokens     map[string][]string // userID -> []tokenID
	tokenMetadata  map[string]*TokenMetadata // tokenID -> metadata
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
		tokenMetadata: make(map[string]*TokenMetadata),
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

	now := time.Now()
	revokedToken, exists := m.revokedTokens[tokenID]

	// Constant-time check: always perform the same operations
	isRevoked := exists && !now.After(revokedToken.ExpiresAt)

	// Cleanup expired tokens (if needed) after the timing-sensitive check
	if exists && now.After(revokedToken.ExpiresAt) {
		// Promote to write lock for cleanup
		m.mutex.RUnlock()
		m.mutex.Lock()
		// Double-check after acquiring write lock
		if token, stillExists := m.revokedTokens[tokenID]; stillExists && now.After(token.ExpiresAt) {
			delete(m.revokedTokens, tokenID)
		}
		m.mutex.Unlock()
		m.mutex.RLock() // Reacquire read lock for defer
	}

	return isRevoked
}

// RevokeAllUserTokens revokes all tokens for a specific user
func (m *MemoryRevocationBackend) RevokeAllUserTokens(userID string, issuedBefore time.Time) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	tokenIDs, exists := m.userTokens[userID]
	if !exists || len(tokenIDs) == 0 {
		return nil
	}

	revokedCount := 0
	now := time.Now()
	
	for _, tokenID := range tokenIDs {
		// Skip if already revoked
		if _, alreadyRevoked := m.revokedTokens[tokenID]; alreadyRevoked {
			continue
		}

		// Get token metadata for proper expiration
		var expiresAt time.Time
		if metadata, hasMetadata := m.tokenMetadata[tokenID]; hasMetadata {
			expiresAt = metadata.ExpiresAt
		} else {
			// Fallback to 24 hours if no metadata
			expiresAt = now.Add(24 * time.Hour)
		}

		// Create revocation entry
		m.revokedTokens[tokenID] = &RevokedToken{
			TokenID:   tokenID,
			UserID:    userID,
			RevokedAt: now,
			ExpiresAt: expiresAt,
		}
		revokedCount++
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
	
	// Clean expired revocations
	for tokenID, revokedToken := range m.revokedTokens {
		if now.After(revokedToken.ExpiresAt) {
			delete(m.revokedTokens, tokenID)
			m.removeUserToken(revokedToken.UserID, tokenID)
		}
	}
	
	// Clean expired metadata
	for tokenID, metadata := range m.tokenMetadata {
		if now.After(metadata.ExpiresAt) {
			delete(m.tokenMetadata, tokenID)
			m.removeUserToken(metadata.UserID, tokenID)
		}
	}

	return nil
}

// removeUserToken removes a token from user's token list (must be called with lock held)
func (m *MemoryRevocationBackend) removeUserToken(userID, tokenID string) {
	if userID == "" {
		return
	}
	
	if tokenIDs, exists := m.userTokens[userID]; exists {
		newTokenIDs := make([]string, 0, len(tokenIDs))
		for _, id := range tokenIDs {
			if id != tokenID {
				newTokenIDs = append(newTokenIDs, id)
			}
		}
		
		if len(newTokenIDs) == 0 {
			delete(m.userTokens, userID)
		} else {
			m.userTokens[userID] = newTokenIDs
		}
	}
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
	m.tokenMetadata = make(map[string]*TokenMetadata)

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
func (m *MemoryRevocationBackend) TrackUserToken(userID, tokenID string, expiresAt time.Time) {
	if userID == "" || tokenID == "" {
		return
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Store token metadata
	m.tokenMetadata[tokenID] = &TokenMetadata{
		TokenID:   tokenID,
		UserID:    userID,
		IssuedAt:  time.Now(),
		ExpiresAt: expiresAt,
	}

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

// GetUserTokens returns all active tokens for a user
func (m *MemoryRevocationBackend) GetUserTokens(userID string) []string {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	
	tokens := make([]string, 0)
	if tokenIDs, exists := m.userTokens[userID]; exists {
		// Return a copy to avoid race conditions
		tokens = make([]string, 0, len(tokenIDs))
		// Filter out revoked tokens
		for _, tokenID := range tokenIDs {
			if _, isRevoked := m.revokedTokens[tokenID]; !isRevoked {
				tokens = append(tokens, tokenID)
			}
		}
	}
	
	return tokens
}

// Test helper methods for safe access to internal state
// These methods should only be used in tests

// hasRevokedToken checks if a token exists in the revoked tokens map (for testing)
func (m *MemoryRevocationBackend) hasRevokedToken(tokenID string) bool {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	_, exists := m.revokedTokens[tokenID]
	return exists
}
