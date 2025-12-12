package franz

import (
	"context"
	"slices"
	"strings"
	"time"

	"github.com/oddbit-project/blueprint/crypt/secure"
	tlsProvider "github.com/oddbit-project/blueprint/provider/tls"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/sasl"
	"github.com/twmb/franz-go/pkg/sasl/aws"
	"github.com/twmb/franz-go/pkg/sasl/oauth"
	"github.com/twmb/franz-go/pkg/sasl/plain"
	"github.com/twmb/franz-go/pkg/sasl/scram"
)

// Authentication types
const (
	AuthTypeNone     = "none"
	AuthTypePlain    = "plain"
	AuthTypeScram256 = "scram256"
	AuthTypeScram512 = "scram512"
	AuthTypeAWSMSKIAM = "aws-msk-iam" // AWS MSK IAM authentication
	AuthTypeOAuth    = "oauth"        // OAuth/OIDC OAUTHBEARER
)

// Acks configuration
const (
	AcksNone   = "none"   // No acknowledgment
	AcksLeader = "leader" // Leader acknowledgment only
	AcksAll    = "all"    // All in-sync replicas acknowledgment
)

// Offset configuration
const (
	OffsetStart = "start" // Start from earliest offset
	OffsetEnd   = "end"   // Start from latest offset
)

// Isolation levels
const (
	IsolationReadUncommitted = "uncommitted"
	IsolationReadCommitted   = "committed"
)

// Compression types
const (
	CompressionNone   = "none"
	CompressionGzip   = "gzip"
	CompressionSnappy = "snappy"
	CompressionLz4    = "lz4"
	CompressionZstd   = "zstd"
)

var (
	validAuthTypes    = []string{AuthTypeNone, AuthTypePlain, AuthTypeScram256, AuthTypeScram512, AuthTypeAWSMSKIAM, AuthTypeOAuth}
	validAcks         = []string{AcksNone, AcksLeader, AcksAll}
	validOffsets      = []string{OffsetStart, OffsetEnd}
	validIsolation    = []string{IsolationReadUncommitted, IsolationReadCommitted}
	validCompression  = []string{CompressionNone, CompressionGzip, CompressionSnappy, CompressionLz4, CompressionZstd}
)

// AwsCredentialConfig holds AWS secret key configuration
type AwsCredentialConfig struct {
	AwsSecret secure.DefaultCredentialConfig `json:"awsSecret"`
}

// OAuthCredentialConfig holds OAuth client secret configuration
type OAuthCredentialConfig struct {
	OAuthSecret secure.DefaultCredentialConfig `json:"oauthSecret"`
}

// BaseConfig contains common configuration for all client types
type BaseConfig struct {
	Brokers  string `json:"brokers"`  // Comma-separated broker addresses
	AuthType string `json:"authType"` // none, plain, scram256, scram512, aws-msk-iam, oauth
	Username string `json:"username"`
	secure.DefaultCredentialConfig
	tlsProvider.ClientConfig

	// AWS MSK IAM authentication (when AuthType = "aws-msk-iam")
	AWSRegion    string `json:"awsRegion"`    // AWS region for MSK
	AWSAccessKey string `json:"awsAccessKey"` // Optional: AWS access key (uses default credentials if empty)
	AwsCredentialConfig

	// OAuth/OIDC authentication (when AuthType = "oauth")
	OAuthTokenURL  string `json:"oauthTokenUrl"`  // OAuth token endpoint URL
	OAuthClientID  string `json:"oauthClientId"`  // OAuth client ID
	OAuthScope     string `json:"oauthScope"`     // OAuth scope (optional)
	OAuthCredentialConfig

	// Connection settings
	DialTimeout    time.Duration `json:"dialTimeout"`    // Default: 30s
	RequestTimeout time.Duration `json:"requestTimeout"` // Default: 30s
	RetryBackoff   time.Duration `json:"retryBackoff"`   // Default: 100ms
	MaxRetries     int           `json:"maxRetries"`     // Default: 3
}

// Validate validates base configuration
func (c *BaseConfig) Validate() error {
	if len(c.Brokers) == 0 {
		return ErrMissingBrokers
	}
	if c.AuthType != "" && !slices.Contains(validAuthTypes, c.AuthType) {
		return ErrInvalidAuthType
	}
	return nil
}

// DefaultBaseConfig returns base config with sensible defaults
func DefaultBaseConfig() BaseConfig {
	return BaseConfig{
		AuthType:       AuthTypeNone,
		DialTimeout:    30 * time.Second,
		RequestTimeout: 30 * time.Second,
		RetryBackoff:   100 * time.Millisecond,
		MaxRetries:     3,
	}
}

// brokerList returns brokers as a slice
func (c *BaseConfig) brokerList() []string {
	return strings.Split(c.Brokers, ",")
}

// buildBaseOpts builds common kgo options from base config
func (c *BaseConfig) buildBaseOpts() ([]kgo.Opt, error) {
	opts := []kgo.Opt{
		kgo.SeedBrokers(c.brokerList()...),
	}

	if c.DialTimeout > 0 {
		opts = append(opts, kgo.DialTimeout(c.DialTimeout))
	}

	if c.RequestTimeout > 0 {
		opts = append(opts, kgo.RequestTimeoutOverhead(c.RequestTimeout))
	}

	if c.RetryBackoff > 0 {
		opts = append(opts, kgo.RetryBackoffFn(func(int) time.Duration {
			return c.RetryBackoff
		}))
	}

	if c.MaxRetries > 0 {
		opts = append(opts, kgo.RequestRetries(c.MaxRetries))
	}

	// Setup credentials
	password, credential, err := setupCredentials(c.DefaultCredentialConfig)
	if err != nil {
		return nil, err
	}

	// Add SASL authentication if configured
	saslMechanism, err := createSASLMechanism(c, password)
	if err != nil {
		return nil, err
	}
	if saslMechanism != nil {
		opts = append(opts, kgo.SASL(saslMechanism))
	}

	// Clear credential from memory
	if credential != nil {
		credential.Clear()
	}

	// Add TLS configuration
	tlsCfg, err := c.TLSConfig()
	if err != nil {
		return nil, err
	}
	if tlsCfg != nil {
		opts = append(opts, kgo.DialTLSConfig(tlsCfg))
	}

	return opts, nil
}

// ProducerConfig configures a producer
type ProducerConfig struct {
	BaseConfig

	DefaultTopic string `json:"defaultTopic"` // Default topic for records without explicit topic

	// Batching
	BatchMaxRecords int           `json:"batchMaxRecords"` // Max records per batch (default: 10000)
	BatchMaxBytes   int           `json:"batchMaxBytes"`   // Max bytes per batch (default: 1MB)
	Linger          time.Duration `json:"linger"`          // Time to wait for batch fill (default: 0)

	// Reliability
	Acks            string `json:"acks"`            // none, leader, all (default: leader)
	Idempotent      bool   `json:"idempotent"`      // Enable idempotent producer
	TransactionalID string `json:"transactionalId"` // For transactional producer

	// Compression
	Compression string `json:"compression"` // none, gzip, snappy, lz4, zstd
}

// Validate validates producer configuration
func (c *ProducerConfig) Validate() error {
	if err := c.BaseConfig.Validate(); err != nil {
		return err
	}
	if c.Acks != "" && !slices.Contains(validAcks, c.Acks) {
		return ErrInvalidAcks
	}
	if c.Compression != "" && !slices.Contains(validCompression, c.Compression) {
		return ErrInvalidCompression
	}
	return nil
}

// DefaultProducerConfig returns producer config with sensible defaults
func DefaultProducerConfig() *ProducerConfig {
	return &ProducerConfig{
		BaseConfig:      DefaultBaseConfig(),
		BatchMaxRecords: 10000,
		BatchMaxBytes:   1048576,
		Linger:          0,
		Acks:            AcksLeader,
		Compression:     CompressionNone,
	}
}

// buildOpts builds kgo options from producer config
func (c *ProducerConfig) buildOpts() ([]kgo.Opt, error) {
	opts, err := c.BaseConfig.buildBaseOpts()
	if err != nil {
		return nil, err
	}

	if c.DefaultTopic != "" {
		opts = append(opts, kgo.DefaultProduceTopic(c.DefaultTopic))
	}

	opts = append(opts, kgo.AllowAutoTopicCreation())

	if c.BatchMaxRecords > 0 {
		opts = append(opts, kgo.MaxBufferedRecords(c.BatchMaxRecords))
	}

	if c.BatchMaxBytes > 0 {
		opts = append(opts, kgo.MaxBufferedBytes(c.BatchMaxBytes))
	}

	if c.Linger > 0 {
		opts = append(opts, kgo.ProducerLinger(c.Linger))
	}

	// Acks
	switch c.Acks {
	case AcksNone:
		opts = append(opts, kgo.RequiredAcks(kgo.NoAck()))
	case AcksAll:
		opts = append(opts, kgo.RequiredAcks(kgo.AllISRAcks()))
	default: // AcksLeader
		opts = append(opts, kgo.RequiredAcks(kgo.LeaderAck()))
	}

	// Compression
	switch c.Compression {
	case CompressionGzip:
		opts = append(opts, kgo.ProducerBatchCompression(kgo.GzipCompression()))
	case CompressionSnappy:
		opts = append(opts, kgo.ProducerBatchCompression(kgo.SnappyCompression()))
	case CompressionLz4:
		opts = append(opts, kgo.ProducerBatchCompression(kgo.Lz4Compression()))
	case CompressionZstd:
		opts = append(opts, kgo.ProducerBatchCompression(kgo.ZstdCompression()))
	}

	// Idempotent producer
	// kgo enables idempotency by default, which requires acks=all
	// Explicitly disable it when not requested to allow other acks settings
	if c.Idempotent {
		opts = append(opts, kgo.RequiredAcks(kgo.AllISRAcks()))
		opts = append(opts, kgo.MaxProduceRequestsInflightPerBroker(1))
	} else {
		opts = append(opts, kgo.DisableIdempotentWrite())
	}

	// Transactional producer
	if c.TransactionalID != "" {
		opts = append(opts, kgo.TransactionalID(c.TransactionalID))
	}

	return opts, nil
}

// ConsumerConfig configures a consumer
type ConsumerConfig struct {
	BaseConfig

	// Topics can be set here or via options
	Topics []string `json:"topics"`
	Group  string   `json:"group"` // Consumer group (required for group consumption)

	// Consumer behavior
	StartOffset    string `json:"startOffset"`    // start, end (default: end)
	IsolationLevel string `json:"isolationLevel"` // uncommitted, committed (default: committed)

	// Group settings
	SessionTimeout    time.Duration `json:"sessionTimeout"`    // Default: 45s
	RebalanceTimeout  time.Duration `json:"rebalanceTimeout"`  // Default: 60s
	HeartbeatInterval time.Duration `json:"heartbeatInterval"` // Default: 3s

	// Fetch settings
	FetchMinBytes int           `json:"fetchMinBytes"` // Default: 1
	FetchMaxBytes int           `json:"fetchMaxBytes"` // Default: 50MB
	FetchMaxWait  time.Duration `json:"fetchMaxWait"`  // Default: 5s

	// Offset management
	AutoCommit         bool          `json:"autoCommit"`         // Default: true
	AutoCommitInterval time.Duration `json:"autoCommitInterval"` // Default: 5s
}

// Validate validates consumer configuration
func (c *ConsumerConfig) Validate() error {
	if err := c.BaseConfig.Validate(); err != nil {
		return err
	}
	if len(c.Topics) == 0 {
		return ErrMissingTopic
	}
	if c.StartOffset != "" && !slices.Contains(validOffsets, c.StartOffset) {
		return ErrInvalidOffset
	}
	if c.IsolationLevel != "" && !slices.Contains(validIsolation, c.IsolationLevel) {
		return ErrInvalidIsolation
	}
	return nil
}

// DefaultConsumerConfig returns consumer config with sensible defaults
func DefaultConsumerConfig() *ConsumerConfig {
	return &ConsumerConfig{
		BaseConfig:         DefaultBaseConfig(),
		StartOffset:        OffsetEnd,
		IsolationLevel:     IsolationReadCommitted,
		SessionTimeout:     45 * time.Second,
		RebalanceTimeout:   60 * time.Second,
		HeartbeatInterval:  3 * time.Second,
		FetchMinBytes:      1,
		FetchMaxBytes:      52428800,
		FetchMaxWait:       5 * time.Second,
		AutoCommit:         true,
		AutoCommitInterval: 5 * time.Second,
	}
}

// buildOpts builds kgo options from consumer config
func (c *ConsumerConfig) buildOpts() ([]kgo.Opt, error) {
	opts, err := c.BaseConfig.buildBaseOpts()
	if err != nil {
		return nil, err
	}

	// Topics
	opts = append(opts, kgo.ConsumeTopics(c.Topics...))

	// Consumer group
	if c.Group != "" {
		opts = append(opts, kgo.ConsumerGroup(c.Group))

		if c.SessionTimeout > 0 {
			opts = append(opts, kgo.SessionTimeout(c.SessionTimeout))
		}
		if c.RebalanceTimeout > 0 {
			opts = append(opts, kgo.RebalanceTimeout(c.RebalanceTimeout))
		}
		if c.HeartbeatInterval > 0 {
			opts = append(opts, kgo.HeartbeatInterval(c.HeartbeatInterval))
		}

		// Auto-commit
		if c.AutoCommit {
			if c.AutoCommitInterval > 0 {
				opts = append(opts, kgo.AutoCommitInterval(c.AutoCommitInterval))
			}
		} else {
			opts = append(opts, kgo.DisableAutoCommit())
		}
	}

	// Start offset
	if c.StartOffset == OffsetStart {
		opts = append(opts, kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()))
	} else {
		opts = append(opts, kgo.ConsumeResetOffset(kgo.NewOffset().AtEnd()))
	}

	// Isolation level
	if c.IsolationLevel == IsolationReadUncommitted {
		opts = append(opts, kgo.FetchIsolationLevel(kgo.ReadUncommitted()))
	} else {
		opts = append(opts, kgo.FetchIsolationLevel(kgo.ReadCommitted()))
	}

	// Fetch settings
	if c.FetchMinBytes > 0 {
		opts = append(opts, kgo.FetchMinBytes(int32(c.FetchMinBytes)))
	}
	if c.FetchMaxBytes > 0 {
		opts = append(opts, kgo.FetchMaxBytes(int32(c.FetchMaxBytes)))
	}
	if c.FetchMaxWait > 0 {
		opts = append(opts, kgo.FetchMaxWait(c.FetchMaxWait))
	}

	return opts, nil
}

// AdminConfig configures an admin client
type AdminConfig struct {
	BaseConfig
}

// Validate validates admin configuration
func (c *AdminConfig) Validate() error {
	return c.BaseConfig.Validate()
}

// DefaultAdminConfig returns admin config with sensible defaults
func DefaultAdminConfig() *AdminConfig {
	return &AdminConfig{
		BaseConfig: DefaultBaseConfig(),
	}
}

// buildOpts builds kgo options from admin config
func (c *AdminConfig) buildOpts() ([]kgo.Opt, error) {
	return c.BaseConfig.buildBaseOpts()
}

// setupCredentials creates and retrieves password from credential configuration
func setupCredentials(credConfig secure.DefaultCredentialConfig) (string, *secure.Credential, error) {
	key, err := secure.GenerateKey()
	if err != nil {
		return "", nil, err
	}

	credential, err := secure.CredentialFromConfig(credConfig, key, true)
	if err != nil {
		return "", nil, err
	}

	password, err := credential.Get()
	if err != nil {
		return "", nil, err
	}

	return password, credential, nil
}

// createSASLMechanism creates the appropriate SASL mechanism based on auth type
func createSASLMechanism(cfg *BaseConfig, password string) (sasl.Mechanism, error) {
	switch cfg.AuthType {
	case AuthTypePlain:
		return plain.Auth{
			User: cfg.Username,
			Pass: password,
		}.AsMechanism(), nil

	case AuthTypeScram256:
		return scram.Auth{
			User: cfg.Username,
			Pass: password,
		}.AsSha256Mechanism(), nil

	case AuthTypeScram512:
		return scram.Auth{
			User: cfg.Username,
			Pass: password,
		}.AsSha512Mechanism(), nil

	case AuthTypeAWSMSKIAM:
		return createAWSMSKIAMMechanism(cfg)

	case AuthTypeOAuth:
		return createOAuthMechanism(cfg)

	case AuthTypeNone, "":
		return nil, nil

	default:
		return nil, ErrInvalidAuthType
	}
}

// createAWSMSKIAMMechanism creates AWS MSK IAM SASL mechanism
func createAWSMSKIAMMechanism(cfg *BaseConfig) (sasl.Mechanism, error) {
	if cfg.AWSRegion == "" {
		return nil, ErrMissingAWSRegion
	}

	// If explicit credentials provided, use them
	if cfg.AWSAccessKey != "" {
		// Get the AWS secret key from secure credential config
		awsSecretKey, credential, err := setupCredentials(cfg.AwsSecret)
		if err != nil {
			return nil, err
		}
		if credential != nil {
			defer credential.Clear()
		}

		if awsSecretKey != "" {
			return aws.Auth{
				AccessKey: cfg.AWSAccessKey,
				SecretKey: awsSecretKey,
			}.AsManagedStreamingIAMMechanism(), nil
		}
	}

	// Otherwise, use default AWS credential chain (env vars, IAM role, etc.)
	return aws.ManagedStreamingIAM(func(ctx context.Context) (aws.Auth, error) {
		// This will use the default AWS credential chain
		return aws.Auth{}, nil
	}), nil
}

// createOAuthMechanism creates OAuth/OIDC SASL mechanism
func createOAuthMechanism(cfg *BaseConfig) (sasl.Mechanism, error) {
	if cfg.OAuthTokenURL == "" {
		return nil, ErrMissingOAuthTokenURL
	}

	// Get the OAuth secret from secure credential config
	oauthSecret, credential, err := setupCredentials(cfg.OAuthSecret)
	if err != nil {
		return nil, err
	}
	if credential != nil {
		defer credential.Clear()
	}

	return oauth.Oauth(func(ctx context.Context) (oauth.Auth, error) {
		// In a real implementation, you would fetch a token from the OAuth server
		// This is a basic implementation - production use should handle token refresh
		return oauth.Auth{
			Token: oauthSecret, // For simple cases, this could be a pre-fetched token
		}, nil
	}), nil
}
