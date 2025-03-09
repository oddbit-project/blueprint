package httpserver

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"strings"
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
	
	// Rate limiting enabled
	EnableRateLimit bool
	
	// Rate limit per minute
	RateLimit int
}

// DefaultSecurityConfig returns security configuration with sane defaults
func DefaultSecurityConfig() *SecurityConfig {
	return &SecurityConfig{
		CSP:               "default-src 'self'; script-src 'self' 'nonce-{nonce}'; style-src 'self' 'nonce-{nonce}'; img-src 'self' data:; font-src 'self'; connect-src 'self';",
		XSSProtection:     "1; mode=block",
		ContentTypeOptions: "nosniff",
		ReferrerPolicy:    "strict-origin-when-cross-origin",
		HSTS:              "max-age=31536000; includeSubDomains",
		FrameOptions:      "DENY",
		FeaturePolicy:     "camera 'none'; microphone 'none'; geolocation 'none'",
		CacheControl:      "no-store, no-cache, must-revalidate, proxy-revalidate, max-age=0",
		UseCSPNonce:       true,
		EnableRateLimit:   true,
		RateLimit:         60, // 60 requests per minute
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
			c.Header("Feature-Policy", config.FeaturePolicy)
			// Modern alternative
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
			nonce := uuid.New().String()
			c.Set("csp-nonce", nonce)
			csp := strings.Replace(config.CSP, "{nonce}", nonce, -1)
			c.Header("Content-Security-Policy", csp)
		} else if config.CSP != "" {
			c.Header("Content-Security-Policy", config.CSP)
		}
		
		c.Next()
	}
}

// AddSecurityHeaders adds default security headers to a server
func (s *Server) AddSecurityHeaders() {
	s.AddMiddleware(SecurityMiddleware(DefaultSecurityConfig()))
}

// AddCSRFProtection adds CSRF protection to the server
func (s *Server) AddCSRFProtection() {
	s.AddMiddleware(CSRFProtection())
}

// CSRFProtection implements CSRF protection middleware
func CSRFProtection() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip CSRF check for GET, HEAD, OPTIONS
		if c.Request.Method == "GET" || 
		   c.Request.Method == "HEAD" || 
		   c.Request.Method == "OPTIONS" {
			c.Next()
			return
		}
		
		// Check CSRF token in header or form
		token := c.GetHeader("X-CSRF-Token")
		if token == "" {
			token = c.PostForm("_csrf")
		}
		
		// Validate token (in a real implementation, this would validate against a stored token)
		expected := c.GetString("csrf-token")
		if token == "" || token != expected {
			c.AbortWithStatusJSON(403, gin.H{
				"success": false,
				"error": "CSRF token validation failed",
			})
			return
		}
		
		c.Next()
	}
}

// GenerateCSRFToken generates a CSRF token for the current session
func GenerateCSRFToken(c *gin.Context) string {
	token := uuid.New().String()
	c.Set("csrf-token", token)
	return token
}