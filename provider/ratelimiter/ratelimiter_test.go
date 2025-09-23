package ratelimiter

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"
)

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		expectedErr error
	}{
		{
			name:        "valid config",
			config:      &Config{RateLimit: 10, Burst: 5, TTL: 60, CleanupInterval: 30},
			expectedErr: nil,
		},
		{
			name:        "zero rate limit",
			config:      &Config{RateLimit: 0, Burst: 5, TTL: 60, CleanupInterval: 30},
			expectedErr: ErrInvalidRateLimit,
		},
		{
			name:        "negative rate limit",
			config:      &Config{RateLimit: -1, Burst: 5, TTL: 60, CleanupInterval: 30},
			expectedErr: ErrInvalidRateLimit,
		},
		{
			name:        "zero burst",
			config:      &Config{RateLimit: 10, Burst: 0, TTL: 60, CleanupInterval: 30},
			expectedErr: ErrInvalidBurst,
		},
		{
			name:        "negative burst",
			config:      &Config{RateLimit: 10, Burst: -1, TTL: 60, CleanupInterval: 30},
			expectedErr: ErrInvalidBurst,
		},
		{
			name:        "zero TTL",
			config:      &Config{RateLimit: 10, Burst: 5, TTL: 0, CleanupInterval: 30},
			expectedErr: ErrInvalidTTL,
		},
		{
			name:        "negative TTL",
			config:      &Config{RateLimit: 10, Burst: 5, TTL: -1, CleanupInterval: 30},
			expectedErr: ErrInvalidTTL,
		},
		{
			name:        "zero cleanup interval",
			config:      &Config{RateLimit: 10, Burst: 5, TTL: 60, CleanupInterval: 0},
			expectedErr: ErrInvalidCleanupInterval,
		},
		{
			name:        "negative cleanup interval",
			config:      &Config{RateLimit: 10, Burst: 5, TTL: 60, CleanupInterval: -1},
			expectedErr: ErrInvalidCleanupInterval,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			assert.Equal(t, tt.expectedErr, err)
		})
	}
}

func TestNewConfig(t *testing.T) {
	cfg := NewConfig()
	require.NotNil(t, cfg)
	assert.Equal(t, rate.Limit(60), cfg.RateLimit)
	assert.Equal(t, 4, cfg.Burst)
	assert.Equal(t, 60, cfg.TTL)
	assert.Equal(t, 60, cfg.CleanupInterval)
	assert.NoError(t, cfg.Validate())
}

func TestNewRateLimiter(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		cfg := &Config{RateLimit: 10, Burst: 5, TTL: 60, CleanupInterval: 30}
		rl, err := NewRateLimiter(cfg)
		require.NoError(t, err)
		require.NotNil(t, rl)
		assert.Equal(t, cfg.RateLimit, rl.rate)
		assert.Equal(t, cfg.Burst, rl.burst)
		assert.Equal(t, time.Duration(cfg.TTL)*time.Second, rl.ttl)
		assert.Equal(t, time.Duration(cfg.CleanupInterval)*time.Second, rl.cleanupFreq)
	})

	t.Run("invalid config", func(t *testing.T) {
		cfg := &Config{RateLimit: 0, Burst: 5, TTL: 60, CleanupInterval: 30}
		rl, err := NewRateLimiter(cfg)
		assert.Error(t, err)
		assert.Nil(t, rl)
		assert.Equal(t, ErrInvalidRateLimit, err)
	})
}

func TestRateLimiter_GetLimiter(t *testing.T) {
	cfg := &Config{RateLimit: 2, Burst: 1, TTL: 60, CleanupInterval: 30}
	rl, err := NewRateLimiter(cfg)
	require.NoError(t, err)

	t.Run("creates new limiter for new key", func(t *testing.T) {
		limiter1 := rl.GetLimiter("key1")
		require.NotNil(t, limiter1)

		limiter2 := rl.GetLimiter("key2")
		require.NotNil(t, limiter2)

		// Compare pointers since rate.Limiter structs can't be directly compared
		assert.NotSame(t, limiter1, limiter2)
	})

	t.Run("returns same limiter for same key", func(t *testing.T) {
		limiter1 := rl.GetLimiter("same-key")
		limiter2 := rl.GetLimiter("same-key")
		assert.Same(t, limiter1, limiter2)
	})

	t.Run("updates lastSeen on access", func(t *testing.T) {
		key := "test-key"
		rl.GetLimiter(key)

		rl.mu.Lock()
		firstTime := rl.limiters[key].lastSeen
		rl.mu.Unlock()

		time.Sleep(10 * time.Millisecond)
		rl.GetLimiter(key)

		rl.mu.Lock()
		secondTime := rl.limiters[key].lastSeen
		rl.mu.Unlock()

		assert.True(t, secondTime.After(firstTime))
	})
}

func TestRateLimiter_Allow(t *testing.T) {
	cfg := &Config{RateLimit: 2, Burst: 1, TTL: 60, CleanupInterval: 30}
	rl, err := NewRateLimiter(cfg)
	require.NoError(t, err)

	t.Run("allows within burst", func(t *testing.T) {
		key := "test-key"
		assert.True(t, rl.Allow(key))
	})

	t.Run("blocks after burst exceeded", func(t *testing.T) {
		key := "burst-test"
		// Use up the burst
		assert.True(t, rl.Allow(key))
		// This should be blocked since burst=1
		assert.False(t, rl.Allow(key))
	})

	t.Run("separate keys have separate limits", func(t *testing.T) {
		key1 := "key1"
		key2 := "key2"

		// Use up key1's burst
		assert.True(t, rl.Allow(key1))
		assert.False(t, rl.Allow(key1))

		// key2 should still be allowed
		assert.True(t, rl.Allow(key2))
	})
}

func TestRateLimiter_Cleanup(t *testing.T) {
	cfg := &Config{RateLimit: 10, Burst: 5, TTL: 1, CleanupInterval: 1}
	rl, err := NewRateLimiter(cfg)
	require.NoError(t, err)

	t.Run("removes expired entries", func(t *testing.T) {
		key1 := "key1"
		key2 := "key2"

		// Create some limiters
		rl.GetLimiter(key1)
		rl.GetLimiter(key2)

		// Verify they exist
		rl.mu.Lock()
		assert.Len(t, rl.limiters, 2)
		rl.mu.Unlock()

		// Wait for TTL to expire
		time.Sleep(1100 * time.Millisecond)

		// Manual cleanup (since we're not running the background loop)
		rl.cleanup()

		// Verify expired entries are removed
		rl.mu.Lock()
		assert.Len(t, rl.limiters, 0)
		rl.mu.Unlock()
	})

	t.Run("keeps fresh entries", func(t *testing.T) {
		key := "fresh-key"

		// Create limiter
		rl.GetLimiter(key)

		// Access it again to update lastSeen
		time.Sleep(500 * time.Millisecond)
		rl.GetLimiter(key)

		// Wait less than TTL
		time.Sleep(500 * time.Millisecond)

		rl.cleanup()

		// Should still exist
		rl.mu.Lock()
		assert.Len(t, rl.limiters, 1)
		rl.mu.Unlock()
	})
}

func TestRateLimiter_StartShutdown(t *testing.T) {
	cfg := &Config{RateLimit: 10, Burst: 5, TTL: 1, CleanupInterval: 1}
	rl, err := NewRateLimiter(cfg)
	require.NoError(t, err)

	t.Run("multiple starts are safe", func(t *testing.T) {
		// Should not panic or create multiple goroutines
		rl.Start()
		rl.Start()
		rl.Start()

		// Give some time for potential issues to surface
		time.Sleep(100 * time.Millisecond)

		rl.Shutdown()
	})

	t.Run("multiple shutdowns are safe", func(t *testing.T) {
		rl2, err := NewRateLimiter(cfg)
		require.NoError(t, err)

		rl2.Start()

		// Should not panic
		rl2.Shutdown()
		rl2.Shutdown()
		rl2.Shutdown()
	})

	t.Run("cleanup goroutine stops after shutdown", func(t *testing.T) {
		rl3, err := NewRateLimiter(cfg)
		require.NoError(t, err)

		rl3.Start()

		// Add some limiters to verify cleanup works
		rl3.GetLimiter("key1")
		rl3.GetLimiter("key2")

		// Shutdown should stop the cleanup goroutine
		rl3.Shutdown()

		// Wait for done signal
		select {
		case <-rl3.done:
			// Success - cleanup goroutine exited
		case <-time.After(2 * time.Second):
			t.Fatal("cleanup goroutine did not exit within timeout")
		}
	})
}

func TestRateLimiter_ShutdownWithContext(t *testing.T) {
	cfg := &Config{RateLimit: 10, Burst: 5, TTL: 1, CleanupInterval: 1}
	rl, err := NewRateLimiter(cfg)
	require.NoError(t, err)

	t.Run("successful shutdown within timeout", func(t *testing.T) {
		rl.Start()

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		err := rl.ShutdownWithContext(ctx)
		assert.NoError(t, err)
	})

	t.Run("timeout before shutdown complete", func(t *testing.T) {
		rl2, err := NewRateLimiter(cfg)
		require.NoError(t, err)

		rl2.Start()

		// Very short timeout
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
		defer cancel()

		time.Sleep(10 * time.Millisecond) // Ensure context times out

		err = rl2.ShutdownWithContext(ctx)
		assert.Equal(t, context.DeadlineExceeded, err)

		// Clean shutdown for cleanup
		rl2.Shutdown()
	})

	t.Run("context already canceled", func(t *testing.T) {
		rl3, err := NewRateLimiter(cfg)
		require.NoError(t, err)

		rl3.Start()

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		err = rl3.ShutdownWithContext(ctx)
		assert.Equal(t, context.Canceled, err)

		// Clean shutdown for cleanup
		rl3.Shutdown()
	})
}

func TestRateLimiter_ConcurrentAccess(t *testing.T) {
	cfg := &Config{RateLimit: 100, Burst: 10, TTL: 60, CleanupInterval: 1}
	rl, err := NewRateLimiter(cfg)
	require.NoError(t, err)

	t.Run("concurrent access is safe", func(t *testing.T) {
		const numGoroutines = 50
		const numOperations = 100

		var wg sync.WaitGroup

		// Start concurrent operations
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				key := fmt.Sprintf("key-%d", id%10) // Use 10 different keys

				for j := 0; j < numOperations; j++ {
					rl.Allow(key)
					rl.GetLimiter(key)
				}
			}(i)
		}

		wg.Wait()

		// Verify state is consistent
		rl.mu.Lock()
		limiterCount := len(rl.limiters)
		rl.mu.Unlock()

		assert.LessOrEqual(t, limiterCount, 10) // Should have at most 10 keys
		assert.Greater(t, limiterCount, 0)      // Should have some keys
	})
}
