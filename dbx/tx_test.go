package dbx

import (
	"context"
	"errors"
	"runtime"
	"sync"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/oddbit-project/blueprint/sqlb"
)

func TestWithTxCommit(t *testing.T) {
	q, mock := newMockQuerier(t)
	mock.ExpectBegin()
	mock.ExpectExec(`DELETE FROM "users"`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err := WithTx(context.Background(), q, nil, func(tx Querier) error {
		_, err := tx.Exec(context.Background(), `DELETE FROM "users"`)
		return err
	})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestWithTxRollbackOnError(t *testing.T) {
	q, mock := newMockQuerier(t)
	mock.ExpectBegin()
	mock.ExpectRollback()

	fnErr := errors.New("boom")
	err := WithTx(context.Background(), q, nil, func(tx Querier) error {
		return fnErr
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, fnErr))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestWithTxRollbackOnPanic(t *testing.T) {
	q, mock := newMockQuerier(t)
	mock.ExpectBegin()
	mock.ExpectRollback()

	assert.Panics(t, func() {
		_ = WithTx(context.Background(), q, nil, func(tx Querier) error {
			panic("boom")
		})
	})
	require.NoError(t, mock.ExpectationsWereMet())
}

// fakeQuerier is a Querier double that is neither a TxQuerier nor a
// TxBeginner, used to prove WithTx fails with ErrTxUnsupported instead of
// silently proceeding.
type fakeQuerier struct {
	calls int
}

func (f *fakeQuerier) Dialect() sqlb.Dialect { return sqlb.Postgres() }
func (f *fakeQuerier) Exec(ctx context.Context, query string, args ...any) (int64, error) {
	f.calls++
	return 0, nil
}
func (f *fakeQuerier) Get(ctx context.Context, dest any, query string, args ...any) error {
	f.calls++
	return nil
}
func (f *fakeQuerier) Select(ctx context.Context, dest any, query string, args ...any) error {
	f.calls++
	return nil
}
func (f *fakeQuerier) QueryInt64(ctx context.Context, query string, args ...any) (int64, error) {
	f.calls++
	return 0, nil
}

var _ Querier = (*fakeQuerier)(nil)

func TestWithTxUnsupported(t *testing.T) {
	q := &fakeQuerier{}
	err := WithTx(context.Background(), q, nil, func(tx Querier) error {
		q.calls++
		return nil
	})
	require.True(t, errors.Is(err, ErrTxUnsupported))
	assert.Equal(t, 0, q.calls)
}

func TestWithTxJoinsExisting(t *testing.T) {
	q, mock := newMockQuerier(t)
	mock.ExpectBegin()
	mock.ExpectExec(`DELETE FROM "users"`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err := WithTx(context.Background(), q, nil, func(outer Querier) error {
		// Nested WithTx over an already-open transaction must not issue
		// its own BEGIN/COMMIT.
		return WithTx(context.Background(), outer, nil, func(inner Querier) error {
			_, err := inner.Exec(context.Background(), `DELETE FROM "users"`)
			return err
		})
	})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestWithTxCommitError(t *testing.T) {
	q, mock := newMockQuerier(t)
	mock.ExpectBegin()
	mock.ExpectCommit().WillReturnError(errors.New("commit failed"))

	err := WithTx(context.Background(), q, nil, func(tx Querier) error {
		return nil
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "commit failed")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestWithTxRollbackFailsToo(t *testing.T) {
	q, mock := newMockQuerier(t)
	mock.ExpectBegin()
	rollbackErr := errors.New("rollback failed")
	mock.ExpectRollback().WillReturnError(rollbackErr)

	fnErr := errors.New("boom")
	err := WithTx(context.Background(), q, nil, func(tx Querier) error {
		return fnErr
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, fnErr))
	assert.True(t, errors.Is(err, rollbackErr))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestWithTxGoexit(t *testing.T) {
	q, mock := newMockQuerier(t)
	mock.ExpectBegin()
	mock.ExpectRollback()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = WithTx(context.Background(), q, nil, func(tx Querier) error {
			runtime.Goexit()
			return nil
		})
	}()
	wg.Wait()
	require.NoError(t, mock.ExpectationsWereMet())
}
