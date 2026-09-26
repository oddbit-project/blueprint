package clickhouse

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/oddbit-project/blueprint/dbx"
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

func TestInsertBatchRejectsColumnName(t *testing.T) {
	type rec struct {
		A int `ch:"a b"`
	}
	q := NewQuerier(nil)
	err := q.InsertBatch(nil, "t", []any{&rec{A: 1}})
	assert.Error(t, err)
}
