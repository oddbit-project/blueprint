package main

import (
	"context"
	"embed"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/oddbit-project/blueprint/db"
	"github.com/oddbit-project/blueprint/db/migrations"
	"github.com/oddbit-project/blueprint/provider/sqlite"
)

//go:embed migrations/*.sql
var migFs embed.FS

type User struct {
	Id        int       `db:"id,auto" goqu:"skipinsert"`
	Username  string    `db:"username"`
	Email     string    `db:"email"`
	CreatedAt time.Time `db:"created_at" goqu:"skipinsert"`
}

func main() {
	dbPath := filepath.Join(os.TempDir(), "blueprint-sqlite-sample.db")
	_ = os.Remove(dbPath)

	cfg := sqlite.NewClientConfig()
	cfg.DSN = dbPath

	client, err := sqlite.NewClient(cfg)
	if err != nil {
		log.Fatal(err)
	}
	if err := client.Connect(); err != nil {
		log.Fatal(err)
	}
	defer client.Disconnect()

	ctx := context.Background()

	var version string
	if err := client.Db().QueryRowContext(ctx, "SELECT sqlite_version()").Scan(&version); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Connected to SQLite %s at %s\n\n", version, dbPath)

	// Apply migrations
	source, err := migrations.NewEmbedSource(migFs, "migrations")
	if err != nil {
		log.Fatal(err)
	}
	mgr, err := sqlite.NewMigrationManager(ctx, client)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Applying migrations...")
	if err := mgr.Run(ctx, source, migrations.DefaultProgressFn); err != nil {
		log.Fatal(err)
	}

	// Insert + read via repository
	repo := db.NewRepository(ctx, client, "users")

	if err := repo.Insert(&User{Username: "alice", Email: "alice@example.com"}); err != nil {
		log.Fatal(err)
	}
	if err := repo.Insert(&User{Username: "bob", Email: "bob@example.com"}); err != nil {
		log.Fatal(err)
	}

	users := make([]*User, 0)
	if err := repo.Fetch(repo.SqlSelect(), &users); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("\nUsers (%d):\n", len(users))
	for _, u := range users {
		fmt.Printf("  #%d  %-10s  %s  (created %s)\n", u.Id, u.Username, u.Email, u.CreatedAt.Format(time.RFC3339))
	}
}
