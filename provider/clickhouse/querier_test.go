package clickhouse

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/oddbit-project/blueprint/dbx"
	"github.com/oddbit-project/blueprint/sqlb"
)

func TestQuerierDialect(t *testing.T) {
	q := NewQuerier(nil)
	assert.Equal(t, "clickhouse", q.Dialect().Name())
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
	err := q.Select(nil, &dest, "SELECT 1")
	assert.Error(t, err)
}

func TestInsertBatchEmpty(t *testing.T) {
	q := NewQuerier(nil)
	err := q.InsertBatch(nil, "t", nil)
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
		"space":       func() error { return q.InsertBatch(nil, "t", []any{&recSpace{A: 1}}) },
		"quote":       func() error { return q.InsertBatch(nil, "t", []any{&recQuote{A: 1}}) },
		"backslash":   func() error { return q.InsertBatch(nil, "t", []any{&recBackslash{A: 1}}) },
		"open paren":  func() error { return q.InsertBatch(nil, "t", []any{&recOpenParen{A: 1}}) },
		"close paren": func() error { return q.InsertBatch(nil, "t", []any{&recCloseParen{A: 1}}) },
		"tab":         func() error { return q.InsertBatch(nil, "t", []any{&recTab{A: 1}}) },
		"form feed":   func() error { return q.InsertBatch(nil, "t", []any{&recFormFeed{A: 1}}) },
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
	err := q.InsertBatch(nil, "t", rows)
	require.Error(t, err)
	assert.True(t, errors.Is(err, sqlb.ErrInconsistentOmit), "got %v", err)
}
