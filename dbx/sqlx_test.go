package dbx

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/oddbit-project/blueprint/db"
	"github.com/oddbit-project/gohan"
)

func newMockQuerier(t *testing.T) (*SQLQuerier, sqlmock.Sqlmock) {
	t.Helper()
	mockDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	t.Cleanup(func() { _ = mockDB.Close() })
	conn := sqlx.NewDb(mockDB, "sqlmock")
	return NewSQL(conn, gohan.Postgres()), mock
}

func TestSQLQuerierExecRowsAffected(t *testing.T) {
	q, mock := newMockQuerier(t)
	mock.ExpectExec(`DELETE FROM "users" WHERE "id" = $1`).
		WithArgs(int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	n, err := q.Exec(context.Background(), `DELETE FROM "users" WHERE "id" = $1`, int64(7))
	require.NoError(t, err)
	assert.Equal(t, int64(1), n)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSQLQuerierGetNotFound(t *testing.T) {
	q, mock := newMockQuerier(t)
	mock.ExpectQuery(`SELECT "id" FROM "users" WHERE "id" = $1`).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	var id int64
	err := q.Get(context.Background(), &id, `SELECT "id" FROM "users" WHERE "id" = $1`, int64(7))
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrNotFound))
	assert.True(t, errors.Is(err, sql.ErrNoRows))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSQLQuerierQueryInt64(t *testing.T) {
	q, mock := newMockQuerier(t)
	mock.ExpectQuery(`SELECT COUNT(*) FROM "users"`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(3)))

	n, err := q.QueryInt64(context.Background(), `SELECT COUNT(*) FROM "users"`)
	require.NoError(t, err)
	assert.Equal(t, int64(3), n)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestFromClientUnknownDriver(t *testing.T) {
	c := db.NewSqlClient("x", "nope", nil)
	q, err := FromClient(c)
	require.Nil(t, q)
	require.True(t, errors.Is(err, gohan.ErrUnknownDialect))
	assert.False(t, c.IsConnected(), "FromClient must resolve the dialect before attempting to connect")
}

func TestFromClientClickHouseRejected(t *testing.T) {
	c := db.NewSqlClient("x", "clickhouse", nil)
	q, err := FromClient(c)
	require.Nil(t, q)
	require.True(t, errors.Is(err, ErrDialectDriver))
}
