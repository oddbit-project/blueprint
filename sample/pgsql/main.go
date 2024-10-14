package main

import (
	"context"
	"fmt"
	"github.com/oddbit-project/blueprint/provider/pgsql"
	"log"
	"os"
)

func main() {
	pgConfig := pgsql.NewClientConfig()
	pgConfig.DSN = "postgres://username:password@localhost:5432/database?sslmode=allow"

	client, err := pgsql.NewClient(pgConfig)
	if err != nil {
		log.Fatal(err)
	}
	db := client.Db()
	defer client.Disconnect()

	var greeting string
	err = db.QueryRowxContext(context.Background(), "select 'Hello, world!'").Scan(&greeting)
	if err != nil {
		fmt.Fprintf(os.Stderr, "QueryRow failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Greeting: ", greeting)
	fmt.Println("Done!")
}
