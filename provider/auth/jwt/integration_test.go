package jwt

import (
	"testing"
	"time"

	"github.com/oddbit-project/blueprint/log"
	"github.com/oddbit-project/blueprint/provider/httpserver/session"
)

// TestJWTBasicFlow tests the basic JWT functionality
func TestJWTBasicFlow(t *testing.T) {
	// Configure logger
	log.Configure(log.NewDefaultConfig())
	logger := log.New("jwt-test")

	// Create JWT config
	config := NewJWTConfig(RandomJWTKey())
	config.ExpirationSeconds = 60 // 1 minute

	// Create JWT manager
	manager, err := NewJWTManager(config, logger)
	if err != nil {
		t.Fatalf("Failed to create JWT manager: %v", err)
	}

	// Create session data
	sessionData := &session.SessionData{
		Values:       map[string]interface{}{"user_id": "test-user"},
		LastAccessed: time.Now(),
		Created:      time.Now(),
		ID:           "session-123",
	}

	// Generate token
	token, err := manager.Generate("session-123", sessionData)
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	if token == "" {
		t.Fatal("Generated token is empty")
	}

	// Validate token
	claims, err := manager.Validate(token)
	if err != nil {
		t.Fatalf("Failed to validate token: %v", err)
	}

	if claims.Subject != "session-123" {
		t.Errorf("Expected subject 'session-123', got '%s'", claims.Subject)
	}

	if userID, ok := claims.Data["user_id"]; !ok || userID != "test-user" {
		t.Errorf("Expected user_id 'test-user', got '%v'", userID)
	}

	t.Log("✅ Basic JWT flow test passed")
}

// TestJWTRevocation tests the token revocation functionality
func TestJWTRevocation(t *testing.T) {
	// Configure logger
	log.Configure(log.NewDefaultConfig())
	logger := log.New("jwt-revocation-test")

	// Create JWT config
	config := NewJWTConfig(RandomJWTKey())

	// Create JWT manager with revocation
	manager, err := NewJWTManagerWithRevocation(config, logger, nil)
	if err != nil {
		t.Fatalf("Failed to create JWT manager: %v", err)
	}

	// Create session data
	sessionData := &session.SessionData{
		Values:       map[string]interface{}{"user_id": "test-user"},
		LastAccessed: time.Now(),
		Created:      time.Now(),
		ID:           "session-123",
	}

	// Generate token
	token, err := manager.Generate("session-123", sessionData)
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	// Token should be valid initially
	_, err = manager.Validate(token)
	if err != nil {
		t.Fatalf("Token should be valid initially: %v", err)
	}

	// Revoke the token
	err = manager.RevokeToken(token)
	if err != nil {
		t.Fatalf("Failed to revoke token: %v", err)
	}

	// Token should now be invalid
	_, err = manager.Validate(token)
	if err == nil {
		t.Fatal("Token should be invalid after revocation")
	}

	if err != ErrTokenAlreadyRevoked {
		t.Errorf("Expected ErrTokenAlreadyRevoked, got %v", err)
	}

	t.Log("✅ Token revocation test passed")
}

// TestAsymmetricJWT tests asymmetric algorithm functionality
func TestAsymmetricJWT(t *testing.T) {
	// Configure logger
	log.Configure(log.NewDefaultConfig())
	logger := log.New("jwt-asymmetric-test")

	// Test RSA
	rsaConfig, err := NewJWTConfigWithRSA("RS256", 2048)
	if err != nil {
		t.Fatalf("Failed to create RSA config: %v", err)
	}

	rsaManager, err := NewJWTManager(rsaConfig, logger)
	if err != nil {
		t.Fatalf("Failed to create RSA JWT manager: %v", err)
	}

	// Generate and validate RSA token
	sessionData := &session.SessionData{
		Values: map[string]interface{}{"test": "rsa"},
		ID:     "rsa-session",
	}

	rsaToken, err := rsaManager.Generate("rsa-session", sessionData)
	if err != nil {
		t.Fatalf("Failed to generate RSA token: %v", err)
	}

	_, err = rsaManager.Validate(rsaToken)
	if err != nil {
		t.Fatalf("Failed to validate RSA token: %v", err)
	}

	// Test ECDSA
	ecdsaConfig, err := NewJWTConfigWithECDSA("ES256")
	if err != nil {
		t.Fatalf("Failed to create ECDSA config: %v", err)
	}

	ecdsaManager, err := NewJWTManager(ecdsaConfig, logger)
	if err != nil {
		t.Fatalf("Failed to create ECDSA JWT manager: %v", err)
	}

	ecdsaToken, err := ecdsaManager.Generate("ecdsa-session", sessionData)
	if err != nil {
		t.Fatalf("Failed to generate ECDSA token: %v", err)
	}

	_, err = ecdsaManager.Validate(ecdsaToken)
	if err != nil {
		t.Fatalf("Failed to validate ECDSA token: %v", err)
	}

	// Test EdDSA
	eddsaConfig, err := NewJWTConfigWithEd25519()
	if err != nil {
		t.Fatalf("Failed to create EdDSA config: %v", err)
	}

	eddsaManager, err := NewJWTManager(eddsaConfig, logger)
	if err != nil {
		t.Fatalf("Failed to create EdDSA JWT manager: %v", err)
	}

	eddsaToken, err := eddsaManager.Generate("eddsa-session", sessionData)
	if err != nil {
		t.Fatalf("Failed to generate EdDSA token: %v", err)
	}

	_, err = eddsaManager.Validate(eddsaToken)
	if err != nil {
		t.Fatalf("Failed to validate EdDSA token: %v", err)
	}

	t.Log("✅ Asymmetric JWT algorithms test passed")
}

// TestJWKSGeneration tests JWKS generation for different algorithms
func TestJWKSGeneration(t *testing.T) {
	// Configure logger
	log.Configure(log.NewDefaultConfig())
	logger := log.New("jwks-test")

	// Test RSA JWKS
	rsaConfig, err := NewJWTConfigWithRSA("RS256", 2048)
	if err != nil {
		t.Fatalf("Failed to create RSA config: %v", err)
	}
	rsaConfig.JWKSConfig = &JWKSConfig{Enabled: true}

	rsaManager, err := NewJWTManager(rsaConfig, logger)
	if err != nil {
		t.Fatalf("Failed to create RSA JWT manager: %v", err)
	}

	rsaJWKS, err := rsaManager.GenerateJWKS()
	if err != nil {
		t.Fatalf("Failed to generate RSA JWKS: %v", err)
	}

	if len(rsaJWKS.Keys) != 1 {
		t.Errorf("Expected 1 key in RSA JWKS, got %d", len(rsaJWKS.Keys))
	}

	if rsaJWKS.Keys[0].KeyType != "RSA" {
		t.Errorf("Expected RSA key type, got %s", rsaJWKS.Keys[0].KeyType)
	}

	// Test ECDSA JWKS
	ecdsaConfig, err := NewJWTConfigWithECDSA("ES256")
	if err != nil {
		t.Fatalf("Failed to create ECDSA config: %v", err)
	}
	ecdsaConfig.JWKSConfig = &JWKSConfig{Enabled: true}

	ecdsaManager, err := NewJWTManager(ecdsaConfig, logger)
	if err != nil {
		t.Fatalf("Failed to create ECDSA JWT manager: %v", err)
	}

	ecdsaJWKS, err := ecdsaManager.GenerateJWKS()
	if err != nil {
		t.Fatalf("Failed to generate ECDSA JWKS: %v", err)
	}

	if len(ecdsaJWKS.Keys) != 1 {
		t.Errorf("Expected 1 key in ECDSA JWKS, got %d", len(ecdsaJWKS.Keys))
	}

	if ecdsaJWKS.Keys[0].KeyType != "EC" {
		t.Errorf("Expected EC key type, got %s", ecdsaJWKS.Keys[0].KeyType)
	}

	t.Log("✅ JWKS generation test passed")
}

// TestSecurityConfig tests the security configuration functionality
func TestSecurityConfig(t *testing.T) {
	// Test default security config (enabled by default)
	defaultConfig := NewSecurityConfig()
	if !defaultConfig.Enabled {
		t.Error("Default security config should be enabled")
	}
	if !defaultConfig.DeviceFingerprintingEnabled {
		t.Error("Device fingerprinting should be enabled by default")
	}

	// Test disabled security config
	disabledConfig := NewDisabledSecurityConfig()
	if disabledConfig.Enabled {
		t.Error("Disabled security config should be disabled")
	}

	// Test feature controller
	customConfig := NewSecurityConfig().WithFeatureControl().
		DisableNonceValidation().
		DisableDeviceFingerprinting().
		Build()

	if customConfig.NonceValidationEnabled {
		t.Error("Nonce validation should be disabled")
	}
	if customConfig.DeviceFingerprintingEnabled {
		t.Error("Device fingerprinting should be disabled")
	}

	t.Log("✅ Security configuration test passed")
}