package db

import (
	_ "database/sql"
	"github.com/jmoiron/sqlx"
	"github.com/oddbit-project/blueprint/utils"
)

const ErrAbstractMethod = utils.Error("Abstract method")

type ClientInterface interface {
	GetClient() *sqlx.DB
	IsConnected() bool
	Connect() error
	Disconnect()
}

type SqlClient struct {
	Conn       *sqlx.DB
	Dsn        string
	DriverName string
}

func NewSqlClient(dsn string, driverName string) *SqlClient {
	return &SqlClient{
		Conn:       nil,
		Dsn:        dsn,
		DriverName: driverName,
	}
}

func (c *SqlClient) Connect() error {
	conn, err := sqlx.Open(c.DriverName, c.Dsn)
	if err != nil {
		return err
	}

	if err := conn.Ping(); err != nil {
		return err
	}
	c.Conn = conn
	return nil
}

func (c *SqlClient) GetClient() *sqlx.DB {
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
