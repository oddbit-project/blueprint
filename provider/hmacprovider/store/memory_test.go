package store

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMemoryNonceStore(t *testing.T) {
	store := NewMemoryNonceStore()
	
	assert.NotNil(t, store)
	
	// Test with custom options
	customStore := NewMemoryNonceStore(
		WithTTL(1*time.Hour),
		WithMaxSize(1000),
		WithCleanupInterval(5*time.Minute),
	)
	
	assert.NotNil(t, customStore)
}

func TestMemoryStoreAddIfNotExists(t *testing.T) {
	store := NewMemoryNonceStore()
	
	nonce := "test-nonce-123"
	
	// First add should succeed
	result := store.AddIfNotExists(nonce)
	assert.True(t, result)
	
	// Second add should fail (nonce already exists)
	result = store.AddIfNotExists(nonce)
	assert.False(t, result)
}

func TestMemoryStoreAddIfNotExistsDifferentNonces(t *testing.T) {
	store := NewMemoryNonceStore()
	
	nonce1 := "test-nonce-1"
	nonce2 := "test-nonce-2"
	
	// Both should succeed since they're different
	result1 := store.AddIfNotExists(nonce1)
	result2 := store.AddIfNotExists(nonce2)
	
	assert.True(t, result1)
	assert.True(t, result2)
	
	// Repeating should fail
	result1 = store.AddIfNotExists(nonce1)
	result2 = store.AddIfNotExists(nonce2)
	
	assert.False(t, result1)
	assert.False(t, result2)
}

func TestMemoryStoreTTLExpiration(t *testing.T) {
	// Use very short TTL for testing
	shortTTL := 50 * time.Millisecond
	store := NewMemoryNonceStore(WithTTL(shortTTL))
	
	nonce := "test-nonce-ttl"
	
	// Add nonce
	result := store.AddIfNotExists(nonce)
	assert.True(t, result)
	
	// Should fail immediately (not expired)
	result = store.AddIfNotExists(nonce)
	assert.False(t, result)
	
	// Wait for expiration
	time.Sleep(shortTTL + 10*time.Millisecond)
	
	// Should succeed after expiration
	result = store.AddIfNotExists(nonce)
	assert.True(t, result)
}

func TestMemoryStoreMaxSize(t *testing.T) {
	maxSize := 3
	store := NewMemoryNonceStore(
		WithMaxSize(maxSize),
		WithTTL(1*time.Hour), // Long TTL so items don't expire
	)
	
	// Add up to max size
	for i := 0; i < maxSize; i++ {
		nonce := fmt.Sprintf("nonce-%d", i)
		result := store.AddIfNotExists(nonce)
		assert.True(t, result, "Should be able to add nonce %d", i)
	}
	
	// Adding one more should fail (at capacity)
	result := store.AddIfNotExists("nonce-overflow")
	assert.False(t, result, "Should fail when at max capacity")
}

func TestMemoryStoreEvictionPolicies(t *testing.T) {
	t.Run("EvictNone", func(t *testing.T) {
		policy := EvictNone()
		assert.NotNil(t, policy)
		
		// Test that it doesn't panic when called
		store := &memStore{
			nonces: make(map[string]time.Time),
			ttl:    1 * time.Hour,
		}
		policy(store)
		// EvictNone should do nothing
	})
	
	t.Run("EvictAll", func(t *testing.T) {
		policy := EvictAll()
		assert.NotNil(t, policy)
		
		store := &memStore{
			nonces: map[string]time.Time{
				"nonce1": time.Now().Add(1 * time.Hour),
				"nonce2": time.Now().Add(1 * time.Hour),
			},
			ttl: 1 * time.Hour,
		}
		
		// Should have items before eviction
		assert.Len(t, store.nonces, 2)
		
		policy(store)
		
		// Should be empty after eviction
		assert.Len(t, store.nonces, 0)
	})
	
	t.Run("EvictHalfLife", func(t *testing.T) {
		policy := EvictHalfLife()
		assert.NotNil(t, policy)
		
		ttl := 1 * time.Hour
		now := time.Now()
		
		store := &memStore{
			nonces: map[string]time.Time{
				"old-nonce":    now.Add(-ttl),           // Should be evicted (reached half-life)
				"recent-nonce": now.Add(-ttl/4),         // Should remain (not at half-life)
			},
			ttl: ttl,
		}
		
		// Should have 2 items before eviction
		assert.Len(t, store.nonces, 2)
		
		policy(store)
		
		// Should have 1 item after eviction (recent one remains)
		assert.Len(t, store.nonces, 1)
		assert.Contains(t, store.nonces, "recent-nonce")
		assert.NotContains(t, store.nonces, "old-nonce")
	})
}

func TestMemoryStoreConcurrency(t *testing.T) {
	store := NewMemoryNonceStore(WithMaxSize(1000))
	
	const numGoroutines = 100
	const noncesPerGoroutine = 10
	
	var wg sync.WaitGroup
	results := make(chan bool, numGoroutines*noncesPerGoroutine)
	
	// Launch multiple goroutines adding nonces concurrently
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < noncesPerGoroutine; j++ {
				nonce := fmt.Sprintf("nonce-%d-%d", goroutineID, j)
				result := store.AddIfNotExists(nonce)
				results <- result
			}
		}(i)
	}
	
	wg.Wait()
	close(results)
	
	// All additions should succeed (different nonces)
	successCount := 0
	for result := range results {
		if result {
			successCount++
		}
	}
	
	assert.Equal(t, numGoroutines*noncesPerGoroutine, successCount)
}

func TestMemoryStoreConcurrentSameNonce(t *testing.T) {
	store := NewMemoryNonceStore()
	
	const numGoroutines = 50
	const sameNonce = "concurrent-test-nonce"
	
	var wg sync.WaitGroup
	results := make(chan bool, numGoroutines)
	
	// Launch multiple goroutines trying to add the same nonce
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result := store.AddIfNotExists(sameNonce)
			results <- result
		}()
	}
	
	wg.Wait()
	close(results)
	
	// Only one should succeed
	successCount := 0
	for result := range results {
		if result {
			successCount++
		}
	}
	
	assert.Equal(t, 1, successCount, "Only one goroutine should successfully add the nonce")
}

func TestMemoryStoreCleanupExpired(t *testing.T) {
	// Short cleanup interval for testing
	store := NewMemoryNonceStore(
		WithTTL(50*time.Millisecond),
		WithCleanupInterval(25*time.Millisecond),
	)
	
	nonce := "test-cleanup-nonce"
	
	// Add nonce
	result := store.AddIfNotExists(nonce)
	assert.True(t, result)
	
	// Should fail immediately
	result = store.AddIfNotExists(nonce)
	assert.False(t, result)
	
	// Wait for TTL + cleanup
	time.Sleep(100 * time.Millisecond)
	
	// Should succeed after cleanup removes expired nonce
	result = store.AddIfNotExists(nonce)
	assert.True(t, result)
}

func TestMemoryStoreClose(t *testing.T) {
	store := NewMemoryNonceStore()
	
	// Cast to access Close method
	memStore, ok := store.(*memStore)
	require.True(t, ok)
	
	// Should not panic
	memStore.Close()
	
	// Should be able to call multiple times (though channel will panic on second close)
	// This is expected behavior - close should only be called once
}

// Benchmark tests
func BenchmarkMemoryStoreAddIfNotExists(b *testing.B) {
	store := NewMemoryNonceStore()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		nonce := fmt.Sprintf("benchmark-nonce-%d", i)
		store.AddIfNotExists(nonce)
	}
}

func BenchmarkMemoryStoreAddIfNotExistsConcurrent(b *testing.B) {
	store := NewMemoryNonceStore()
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			nonce := fmt.Sprintf("benchmark-nonce-%d", i)
			store.AddIfNotExists(nonce)
			i++
		}
	})
}

func BenchmarkMemoryStoreAddIfNotExistsSameNonce(b *testing.B) {
	store := NewMemoryNonceStore()
	nonce := "same-nonce"
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.AddIfNotExists(nonce)
	}
}