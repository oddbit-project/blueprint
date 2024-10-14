package main

import (
	"context"
	"embed"
	"fmt"
	"github.com/oddbit-project/blueprint/db/migrations"
	"github.com/oddbit-project/blueprint/provider/pgsql"
	"log"
)

//go:embed migrations/*.sql
var migFs embed.FS

func main() {
	pgConfig := pgsql.NewClientConfig()
	pgConfig.DSN = "postgres://username:password@localhost:5432/database?sslmode=allow"

	client, err := pgsql.NewClient(pgConfig)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Disconnect()

	source, err := migrations.NewEmbedSource(migFs, "migrations")

	manager, err := pgsql.NewMigrationManager(context.Background(), client)
	if err != nil {
		panic(err)
	}
	fmt.Println("Applying migrations...")
	if err = manager.Run(context.Background(), source, migrations.DefaultProgressFn); err != nil {
		fmt.Println("Error: ", err)
	}
	fmt.Println("Done!")
}
