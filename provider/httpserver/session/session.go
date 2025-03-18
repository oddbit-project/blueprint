package session

import (
	"encoding/json"
	"github.com/oddbit-project/blueprint/log"
	"github.com/oddbit-project/blueprint/provider/kv"
	"github.com/oddbit-project/blueprint/utils"
	"net/http"
	"slices"
	"sync"
	"time"
)

const (
	// DefaultSessionCookieName is the default cookie name for storing sessions
	DefaultSessionCookieName = "blueprint_session"

	// DefaultSessionExpiration is the default expiration time for sessions (30 minutes)
	DefaultSessionExpiration = 1800

	// DefaultSessionIdleTimeout is the default idle timeout for sessions (15 minutes)
	DefaultSessionIdleTimeout = 900

	// DefaultSecure sets the Secure flag on session cookies
	DefaultSecure = true

	// DefaultHttpOnly sets the HttpOnly flag on session cookies
	DefaultHttpOnly = true

	// DefaultSameSite sets the SameSite policy for session cookies
	DefaultSameSite = int(http.SameSiteStrictMode)

	// DefaultCleanupInterval sets how often the session cleanup runs
	DefaultCleanupInterval = 300 // 5 min

	// ContextSessionKey is the key used to store the session in the gin.Context
	ContextSessionKey = "session"

	ErrInvalidExpirationSeconds      = utils.Error("session expiration seconds must be a positive integer")
	ErrInvalidIdleTimeoutSeconds     = utils.Error("session idle timeout seconds must be a positive integer")
	ErrInvalidSameSite               = utils.Error("invalid sameSite value")
	ErrInvalidCleanupIntervalSeconds = utils.Error("session cleanup interval seconds must be a positive integer")
	ErrSessionNotFound               = utils.Error("session not found")
	ErrSessionExpired                = utils.Error("session expired")
)

// SessionData represents the session data stored in memory
type SessionData struct {
	Values       map[string]interface{}
	LastAccessed time.Time
	Created      time.Time
	ID           string
}

// Config holds configuration for the session store
type Config struct {
	CookieName             string `json:"cookieName"`             // CookieName is the name of the cookie used to store the session ID
	ExpirationSeconds      int    `json:"expirationSeconds"`      // Expiration is the maximum lifetime of a session
	IdleTimeoutSeconds     int    `json:"idleTimeoutSeconds"`     // IdleTimeoutSeconds is the maximum time a session can be inactive
	Secure                 bool   `json:"secure"`                 // Secure sets the Secure flag on cookies (should be true in production)
	HttpOnly               bool   `json:"httpOnly"`               // HttpOnly sets the HttpOnly flag on cookies (should be true)
	SameSite               int    `json:"sameSite"`               // SameSite sets the SameSite policy for cookies
	Domain                 string `json:"domain"`                 // Domain sets the domain for the cookie
	Path                   string `json:"path"`                   // Path sets the path for the cookie
	CleanupIntervalSeconds int    `json:"cleanupIntervalSeconds"` // CleanupIntervalSeconds sets how often the session cleanup runs
}

func (c *Config) Validate() error {
	if c.ExpirationSeconds <= 0 {
		return ErrInvalidExpirationSeconds
	}
	if c.IdleTimeoutSeconds <= 0 {
		return ErrInvalidIdleTimeoutSeconds
	}
	if c.CleanupIntervalSeconds <= 0 {
		return ErrInvalidCleanupIntervalSeconds
	}
	if slices.Index([]int{
		int(http.SameSiteDefaultMode),
		int(http.SameSiteLaxMode),
		int(http.SameSiteStrictMode),
		int(http.SameSiteNoneMode)}, c.SameSite) < 0 {
		return ErrInvalidSameSite

	}
	return nil
}

// NewConfig returns a default session configuration
func NewConfig() *Config {
	return &Config{
		CookieName:             DefaultSessionCookieName,
		ExpirationSeconds:      DefaultSessionExpiration,
		IdleTimeoutSeconds:     DefaultSessionIdleTimeout,
		Secure:                 DefaultSecure,
		HttpOnly:               DefaultHttpOnly,
		SameSite:               DefaultSameSite,
		Path:                   "/",
		Domain:                 "",
		CleanupIntervalSeconds: DefaultCleanupInterval,
	}
}

type Store struct {
	backend        kv.KV
	config         *Config
	stopCleanup    chan bool
	cleanupTicker  *time.Ticker
	cleanupMutex   sync.Mutex
	cleanupRunning bool
	logger         *log.Logger
}

// NewStore creates session store
func NewStore(config *Config, backend kv.KV, logger *log.Logger) *Store {
	if config == nil {
		config = NewConfig()
	}

	if backend == nil {
		backend = kv.NewMemoryKV()
	}

	return &Store{
		backend:      backend,
		config:       config,
		stopCleanup:  make(chan bool),
		logger:       logger,
		cleanupMutex: sync.Mutex{},
	}
}

// Get retrieves a session from Client
func (s *Store) Get(id string) (*SessionData, error) {
	data, err := s.backend.Get(id)
	if err != nil {
		return nil, err
	}
	if data == nil {
		return nil, ErrSessionNotFound
	}

	// Deserialize the session
	var session SessionData
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, err
	}

	// Check if the session has expired
	now := time.Now()
	if now.Sub(session.Created) > time.Duration(s.config.ExpirationSeconds)*time.Second {
		s.backend.Delete(id)
		return nil, ErrSessionExpired
	}

	// Check idle timeout
	if now.Sub(session.LastAccessed) > time.Duration(s.config.IdleTimeoutSeconds)*time.Second {
		s.backend.Delete(id)
		return nil, ErrSessionExpired
	}

	// Update last accessed time
	err = s.Set(id, &session)
	if err != nil {
		// Log the error but return the session anyway
		if s.logger != nil {
			s.logger.Error(err, "Failed to update session last accessed time")
		}
	}

	return &session, nil
}

// Set saves a session
func (s *Store) Set(id string, session *SessionData) error {
	// Update last accessed time
	session.LastAccessed = time.Now()

	// Serialize the session
	data, err := json.Marshal(session)
	if err != nil {
		return err
	}

	// Calculate expiration time (use the smaller of Expiration and IdleTimeout)
	expiration := time.Duration(s.config.ExpirationSeconds) * time.Second
	if time.Duration(s.config.IdleTimeoutSeconds)*time.Second < expiration {
		expiration = time.Duration(s.config.IdleTimeoutSeconds) * time.Second
	}

	// Save with expiration
	return s.backend.SetTTL(id, data, expiration)
}

// Delete removes a session from Client
func (s *Store) Delete(id string) error {
	return s.backend.Delete(id)
}

// Generate creates a new session and returns its ID
func (s *Store) Generate() (*SessionData, string) {
	id := generateSessionID()
	session := &SessionData{
		Values:       make(map[string]interface{}),
		LastAccessed: time.Now(),
		Created:      time.Now(),
		ID:           id,
	}

	return session, id
}

// StartCleanup is a no-op for Client as Client handles expiration
func (s *Store) StartCleanup() {
	s.cleanupMutex.Lock()
	defer s.cleanupMutex.Unlock()

	if s.cleanupRunning {
		return
	}

	s.cleanupTicker = time.NewTicker(time.Duration(s.config.CleanupIntervalSeconds) * time.Second)
	s.cleanupRunning = true

	go func() {
		for {
			select {
			case <-s.cleanupTicker.C:
				err := s.backend.Prune()
				if s.logger != nil {
					if err != nil {
						s.logger.Error(err, "Failed to cleanup sessions")
					}
					s.logger.Debug("pruned expired sessions")
				}

			case <-s.stopCleanup:
				s.cleanupTicker.Stop()
				return
			}
		}
	}()
}

// StopCleanup stops the cleanup goroutine
func (s *Store) StopCleanup() {
	s.cleanupMutex.Lock()
	defer s.cleanupMutex.Unlock()

	if !s.cleanupRunning {
		return
	}

	s.stopCleanup <- true
	s.cleanupRunning = false
}

// Close closes the store
func (s *Store) Close() {
	s.StopCleanup()
	// Note: backend may require manual closing
}
