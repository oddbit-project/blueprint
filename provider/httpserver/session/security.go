package session

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

// NewSecurityConfig creates a new security configuration with safe defaults
func NewSecurityConfig() *SecurityConfig {
	return &SecurityConfig{
		Enabled:                     false, // Disabled by default for backward compatibility
		DeviceFingerprintingEnabled: false,
		RequireDeviceBinding:        false,
		IPValidationEnabled:         false,
		AllowIPSubnetChange:         true,
		GeolocationValidation:       false,
		NonceValidationEnabled:      false,
		NonceWindow:                 5 * time.Minute,
		MaxConcurrentSessions:       0, // Unlimited by default
		SuspiciousActivityThreshold: 3,
		BlockDuration:               30 * time.Minute,
		AutoSessionRegeneration:     true,
	}
}

// NewBalancedSecurityConfig creates a balanced security configuration
func NewBalancedSecurityConfig() *SecurityConfig {
	return &SecurityConfig{
		Enabled:                     true,
		DeviceFingerprintingEnabled: true,
		RequireDeviceBinding:        true,
		IPValidationEnabled:         true,
		AllowIPSubnetChange:         true,
		GeolocationValidation:       true,
		NonceValidationEnabled:      true,
		NonceWindow:                 5 * time.Minute,
		MaxConcurrentSessions:       3,
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

// NewMobileSecurityConfig creates a mobile-friendly security configuration
func NewMobileSecurityConfig() *SecurityConfig {
	return &SecurityConfig{
		Enabled:                     true,
		DeviceFingerprintingEnabled: true,
		RequireDeviceBinding:        false, // More flexible for mobile
		IPValidationEnabled:         true,
		AllowIPSubnetChange:         true,
		GeolocationValidation:       false, // Mobile users travel
		NonceValidationEnabled:      true,
		NonceWindow:                 10 * time.Minute, // Longer window for mobile
		MaxConcurrentSessions:       5,                // Multiple devices
		SuspiciousActivityThreshold: 5,                // More tolerant
		BlockDuration:               15 * time.Minute, // Shorter blocks
		AutoSessionRegeneration:     true,
	}
}

// Validate validates the security configuration
func (c *SecurityConfig) Validate() error {
	if !c.Enabled {
		return nil // No validation needed if disabled
	}

	if c.NonceValidationEnabled && c.NonceWindow <= 0 {
		return ErrInvalidNonceWindow
	}

	if c.MaxConcurrentSessions < 0 {
		return ErrInvalidSessionLimit
	}

	if c.BlockDuration <= 0 {
		return ErrInvalidBlockDuration
	}

	if c.SuspiciousActivityThreshold <= 0 {
		return ErrInvalidActivityThreshold
	}

	return nil
}

// SecurityLevel represents predefined security configurations
type SecurityLevel string

const (
	SecurityLevelDisabled SecurityLevel = "disabled"
	SecurityLevelBalanced SecurityLevel = "balanced"
	SecurityLevelHigh     SecurityLevel = "high"
	SecurityLevelMobile   SecurityLevel = "mobile"
)

// NewSecurityConfigFromLevel creates a security config from a predefined level
func NewSecurityConfigFromLevel(level SecurityLevel) *SecurityConfig {
	switch level {
	case SecurityLevelHigh:
		return NewHighSecurityConfig()
	case SecurityLevelMobile:
		return NewMobileSecurityConfig()
	case SecurityLevelBalanced:
		return NewBalancedSecurityConfig()
	case SecurityLevelDisabled:
		fallthrough
	default:
		return NewSecurityConfig() // Disabled by default
	}
}

// DeviceFingerprintValidator provides validation logic for device fingerprints with security config
type DeviceFingerprintValidator struct {
	generator *fingerprint.Generator
}

// NewDeviceFingerprintValidator creates a new device fingerprint validator
func NewDeviceFingerprintValidator() *DeviceFingerprintValidator {
	// Create fingerprint generator with default config
	generator := fingerprint.NewGenerator(fingerprint.NewDefaultConfig())
	return &DeviceFingerprintValidator{
		generator: generator,
	}
}

// NewDeviceFingerprintValidatorWithConfig creates a validator with custom fingerprint config
func NewDeviceFingerprintValidatorWithConfig(fpConfig *fingerprint.Config) *DeviceFingerprintValidator {
	generator := fingerprint.NewGenerator(fpConfig)
	return &DeviceFingerprintValidator{
		generator: generator,
	}
}

// GenerateFingerprint creates a device fingerprint from a Gin context
func (v *DeviceFingerprintValidator) GenerateFingerprint(c *gin.Context) *fingerprint.DeviceFingerprint {
	return v.generator.Generate(c)
}

// ValidateFingerprint compares two device fingerprints based on security configuration
func (v *DeviceFingerprintValidator) ValidateFingerprint(stored, current *fingerprint.DeviceFingerprint, config *SecurityConfig) bool {
	if stored == nil || current == nil {
		return false
	}

	// Determine strictness based on security config
	strict := !config.AllowIPSubnetChange || config.RequireDeviceBinding

	// Use the fingerprint generator's comparison logic
	if v.generator.Compare(stored, current, strict) {
		return true
	}

	// Additional validation based on security config
	if config.IPValidationEnabled {
		if config.AllowIPSubnetChange {
			// Allow changes within same subnet
			if stored.IPSubnet != current.IPSubnet {
				return false
			}
		} else {
			// Require exact IP match
			if stored.IPAddress != current.IPAddress {
				return false
			}
		}
	}

	// Geolocation validation if enabled
	if config.GeolocationValidation {
		if stored.Country != current.Country {
			return false
		}
	}

	return false
}

// DetectSuspiciousActivity analyzes differences between fingerprints
func (v *DeviceFingerprintValidator) DetectSuspiciousActivity(stored, current *fingerprint.DeviceFingerprint) []string {
	return v.generator.DetectChanges(stored, current)
}

// GetGenerator returns the underlying fingerprint generator
func (v *DeviceFingerprintValidator) GetGenerator() *fingerprint.Generator {
	return v.generator
}

// SessionSecurityValidator defines the interface for validating session security
type SessionSecurityValidator interface {
	// ValidateSessionSecurity validates security context for an existing session
	// Returns an error if validation fails, nil if validation passes
	ValidateSessionSecurity(c *gin.Context, session *SessionData, currentFingerprint *fingerprint.DeviceFingerprint, config *SecurityConfig, storage storage.SecurityStorage) error
}

// DefaultSessionSecurityValidator provides the default implementation of session security validation
type DefaultSessionSecurityValidator struct {
	fingerprintValidator *DeviceFingerprintValidator
}

// NewDefaultSessionSecurityValidator creates a new default session security validator
func NewDefaultSessionSecurityValidator() SessionSecurityValidator {
	return &DefaultSessionSecurityValidator{
		fingerprintValidator: NewDeviceFingerprintValidator(),
	}
}

// NewDefaultSessionSecurityValidatorWithFingerprintValidator creates a validator with custom fingerprint validator
func NewDefaultSessionSecurityValidatorWithFingerprintValidator(fpValidator *DeviceFingerprintValidator) SessionSecurityValidator {
	return &DefaultSessionSecurityValidator{
		fingerprintValidator: fpValidator,
	}
}

// ValidateSessionSecurity implements the default session security validation logic
func (v *DefaultSessionSecurityValidator) ValidateSessionSecurity(c *gin.Context, session *SessionData, currentFingerprint *fingerprint.DeviceFingerprint, config *SecurityConfig, securityStorage storage.SecurityStorage) error {
	if !config.DeviceFingerprintingEnabled {
		return nil // No validation needed
	}

	// Get stored security context
	securityCtx, err := securityStorage.GetSecurityContext(session.ID)
	if err != nil {
		return utils.Error("failed to get security context: " + err.Error())
	}

	// If no security context exists, this is a new session
	if securityCtx == nil {
		return nil
	}

	// Validate device fingerprint
	if config.RequireDeviceBinding {
		if !v.fingerprintValidator.ValidateFingerprint(securityCtx.DeviceFingerprint, currentFingerprint, config) {
			return utils.Error("device fingerprint mismatch")
		}
	}

	// Update last activity
	securityCtx.LastActivity = time.Now().Unix()
	securityStorage.StoreSecurityContext(session.ID, securityCtx)

	return nil
}
