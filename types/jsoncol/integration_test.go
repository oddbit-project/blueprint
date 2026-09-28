package jsoncol_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/oddbit-project/blueprint/db"
	"github.com/oddbit-project/blueprint/dbx"
	"github.com/oddbit-project/blueprint/provider/sqlite"
	"github.com/oddbit-project/blueprint/types/jsoncol"
	"github.com/oddbit-project/gohan"
)

type prefs struct {
	Theme string         `json:"theme"`
	Tags  []string       `json:"tags"`
	Extra map[string]int `json:"extra,omitempty"`
}

type account struct {
	ID    int64                           `db:"id,auto"`
	Prefs jsoncol.JSON[prefs]             `db:"prefs"`
	Meta  *jsoncol.JSON[map[string]any]   `db:"meta"`
	List  jsoncol.JSON[[]int]             `db:"list"`
	Opt   jsoncol.JSON[map[string]string] `db:"opt"`
}

// runRoundTrip inserts and reads back accounts through a dbx repository,
// covering a populated value, SQL NULL through *JSON[T], and JSON null
// through a nil map.
func runRoundTrip(t *testing.T, q dbx.Querier) {
	ctx := context.Background()
	repo, err := dbx.NewRepository[account](q, "accounts")
	require.NoError(t, err)

	full := &account{
		Prefs: jsoncol.JSON[prefs]{V: prefs{Theme: "dark", Tags: []string{"a", "b'c", `d"e\f`}, Extra: map[string]int{"x": 1}}},
		Meta:  &jsoncol.JSON[map[string]any]{V: map[string]any{"n": 1.5, "s": "é", "nested": map[string]any{"b": true}}},
		List:  jsoncol.JSON[[]int]{V: []int{1, 2, 3}},
		Opt:   jsoncol.JSON[map[string]string]{V: map[string]string{"k": "v"}},
	}
	empty := &account{Prefs: jsoncol.JSON[prefs]{V: prefs{Theme: "light"}}}
	require.NoError(t, repo.Insert(ctx, full))
	require.NoError(t, repo.Insert(ctx, empty))

	rows, err := repo.List(ctx, repo.Select().OrderBy(gohan.Col("id").Asc()))
	require.NoError(t, err)
	require.Len(t, rows, 2)

	assert.Equal(t, full.Prefs, rows[0].Prefs)
	require.NotNil(t, rows[0].Meta)
	assert.Equal(t, full.Meta.V, rows[0].Meta.V)
	assert.Equal(t, full.List, rows[0].List)
	assert.Equal(t, full.Opt, rows[0].Opt)

	assert.Equal(t, empty.Prefs, rows[1].Prefs)
	assert.Nil(t, rows[1].Meta, "SQL NULL must scan into a nil *JSON[T]")
	assert.Nil(t, rows[1].List.V, "JSON null must scan into a nil slice")
	assert.Nil(t, rows[1].Opt.V)

	// a nil *JSON[T] is SQL NULL, a nil map inside JSON[T] is JSON null
	n, err := repo.Count(ctx, gohan.And(gohan.Col("meta").IsNull(), gohan.Col("opt").IsNotNull()))
	require.NoError(t, err)
	assert.Equal(t, int64(1), n)

	// UpdateFields binds a JSON[T] value directly
	_, err = repo.UpdateFields(ctx, map[string]any{"prefs": jsoncol.JSON[prefs]{V: prefs{Theme: "blue"}}}, gohan.Col("id").Eq(rows[1].ID))
	require.NoError(t, err)
	got, err := repo.GetBy(ctx, map[string]any{"id": rows[1].ID})
	require.NoError(t, err)
	assert.Equal(t, "blue", got.Prefs.V.Theme)
}

func TestSQLiteRoundTrip(t *testing.T) {
	cfg := sqlite.NewClientConfig()
	cfg.DSN = filepath.Join(t.TempDir(), "jsoncol.db")
	client, err := sqlite.NewClient(cfg)
	require.NoError(t, err)
	t.Cleanup(client.Disconnect)

	_, err = client.Db().ExecContext(context.Background(),
		`CREATE TABLE accounts (id INTEGER PRIMARY KEY AUTOINCREMENT, prefs TEXT NOT NULL, meta TEXT, list TEXT NOT NULL, opt TEXT NOT NULL)`)
	require.NoError(t, err)

	q, err := dbx.FromClient(client)
	require.NoError(t, err)
	runRoundTrip(t, q)

	// stored as TEXT, not BLOB
	var typ string
	require.NoError(t, client.Db().QueryRowContext(context.Background(), `SELECT typeof(prefs) FROM accounts LIMIT 1`).Scan(&typ))
	assert.Equal(t, "text", typ)
}

func TestPostgresRoundTrip(t *testing.T) {
	if testing.Short() {
		t.Skip("requires Docker; skipped under -short")
	}
	ctx := context.Background()
	pg, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("jsoncol"),
		postgres.WithUsername("jsoncol"),
		postgres.WithPassword("password"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		if err := pg.Terminate(ctx); err != nil {
			t.Logf("failed to terminate container: %v", err)
		}
	})

	baseDSN, err := pg.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	for _, v := range []struct{ name, dsn string }{
		{"extended protocol", baseDSN},
		{"simple protocol", baseDSN + "&default_query_exec_mode=simple_protocol"},
	} {
		t.Run(v.name, func(t *testing.T) {
			client := db.NewSqlClient(v.dsn, "pgx", nil)
			q, err := dbx.FromClient(client)
			require.NoError(t, err)
			t.Cleanup(client.Disconnect)

			_, err = client.Conn.ExecContext(ctx, `DROP TABLE IF EXISTS accounts`)
			require.NoError(t, err)
			_, err = client.Conn.ExecContext(ctx,
				`CREATE TABLE accounts (id serial PRIMARY KEY, prefs jsonb NOT NULL, meta jsonb, list jsonb NOT NULL, opt json NOT NULL)`)
			require.NoError(t, err)
			runRoundTrip(t, q)

			// the stored value is a real JSON document, queryable server-side
			var theme string
			require.NoError(t, client.Conn.QueryRowContext(ctx, `SELECT prefs->>'theme' FROM accounts ORDER BY id LIMIT 1`).Scan(&theme))
			assert.Equal(t, "dark", theme)
		})
	}
}
