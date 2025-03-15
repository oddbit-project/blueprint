package session

import (
	"encoding/base64"
	"fmt"
	"github.com/google/uuid"
	"github.com/oddbit-project/blueprint/log"
	"net/http"
	"sync"
	"time"
)

const (
	// DefaultSessionCookieName is the default cookie name for storing sessions
	DefaultSessionCookieName = "blueprint_session"

	// DefaultSessionExpiration is the default expiration time for sessions (30 minutes)
	DefaultSessionExpiration = 30 * time.Minute

	// DefaultSessionIdleTimeout is the default idle timeout for sessions (15 minutes)
	DefaultSessionIdleTimeout = 15 * time.Minute

	// DefaultMaxSessions is the default maximum number of sessions to store in memory
	DefaultMaxSessions = 10000

	// DefaultSecure sets the Secure flag on session cookies
	DefaultSecure = true

	// DefaultHttpOnly sets the HttpOnly flag on session cookies
	DefaultHttpOnly = true

	// DefaultSameSite sets the SameSite policy for session cookies
	DefaultSameSite = http.SameSiteStrictMode

	// DefaultCleanupInterval sets how often the session cleanup runs
	DefaultCleanupInterval = 5 * time.Minute

	// ContextSessionKey is the key used to store the session in the gin.Context
	ContextSessionKey = "session"
)

// SessionData represents the session data stored in memory
type SessionData struct {
	Values       map[string]interface{}
	LastAccessed time.Time
	Created      time.Time
	ID           string
}

// SessionError represents session-related errors
type SessionError struct {
	Message string
}

func (e *SessionError) Error() string {
	return e.Message
}

var (
	ErrSessionNotFound = &SessionError{Message: "session not found"}
	ErrSessionExpired  = &SessionError{Message: "session expired"}
	ErrSessionInvalid  = &SessionError{Message: "invalid session"}
)

// SessionConfig holds configuration for the session store
type SessionConfig struct {
	// CookieName is the name of the cookie used to store the session ID
	CookieName string

	// Expiration is the maximum lifetime of a session
	Expiration time.Duration

	// IdleTimeout is the maximum time a session can be inactive
	IdleTimeout time.Duration

	// MaxSessions is the maximum number of sessions to store in memory
	MaxSessions int

	// Secure sets the Secure flag on cookies (should be true in production)
	Secure bool

	// HttpOnly sets the HttpOnly flag on cookies (should be true)
	HttpOnly bool

	// SameSite sets the SameSite policy for cookies
	SameSite http.SameSite

	// Domain sets the domain for the cookie
	Domain string

	// Path sets the path for the cookie
	Path string

	// CleanupInterval sets how often the session cleanup runs
	CleanupInterval time.Duration

	// Logger for the session store
	Logger *log.Logger
}

// DefaultSessionConfig returns a default session configuration
func DefaultSessionConfig() *SessionConfig {
	return &SessionConfig{
		CookieName:      DefaultSessionCookieName,
		Expiration:      DefaultSessionExpiration,
		IdleTimeout:     DefaultSessionIdleTimeout,
		MaxSessions:     DefaultMaxSessions,
		Secure:          DefaultSecure,
		HttpOnly:        DefaultHttpOnly,
		SameSite:        DefaultSameSite,
		Path:            "/",
		Domain:          "",
		CleanupInterval: DefaultCleanupInterval,
		Logger:          nil,
	}
}

// Store represents a session store
type Store interface {
	// Get retrieves a session from the store
	Get(id string) (*SessionData, error)

	// Set saves a session to the store
	Set(id string, session *SessionData) error

	// Delete removes a session from the store
	Delete(id string) error

	// Generate creates a new session and returns its ID
	Generate() (*SessionData, string)

	// StartCleanup starts the cleanup goroutine
	StartCleanup()

	// StopCleanup stops the cleanup goroutine
	StopCleanup()
}

// MemoryStore is an in-memory implementation of the session store
type MemoryStore struct {
	sessions      map[string]*SessionData
	sessionsMutex sync.RWMutex
	config        *SessionConfig
	stopCleanup   chan bool
	cleanupTicker *time.Ticker
	cleanupMutex  sync.Mutex
	cleanupRunning bool
	logger        *log.Logger
}

// NewMemoryStore creates a new memory-based session store
func NewMemoryStore(config *SessionConfig) *MemoryStore {
	if config == nil {
		config = DefaultSessionConfig()
	}

	return &MemoryStore{
		sessions:     make(map[string]*SessionData),
		config:       config,
		stopCleanup:  make(chan bool),
		logger:       config.Logger,
	}
}

// Get retrieves a session from the memory store
func (s *MemoryStore) Get(id string) (*SessionData, error) {
	s.sessionsMutex.RLock()
	session, exists := s.sessions[id]
	s.sessionsMutex.RUnlock()

	if !exists {
		return nil, ErrSessionNotFound
	}

	// Check if the session has expired
	now := time.Now()
	if now.Sub(session.Created) > s.config.Expiration {
		s.Delete(id)
		return nil, ErrSessionExpired
	}

	// Check idle timeout
	if now.Sub(session.LastAccessed) > s.config.IdleTimeout {
		s.Delete(id)
		return nil, ErrSessionExpired
	}

	// Update last accessed time
	session.LastAccessed = now
	s.sessionsMutex.Lock()
	s.sessions[id] = session
	s.sessionsMutex.Unlock()

	return session, nil
}

// Set saves a session to the memory store
func (s *MemoryStore) Set(id string, session *SessionData) error {
	s.sessionsMutex.Lock()
	defer s.sessionsMutex.Unlock()

	// Make sure we don't exceed the maximum number of sessions
	if len(s.sessions) >= s.config.MaxSessions {
		// Find the oldest session and remove it
		var oldestID string
		var oldest time.Time
		first := true

		for key, val := range s.sessions {
			if first || val.LastAccessed.Before(oldest) {
				oldestID = key
				oldest = val.LastAccessed
				first = false
			}
		}

		if oldestID != "" {
			delete(s.sessions, oldestID)
		}
	}

	// Update the last accessed time
	session.LastAccessed = time.Now()
	s.sessions[id] = session
	return nil
}

// Delete removes a session from the memory store
func (s *MemoryStore) Delete(id string) error {
	s.sessionsMutex.Lock()
	delete(s.sessions, id)
	s.sessionsMutex.Unlock()
	return nil
}

// Generate creates a new session and returns its ID
func (s *MemoryStore) Generate() (*SessionData, string) {
	// Generate a random session ID
	id := generateSessionID()
	
	// Create a new session
	session := &SessionData{
		Values:       make(map[string]interface{}),
		LastAccessed: time.Now(),
		Created:      time.Now(),
		ID:           id,
	}

	return session, id
}

// generateSessionID creates a random session ID
func generateSessionID() string {
	return base64.URLEncoding.EncodeToString([]byte(uuid.New().String()))
}

// StartCleanup starts the cleanup goroutine to remove expired sessions
func (s *MemoryStore) StartCleanup() {
	s.cleanupMutex.Lock()
	defer s.cleanupMutex.Unlock()

	if s.cleanupRunning {
		return
	}

	s.cleanupTicker = time.NewTicker(s.config.CleanupInterval)
	s.cleanupRunning = true

	go func() {
		for {
			select {
			case <-s.cleanupTicker.C:
				s.cleanup()
			case <-s.stopCleanup:
				s.cleanupTicker.Stop()
				return
			}
		}
	}()
}

// StopCleanup stops the cleanup goroutine
func (s *MemoryStore) StopCleanup() {
	s.cleanupMutex.Lock()
	defer s.cleanupMutex.Unlock()

	if !s.cleanupRunning {
		return
	}

	s.stopCleanup <- true
	s.cleanupRunning = false
}

// cleanup removes expired sessions from the store
func (s *MemoryStore) cleanup() {
	now := time.Now()
	var expired []string

	// Find expired sessions
	s.sessionsMutex.RLock()
	for id, session := range s.sessions {
		// Check if the session has expired
		if now.Sub(session.Created) > s.config.Expiration {
			expired = append(expired, id)
			continue
		}

		// Check idle timeout
		if now.Sub(session.LastAccessed) > s.config.IdleTimeout {
			expired = append(expired, id)
		}
	}
	s.sessionsMutex.RUnlock()

	// Delete expired sessions
	for _, id := range expired {
		s.Delete(id)
	}

	if s.logger != nil && len(expired) > 0 {
		s.logger.Debug(fmt.Sprintf("Cleaned up %d expired sessions", len(expired)))
	}
}