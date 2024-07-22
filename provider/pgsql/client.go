package pgsql

import (
	"context"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/oddbit-project/blueprint/db"
	"github.com/oddbit-project/blueprint/utils"
	"time"
)

const (
	DefaultMinConns           = 2
	DefaultMaxConns           = 4
	DefaultConnLifeTimeSecond = 3600
	DefaultConnIdleTimeSecond = 1800
	DefaultHealthCheckSecond  = 60
	DefaultConnTimeoutSecond  = 5

	ErrEmptyDSN                   = utils.Error("Empty DSN")
	ErrNilConfig                  = utils.Error("Config is nil")
	ErrInvalidMinConns            = utils.Error("Invalid minConns")
	ErrInvalidMaxConns            = utils.Error("Invalid maxConns")
	ErrInvalidMinMaxConns         = utils.Error("minConns must be <= maxConns")
	ErrInvalidConnLifeTime        = utils.Error("connLifeTime must be >= 1")
	ErrInvalidConnIdleTime        = utils.Error("connIdleTime must be >= 1")
	ErrInvalidHealthCheckInterval = utils.Error("healthCheckInterval must be >= 1")
	ErrInvalidConnTimeout         = utils.Error("connTimeout must be >= 1")
)

type ClientConfig struct {
	DSN string `json:"dsn"`
}

type PoolConfig struct {
	DSN      string `json:"dsn"`      // DSN database connection string
	MinConns int32  `json:"minConns"` // MinConns minimum number of pool connections
	MaxConns int32  `json:"maxConns"` // MaxConns max number of pool connections

	// ConnLifeTime is the duration in seconds since creation after which a connection will be automatically closed
	ConnLifeTime int `json:"connLifeTime"`
	// ConnIdleTime is the duration in seconds after which an idle connection will be automatically closed by the health check
	ConnIdleTime int `json:"connIdleTime"`
	// HealthCheckInterval is the duration in seconds between checks of the health of idle connections
	HealthCheckInterval int `json:"healthCheckInterval"`
	// ConnTimeout is the max duration in seconds of a database operation until timeout is reached
	ConnTimeout int `json:"connTimeout"`

	// optional method override
	BeforeConnect func(context.Context, *pgx.ConnConfig) error
	AfterConnect  func(context.Context, *pgx.Conn) error
	BeforeAcquire func(context.Context, *pgx.Conn) bool
	AfterRelease  func(*pgx.Conn) bool
	BeforeClose   func(*pgx.Conn)
}

func NewPoolConfig() *PoolConfig {
	return &PoolConfig{
		DSN:                 "",
		MinConns:            DefaultMinConns,
		MaxConns:            DefaultMaxConns,
		ConnLifeTime:        DefaultConnLifeTimeSecond,
		ConnIdleTime:        DefaultConnIdleTimeSecond,
		HealthCheckInterval: DefaultHealthCheckSecond,
		ConnTimeout:         DefaultConnTimeoutSecond,

		BeforeClose:   nil,
		BeforeConnect: nil,
		AfterConnect:  nil,
		BeforeAcquire: nil,
		AfterRelease:  nil,
	}
}

func (c PoolConfig) Validate() error {
	if len(c.DSN) == 0 {
		return ErrEmptyDSN
	}
	if c.MinConns < 1 {
		return ErrInvalidMinConns
	}
	if c.MaxConns < 1 {
		return ErrInvalidMaxConns
	}
	if c.MinConns > c.MaxConns {
		return ErrInvalidMinMaxConns
	}
	if c.ConnLifeTime < 1 {
		return ErrInvalidConnLifeTime
	}
	if c.ConnIdleTime < 1 {
		return ErrInvalidConnIdleTime
	}
	if c.HealthCheckInterval < 1 {
		return ErrInvalidHealthCheckInterval
	}
	if c.ConnTimeout < 1 {
		return ErrInvalidConnTimeout
	}
	return nil
}

func (c ClientConfig) Validate() error {
	if len(c.DSN) == 0 {
		return ErrEmptyDSN
	}
	return nil
}

func NewClient(config *ClientConfig) (*db.SqlClient, error) {
	if config == nil {
		return nil, ErrNilConfig
	}
	if err := config.Validate(); err != nil {
		return nil, err
	}
	return db.NewSqlClient(config.DSN, "pgx"), nil
}

func NewClientX(ctx context.Context, config *ClientConfig) (*pgx.Conn, error) {
	if config == nil {
		return nil, ErrNilConfig
	}
	if err := config.Validate(); err != nil {
		return nil, err
	}
	return pgx.Connect(ctx, config.DSN)
}

func NewPool(ctx context.Context, config *PoolConfig) (*pgxpool.Pool, error) {
	if config == nil {
		return nil, ErrNilConfig
	}
	if err := config.Validate(); err != nil {
		return nil, err
	}

	cfg, err := pgxpool.ParseConfig(config.DSN)
	if err != nil {
		return nil, err
	}

	cfg.MaxConns = config.MaxConns
	cfg.MinConns = config.MinConns
	cfg.MaxConnLifetime = time.Second * time.Duration(config.ConnLifeTime)
	cfg.MaxConnIdleTime = time.Second * time.Duration(config.ConnIdleTime)
	cfg.HealthCheckPeriod = time.Second * time.Duration(config.HealthCheckInterval)
	cfg.ConnConfig.ConnectTimeout = time.Second * time.Duration(config.ConnTimeout)
	cfg.BeforeClose = config.BeforeClose
	cfg.BeforeConnect = config.BeforeConnect
	cfg.AfterConnect = config.AfterConnect
	cfg.BeforeAcquire = config.BeforeAcquire
	cfg.AfterRelease = config.AfterRelease

	return pgxpool.NewWithConfig(ctx, cfg)
}
