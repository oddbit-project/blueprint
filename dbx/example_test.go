package dbx_test

import (
	"context"
	"os"
	"path/filepath"

	"github.com/oddbit-project/blueprint/dbx"
	"github.com/oddbit-project/blueprint/provider/sqlite"
	"github.com/oddbit-project/gohan"
)

// exampleUser is the record type used by ExampleNewRepository.
type exampleUser struct {
	ID   int64  `db:"id,auto"`
	Name string `db:"name"`
}

// ExampleNewRepository shows the shape of a dbx repository over a
// database/sql client: FromClient resolves the dialect from the client's
// driver, NewRepository binds a table, and every call takes a ctx. This
// example only compiles (no // Output: line) — it opens a real SQLite
// file to keep the code exactly what a caller would write, but does not
// print anything comparable across environments.
func ExampleNewRepository() {
	dbPath := filepath.Join(os.TempDir(), "blueprint-dbx-example.db")
	_ = os.Remove(dbPath)
	defer func() { _ = os.Remove(dbPath) }()

	cfg := sqlite.NewClientConfig()
	cfg.DSN = dbPath
	client, err := sqlite.NewClient(cfg)
	if err != nil {
		panic(err)
	}
	defer client.Disconnect()

	ctx := context.Background()
	if _, err := client.Db().ExecContext(ctx, `CREATE TABLE users (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL)`); err != nil {
		panic(err)
	}

	q, err := dbx.FromClient(client)
	if err != nil {
		panic(err)
	}

	repo, err := dbx.NewRepository[exampleUser](q, "users")
	if err != nil {
		panic(err)
	}

	if err := repo.Insert(ctx, &exampleUser{Name: "alice"}); err != nil {
		panic(err)
	}

	user, err := repo.GetBy(ctx, map[string]any{"name": "alice"})
	if err != nil {
		panic(err)
	}
	_ = user

	users, err := repo.List(ctx, repo.Select().Where(gohan.Col("name").HasPrefix("al")))
	if err != nil {
		panic(err)
	}
	_ = users

	err = dbx.WithTx(ctx, q, nil, func(tx dbx.Querier) error {
		txRepo := repo.With(tx)
		_, err := txRepo.Delete(ctx, gohan.Col("id").Eq(user.ID))
		return err
	})
	if err != nil {
		panic(err)
	}
}
