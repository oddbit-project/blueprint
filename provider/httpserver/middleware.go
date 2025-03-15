package httpserver

import (
	"github.com/oddbit-project/blueprint/provider/httpserver/auth"
	"github.com/oddbit-project/blueprint/provider/httpserver/security"
	"github.com/oddbit-project/blueprint/provider/httpserver/session"
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

// UseSessionWithMemoryStore adds session middleware with in-memory storage
func (s *Server) UseSessionWithMemoryStore(config *session.SessionConfig) *session.Manager {
	if config == nil {
		config = session.DefaultSessionConfig()
	}
	
	// Set logger if not provided
	if config.Logger == nil && s.Logger != nil {
		config.Logger = s.Logger
	}
	
	// Create store
	store := session.NewMemoryStore(config)
	
	// Create manager
	manager := session.NewManager(store, config)
	
	// Add middleware
	s.AddMiddleware(manager.Middleware())
	
	return manager
}

// UseSessionWithRedisStore adds session middleware with Redis storage
func (s *Server) UseSessionWithRedisStore(sessionConfig *session.SessionConfig, redisConfig *session.RedisConfig) (*session.Manager, error) {
	if sessionConfig == nil {
		sessionConfig = session.DefaultSessionConfig()
	}
	
	if redisConfig == nil {
		redisConfig = session.DefaultRedisConfig()
	}
	
	// Set logger if not provided
	if sessionConfig.Logger == nil && s.Logger != nil {
		sessionConfig.Logger = s.Logger
	}
	
	if redisConfig.Logger == nil && s.Logger != nil {
		redisConfig.Logger = s.Logger
	}
	
	// Create store
	store, err := session.NewRedisStore(sessionConfig, redisConfig)
	if err != nil {
		return nil, err
	}
	
	// Create manager
	manager := session.NewManager(store, sessionConfig)
	
	// Add middleware
	s.AddMiddleware(manager.Middleware())
	
	return manager, nil
}

// UseSessionWithJWT adds session middleware with JWT token support
func (s *Server) UseSessionWithJWT(sessionConfig *session.SessionConfig, jwtConfig *session.JWTConfig) (*session.JWTSessionManager, error) {
	if sessionConfig == nil {
		sessionConfig = session.DefaultSessionConfig()
	}
	
	if jwtConfig == nil {
		jwtConfig = session.DefaultJWTConfig()
	}
	
	// Set logger if not provided
	if sessionConfig.Logger == nil && s.Logger != nil {
		sessionConfig.Logger = s.Logger
	}
	
	if jwtConfig.Logger == nil && s.Logger != nil {
		jwtConfig.Logger = s.Logger
	}
	
	// Create JWT manager
	jwtManager, err := session.NewJWTManager(jwtConfig)
	if err != nil {
		return nil, err
	}
	
	// Create session manager
	manager := session.NewJWTSessionManager(jwtManager, sessionConfig)
	
	// Add middleware
	s.AddMiddleware(manager.Middleware())
	
	return manager, nil
}
