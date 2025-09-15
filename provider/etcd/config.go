package etcd

import (
	"errors"
	"github.com/oddbit-project/blueprint/crypt/secure"
	tlsProvider "github.com/oddbit-project/blueprint/provider/tls"
)

// Config holds the configuration for connecting to etcd.
// It includes connection settings, authentication, TLS, encryption, and timeout options.
type Config struct {
	// Endpoints is the list of etcd server URLs to connect to
	Endpoints []string `json:"endpoints"`

	// Username for etcd authentication (optional)
	Username string `json:"username"`
	secure.DefaultCredentialConfig
	tlsProvider.ClientConfig

	// DialTimeout is the timeout, in seconds, for connecting to etcd
	DialTimeout int `json:"dialTimeout"`
	// DialKeepAliveTime is the time interval, in seconds, for keep-alive pings
	DialKeepAliveTime int `json:"dialKeepAliveTime"`
	// DialKeepAliveTimeout is the timeout, in seconds, for keep-alive pings
	DialKeepAliveTimeout int `json:"dialKeepAliveTimeout"`

	// RequestTimeout is the timeout, in seconds, for individual etcd requests
	RequestTimeout int `json:"requestTimeout"`

	// EnableEncryption enables client-side encryption of values
	EnableEncryption bool `json:"enableEncryption"`
	// EncryptionKey is the key used for client-side encryption (required if EnableEncryption is true)
	EncryptionKey []byte `json:"encryptionKey"`

	// MaxCallSendMsgSize is the maximum size of messages sent to etcd
	MaxCallSendMsgSize int `json:"maxCallSendMsgSize"`
	// MaxCallRecvMsgSize is the maximum size of messages received from etcd
	MaxCallRecvMsgSize int `json:"maxCallRecvMsgSize"`

	// PermitWithoutStream allows RPCs to be sent without streams
	PermitWithoutStream bool `json:"permitWithoutStream"`
	// RejectOldCluster rejects connections to etcd clusters with old versions
	RejectOldCluster bool `json:"rejectOldCluster"`
}

// DefaultConfig returns a Config with sensible default values.
// Uses localhost:2379 as the default endpoint with 5-second timeouts.
func DefaultConfig() *Config {
	return &Config{
		Endpoints:            []string{"localhost:2379"},
		DialTimeout:          5,
		DialKeepAliveTime:    30,
		DialKeepAliveTimeout: 10,
		RequestTimeout:       5,
		EnableEncryption:     false,
		MaxCallSendMsgSize:   2 * 1024 * 1024,
		MaxCallRecvMsgSize:   2 * 1024 * 1024,
		PermitWithoutStream:  false,
		RejectOldCluster:     false,
	}
}

// WithEndpoints sets the etcd server endpoints and returns the config for chaining.
func (c *Config) WithEndpoints(endpoints ...string) *Config {
	c.Endpoints = endpoints
	return c
}

// WithAuth configures username/password authentication and returns the config for chaining.
func (c *Config) WithAuth(username, password string) *Config {
	c.Username = username
	c.Password = password
	return c
}

// WithTLS configures TLS settings for secure connections and returns the config for chaining.
// Set allowInsecure to true to skip certificate verification (not recommended for production).
func (c *Config) WithTLS(certFile, keyFile, caFile string, allowInsecure bool) *Config {
	c.TLSEnable = true
	c.TLSCert = certFile
	c.TLSKey = keyFile
	c.TLSCA = caFile
	c.TLSInsecureSkipVerify = allowInsecure
	return c
}

// WithEncryption enables client-side encryption with the provided key and returns the config for chaining.
// The key should be 32 bytes for AES-256-GCM encryption.
func (c *Config) WithEncryption(key []byte) *Config {
	c.EnableEncryption = true
	c.EncryptionKey = key
	return c
}

// WithTimeout sets the request timeout and returns the config for chaining.
func (c *Config) WithTimeout(timeoutSeconds int) *Config {
	if timeoutSeconds > -1 {
		c.RequestTimeout = timeoutSeconds
	}
	return c
}

// WithDialTimeout sets the connection timeout and returns the config for chaining.
func (c *Config) WithDialTimeout(timeoutSeconds int) *Config {
	if timeoutSeconds > -1 {
		c.DialTimeout = timeoutSeconds
	}
	return c
}

// Validate checks if the configuration is valid and returns an error if not.
func (c *Config) Validate() error {
	if len(c.Endpoints) == 0 {
		return errors.New("no etcd endpoints provided")
	}
	return nil
}

// NewClient creates and returns a new etcd client using this configuration.
// This is a convenience method equivalent to calling NewClient(config).
func (c *Config) NewClient() (*Client, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return NewClient(c)
}
