package store

import (
	"sync"
	"time"
)

type memStore struct {
	sync.Mutex
	nonces          map[string]time.Time
	maxSize         int
	ttl             time.Duration
	cleanupInterval time.Duration
	done            chan struct{}
	evictPolicy     MemEvictPolicyFn
}

type MemEvictPolicyFn func(m *memStore)

type MemStoreOption func(*memStore)

func WithTTL(ttl time.Duration) MemStoreOption {
	return func(store *memStore) {
		store.ttl = ttl
	}
}

func WithMaxSize(maxSize int) MemStoreOption {
	return func(store *memStore) {
		store.maxSize = maxSize
	}
}

func WithCleanupInterval(interval time.Duration) MemStoreOption {
	return func(store *memStore) {
		store.cleanupInterval = interval
	}
}

func WithEvictPolicy(evictPolicy MemEvictPolicyFn) MemStoreOption {
	return func(store *memStore) {
		store.evictPolicy = evictPolicy
	}
}

func EvictNone() MemEvictPolicyFn {
	return func(store *memStore) {
		return
	}
}

// EvictHalfLife evict all noces older than half of the ttl
func EvictHalfLife() MemEvictPolicyFn {
	return func(store *memStore) {
		// expire all entries that reached middle of the TTL
		now := time.Now().Add(-(store.ttl / 2))
		for nonce, expiry := range store.nonces {
			if now.After(expiry) {
				delete(store.nonces, nonce)
			}
		}
	}
}

func EvictAll() MemEvictPolicyFn {
	return func(store *memStore) {
		store.nonces = make(map[string]time.Time)
	}
}

func NewMemoryNonceStore(opts ...MemStoreOption) NonceStore {
	store := &memStore{
		nonces:          make(map[string]time.Time),
		ttl:             DefaultTTL,
		cleanupInterval: DefaultCleanupInterval,
		maxSize:         DefaultMaxSize,
		done:            make(chan struct{}),
		evictPolicy:     EvictNone(), // default eviction policy
	}
	for _, opt := range opts {
		opt(store)
	}

	go store.cleanupLoop()
	return store
}

func (ns *memStore) AddIfNotExists(nonce string) bool {
	ns.Lock()
	defer ns.Unlock()

	// Check existence
	if expiry, exists := ns.nonces[nonce]; exists {
		if time.Now().After(expiry) {
			// Expired - remove and allow reuse
			delete(ns.nonces, nonce)
		} else {
			// Still valid - reject
			return false
		}
	}

	// Check if at capacity
	if len(ns.nonces) >= ns.maxSize {
		// Force cleanup of expired entries
		ns.cleanupExpiredLocked(time.Now())

		// Still at capacity? use Eviction policy
		// typically this happens either if the API is an heavy-traffic API *OR* if the endpoints are
		// under attack; two million entries with a TTL of 4h are roughly 140 requests/s
		// for high-traffic apis use other backends for nonce
		if len(ns.nonces) >= ns.maxSize {
			ns.evictPolicy(ns)
		}
	}

	// Add new nonce atomically
	ns.nonces[nonce] = time.Now().Add(ns.ttl)
	return true
}

func (ns *memStore) Close() error {
	close(ns.done)
	return nil
}

func (ns *memStore) cleanupExpiredLocked(now time.Time) {
	for nonce, expiry := range ns.nonces {
		if now.After(expiry) {
			delete(ns.nonces, nonce)
		}
	}
}

func (ns *memStore) cleanupLoop() {
	ticker := time.NewTicker(ns.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ns.Lock()
			ns.cleanupExpiredLocked(time.Now())
			ns.Unlock()
		case <-ns.done:
			return
		}
	}
}
