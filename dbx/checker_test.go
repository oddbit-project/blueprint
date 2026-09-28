package dbx

import (
	"errors"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/oddbit-project/gohan"
)

// checkingQuerier is a Querier + RecordChecker double that records every
// CheckRecord call and returns err.
type checkingQuerier struct {
	countingQuerier
	err     error
	checked []reflect.Type
}

func (c *checkingQuerier) CheckRecord(t reflect.Type) error {
	c.checked = append(c.checked, t)
	return c.err
}

var _ RecordChecker = (*checkingQuerier)(nil)

func TestNewRepositoryRecordChecker(t *testing.T) {
	t.Run("rejection fails NewRepository", func(t *testing.T) {
		errBad := errors.New("bad record")
		q := &checkingQuerier{countingQuerier: countingQuerier{d: gohan.Postgres()}, err: errBad}
		r, err := NewRepository[user](q, "users")
		assert.Nil(t, r)
		assert.True(t, errors.Is(err, errBad), "got %v", err)
		assert.Equal(t, []reflect.Type{reflect.TypeFor[user]()}, q.checked)
		assert.Zero(t, q.calls, "no query may be issued")
	})

	t.Run("acceptance builds the repository", func(t *testing.T) {
		q := &checkingQuerier{countingQuerier: countingQuerier{d: gohan.Postgres()}}
		r, err := NewRepository[user](q, "users")
		require.NoError(t, err)
		assert.Equal(t, []string{"id", "name", "email"}, r.cols)
		assert.Equal(t, []reflect.Type{reflect.TypeFor[user]()}, q.checked)
	})

	t.Run("shape errors are reported before the checker runs", func(t *testing.T) {
		q := &checkingQuerier{countingQuerier: countingQuerier{d: gohan.Postgres()}}
		_, err := NewRepository[shapePtrEmbed](q, "t")
		assert.True(t, errors.Is(err, gohan.ErrRecordShape), "got %v", err)
		assert.Empty(t, q.checked)
	})
}

func TestSQLQuerierIsNotRecordChecker(t *testing.T) {
	q, _ := newMockQuerier(t)
	var i any = q
	_, ok := i.(RecordChecker)
	assert.False(t, ok, "SQLQuerier must not implement RecordChecker")

	r, err := NewRepository[user](q, "users")
	require.NoError(t, err)
	assert.Equal(t, []string{"id", "name", "email"}, r.cols)
}
