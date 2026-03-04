package security

import (
	"github.com/gin-gonic/gin"
	"github.com/oddbit-project/blueprint/provider/httpserver/response"
	"golang.org/x/time/rate"
	"sync"
	"time"
)

const (
	// DefaultMaxClients is the maximum number of tracked clients before forced eviction
	DefaultMaxClients = 10000
)

// clientEntry tracks a rate limiter and its last access time
type clientEntry struct {
	limiter    *rate.Limiter
	lastAccess time.Time
}

// ClientRateLimiter manages per-client rate limiters
type ClientRateLimiter struct {
	clients map[string]*clientEntry
	mu      sync.RWMutex
	// Configuration
	rate         rate.Limit    // Rate is limit per second
	burst        int           // Burst is maximum token bucket size
	cleanupTimer *time.Timer   // Timer for cleanup
	clientExpiry time.Duration // How long to keep a client limiter around
	maxClients   int           // Maximum tracked clients before forced eviction
	stopped      bool          // Whether the limiter has been stopped
}

// NewClientRateLimiter creates a new ClientRateLimiter
func NewClientRateLimiter(r rate.Limit, b int) *ClientRateLimiter {
	rl := &ClientRateLimiter{
		clients:      make(map[string]*clientEntry),
		rate:         r,
		burst:        b,
		clientExpiry: 1 * time.Hour,
		maxClients:   DefaultMaxClients,
	}

	// Start cleanup routine
	rl.cleanupTimer = time.AfterFunc(rl.clientExpiry, rl.cleanup)

	return rl
}

// GetLimiter returns a rate limiter for the specified IP address
func (rl *ClientRateLimiter) GetLimiter(ip string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if entry, exists := rl.clients[ip]; exists {
		entry.lastAccess = time.Now()
		return entry.limiter
	}

	// Enforce max clients cap
	if len(rl.clients) >= rl.maxClients {
		rl.evictExpired()
	}

	entry := &clientEntry{
		limiter:    rate.NewLimiter(rl.rate, rl.burst),
		lastAccess: time.Now(),
	}
	rl.clients[ip] = entry
	return entry.limiter
}

// evictExpired removes entries older than clientExpiry.
// Must be called with write lock held.
func (rl *ClientRateLimiter) evictExpired() {
	cutoff := time.Now().Add(-rl.clientExpiry)
	for ip, entry := range rl.clients {
		if entry.lastAccess.Before(cutoff) {
			delete(rl.clients, ip)
		}
	}
}

// cleanup periodically removes stale client entries
func (rl *ClientRateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if rl.stopped {
		return
	}

	rl.evictExpired()

	// Reschedule cleanup
	rl.cleanupTimer.Reset(rl.clientExpiry)
}

// Stop stops the cleanup timer and releases resources
func (rl *ClientRateLimiter) Stop() {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.stopped = true
	if rl.cleanupTimer != nil {
		rl.cleanupTimer.Stop()
	}
}

// RateLimitMiddleware creates a Gin middleware for rate limiting.
// Returns the middleware handler and the underlying ClientRateLimiter for lifecycle management.
// Callers should call Stop() on the returned limiter during shutdown.
func RateLimitMiddleware(r rate.Limit, b int) (gin.HandlerFunc, *ClientRateLimiter) {
	limiter := NewClientRateLimiter(r, b)

	handler := func(c *gin.Context) {
		// Use Gin's ClientIP() which respects trusted proxies configuration
		ip := c.ClientIP()

		// Get the rate limiter for this IP
		clientLimiter := limiter.GetLimiter(ip)

		// Check if rate limit exceeded
		if !clientLimiter.Allow() {
			response.Http429(c)
			return
		}

		c.Next()
	}

	return handler, limiter
}
