package jwt

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/oddbit-project/blueprint/provider/httpserver/fingerprint"
	"github.com/oddbit-project/blueprint/provider/httpserver/session/storage"
	"github.com/oddbit-project/blueprint/utils"
)

const (
	// Security-related errors
	ErrInvalidNonceWindow       = utils.Error("nonce window must be positive")
	ErrInvalidSessionLimit      = utils.Error("max concurrent sessions must be positive")
	ErrInvalidBlockDuration     = utils.Error("block duration must be positive")
	ErrInvalidActivityThreshold = utils.Error("suspicious activity threshold must be positive")
)

// SecurityConfig holds configuration for enhanced JWT security features
type SecurityConfig struct {
	// Master switch for all security features
	Enabled bool `json:"enabled"`

	// Device fingerprinting configuration
	DeviceFingerprintingEnabled bool `json:"deviceFingerprintingEnabled"` // Enable device fingerprinting
	RequireDeviceBinding        bool `json:"requireDeviceBinding"`        // Enforce strict device binding

	// IP validation configuration
	IPValidationEnabled   bool `json:"ipValidationEnabled"`   // Enable IP address validation
	AllowIPSubnetChange   bool `json:"allowIPSubnetChange"`   // Allow same /24 subnet changes
	GeolocationValidation bool `json:"geolocationValidation"` // Enable country-level validation

	// Nonce validation configuration
	NonceValidationEnabled bool          `json:"nonceValidationEnabled"` // Enable nonce-based replay prevention
	NonceWindow            time.Duration `json:"nonceWindow"`            // Nonce validity window

	// Session management configuration
	MaxConcurrentSessions int `json:"maxConcurrentSessions"` // Maximum sessions per user (0 = unlimited)

	// Suspicious activity detection
	SuspiciousActivityThreshold int           `json:"suspiciousActivityThreshold"` // Failed attempts before blocking
	BlockDuration               time.Duration `json:"blockDuration"`               // How long to block devices
	AutoSessionRegeneration     bool          `json:"autoSessionRegeneration"`     // Auto-regenerate on suspicious activity
}

// NewSecurityConfig creates a new security configuration with security enabled by default
func NewSecurityConfig() *SecurityConfig {
	return &SecurityConfig{
		Enabled:                     true, // ENABLED BY DEFAULT for security
		DeviceFingerprintingEnabled: true, // Enabled by default
		RequireDeviceBinding:        true, // Enabled by default
		IPValidationEnabled:         true, // Enabled by default
		AllowIPSubnetChange:         true, // Allow subnet changes for mobile users
		GeolocationValidation:       false, // Disabled by default (requires external service)
		NonceValidationEnabled:      true, // Enabled by default
		NonceWindow:                 5 * time.Minute,
		MaxConcurrentSessions:       3, // Reasonable default limit
		SuspiciousActivityThreshold: 3,
		BlockDuration:               30 * time.Minute,
		AutoSessionRegeneration:     true,
	}
}

// NewDisabledSecurityConfig creates a security configuration with all features disabled (for backward compatibility)
func NewDisabledSecurityConfig() *SecurityConfig {
	return &SecurityConfig{
		Enabled:                     false,
		DeviceFingerprintingEnabled: false,
		RequireDeviceBinding:        false,
		IPValidationEnabled:         false,
		AllowIPSubnetChange:         true,
		GeolocationValidation:       false,
		NonceValidationEnabled:      false,
		NonceWindow:                 5 * time.Minute,
		MaxConcurrentSessions:       0, // Unlimited
		SuspiciousActivityThreshold: 3,
		BlockDuration:               30 * time.Minute,
		AutoSessionRegeneration:     true,
	}
}

// NewHighSecurityConfig creates a high security configuration
func NewHighSecurityConfig() *SecurityConfig {
	return &SecurityConfig{
		Enabled:                     true,
		DeviceFingerprintingEnabled: true,
		RequireDeviceBinding:        true,
		IPValidationEnabled:         true,
		AllowIPSubnetChange:         false, // Strict IP validation
		GeolocationValidation:       true,
		NonceValidationEnabled:      true,
		NonceWindow:                 2 * time.Minute, // Shorter window
		MaxConcurrentSessions:       1,               // Single session only
		SuspiciousActivityThreshold: 1,               // Immediate blocking
		BlockDuration:               1 * time.Hour,   // Longer blocks
		AutoSessionRegeneration:     true,
	}
}

// NewMobileFriendlySecurityConfig creates a configuration optimized for mobile applications
func NewMobileFriendlySecurityConfig() *SecurityConfig {
	return &SecurityConfig{
		Enabled:                     true,
		DeviceFingerprintingEnabled: true,
		RequireDeviceBinding:        false, // Mobile devices change frequently
		IPValidationEnabled:         true,
		AllowIPSubnetChange:         true, // Mobile networks change subnets
		GeolocationValidation:       false, // May be problematic for VPN users
		NonceValidationEnabled:      true,
		NonceWindow:                 10 * time.Minute, // Longer window for mobile latency
		MaxConcurrentSessions:       5,                // Allow multiple device types
		SuspiciousActivityThreshold: 5,                // More lenient
		BlockDuration:               15 * time.Minute, // Shorter blocks
		AutoSessionRegeneration:     true,
	}
}

// WithFeatureControl allows granular control over security features
func (c *SecurityConfig) WithFeatureControl() *FeatureController {
	return &FeatureController{config: c}
}

// FeatureController provides granular control over security features
type FeatureController struct {
	config *SecurityConfig
}

// DisableDeviceFingerprinting disables device fingerprinting
func (fc *FeatureController) DisableDeviceFingerprinting() *FeatureController {
	fc.config.DeviceFingerprintingEnabled = false
	fc.config.RequireDeviceBinding = false
	return fc
}

// DisableIPValidation disables IP validation
func (fc *FeatureController) DisableIPValidation() *FeatureController {
	fc.config.IPValidationEnabled = false
	fc.config.GeolocationValidation = false
	return fc
}

// DisableNonceValidation disables nonce validation
func (fc *FeatureController) DisableNonceValidation() *FeatureController {
	fc.config.NonceValidationEnabled = false
	return fc
}

// DisableSessionLimiting disables session limiting
func (fc *FeatureController) DisableSessionLimiting() *FeatureController {
	fc.config.MaxConcurrentSessions = 0
	return fc
}

// DisableSuspiciousActivityDetection disables suspicious activity detection
func (fc *FeatureController) DisableSuspiciousActivityDetection() *FeatureController {
	fc.config.SuspiciousActivityThreshold = 0 // Effectively disabled
	return fc
}

// DisableAllSecurity disables all security features
func (fc *FeatureController) DisableAllSecurity() *FeatureController {
	fc.config.Enabled = false
	fc.config.DeviceFingerprintingEnabled = false
	fc.config.RequireDeviceBinding = false
	fc.config.IPValidationEnabled = false
	fc.config.GeolocationValidation = false
	fc.config.NonceValidationEnabled = false
	fc.config.MaxConcurrentSessions = 0
	fc.config.SuspiciousActivityThreshold = 0
	return fc
}

// Build returns the configured SecurityConfig
func (fc *FeatureController) Build() *SecurityConfig {
	return fc.config
}

// Validate validates the security configuration
func (c *SecurityConfig) Validate() error {
	if c.NonceWindow <= 0 {
		return ErrInvalidNonceWindow
	}
	if c.MaxConcurrentSessions < 0 {
		return ErrInvalidSessionLimit
	}
	if c.BlockDuration <= 0 {
		return ErrInvalidBlockDuration
	}
	if c.SuspiciousActivityThreshold < 0 {
		return ErrInvalidActivityThreshold
	}
	return nil
}

// SecurityLevel represents the level of security applied
type SecurityLevel string

const (
	SecurityLevelDisabled      SecurityLevel = "disabled"
	SecurityLevelBalanced      SecurityLevel = "balanced"
	SecurityLevelHigh          SecurityLevel = "high"
	SecurityLevelMobileFriendly SecurityLevel = "mobile_friendly"
)

// SessionSecurityValidator interface for custom security validation
type SessionSecurityValidator interface {
	ValidateSecurityEvent(c *gin.Context, event SecurityEvent) (SecurityAction, error)
}

// SecurityEvent represents a security-related event
type SecurityEvent struct {
	Type        SecurityEventType
	UserID      string
	SessionID   string
	IPAddress   string
	UserAgent   string
	Fingerprint *fingerprint.DeviceFingerprint
	Details     map[string]interface{}
	Timestamp   time.Time
}

// SecurityEventType represents the type of security event
type SecurityEventType string

const (
	SecurityEventNonceValidationFailed   SecurityEventType = "nonce_validation_failed"
	SecurityEventDeviceFingerprintFailed SecurityEventType = "device_fingerprint_failed"
	SecurityEventIPValidationFailed      SecurityEventType = "ip_validation_failed"
	SecurityEventGeolocationFailed       SecurityEventType = "geolocation_failed"
	SecurityEventSessionLimitExceeded    SecurityEventType = "session_limit_exceeded"
	SecurityEventSuspiciousActivity      SecurityEventType = "suspicious_activity"
)

// SecurityAction represents the action to take in response to a security event
type SecurityAction string

const (
	SecurityActionAllow           SecurityAction = "allow"
	SecurityActionWarn            SecurityAction = "warn"
	SecurityActionBlock           SecurityAction = "block"
	SecurityActionRegenerateSession SecurityAction = "regenerate_session"
	SecurityActionExtendedBlock   SecurityAction = "extended_block"
)

// DefaultSessionSecurityValidator provides default security validation logic
type DefaultSessionSecurityValidator struct {
	storage storage.SecurityStorage
}

// NewDefaultSessionSecurityValidator creates a new default session security validator
func NewDefaultSessionSecurityValidator() *DefaultSessionSecurityValidator {
	return &DefaultSessionSecurityValidator{
		storage: storage.NewMemorySecurityStorage(),
	}
}

// ValidateSecurityEvent validates a security event and returns the appropriate action
func (v *DefaultSessionSecurityValidator) ValidateSecurityEvent(c *gin.Context, event SecurityEvent) (SecurityAction, error) {
	// This is a simplified implementation
	// In a real-world scenario, you'd implement sophisticated logic here
	
	switch event.Type {
	case SecurityEventNonceValidationFailed:
		return SecurityActionWarn, nil
	case SecurityEventDeviceFingerprintFailed:
		return SecurityActionBlock, nil
	case SecurityEventIPValidationFailed:
		return SecurityActionWarn, nil
	case SecurityEventGeolocationFailed:
		return SecurityActionWarn, nil
	case SecurityEventSessionLimitExceeded:
		return SecurityActionBlock, nil
	case SecurityEventSuspiciousActivity:
		return SecurityActionRegenerateSession, nil
	default:
		return SecurityActionAllow, nil
	}
}