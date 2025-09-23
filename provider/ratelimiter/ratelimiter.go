package ratelimiter

import (
	"context"
	"github.com/oddbit-project/blueprint/utils"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

const (
	ErrInvalidRateLimit       = utils.Error("rate limit must be positive")
	ErrInvalidBurst           = utils.Error("burst must be positive")
	ErrInvalidTTL             = utils.Error("TTL must be positive")
	ErrInvalidCleanupInterval = utils.Error("cleanup interval must be positive")
)

type limiterEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type Config struct {
	RateLimit       rate.Limit `json:"rateLimit"`
	Burst           int        `json:"burst"`
	TTL             int        `json:"ttl"`             // seconds
	CleanupInterval int        `json:"cleanupInterval"` // seconds
}

// RateLimiter manages rate limiters with expiration
type RateLimiter struct {
	mu          sync.Mutex
	limiters    map[string]*limiterEntry
	rate        rate.Limit
	burst       int
	ttl         time.Duration
	cleanupFreq time.Duration
	stopCleanup chan struct{}
	done        chan struct{}
	startOnce   sync.Once
	stopOnce    sync.Once
}

func NewConfig() *Config {
	return &Config{
		RateLimit:       60, // 60 events per second
		Burst:           4,  // burst
		TTL:             60, // expires after 60s
		CleanupInterval: 60, // clean every 60s
	}

}

// Validate checks if config values are valid
func (c *Config) Validate() error {
	if c.RateLimit <= 0 {
		return ErrInvalidRateLimit
	}
	if c.Burst <= 0 {
		return ErrInvalidBurst
	}
	if c.TTL <= 0 {
		return ErrInvalidTTL
	}
	if c.CleanupInterval <= 0 {
		return ErrInvalidCleanupInterval
	}
	return nil
}

// NewRateLimiter creates a rate limiter with TTL and cleanup
func NewRateLimiter(cfg *Config) (*RateLimiter, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &RateLimiter{
		limiters:    make(map[string]*limiterEntry),
		rate:        cfg.RateLimit,
		burst:       cfg.Burst,
		ttl:         time.Duration(cfg.TTL) * time.Second,
		cleanupFreq: time.Duration(cfg.CleanupInterval) * time.Second,
		stopCleanup: make(chan struct{}),
		done:        make(chan struct{}),
	}, nil
}

// Start begins the cleanup goroutine (safe to call multiple times)
func (r *RateLimiter) Start() {
	r.startOnce.Do(func() {
		go r.cleanupLoop()
	})
}

// GetLimiter returns or creates a rate limiter for a key
func (r *RateLimiter) GetLimiter(key string) *rate.Limiter {
	r.mu.Lock()
	defer r.mu.Unlock()

	entry, exists := r.limiters[key]
	if !exists {
		limiter := rate.NewLimiter(r.rate, r.burst)
		r.limiters[key] = &limiterEntry{
			limiter:  limiter,
			lastSeen: time.Now(),
		}
		return limiter
	}

	entry.lastSeen = time.Now()
	return entry.limiter
}

// Allow checks if the key can perform an action now
func (r *RateLimiter) Allow(key string) bool {
	return r.GetLimiter(key).Allow()
}

// cleanupLoop runs periodically to remove stale entries
func (r *RateLimiter) cleanupLoop() {
	defer close(r.done)
	ticker := time.NewTicker(r.cleanupFreq)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			r.cleanup()
		case <-r.stopCleanup:
			return
		}
	}
}

// cleanup removes limiters that haven't been seen within TTL
func (r *RateLimiter) cleanup() {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	for addr, entry := range r.limiters {
		if now.Sub(entry.lastSeen) > r.ttl {
			delete(r.limiters, addr)
		}
	}
}

// Shutdown stops the background cleanup goroutine (safe to call multiple times)
func (r *RateLimiter) Shutdown() {
	r.stopOnce.Do(func() {
		close(r.stopCleanup)
	})
}

// ShutdownWithContext stops cleanup goroutine with timeout
func (r *RateLimiter) ShutdownWithContext(ctx context.Context) error {
	r.stopOnce.Do(func() {
		close(r.stopCleanup)
	})

	select {
	case <-r.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
