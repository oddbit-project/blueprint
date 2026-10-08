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
	tls.ClientConfig
}

type Client struct {
	Redis   *redis.Client
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
		ClientConfig: tls.ClientConfig{
			TLSCA:                 "",
			TLSCert:               "",
			TLSKey:                "",
			TlsKeyCredential:      tls.TlsKeyCredential{},
			TLSEnable:             false,
			TLSInsecureSkipVerify: false,
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
	return c.ValidateEnabled()
}

func NewClient(config *Config) (*Client, error) {
	if config == nil {
		config = NewConfig()
	}

	err := config.Validate()
	if err != nil {
		return nil, err
	}
	tlsConfig, err := config.TLSConfig()
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
		Redis: redis.NewClient(&redis.Options{
			Addr:      config.Address,
			Password:  pwd,
			DB:        config.DB,
			TLSConfig: tlsConfig,
		}),
	}
	return client, nil
}

func (c *Client) Connect() error {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	_, err := c.Redis.Ping(ctx).Result()
	return err
}

func (c *Client) Close() error {
	return c.Redis.Close()
}

// Key assemble key
func (c *Client) Key(key string) string {
	return c.config.KeyPrefix + key
}

// Prune stub method for compatibility with kv.KV interface; Redis expires keys itself
func (c *Client) Prune() error {
	return nil
}

// Get fetch a key
func (c *Client) Get(key string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	// Get data from Redis
	data, err := c.Redis.Get(ctx, c.Key(key)).Bytes()
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
	return c.Redis.Set(ctx, c.Key(key), value, c.ttl).Err()
}

// SetTTL sets a value with custom TTL
func (c *Client) SetTTL(key string, value []byte, ttl time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()
	return c.Redis.Set(ctx, c.Key(key), value, ttl).Err()
}

// SetNX sets a value with custom TTL only if the key does not exist; returns true if the key was set
func (c *Client) SetNX(key string, value []byte, ttl time.Duration) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()
	return c.Redis.SetNX(ctx, c.Key(key), value, ttl).Result()
}

// Delete removes a key
func (c *Client) Delete(key string) error {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()
	return c.Redis.Del(ctx, c.Key(key)).Err()
}

// TTL Fetch default ttl
func (c *Client) TTL() time.Duration {
	return c.ttl
}
