package db

import (
	_ "database/sql"
	"github.com/jmoiron/sqlx"
	"github.com/oddbit-project/blueprint/utils"
)

const (
	ErrInvalidParameters = utils.Error("invalid parameter count or parameter is nil")
)

type ClientInterface interface {
	GetClient() *sqlx.DB
	IsConnected() bool
	Connect() error
	Disconnect()
}

type ConnectionOptions interface {
	Apply(db *sqlx.DB) error
}

type SqlClient struct {
	Conn        *sqlx.DB
	Dsn         string
	DriverName  string
	connOptions ConnectionOptions
}

func NewSqlClient(dsn string, driverName string, connOptions ConnectionOptions) *SqlClient {
	return &SqlClient{
		Conn:        nil,
		Dsn:         dsn,
		DriverName:  driverName,
		connOptions: connOptions,
	}
}

func (c *SqlClient) Connect() error {
	conn, err := sqlx.Open(c.DriverName, c.Dsn)
	if err != nil {
		return err
	}

	if c.connOptions != nil {
		if err = c.connOptions.Apply(conn); err != nil {
			return err
		}
	}
	if err := conn.Ping(); err != nil {
		return err
	}
	c.Conn = conn
	return nil
}

func (c *SqlClient) Db() *sqlx.DB {
	if c.Conn == nil {
		if err := c.Connect(); err != nil {
			panic(err)
		}
	}
	return c.Conn
}

func (c *SqlClient) IsConnected() bool {
	return c.Conn != nil
}

func (c *SqlClient) Disconnect() {
	if c.Conn == nil {
		return
	}
	_ = c.Conn.Close()
	c.Conn = nil
}
