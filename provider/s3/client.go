package s3

import (
	"context"
	"crypto/tls"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/oddbit-project/blueprint/log"
	"net/http"
	"sync"
	"time"
)

type Client struct {
	config        *Config
	s3Client      *s3.Client
	uploader      *manager.Uploader
	downloader    *manager.Downloader
	timeout       time.Duration
	uploadTimeout time.Duration
	logger        *log.Logger
	connected     bool
	mu            sync.RWMutex
}

// NewClient creates a new S3 client
func NewClient(cfg *Config, logger *log.Logger) (*Client, error) {
	if cfg == nil {
		cfg = NewConfig()
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	if logger != nil {
		logger = logger.
			WithField("endpoint", cfg.Endpoint).
			WithField("region", cfg.Region)
	}

	// Create client instance
	c := &Client{
		config:        cfg,
		timeout:       time.Duration(cfg.TimeoutSeconds) * time.Second,
		uploadTimeout: time.Duration(cfg.UploadTimeoutSeconds) * time.Second,
		logger:        logger,
	}

	return c, nil
}

// Connect establishes connection to S3
func (c *Client) Connect(ctx context.Context) error {
	// Log connection attempt
	startTime := logOperationStart(c.logger, "connect", c.config.Endpoint, nil)

	c.mu.Lock()
	defer c.mu.Unlock()

	// Defer error logging in case of failure
	var connectionError error
	defer func() {
		if connectionError != nil {
			logOperationEnd(c.logger, "connect", c.config.Endpoint, startTime, connectionError, log.KV{
				"connection_failed": true,
			})
		}
	}()

	if c.connected {
		logOperationEnd(c.logger, "connect", c.config.Endpoint, startTime, nil, log.KV{
			"already_connected": true,
		})
		return nil
	}

	// Handle secure credentials - try to get secret key from Blueprint system
	var secretKey string
	if c.config.AccessKeyID != "" {
		var err error
		secretKey, err = c.config.DefaultCredentialConfig.Fetch()
		if err != nil {
			// Log the credential fetch error for debugging
			c.logger.Error(err, "Failed to fetch secret key from Blueprint credential system", log.KV{
				"env_var": c.config.DefaultCredentialConfig.PasswordEnvVar,
			})
			connectionError = err
			return err
		}

		// Clear secret key after use
		defer func() {
			for i := range secretKey {
				[]byte(secretKey)[i] = 0
			}
		}()
	}

	// Create AWS Config directly without default credential chain
	awsConfig := aws.Config{
		Region: c.config.Region,
	}

	// For MinIO compatibility, ensure we use the correct signing
	if c.config.IsCustomEndpoint() {
		awsConfig.RetryMaxAttempts = 1 // Minimize retries for faster feedback
	}

	// Set credentials if provided
	if c.config.AccessKeyID != "" && secretKey != "" {
		awsConfig.Credentials = credentials.NewStaticCredentialsProvider(c.config.AccessKeyID, secretKey, "")
	}

	// Add retry configuration
	if c.config.MaxRetries > 0 {
		awsConfig.RetryMaxAttempts = c.config.MaxRetries
	}
	if c.config.RetryMode != "" {
		awsConfig.RetryMode = aws.RetryMode(c.config.RetryMode)
	}

	// Configure custom HTTP client for MinIO/S3-compatible services
	if c.config.IsCustomEndpoint() && !c.config.UseSSL {
		// For HTTP endpoints, use custom HTTP client that forces HTTP
		// Create a custom transport that converts HTTPS requests to HTTP
		baseTransport := &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
			DisableKeepAlives: false,
		}

		httpClient := &http.Client{
			Transport: &httpForceTransport{
				base: baseTransport,
			},
		}
		awsConfig.HTTPClient = httpClient
	}

	// Create S3 client options
	s3Options := []func(*s3.Options){
		func(o *s3.Options) {
			o.UsePathStyle = c.config.ForcePathStyle
			o.UseAccelerate = c.config.UseAccelerate
		},
	}

	// Set S3-specific options for custom endpoints
	if c.config.IsCustomEndpoint() {
		endpointURL := c.config.GetEndpointURL()

		// Use BaseEndpoint approach for better MinIO compatibility
		awsConfig.BaseEndpoint = aws.String(endpointURL)

		s3Options = append(s3Options, func(o *s3.Options) {
			o.UsePathStyle = true // Force path style for S3-compatible services
		})
	}

	// Create S3 client
	c.s3Client = s3.NewFromConfig(awsConfig, s3Options...)

	// Create uploader with configuration
	c.uploader = manager.NewUploader(c.s3Client, func(u *manager.Uploader) {
		u.PartSize = c.config.PartSize
		u.Concurrency = c.config.Concurrency
	})

	// Create downloader with configuration
	c.downloader = manager.NewDownloader(c.s3Client, func(d *manager.Downloader) {
		d.PartSize = c.config.PartSize
		d.Concurrency = c.config.Concurrency
	})

	c.connected = true

	// Log successful connection
	logOperationEnd(c.logger, "connect", c.config.Endpoint, startTime, nil, log.KV{
		"connection_established": true,
	})

	return nil
}

// Close closes the S3 client connection
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.connected = false
	c.s3Client = nil
	c.uploader = nil
	c.downloader = nil

	return nil
}

// IsConnected returns true if the client is connected
func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

// ListBuckets lists all S3 buckets
func (c *Client) ListBuckets(ctx context.Context) ([]BucketInfo, error) {
	if !c.IsConnected() {
		return nil, ErrClientNotConnected
	}

	ctx, cancel := getContextWithTimeout(c.timeout, ctx)
	defer cancel()

	result, err := c.s3Client.ListBuckets(ctx, &s3.ListBucketsInput{})
	if err != nil {
		return nil, err
	}

	buckets := make([]BucketInfo, 0, len(result.Buckets))
	for _, b := range result.Buckets {
		info := BucketInfo{
			Name: aws.ToString(b.Name),
		}
		if b.CreationDate != nil {
			info.CreationDate = *b.CreationDate
		}
		buckets = append(buckets, info)
	}

	return buckets, nil
}

// Bucket create bucket object
func (c *Client) Bucket(bucketName string) (*Bucket, error) {
	return NewBucket(c, bucketName, c.logger)
}
