// Command dbx-sqlite is a runnable sample for the dbx package: it opens a
// SQLite database (no external service required), creates a users table,
// inserts two users through a dbx.Repository, counts them, fetches one by
// name with GetBy, and runs a Contains search.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/oddbit-project/blueprint/dbx"
	"github.com/oddbit-project/blueprint/provider/sqlite"
	"github.com/oddbit-project/gohan"
)

// User is the record type for the sample's users table.
type User struct {
	ID   int64  `db:"id,auto"`
	Name string `db:"name"`
}

func main() {
	dbPath := filepath.Join(os.TempDir(), "blueprint-dbx-sample.db")
	_ = os.Remove(dbPath)
	defer os.Remove(dbPath)

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

	const createUsers = `CREATE TABLE users (
		id   INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL
	)`
	if _, err := client.Db().ExecContext(ctx, createUsers); err != nil {
		log.Fatal(err)
	}

	q, err := dbx.FromClient(client)
	if err != nil {
		log.Fatal(err)
	}

	repo, err := dbx.NewRepository[User](q, "users")
	if err != nil {
		log.Fatal(err)
	}

	if err := repo.Insert(ctx, &User{Name: "alice"}); err != nil {
		log.Fatal(err)
	}
	if err := repo.Insert(ctx, &User{Name: "bob"}); err != nil {
		log.Fatal(err)
	}

	count, err := repo.Count(ctx, nil)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("count=%d\n", count)

	alice, err := repo.GetBy(ctx, map[string]any{"name": "alice"})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("name=%s\n", alice.Name)

	found, err := repo.List(ctx, repo.Select().Where(gohan.Col("name").Contains("ali")))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("found=%d\n", len(found))
}
