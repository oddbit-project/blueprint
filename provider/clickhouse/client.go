package clickhouse

import (
	_ "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/doug-martin/goqu/v9"
	"github.com/oddbit-project/blueprint/db"
	"github.com/oddbit-project/blueprint/utils"
)

const (
	ErrEmptyDSN  = utils.Error("Empty DSN")
	ErrNilConfig = utils.Error("Nil Config")
)

type ClientConfig struct {
	DSN string `json:"dsn"`
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
	return db.NewSqlClient(config.DSN, "clickhouse", nil), nil
}

func DialectOptions() *goqu.SQLDialectOptions {
	do := goqu.DefaultDialectOptions()
	do.PlaceHolderFragment = []byte("?")
	do.IncludePlaceholderNum = false
	return do
}

func init() {
	goqu.RegisterDialect("clickhouse", DialectOptions())
}
