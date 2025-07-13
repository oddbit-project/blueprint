package store

import (
	"context"
	"time"

	"github.com/oddbit-project/blueprint/provider/redis"
)

// redisStore implements NonceStore using Redis as the backend
type redisStore struct {
	client  *redis.Client
	ttl     time.Duration
	timeout time.Duration
	prefix  string
}

// RedisStoreOption allows customization of Redis store
type RedisStoreOption func(*redisStore)

// WithTimeout sets the timeout for Redis operations
func WithTimeout(timeout time.Duration) RedisStoreOption {
	return func(rs *redisStore) {
		rs.timeout = timeout
	}
}

// WithPrefix sets a key prefix for nonce storage
func WithPrefix(prefix string) RedisStoreOption {
	return func(rs *redisStore) {
		rs.prefix = prefix
	}
}

// NewRedisNonceStore creates a new Redis-backed nonce store
func NewRedisNonceStore(client *redis.Client, ttl time.Duration, opts ...RedisStoreOption) NonceStore {
	store := &redisStore{
		client:  client,
		ttl:     ttl,
		timeout: 5 * time.Second, // Default timeout
		prefix:  "nonce:",        // Default prefix
	}

	for _, opt := range opts {
		opt(store)
	}

	return store
}

// AddIfNotExists atomically adds a nonce if it doesn't already exist
// Returns true if the nonce was successfully added (didn't exist before)
// Returns false if the nonce already exists or on any error (fail-safe)
func (r *redisStore) AddIfNotExists(nonce string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()

	// Use Redis SetNX for atomic check-and-set operation
	// SetNX returns true only if the key was set (didn't exist before)
	key := r.prefix + nonce
	success, err := r.client.Redis.SetNX(ctx, key, "1", r.ttl).Result()
	if err != nil {
		// On any error, fail safely by rejecting the nonce
		// This prevents replay attacks even during Redis connectivity issues
		return false
	}

	return success
}

// Close closes the Redis client connection (optional cleanup)
func (r *redisStore) Close() error {
	return r.client.Close()
}
