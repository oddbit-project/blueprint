package s3

import (
	"errors"
	"github.com/oddbit-project/blueprint/crypt/secure"
	"github.com/oddbit-project/blueprint/provider/tls"
)

// Config represents the S3 client configuration
type Config struct {
	// Connection settings
	Endpoint    string `json:"endpoint"`    // Custom endpoint for S3-compatible services
	Region      string `json:"region"`      // AWS region
	AccessKeyID string `json:"accessKeyId"` // AWS access key ID

	// Secret access key using secure credential handling
	secure.DefaultCredentialConfig

	// Optional bucket default
	DefaultBucket string `json:"defaultBucket"`

	// Behavior settings
	ForcePathStyle bool `json:"forcePathStyle"` // Force path-style addressing (for MinIO, etc.)
	UseAccelerate  bool `json:"useAccelerate"`  // Use S3 transfer acceleration
	UseSSL         bool `json:"useSSL"`         // Use SSL/TLS (default: true)

	// Timeout settings
	TimeoutSeconds       int `json:"timeoutSeconds"`       // Request timeout in seconds
	UploadTimeoutSeconds int `json:"uploadTimeoutSeconds"` // Upload timeout in seconds (for large files)

	// Multipart upload settings
	MultipartThreshold int64 `json:"multipartThreshold"` // Threshold for multipart uploads in bytes
	PartSize           int64 `json:"partSize"`           // Size of each part in multipart uploads
	MaxUploadParts     int   `json:"maxUploadParts"`     // Maximum number of parts in multipart upload
	Concurrency        int   `json:"concurrency"`        // Number of concurrent uploads

	// TLS configuration
	tls.ClientConfig

	// Retry settings
	MaxRetries int    `json:"maxRetries"` // Maximum number of retries
	RetryMode  string `json:"retryMode"`  // Retry mode: "standard" or "adaptive"
}

// NewConfig creates a new configuration with default values
func NewConfig() *Config {
	return &Config{
		Region:               DefaultRegion,
		UseSSL:               true,
		TimeoutSeconds:       int(DefaultTimeout.Seconds()),
		UploadTimeoutSeconds: int(DefaultUploadTimeout.Seconds()),
		MultipartThreshold:   DefaultMultipartThreshold,
		PartSize:             DefaultPartSize,
		MaxUploadParts:       DefaultMaxUploadParts,
		Concurrency:          5,
		MaxRetries:           DefaultMaxRetries,
		RetryMode:            "standard",
	}
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c == nil {
		return ErrNilConfig
	}

	// For AWS S3, endpoint is optional, but for S3-compatible services it's required
	if c.Endpoint == "" && c.Region == "" {
		return ErrMissingRegion
	}

	// Set default timeout if not specified
	if c.TimeoutSeconds == 0 {
		c.TimeoutSeconds = int(DefaultTimeout.Seconds())
	}
	if c.UploadTimeoutSeconds == 0 {
		c.UploadTimeoutSeconds = int(DefaultUploadTimeout.Seconds())
	}

	// Validate timeout (reject negative values)
	if c.TimeoutSeconds < 0 || c.TimeoutSeconds >= 3600 {
		return ErrInvalidTimeout
	}
	if c.UploadTimeoutSeconds < 0 {
		return ErrInvalidTimeout
	}

	// Validate retry settings
	if c.MaxRetries < 0 {
		return errors.New("invalid max retries: cannot be negative")
	}

	// Validate multipart settings
	if c.PartSize < MinPartSize || c.PartSize > MaxPartSize {
		return ErrInvalidPartSize
	}

	if c.MultipartThreshold < c.PartSize {
		return ErrInvalidThreshold
	}

	// Validate credentials if provided
	if c.AccessKeyID != "" && c.DefaultCredentialConfig.IsEmpty() {
		return errors.New("missing secret access key")
	}

	// Enforce SSL for AWS endpoints (security requirement)
	if !c.IsCustomEndpoint() && !c.UseSSL {
		return errors.New("SSL cannot be disabled for AWS endpoints")
	}

	return nil
}

// IsCustomEndpoint returns true if a custom endpoint is configured
func (c *Config) IsCustomEndpoint() bool {
	return c.Endpoint != ""
}

// GetEndpointURL returns the full endpoint URL with protocol
func (c *Config) GetEndpointURL() string {
	if c.Endpoint == "" {
		return ""
	}

	protocol := "https"
	if !c.UseSSL {
		protocol = "http"
	}

	// Check if endpoint already has protocol
	if len(c.Endpoint) > 7 && (c.Endpoint[:7] == "http://" || c.Endpoint[:8] == "https://") {
		return c.Endpoint
	}

	return protocol + "://" + c.Endpoint
}
