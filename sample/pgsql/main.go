package main

import (
	"context"
	"fmt"
	"github.com/oddbit-project/blueprint/provider/pgsql"
	"log"
	"os"
)

func main() {
	pgConfig := pgsql.NewPoolConfig()
	pgConfig.DSN = "postgres://username:password@localhost:5432/database?sslmode=allow"

	pool, err := pgsql.NewPool(context.Background(), pgConfig)
	if err != nil {
		log.Fatal(err)
	}

	db, err := pool.Acquire(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	defer db.Release()

	var greeting string
	err = db.QueryRow(context.Background(), "select 'Hello, world!'").Scan(&greeting)
	if err != nil {
		fmt.Fprintf(os.Stderr, "QueryRow failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Greeting: ", greeting)
	fmt.Println("Done!")
}
