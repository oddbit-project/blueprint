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
	Dsn  string
}
