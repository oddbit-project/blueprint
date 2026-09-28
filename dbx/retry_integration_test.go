package dbx

import (
	"database/sql"
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWithTxRetrySerializationFailure provokes a real SQLSTATE 40001: the
// first attempt reads a row, a concurrent serializable transaction then
// commits an update to that row, and the attempt's own update fails with
// "could not serialize access". WithTxRetry must retry in a fresh
// transaction, which sees the concurrent write and succeeds.
func (s *DbxIntegrationSuite) TestWithTxRetrySerializationFailure() {
	t := s.T()
	_, err := s.adminClient.Conn.ExecContext(s.ctx, `create table retry_counter(id int primary key, n int not null)`)
	require.NoError(t, err)
	defer func() { _, _ = s.adminClient.Conn.ExecContext(s.ctx, `drop table retry_counter`) }()
	_, err = s.adminClient.Conn.ExecContext(s.ctx, `insert into retry_counter(id, n) values (1, 0)`)
	require.NoError(t, err)

	q := s.newQuerier(s.dsn)
	serializable := &sql.TxOptions{Isolation: sql.LevelSerializable}

	attempts := 0
	var firstErr error
	err = WithTxRetry(s.ctx, q, serializable, 3, func(tx Querier) error {
		attempts++
		n, err := tx.QueryInt64(s.ctx, `select n from retry_counter where id = 1`)
		if err != nil {
			return err
		}
		if attempts == 1 {
			other, err := s.adminClient.Conn.BeginTxx(s.ctx, serializable)
			require.NoError(t, err)
			_, err = other.ExecContext(s.ctx, `update retry_counter set n = n + 100 where id = 1`)
			require.NoError(t, err)
			require.NoError(t, other.Commit())
		}
		_, err = tx.Exec(s.ctx, `update retry_counter set n = $1 where id = 1`, n+1)
		if attempts == 1 {
			firstErr = err
		}
		return err
	})
	require.NoError(t, err)
	assert.Equal(t, 2, attempts)

	var pgErr *pgconn.PgError
	require.True(t, errors.As(firstErr, &pgErr), "first attempt must fail with a PostgreSQL error, got %v", firstErr)
	assert.Equal(t, "40001", pgErr.Code)

	var n int64
	require.NoError(t, s.adminClient.Conn.GetContext(s.ctx, &n, `select n from retry_counter where id = 1`))
	assert.Equal(t, int64(101), n, "the retry saw the concurrent write")
}
