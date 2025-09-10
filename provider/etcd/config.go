package etcd

import (
	"errors"
	"github.com/oddbit-project/blueprint/crypt/secure"
	tlsProvider "github.com/oddbit-project/blueprint/provider/tls"
	"time"
)

type Config struct {
	Endpoints []string `json:"endpoints"`

	Username string `json:"username"`
	secure.DefaultCredentialConfig
	tlsProvider.ClientConfig

	DialTimeout          time.Duration `json:"dialTimeout"`
	DialKeepAliveTime    time.Duration `json:"dialKeepAliveTime"`
	DialKeepAliveTimeout time.Duration `json:"dialKeepAliveTimeout"`

	RequestTimeout time.Duration `json:"requestTimeout"`

	EnableEncryption bool   `json:"enableEncryption"`
	EncryptionKey    []byte `json:"encryptionKey"`

	MaxCallSendMsgSize int `json:"maxCallSendMsgSize"`
	MaxCallRecvMsgSize int `json:"maxCallRecvMsgSize"`

	PermitWithoutStream bool `json:"permitWithoutStream"`
	RejectOldCluster    bool `json:"rejectOldCluster"`
}

func DefaultConfig() *Config {
	return &Config{
		Endpoints:            []string{"localhost:2379"},
		DialTimeout:          5 * time.Second,
		DialKeepAliveTime:    30 * time.Second,
		DialKeepAliveTimeout: 10 * time.Second,
		RequestTimeout:       5 * time.Second,
		EnableEncryption:     false,
		MaxCallSendMsgSize:   2 * 1024 * 1024,
		MaxCallRecvMsgSize:   2 * 1024 * 1024,
		PermitWithoutStream:  false,
		RejectOldCluster:     false,
	}
}

func (c *Config) WithEndpoints(endpoints ...string) *Config {
	c.Endpoints = endpoints
	return c
}

func (c *Config) WithAuth(username, password string) *Config {
	c.Username = username
	c.Password = password
	return c
}

func (c *Config) WithTLS(certFile, keyFile, caFile string, allowInsecure bool) *Config {
	c.TLSEnable = true
	c.TLSCert = certFile
	c.TLSKey = keyFile
	c.TLSCA = caFile
	c.TLSInsecureSkipVerify = allowInsecure
	return c
}

func (c *Config) WithEncryption(key []byte) *Config {
	c.EnableEncryption = true
	c.EncryptionKey = key
	return c
}

func (c *Config) WithTimeout(timeout time.Duration) *Config {
	c.RequestTimeout = timeout
	return c
}

func (c *Config) WithDialTimeout(timeout time.Duration) *Config {
	c.DialTimeout = timeout
	return c
}

func (c *Config) Validate() error {
	if len(c.Endpoints) == 0 {
		return errors.New("no etcd endpoints provided")
	}
	return nil
}

func (c *Config) NewClient() (*Client, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return NewClient(c)
}
