package pgsql

import (
	"fmt"
	"github.com/oddbit-project/blueprint/db"
	"os"
	"testing"
)

func getDSN() string {
	user := os.Getenv("POSTGRES_USER")
	pwd := os.Getenv("POSTGRES_PASSWORD")
	database := os.Getenv("POSTGRES_DB")
	port := os.Getenv("POSTGRES_PORT")
	host := os.Getenv("POSTGRES_HOST")
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s", user, pwd, host, port, database)
}

func dbClient(t *testing.T) *db.SqlClient {

	cfg := NewClientConfig()
	cfg.DSN = getDSN()
	client, err := NewClient(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return client
}
