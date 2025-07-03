package fingerprint

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestNewDefaultConfig(t *testing.T) {
	config := NewDefaultConfig()
	
	if !config.IncludeUserAgent {
		t.Error("Expected IncludeUserAgent to be true")
	}
	if !config.IncludeAcceptHeaders {
		t.Error("Expected IncludeAcceptHeaders to be true")
	}
	if !config.IncludeTimezone {
		t.Error("Expected IncludeTimezone to be true")
	}
	if !config.IncludeIPAddress {
		t.Error("Expected IncludeIPAddress to be true")
	}
	if !config.UseIPSubnet {
		t.Error("Expected UseIPSubnet to be true")
	}
	if config.IncludeGeolocation {
		t.Error("Expected IncludeGeolocation to be false by default")
	}
}

func TestNewStrictConfig(t *testing.T) {
	config := NewStrictConfig()
	
	if !config.IncludeUserAgent {
		t.Error("Expected IncludeUserAgent to be true")
	}
	if !config.IncludeAcceptHeaders {
		t.Error("Expected IncludeAcceptHeaders to be true")
	}
	if !config.IncludeTimezone {
		t.Error("Expected IncludeTimezone to be true")
	}
	if !config.IncludeIPAddress {
		t.Error("Expected IncludeIPAddress to be true")
	}
	if config.UseIPSubnet {
		t.Error("Expected UseIPSubnet to be false for strict mode")
	}
	if !config.IncludeGeolocation {
		t.Error("Expected IncludeGeolocation to be true for strict mode")
	}
}

func TestNewPrivacyFriendlyConfig(t *testing.T) {
	config := NewPrivacyFriendlyConfig()
	
	if !config.IncludeUserAgent {
		t.Error("Expected IncludeUserAgent to be true")
	}
	if config.IncludeAcceptHeaders {
		t.Error("Expected IncludeAcceptHeaders to be false for privacy")
	}
	if config.IncludeTimezone {
		t.Error("Expected IncludeTimezone to be false for privacy")
	}
	if !config.IncludeIPAddress {
		t.Error("Expected IncludeIPAddress to be true")
	}
	if !config.UseIPSubnet {
		t.Error("Expected UseIPSubnet to be true for privacy")
	}
	if config.IncludeGeolocation {
		t.Error("Expected IncludeGeolocation to be false for privacy")
	}
}

func TestNewGenerator(t *testing.T) {
	config := NewDefaultConfig()
	generator := NewGenerator(config)
	
	if generator == nil {
		t.Fatal("Expected generator to be created")
	}
	if generator.config != config {
		t.Error("Expected generator to use provided config")
	}
	
	// Test with nil config (should use default)
	generator2 := NewGenerator(nil)
	if generator2 == nil {
		t.Fatal("Expected generator to be created with nil config")
	}
	if generator2.config == nil {
		t.Error("Expected generator to have default config when nil provided")
	}
}

func TestGeneratorGenerate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	// Create test gin context
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("User-Agent", "test-user-agent")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Accept-Encoding", "gzip, deflate")
	req.Header.Set("X-Timezone", "America/New_York")
	req.RemoteAddr = "192.168.1.100:12345"
	c.Request = req

	generator := NewGenerator(NewDefaultConfig())
	fingerprint := generator.Generate(c)
	
	// Test basic fields
	if fingerprint == nil {
		t.Fatal("Expected fingerprint to be generated")
	}
	if fingerprint.UserAgent != "test-user-agent" {
		t.Errorf("Expected UserAgent 'test-user-agent', got '%s'", fingerprint.UserAgent)
	}
	if fingerprint.AcceptLang != "en-US,en;q=0.9" {
		t.Errorf("Expected AcceptLang 'en-US,en;q=0.9', got '%s'", fingerprint.AcceptLang)
	}
	if fingerprint.AcceptEnc != "gzip, deflate" {
		t.Errorf("Expected AcceptEnc 'gzip, deflate', got '%s'", fingerprint.AcceptEnc)
	}
	if fingerprint.Timezone != "America/New_York" {
		t.Errorf("Expected Timezone 'America/New_York', got '%s'", fingerprint.Timezone)
	}
	if fingerprint.Fingerprint == "" {
		t.Error("Expected non-empty fingerprint hash")
	}
	if fingerprint.CreatedAt == 0 {
		t.Error("Expected CreatedAt to be set")
	}
	if fingerprint.IPSubnet == "" {
		t.Error("Expected IPSubnet to be calculated")
	}
}

func TestGeneratorGenerateWithDifferentConfigs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	// Create test gin context
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("User-Agent", "test-agent")
	req.Header.Set("Accept-Language", "en-US")
	req.Header.Set("X-Timezone", "UTC")
	c.Request = req

	// Test with privacy config (no accept headers, no timezone)
	privacyGenerator := NewGenerator(NewPrivacyFriendlyConfig())
	privacyFP := privacyGenerator.Generate(c)
	
	if privacyFP.AcceptLang != "" {
		t.Error("Expected AcceptLang to be empty with privacy config")
	}
	if privacyFP.AcceptEnc != "" {
		t.Error("Expected AcceptEnc to be empty with privacy config")
	}
	if privacyFP.Timezone != "" {
		t.Error("Expected Timezone to be empty with privacy config")
	}
	if privacyFP.UserAgent != "test-agent" {
		t.Error("Expected UserAgent to still be included with privacy config")
	}
}

func TestGeneratorCompare(t *testing.T) {
	generator := NewGenerator(NewDefaultConfig())
	
	// Create two identical fingerprints
	fp1 := &DeviceFingerprint{
		UserAgent:   "test-agent",
		AcceptLang:  "en-US",
		AcceptEnc:   "gzip",
		IPAddress:   "192.168.1.100",
		IPSubnet:    "192.168.1.0/24",
		Timezone:    "UTC",
		Fingerprint: "abc123",
		Country:     "US",
		CreatedAt:   time.Now().Unix(),
	}
	
	fp2 := &DeviceFingerprint{
		UserAgent:   "test-agent",
		AcceptLang:  "en-US",
		AcceptEnc:   "gzip",
		IPAddress:   "192.168.1.100",
		IPSubnet:    "192.168.1.0/24",
		Timezone:    "UTC",
		Fingerprint: "abc123",
		Country:     "US",
		CreatedAt:   time.Now().Unix(),
	}
	
	// Test identical fingerprints
	if !generator.Compare(fp1, fp2, true) {
		t.Error("Expected identical fingerprints to match in strict mode")
	}
	if !generator.Compare(fp1, fp2, false) {
		t.Error("Expected identical fingerprints to match in non-strict mode")
	}
	
	// Test with nil fingerprints
	if generator.Compare(nil, fp2, true) {
		t.Error("Expected nil fingerprint comparison to fail")
	}
	if generator.Compare(fp1, nil, false) {
		t.Error("Expected nil fingerprint comparison to fail")
	}
	
	// Test with different fingerprint hashes
	fp3 := *fp1
	fp3.Fingerprint = "different"
	if generator.Compare(fp1, &fp3, true) {
		t.Error("Expected different fingerprint hashes to not match in strict mode")
	}
	
	// Test subnet matching in non-strict mode
	fp4 := *fp1
	fp4.IPAddress = "192.168.1.101" // Different IP, same subnet
	fp4.Fingerprint = "different"
	
	// Should fail in strict mode
	if generator.Compare(fp1, &fp4, true) {
		t.Error("Expected different IPs to not match in strict mode")
	}
	
	// Should potentially pass in non-strict mode if enough components match
	// (This tests the partial matching logic with 70% threshold)
}

func TestGeneratorDetectChanges(t *testing.T) {
	// Use strict config to include geolocation for comprehensive change detection
	generator := NewGenerator(NewStrictConfig())
	
	fp1 := &DeviceFingerprint{
		UserAgent:  "test-agent",
		AcceptLang: "en-US",
		AcceptEnc:  "gzip",
		IPAddress:  "192.168.1.100",
		IPSubnet:   "192.168.1.0/24",
		Timezone:   "UTC",
		Country:    "US",
	}
	
	fp2 := &DeviceFingerprint{
		UserAgent:  "different-agent",    // Changed
		AcceptLang: "en-US",
		AcceptEnc:  "deflate",           // Changed
		IPAddress:  "192.168.2.100",     // Changed
		IPSubnet:   "192.168.2.0/24",    // Changed (but strict config has UseIPSubnet=false)
		Timezone:   "EST",               // Changed
		Country:    "CA",                // Changed
	}
	
	changes := generator.DetectChanges(fp1, fp2)
	
	// Note: strict config has UseIPSubnet=false, so ip_subnet_change won't be detected
	expectedChanges := []string{
		"user_agent_change",
		"accept_encoding_change",
		"timezone_change",
		"ip_change",
		"country_change",
	}
	
	if len(changes) != len(expectedChanges) {
		t.Errorf("Expected %d changes, got %d: %v", len(expectedChanges), len(changes), changes)
	}
	
	// Check that all expected changes are present
	changeMap := make(map[string]bool)
	for _, change := range changes {
		changeMap[change] = true
	}
	
	for _, expected := range expectedChanges {
		if !changeMap[expected] {
			t.Errorf("Expected change '%s' not found", expected)
		}
	}
	
	// Test with nil fingerprints
	nilChanges := generator.DetectChanges(nil, fp2)
	if len(nilChanges) != 0 {
		t.Error("Expected no changes with nil fingerprint")
	}
}

func TestGeneratorDetectChangesWithDefaultConfig(t *testing.T) {
	// Test with default config that includes IP subnet detection
	generator := NewGenerator(NewDefaultConfig())
	
	fp1 := &DeviceFingerprint{
		UserAgent:  "test-agent",
		AcceptLang: "en-US",
		AcceptEnc:  "gzip",
		IPAddress:  "192.168.1.100",
		IPSubnet:   "192.168.1.0/24",
		Timezone:   "UTC",
		Country:    "US",
	}
	
	fp2 := &DeviceFingerprint{
		UserAgent:  "different-agent",    // Changed
		AcceptLang: "en-US",
		AcceptEnc:  "deflate",           // Changed
		IPAddress:  "192.168.2.100",     // Changed
		IPSubnet:   "192.168.2.0/24",    // Changed (will be detected with default config)
		Timezone:   "EST",               // Changed
		Country:    "CA",                // Changed (but won't be detected as geolocation is disabled)
	}
	
	changes := generator.DetectChanges(fp1, fp2)
	
	// Default config has UseIPSubnet=true but IncludeGeolocation=false
	expectedChanges := []string{
		"user_agent_change",
		"accept_encoding_change",
		"timezone_change",
		"ip_change",
		"ip_subnet_change",
	}
	
	if len(changes) != len(expectedChanges) {
		t.Errorf("Expected %d changes, got %d: %v", len(expectedChanges), len(changes), changes)
	}
	
	// Check that all expected changes are present
	changeMap := make(map[string]bool)
	for _, change := range changes {
		changeMap[change] = true
	}
	
	for _, expected := range expectedChanges {
		if !changeMap[expected] {
			t.Errorf("Expected change '%s' not found", expected)
		}
	}
}

func TestGeneratorGetSetConfig(t *testing.T) {
	config1 := NewDefaultConfig()
	config2 := NewStrictConfig()
	
	generator := NewGenerator(config1)
	
	if generator.GetConfig() != config1 {
		t.Error("Expected GetConfig to return original config")
	}
	
	generator.SetConfig(config2)
	if generator.GetConfig() != config2 {
		t.Error("Expected GetConfig to return updated config")
	}
	
	// Test setting nil config (should be ignored)
	generator.SetConfig(nil)
	if generator.GetConfig() != config2 {
		t.Error("Expected nil config to be ignored")
	}
}

func TestValidateFingerprint(t *testing.T) {
	validFP := &DeviceFingerprint{
		Fingerprint: "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890", // 64 chars
		CreatedAt:   time.Now().Unix(),
	}
	
	// Test valid fingerprint
	if err := ValidateFingerprint(validFP); err != nil {
		t.Errorf("Expected valid fingerprint to pass validation: %v", err)
	}
	
	// Test nil fingerprint
	if err := ValidateFingerprint(nil); err == nil {
		t.Error("Expected nil fingerprint to fail validation")
	}
	
	// Test empty fingerprint hash
	invalidFP1 := &DeviceFingerprint{
		Fingerprint: "",
		CreatedAt:   time.Now().Unix(),
	}
	if err := ValidateFingerprint(invalidFP1); err == nil {
		t.Error("Expected empty fingerprint hash to fail validation")
	}
	
	// Test invalid creation time
	invalidFP2 := &DeviceFingerprint{
		Fingerprint: "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
		CreatedAt:   -1,
	}
	if err := ValidateFingerprint(invalidFP2); err == nil {
		t.Error("Expected negative CreatedAt to fail validation")
	}
	
	// Test invalid hash length
	invalidFP3 := &DeviceFingerprint{
		Fingerprint: "short",
		CreatedAt:   time.Now().Unix(),
	}
	if err := ValidateFingerprint(invalidFP3); err == nil {
		t.Error("Expected short fingerprint hash to fail validation")
	}
	
	// Test invalid hex characters
	invalidFP4 := &DeviceFingerprint{
		Fingerprint: "ghijkl1234567890abcdef1234567890abcdef1234567890abcdef1234567890", // Contains 'g', 'h', etc.
		CreatedAt:   time.Now().Unix(),
	}
	if err := ValidateFingerprint(invalidFP4); err == nil {
		t.Error("Expected invalid hex characters to fail validation")
	}
}

func TestCalculateIPSubnet(t *testing.T) {
	// Test IPv4
	ipv4Subnet := calculateIPSubnet("192.168.1.100")
	if ipv4Subnet != "192.168.1.0/24" {
		t.Errorf("Expected IPv4 subnet '192.168.1.0/24', got '%s'", ipv4Subnet)
	}
	
	// Test IPv6
	ipv6Subnet := calculateIPSubnet("2001:db8::1")
	if ipv6Subnet == "" {
		t.Error("Expected non-empty IPv6 subnet")
	}
	
	// Test invalid IP
	invalidSubnet := calculateIPSubnet("invalid-ip")
	if invalidSubnet != "" {
		t.Error("Expected empty subnet for invalid IP")
	}
}

func TestGetCountryFromIP(t *testing.T) {
	// Test local/private IPs
	localIPs := []string{
		"192.168.1.1",
		"10.0.0.1", 
		"172.16.0.1",
		"127.0.0.1",
		"::1",
	}
	
	for _, ip := range localIPs {
		country := getCountryFromIP(ip)
		if country != "LOCAL" {
			t.Errorf("Expected 'LOCAL' for IP %s, got '%s'", ip, country)
		}
	}
	
	// Test public IP (should return UNKNOWN in basic implementation)
	publicCountry := getCountryFromIP("8.8.8.8")
	if publicCountry != "UNKNOWN" {
		t.Errorf("Expected 'UNKNOWN' for public IP, got '%s'", publicCountry)
	}
}

func TestGetRealIP(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	tests := []struct {
		name     string
		headers  map[string]string
		remoteIP string
		expected string
	}{
		{
			name: "X-Forwarded-For header",
			headers: map[string]string{
				"X-Forwarded-For": "203.0.113.1, 198.51.100.1",
			},
			remoteIP: "192.168.1.1:8080",
			expected: "203.0.113.1",
		},
		{
			name: "X-Real-IP header",
			headers: map[string]string{
				"X-Real-IP": "203.0.113.2",
			},
			remoteIP: "192.168.1.1:8080",
			expected: "203.0.113.2",
		},
		{
			name: "X-Forwarded-IP header",
			headers: map[string]string{
				"X-Forwarded-IP": "203.0.113.3",
			},
			remoteIP: "192.168.1.1:8080",
			expected: "203.0.113.3",
		},
		{
			name:     "No proxy headers",
			headers:  map[string]string{},
			remoteIP: "192.168.1.1:8080",
			expected: "192.168.1.1", // Should fall back to ClientIP()
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			
			req, _ := http.NewRequest("GET", "/test", nil)
			for header, value := range tt.headers {
				req.Header.Set(header, value)
			}
			req.RemoteAddr = tt.remoteIP
			c.Request = req
			
			ip := getRealIP(c)
			if ip != tt.expected {
				t.Errorf("Expected IP '%s', got '%s'", tt.expected, ip)
			}
		})
	}
}