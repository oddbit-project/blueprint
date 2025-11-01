package session

import (
	"github.com/oddbit-project/blueprint/crypt/secure"
	"github.com/oddbit-project/blueprint/utils"
	"net/http"
	"slices"
)

const (
	// DefaultSessionCookieName is the default cookie name for storing sessions
	DefaultSessionCookieName = "blueprint_session"

	// DefaultSessionExpiration is the default expiration time for sessions (30 minutes)
	DefaultSessionExpiration = 1800

	// DefaultSessionIdleTimeout is the default idle timeout for sessions (15 minutes)
	DefaultSessionIdleTimeout = 900

	// DefaultSecure sets the Secure flag on session cookies
	DefaultSecure = true

	// DefaultHttpOnly sets the HttpOnly flag on session cookies
	DefaultHttpOnly = true

	// DefaultSameSite sets the SameSite policy for session cookies
	DefaultSameSite = int(http.SameSiteStrictMode)

	// DefaultCleanupInterval sets how often the session cleanup runs
	DefaultCleanupInterval = 300 // 5 min

	// ContextSessionKey is the key used to store the session in the gin.Context
	ContextSessionKey = "session"

	ErrInvalidExpirationSeconds      = utils.Error("session expiration seconds must be a positive integer")
	ErrInvalidIdleTimeoutSeconds     = utils.Error("session idle timeout seconds must be a positive integer")
	ErrInvalidSameSite               = utils.Error("invalid sameSite value")
	ErrInvalidCleanupIntervalSeconds = utils.Error("session cleanup interval seconds must be a positive integer")
	ErrSessionNotFound               = utils.Error("session not found")
	ErrSessionExpired                = utils.Error("session expired")
)

// Config holds configuration for the session store
type Config struct {
	CookieName             string                         `json:"cookieName"`             // CookieName is the name of the cookie used to store the session ID
	ExpirationSeconds      int                            `json:"expirationSeconds"`      // expiration is the maximum lifetime of a session
	IdleTimeoutSeconds     int                            `json:"idleTimeoutSeconds"`     // IdleTimeoutSeconds is the maximum time a session can be inactive
	Secure                 bool                           `json:"secure"`                 // Secure sets the Secure flag on cookies (should be true in production)
	HttpOnly               bool                           `json:"httpOnly"`               // HttpOnly sets the HttpOnly flag on cookies (should be true)
	SameSite               int                            `json:"sameSite"`               // SameSite sets the SameSite policy for cookies
	Domain                 string                         `json:"domain"`                 // Domain sets the domain for the cookie
	Path                   string                         `json:"path"`                   // Path sets the path for the cookie
	EncryptionKey          secure.DefaultCredentialConfig `json:"encryptionKey"`          // Optional encryption key to encrypt cookie data; if defined, cookie data is encrypted
	CleanupIntervalSeconds int                            `json:"cleanupIntervalSeconds"` // CleanupIntervalSeconds sets how often the session cleanup runs
}

func (c *Config) Validate() error {
	if c.ExpirationSeconds <= 0 {
		return ErrInvalidExpirationSeconds
	}
	if c.IdleTimeoutSeconds <= 0 {
		return ErrInvalidIdleTimeoutSeconds
	}
	if c.CleanupIntervalSeconds <= 0 {
		return ErrInvalidCleanupIntervalSeconds
	}
	if slices.Index([]int{
		int(http.SameSiteDefaultMode),
		int(http.SameSiteLaxMode),
		int(http.SameSiteStrictMode),
		int(http.SameSiteNoneMode)}, c.SameSite) < 0 {
		return ErrInvalidSameSite

	}

	return nil
}

// NewConfig returns a default session configuration
func NewConfig() *Config {
	return &Config{
		CookieName:             DefaultSessionCookieName,
		ExpirationSeconds:      DefaultSessionExpiration,
		IdleTimeoutSeconds:     DefaultSessionIdleTimeout,
		Secure:                 DefaultSecure,
		HttpOnly:               DefaultHttpOnly,
		SameSite:               DefaultSameSite,
		Path:                   "/",
		Domain:                 "",
		CleanupIntervalSeconds: DefaultCleanupInterval,
	}
}
