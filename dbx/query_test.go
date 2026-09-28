package dbx

import (
	"context"
	"errors"
	"reflect"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/oddbit-project/gohan"
)

// userPostCount is a projection DTO: no table of its own, filled from a
// join + aggregate.
type userPostCount struct {
	Name  string `db:"name"`
	Posts int64  `db:"posts"`
}

func userPostCountQuery() *gohan.SelectBuilder {
	return gohan.Select(gohan.Col("u.name"), gohan.Count(gohan.Col("p.id")).As("posts")).
		From(gohan.Table("users").As("u")).
		Join(gohan.Table("posts").As("p"), gohan.Col("p.user_id").Eq(gohan.Col("u.id"))).
		GroupBy(gohan.Col("u.name")).
		OrderBy(gohan.Col("u.name").Asc())
}

const userPostCountSQL = `SELECT "u"."name", COUNT("p"."id") AS "posts" FROM "users" AS "u" INNER JOIN "posts" AS "p" ON "p"."user_id" = "u"."id" GROUP BY "u"."name" ORDER BY "u"."name" ASC`

func TestQuery(t *testing.T) {
	q, mock := newMockQuerier(t)

	mock.ExpectQuery(userPostCountSQL).
		WillReturnRows(sqlmock.NewRows([]string{"name", "posts"}).
			AddRow("alice", int64(3)).
			AddRow("bob", int64(0)))

	got, err := Query[userPostCount](context.Background(), q, userPostCountQuery())
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, userPostCount{Name: "alice", Posts: 3}, *got[0])
	assert.Equal(t, userPostCount{Name: "bob", Posts: 0}, *got[1])
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestQueryEmpty(t *testing.T) {
	q, mock := newMockQuerier(t)
	mock.ExpectQuery(userPostCountSQL).WillReturnRows(sqlmock.NewRows([]string{"name", "posts"}))

	got, err := Query[userPostCount](context.Background(), q, userPostCountQuery())
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Empty(t, got)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestQueryOne(t *testing.T) {
	q, mock := newMockQuerier(t)
	st := userPostCountQuery().Having(gohan.Count(gohan.Col("p.id")).Gt(int64(2)))

	mock.ExpectQuery(`SELECT "u"."name", COUNT("p"."id") AS "posts" FROM "users" AS "u" INNER JOIN "posts" AS "p" ON "p"."user_id" = "u"."id" GROUP BY "u"."name" HAVING COUNT("p"."id") > $1 ORDER BY "u"."name" ASC`).
		WithArgs(int64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"name", "posts"}).AddRow("alice", int64(3)))

	got, err := QueryOne[userPostCount](context.Background(), q, st)
	require.NoError(t, err)
	assert.Equal(t, userPostCount{Name: "alice", Posts: 3}, *got)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestQueryOneNotFound(t *testing.T) {
	q, mock := newMockQuerier(t)
	mock.ExpectQuery(userPostCountSQL).WillReturnRows(sqlmock.NewRows([]string{"name", "posts"}))

	got, err := QueryOne[userPostCount](context.Background(), q, userPostCountQuery())
	assert.Nil(t, got)
	assert.True(t, errors.Is(err, ErrNotFound), "got %v", err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestQueryUsesQuerierDialect(t *testing.T) {
	rq := &recordingQuerier{d: gohan.SQLite()}
	st := gohan.Select("name").From("users").Where(gohan.Col("id").Eq(int64(1)))

	_, err := Query[userPostCount](context.Background(), rq, st)
	require.NoError(t, err)
	_, err = QueryOne[userPostCount](context.Background(), rq, st)
	require.NoError(t, err)

	require.Len(t, rq.calls, 2)
	assert.Equal(t, "Select", rq.calls[0].method)
	assert.Equal(t, "Get", rq.calls[1].method)
	for _, c := range rq.calls {
		assert.Equal(t, "SELECT `name` FROM `users` WHERE `id` = ?", c.sql)
		assert.Equal(t, []any{int64(1)}, c.args)
	}
}

func TestQueryBuildErrorNeverHitsDB(t *testing.T) {
	cq := &countingQuerier{d: gohan.Postgres()}
	st := gohan.Select("name").From("bad\x00table")

	got, err := Query[userPostCount](context.Background(), cq, st)
	assert.Nil(t, got)
	assert.True(t, errors.Is(err, gohan.ErrInvalidIdentifier), "got %v", err)

	one, err := QueryOne[userPostCount](context.Background(), cq, st)
	assert.Nil(t, one)
	assert.True(t, errors.Is(err, gohan.ErrInvalidIdentifier), "got %v", err)
	assert.Equal(t, 0, cq.calls)
}

// TestQueryRecordChecker checks that Query and QueryOne run a
// RecordChecker Querier's check on D before any query, so a DTO whose
// columns the Querier would scan into other fields is rejected.
func TestQueryRecordChecker(t *testing.T) {
	st := userPostCountQuery()
	dto := reflect.TypeFor[userPostCount]()

	t.Run("rejection", func(t *testing.T) {
		errBad := errors.New("bad record")
		q := &checkingQuerier{countingQuerier: countingQuerier{d: gohan.Postgres()}, err: errBad}

		got, err := Query[userPostCount](context.Background(), q, st)
		assert.Nil(t, got)
		assert.ErrorIs(t, err, errBad)

		one, err := QueryOne[userPostCount](context.Background(), q, st)
		assert.Nil(t, one)
		assert.ErrorIs(t, err, errBad)

		assert.Equal(t, []reflect.Type{dto, dto}, q.checked)
		assert.Zero(t, q.calls, "no query may be issued")
	})

	t.Run("acceptance runs the query", func(t *testing.T) {
		q := &checkingQuerier{countingQuerier: countingQuerier{d: gohan.Postgres()}}
		_, err := Query[userPostCount](context.Background(), q, st)
		require.NoError(t, err)
		_, err = QueryOne[userPostCount](context.Background(), q, st)
		require.NoError(t, err)
		assert.Equal(t, []reflect.Type{dto, dto}, q.checked)
		assert.Equal(t, 2, q.calls)
	})
}
