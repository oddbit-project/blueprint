package dbx

import (
	"context"
	"errors"
	"strings"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/oddbit-project/gohan"
)

// --- test doubles ---

// recordedCall is one I/O call seen by recordingQuerier.
type recordedCall struct {
	method string
	sql    string
	args   []any
}

// recordingQuerier is a Querier double that records every statement it is
// asked to run (method, SQL, args), for goldens on dialects sqlmock cannot
// stand in for. It returns zero results and implements neither TxQuerier
// nor TxBeginner.
type recordingQuerier struct {
	d     gohan.Dialect
	calls []recordedCall
	count int64
}

func (r *recordingQuerier) record(method, query string, args []any) {
	r.calls = append(r.calls, recordedCall{method: method, sql: query, args: args})
}

func (r *recordingQuerier) Dialect() gohan.Dialect { return r.d }
func (r *recordingQuerier) Exec(ctx context.Context, query string, args ...any) (int64, error) {
	r.record("Exec", query, args)
	return 0, nil
}
func (r *recordingQuerier) Get(ctx context.Context, dest any, query string, args ...any) error {
	r.record("Get", query, args)
	return nil
}
func (r *recordingQuerier) Select(ctx context.Context, dest any, query string, args ...any) error {
	r.record("Select", query, args)
	return nil
}
func (r *recordingQuerier) QueryInt64(ctx context.Context, query string, args ...any) (int64, error) {
	r.record("QueryInt64", query, args)
	return r.count, nil
}

var _ Querier = (*recordingQuerier)(nil)

// --- record types ---

// allOmit maps only an auto column and an omitnil column, so a record with
// a nil Note writes no column at all.
type allOmit struct {
	ID   int64   `db:"id,auto"`
	Note *string `db:"note" goqu:"omitnil"`
}

func strPtr(s string) *string { return &s }

// --- W1: Grid.Conds / QueryGridWithCount ---

func TestGridConds(t *testing.T) {
	g, err := NewGrid[row]()
	require.NoError(t, err)

	t.Run("filters and search only, same WHERE as Build", func(t *testing.T) {
		q := &GridQuery{
			FilterFields: map[string]any{"tag": "a", "id": float64(3)},
			SearchType:   SearchAny,
			SearchText:   "x",
			Sort:         []SortField{{Field: "name", Order: SortAscending}},
			Limit:        10,
			Offset:       20,
		}
		conds, err := g.Conds(q)
		require.NoError(t, err)
		require.Len(t, conds, 3)

		sqlStr, args, err := rowBase().Where(conds...).Build(gohan.Postgres())
		require.NoError(t, err)
		assert.Equal(t, `SELECT "id", "name", "email", "tag" FROM "rows" WHERE ("id" = $1 AND "tag" = $2 AND ("name" LIKE $3 ESCAPE '!' OR "email" LIKE $4 ESCAPE '!'))`, sqlStr)
		assert.Equal(t, []any{float64(3), "a", "%x%", "%x%"}, args)

		built, err := g.Build(rowBase(), q)
		require.NoError(t, err)
		builtSQL, builtArgs, err := built.Build(gohan.Postgres())
		require.NoError(t, err)
		assert.True(t, strings.HasPrefix(builtSQL, sqlStr+" ORDER BY "), builtSQL)
		assert.Equal(t, args, builtArgs)
	})

	t.Run("no filters or search yields no conds", func(t *testing.T) {
		conds, err := g.Conds(&GridQuery{SortFields: map[string]string{"id": "asc"}, Limit: 5})
		require.NoError(t, err)
		assert.Empty(t, conds)
	})

	t.Run("filter func result is used", func(t *testing.T) {
		gf, err := NewGrid[row]()
		require.NoError(t, err)
		gf.AddFilterFunc("id", func(v any) (any, error) { return int64(v.(float64)) * 10, nil })
		conds, err := gf.Conds(&GridQuery{FilterFields: map[string]any{"id": float64(2)}})
		require.NoError(t, err)
		_, args, err := rowBase().Where(conds...).Build(gohan.Postgres())
		require.NoError(t, err)
		assert.Equal(t, []any{int64(20)}, args)
	})

	t.Run("validates the query", func(t *testing.T) {
		_, err := g.Conds(nil)
		var ge GridError
		require.True(t, errors.As(err, &ge), "got %v", err)
		assert.Equal(t, "query", ge.Scope)

		_, err = g.Conds(&GridQuery{FilterFields: map[string]any{"note": "x"}})
		require.True(t, errors.As(err, &ge), "got %v", err)
		assert.Equal(t, GridError{Scope: "filter", Field: "note", Message: "field is not valid"}, ge)

		// an invalid sort is not part of the conds but still rejects the query
		_, err = g.Conds(&GridQuery{SortFields: map[string]string{"tag": "asc"}})
		require.True(t, errors.As(err, &ge), "got %v", err)
		assert.Equal(t, "sort", ge.Scope)
	})

	t.Run("reports configuration errors", func(t *testing.T) {
		gb, err := NewGrid[row]()
		require.NoError(t, err)
		_, err = gb.WithTiebreaker("bogus").Conds(&GridQuery{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "tiebreaker")
	})
}

// gridCountQuery is the GridQuery used by the QueryGridWithCount goldens:
// filter + search + sort + a non-zero offset.
func gridCountQuery() *GridQuery {
	return &GridQuery{
		FilterFields: map[string]any{"email": "bob@x.com"},
		SearchType:   SearchAny,
		SearchText:   "b",
		Sort:         []SortField{{Field: "name", Order: SortAscending}},
		Limit:        10,
		Offset:       20,
	}
}

func TestRepositoryQueryGridWithCount(t *testing.T) {
	q, mock := newMockQuerier(t)
	r, err := NewRepository[gridUser](q, "users")
	require.NoError(t, err)
	g, err := NewGrid[gridUser]()
	require.NoError(t, err)

	mock.ExpectQuery(`SELECT "id", "name", "email" FROM "users" WHERE ("email" = $1 AND "name" LIKE $2 ESCAPE '!') ORDER BY "name" ASC LIMIT 10 OFFSET 20`).
		WithArgs("bob@x.com", "%b%").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "email"}).AddRow(int64(21), "Bob", "bob@x.com"))
	mock.ExpectQuery(`SELECT COUNT(*) FROM "users" WHERE ("email" = $1 AND "name" LIKE $2 ESCAPE '!')`).
		WithArgs("bob@x.com", "%b%").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(42)))

	rows, total, err := r.QueryGridWithCount(context.Background(), g, gridCountQuery())
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, int64(21), rows[0].ID)
	assert.Equal(t, int64(42), total)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRepositoryQueryGridWithCountSQL(t *testing.T) {
	cases := []struct {
		name      string
		d         gohan.Dialect
		wantRows  string
		wantCount string
	}{
		{
			name:      "postgres",
			d:         gohan.Postgres(),
			wantRows:  `SELECT "id", "name", "email" FROM "users" WHERE ("email" = $1 AND "name" LIKE $2 ESCAPE '!') ORDER BY "name" ASC LIMIT 10 OFFSET 20`,
			wantCount: `SELECT COUNT(*) FROM "users" WHERE ("email" = $1 AND "name" LIKE $2 ESCAPE '!')`,
		},
		{
			name:      "sqlite",
			d:         gohan.SQLite(),
			wantRows:  "SELECT `id`, `name`, `email` FROM `users` WHERE (`email` = ? AND `name` LIKE ? ESCAPE '!') ORDER BY `name` ASC LIMIT 10 OFFSET 20",
			wantCount: "SELECT COUNT(*) FROM `users` WHERE (`email` = ? AND `name` LIKE ? ESCAPE '!')",
		},
		{
			name:      "clickhouse",
			d:         gohan.ClickHouseNamed(),
			wantRows:  `SELECT "id", "name", "email" FROM "users" WHERE ("email" = @p1 AND "name" LIKE @p2) ORDER BY "name" ASC LIMIT 10 OFFSET 20`,
			wantCount: `SELECT COUNT(*) FROM "users" WHERE ("email" = @p1 AND "name" LIKE @p2)`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rq := &recordingQuerier{d: tc.d, count: 7}
			r, err := NewRepository[gridUser](rq, "users")
			require.NoError(t, err)
			g, err := NewGrid[gridUser]()
			require.NoError(t, err)

			rows, total, err := r.QueryGridWithCount(context.Background(), g, gridCountQuery())
			require.NoError(t, err)
			assert.NotNil(t, rows)
			assert.Equal(t, int64(7), total)

			require.Len(t, rq.calls, 2)
			assert.Equal(t, "Select", rq.calls[0].method)
			assert.Equal(t, tc.wantRows, rq.calls[0].sql)
			assert.Equal(t, "QueryInt64", rq.calls[1].method)
			assert.Equal(t, tc.wantCount, rq.calls[1].sql)
			assert.Equal(t, rq.calls[0].args, rq.calls[1].args)
			for _, kw := range []string{"ORDER BY", "LIMIT", "OFFSET"} {
				assert.NotContains(t, rq.calls[1].sql, kw)
			}
		})
	}
}

func TestRepositoryQueryGridWithCountNoConds(t *testing.T) {
	rq := &recordingQuerier{d: gohan.Postgres(), count: 3}
	r, err := NewRepository[gridUser](rq, "users")
	require.NoError(t, err)
	g, err := NewGrid[gridUser]()
	require.NoError(t, err)

	_, total, err := r.QueryGridWithCount(context.Background(), g, &GridQuery{
		SortFields: map[string]string{"id": "desc"},
		Offset:     5,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(3), total)
	require.Len(t, rq.calls, 2)
	assert.Equal(t, `SELECT "id", "name", "email" FROM "users" ORDER BY "id" DESC LIMIT 1000 OFFSET 5`, rq.calls[0].sql)
	assert.Equal(t, `SELECT COUNT(*) FROM "users"`, rq.calls[1].sql)
	assert.Empty(t, rq.calls[1].args)
}

func TestRepositoryQueryGridWithCountInvalidQueryNeverHitsDB(t *testing.T) {
	cq := &countingQuerier{d: gohan.Postgres()}
	r, err := NewRepository[gridUser](cq, "users")
	require.NoError(t, err)
	g, err := NewGrid[gridUser]()
	require.NoError(t, err)

	rows, total, err := r.QueryGridWithCount(context.Background(), g, &GridQuery{FilterFields: map[string]any{"bogus": "x"}})
	var ge GridError
	require.True(t, errors.As(err, &ge), "got %v", err)
	assert.Nil(t, rows)
	assert.Zero(t, total)
	assert.Equal(t, 0, cq.calls)

	_, _, err = r.QueryGridWithCount(context.Background(), g, nil)
	require.True(t, errors.As(err, &ge), "got %v", err)
	assert.Equal(t, 0, cq.calls)
}

func TestRepositoryQueryGridWithCountRowsErrorSkipsCount(t *testing.T) {
	q, mock := newMockQuerier(t)
	r, err := NewRepository[gridUser](q, "users")
	require.NoError(t, err)
	g, err := NewGrid[gridUser]()
	require.NoError(t, err)

	boom := errors.New("boom")
	mock.ExpectQuery(`SELECT "id", "name", "email" FROM "users" LIMIT 1000 OFFSET 0`).WillReturnError(boom)

	rows, total, err := r.QueryGridWithCount(context.Background(), g, &GridQuery{})
	assert.True(t, errors.Is(err, boom), "got %v", err)
	assert.Nil(t, rows)
	assert.Zero(t, total)
	require.NoError(t, mock.ExpectationsWereMet())
}

// --- W3: InsertIgnore ---

func TestRepositoryInsertIgnore(t *testing.T) {
	cases := []struct {
		name     string
		conflict []string
		affected int64
		wantSQL  string
		want     bool
	}{
		{"targeted, inserted", []string{"email"}, 1, `INSERT INTO "users" ("name", "email") VALUES ($1, $2) ON CONFLICT ("email") DO NOTHING`, true},
		{"targeted, ignored", []string{"email"}, 0, `INSERT INTO "users" ("name", "email") VALUES ($1, $2) ON CONFLICT ("email") DO NOTHING`, false},
		{"untargeted, inserted", nil, 1, `INSERT INTO "users" ("name", "email") VALUES ($1, $2) ON CONFLICT DO NOTHING`, true},
		{"untargeted, ignored", nil, 0, `INSERT INTO "users" ("name", "email") VALUES ($1, $2) ON CONFLICT DO NOTHING`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			q, mock := newMockQuerier(t)
			r, err := NewRepository[user](q, "users")
			require.NoError(t, err)

			mock.ExpectExec(tc.wantSQL).
				WithArgs("Bob", "bob@x.com").
				WillReturnResult(sqlmock.NewResult(0, tc.affected))

			ok, err := r.InsertIgnore(context.Background(), &user{Name: "Bob", Email: "bob@x.com"}, tc.conflict...)
			require.NoError(t, err)
			assert.Equal(t, tc.want, ok)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestRepositoryInsertIgnoreSQLite(t *testing.T) {
	rq := &recordingQuerier{d: gohan.SQLite()}
	r, err := NewRepository[user](rq, "users")
	require.NoError(t, err)

	_, err = r.InsertIgnore(context.Background(), &user{Name: "Bob", Email: "bob@x.com"}, "name", "email")
	require.NoError(t, err)
	_, err = r.InsertIgnore(context.Background(), &user{Name: "Bob", Email: "bob@x.com"})
	require.NoError(t, err)

	require.Len(t, rq.calls, 2)
	assert.Equal(t, "INSERT INTO `users` (`name`, `email`) VALUES (?, ?) ON CONFLICT (`name`, `email`) DO NOTHING", rq.calls[0].sql)
	assert.Equal(t, "INSERT INTO `users` (`name`, `email`) VALUES (?, ?) ON CONFLICT DO NOTHING", rq.calls[1].sql)
	assert.Equal(t, []any{"Bob", "bob@x.com"}, rq.calls[0].args)
}

func TestRepositoryInsertIgnoreRejects(t *testing.T) {
	t.Run("unknown conflict column", func(t *testing.T) {
		cq := &countingQuerier{d: gohan.Postgres()}
		r, err := NewRepository[user](cq, "users")
		require.NoError(t, err)
		ok, err := r.InsertIgnore(context.Background(), &user{Name: "a", Email: "b"}, "bogus")
		assert.False(t, ok)
		assert.True(t, errors.Is(err, ErrUnknownColumn), "got %v", err)
		assert.Equal(t, 0, cq.calls)
	})
	for _, d := range []gohan.Dialect{gohan.ClickHouse(), gohan.ClickHouseNamed(), gohan.Generic()} {
		t.Run(d.Name()+" unsupported", func(t *testing.T) {
			cq := &countingQuerier{d: d}
			r, err := NewRepository[user](cq, "users")
			require.NoError(t, err)
			ok, err := r.InsertIgnore(context.Background(), &user{Name: "a", Email: "b"}, "email")
			assert.False(t, ok)
			assert.True(t, errors.Is(err, gohan.ErrUnsupported), "got %v", err)
			assert.Equal(t, 0, cq.calls)
		})
	}
	t.Run("exec error", func(t *testing.T) {
		q, mock := newMockQuerier(t)
		r, err := NewRepository[user](q, "users")
		require.NoError(t, err)
		boom := errors.New("boom")
		mock.ExpectExec(`INSERT INTO "users" ("name", "email") VALUES ($1, $2) ON CONFLICT ("email") DO NOTHING`).
			WillReturnError(boom)
		ok, err := r.InsertIgnore(context.Background(), &user{Name: "a", Email: "b"}, "email")
		assert.False(t, ok)
		assert.True(t, errors.Is(err, boom), "got %v", err)
	})
}

// --- W4: UpdateReturning / UpsertReturning ---

func TestRepositoryUpdateReturning(t *testing.T) {
	q, mock := newMockQuerier(t)
	r, err := NewRepository[user](q, "users")
	require.NoError(t, err)

	mock.ExpectQuery(`UPDATE "users" SET "name" = $1 WHERE "email" LIKE $2 ESCAPE '!' RETURNING "id", "name", "email"`).
		WithArgs("x", "%@x.com").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "email"}).
			AddRow(int64(1), "x", "a@x.com").
			AddRow(int64(2), "x", "b@x.com"))

	got, err := r.UpdateReturning(context.Background(), map[string]any{"name": "x"}, gohan.Col("email").HasSuffix("@x.com"))
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, user{ID: 1, Name: "x", Email: "a@x.com"}, *got[0])
	assert.Equal(t, user{ID: 2, Name: "x", Email: "b@x.com"}, *got[1])
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRepositoryUpdateReturningNoMatch(t *testing.T) {
	q, mock := newMockQuerier(t)
	r, err := NewRepository[user](q, "users")
	require.NoError(t, err)

	mock.ExpectQuery(`UPDATE "users" SET "name" = $1 WHERE "id" = $2 RETURNING "id", "name", "email"`).
		WithArgs("x", int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "email"}))

	got, err := r.UpdateReturning(context.Background(), map[string]any{"name": "x"}, gohan.Col("id").Eq(int64(9)))
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Empty(t, got)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRepositoryUpdateReturningSQLite(t *testing.T) {
	rq := &recordingQuerier{d: gohan.SQLite()}
	r, err := NewRepository[user](rq, "users")
	require.NoError(t, err)

	got, err := r.UpdateReturning(context.Background(), map[string]any{"name": "x", "email": "e"}, gohan.Col("id").Eq(int64(1)))
	require.NoError(t, err)
	assert.NotNil(t, got)
	require.Len(t, rq.calls, 1)
	assert.Equal(t, "Select", rq.calls[0].method)
	assert.Equal(t, "UPDATE `users` SET `email` = ?, `name` = ? WHERE `id` = ? RETURNING `id`, `name`, `email`", rq.calls[0].sql)
	assert.Equal(t, []any{"e", "x", int64(1)}, rq.calls[0].args)
}

func TestRepositoryUpdateReturningRejects(t *testing.T) {
	newRepo := func(t *testing.T, d gohan.Dialect) (*Repository[user], *countingQuerier) {
		cq := &countingQuerier{d: d}
		r, err := NewRepository[user](cq, "users")
		require.NoError(t, err)
		return r, cq
	}
	ctx := context.Background()

	cases := []struct {
		name    string
		d       gohan.Dialect
		fields  map[string]any
		where   gohan.Expr
		wantErr error
	}{
		{"nil where", gohan.Postgres(), map[string]any{"name": "x"}, nil, gohan.ErrNoWhere},
		{"trivial where", gohan.Postgres(), map[string]any{"name": "x"}, gohan.And(), gohan.ErrNoWhere},
		{"unknown column", gohan.Postgres(), map[string]any{"bogus": "x"}, gohan.Col("id").Eq(1), ErrUnknownColumn},
		{"clickhouse", gohan.ClickHouseNamed(), map[string]any{"name": "x"}, gohan.Col("id").Eq(1), gohan.ErrUnsupported},
		{"generic has no RETURNING", gohan.Generic(), map[string]any{"name": "x"}, gohan.Col("id").Eq(1), gohan.ErrUnsupported},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, cq := newRepo(t, tc.d)
			got, err := r.UpdateReturning(ctx, tc.fields, tc.where)
			assert.Nil(t, got)
			assert.True(t, errors.Is(err, tc.wantErr), "got %v", err)
			assert.Equal(t, 0, cq.calls)
		})
	}
}

func TestRepositoryUpsertReturning(t *testing.T) {
	q, mock := newMockQuerier(t)
	r, err := NewRepository[user](q, "users")
	require.NoError(t, err)

	mock.ExpectQuery(`INSERT INTO "users" ("name", "email") VALUES ($1, $2) ON CONFLICT ("email") DO UPDATE SET "name" = excluded."name" RETURNING "id", "name", "email"`).
		WithArgs("Bob", "bob@x.com").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "email"}).AddRow(int64(5), "Bob", "bob@x.com"))

	rec := &user{Name: "Bob", Email: "bob@x.com"}
	got, err := r.UpsertReturning(context.Background(), rec, []string{"email"})
	require.NoError(t, err)
	assert.Equal(t, user{ID: 5, Name: "Bob", Email: "bob@x.com"}, *got)
	assert.NotSame(t, rec, got)
	assert.Zero(t, rec.ID, "caller's record must not be written")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRepositoryUpsertReturningExplicitUpdate(t *testing.T) {
	q, mock := newMockQuerier(t)
	r, err := NewRepository[userOpt](q, "users")
	require.NoError(t, err)

	mock.ExpectQuery(`INSERT INTO "users" ("name", "email", "bio") VALUES ($1, $2, $3) ON CONFLICT ("email") DO UPDATE SET "bio" = excluded."bio" RETURNING "id", "name", "email", "bio"`).
		WithArgs("Bob", "bob@x.com", "hi").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "email", "bio"}).AddRow(int64(5), "Old", "bob@x.com", "hi"))

	got, err := r.UpsertReturning(context.Background(), &userOpt{Name: "Bob", Email: "bob@x.com", Bio: strPtr("hi")}, []string{"email"}, "bio")
	require.NoError(t, err)
	assert.Equal(t, "Old", got.Name)
	require.NotNil(t, got.Bio)
	assert.Equal(t, "hi", *got.Bio)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRepositoryUpsertReturningDoNothingIsNotFound(t *testing.T) {
	q, mock := newMockQuerier(t)
	r, err := NewRepository[user](q, "users")
	require.NoError(t, err)

	// every written column is a conflict column, so the statement is DO
	// NOTHING, and a conflicting row returns nothing.
	mock.ExpectQuery(`INSERT INTO "users" ("name", "email") VALUES ($1, $2) ON CONFLICT ("name", "email") DO NOTHING RETURNING "id", "name", "email"`).
		WithArgs("Bob", "bob@x.com").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "email"}))

	got, err := r.UpsertReturning(context.Background(), &user{Name: "Bob", Email: "bob@x.com"}, []string{"name", "email"})
	assert.Nil(t, got)
	assert.True(t, errors.Is(err, ErrNotFound), "got %v", err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRepositoryUpsertReturningScanErrorLeavesRecord(t *testing.T) {
	q, mock := newMockQuerier(t)
	r, err := NewRepository[user](q, "users")
	require.NoError(t, err)

	mock.ExpectQuery(`INSERT INTO "users" ("name", "email") VALUES ($1, $2) ON CONFLICT ("email") DO UPDATE SET "name" = excluded."name" RETURNING "id", "name", "email"`).
		WithArgs("Bob", "bob@x.com").
		// the bad value (NULL into a string) is in the last column: database/sql
		// stops at the first failing column, so earlier columns are already
		// scanned into the destination when the error surfaces.
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "email"}).AddRow(int64(7), "Mallory", nil))

	rec := &user{ID: 3, Name: "Bob", Email: "bob@x.com"}
	got, err := r.UpsertReturning(context.Background(), rec, []string{"email"})
	require.Error(t, err)
	assert.Nil(t, got)
	assert.Equal(t, user{ID: 3, Name: "Bob", Email: "bob@x.com"}, *rec)
}

func TestRepositoryUpsertReturningSQLite(t *testing.T) {
	rq := &recordingQuerier{d: gohan.SQLite()}
	r, err := NewRepository[user](rq, "users")
	require.NoError(t, err)

	got, err := r.UpsertReturning(context.Background(), &user{Name: "Bob", Email: "bob@x.com"}, []string{"email"})
	require.NoError(t, err)
	assert.NotNil(t, got)
	require.Len(t, rq.calls, 1)
	assert.Equal(t, "Get", rq.calls[0].method)
	assert.Equal(t, "INSERT INTO `users` (`name`, `email`) VALUES (?, ?) ON CONFLICT (`email`) DO UPDATE SET `name` = excluded.`name` RETURNING `id`, `name`, `email`", rq.calls[0].sql)
}

func TestRepositoryUpsertReturningRejects(t *testing.T) {
	cases := []struct {
		name     string
		d        gohan.Dialect
		conflict []string
		update   []string
		wantErr  error
	}{
		{"unknown conflict column", gohan.Postgres(), []string{"bogus"}, nil, ErrUnknownColumn},
		{"unknown update column", gohan.Postgres(), []string{"email"}, []string{"bogus"}, ErrUnknownColumn},
		{"clickhouse", gohan.ClickHouseNamed(), []string{"email"}, nil, gohan.ErrUnsupported},
		{"generic", gohan.Generic(), []string{"email"}, nil, gohan.ErrUnsupported},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cq := &countingQuerier{d: tc.d}
			r, err := NewRepository[user](cq, "users")
			require.NoError(t, err)
			got, err := r.UpsertReturning(context.Background(), &user{Name: "a", Email: "b"}, tc.conflict, tc.update...)
			assert.Nil(t, got)
			assert.True(t, errors.Is(err, tc.wantErr), "got %v", err)
			assert.Equal(t, 0, cq.calls)
		})
	}
}

// --- W7: grouped inserts ---

func TestRepositoryInsertMixedOmitDefaultStillFails(t *testing.T) {
	cq := &countingQuerier{d: gohan.Postgres()}
	r, err := NewRepository[userOpt](cq, "users")
	require.NoError(t, err)

	err = r.Insert(context.Background(),
		&userOpt{Name: "a", Email: "a@x.com"},
		&userOpt{Name: "b", Email: "b@x.com", Bio: strPtr("hi")})
	assert.True(t, errors.Is(err, gohan.ErrInconsistentOmit), "got %v", err)
	assert.Equal(t, 0, cq.calls)

	// WithGroupedInserts returns a new repository; the original keeps the
	// default behaviour.
	g := r.WithGroupedInserts()
	require.NotSame(t, r, g)
	err = r.Insert(context.Background(),
		&userOpt{Name: "a", Email: "a@x.com"},
		&userOpt{Name: "b", Email: "b@x.com", Bio: strPtr("hi")})
	assert.True(t, errors.Is(err, gohan.ErrInconsistentOmit), "got %v", err)

	// With keeps the option.
	assert.True(t, g.With(cq).grouped)
	assert.False(t, r.With(cq).grouped)
}

func TestRepositoryInsertGrouped(t *testing.T) {
	q, mock := newMockQuerier(t)
	r, err := NewRepository[userOpt](q, "users")
	require.NoError(t, err)
	r = r.WithGroupedInserts()

	mock.ExpectBegin()
	mock.ExpectExec(`INSERT INTO "users" ("name", "email") VALUES ($1, $2), ($3, $4)`).
		WithArgs("a", "a@x.com", "c", "c@x.com").
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec(`INSERT INTO "users" ("name", "email", "bio") VALUES ($1, $2, $3)`).
		WithArgs("b", "b@x.com", "hi").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err = r.Insert(context.Background(),
		&userOpt{Name: "a", Email: "a@x.com"},
		&userOpt{Name: "b", Email: "b@x.com", Bio: strPtr("hi")},
		&userOpt{Name: "c", Email: "c@x.com"})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRepositoryInsertGroupedRollsBack(t *testing.T) {
	q, mock := newMockQuerier(t)
	r, err := NewRepository[userOpt](q, "users")
	require.NoError(t, err)
	r = r.WithGroupedInserts()

	boom := errors.New("boom")
	mock.ExpectBegin()
	mock.ExpectExec(`INSERT INTO "users" ("name", "email") VALUES ($1, $2)`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO "users" ("name", "email", "bio") VALUES ($1, $2, $3)`).
		WillReturnError(boom)
	mock.ExpectRollback()

	err = r.Insert(context.Background(),
		&userOpt{Name: "a", Email: "a@x.com"},
		&userOpt{Name: "b", Email: "b@x.com", Bio: strPtr("hi")})
	assert.True(t, errors.Is(err, boom), "got %v", err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRepositoryInsertGroupedSingleGroupNoTx(t *testing.T) {
	q, mock := newMockQuerier(t)
	r, err := NewRepository[userOpt](q, "users")
	require.NoError(t, err)
	r = r.WithGroupedInserts()

	mock.ExpectExec(`INSERT INTO "users" ("name", "email") VALUES ($1, $2), ($3, $4)`).
		WithArgs("a", "a@x.com", "b", "b@x.com").
		WillReturnResult(sqlmock.NewResult(0, 2))

	err = r.Insert(context.Background(),
		&userOpt{Name: "a", Email: "a@x.com"},
		&userOpt{Name: "b", Email: "b@x.com"})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRepositoryInsertGroupedDefaultValues(t *testing.T) {
	t.Run("single empty record", func(t *testing.T) {
		q, mock := newMockQuerier(t)
		r, err := NewRepository[allOmit](q, "notes")
		require.NoError(t, err)

		mock.ExpectExec(`INSERT INTO "notes" DEFAULT VALUES`).
			WillReturnResult(sqlmock.NewResult(0, 1))

		require.NoError(t, r.WithGroupedInserts().Insert(context.Background(), &allOmit{}))
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("one statement per empty record, in a transaction", func(t *testing.T) {
		q, mock := newMockQuerier(t)
		r, err := NewRepository[allOmit](q, "notes")
		require.NoError(t, err)

		// the empty records' group comes first (it has the first record),
		// one statement per record.
		mock.ExpectBegin()
		mock.ExpectExec(`INSERT INTO "notes" DEFAULT VALUES`).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(`INSERT INTO "notes" DEFAULT VALUES`).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(`INSERT INTO "notes" ("note") VALUES ($1)`).
			WithArgs("n").
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()

		require.NoError(t, r.WithGroupedInserts().Insert(context.Background(), &allOmit{}, &allOmit{Note: strPtr("n")}, &allOmit{}))
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("default Insert still rejects an empty record", func(t *testing.T) {
		cq := &countingQuerier{d: gohan.Postgres()}
		r, err := NewRepository[allOmit](cq, "notes")
		require.NoError(t, err)
		err = r.Insert(context.Background(), &allOmit{})
		assert.True(t, errors.Is(err, gohan.ErrNoColumns), "got %v", err)
		assert.Equal(t, 0, cq.calls)
	})

	t.Run("clickhouse has no DEFAULT VALUES", func(t *testing.T) {
		cq := &countingQuerier{d: gohan.ClickHouseNamed()}
		r, err := NewRepository[allOmit](cq, "notes")
		require.NoError(t, err)
		err = r.WithGroupedInserts().Insert(context.Background(), &allOmit{})
		assert.True(t, errors.Is(err, gohan.ErrUnsupported), "got %v", err)
		assert.Equal(t, 0, cq.calls)
	})
}

func TestRepositoryInsertGroupedChunked(t *testing.T) {
	mockDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer func() { _ = mockDB.Close() }()
	q := NewSQL(sqlx.NewDb(mockDB, "sqlmock"), gohan.Generic())
	r, err := NewRepository[userOpt](q, "users")
	require.NoError(t, err)

	// Generic: MaxArgs 999 / 4 columns (auto id included) = 249 per chunk.
	records := make([]*userOpt, 600)
	for i := range records {
		records[i] = &userOpt{Name: "n", Email: "e"}
		if i%2 == 1 {
			records[i].Bio = strPtr("b")
		}
	}
	two := []string{"name", "email"}
	three := []string{"name", "email", "bio"}

	mock.ExpectBegin()
	for _, cols := range [][]string{two, three} {
		for _, n := range []int{249, 51} {
			mock.ExpectExec(chunkInsertSQL("users", cols, n)).
				WithArgs(repeatAnyArg(n * len(cols))...).
				WillReturnResult(sqlmock.NewResult(0, int64(n)))
		}
	}
	mock.ExpectCommit()

	require.NoError(t, r.WithGroupedInserts().Insert(context.Background(), records...))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRepositoryInsertGroupedRejects(t *testing.T) {
	t.Run("several groups without a transaction", func(t *testing.T) {
		cq := &countingQuerier{d: gohan.Postgres()}
		r, err := NewRepository[userOpt](cq, "users")
		require.NoError(t, err)
		err = r.WithGroupedInserts().Insert(context.Background(),
			&userOpt{Name: "a", Email: "a@x.com"},
			&userOpt{Name: "b", Email: "b@x.com", Bio: strPtr("hi")})
		assert.True(t, errors.Is(err, ErrTxUnsupported), "got %v", err)
		assert.Equal(t, 0, cq.calls)
	})

	t.Run("nil record", func(t *testing.T) {
		cq := &countingQuerier{d: gohan.Postgres()}
		r, err := NewRepository[userOpt](cq, "users")
		require.NoError(t, err)
		err = r.WithGroupedInserts().Insert(context.Background(), &userOpt{Name: "a"}, nil)
		assert.True(t, errors.Is(err, gohan.ErrInvalidRecord), "got %v", err)
		assert.Equal(t, 0, cq.calls)
	})

	t.Run("batch inserter still gets every row", func(t *testing.T) {
		bi := &batchInserterDouble{countingQuerier: countingQuerier{d: gohan.ClickHouseNamed()}}
		r, err := NewRepository[userOpt](bi, "users")
		require.NoError(t, err)
		a := &userOpt{Name: "a", Email: "a@x.com"}
		b := &userOpt{Name: "b", Email: "b@x.com", Bio: strPtr("hi")}
		require.NoError(t, r.WithGroupedInserts().Insert(context.Background(), a, b))
		assert.Equal(t, 1, bi.batchCalls)
		assert.Equal(t, []any{a, b}, bi.gotRows)
		assert.Equal(t, 0, bi.calls)
	})
}
