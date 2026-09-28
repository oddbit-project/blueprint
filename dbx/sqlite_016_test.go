package dbx_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/oddbit-project/blueprint/dbx"
	"github.com/oddbit-project/blueprint/provider/sqlite"
	"github.com/oddbit-project/gohan"
)

// sqliteItem is the record type for the SQLite tests below: version is
// filled by a column default, never written by the repository.
type sqliteItem struct {
	ID      int64   `db:"id,auto" json:"id" grid:"sort,filter"`
	Name    string  `db:"name" json:"name" grid:"search,sort"`
	Note    *string `db:"note" json:"note" goqu:"omitnil"`
	Version int64   `db:"version,auto" json:"version"`
}

// newSQLiteRepo opens a fresh SQLite file database with an "items" table
// and returns its Querier and a repository over it.
func newSQLiteRepo(t *testing.T) (*dbx.SQLQuerier, *dbx.Repository[sqliteItem]) {
	t.Helper()
	cfg := sqlite.NewClientConfig()
	cfg.DSN = filepath.Join(t.TempDir(), "dbx.db")
	client, err := sqlite.NewClient(cfg)
	require.NoError(t, err)
	t.Cleanup(client.Disconnect)

	_, err = client.Db().ExecContext(context.Background(),
		`CREATE TABLE items (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT UNIQUE, note TEXT NOT NULL DEFAULT 'none', version INTEGER NOT NULL DEFAULT 7)`)
	require.NoError(t, err)

	q, err := dbx.FromClient(client)
	require.NoError(t, err)
	r, err := dbx.NewRepository[sqliteItem](q, "items")
	require.NoError(t, err)
	return q, r
}

func TestSQLiteInsertIgnore(t *testing.T) {
	ctx := context.Background()
	_, r := newSQLiteRepo(t)

	ok, err := r.InsertIgnore(ctx, &sqliteItem{Name: "a"}, "name")
	require.NoError(t, err)
	assert.True(t, ok)

	ok, err = r.InsertIgnore(ctx, &sqliteItem{Name: "a"}, "name")
	require.NoError(t, err)
	assert.False(t, ok)

	ok, err = r.InsertIgnore(ctx, &sqliteItem{Name: "a"})
	require.NoError(t, err)
	assert.False(t, ok, "untargeted DO NOTHING ignores the conflict too")

	n, err := r.Count(ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, int64(1), n)
}

func TestSQLiteReturning(t *testing.T) {
	ctx := context.Background()
	_, r := newSQLiteRepo(t)

	rec := &sqliteItem{Name: "a"}
	ins, err := r.UpsertReturning(ctx, rec, []string{"name"})
	require.NoError(t, err)
	assert.NotZero(t, ins.ID)
	require.NotNil(t, ins.Note)
	assert.Equal(t, "none", *ins.Note, "column default")
	assert.Equal(t, int64(7), ins.Version, "column default")
	assert.Zero(t, rec.ID, "caller's record is not written")

	note := "n"
	upd, err := r.UpsertReturning(ctx, &sqliteItem{Name: "a", Note: &note}, []string{"name"})
	require.NoError(t, err)
	assert.Equal(t, ins.ID, upd.ID)
	require.NotNil(t, upd.Note)
	assert.Equal(t, "n", *upd.Note)

	list, err := r.UpdateReturning(ctx, map[string]any{"name": "b"}, gohan.Col("id").Eq(ins.ID))
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "b", list[0].Name)
	assert.Equal(t, int64(7), list[0].Version, "column the update did not set")

	list, err = r.UpdateReturning(ctx, map[string]any{"name": "c"}, gohan.Col("id").Eq(ins.ID+100))
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestSQLiteUpsertReturningDoNothing(t *testing.T) {
	ctx := context.Background()
	q, _ := newSQLiteRepo(t)

	// nameOnly writes a single column, so conflicting on it leaves nothing
	// to update: the statement is DO NOTHING.
	type nameOnly struct {
		ID   int64  `db:"id,auto"`
		Name string `db:"name"`
	}
	r, err := dbx.NewRepository[nameOnly](q, "items")
	require.NoError(t, err)

	got, err := r.UpsertReturning(ctx, &nameOnly{Name: "a"}, []string{"name"})
	require.NoError(t, err)
	assert.NotZero(t, got.ID)

	got, err = r.UpsertReturning(ctx, &nameOnly{Name: "a"}, []string{"name"})
	assert.Nil(t, got)
	assert.True(t, errors.Is(err, dbx.ErrNotFound), "got %v", err)
}

func TestSQLiteGroupedInsert(t *testing.T) {
	ctx := context.Background()
	q, r := newSQLiteRepo(t)

	type noteOnly struct {
		ID   int64   `db:"id,auto"`
		Note *string `db:"note" goqu:"omitnil"`
	}
	rn, err := dbx.NewRepository[noteOnly](q, "items")
	require.NoError(t, err)

	x := "x"
	require.NoError(t, rn.WithGroupedInserts().Insert(ctx, &noteOnly{}, &noteOnly{Note: &x}, &noteOnly{}))

	note := "n"
	err = r.Insert(ctx, &sqliteItem{Name: "a"}, &sqliteItem{Name: "b", Note: &note})
	assert.True(t, errors.Is(err, gohan.ErrInconsistentOmit), "default Insert: got %v", err)
	require.NoError(t, r.WithGroupedInserts().Insert(ctx, &sqliteItem{Name: "a"}, &sqliteItem{Name: "b", Note: &note}))

	// read through rn: the DEFAULT VALUES rows have a NULL name
	got, err := rn.List(ctx, rn.Select().OrderBy(gohan.Col("id").Asc()))
	require.NoError(t, err)
	require.Len(t, got, 5)
	notes := make([]string, len(got))
	for i, g := range got {
		notes[i] = *g.Note
	}
	assert.Equal(t, []string{"none", "none", "x", "none", "n"}, notes)

	// a failing group rolls back the groups already written
	err = r.WithGroupedInserts().Insert(ctx, &sqliteItem{Name: "c"}, &sqliteItem{Name: "a", Note: &note})
	require.Error(t, err)
	n, err := r.Count(ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, int64(5), n)
}

func TestSQLiteQueryGridWithCount(t *testing.T) {
	ctx := context.Background()
	_, r := newSQLiteRepo(t)
	for _, n := range []string{"ann", "anna", "bob", "annie"} {
		require.NoError(t, r.Insert(ctx, &sqliteItem{Name: n}))
	}
	g, err := dbx.NewGrid[sqliteItem]()
	require.NoError(t, err)

	rows, total, err := r.QueryGridWithCount(ctx, g, &dbx.GridQuery{
		SearchType: dbx.SearchStart,
		SearchText: "ann",
		Sort:       []dbx.SortField{{Field: "id", Order: dbx.SortAscending}},
		Limit:      1,
		Offset:     2,
	})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "annie", rows[0].Name)
	assert.Equal(t, int64(3), total)
}

func TestSQLiteQuery(t *testing.T) {
	ctx := context.Background()
	q, r := newSQLiteRepo(t)
	for _, n := range []string{"a", "b"} {
		require.NoError(t, r.Insert(ctx, &sqliteItem{Name: n}))
	}

	type noteCount struct {
		Note  string `db:"note"`
		Items int64  `db:"items"`
	}
	st := gohan.Select("note", gohan.Count(gohan.Col("id")).As("items")).From("items").GroupBy("note")

	list, err := dbx.Query[noteCount](ctx, q, st)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, noteCount{Note: "none", Items: 2}, *list[0])

	one, err := dbx.QueryOne[noteCount](ctx, q, st.Where(gohan.Col("name").Eq("a")))
	require.NoError(t, err)
	assert.Equal(t, int64(1), one.Items)

	_, err = dbx.QueryOne[noteCount](ctx, q, st.Where(gohan.Col("name").Eq("zzz")))
	assert.True(t, errors.Is(err, dbx.ErrNotFound), "got %v", err)
}
