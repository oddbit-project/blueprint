package security

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/oddbit-project/blueprint/provider/httpserver/response"
	"github.com/oddbit-project/blueprint/provider/httpserver/session"
	"strings"
)

const (
	// ContextCSPNonce is the context key for the CSP nonce value
	ContextCSPNonce = "csp-nonce"
)

// SecurityConfig contains configuration for security headers
type SecurityConfig struct {
	// Content Security Policy
	CSP string

	// XSS Protection header
	XSSProtection string

	// X-Content-Type-Options header
	ContentTypeOptions string

	// Referrer-Policy header
	ReferrerPolicy string

	// Strict-Transport-Security header
	HSTS string

	// X-Frame-Options header
	FrameOptions string

	// Feature-Policy header
	FeaturePolicy string

	// Cache-Control header
	CacheControl string

	// Generate and add CSP nonce to requests
	UseCSPNonce bool
}

// DefaultSecurityConfig returns security configuration with sane defaults
func DefaultSecurityConfig() *SecurityConfig {
	return &SecurityConfig{
		CSP:                "default-src 'self'; script-src 'self' 'nonce-{nonce}'; style-src 'self' 'nonce-{nonce}'; img-src 'self' data:; font-src 'self'; connect-src 'self';",
		XSSProtection:      "1; mode=block",
		ContentTypeOptions: "nosniff",
		ReferrerPolicy:     "strict-origin-when-cross-origin",
		HSTS:               "max-age=31536000; includeSubDomains",
		FrameOptions:       "DENY",
		FeaturePolicy:      "camera=(), microphone=(), geolocation=()",
		CacheControl:       "no-store, no-cache, must-revalidate, proxy-revalidate, max-age=0",
		UseCSPNonce:        true,
	}
}

// SecurityMiddleware adds security headers to each response
func SecurityMiddleware(config *SecurityConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Set security headers
		if config.ContentTypeOptions != "" {
			c.Header("X-Content-Type-Options", config.ContentTypeOptions)
		}

		if config.XSSProtection != "" {
			c.Header("X-XSS-Protection", config.XSSProtection)
		}

		if config.FrameOptions != "" {
			c.Header("X-Frame-Options", config.FrameOptions)
		}

		if config.ReferrerPolicy != "" {
			c.Header("Referrer-Policy", config.ReferrerPolicy)
		}

		if config.FeaturePolicy != "" {
			c.Header("Permissions-Policy", config.FeaturePolicy)
		}

		if config.CacheControl != "" {
			c.Header("Cache-Control", config.CacheControl)
			c.Header("Pragma", "no-cache")
			c.Header("Expires", "0")
		}

		// Only add HSTS header if using HTTPS
		if c.Request.TLS != nil && config.HSTS != "" {
			c.Header("Strict-Transport-Security", config.HSTS)
		}

		// Generate CSP nonce if enabled
		if config.UseCSPNonce && config.CSP != "" {
			nonce := generateCSPNonce()
			c.Set(ContextCSPNonce, nonce)
			csp := strings.Replace(config.CSP, "{nonce}", nonce, -1)
			c.Header("Content-Security-Policy", csp)
		} else if config.CSP != "" {
			c.Header("Content-Security-Policy", config.CSP)
		}

		c.Next()
	}
}

// CSRFProtection implements CSRF protection middleware
func CSRFProtection() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip CSRF check for GET, HEAD, OPTIONS
		if c.Request.Method == "GET" ||
			c.Request.Method == "HEAD" ||
			c.Request.Method == "OPTIONS" {
			// Seed CSRF token if not present
			sess := session.Get(c)
			if sess != nil {
				if existing, _ := sess.GetString("_csrf"); existing == "" {
					token := GenerateCSRFToken(c)
					sess.Set("_csrf", token)
					c.Header("X-CSRF-Token", token)
				}
			}
			c.Next()
			return
		}

		// get session
		sess := session.Get(c)
		if sess == nil {
			response.Http403(c)
			return
		}
		expected, _ := sess.GetString("_csrf")
		// Check CSRF token in header or form
		token := c.GetHeader("X-CSRF-Token")
		if token == "" {
			token = c.PostForm("_csrf")
		}
		// Constant-time comparison to prevent timing attacks
		if token == "" || expected == "" || subtle.ConstantTimeCompare([]byte(token), []byte(expected)) != 1 {
			response.Http403(c)
			return
		}

		c.Next()

		// refresh token
		newToken := GenerateCSRFToken(c)
		c.Header("X-CSRF-Token", newToken)
		sess.Set("_csrf", newToken)
	}
}

// GenerateCSRFToken generates a CSRF token for the current session
func GenerateCSRFToken(c *gin.Context) string {
	return uuid.New().String()
}

// generateCSPNonce generates a cryptographically random base64-encoded nonce
// for Content Security Policy headers, per W3C CSP specification.
func generateCSPNonce() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		// Fallback to UUID if crypto/rand fails (should never happen)
		return uuid.New().String()
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
