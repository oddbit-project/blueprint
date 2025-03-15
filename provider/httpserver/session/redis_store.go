package session

import (
	"context"
	"encoding/json"
	"github.com/oddbit-project/blueprint/log"
	"github.com/redis/go-redis/v9"
	"sync"
	"time"
)

// RedisConfig holds configuration for the Redis session store
type RedisConfig struct {
	// Address of the Redis server
	Address string

	// Password for Redis authentication
	Password string

	// DB is the Redis database to use
	DB int

	// KeyPrefix is the prefix for session keys in Redis
	KeyPrefix string

	// CleanupInterval is how often cleanup runs
	CleanupInterval time.Duration

	// Logger for the session store
	Logger *log.Logger
}

// DefaultRedisConfig returns a default Redis configuration
func DefaultRedisConfig() *RedisConfig {
	return &RedisConfig{
		Address:         "localhost:6379",
		Password:        "",
		DB:              0,
		KeyPrefix:       "session:",
		CleanupInterval: DefaultCleanupInterval,
		Logger:          nil,
	}
}

// RedisStore is a Redis-based session store
type RedisStore struct {
	client        *redis.Client
	config        *SessionConfig
	redisConfig   *RedisConfig
	stopCleanup   chan bool
	cleanupTicker *time.Ticker
	cleanupMutex  sync.Mutex
	cleanupRunning bool
	logger        *log.Logger
}

// NewRedisStore creates a new Redis-based session store
func NewRedisStore(sessionConfig *SessionConfig, redisConfig *RedisConfig) (*RedisStore, error) {
	if sessionConfig == nil {
		sessionConfig = DefaultSessionConfig()
	}
	if redisConfig == nil {
		redisConfig = DefaultRedisConfig()
	}

	client := redis.NewClient(&redis.Options{
		Addr:     redisConfig.Address,
		Password: redisConfig.Password,
		DB:       redisConfig.DB,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	_, err := client.Ping(ctx).Result()
	if err != nil {
		return nil, err
	}

	return &RedisStore{
		client:       client,
		config:       sessionConfig,
		redisConfig:  redisConfig,
		stopCleanup:  make(chan bool),
		logger:       redisConfig.Logger,
	}, nil
}

// getRedisKey builds the Redis key for a session
func (s *RedisStore) getRedisKey(id string) string {
	return s.redisConfig.KeyPrefix + id
}

// Get retrieves a session from Redis
func (s *RedisStore) Get(id string) (*SessionData, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Get data from Redis
	data, err := s.client.Get(ctx, s.getRedisKey(id)).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, ErrSessionNotFound
		}
		return nil, err
	}

	// Deserialize the session
	var session SessionData
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, err
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
	err = s.Set(id, &session)
	if err != nil {
		// Log the error but return the session anyway
		if s.logger != nil {
			s.logger.Error(err, "Failed to update session last accessed time")
		}
	}

	return &session, nil
}

// Set saves a session to Redis
func (s *RedisStore) Set(id string, session *SessionData) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Update last accessed time
	session.LastAccessed = time.Now()

	// Serialize the session
	data, err := json.Marshal(session)
	if err != nil {
		return err
	}

	// Calculate expiration time (use the smaller of Expiration and IdleTimeout)
	expiration := s.config.Expiration
	if s.config.IdleTimeout < expiration {
		expiration = s.config.IdleTimeout
	}

	// Save to Redis with expiration
	err = s.client.Set(ctx, s.getRedisKey(id), data, expiration).Err()
	return err
}

// Delete removes a session from Redis
func (s *RedisStore) Delete(id string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return s.client.Del(ctx, s.getRedisKey(id)).Err()
}

// Generate creates a new session and returns its ID
func (s *RedisStore) Generate() (*SessionData, string) {
	// For Redis, we don't store the session until Set is called
	// so we just create a new session structure and ID
	id := generateSessionID()
	
	session := &SessionData{
		Values:       make(map[string]interface{}),
		LastAccessed: time.Now(),
		Created:      time.Now(),
		ID:           id,
	}

	return session, id
}

// StartCleanup is a no-op for Redis as Redis handles expiration
func (s *RedisStore) StartCleanup() {
	// Redis handles expiration automatically, so we don't need to run cleanup
	// But we might still want to do some maintenance
	s.cleanupMutex.Lock()
	defer s.cleanupMutex.Unlock()

	if s.cleanupRunning {
		return
	}

	s.cleanupTicker = time.NewTicker(s.redisConfig.CleanupInterval)
	s.cleanupRunning = true

	go func() {
		for {
			select {
			case <-s.cleanupTicker.C:
				// Optional maintenance tasks could be added here
			case <-s.stopCleanup:
				s.cleanupTicker.Stop()
				return
			}
		}
	}()
}

// StopCleanup stops the cleanup goroutine
func (s *RedisStore) StopCleanup() {
	s.cleanupMutex.Lock()
	defer s.cleanupMutex.Unlock()

	if !s.cleanupRunning {
		return
	}

	s.stopCleanup <- true
	s.cleanupRunning = false
}

// Close closes the Redis connection
func (s *RedisStore) Close() error {
	s.StopCleanup()
	return s.client.Close()
}