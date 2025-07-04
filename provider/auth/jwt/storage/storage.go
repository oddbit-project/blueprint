package storage

import (
	"time"

	"github.com/oddbit-project/blueprint/provider/httpserver/fingerprint"
)

// SecurityStorage defines the interface for storing security-related data
type SecurityStorage interface {
	// Nonce management for replay prevention
	StoreNonce(nonce string, ttl time.Duration) error
	NonceExists(nonce string) bool

	// Device fingerprint management
	StoreDeviceFingerprint(sessionID string, fingerprint *fingerprint.DeviceFingerprint) error
	GetDeviceFingerprint(sessionID string) (*fingerprint.DeviceFingerprint, error)
	DeleteDeviceFingerprint(sessionID string) error

	// Device blocking management
	BlockDevice(fingerprint string, until time.Time) error
	IsDeviceBlocked(fingerprint string) bool
	UnblockDevice(fingerprint string) error

	// User session tracking for concurrent limits
	TrackUserSession(userID, sessionID string) error
	GetUserSessions(userID string) []string
	RemoveUserSession(userID, sessionID string) error

	// Security context storage
	StoreSecurityContext(sessionID string, context *SessionSecurityContext) error
	GetSecurityContext(sessionID string) (*SessionSecurityContext, error)
	DeleteSecurityContext(sessionID string) error

	// Cleanup and maintenance
	PruneExpired() error
	Close() error
}

// SessionSecurityContext holds security information for a session
type SessionSecurityContext struct {
	DeviceFingerprint *fingerprint.DeviceFingerprint `json:"device_fingerprint"`
	FirstSeen         int64                          `json:"first_seen"`
	LastActivity      int64                          `json:"last_activity"`
	FailedAttempts    int                            `json:"failed_attempts"`
	SuspiciousFlags   []string                       `json:"suspicious_flags"`
	UserID            string                         `json:"user_id"`
}