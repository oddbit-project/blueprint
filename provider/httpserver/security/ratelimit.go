package security

import (
	"github.com/gin-gonic/gin"
	"github.com/oddbit-project/blueprint/provider/httpserver/response"
	"golang.org/x/time/rate"
	"net"
	"strings"
	"sync"
	"time"
)

// ClientRateLimiter manages per-client rate limiters
type ClientRateLimiter struct {
	limiters map[string]*rate.Limiter
	mu       sync.RWMutex
	// Configuration
	rate         rate.Limit    // Rate is limit per second
	burst        int           // Burst is maximum token bucket size
	cleanupTimer *time.Timer   // Timer for cleanup
	clientExpiry time.Duration // How long to keep a client limiter around
}

// NewClientRateLimiter creates a new ClientRateLimiter
func NewClientRateLimiter(r rate.Limit, b int) *ClientRateLimiter {
	rl := &ClientRateLimiter{
		limiters:     make(map[string]*rate.Limiter),
		rate:         r,
		burst:        b,
		clientExpiry: 1 * time.Hour,
	}

	// Start cleanup routine
	rl.cleanupTimer = time.AfterFunc(rl.clientExpiry, rl.cleanup)

	return rl
}

// GetLimiter returns a rate limiter for the specified IP address
func (rl *ClientRateLimiter) GetLimiter(ip string) *rate.Limiter {
	rl.mu.RLock()
	limiter, exists := rl.limiters[ip]
	rl.mu.RUnlock()

	if !exists {
		rl.mu.Lock()
		// Double check after obtaining write lock
		limiter, exists = rl.limiters[ip]
		if !exists {
			limiter = rate.NewLimiter(rl.rate, rl.burst)
			rl.limiters[ip] = limiter
		}
		rl.mu.Unlock()
	}

	return limiter
}

// cleanup removes old limiters
func (rl *ClientRateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// In a more sophisticated implementation, we would track last access time
	// for each limiter and remove those that haven't been used recently
	// For now, we just reset the map periodically
	rl.limiters = make(map[string]*rate.Limiter)

	// Reschedule cleanup
	rl.cleanupTimer.Reset(rl.clientExpiry)
}

// RateLimitMiddleware creates a Gin middleware for rate limiting
func RateLimitMiddleware(r rate.Limit, b int) gin.HandlerFunc {
	limiter := NewClientRateLimiter(r, b)

	return func(c *gin.Context) {
		// Get client IP
		ip, _, err := net.SplitHostPort(c.Request.RemoteAddr)
		if err != nil {
			ip = c.Request.RemoteAddr
		}

		// Use X-Forwarded-For if behind proxy
		if c.GetHeader("X-Forwarded-For") != "" {
			ips := c.GetHeader("X-Forwarded-For")
			ipList := strings.Split(ips, ",")
			ip = strings.TrimSpace(ipList[0])
		}

		// Get the rate limiter for this IP
		clientLimiter := limiter.GetLimiter(ip)

		// Check if rate limit exceeded
		if !clientLimiter.Allow() {
			response.Http429(c)
			return
		}

		c.Next()
	}
}
