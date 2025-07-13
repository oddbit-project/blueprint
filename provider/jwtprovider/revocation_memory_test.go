package jwtprovider

// addRevokedTokenForTest adds a revoked token directly (for testing)
func (m *MemoryRevocationBackend) addRevokedTokenForTest(tokenID string, revokedToken *RevokedToken) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.revokedTokens[tokenID] = revokedToken
}

// setUserTokensForTest sets user tokens directly (for testing)
func (m *MemoryRevocationBackend) setUserTokensForTest(userID string, tokenIDs []string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.userTokens[userID] = tokenIDs
}

// getRevokedTokenCountForTest returns the number of revoked tokens (for testing)
func (m *MemoryRevocationBackend) getRevokedTokenCountForTest() int {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return len(m.revokedTokens)
}

// containsRevokedTokenForTest checks if a specific token is revoked (for testing)
func (m *MemoryRevocationBackend) containsRevokedTokenForTest(tokenID string) bool {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	_, exists := m.revokedTokens[tokenID]
	return exists
}

// getUserTokensForTest gets user tokens (for testing)
func (m *MemoryRevocationBackend) getUserTokensForTest(userID string) []string {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	if tokens, exists := m.userTokens[userID]; exists {
		result := make([]string, len(tokens))
		copy(result, tokens)
		return result
	}
	return nil
}

// getRevokedTokenForTest gets a revoked token safely (for testing)
func (m *MemoryRevocationBackend) getRevokedTokenForTest(tokenID string) (*RevokedToken, bool) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	if token, exists := m.revokedTokens[tokenID]; exists {
		// Return a copy to avoid race conditions
		tokenCopy := *token
		return &tokenCopy, true
	}
	return nil, false
}
