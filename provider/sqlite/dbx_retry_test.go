package sqlite

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	msqlite "modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"

	"github.com/oddbit-project/blueprint/dbx"
)

// TestDbxWithTxRetryBusy provokes a real SQLITE_BUSY: a second connection
// holds the write lock (BEGIN IMMEDIATE) while the first WithTxRetry
// attempt writes. The lock is released before the attempt returns, so the
// retry, in a fresh transaction, succeeds.
func TestDbxWithTxRetryBusy(t *testing.T) {
	ctx := context.Background()
	dsn := filepath.Join(t.TempDir(), "retry.db")

	newClient := func() *dbx.SQLQuerier {
		cfg := NewClientConfig()
		cfg.DSN = dsn
		c, err := NewClient(cfg)
		require.NoError(t, err)
		q, err := dbx.FromClient(c)
		require.NoError(t, err)
		t.Cleanup(c.Disconnect)
		return q
	}
	q := newClient()
	_, err := q.Exec(ctx, `CREATE TABLE retry_items (id INTEGER PRIMARY KEY, name TEXT NOT NULL)`)
	require.NoError(t, err)

	lockCfg := NewClientConfig()
	lockCfg.DSN = dsn
	lockClient, err := NewClient(lockCfg)
	require.NoError(t, err)
	require.NoError(t, lockClient.Connect())
	t.Cleanup(lockClient.Disconnect)
	lockConn, err := lockClient.Conn.Conn(ctx)
	require.NoError(t, err)
	defer func() { _ = lockConn.Close() }()
	_, err = lockConn.ExecContext(ctx, `BEGIN IMMEDIATE`)
	require.NoError(t, err)

	attempts := 0
	var firstErr error
	err = dbx.WithTxRetry(ctx, q, nil, 3, func(tx dbx.Querier) error {
		attempts++
		_, err := tx.Exec(ctx, `INSERT INTO retry_items (name) VALUES ('a')`)
		if attempts == 1 {
			firstErr = err
			_, cErr := lockConn.ExecContext(ctx, `COMMIT`)
			require.NoError(t, cErr)
		}
		return err
	})
	require.NoError(t, err)
	assert.Equal(t, 2, attempts)

	var sErr *msqlite.Error
	require.True(t, errors.As(firstErr, &sErr), "first attempt must fail with a sqlite error, got %v", firstErr)
	assert.Equal(t, sqlite3.SQLITE_BUSY, sErr.Code()&0xff)
	assert.True(t, q.IsRetryable(firstErr))

	n, err := q.QueryInt64(ctx, `SELECT COUNT(*) FROM retry_items`)
	require.NoError(t, err)
	assert.Equal(t, int64(1), n)
}
