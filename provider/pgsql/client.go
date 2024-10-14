package pgsql

import (
	"github.com/doug-martin/goqu/v9"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"github.com/oddbit-project/blueprint/db"
	"github.com/oddbit-project/blueprint/utils"
	"time"
)

const (
	DefaultIdleConns          = 2
	DefaultMaxConns           = 4
	DefaultConnLifeTimeSecond = 3600
	DefaultConnIdleTimeSecond = 1800

	ErrEmptyDSN            = utils.Error("Empty DSN")
	ErrNilConfig           = utils.Error("Config is nil")
	ErrInvalidIdleConns    = utils.Error("Invalid idleConns")
	ErrInvalidMaxConns     = utils.Error("Invalid maxConns")
	ErrInvalidConnLifeTime = utils.Error("connLifeTime must be >= 1")
	ErrInvalidConnIdleTime = utils.Error("connIdleTime must be >= 1")
)

type ClientConfig struct {
	DSN          string `json:"dsn"`
	MaxOpenConns int    `json:"maxOpenConns"` // MaxOpenConns max number of pool connections
	MaxIdleConns int    `json:"maxIdleConns"` // MaxIdleConns max number of idle pool connections

	// ConnLifeTime is the duration in seconds since creation after which a connection will be automatically closed
	ConnLifetime int `json:"connLifetime"`
	// ConnIdleTime is the duration in seconds after which an idle connection will be automatically closed by the health check
	ConnIdleTime int `json:"connIdleTime"`
}

func NewClientConfig() *ClientConfig {
	return &ClientConfig{
		DSN:          "",
		MaxIdleConns: DefaultIdleConns,
		MaxOpenConns: DefaultMaxConns,
		ConnLifetime: DefaultConnLifeTimeSecond,
		ConnIdleTime: DefaultConnIdleTimeSecond,
	}
}

func (c ClientConfig) Validate() error {
	if len(c.DSN) == 0 {
		return ErrEmptyDSN
	}
	if c.MaxIdleConns < 0 {
		return ErrInvalidIdleConns
	}
	if c.MaxOpenConns < 1 {
		return ErrInvalidMaxConns
	}
	if c.ConnLifetime < 1 {
		return ErrInvalidConnLifeTime
	}
	if c.ConnIdleTime < 1 {
		return ErrInvalidConnIdleTime
	}
	return nil
}

func (c ClientConfig) Apply(db *sqlx.DB) error {
	db.SetMaxOpenConns(c.MaxOpenConns)
	db.SetMaxIdleConns(c.MaxIdleConns)
	db.SetConnMaxIdleTime(time.Duration(c.ConnIdleTime) * time.Second)
	db.SetConnMaxLifetime(time.Duration(c.ConnLifetime) * time.Second)
	return nil
}

func NewClient(config *ClientConfig) (*db.SqlClient, error) {
	if config == nil {
		return nil, ErrNilConfig
	}
	if err := config.Validate(); err != nil {
		return nil, err
	}
	return db.NewSqlClient(config.DSN, "pgx", config), nil
}

func DialectOptions() *goqu.SQLDialectOptions {
	do := goqu.DefaultDialectOptions()
	do.PlaceHolderFragment = []byte("$")
	do.IncludePlaceholderNum = true
	return do
}

func init() {
	goqu.RegisterDialect("pgx", DialectOptions())
}
