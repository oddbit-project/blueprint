package main

import (
	"fmt"
	"github.com/oddbit-project/blueprint/provider/pgsql"
	"log"
	"os"
)

func main() {
	pgConfig := &pgsql.ClientConfig{
		DSN: "postgres://username:password@localhost:5432/database?sslmode=allow",
	}

	client, err := pgsql.NewClient(pgConfig)
	if err != nil {
		log.Fatal(err)
	}
	if err = client.Connect(); err != nil {
		log.Fatal(err)
	}
	defer client.Disconnect()

	var greeting string
	err = client.Conn.QueryRow("select 'Hello, world!'").Scan(&greeting)
	if err != nil {
		fmt.Fprintf(os.Stderr, "QueryRow failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Greeting: ", greeting)
	fmt.Println("Done!")
}
