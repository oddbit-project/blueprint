package pgsql

import (
	_ "github.com/jackc/pgx/v5/stdlib"
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
	return db.NewSqlClient(config.DSN, "pgx"), nil
}
