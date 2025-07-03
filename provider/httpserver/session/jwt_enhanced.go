package session

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/oddbit-project/blueprint/log"
	"github.com/oddbit-project/blueprint/provider/httpserver/fingerprint"
	"github.com/oddbit-project/blueprint/provider/httpserver/session/storage"
)

// EnhancedJWTConfig extends JWT configuration with security features
type EnhancedJWTConfig struct {
	*JWTConfig                                        // Embed existing JWT configuration
	SecurityConfig           *SecurityConfig          // Enhanced security configuration
	SecurityStorage          storage.SecurityStorage  // Storage for security data
	SessionSecurityValidator SessionSecurityValidator // Custom session security validator (optional)
}

// NewEnhancedJWTConfig creates a new enhanced JWT configuration
func NewEnhancedJWTConfig(jwtConfig *JWTConfig, securityConfig *SecurityConfig, securityStorage storage.SecurityStorage) *EnhancedJWTConfig {
	if jwtConfig == nil {
		jwtConfig = NewJWTConfig()
	}
	if securityConfig == nil {
		securityConfig = NewSecurityConfig()
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

	enhancedManager := &EnhancedJWTSessionManager{
		JWTSessionManager:        jwtSessionManager,
		config:                   config,
		logger:                   logger,
		fingerprintValidator:     NewDeviceFingerprintValidator(),
		sessionSecurityValidator: sessionSecurityValidator,
		mutex:                    sync.RWMutex{},
		stopCleanup:              make(chan bool),
	}

	// Start cleanup routine if security is enabled
	if config.SecurityConfig.Enabled {
		enhancedManager.startCleanupRoutine()
	}

	return enhancedManager, nil
}

// Middleware returns enhanced Gin middleware with security features
func (e *EnhancedJWTSessionManager) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// If security is disabled, use base middleware
		if !e.config.SecurityConfig.Enabled {
			e.JWTSessionManager.Middleware()(c)
			return
		}

		// Enhanced security middleware
		e.handleEnhancedSecurity(c)
	}
}

// handleEnhancedSecurity processes requests with enhanced security features
func (e *EnhancedJWTSessionManager) handleEnhancedSecurity(c *gin.Context) {
	var session *SessionData
	var tokenString string
	var err error

	// Generate current device fingerprint
	currentFingerprint := e.fingerprintValidator.GenerateFingerprint(c)

	// Check if device is blocked
	if e.config.SecurityConfig.DeviceFingerprintingEnabled {
		if e.config.SecurityStorage.IsDeviceBlocked(currentFingerprint.Fingerprint) {
			e.logSecurityEvent("blocked_device_access", c, map[string]interface{}{
				"fingerprint": currentFingerprint.Fingerprint,
				"ip":          currentFingerprint.IPAddress,
			})
			c.AbortWithStatusJSON(403, gin.H{"error": "Device is blocked"})
			return
		}
	}

	// Validate nonce if required
	if e.config.SecurityConfig.NonceValidationEnabled {
		if err := e.validateNonce(c); err != nil {
			e.logSecurityEvent("nonce_validation_failed", c, map[string]interface{}{
				"error": err.Error(),
			})
			// Continue processing for non-critical endpoints
		}
	}

	// Try to get the token from the Authorization header
	authHeader := c.GetHeader("Authorization")
	if authHeader != "" && strings.HasPrefix(authHeader, "Bearer ") {
		tokenString = strings.TrimPrefix(authHeader, "Bearer ")

		// Get session from token
		session, err = e.JWTSessionManager.manager.Get(tokenString)
		if err != nil {
			if err == ErrJWTInvalid || err == ErrJWTExpired {
				session, _ = e.JWTSessionManager.manager.NewSession()
			}
		} else {
			// Validate security context for existing session
			if e.config.SecurityConfig.Enabled {
				if err := e.validateSessionSecurity(c, session, currentFingerprint); err != nil {
					e.logSecurityEvent("session_security_validation_failed", c, map[string]interface{}{
						"session_id": session.ID,
						"error":      err.Error(),
					})

					// Handle suspicious activity
					e.handleSuspiciousActivity(c, session, currentFingerprint, err)
					return
				}
			}
		}
	}

	// If no valid token was found, create a new session
	if session == nil {
		session, _ = e.JWTSessionManager.manager.NewSession()
	}

	// Store the session in the context
	c.Set(ContextSessionKey, session)

	// Process the request
	c.Next()

	// After the request is processed, handle session updates
	e.postRequestProcessing(c, currentFingerprint)
}

// validateNonce validates request nonce for replay prevention
func (e *EnhancedJWTSessionManager) validateNonce(c *gin.Context) error {
	nonce := c.GetHeader("X-Request-Nonce")
	if nonce == "" {
		return fmt.Errorf("missing nonce header")
	}

	// Check if nonce was already used
	if e.config.SecurityStorage.NonceExists(nonce) {
		return fmt.Errorf("nonce already used")
	}

	// Store nonce with TTL
	if err := e.config.SecurityStorage.StoreNonce(nonce, e.config.SecurityConfig.NonceWindow); err != nil {
		return fmt.Errorf("failed to store nonce: %w", err)
	}

	return nil
}

// validateSessionSecurity validates security context for an existing session using pluggable validator
func (e *EnhancedJWTSessionManager) validateSessionSecurity(c *gin.Context, session *SessionData, currentFingerprint *fingerprint.DeviceFingerprint) error {
	// Use the pluggable session security validator
	return e.sessionSecurityValidator.ValidateSessionSecurity(c, session, currentFingerprint, e.config.SecurityConfig, e.config.SecurityStorage)
}

// handleSuspiciousActivity processes suspicious activity detection
func (e *EnhancedJWTSessionManager) handleSuspiciousActivity(c *gin.Context, session *SessionData, currentFingerprint *fingerprint.DeviceFingerprint, securityErr error) {
	// Get or create security context
	securityCtx, _ := e.config.SecurityStorage.GetSecurityContext(session.ID)
	if securityCtx == nil {
		securityCtx = &storage.SessionSecurityContext{
			DeviceFingerprint: currentFingerprint,
			FirstSeen:         time.Now().Unix(),
			LastActivity:      time.Now().Unix(),
			FailedAttempts:    0,
			SuspiciousFlags:   []string{},
		}
	}

	// Increment failed attempts
	securityCtx.FailedAttempts++

	// Detect suspicious activity flags
	if securityCtx.DeviceFingerprint != nil {
		flags := e.fingerprintValidator.DetectSuspiciousActivity(securityCtx.DeviceFingerprint, currentFingerprint)
		securityCtx.SuspiciousFlags = append(securityCtx.SuspiciousFlags, flags...)
	}

	// Log the suspicious activity
	e.logSecurityEvent("suspicious_activity", c, map[string]interface{}{
		"session_id":      session.ID,
		"failed_attempts": securityCtx.FailedAttempts,
		"flags":           securityCtx.SuspiciousFlags,
		"error":           securityErr.Error(),
	})

	// Check if we should block the device
	if securityCtx.FailedAttempts >= e.config.SecurityConfig.SuspiciousActivityThreshold {
		// Block the device
		blockUntil := time.Now().Add(e.config.SecurityConfig.BlockDuration)
		e.config.SecurityStorage.BlockDevice(currentFingerprint.Fingerprint, blockUntil)

		e.logSecurityEvent("device_blocked", c, map[string]interface{}{
			"fingerprint": currentFingerprint.Fingerprint,
			"block_until": blockUntil.Unix(),
		})

		// Regenerate session if configured
		if e.config.SecurityConfig.AutoSessionRegeneration {
			e.regenerateSessionSecurity(c, session)
		}

		c.AbortWithStatusJSON(403, gin.H{"error": "Device blocked due to suspicious activity"})
		return
	}

	// Store updated security context
	e.config.SecurityStorage.StoreSecurityContext(session.ID, securityCtx)

	// Return error for this request
	c.AbortWithStatusJSON(401, gin.H{"error": "Authentication failed"})
}

// postRequestProcessing handles session updates after request processing
func (e *EnhancedJWTSessionManager) postRequestProcessing(c *gin.Context, currentFingerprint *fingerprint.DeviceFingerprint) {
	// Check if we need to issue a new token
	modifiedSession, exists := c.Get(ContextSessionKey)
	if exists {
		if s, ok := modifiedSession.(*SessionData); ok {
			// Update security context if security is enabled
			if e.config.SecurityConfig.Enabled {
				e.updateSessionSecurity(s, currentFingerprint)
			}

			// Generate a new token using base manager
			err := e.JWTSessionManager.manager.Set(s.ID, s)
			if err == nil && s.Values["_jwt_token"] != nil {
				if token, ok := s.Values["_jwt_token"].(string); ok {
					c.Header("Authorization", "Bearer "+token)
				}
			}
		}
	}
}

// updateSessionSecurity updates security context for a session
func (e *EnhancedJWTSessionManager) updateSessionSecurity(session *SessionData, currentFingerprint *fingerprint.DeviceFingerprint) {
	if !e.config.SecurityConfig.DeviceFingerprintingEnabled {
		return
	}

	// Get or create security context
	securityCtx, _ := e.config.SecurityStorage.GetSecurityContext(session.ID)
	if securityCtx == nil {
		securityCtx = &storage.SessionSecurityContext{
			DeviceFingerprint: currentFingerprint,
			FirstSeen:         time.Now().Unix(),
			LastActivity:      time.Now().Unix(),
			FailedAttempts:    0,
			SuspiciousFlags:   []string{},
		}

		// Store device fingerprint
		e.config.SecurityStorage.StoreDeviceFingerprint(session.ID, currentFingerprint)

		// Track user session if we have user info
		if userID, exists := session.Values["user_id"]; exists {
			if uid, ok := userID.(string); ok && uid != "" {
				securityCtx.UserID = uid
				e.enforceSessionLimits(uid, session.ID)
			}
		}
	} else {
		securityCtx.LastActivity = time.Now().Unix()
	}

	// Store updated security context
	e.config.SecurityStorage.StoreSecurityContext(session.ID, securityCtx)
}

// enforceSessionLimits enforces concurrent session limits
func (e *EnhancedJWTSessionManager) enforceSessionLimits(userID, sessionID string) {
	if e.config.SecurityConfig.MaxConcurrentSessions <= 0 {
		return // No limits
	}

	// Track this session
	e.config.SecurityStorage.TrackUserSession(userID, sessionID)

	// Get user sessions
	sessions := e.config.SecurityStorage.GetUserSessions(userID)
	if len(sessions) <= e.config.SecurityConfig.MaxConcurrentSessions {
		return // Within limits
	}

	// Remove oldest sessions (FIFO)
	excessSessions := len(sessions) - e.config.SecurityConfig.MaxConcurrentSessions
	for i := 0; i < excessSessions; i++ {
		oldSessionID := sessions[i]
		e.config.SecurityStorage.RemoveUserSession(userID, oldSessionID)
		e.config.SecurityStorage.DeleteSecurityContext(oldSessionID)
		e.config.SecurityStorage.DeleteDeviceFingerprint(oldSessionID)

		e.logSecurityEvent("session_limit_eviction", nil, map[string]interface{}{
			"user_id":         userID,
			"evicted_session": oldSessionID,
			"limit":           e.config.SecurityConfig.MaxConcurrentSessions,
		})
	}
}

// regenerateSessionSecurity regenerates session with security updates
func (e *EnhancedJWTSessionManager) regenerateSessionSecurity(c *gin.Context, session *SessionData) {
	// Use base regeneration method
	e.JWTSessionManager.Regenerate(c)

	// Get the new session
	if newSession := Get(c); newSession != nil {
		// Clean up old security context
		e.config.SecurityStorage.DeleteSecurityContext(session.ID)
		e.config.SecurityStorage.DeleteDeviceFingerprint(session.ID)

		// Remove from user sessions
		if userID, exists := session.Values["user_id"]; exists {
			if uid, ok := userID.(string); ok {
				e.config.SecurityStorage.RemoveUserSession(uid, session.ID)
			}
		}

		e.logSecurityEvent("session_regenerated", c, map[string]interface{}{
			"old_session_id": session.ID,
			"new_session_id": newSession.ID,
		})
	}
}

// logSecurityEvent logs security-related events
func (e *EnhancedJWTSessionManager) logSecurityEvent(event string, c *gin.Context, details map[string]interface{}) {
	if e.logger == nil {
		return
	}

	logData := map[string]interface{}{
		"event":     event,
		"timestamp": time.Now().Unix(),
	}

	// Add request details if context available
	if c != nil {
		// Generate a fingerprint to get the real IP (temporary workaround)
		tempFP := e.fingerprintValidator.GenerateFingerprint(c)
		logData["ip"] = tempFP.IPAddress
		logData["user_agent"] = c.GetHeader("User-Agent")
		logData["path"] = c.Request.URL.Path
	}

	// Merge additional details
	for k, v := range details {
		logData[k] = v
	}

	e.logger.Info("Security event", logData)
}

// GetSecurityStats returns current security statistics
func (e *EnhancedJWTSessionManager) GetSecurityStats() map[string]interface{} {
	if !e.config.SecurityConfig.Enabled {
		return map[string]interface{}{
			"security_enabled": false,
		}
	}

	// Note: For a complete implementation, these would query the storage
	// This is a simplified version for demonstration
	return map[string]interface{}{
		"security_enabled": true,
		"features": map[string]bool{
			"device_fingerprinting": e.config.SecurityConfig.DeviceFingerprintingEnabled,
			"nonce_validation":      e.config.SecurityConfig.NonceValidationEnabled,
			"ip_validation":         e.config.SecurityConfig.IPValidationEnabled,
			"geolocation_check":     e.config.SecurityConfig.GeolocationValidation,
			"session_limiting":      e.config.SecurityConfig.MaxConcurrentSessions > 0,
		},
		"configuration": map[string]interface{}{
			"max_concurrent_sessions":       e.config.SecurityConfig.MaxConcurrentSessions,
			"nonce_window_minutes":          e.config.SecurityConfig.NonceWindow.Minutes(),
			"suspicious_activity_threshold": e.config.SecurityConfig.SuspiciousActivityThreshold,
			"block_duration_minutes":        e.config.SecurityConfig.BlockDuration.Minutes(),
		},
	}
}

// startCleanupRoutine starts the background cleanup routine
func (e *EnhancedJWTSessionManager) startCleanupRoutine() {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	if e.cleanupRunning {
		return
	}

	// Run cleanup every 15 minutes
	e.cleanupTicker = time.NewTicker(15 * time.Minute)
	e.cleanupRunning = true

	go func() {
		for {
			select {
			case <-e.cleanupTicker.C:
				if err := e.config.SecurityStorage.PruneExpired(); err != nil && e.logger != nil {
					e.logger.Error(err, "Failed to prune expired security data")
				} else if e.logger != nil {
					e.logger.Debug("Security data cleanup completed")
				}

			case <-e.stopCleanup:
				e.cleanupTicker.Stop()
				return
			}
		}
	}()
}

// stopCleanupRoutine stops the background cleanup routine
func (e *EnhancedJWTSessionManager) stopCleanupRoutine() {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	if !e.cleanupRunning {
		return
	}

	e.stopCleanup <- true
	e.cleanupRunning = false
}

// Close cleans up resources
func (e *EnhancedJWTSessionManager) Close() {
	e.stopCleanupRoutine()
	if e.config.SecurityStorage != nil {
		e.config.SecurityStorage.Close()
	}
}
