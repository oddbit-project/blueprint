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
	pgConfig := pgsql.NewPoolConfig()
	pgConfig.DSN = "postgres://username:password@localhost:5432/database?sslmode=allow"

	db, err := pgsql.NewPool(context.Background(), pgConfig)
	if err != nil {
		log.Fatal(err)
	}

	source, err := migrations.NewEmbedSource(migFs, "migrations")

	manager := pgsql.NewMigrationManager(db)
	fmt.Println("Applying migrations...")
	if err = manager.Run(context.Background(), source, migrations.DefaultProgressFn); err != nil {
		fmt.Println("Error: ", err)
	}
	fmt.Println("Done!")
}
