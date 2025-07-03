package storage

import (
	"sync"
	"time"

	"github.com/oddbit-project/blueprint/provider/httpserver/fingerprint"
)

// memorySecurityStorage is an in-memory implementation of SecurityStorage
type memorySecurityStorage struct {
	nonces             map[string]int64                       // nonce -> expires_at
	deviceFingerprints map[string]*fingerprint.DeviceFingerprint // sessionID -> fingerprint
	blockedDevices     map[string]int64                       // fingerprint -> blocked_until
	userSessions       map[string][]string                    // userID -> []sessionID
	securityContexts   map[string]*SessionSecurityContext    // sessionID -> context
	mutex              sync.RWMutex
}

// NewMemorySecurityStorage creates a new in-memory security storage
func NewMemorySecurityStorage() SecurityStorage {
	return &memorySecurityStorage{
		nonces:             make(map[string]int64),
		deviceFingerprints: make(map[string]*fingerprint.DeviceFingerprint),
		blockedDevices:     make(map[string]int64),
		userSessions:       make(map[string][]string),
		securityContexts:   make(map[string]*SessionSecurityContext),
		mutex:              sync.RWMutex{},
	}
}

// StoreNonce stores a nonce with TTL for replay prevention
func (m *memorySecurityStorage) StoreNonce(nonce string, ttl time.Duration) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	expiresAt := time.Now().Add(ttl).Unix()
	m.nonces[nonce] = expiresAt
	return nil
}

// NonceExists checks if a nonce exists and is not expired
func (m *memorySecurityStorage) NonceExists(nonce string) bool {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	expiresAt, exists := m.nonces[nonce]
	if !exists {
		return false
	}
	
	// Check if expired
	if time.Now().Unix() > expiresAt {
		// Clean up expired nonce
		delete(m.nonces, nonce)
		return false
	}
	
	return true
}

// StoreDeviceFingerprint stores a device fingerprint for a session
func (m *memorySecurityStorage) StoreDeviceFingerprint(sessionID string, fp *fingerprint.DeviceFingerprint) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	// Create a copy to avoid mutation
	fingerprintCopy := *fp
	m.deviceFingerprints[sessionID] = &fingerprintCopy
	return nil
}

// GetDeviceFingerprint retrieves a device fingerprint for a session
func (m *memorySecurityStorage) GetDeviceFingerprint(sessionID string) (*fingerprint.DeviceFingerprint, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	
	fp, exists := m.deviceFingerprints[sessionID]
	if !exists {
		return nil, nil
	}
	
	// Return a copy to avoid mutation
	fingerprintCopy := *fp
	return &fingerprintCopy, nil
}

// DeleteDeviceFingerprint removes a device fingerprint
func (m *memorySecurityStorage) DeleteDeviceFingerprint(sessionID string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	delete(m.deviceFingerprints, sessionID)
	return nil
}

// BlockDevice blocks a device fingerprint until the specified time
func (m *memorySecurityStorage) BlockDevice(fingerprint string, until time.Time) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	m.blockedDevices[fingerprint] = until.Unix()
	return nil
}

// IsDeviceBlocked checks if a device fingerprint is currently blocked
func (m *memorySecurityStorage) IsDeviceBlocked(fingerprint string) bool {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	blockedUntil, exists := m.blockedDevices[fingerprint]
	if !exists {
		return false
	}
	
	// Check if block has expired
	if time.Now().Unix() > blockedUntil {
		// Clean up expired block
		delete(m.blockedDevices, fingerprint)
		return false
	}
	
	return true
}

// UnblockDevice removes a device block
func (m *memorySecurityStorage) UnblockDevice(fingerprint string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	delete(m.blockedDevices, fingerprint)
	return nil
}

// TrackUserSession adds a session to a user's session list
func (m *memorySecurityStorage) TrackUserSession(userID, sessionID string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	sessions := m.userSessions[userID]
	
	// Check if session already tracked
	for _, sid := range sessions {
		if sid == sessionID {
			return nil // Already tracked
		}
	}
	
	// Add session to user's list
	m.userSessions[userID] = append(sessions, sessionID)
	return nil
}

// GetUserSessions returns all sessions for a user
func (m *memorySecurityStorage) GetUserSessions(userID string) []string {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	
	sessions := m.userSessions[userID]
	if sessions == nil {
		return []string{}
	}
	
	// Return a copy to avoid mutation
	result := make([]string, len(sessions))
	copy(result, sessions)
	return result
}

// RemoveUserSession removes a session from a user's session list
func (m *memorySecurityStorage) RemoveUserSession(userID, sessionID string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	sessions := m.userSessions[userID]
	if sessions == nil {
		return nil
	}
	
	// Find and remove session
	for i, sid := range sessions {
		if sid == sessionID {
			// Remove session by slicing
			m.userSessions[userID] = append(sessions[:i], sessions[i+1:]...)
			break
		}
	}
	
	// Clean up empty slice
	if len(m.userSessions[userID]) == 0 {
		delete(m.userSessions, userID)
	}
	
	return nil
}

// StoreSecurityContext stores security context for a session
func (m *memorySecurityStorage) StoreSecurityContext(sessionID string, context *SessionSecurityContext) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	// Create a deep copy to avoid mutation
	contextCopy := &SessionSecurityContext{
		DeviceFingerprint: context.DeviceFingerprint,
		FirstSeen:         context.FirstSeen,
		LastActivity:      context.LastActivity,
		FailedAttempts:    context.FailedAttempts,
		SuspiciousFlags:   make([]string, len(context.SuspiciousFlags)),
		UserID:            context.UserID,
	}
	copy(contextCopy.SuspiciousFlags, context.SuspiciousFlags)
	
	// Copy device fingerprint if present
	if context.DeviceFingerprint != nil {
		fingerprintCopy := *context.DeviceFingerprint
		contextCopy.DeviceFingerprint = &fingerprintCopy
	}
	
	m.securityContexts[sessionID] = contextCopy
	return nil
}

// GetSecurityContext retrieves security context for a session
func (m *memorySecurityStorage) GetSecurityContext(sessionID string) (*SessionSecurityContext, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	
	context, exists := m.securityContexts[sessionID]
	if !exists {
		return nil, nil
	}
	
	// Return a deep copy to avoid mutation
	contextCopy := &SessionSecurityContext{
		DeviceFingerprint: context.DeviceFingerprint,
		FirstSeen:         context.FirstSeen,
		LastActivity:      context.LastActivity,
		FailedAttempts:    context.FailedAttempts,
		SuspiciousFlags:   make([]string, len(context.SuspiciousFlags)),
		UserID:            context.UserID,
	}
	copy(contextCopy.SuspiciousFlags, context.SuspiciousFlags)
	
	// Copy device fingerprint if present
	if context.DeviceFingerprint != nil {
		fingerprintCopy := *context.DeviceFingerprint
		contextCopy.DeviceFingerprint = &fingerprintCopy
	}
	
	return contextCopy, nil
}

// DeleteSecurityContext removes security context for a session
func (m *memorySecurityStorage) DeleteSecurityContext(sessionID string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	delete(m.securityContexts, sessionID)
	return nil
}

// PruneExpired removes all expired data (nonces and blocks)
func (m *memorySecurityStorage) PruneExpired() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	now := time.Now().Unix()
	
	// Clean up expired nonces
	for nonce, expiresAt := range m.nonces {
		if now > expiresAt {
			delete(m.nonces, nonce)
		}
	}
	
	// Clean up expired device blocks
	for fingerprint, blockedUntil := range m.blockedDevices {
		if now > blockedUntil {
			delete(m.blockedDevices, fingerprint)
		}
	}
	
	return nil
}

// Close cleans up resources (no-op for memory storage)
func (m *memorySecurityStorage) Close() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	// Clear all data
	m.nonces = make(map[string]int64)
	m.deviceFingerprints = make(map[string]*fingerprint.DeviceFingerprint)
	m.blockedDevices = make(map[string]int64)
	m.userSessions = make(map[string][]string)
	m.securityContexts = make(map[string]*SessionSecurityContext)
	
	return nil
}