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

	backend := pgsql.NewMigrationBackend(client.Conn)
	err = backend.Initialize()
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
	rows, err := backend.List()
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
	fmt.Println("Listing Migrations:")
	for _, r := range rows {
		fmt.Println(r.Created, r.Name, r.SHA2)
	}

	fmt.Println("Done!")
}
