package session

import (
	"bytes"
	"encoding/gob"
	"github.com/oddbit-project/blueprint/crypt/secure"
	"github.com/oddbit-project/blueprint/log"
	"github.com/oddbit-project/blueprint/provider/kv"
	"sync"
	"time"
)

type Store struct {
	backend        kv.KV
	config         *Config
	stopCleanup    chan bool
	cleanupTicker  *time.Ticker
	cleanupMutex   sync.Mutex
	cleanupRunning bool
	logger         *log.Logger
	crypt          secure.AES256GCM
}

// NewStore creates session store
// if no backend is specified, a memory KV is used
func NewStore(config *Config, backend kv.KV, logger *log.Logger) (*Store, error) {
	if config == nil {
		config = NewConfig()
	}

	if backend == nil {
		backend = kv.NewMemoryKV()
	}

	if logger == nil {
		logger = log.New("session-store")
	}

	// if key exists, activate encryption
	var enc secure.AES256GCM
	if !config.EncryptionKey.IsEmpty() {
		secret, err := config.EncryptionKey.Fetch()
		if err != nil {
			return nil, err
		}

		enc, err = secure.NewAES256GCM([]byte(secret))
		if err != nil {
			return nil, err
		}
	}

	return &Store{
		backend:      backend,
		config:       config,
		stopCleanup:  make(chan bool),
		logger:       logger,
		cleanupMutex: sync.Mutex{},
		crypt:        enc,
	}, nil
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

	// Decrypt the data, if necessary
	if s.crypt != nil {
		data, err = s.crypt.Decrypt(data)
	}

	// Deserialize the session
	var session *SessionData
	if session, err = unmarshalSession(data); err != nil {
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
	err = s.Set(id, session)
	if err != nil {
		// Log the error but return the session anyway
		if s.logger != nil {
			s.logger.Error(err, "Failed to update session last accessed time")
		}
	}

	return session, nil
}

// Set saves a session
func (s *Store) Set(id string, session *SessionData) error {
	// Update last accessed time
	session.LastAccessed = time.Now()

	// Serialize the session
	data, err := marshalSession(session)
	if err != nil {
		return err
	}

	// Calculate expiration time (use the smaller of expiration and IdleTimeout)
	expiration := time.Duration(s.config.ExpirationSeconds) * time.Second
	if time.Duration(s.config.IdleTimeoutSeconds)*time.Second < expiration {
		expiration = time.Duration(s.config.IdleTimeoutSeconds) * time.Second
	}

	// encrypt data if crypt is configured
	if s.crypt != nil {
		data, err = s.crypt.Encrypt(data)
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
		Values:       make(map[string]any),
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

// marshalSession use gob to marshal session
func marshalSession(session *SessionData) ([]byte, error) {
	var b bytes.Buffer
	enc := gob.NewEncoder(&b)
	if err := enc.Encode(session); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// unmarshalSession use gob to unmarshal session
func unmarshalSession(data []byte) (*SessionData, error) {
	var session SessionData
	dec := gob.NewDecoder(bytes.NewBuffer(data))
	if err := dec.Decode(&session); err != nil {
		return nil, err
	}
	return &session, nil
}

func init() {
	// register type to be used with session data
	gob.Register(&SessionData{})
}
