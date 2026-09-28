package dbx

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/oddbit-project/gohan"
)

var errRetryable = errors.New("retryable")

// retryFakeTx is a TxQuerier double counting Commit/Rollback calls.
type retryFakeTx struct {
	fakeQuerier
	commits   int
	rollbacks int
}

func (t *retryFakeTx) Commit() error   { t.commits++; return nil }
func (t *retryFakeTx) Rollback() error { t.rollbacks++; return nil }

// retryFakeDB is a TxBeginner Querier double that is not a RetryClassifier.
type retryFakeDB struct {
	fakeQuerier
	begins int
	txs    []*retryFakeTx
}

func (d *retryFakeDB) BeginTx(ctx context.Context, opts *sql.TxOptions) (TxQuerier, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	d.begins++
	tx := &retryFakeTx{}
	d.txs = append(d.txs, tx)
	return tx, nil
}

// retryClassifyingDB is a retryFakeDB that classifies errRetryable as
// retryable.
type retryClassifyingDB struct {
	retryFakeDB
}

func (d *retryClassifyingDB) IsRetryable(err error) bool { return errors.Is(err, errRetryable) }

// retryClassifyingTx is a TxQuerier that also claims every error is
// retryable, proving WithTxRetry never retries a joined transaction.
type retryClassifyingTx struct {
	retryFakeTx
}

func (t *retryClassifyingTx) IsRetryable(error) bool { return true }

var (
	_ RetryClassifier = (*retryClassifyingDB)(nil)
	_ RetryClassifier = (*SQLQuerier)(nil)
)

// noBackoff replaces retryBackoff for the duration of the test.
func noBackoff(t *testing.T) {
	t.Helper()
	prev := retryBackoff
	retryBackoff = func(int) time.Duration { return 0 }
	t.Cleanup(func() { retryBackoff = prev })
}

func TestWithTxRetry(t *testing.T) {
	otherErr := errors.New("not retryable")
	tests := []struct {
		name      string
		attempts  int
		failures  int   // attempts that fail before fn succeeds
		failErr   error // error returned by a failing attempt
		wantCalls int
		wantErr   error
	}{
		{name: "success first try", attempts: 3, failures: 0, failErr: errRetryable, wantCalls: 1},
		{name: "retryable then success", attempts: 3, failures: 2, failErr: errRetryable, wantCalls: 3},
		{name: "retryable exhausted", attempts: 3, failures: 5, failErr: errRetryable, wantCalls: 3, wantErr: errRetryable},
		{name: "non-retryable returns at once", attempts: 3, failures: 5, failErr: otherErr, wantCalls: 1, wantErr: otherErr},
		{name: "zero attempts runs once", attempts: 0, failures: 5, failErr: errRetryable, wantCalls: 1, wantErr: errRetryable},
		{name: "negative attempts runs once", attempts: -2, failures: 0, failErr: errRetryable, wantCalls: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			noBackoff(t)
			q := &retryClassifyingDB{}
			calls := 0
			err := WithTxRetry(context.Background(), q, nil, tt.attempts, func(tx Querier) error {
				calls++
				if calls <= tt.failures {
					return fmt.Errorf("attempt %d: %w", calls, tt.failErr)
				}
				return nil
			})
			assert.Equal(t, tt.wantCalls, calls)
			assert.Equal(t, tt.wantCalls, q.begins, "a fresh transaction per attempt")
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				assert.Contains(t, err.Error(), fmt.Sprintf("attempt %d", tt.wantCalls), "the last attempt's error")
			} else {
				require.NoError(t, err)
			}
			for i, tx := range q.txs {
				if i == len(q.txs)-1 && tt.wantErr == nil {
					assert.Equal(t, 1, tx.commits)
					assert.Equal(t, 0, tx.rollbacks)
				} else {
					assert.Equal(t, 0, tx.commits)
					assert.Equal(t, 1, tx.rollbacks)
				}
			}
		})
	}
}

func TestWithTxRetryNoClassifier(t *testing.T) {
	noBackoff(t)
	q := &retryFakeDB{}
	calls := 0
	err := WithTxRetry(context.Background(), q, nil, 5, func(tx Querier) error {
		calls++
		return errRetryable
	})
	require.ErrorIs(t, err, errRetryable)
	assert.Equal(t, 1, calls)
}

func TestWithTxRetryUnsupported(t *testing.T) {
	q := &fakeQuerier{}
	calls := 0
	err := WithTxRetry(context.Background(), q, nil, 5, func(tx Querier) error {
		calls++
		return nil
	})
	require.ErrorIs(t, err, ErrTxUnsupported)
	assert.Equal(t, 0, calls)
}

func TestWithTxRetryJoinedTxRunsOnce(t *testing.T) {
	noBackoff(t)
	tx := &retryClassifyingTx{}
	calls := 0
	err := WithTxRetry(context.Background(), tx, nil, 5, func(inner Querier) error {
		calls++
		assert.Same(t, tx, inner)
		return errRetryable
	})
	require.ErrorIs(t, err, errRetryable)
	assert.Equal(t, 1, calls)
	assert.Equal(t, 0, tx.commits, "the outer transaction owns commit")
	assert.Equal(t, 0, tx.rollbacks, "the outer transaction owns rollback")
}

func TestWithTxRetryJoinedSQLTxRunsOnce(t *testing.T) {
	noBackoff(t)
	q, mock := newMockQuerier(t)
	mock.ExpectBegin()
	mock.ExpectRollback()

	calls := 0
	serialization := &pgconn.PgError{Code: "40001"}
	err := WithTx(context.Background(), q, nil, func(outer Querier) error {
		return WithTxRetry(context.Background(), outer, nil, 5, func(inner Querier) error {
			calls++
			return serialization
		})
	})
	require.ErrorIs(t, err, serialization)
	assert.Equal(t, 1, calls)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestWithTxRetryContextCancelledDuringBackoff(t *testing.T) {
	prev := retryBackoff
	retryBackoff = func(int) time.Duration { return time.Hour }
	t.Cleanup(func() { retryBackoff = prev })

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	q := &retryClassifyingDB{}
	calls := 0
	done := make(chan error, 1)
	go func() {
		done <- WithTxRetry(ctx, q, nil, 5, func(tx Querier) error {
			calls++
			return errRetryable
		})
	}()
	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		require.ErrorIs(t, err, context.Canceled)
		require.ErrorIs(t, err, errRetryable, "the last attempt's error is kept")
		assert.Equal(t, 1, calls)
	case <-time.After(5 * time.Second):
		t.Fatal("WithTxRetry did not stop on context cancellation")
	}
}

// TestWithTxRetryContextAlreadyCancelled cancels ctx during the first
// attempt with a zero backoff, where the backoff timer and ctx.Done() are
// both ready; over many runs, the next attempt must never start and the
// first attempt's error must always be kept.
func TestWithTxRetryContextAlreadyCancelled(t *testing.T) {
	noBackoff(t)
	for i := 0; i < 500; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		q := &retryClassifyingDB{}
		calls := 0
		err := WithTxRetry(ctx, q, nil, 5, func(tx Querier) error {
			calls++
			cancel()
			return errRetryable
		})
		require.ErrorIs(t, err, context.Canceled, "run %d", i)
		require.ErrorIs(t, err, errRetryable, "run %d: the last attempt's error is kept", i)
		require.Equal(t, 1, calls, "run %d", i)
		require.Equal(t, 1, q.begins, "run %d: no transaction begun after cancellation", i)
	}
}

func TestRetryBackoffBounds(t *testing.T) {
	for retry := 0; retry < 70; retry++ {
		limit := retryMaxDelay
		if retry < 16 && retryBaseDelay<<retry < limit {
			limit = retryBaseDelay << retry
		}
		for range 50 {
			d := fullJitterBackoff(retry)
			require.GreaterOrEqual(t, d, time.Duration(0))
			require.Less(t, d, limit, "retry %d", retry)
		}
	}
	// The cap applies: some draws at a high retry exceed the base window.
	var over bool
	for range 200 {
		if fullJitterBackoff(10) > 4*retryBaseDelay {
			over = true
			break
		}
	}
	assert.True(t, over, "backoff grows beyond the base delay")
}

func TestSQLQuerierIsRetryable(t *testing.T) {
	pg := NewSQL(nil, gohan.Postgres())
	lite := NewSQL(nil, gohan.SQLite())
	tests := []struct {
		name string
		q    *SQLQuerier
		err  error
		want bool
	}{
		{"pgx serialization failure", pg, &pgconn.PgError{Code: "40001"}, true},
		{"pgx deadlock", pg, &pgconn.PgError{Code: "40P01"}, true},
		{"pgx wrapped", pg, fmt.Errorf("commit: %w", &pgconn.PgError{Code: "40001"}), true},
		{"pgx joined with rollback error", pg, errors.Join(errors.New("rb"), &pgconn.PgError{Code: "40P01"}), true},
		{"pgx unique violation", pg, &pgconn.PgError{Code: "23505"}, false},
		{"lib/pq serialization failure", pg, &pq.Error{Code: "40001"}, true},
		{"lib/pq deadlock", pg, &pq.Error{Code: "40P01"}, true},
		{"lib/pq other", pg, &pq.Error{Code: "42P01"}, false},
		{"plain error", pg, errors.New("40001"), false},
		{"nil", pg, nil, false},
		{"sqlite code on postgres", pg, sqliteCodeErr(5), false},
		{"sqlite busy", lite, sqliteCodeErr(5), true},
		{"sqlite locked", lite, sqliteCodeErr(6), true},
		{"sqlite busy snapshot (extended)", lite, sqliteCodeErr(5 | 2<<8), true},
		{"sqlite locked sharedcache (extended)", lite, sqliteCodeErr(6 | 1<<8), true},
		{"sqlite wrapped busy", lite, fmt.Errorf("exec: %w", sqliteCodeErr(5)), true},
		{"sqlite constraint", lite, sqliteCodeErr(19), false},
		{"sqlite constraint unique (extended)", lite, sqliteCodeErr(19 | 8<<8), false},
		{"sqlite ioerr read (extended)", lite, sqliteCodeErr(10 | 1<<8), false},
		{"postgres code on sqlite", lite, &pgconn.PgError{Code: "40001"}, false},
		{"sqlite plain error", lite, errors.New("database is locked"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.q.IsRetryable(tt.err))
		})
	}
}

// sqliteCodeErr mimics modernc.org/sqlite's *Error, which exposes its
// (possibly extended) result code through a Code() int method.
type sqliteCodeErr int

func (e sqliteCodeErr) Error() string { return fmt.Sprintf("sqlite error %d", int(e)) }
func (e sqliteCodeErr) Code() int     { return int(e) }

func TestWithTxRetrySQLQuerierCommitSerializationFailure(t *testing.T) {
	noBackoff(t)
	q, mock := newMockQuerier(t)
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "t" SET "n" = "n" + 1`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit().WillReturnError(&pgconn.PgError{Code: "40001"})
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "t" SET "n" = "n" + 1`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	calls := 0
	err := WithTxRetry(context.Background(), q, &sql.TxOptions{Isolation: sql.LevelSerializable}, 3, func(tx Querier) error {
		calls++
		_, err := tx.Exec(context.Background(), `UPDATE "t" SET "n" = "n" + 1`)
		return err
	})
	require.NoError(t, err)
	assert.Equal(t, 2, calls)
	require.NoError(t, mock.ExpectationsWereMet())
}
