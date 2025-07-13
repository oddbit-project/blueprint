package store

import (
	"github.com/oddbit-project/blueprint/provider/kv"
	"time"
)

type kvStore struct {
	kv  kv.KV
	ttl time.Duration
}

func NewKvStore(kv kv.KV, ttl time.Duration) NonceStore {
	return &kvStore{
		kv:  kv,
		ttl: ttl,
	}
}

func (s *kvStore) AddIfNotExists(nonce string) bool {
	// Check if nonce already exists
	existing, err := s.kv.Get(nonce)
	if err != nil {
		// On error, fail safely - assume nonce exists to prevent replay attacks
		return false
	}
	
	// If nonce exists, reject
	if existing != nil {
		return false
	}
	
	// Nonce doesn't exist, add it with TTL
	// Use a simple marker value since we only care about existence
	err = s.kv.SetTTL(nonce, []byte("1"), s.ttl)
	if err != nil {
		// If we can't store the nonce, fail safely
		return false
	}
	
	return true
}

func (s *kvStore) Close() {
	// KV interface doesn't have Close method, so this is a no-op
}