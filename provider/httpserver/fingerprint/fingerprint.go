package fingerprint

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// DeviceFingerprint represents a unique device identifier
type DeviceFingerprint struct {
	UserAgent   string `json:"user_agent"`
	AcceptLang  string `json:"accept_language"`
	AcceptEnc   string `json:"accept_encoding"`
	IPAddress   string `json:"ip_address"`
	IPSubnet    string `json:"ip_subnet"`
	Timezone    string `json:"timezone"`
	Fingerprint string `json:"fingerprint"` // SHA256 hash of components
	Location    string `json:"location"`
	CreatedAt   int64  `json:"created_at"`
}

// Config holds configuration for device fingerprinting
type Config struct {
	// IncludeUserAgent determines if User-Agent header should be included
	IncludeUserAgent bool `json:"includeUserAgent"`

	// IncludeAcceptHeaders determines if Accept-* headers should be included
	IncludeAcceptHeaders bool `json:"includeAcceptHeaders"`

	// IncludeTimezone determines if timezone should be included
	IncludeTimezone bool `json:"includeTimezone"`

	// IncludeIPAddress determines if IP address should be included
	IncludeIPAddress bool `json:"includeIPAddress"`

	// UseIPSubnet determines if IP subnet should be used instead of exact IP
	UseIPSubnet bool `json:"useIPSubnet"`

	// IncludeGeolocation determines if country detection should be included
	IncludeGeolocation bool `json:"includeGeolocation"`
}

// NewDefaultConfig creates a default fingerprinting configuration
func NewDefaultConfig() *Config {
	return &Config{
		IncludeUserAgent:     true,
		IncludeAcceptHeaders: true,
		IncludeTimezone:      true,
		IncludeIPAddress:     true,
		UseIPSubnet:          true,
		IncludeGeolocation:   false, // Disabled by default as it's basic implementation
	}
}

// NewStrictConfig creates a strict fingerprinting configuration
func NewStrictConfig() *Config {
	return &Config{
		IncludeUserAgent:     true,
		IncludeAcceptHeaders: true,
		IncludeTimezone:      true,
		IncludeIPAddress:     true,
		UseIPSubnet:          false, // Use exact IP for strict mode
		IncludeGeolocation:   true,
	}
}

// NewPrivacyFriendlyConfig creates a privacy-friendly configuration
func NewPrivacyFriendlyConfig() *Config {
	return &Config{
		IncludeUserAgent:     true,
		IncludeAcceptHeaders: false, // Less detailed headers
		IncludeTimezone:      false, // Don't include timezone
		IncludeIPAddress:     true,
		UseIPSubnet:          true,  // Use subnet for privacy
		IncludeGeolocation:   false, // No geolocation
	}
}

// GeoResolverFunc convert ip address in location info
type GeoResolverFunc func(string) string

// Generator generates device fingerprints from HTTP requests
type Generator struct {
	config      *Config
	geoResolver GeoResolverFunc
}

type GeneratorOption func(*Generator)

// WithGeoResolver specify custom function to resolve ipaddr -> location
func WithGeoResolver(geoResolver GeoResolverFunc) GeneratorOption {
	return func(g *Generator) {
		g.geoResolver = geoResolver
	}
}

// NewGenerator creates a new device fingerprint generator
func NewGenerator(config *Config, opts ...GeneratorOption) *Generator {
	if config == nil {
		config = NewDefaultConfig()
	}
	result := &Generator{
		config:      config,
		geoResolver: getCountryFromIP,
	}
	for _, opt := range opts {
		opt(result)
	}
	return result
}

// Generate creates a device fingerprint from a Gin context
func (g *Generator) Generate(c *gin.Context) *DeviceFingerprint {
	var components []string

	fingerprint := &DeviceFingerprint{
		CreatedAt: time.Now().Unix(),
	}

	// User-Agent
	if g.config.IncludeUserAgent {
		userAgent := c.GetHeader("User-Agent")
		fingerprint.UserAgent = userAgent
		components = append(components, "ua:"+userAgent)
	}

	// Accept headers
	if g.config.IncludeAcceptHeaders {
		acceptLang := c.GetHeader("Accept-Language")
		acceptEnc := c.GetHeader("Accept-Encoding")
		fingerprint.AcceptLang = acceptLang
		fingerprint.AcceptEnc = acceptEnc
		components = append(components, "al:"+acceptLang, "ae:"+acceptEnc)
	}

	// Timezone
	if g.config.IncludeTimezone {
		timezone := c.GetHeader("X-Timezone")
		if timezone == "" {
			timezone = "UTC" // Default if not provided
		}
		fingerprint.Timezone = timezone
		components = append(components, "tz:"+timezone)
	}

	// IP Address
	if g.config.IncludeIPAddress {
		ipAddress := c.ClientIP()
		fingerprint.IPAddress = ipAddress

		if g.config.UseIPSubnet {
			ipSubnet := calculateIPSubnet(ipAddress)
			fingerprint.IPSubnet = ipSubnet
			components = append(components, "subnet:"+ipSubnet)
		} else {
			components = append(components, "ip:"+ipAddress)
		}
	}

	// Geolocation
	if g.config.IncludeGeolocation {
		country := getCountryFromIP(fingerprint.IPAddress)
		fingerprint.Location = country
		components = append(components, "country:"+country)
	}

	// Generate fingerprint hash
	fingerprintData := strings.Join(components, "|")
	hash := sha256.Sum256([]byte(fingerprintData))
	fingerprint.Fingerprint = hex.EncodeToString(hash[:])

	return fingerprint
}

// Compare compares two device fingerprints with configurable strictness
func (g *Generator) Compare(stored, current *DeviceFingerprint, strict bool) bool {
	if stored == nil || current == nil {
		return false
	}

	// Always compare core fingerprint if using the same config
	if stored.Fingerprint == current.Fingerprint {
		return true
	}

	// In non-strict mode, allow some flexibility
	if !strict {
		// Allow IP subnet changes if configured
		if g.config.UseIPSubnet && g.config.IncludeIPAddress {
			if stored.IPSubnet != "" && current.IPSubnet != "" {
				return stored.IPSubnet == current.IPSubnet
			}
		}

		// Compare individual components for partial matches
		matches := 0
		total := 0

		if g.config.IncludeUserAgent {
			total++
			if stored.UserAgent == current.UserAgent {
				matches++
			}
		}

		if g.config.IncludeTimezone {
			total++
			if stored.Timezone == current.Timezone {
				matches++
			}
		}

		if g.config.IncludeGeolocation && stored.Location != "" && current.Location != "" {
			total++
			if stored.Location == current.Location {
				matches++
			}
		}

		// Require at least 70% match for non-strict comparison
		return total > 0 && float64(matches)/float64(total) >= 0.7
	}

	return false
}

// DetectChanges analyzes differences between fingerprints and returns change flags
func (g *Generator) DetectChanges(stored, current *DeviceFingerprint) []string {
	var changes []string

	if stored == nil || current == nil {
		return changes
	}

	// Check for User-Agent changes
	if g.config.IncludeUserAgent && stored.UserAgent != current.UserAgent {
		changes = append(changes, "user_agent_change")
	}

	// Check for Accept header changes
	if g.config.IncludeAcceptHeaders {
		if stored.AcceptLang != current.AcceptLang {
			changes = append(changes, "accept_language_change")
		}
		if stored.AcceptEnc != current.AcceptEnc {
			changes = append(changes, "accept_encoding_change")
		}
	}

	// Check for timezone changes
	if g.config.IncludeTimezone && stored.Timezone != current.Timezone {
		changes = append(changes, "timezone_change")
	}

	// Check for IP address changes
	if g.config.IncludeIPAddress {
		if stored.IPAddress != current.IPAddress {
			changes = append(changes, "ip_change")

			// Check for subnet changes
			if g.config.UseIPSubnet && stored.IPSubnet != current.IPSubnet {
				changes = append(changes, "ip_subnet_change")
			}
		}
	}

	// Check for country changes
	if g.config.IncludeGeolocation && stored.Location != current.Location {
		changes = append(changes, "country_change")
	}

	return changes
}

// GetConfig returns the current configuration
func (g *Generator) GetConfig() *Config {
	return g.config
}

// SetConfig updates the generator configuration
func (g *Generator) SetConfig(config *Config) {
	if config != nil {
		g.config = config
	}
}

// ValidateFingerprint validates a fingerprint structure
func ValidateFingerprint(fp *DeviceFingerprint) error {
	if fp == nil {
		return fmt.Errorf("fingerprint cannot be nil")
	}

	if fp.Fingerprint == "" {
		return fmt.Errorf("fingerprint hash cannot be empty")
	}

	if fp.CreatedAt <= 0 {
		return fmt.Errorf("fingerprint creation time must be positive")
	}

	// ParseToken fingerprint hash format (should be 64-character hex string for SHA256)
	if len(fp.Fingerprint) != 64 {
		return fmt.Errorf("fingerprint hash must be 64 characters for SHA256")
	}

	for _, char := range fp.Fingerprint {
		if !((char >= '0' && char <= '9') || (char >= 'a' && char <= 'f')) {
			return fmt.Errorf("fingerprint hash must be valid hexadecimal")
		}
	}

	return nil
}

// calculateIPSubnet calculates the subnet for an IP address
func calculateIPSubnet(ipAddress string) string {
	ip := net.ParseIP(ipAddress)
	if ip == nil {
		return ""
	}

	// Handle IPv4
	if ip.To4() != nil {
		// Calculate /24 subnet
		ipNet := &net.IPNet{
			IP:   ip.Mask(net.CIDRMask(24, 32)),
			Mask: net.CIDRMask(24, 32),
		}
		return ipNet.String()
	}

	// Handle IPv6 with /64 subnet
	ipNet := &net.IPNet{
		IP:   ip.Mask(net.CIDRMask(64, 128)),
		Mask: net.CIDRMask(64, 128),
	}
	return ipNet.String()
}

// getCountryFromIP performs basic country detection from IP address
func getCountryFromIP(ipAddress string) string {
	// Basic detection for private/local IPs
	if strings.HasPrefix(ipAddress, "192.168.") ||
		strings.HasPrefix(ipAddress, "10.") ||
		strings.HasPrefix(ipAddress, "172.16.") ||
		strings.HasPrefix(ipAddress, "172.17.") ||
		strings.HasPrefix(ipAddress, "172.18.") ||
		strings.HasPrefix(ipAddress, "172.19.") ||
		strings.HasPrefix(ipAddress, "172.20.") ||
		strings.HasPrefix(ipAddress, "172.21.") ||
		strings.HasPrefix(ipAddress, "172.22.") ||
		strings.HasPrefix(ipAddress, "172.23.") ||
		strings.HasPrefix(ipAddress, "172.24.") ||
		strings.HasPrefix(ipAddress, "172.25.") ||
		strings.HasPrefix(ipAddress, "172.26.") ||
		strings.HasPrefix(ipAddress, "172.27.") ||
		strings.HasPrefix(ipAddress, "172.28.") ||
		strings.HasPrefix(ipAddress, "172.29.") ||
		strings.HasPrefix(ipAddress, "172.30.") ||
		strings.HasPrefix(ipAddress, "172.31.") ||
		ipAddress == "127.0.0.1" ||
		ipAddress == "::1" {
		return "LOCAL"
	}

	// For the initial implementation, return a default
	// In production, this would integrate with a GeoIP service
	return "UNKNOWN"
}
