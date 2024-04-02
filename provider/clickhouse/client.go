package clickhouse

import (
	_ "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/jmoiron/sqlx"
	"github.com/oddbit-project/blueprint/db"
	"github.com/oddbit-project/blueprint/utils"
)

const (
	ErrEmptyDSN = utils.Error("Empty DSN")
)

type ClientConfig struct {
	DSN string `json:"dsn"`
}

type Client db.Client

func (c ClientConfig) Validate() error {
	if len(c.DSN) == 0 {
		return ErrEmptyDSN
	}
	return nil
}

func NewClient(config *ClientConfig) (*Client, error) {

	if err := config.Validate(); err != nil {
		return nil, err
	}
	return &Client{
		Dsn: config.DSN,
	}, nil
}

func (db *Client) Connect() error {
	conn, err := sqlx.Open("clickhouse", db.Dsn)
	if err != nil {
		return err
	}

	if err := conn.Ping(); err != nil {
		return err
	}
	db.Conn = conn
	return nil
}

func (db *Client) GetClient() *sqlx.DB {
	return db.Conn
}

func (db *Client) IsConnected() bool {
	return db.Conn != nil
}

func (db *Client) Disconnect() {
	if db.Conn == nil {
		return
	}
	_ = db.Conn.Close()
	db.Conn = nil
}
