package redis

import (
	"context"
	"errors"
	"github.com/oddbit-project/blueprint/crypt/secure"
	"github.com/oddbit-project/blueprint/provider/tls"
	"github.com/oddbit-project/blueprint/utils"
	"github.com/redis/go-redis/v9"
	"time"
)

const (
	ErrMissingAddress = utils.Error("Missing address")
)

// Config
type Config struct {
	Address        string `json:"address"`        // Address of the Client server
	DB             int    `json:"db"`             // DB is the Client database to use
	KeyPrefix      string `json:"keyPrefix"`      // KeyPrefix is the prefix for session keys in Client
	TTL            uint   `json:"ttl"`            // TTl in seconds
	TimeoutSeconds uint   `json:"timeoutSeconds"` // TimeoutSeconds seconds to wait for operation
	secure.DefaultCredentialConfig
	tls.ServerConfig
}

type Client struct {
	Client  *redis.Client
	config  *Config
	timeout time.Duration
	ttl     time.Duration
}

// NewConfig returns a default Client configuration
func NewConfig() *Config {
	return &Config{
		Address: "localhost:6379",
		DefaultCredentialConfig: secure.DefaultCredentialConfig{
			Password:       "",
			PasswordEnvVar: "",
			PasswordFile:   "",
		},
		ServerConfig: tls.ServerConfig{
			TLSCert:            "",
			TLSKey:             "",
			TlsKeyCredential:   tls.TlsKeyCredential{},
			TLSAllowedCACerts:  nil,
			TLSCipherSuites:    nil,
			TLSMinVersion:      "",
			TLSMaxVersion:      "",
			TLSAllowedDNSNames: nil,
			TLSEnable:          false,
		},
		TTL:            3600 * 24 * 30, // 1 month
		TimeoutSeconds: 10,
		DB:             0,
		KeyPrefix:      "",
	}
}

// Validate Config
func (c *Config) Validate() error {
	if len(c.Address) == 0 {
		return ErrMissingAddress
	}
	return nil
}

func NewClient(config *Config) (*Client, error) {
	if config == nil {
		config = NewConfig()
	}

	err := config.Validate()
	if err != nil {
		return nil, err
	}
	var key []byte
	var cred *secure.Credential
	var pwd string
	key, err = secure.GenerateKey()
	if err != nil {
		return nil, err
	}
	if cred, err = secure.CredentialFromConfig(config.DefaultCredentialConfig, key, true); err != nil {
		return nil, err
	}

	pwd, err = cred.Get()
	cred.Clear()
	if err != nil {
		return nil, err
	}

	client := &Client{
		config:  config,
		timeout: time.Duration(config.TimeoutSeconds) * time.Second,
		ttl:     time.Duration(config.TTL) * time.Second,
		Client: redis.NewClient(&redis.Options{
			Addr:     config.Address,
			Password: pwd,
			DB:       config.DB,
		}),
	}
	for i := range pwd {
		[]byte(pwd)[i] = 0
	}
	return client, nil
}

func (c *Client) Connect() error {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	_, err := c.Client.Ping(ctx).Result()
	return err
}

func (c *Client) Close() error {
	return c.Client.Close()
}

// Key assemble key
func (c *Client) Key(key string) string {
	return c.config.KeyPrefix + key
}

// Prune stub method for compatibility with kv.KV interface
func (c *Client) Prune() error {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	if err := c.Client.FlushDB(ctx).Err(); err != nil {
		return err
	}
	return nil
}

// Get fetch a key
func (c *Client) Get(key string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	// Get data from Client
	data, err := c.Client.Get(ctx, c.Key(key)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			// not found
			return nil, nil
		}
	}
	return data, err
}

// Set sets a value
func (c *Client) Set(key string, value []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()
	return c.Client.Set(ctx, c.Key(key), value, c.ttl).Err()
}

// SetTTL sets a value with custom TTL
func (c *Client) SetTTL(key string, value []byte, ttl time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()
	return c.Client.Set(ctx, c.Key(key), value, ttl).Err()
}

// Delete removes a key
func (c *Client) Delete(key string) error {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()
	return c.Client.Del(ctx, c.Key(key)).Err()
}

// Fetch default ttl
func (c *Client) TTL() time.Duration {
	return c.ttl
}
