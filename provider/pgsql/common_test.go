package pgsql

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"testing"
)

func getDSN() string {
	user := os.Getenv("POSTGRES_USER")
	pwd := os.Getenv("POSTGRES_PASSWORD")
	db := os.Getenv("POSTGRES_DB")
	port := os.Getenv("POSTGRES_PORT")
	host := os.Getenv("POSTGRES_HOST")
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s", user, pwd, host, port, db)
}

func dbClient(t *testing.T) *pgxpool.Pool {

	cfg := NewPoolConfig()
	cfg.DSN = getDSN()
	pool, err := NewPool(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}

	return pool
}
