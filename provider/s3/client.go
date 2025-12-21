package s3

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/oddbit-project/blueprint/log"
)

type Client struct {
	config        *Config
	minioClient   *minio.Client
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

	// Create MinIO client options
	opts := &minio.Options{
		Secure: c.config.UseSSL,
		Region: c.config.Region,
	}

	// Set credentials if provided
	if c.config.AccessKeyID != "" && secretKey != "" {
		opts.Creds = credentials.NewStaticV4(c.config.AccessKeyID, secretKey, "")
	}

	// connection pooling
	transport := &http.Transport{
		MaxIdleConns:        c.config.MaxIdleConns,
		MaxIdleConnsPerHost: c.config.MaxIdleConnsPerHost,
		MaxConnsPerHost:     c.config.MaxConnsPerHost,
		IdleConnTimeout:     c.config.IdleConnTimeout,
	}

	// Configure custom HTTP client for non-SSL connections
	if c.config.UseSSL {
		tlsConfig, err := c.config.TLSConfig()
		if err != nil {
			return err
		}
		transport.TLSClientConfig = tlsConfig
	}
	opts.Transport = transport

	// Create MinIO client
	var err error
	c.minioClient, err = minio.New(c.config.Endpoint, opts)
	if err != nil {
		connectionError = err
		return err
	}

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
	c.minioClient = nil

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

	result, err := c.minioClient.ListBuckets(ctx)
	if err != nil {
		return nil, err
	}

	buckets := make([]BucketInfo, 0, len(result))
	for _, b := range result {
		info := BucketInfo{
			Name:         b.Name,
			CreationDate: b.CreationDate,
		}
		buckets = append(buckets, info)
	}

	return buckets, nil
}

// MinioClient returns the underlying MinIO client
func (c *Client) MinioClient() *minio.Client {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.minioClient
}

// CreateBucket creates a new bucket
func (c *Client) CreateBucket(ctx context.Context, bucket string, opts ...BucketOptions) error {
	b, err := c.Bucket(bucket)
	if err != nil {
		return err
	}
	return b.Create(ctx, opts...)
}

// DeleteBucket deletes a bucket
func (c *Client) DeleteBucket(ctx context.Context, bucket string) error {
	b, err := c.Bucket(bucket)
	if err != nil {
		return err
	}
	return b.Delete(ctx)
}

// BucketExists checks if a bucket exists
func (c *Client) BucketExists(ctx context.Context, bucket string) (bool, error) {
	b, err := c.Bucket(bucket)
	if err != nil {
		return false, err
	}
	return b.Exists(ctx)
}

// Bucket create bucket object
func (c *Client) Bucket(bucketName string) (*Bucket, error) {
	return NewBucket(c, bucketName)
}
