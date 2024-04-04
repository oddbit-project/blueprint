package clickhouse

import (
	_ "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/oddbit-project/blueprint/db"
	"github.com/oddbit-project/blueprint/utils"
)

const (
	ErrEmptyDSN = utils.Error("Empty DSN")
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

	if err := config.Validate(); err != nil {
		return nil, err
	}
	return db.NewSqlClient(config.DSN, "clickhouse"), nil
}
