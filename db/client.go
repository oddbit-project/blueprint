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

type Client struct {
	ClientInterface
	Conn *sqlx.DB
}

func (c *Client) Connect() error {

	return ErrAbstractMethod
}
func (c *Client) GetClient() *sqlx.DB {
	return c.Conn
}

func (c *Client) IsConnected() bool {
	return c.Conn != nil
}

func (c *Client) Disconnect() {
	if c.Conn == nil {
		return
	}
	_ = c.Conn.Close()
	c.Conn = nil
}
