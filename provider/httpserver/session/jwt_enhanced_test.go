package session

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/oddbit-project/blueprint/log"
	"github.com/oddbit-project/blueprint/provider/httpserver/fingerprint"
	"github.com/oddbit-project/blueprint/provider/httpserver/session/storage"
)

func TestEnhancedJWTSessionManager(t *testing.T) {
	// Setup test environment
	gin.SetMode(gin.TestMode)
	
	// Configure logger for testing
	log.Configure(log.NewDefaultConfig())
	logger := log.New("test-enhanced-jwt")

	// Create enhanced JWT configuration
	jwtConfig := NewJWTConfig()
	jwtConfig.SigningKey = []byte("test-signing-key")
	jwtConfig.Issuer = "test-issuer"
	jwtConfig.Audience = "test-audience"

	securityConfig := NewBalancedSecurityConfig()
	securityStorage := storage.NewMemorySecurityStorage()

	enhancedConfig := NewEnhancedJWTConfig(jwtConfig, securityConfig, securityStorage)
	enhancedManager, err := NewEnhancedJWTSessionManager(enhancedConfig, logger)
	if err != nil {
		t.Fatalf("Failed to create enhanced JWT manager: %v", err)
	}
	defer enhancedManager.Close()

	// Create test router
	router := gin.New()
	router.Use(enhancedManager.Middleware())

	// Add test routes
	router.POST("/login", func(c *gin.Context) {
		Set(c, "authenticated", true)
		Set(c, "user_id", "testuser")
		Set(c, "role", "user")
		c.JSON(200, gin.H{"message": "logged in"})
	})

	router.GET("/protected", func(c *gin.Context) {
		authenticated, _ := GetBool(c, "authenticated")
		if !authenticated {
			c.JSON(401, gin.H{"error": "not authenticated"})
			return
		}
		c.JSON(200, gin.H{"message": "protected resource"})
	})

	// Test 1: Basic login should work
	t.Run("BasicLogin", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/login", bytes.NewBufferString(`{}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "test-agent")
		
		router.ServeHTTP(w, req)
		
		if w.Code != 200 {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		authHeader := w.Header().Get("Authorization")
		if authHeader == "" {
			t.Error("Expected Authorization header with JWT token")
		}
	})

	// Test 2: Security stats should be available
	t.Run("SecurityStats", func(t *testing.T) {
		stats := enhancedManager.GetSecurityStats()
		
		if !stats["security_enabled"].(bool) {
			t.Error("Expected security to be enabled")
		}

		features := stats["features"].(map[string]bool)
		if !features["device_fingerprinting"] {
			t.Error("Expected device fingerprinting to be enabled")
		}
	})
}

func TestDeviceFingerprintGenerator(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	// Create test gin context
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("User-Agent", "test-agent")
	req.Header.Set("Accept-Language", "en-US")
	req.Header.Set("X-Timezone", "UTC")
	c.Request = req

	generator := fingerprint.NewGenerator(fingerprint.NewDefaultConfig())
	fp := generator.Generate(c)

	if fp.UserAgent != "test-agent" {
		t.Errorf("Expected User-Agent 'test-agent', got '%s'", fp.UserAgent)
	}
	if fp.Fingerprint == "" {
		t.Error("Expected non-empty fingerprint hash")
	}
	if fp.Timezone != "UTC" {
		t.Errorf("Expected timezone 'UTC', got '%s'", fp.Timezone)
	}
}

func TestSecurityStorage(t *testing.T) {
	securityStorage := storage.NewMemorySecurityStorage()

	// Test nonce storage
	nonce := fmt.Sprintf("test-nonce-%d", time.Now().UnixNano())
	err := securityStorage.StoreNonce(nonce, 5*time.Minute)
	if err != nil {
		t.Errorf("Failed to store nonce: %v", err)
	}

	if !securityStorage.NonceExists(nonce) {
		t.Error("Nonce should exist after storing")
	}

	// Test device fingerprint storage
	fp := &fingerprint.DeviceFingerprint{
		UserAgent:   "test-agent",
		Fingerprint: "test-fingerprint-hash",
		CreatedAt:   time.Now().Unix(),
	}

	err = securityStorage.StoreDeviceFingerprint("session123", fp)
	if err != nil {
		t.Errorf("Failed to store device fingerprint: %v", err)
	}

	retrieved, err := securityStorage.GetDeviceFingerprint("session123")
	if err != nil {
		t.Errorf("Failed to get device fingerprint: %v", err)
	}
	if retrieved == nil || retrieved.UserAgent != "test-agent" {
		t.Error("Failed to retrieve correct device fingerprint")
	}

	// Test device blocking
	err = securityStorage.BlockDevice("test-fingerprint", time.Now().Add(1*time.Hour))
	if err != nil {
		t.Errorf("Failed to block device: %v", err)
	}

	if !securityStorage.IsDeviceBlocked("test-fingerprint") {
		t.Error("Device should be blocked")
	}

	// Test user session tracking
	err = securityStorage.TrackUserSession("user123", "session1")
	if err != nil {
		t.Errorf("Failed to track user session: %v", err)
	}

	sessions := securityStorage.GetUserSessions("user123")
	if len(sessions) != 1 || sessions[0] != "session1" {
		t.Error("Failed to track user session correctly")
	}
}

func TestSecurityConfig(t *testing.T) {
	// Test default config (disabled)
	defaultConfig := NewSecurityConfig()
	if defaultConfig.Enabled {
		t.Error("Default config should be disabled for backward compatibility")
	}

	// Test balanced config
	balancedConfig := NewBalancedSecurityConfig()
	if !balancedConfig.Enabled {
		t.Error("Balanced config should be enabled")
	}
	if !balancedConfig.DeviceFingerprintingEnabled {
		t.Error("Balanced config should have device fingerprinting enabled")
	}

	// Test high security config
	highConfig := NewHighSecurityConfig()
	if highConfig.MaxConcurrentSessions != 1 {
		t.Error("High security config should limit to 1 concurrent session")
	}
	if highConfig.AllowIPSubnetChange {
		t.Error("High security config should not allow IP subnet changes")
	}

	// Test mobile config
	mobileConfig := NewMobileSecurityConfig()
	if mobileConfig.RequireDeviceBinding {
		t.Error("Mobile config should not require strict device binding")
	}
	if mobileConfig.GeolocationValidation {
		t.Error("Mobile config should not require geolocation validation")
	}

	// Test config validation
	invalidConfig := &SecurityConfig{
		Enabled:                     true,
		NonceValidationEnabled:      true,
		NonceWindow:                 -1 * time.Minute, // Invalid
		SuspiciousActivityThreshold: 0,                // Invalid
	}

	if err := invalidConfig.Validate(); err == nil {
		t.Error("Expected validation error for invalid config")
	}
}

func TestCustomSessionSecurityValidator(t *testing.T) {
	// Setup test environment
	gin.SetMode(gin.TestMode)
	
	// Configure logger for testing
	log.Configure(log.NewDefaultConfig())
	logger := log.New("test-custom-validator")

	// Create enhanced JWT configuration
	jwtConfig := NewJWTConfig()
	jwtConfig.SigningKey = []byte("test-signing-key")

	securityConfig := NewBalancedSecurityConfig()
	securityStorage := storage.NewMemorySecurityStorage()

	enhancedConfig := NewEnhancedJWTConfig(jwtConfig, securityConfig, securityStorage)

	// Test 1: Default validator (should be nil, will use default)
	if enhancedConfig.SessionSecurityValidator != nil {
		t.Error("Expected default SessionSecurityValidator to be nil")
	}

	// Test 2: Custom validator
	customValidator := &MockSessionSecurityValidator{
		shouldFail: false,
		callCount:  0,
	}

	enhancedConfig.WithSessionSecurityValidator(customValidator)
	if enhancedConfig.SessionSecurityValidator != customValidator {
		t.Error("Expected custom validator to be set")
	}

	// Test 3: Enhanced manager uses custom validator
	enhancedManager, err := NewEnhancedJWTSessionManager(enhancedConfig, logger)
	if err != nil {
		t.Fatalf("Failed to create enhanced JWT manager: %v", err)
	}
	defer enhancedManager.Close()

	if enhancedManager.sessionSecurityValidator != customValidator {
		t.Error("Expected enhanced manager to use custom validator")
	}

	// Test 4: Custom validator is called during validation
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("User-Agent", "test-agent")
	c.Request = req

	// Create a mock session
	session := &SessionData{
		ID:     "test-session",
		Values: make(map[string]interface{}),
	}

	// Create a fingerprint
	generator := fingerprint.NewGenerator(fingerprint.NewDefaultConfig())
	fp := generator.Generate(c)

	// Call validation (should call our custom validator)
	err = enhancedManager.validateSessionSecurity(c, session, fp)
	if err != nil {
		t.Errorf("Unexpected validation error: %v", err)
	}

	// Verify custom validator was called
	if customValidator.callCount != 1 {
		t.Errorf("Expected custom validator to be called once, got %d calls", customValidator.callCount)
	}

	// Test 5: Custom validator failure
	customValidator.shouldFail = true
	err = enhancedManager.validateSessionSecurity(c, session, fp)
	if err == nil {
		t.Error("Expected validation to fail when custom validator returns error")
	}
	if customValidator.callCount != 2 {
		t.Errorf("Expected custom validator to be called twice, got %d calls", customValidator.callCount)
	}
}

// MockSessionSecurityValidator is a mock implementation for testing
type MockSessionSecurityValidator struct {
	shouldFail bool
	callCount  int
}

func (m *MockSessionSecurityValidator) ValidateSessionSecurity(c *gin.Context, session *SessionData, currentFingerprint *fingerprint.DeviceFingerprint, config *SecurityConfig, securityStorage storage.SecurityStorage) error {
	m.callCount++
	if m.shouldFail {
		return fmt.Errorf("mock validation failure")
	}
	return nil
}