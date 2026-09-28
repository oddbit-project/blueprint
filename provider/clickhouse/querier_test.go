package clickhouse

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	chgo "github.com/ClickHouse/clickhouse-go/v2"
	chdriver "github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/oddbit-project/blueprint/dbx"
	"github.com/oddbit-project/gohan"
)

func TestQuerierDialect(t *testing.T) {
	q := NewQuerier(nil)
	assert.Equal(t, "clickhouse-named", q.Dialect().Name())
}

func TestQuerierNotTransactional(t *testing.T) {
	q := NewQuerier(nil)
	var i any = q
	_, ok := i.(dbx.TxBeginner)
	assert.False(t, ok, "Querier must not implement dbx.TxBeginner")
	_, ok = i.(dbx.TxQuerier)
	assert.False(t, ok, "Querier must not implement dbx.TxQuerier")
}

func TestQuerierSelectRejectsNonSlice(t *testing.T) {
	q := NewQuerier(nil)
	var dest int
	err := q.Select(context.TODO(), &dest, "SELECT 1")
	assert.Error(t, err)
}

func TestInsertBatchEmpty(t *testing.T) {
	q := NewQuerier(nil)
	err := q.InsertBatch(context.TODO(), "t", nil)
	assert.NoError(t, err)
}

// Struct tags cannot be built from a variable, so each rejected column
// name gets its own named record type; rejectColumnNameCases maps each
// case's name to a func that runs InsertBatch with that type and returns
// its error.
func rejectColumnNameCases(q *Querier) map[string]func() error {
	type recSpace struct {
		A int `ch:"a b"`
	}
	type recQuote struct {
		A int `ch:"a\"b"`
	}
	type recBackslash struct {
		A int `ch:"a\\b"`
	}
	type recOpenParen struct {
		A int `ch:"a(b"`
	}
	type recCloseParen struct {
		A int `ch:"a)b"`
	}
	type recTab struct {
		A int `ch:"a\tb"`
	}
	type recFormFeed struct {
		A int `ch:"a\fb"`
	}
	return map[string]func() error{
		"space":       func() error { return q.InsertBatch(context.TODO(), "t", []any{&recSpace{A: 1}}) },
		"quote":       func() error { return q.InsertBatch(context.TODO(), "t", []any{&recQuote{A: 1}}) },
		"backslash":   func() error { return q.InsertBatch(context.TODO(), "t", []any{&recBackslash{A: 1}}) },
		"open paren":  func() error { return q.InsertBatch(context.TODO(), "t", []any{&recOpenParen{A: 1}}) },
		"close paren": func() error { return q.InsertBatch(context.TODO(), "t", []any{&recCloseParen{A: 1}}) },
		"tab":         func() error { return q.InsertBatch(context.TODO(), "t", []any{&recTab{A: 1}}) },
		"form feed":   func() error { return q.InsertBatch(context.TODO(), "t", []any{&recFormFeed{A: 1}}) },
	}
}

func TestInsertBatchRejectsColumnName(t *testing.T) {
	q := NewQuerier(nil)
	for name, run := range rejectColumnNameCases(q) {
		t.Run(name, func(t *testing.T) {
			assert.Error(t, run())
		})
	}
}

func TestInsertBatchInconsistentOmit(t *testing.T) {
	type rec struct {
		ID   uint32 `ch:"id"`
		Name string `ch:"name" goqu:"omitempty"`
	}
	q := NewQuerier(nil)
	rows := []any{
		&rec{ID: 1, Name: ""},
		&rec{ID: 2, Name: "x"},
	}
	// A nil conn would panic if InsertBatch reached PrepareBatch/AppendStruct;
	// reaching this error without a panic proves the check runs first.
	err := q.InsertBatch(context.TODO(), "t", rows)
	require.Error(t, err)
	assert.True(t, errors.Is(err, gohan.ErrInconsistentOmit), "got %v", err)
}

func TestNamedArgs(t *testing.T) {
	tm := time.Date(2026, 9, 27, 12, 34, 56, 123456789, time.UTC)
	loc := time.FixedZone("UTC+2", 2*60*60)
	tmNonUTC := time.Date(2026, 9, 27, 14, 34, 56, 123456789, loc)

	t.Run("NamedArg time.Time becomes DateNamed", func(t *testing.T) {
		out := namedArgs([]any{sql.NamedArg{Name: "p1", Value: tm}})
		require.Len(t, out, 1)
		nd, ok := out[0].(chdriver.NamedDateValue)
		require.True(t, ok, "got %T", out[0])
		assert.Equal(t, "p1", nd.Name)
		assert.Equal(t, tm, nd.Value)
		assert.Equal(t, uint8(chgo.NanoSeconds), nd.Scale)
	})

	t.Run("NamedArg non-UTC time.Time keeps its instant", func(t *testing.T) {
		out := namedArgs([]any{sql.NamedArg{Name: "p1", Value: tmNonUTC}})
		nd, ok := out[0].(chdriver.NamedDateValue)
		require.True(t, ok, "got %T", out[0])
		assert.True(t, nd.Value.Equal(tmNonUTC))
	})

	t.Run("NamedArg *time.Time set becomes DateNamed", func(t *testing.T) {
		out := namedArgs([]any{sql.NamedArg{Name: "p1", Value: &tm}})
		nd, ok := out[0].(chdriver.NamedDateValue)
		require.True(t, ok, "got %T", out[0])
		assert.Equal(t, "p1", nd.Name)
		assert.Equal(t, tm, nd.Value)
	})

	t.Run("NamedArg nil *time.Time becomes Named(nil)", func(t *testing.T) {
		var np *time.Time
		out := namedArgs([]any{sql.NamedArg{Name: "p1", Value: np}})
		nv, ok := out[0].(chdriver.NamedValue)
		require.True(t, ok, "got %T", out[0])
		assert.Equal(t, "p1", nv.Name)
		assert.Nil(t, nv.Value)
	})

	t.Run("NamedArg valid sql.NullTime becomes DateNamed", func(t *testing.T) {
		nt := &sql.NullTime{Time: tm, Valid: true}
		out := namedArgs([]any{sql.NamedArg{Name: "p1", Value: nt}})
		nd, ok := out[0].(chdriver.NamedDateValue)
		require.True(t, ok, "got %T", out[0])
		assert.Equal(t, "p1", nd.Name)
		assert.Equal(t, tm, nd.Value)
	})

	t.Run("NamedArg invalid sql.NullTime falls through to Named", func(t *testing.T) {
		nt := &sql.NullTime{Valid: false}
		out := namedArgs([]any{sql.NamedArg{Name: "p1", Value: nt}})
		nv, ok := out[0].(chdriver.NamedValue)
		require.True(t, ok, "got %T", out[0])
		assert.Equal(t, "p1", nv.Name)
		assert.Equal(t, nt, nv.Value)
	})

	t.Run("NamedArg nil *sql.NullTime does not panic", func(t *testing.T) {
		var nt *sql.NullTime
		assert.NotPanics(t, func() {
			namedArgs([]any{sql.NamedArg{Name: "p1", Value: nt}})
		})
	})

	t.Run("NamedArg string/int becomes Named", func(t *testing.T) {
		out := namedArgs([]any{
			sql.NamedArg{Name: "p1", Value: "x"},
			sql.NamedArg{Name: "p2", Value: 42},
		})
		require.Len(t, out, 2)
		nv1, ok := out[0].(chdriver.NamedValue)
		require.True(t, ok, "got %T", out[0])
		assert.Equal(t, "p1", nv1.Name)
		assert.Equal(t, "x", nv1.Value)
		nv2, ok := out[1].(chdriver.NamedValue)
		require.True(t, ok, "got %T", out[1])
		assert.Equal(t, "p2", nv2.Name)
		assert.Equal(t, 42, nv2.Value)
	})

	t.Run("plain non-NamedArg value is unchanged", func(t *testing.T) {
		out := namedArgs([]any{42})
		require.Len(t, out, 1)
		assert.Equal(t, 42, out[0])
	})

	t.Run("input slice is not modified", func(t *testing.T) {
		in := []any{sql.NamedArg{Name: "p1", Value: tm}, 42}
		out := namedArgs(in)
		require.NotSame(t, &in[0], &out[0])
		na, ok := in[0].(sql.NamedArg)
		require.True(t, ok)
		assert.Equal(t, tm, na.Value, "input slice element must be unchanged")
		assert.Equal(t, 42, in[1])
	})
}
