package httpserver

import (
	"github.com/oddbit-project/blueprint/log"
	"github.com/oddbit-project/blueprint/provider/httpserver/auth"
	"github.com/oddbit-project/blueprint/provider/httpserver/security"
	"github.com/oddbit-project/blueprint/provider/httpserver/session"
	"github.com/oddbit-project/blueprint/provider/kv"
	"golang.org/x/time/rate"
)

// UseAuth registers an auth middleware
func (s *Server) UseAuth(provider auth.Provider) {
	s.AddMiddleware(auth.AuthMiddleware(provider))
}

// UseSecurityHeaders adds default security headers to a server
func (s *Server) UseSecurityHeaders(config *security.SecurityConfig) {
	s.AddMiddleware(security.SecurityMiddleware(config))
}

// UseDefaultSecurityHeaders adds default security headers to a server
func (s *Server) UseDefaultSecurityHeaders() {
	s.AddMiddleware(security.SecurityMiddleware(security.DefaultSecurityConfig()))
}

// UseCSRFProtection adds CSRF protection to the server
func (s *Server) UseCSRFProtection() {
	s.AddMiddleware(security.CSRFProtection())
}

// UseRateLimiting adds rate limiting middleware to the server
// ratePerMinute specifies the allowed requests per minute
func (s *Server) UseRateLimiting(ratePerMinute int) {
	// Convert rate per minute to rate per second
	r := rate.Limit(float64(ratePerMinute) / 60.0)

	// Allow bursts of up to 5 requests
	b := 5

	s.AddMiddleware(security.RateLimitMiddleware(r, b))
}

// UseSessionWithMemoryStore adds session middleware with provided storage
func (s *Server) UseSession(config *session.Config, backend kv.KV, logger *log.Logger) *session.SessionManager {
	if config == nil {
		config = session.NewConfig()
	}
	// Create store
	store := session.NewStore(config, backend, logger)

	// Create manager
	manager := session.NewManager(store, config, logger)

	// Add middleware
	s.AddMiddleware(manager.Middleware())

	return manager
}
