package jwt

import (
	"github.com/oddbit-project/blueprint/provider/auth/jwt/storage"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/oddbit-project/blueprint/provider/httpserver/fingerprint"
	"github.com/stretchr/testify/assert"
)

func TestNewSecurityConfig(t *testing.T) {
	config := NewSecurityConfig()

	// Should be enabled by default
	assert.True(t, config.Enabled)
	assert.True(t, config.DeviceFingerprintingEnabled)
	assert.True(t, config.RequireDeviceBinding)
	assert.True(t, config.IPValidationEnabled)
	assert.True(t, config.AllowIPSubnetChange)
	assert.False(t, config.GeolocationValidation)
	assert.True(t, config.NonceValidationEnabled)
	assert.Equal(t, 5*time.Minute, config.NonceWindow)
	assert.Equal(t, 3, config.MaxConcurrentSessions)
	assert.Equal(t, 3, config.SuspiciousActivityThreshold)
	assert.Equal(t, 30*time.Minute, config.BlockDuration)
	assert.True(t, config.AutoSessionRegeneration)
}

func TestNewDisabledSecurityConfig(t *testing.T) {
	config := NewDisabledSecurityConfig()

	// Should be disabled
	assert.False(t, config.Enabled)
	assert.False(t, config.DeviceFingerprintingEnabled)
	assert.False(t, config.RequireDeviceBinding)
	assert.False(t, config.IPValidationEnabled)
	assert.True(t, config.AllowIPSubnetChange)
	assert.False(t, config.GeolocationValidation)
	assert.False(t, config.NonceValidationEnabled)
	assert.Equal(t, 5*time.Minute, config.NonceWindow)
	assert.Equal(t, 0, config.MaxConcurrentSessions) // Unlimited
	assert.Equal(t, 3, config.SuspiciousActivityThreshold)
	assert.Equal(t, 30*time.Minute, config.BlockDuration)
	assert.True(t, config.AutoSessionRegeneration)
}

func TestNewHighSecurityConfig(t *testing.T) {
	config := NewHighSecurityConfig()

	assert.True(t, config.Enabled)
	assert.True(t, config.DeviceFingerprintingEnabled)
	assert.True(t, config.RequireDeviceBinding)
	assert.True(t, config.IPValidationEnabled)
	assert.False(t, config.AllowIPSubnetChange) // Strict IP validation
	assert.True(t, config.GeolocationValidation)
	assert.True(t, config.NonceValidationEnabled)
	assert.Equal(t, 2*time.Minute, config.NonceWindow)     // Shorter window
	assert.Equal(t, 1, config.MaxConcurrentSessions)       // Single session only
	assert.Equal(t, 1, config.SuspiciousActivityThreshold) // Immediate blocking
	assert.Equal(t, 1*time.Hour, config.BlockDuration)     // Longer blocks
	assert.True(t, config.AutoSessionRegeneration)
}

func TestNewMobileFriendlySecurityConfig(t *testing.T) {
	config := NewMobileFriendlySecurityConfig()

	assert.True(t, config.Enabled)
	assert.True(t, config.DeviceFingerprintingEnabled)
	assert.False(t, config.RequireDeviceBinding) // Mobile devices change frequently
	assert.True(t, config.IPValidationEnabled)
	assert.True(t, config.AllowIPSubnetChange)    // Mobile networks change subnets
	assert.False(t, config.GeolocationValidation) // May be problematic for VPN users
	assert.True(t, config.NonceValidationEnabled)
	assert.Equal(t, 10*time.Minute, config.NonceWindow)    // Longer window for mobile latency
	assert.Equal(t, 5, config.MaxConcurrentSessions)       // Allow multiple device types
	assert.Equal(t, 5, config.SuspiciousActivityThreshold) // More lenient
	assert.Equal(t, 15*time.Minute, config.BlockDuration)  // Shorter blocks
	assert.True(t, config.AutoSessionRegeneration)
}

func TestSecurityConfigValidate(t *testing.T) {
	t.Run("Valid config", func(t *testing.T) {
		config := NewSecurityConfig()
		err := config.Validate()
		assert.NoError(t, err)
	})

	t.Run("Invalid nonce window", func(t *testing.T) {
		config := NewSecurityConfig()
		config.NonceWindow = 0
		err := config.Validate()
		assert.Error(t, err)
		assert.Equal(t, ErrInvalidNonceWindow, err)
	})

	t.Run("Negative nonce window", func(t *testing.T) {
		config := NewSecurityConfig()
		config.NonceWindow = -time.Minute
		err := config.Validate()
		assert.Error(t, err)
		assert.Equal(t, ErrInvalidNonceWindow, err)
	})

	t.Run("Negative session limit", func(t *testing.T) {
		config := NewSecurityConfig()
		config.MaxConcurrentSessions = -1
		err := config.Validate()
		assert.Error(t, err)
		assert.Equal(t, ErrInvalidSessionLimit, err)
	})

	t.Run("Invalid block duration", func(t *testing.T) {
		config := NewSecurityConfig()
		config.BlockDuration = 0
		err := config.Validate()
		assert.Error(t, err)
		assert.Equal(t, ErrInvalidBlockDuration, err)
	})

	t.Run("Negative activity threshold", func(t *testing.T) {
		config := NewSecurityConfig()
		config.SuspiciousActivityThreshold = -1
		err := config.Validate()
		assert.Error(t, err)
		assert.Equal(t, ErrInvalidActivityThreshold, err)
	})
}

func TestFeatureController(t *testing.T) {
	t.Run("Disable device fingerprinting", func(t *testing.T) {
		config := NewSecurityConfig().WithFeatureControl().
			DisableDeviceFingerprinting().
			Build()

		assert.False(t, config.DeviceFingerprintingEnabled)
		assert.False(t, config.RequireDeviceBinding)
	})

	t.Run("Disable IP validation", func(t *testing.T) {
		config := NewSecurityConfig().WithFeatureControl().
			DisableIPValidation().
			Build()

		assert.False(t, config.IPValidationEnabled)
		assert.False(t, config.GeolocationValidation)
	})

	t.Run("Disable nonce validation", func(t *testing.T) {
		config := NewSecurityConfig().WithFeatureControl().
			DisableNonceValidation().
			Build()

		assert.False(t, config.NonceValidationEnabled)
	})

	t.Run("Disable session limiting", func(t *testing.T) {
		config := NewSecurityConfig().WithFeatureControl().
			DisableSessionLimiting().
			Build()

		assert.Equal(t, 0, config.MaxConcurrentSessions)
	})

	t.Run("Disable suspicious activity detection", func(t *testing.T) {
		config := NewSecurityConfig().WithFeatureControl().
			DisableSuspiciousActivityDetection().
			Build()

		assert.Equal(t, 0, config.SuspiciousActivityThreshold)
	})

	t.Run("Disable all security", func(t *testing.T) {
		config := NewSecurityConfig().WithFeatureControl().
			DisableAllSecurity().
			Build()

		assert.False(t, config.Enabled)
		assert.False(t, config.DeviceFingerprintingEnabled)
		assert.False(t, config.RequireDeviceBinding)
		assert.False(t, config.IPValidationEnabled)
		assert.False(t, config.GeolocationValidation)
		assert.False(t, config.NonceValidationEnabled)
		assert.Equal(t, 0, config.MaxConcurrentSessions)
		assert.Equal(t, 0, config.SuspiciousActivityThreshold)
	})

	t.Run("Chain multiple operations", func(t *testing.T) {
		config := NewSecurityConfig().WithFeatureControl().
			DisableDeviceFingerprinting().
			DisableNonceValidation().
			DisableSessionLimiting().
			Build()

		assert.False(t, config.DeviceFingerprintingEnabled)
		assert.False(t, config.RequireDeviceBinding)
		assert.False(t, config.NonceValidationEnabled)
		assert.Equal(t, 0, config.MaxConcurrentSessions)

		// These should still be enabled
		assert.True(t, config.Enabled)
		assert.True(t, config.IPValidationEnabled)
	})
}

func TestSecurityEventTypes(t *testing.T) {
	eventTypes := []SecurityEventType{
		SecurityEventNonceValidationFailed,
		SecurityEventDeviceFingerprintFailed,
		SecurityEventIPValidationFailed,
		SecurityEventGeolocationFailed,
		SecurityEventSessionLimitExceeded,
		SecurityEventSuspiciousActivity,
	}

	for _, eventType := range eventTypes {
		assert.NotEmpty(t, string(eventType))
	}
}

func TestSecurityActions(t *testing.T) {
	actions := []SecurityAction{
		SecurityActionAllow,
		SecurityActionWarn,
		SecurityActionBlock,
		SecurityActionRegenerateSession,
		SecurityActionExtendedBlock,
	}

	for _, action := range actions {
		assert.NotEmpty(t, string(action))
	}
}

func TestSecurityEvent(t *testing.T) {
	event := SecurityEvent{
		Type:        SecurityEventNonceValidationFailed,
		UserID:      "user-123",
		SessionID:   "session-456",
		IPAddress:   "192.168.1.1",
		UserAgent:   "test-agent",
		Fingerprint: &fingerprint.DeviceFingerprint{UserAgent: "test-agent"},
		Details:     map[string]interface{}{"reason": "missing nonce"},
		Timestamp:   time.Now(),
	}

	assert.Equal(t, SecurityEventNonceValidationFailed, event.Type)
	assert.Equal(t, "user-123", event.UserID)
	assert.Equal(t, "session-456", event.SessionID)
	assert.Equal(t, "192.168.1.1", event.IPAddress)
	assert.Equal(t, "test-agent", event.UserAgent)
	assert.NotNil(t, event.Fingerprint)
	assert.Contains(t, event.Details, "reason")
	assert.False(t, event.Timestamp.IsZero())
}

func TestDefaultSessionSecurityValidator(t *testing.T) {
	validator := NewDefaultSessionSecurityValidator()
	assert.NotNil(t, validator)
	assert.NotNil(t, validator.storage)

	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(nil)

	tests := []struct {
		eventType      SecurityEventType
		expectedAction SecurityAction
	}{
		{SecurityEventNonceValidationFailed, SecurityActionWarn},
		{SecurityEventDeviceFingerprintFailed, SecurityActionBlock},
		{SecurityEventIPValidationFailed, SecurityActionWarn},
		{SecurityEventGeolocationFailed, SecurityActionWarn},
		{SecurityEventSessionLimitExceeded, SecurityActionBlock},
		{SecurityEventSuspiciousActivity, SecurityActionRegenerateSession},
	}

	for _, tt := range tests {
		t.Run(string(tt.eventType), func(t *testing.T) {
			event := SecurityEvent{Type: tt.eventType}
			action, err := validator.ValidateSecurityEvent(c, event)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedAction, action)
		})
	}

	t.Run("Unknown event type", func(t *testing.T) {
		event := SecurityEvent{Type: "unknown_event"}
		action, err := validator.ValidateSecurityEvent(c, event)
		assert.NoError(t, err)
		assert.Equal(t, SecurityActionAllow, action)
	})
}

func TestSecurityLevels(t *testing.T) {
	levels := []SecurityLevel{
		SecurityLevelDisabled,
		SecurityLevelBalanced,
		SecurityLevelHigh,
		SecurityLevelMobileFriendly,
	}

	for _, level := range levels {
		assert.NotEmpty(t, string(level))
	}
}

func TestSecurityConfigWithFeatureController(t *testing.T) {
	config := NewSecurityConfig()
	featureController := config.WithFeatureControl()

	assert.NotNil(t, featureController)
	assert.Equal(t, config, featureController.config)

	// Test that modifying through feature controller changes the original config
	featureController.DisableDeviceFingerprinting()
	assert.False(t, config.DeviceFingerprintingEnabled)
	assert.False(t, config.RequireDeviceBinding)

	// Test Build returns the same config
	builtConfig := featureController.Build()
	assert.Equal(t, config, builtConfig)
}

func TestSecurityConfigErrors(t *testing.T) {
	// Test all error constants exist and are not empty
	assert.NotEmpty(t, string(ErrInvalidNonceWindow))
	assert.NotEmpty(t, string(ErrInvalidSessionLimit))
	assert.NotEmpty(t, string(ErrInvalidBlockDuration))
	assert.NotEmpty(t, string(ErrInvalidActivityThreshold))
}

func TestSessionSecurityValidatorInterface(t *testing.T) {
	// Test that DefaultSessionSecurityValidator implements the interface
	var validator SessionSecurityValidator = NewDefaultSessionSecurityValidator()
	assert.NotNil(t, validator)

	// Test the interface method
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(nil)
	event := SecurityEvent{Type: SecurityEventNonceValidationFailed}

	action, err := validator.ValidateSecurityEvent(c, event)
	assert.NoError(t, err)
	assert.Equal(t, SecurityActionWarn, action)
}

func TestDefaultSessionSecurityValidatorStorage(t *testing.T) {
	validator := NewDefaultSessionSecurityValidator()

	// Verify it creates a memory storage by default
	assert.NotNil(t, validator.storage)

	// Test that it's actually a memory storage by checking interface compliance
	var _ storage.SecurityStorage = validator.storage
}
