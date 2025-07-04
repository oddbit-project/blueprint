package jwt

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/oddbit-project/blueprint/log"
	"github.com/oddbit-project/blueprint/provider/httpserver/fingerprint"
	"github.com/oddbit-project/blueprint/provider/httpserver/session"
	"github.com/oddbit-project/blueprint/provider/httpserver/session/storage"
)

// EnhancedJWTConfig extends JWT configuration with security features
type EnhancedJWTConfig struct {
	*JWTConfig                                        // Embed existing JWT configuration
	SecurityConfig           *SecurityConfig          // Enhanced security configuration
	SecurityStorage          storage.SecurityStorage  // Storage for security data
	SessionSecurityValidator SessionSecurityValidator // Custom session security validator (optional)
}

// NewEnhancedJWTConfig creates a new enhanced JWT configuration with security enabled by default
func NewEnhancedJWTConfig(jwtConfig *JWTConfig, securityConfig *SecurityConfig, securityStorage storage.SecurityStorage) *EnhancedJWTConfig {
	if jwtConfig == nil {
		jwtConfig = NewJWTConfig(RandomJWTKey())
	}
	if securityConfig == nil {
		securityConfig = NewSecurityConfig() // Security enabled by default
	}
	if securityStorage == nil {
		securityStorage = storage.NewMemorySecurityStorage()
	}

	return &EnhancedJWTConfig{
		JWTConfig:                jwtConfig,
		SecurityConfig:           securityConfig,
		SecurityStorage:          securityStorage,
		SessionSecurityValidator: nil, // Will use default if not set
	}
}

// Validate validates both JWT and security configurations
func (c *EnhancedJWTConfig) Validate() error {
	// Validate base JWT config
	if err := c.JWTConfig.Validate(); err != nil {
		return err
	}

	// Validate security config
	if err := c.SecurityConfig.Validate(); err != nil {
		return err
	}

	return nil
}

// WithSessionSecurityValidator sets a custom session security validator
func (c *EnhancedJWTConfig) WithSessionSecurityValidator(validator SessionSecurityValidator) *EnhancedJWTConfig {
	c.SessionSecurityValidator = validator
	return c
}

// EnhancedJWTSessionManager provides JWT session management with enhanced security features
type EnhancedJWTSessionManager struct {
	*JWTSessionManager       // Embed existing JWT session manager
	config                   *EnhancedJWTConfig
	logger                   *log.Logger
	fingerprintValidator     *DeviceFingerprintValidator
	sessionSecurityValidator SessionSecurityValidator
	mutex                    sync.RWMutex
	cleanupTicker            *time.Ticker
	stopCleanup              chan bool
	cleanupRunning           bool
}

// NewEnhancedJWTSessionManager creates a new enhanced JWT session manager
func NewEnhancedJWTSessionManager(config *EnhancedJWTConfig, logger *log.Logger) (*EnhancedJWTSessionManager, error) {
	if config == nil {
		return nil, fmt.Errorf("enhanced JWT config is required")
	}

	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid enhanced JWT config: %w", err)
	}

	// Create base JWT manager
	baseManager, err := NewJWTManager(config.JWTConfig, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create base JWT manager: %w", err)
	}

	// Create base JWT session manager
	jwtSessionManager := NewJWTSessionManager(baseManager)

	// Use custom session security validator or default
	sessionSecurityValidator := config.SessionSecurityValidator
	if sessionSecurityValidator == nil {
		sessionSecurityValidator = NewDefaultSessionSecurityValidator()
	}

	// Create device fingerprint validator if enabled
	var fingerprintValidator *DeviceFingerprintValidator
	if config.SecurityConfig.DeviceFingerprintingEnabled {
		fingerprintValidator = NewDeviceFingerprintValidator(
			config.SecurityConfig.RequireDeviceBinding,
			config.SecurityStorage,
			logger,
		)
	}

	manager := &EnhancedJWTSessionManager{
		JWTSessionManager:        jwtSessionManager,
		config:                   config,
		logger:                   logger,
		fingerprintValidator:     fingerprintValidator,
		sessionSecurityValidator: sessionSecurityValidator,
		stopCleanup:              make(chan bool),
	}

	// Start cleanup if security is enabled
	if config.SecurityConfig.Enabled {
		manager.startCleanup()
	}

	return manager, nil
}

// Middleware returns an enhanced Gin middleware with security features
func (m *EnhancedJWTSessionManager) Middleware() gin.HandlerFunc {
	// If security is disabled, use the base middleware
	if !m.config.SecurityConfig.Enabled {
		return m.JWTSessionManager.Middleware()
	}

	return func(c *gin.Context) {
		var sessionData *session.SessionData
		var tokenString string

		// Extract client information for security validation
		clientIP := c.ClientIP()
		userAgent := c.GetHeader("User-Agent")

		// Try to get the token from the Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader != "" && strings.HasPrefix(authHeader, "Bearer ") {
			tokenString = strings.TrimPrefix(authHeader, "Bearer ")

			// Get session from token
			var err error
			sessionData, err = m.manager.Get(tokenString)
			if err != nil {
				// Handle JWT errors with security logging
				m.handleJWTError(c, err, clientIP, userAgent)
				
				// If token is invalid or expired, create a new session
				if err == ErrJWTInvalid || err == ErrJWTExpired {
					sessionData, _ = m.manager.NewSession()
				}
			} else if sessionData != nil {
				// Perform enhanced security validation
				if !m.validateSessionSecurity(c, sessionData, clientIP, userAgent) {
					// Security validation failed, create new session
					sessionData, _ = m.manager.NewSession()
				}
			}
		}

		// If no valid token was found, create a new session
		if sessionData == nil {
			sessionData, _ = m.manager.NewSession()
		}

		// Store the session in the context
		c.Set(session.ContextSessionKey, sessionData)

		// Process the request
		c.Next()

		// After the request is processed, check if we need to issue a new token
		modifiedSession, exists := c.Get(session.ContextSessionKey)
		if exists {
			if s, ok := modifiedSession.(*session.SessionData); ok {
				shouldRotate := m.shouldRotateToken(s, tokenString)
				
				if shouldRotate {
					// Generate a new rotated token
					newToken, err := m.manager.Generate(s.ID, s)
					if err == nil {
						// Set the new token in the response header
						c.Header("Authorization", "Bearer "+newToken)
					}
				} else {
					// Just update session metadata
					_ = m.manager.Set(s.ID, s)
				}
			}
		}
	}
}

// validateSessionSecurity performs comprehensive security validation
func (m *EnhancedJWTSessionManager) validateSessionSecurity(c *gin.Context, sessionData *session.SessionData, clientIP, userAgent string) bool {
	config := m.config.SecurityConfig

	// Skip validation if security is disabled
	if !config.Enabled {
		return true
	}

	// Device fingerprinting validation
	if config.DeviceFingerprintingEnabled && m.fingerprintValidator != nil {
		if !m.fingerprintValidator.ValidateDeviceFingerprint(c, sessionData) {
			m.logSecurityEvent(c, SecurityEventDeviceFingerprintFailed, sessionData, "Device fingerprint validation failed")
			return false
		}
	}

	// IP validation
	if config.IPValidationEnabled {
		if !m.validateIPAddress(sessionData, clientIP) {
			m.logSecurityEvent(c, SecurityEventIPValidationFailed, sessionData, "IP address validation failed")
			return false
		}
	}

	// Nonce validation
	if config.NonceValidationEnabled {
		if !m.validateNonce(c, sessionData) {
			m.logSecurityEvent(c, SecurityEventNonceValidationFailed, sessionData, "Nonce validation failed")
			return false
		}
	}

	// Session limit validation
	if config.MaxConcurrentSessions > 0 {
		if !m.validateSessionLimit(sessionData) {
			m.logSecurityEvent(c, SecurityEventSessionLimitExceeded, sessionData, "Session limit exceeded")
			return false
		}
	}

	return true
}

// validateIPAddress validates IP address changes
func (m *EnhancedJWTSessionManager) validateIPAddress(sessionData *session.SessionData, currentIP string) bool {
	config := m.config.SecurityConfig
	
	// Get stored IP from session
	storedIP, exists := sessionData.Values["_ip_address"]
	if !exists {
		// First time, store the IP
		sessionData.Values["_ip_address"] = currentIP
		return true
	}

	storedIPStr, ok := storedIP.(string)
	if !ok {
		return false
	}

	// If IP hasn't changed, it's valid
	if storedIPStr == currentIP {
		return true
	}

	// If subnet changes are allowed, check if it's in the same subnet
	if config.AllowIPSubnetChange {
		return m.isSameSubnet(storedIPStr, currentIP)
	}

	// Strict IP validation - no changes allowed
	return false
}

// isSameSubnet checks if two IP addresses are in the same subnet
func (m *EnhancedJWTSessionManager) isSameSubnet(ip1, ip2 string) bool {
	// Simple implementation - calculate subnets for both IPs and compare
	subnet1 := m.calculateIPSubnet(ip1)
	subnet2 := m.calculateIPSubnet(ip2)
	return subnet1 != "" && subnet1 == subnet2
}

// calculateIPSubnet calculates the subnet for an IP address (same logic as fingerprint package)
func (m *EnhancedJWTSessionManager) calculateIPSubnet(ipAddress string) string {
	// Use the same logic as the fingerprint package
	// This is a simplified version - in production you'd import the function
	return ipAddress[:strings.LastIndex(ipAddress, ".")] + ".0/24"
}

// validateNonce validates nonce to prevent replay attacks
func (m *EnhancedJWTSessionManager) validateNonce(c *gin.Context, sessionData *session.SessionData) bool {
	nonce := c.GetHeader("X-Request-Nonce")
	if nonce == "" {
		return false // Nonce required
	}

	// Check if nonce was already used
	nonceKey := fmt.Sprintf("%s:%s", sessionData.ID, nonce)
	if m.config.SecurityStorage.NonceExists(nonceKey) {
		return false // Replay attack detected
	}

	// Store nonce to prevent replay
	_ = m.config.SecurityStorage.StoreNonce(nonceKey, m.config.SecurityConfig.NonceWindow)
	return true
}

// validateSessionLimit validates concurrent session limits
func (m *EnhancedJWTSessionManager) validateSessionLimit(sessionData *session.SessionData) bool {
	config := m.config.SecurityConfig
	if config.MaxConcurrentSessions <= 0 {
		return true // No limit
	}

	userID, exists := sessionData.Values["user_id"]
	if !exists {
		return true // No user associated yet
	}

	userIDStr := fmt.Sprintf("%v", userID)
	
	// Get current sessions for this user
	sessions := m.config.SecurityStorage.GetUserSessions(userIDStr)
	
	// Check if we're under the limit
	return len(sessions) < config.MaxConcurrentSessions
}

// handleJWTError handles JWT-related errors with security logging
func (m *EnhancedJWTSessionManager) handleJWTError(c *gin.Context, err error, clientIP, userAgent string) {
	if m.logger != nil {
		m.logger.Warn("JWT validation failed", map[string]interface{}{
			"error":      err.Error(),
			"client_ip":  clientIP,
			"user_agent": userAgent,
		})
	}
}

// logSecurityEvent logs security events
func (m *EnhancedJWTSessionManager) logSecurityEvent(c *gin.Context, eventType SecurityEventType, sessionData *session.SessionData, message string) {
	if m.logger != nil {
		m.logger.Warn("Security event", map[string]interface{}{
			"event":      string(eventType),
			"session_id": sessionData.ID,
			"client_ip":  c.ClientIP(),
			"user_agent": c.GetHeader("User-Agent"),
			"message":    message,
		})
	}
}

// startCleanup starts background cleanup processes
func (m *EnhancedJWTSessionManager) startCleanup() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.cleanupRunning {
		return
	}

	m.cleanupTicker = time.NewTicker(5 * time.Minute)
	m.cleanupRunning = true

	go func() {
		for {
			select {
			case <-m.cleanupTicker.C:
				m.performCleanup()
			case <-m.stopCleanup:
				m.cleanupTicker.Stop()
				return
			}
		}
	}()
}

// performCleanup performs background cleanup of security data
func (m *EnhancedJWTSessionManager) performCleanup() {
	if m.config.SecurityStorage != nil {
		_ = m.config.SecurityStorage.PruneExpired()
	}
}

// Stop stops the enhanced session manager and cleanup processes
func (m *EnhancedJWTSessionManager) Stop() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.cleanupRunning {
		m.stopCleanup <- true
		m.cleanupRunning = false
	}
}

// DeviceFingerprintValidator validates device fingerprints
type DeviceFingerprintValidator struct {
	requireBinding bool
	storage        storage.SecurityStorage
	logger         *log.Logger
}

// NewDeviceFingerprintValidator creates a new device fingerprint validator
func NewDeviceFingerprintValidator(requireBinding bool, storage storage.SecurityStorage, logger *log.Logger) *DeviceFingerprintValidator {
	return &DeviceFingerprintValidator{
		requireBinding: requireBinding,
		storage:        storage,
		logger:         logger,
	}
}

// ValidateDeviceFingerprint validates device fingerprint against stored data
func (v *DeviceFingerprintValidator) ValidateDeviceFingerprint(c *gin.Context, sessionData *session.SessionData) bool {
	// Generate current fingerprint using a generator
	generator := fingerprint.NewGenerator(fingerprint.NewDefaultConfig())
	fp := generator.Generate(c)
	
	// Get stored fingerprint from session
	storedFP, exists := sessionData.Values["_device_fingerprint"]
	if !exists {
		// First time, store the fingerprint
		sessionData.Values["_device_fingerprint"] = fp
		return true
	}

	// Compare fingerprints
	if storedFPMap, ok := storedFP.(map[string]interface{}); ok {
		storedFingerprint := &fingerprint.DeviceFingerprint{}
		// Convert map back to fingerprint struct (simplified)
		if userAgent, ok := storedFPMap["user_agent"].(string); ok {
			storedFingerprint.UserAgent = userAgent
		}
		
		// Use the generator's comparison method
		return generator.Compare(storedFingerprint, fp, v.requireBinding)
	}

	return !v.requireBinding // If we can't validate, only fail if binding is required
}