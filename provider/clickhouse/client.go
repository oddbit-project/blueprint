package clickhouse

import (
	"context"
	"crypto/tls"
	"github.com/ClickHouse/ch-go/compress"
	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/doug-martin/goqu/v9"
	"github.com/oddbit-project/blueprint/crypt/secure"
	"github.com/oddbit-project/blueprint/db"
	"github.com/oddbit-project/blueprint/db/qb"
	tlsProvider "github.com/oddbit-project/blueprint/provider/tls"
	"github.com/oddbit-project/blueprint/utils"
	"slices"
	"time"
)

const (
	ErrEmptyHosts             = utils.Error("empty hosts")
	ErrNilConfig              = utils.Error("Nil Config")
	ErrInvalidCompression     = utils.Error("invalid compression value")
	ErrInvalidDialTimeout     = utils.Error("invalid dial timeout value")
	ErrInvalidMaxOpenConns    = utils.Error("invalid max open connections value")
	ErrInvalidMaxIdleConns    = utils.Error("invalid max idle connections value")
	ErrInvalidConnMaxLifetime = utils.Error("invalid connMaxLifetime value")
	ErrInvalidConnStrategy    = utils.Error("invalid connStrategy value")

	CompressionLZ4     = "lz4"
	CompressionNone    = "none"
	CompressionZSTD    = "zstd"
	CompressionGZIP    = "gzip"
	CompressionBrotli  = "br"
	CompressionDeflate = "deflate"

	ConnSequential = "sequential"
	ConnRoundRobin = "roundRobin"
)

type ClientConfig struct {
	Hosts                          []string       `json:"hosts"`
	Database                       string         `json:"database"`
	Username                       string         `json:"username"`
	Debug                          bool           `json:"debug"`       //Debug true/false to enable debugging
	Compression                    string         `json:"compression"` // Compression algorithm: lz4, none
	DialTimeout                    int            `json:"dialTimeout"`
	MaxOpenConns                   int            `json:"maxOpenConns"`
	MaxIdleConns                   int            `json:"maxIdleConns"`
	ConnMaxLifetime                int            `json:"connMaxLifetime"`
	ConnStrategy                   string         `json:"connStrategy"` // either sequential or roundRobin
	BlockBufferSize                uint8          `json:"blockBufferSize"`
	Settings                       map[string]any `json:"settings"`
	secure.DefaultCredentialConfig                // optional password
	tlsProvider.ClientConfig
}

type Client struct {
	Conn    clickhouse.Conn
	Version *clickhouse.ServerVersion
}

var validCompression = []string{
	CompressionLZ4,
	CompressionBrotli,
	CompressionGZIP,
	CompressionNone,
	CompressionZSTD,
	CompressionDeflate,
}
var compressionMap = map[string]clickhouse.CompressionMethod{
	CompressionLZ4:     clickhouse.CompressionMethod(compress.LZ4),
	CompressionNone:    clickhouse.CompressionMethod(compress.None),
	CompressionZSTD:    clickhouse.CompressionMethod(compress.ZSTD),
	CompressionGZIP:    clickhouse.CompressionMethod(0x95),
	CompressionDeflate: clickhouse.CompressionMethod(0x96),
	CompressionBrotli:  clickhouse.CompressionMethod(0x97),
}

var validConnStrategy = []string{ConnSequential, ConnRoundRobin}

var connStrategyMap = map[string]clickhouse.ConnOpenStrategy{
	ConnSequential: clickhouse.ConnOpenInOrder,
	ConnRoundRobin: clickhouse.ConnOpenRoundRobin,
}

func NewClientConfig() *ClientConfig {
	return &ClientConfig{
		Hosts:           []string{},
		Debug:           false,
		Compression:     "lz4",
		DialTimeout:     5,
		MaxOpenConns:    100,
		MaxIdleConns:    0,
		ConnMaxLifetime: 3600,
		ConnStrategy:    ConnSequential,
		BlockBufferSize: 2, // default value

	}
}

func (c ClientConfig) Validate() error {
	if c.Hosts == nil || len(c.Hosts) == 0 {
		return ErrEmptyHosts
	}

	if c.DialTimeout < 0 {
		return ErrInvalidDialTimeout
	}
	if c.MaxOpenConns < 0 {
		return ErrInvalidMaxOpenConns
	}
	if c.MaxIdleConns < 0 {
		return ErrInvalidMaxIdleConns
	}
	if c.ConnMaxLifetime < 0 {
		return ErrInvalidConnMaxLifetime
	}

	if slices.Index(validCompression, c.Compression) < 0 {
		return ErrInvalidCompression
	}

	if slices.Index(validConnStrategy, c.ConnStrategy) < 0 {
		return ErrInvalidConnStrategy
	}

	return nil
}

func NewClient(config *ClientConfig) (*Client, error) {
	if config == nil {
		return nil, ErrNilConfig
	}
	if err := config.Validate(); err != nil {
		return nil, err
	}

	// username & password
	var key []byte
	var credential *secure.Credential
	var password string
	var err error

	key, err = secure.GenerateKey()
	if err != nil {
		return nil, err
	}
	if credential, err = secure.CredentialFromConfig(config.DefaultCredentialConfig, key, true); err != nil {
		return nil, err
	}
	password, err = credential.Get()
	if err != nil {
		return nil, err
	}

	// TLS
	var tlsSettings *tls.Config
	if tlsSettings, err = config.TLSConfig(); err != nil {
		return nil, err
	}

	opts := &clickhouse.Options{
		Addr:  config.Hosts,
		Debug: config.Debug,
		Auth: clickhouse.Auth{
			Database: config.Database,
			Username: config.Username,
			Password: password,
		},
		TLS: tlsSettings,
		Compression: &clickhouse.Compression{
			Method: compressionMap[config.Compression],
		},
		ConnOpenStrategy: connStrategyMap[config.ConnStrategy],
	}

	if config.DialTimeout > 0 {
		opts.DialTimeout = time.Duration(config.DialTimeout) * time.Second
	}
	if config.MaxOpenConns > 0 {
		opts.MaxOpenConns = config.MaxOpenConns
	}
	if config.MaxIdleConns > 0 {
		opts.MaxIdleConns = config.MaxIdleConns
	}
	if config.ConnMaxLifetime > 0 {
		opts.ConnMaxLifetime = time.Duration(config.ConnMaxLifetime) * time.Second
	}
	if config.BlockBufferSize > 0 {
		opts.BlockBufferSize = config.BlockBufferSize
	}

	if config.Settings != nil && len(config.Settings) > 0 {
		opts.Settings = config.Settings
	}

	var conn driver.Conn
	conn, err = clickhouse.Open(opts)
	if err != nil {
		return nil, err
	}

	if version, err := conn.ServerVersion(); err != nil {
		return nil, err
	} else {
		return &Client{
			Conn:    conn,
			Version: version,
		}, nil
	}
}

// NewRepository create a new repository
func (c *Client) NewRepository(ctx context.Context, tableName string) Repository {
	return NewRepository(ctx, c.Conn, tableName)
}

func (c *Client) Ping(ctx context.Context) error {
	return c.Conn.Ping(ctx)
}

func (c *Client) Stats() driver.Stats {
	return c.Conn.Stats()
}

func (c *Client) Close() error {
	return c.Conn.Close()
}

func DialectOptions() *goqu.SQLDialectOptions {
	do := goqu.DefaultDialectOptions()
	do.PlaceHolderFragment = []byte("?")
	do.IncludePlaceholderNum = false
	return do
}

func init() {
	goqu.RegisterDialect("clickhouse", DialectOptions())
	db.RegisterDialect("clickhouse", qb.DefaultSqlDialect())
}
