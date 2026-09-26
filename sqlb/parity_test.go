package sqlb_test

// This file requires exact string/args equality between sqlb and the
// frozen db/qb over every record shape and update-option combination qb
// handles correctly. It deliberately excludes: the shapes sqlb rejects
// (duplicate promoted names, embedded pointers, unexported/tagged embedded
// structs — see sqlb/record_test.go's TestRecordShapes) and unknown
// include/exclude names (qb ignores them; sqlb returns ErrUnknownField),
// both tested directly instead of here.

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/oddbit-project/blueprint/db/qb"
	"github.com/oddbit-project/blueprint/sqlb"
)

func intPtr(n int) *int       { return &n }
func strPtr(s string) *string { return &s }

var fixedTime = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

// --- record shapes qb handles correctly ---

type ParityRecord struct {
	ID      int       `db:"id" auto:"true"`
	Name    string    `db:"name"`
	Email   string    `db:"email" goqu:"omitempty"`
	Age     *int      `db:"age" goqu:"omitnil"`
	Note    *string   `db:"note"`
	Tags    []string  `db:"tags" goqu:"omitempty"`
	Count   int       `db:"count" goqu:"omitempty"`
	Hidden  string    `db:"-"`
	Created time.Time `db:"created_at"`
	Data    any       `db:"data"`
}

type ParityAutoForms struct {
	A1 int    `db:",auto"`
	A2 int    `auto:"true"`
	A3 int    `grid:"auto"`
	A4 int    `goqu:"skipinsert"`
	A5 int    `goqu:"skipupdate"`
	X  string `db:"x"`
}

type EmbeddedAddr struct {
	Street string `db:"street"`
	City   string `db:"city"`
}

type ParityEmbedded struct {
	ID int `db:"id" auto:"true"`
	EmbeddedAddr
	Name string `db:"name"`
}

type Level3Emb struct {
	Country string `db:"country"`
}

type Level2Emb struct {
	Level3Emb
	Region string `db:"region"`
}

type ParityTwoLevel struct {
	ID int `db:"id" auto:"true"`
	Level2Emb
	Name string `db:"name"`
}

type ParityNamedStruct struct {
	ID   int          `db:"id" auto:"true"`
	Meta EmbeddedAddr `db:"meta"`
}

// --- helpers ---

func assertInsertParity(t *testing.T, rec any) {
	t.Helper()
	sqlA, argsA, errA := sqlb.Insert("t").Rows(rec).Build(sqlb.Postgres())
	require.NoError(t, errA)

	b := qb.NewSqlBuilder(qb.PostgreSQLDialect())
	sqlB, argsB, errB := b.BuildSQLInsert("t", rec)
	require.NoError(t, errB)

	assert.Equal(t, sqlB, sqlA)
	assert.Equal(t, argsB, argsA)
}

func TestInsertParityWithQB(t *testing.T) {
	tests := []struct {
		name string
		rec  any
	}{
		{"zero-ish fields", ParityRecord{Name: "n", Created: fixedTime, Hidden: "h"}},
		{"all fields set", ParityRecord{
			Name: "n", Email: "e", Age: intPtr(5), Note: strPtr("note"),
			Tags: []string{"a", "b"}, Count: 3, Hidden: "h", Created: fixedTime, Data: "d",
		}},
		{"nil non-omitnil pointer", ParityRecord{Name: "n", Note: nil, Created: fixedTime}},
		{"auto tag forms", ParityAutoForms{X: "v"}},
		{"exported embedded struct", ParityEmbedded{EmbeddedAddr: EmbeddedAddr{Street: "s", City: "c"}, Name: "n"}},
		{"two-level embedding, no dup names", ParityTwoLevel{
			Level2Emb: Level2Emb{Level3Emb: Level3Emb{Country: "pt"}, Region: "lis"}, Name: "n",
		}},
		{"named (non-embedded) struct field", ParityNamedStruct{Meta: EmbeddedAddr{Street: "s", City: "c"}}},
		{"interface field, pointer receiver", &ParityRecord{Name: "n", Data: 7, Created: fixedTime}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertInsertParity(t, tt.rec)
		})
	}
}

func TestBatchInsertParityWithQB(t *testing.T) {
	r1 := ParityRecord{Name: "a", Age: intPtr(1), Tags: []string{"x"}, Count: 1, Created: fixedTime}
	r2 := &ParityRecord{Name: "b", Age: intPtr(2), Tags: []string{"y"}, Count: 2, Created: fixedTime}
	r3 := ParityRecord{Name: "c", Age: intPtr(3), Tags: []string{"z"}, Count: 3, Created: fixedTime}
	recs := []any{r1, r2, r3}

	sqlA, argsA, errA := sqlb.Insert("t").Rows(recs...).Build(sqlb.Postgres())
	require.NoError(t, errA)

	b := qb.NewSqlBuilder(qb.PostgreSQLDialect())
	sqlB, argsB, errB := b.BuildSQLBatchInsert("t", recs)
	require.NoError(t, errB)

	assert.Equal(t, sqlB, sqlA)
	assert.Equal(t, argsB, argsA)

	// First record includes Age/Tags/Count; second omits them all -> both
	// engines must error.
	omitting := []any{r1, ParityRecord{Name: "b", Created: fixedTime}}
	_, _, errA2 := sqlb.Insert("t").Rows(omitting...).Build(sqlb.Postgres())
	_, _, errB2 := b.BuildSQLBatchInsert("t", omitting)
	assert.Error(t, errA2)
	assert.Error(t, errB2)

	// Reverse: first record omits, second includes -> both engines must
	// error.
	including := []any{ParityRecord{Name: "a", Created: fixedTime}, r2}
	_, _, errA3 := sqlb.Insert("t").Rows(including...).Build(sqlb.Postgres())
	_, _, errB3 := b.BuildSQLBatchInsert("t", including)
	assert.Error(t, errA3)
	assert.Error(t, errB3)
}

// --- update option combinations ---

func assertUpdateParity(t *testing.T, rec any, sqlbOpts []sqlb.RecordOption, qbOpts *qb.UpdateOptions) {
	t.Helper()
	sqlA, argsA, errA := sqlb.Update("t").SetRecord(rec, sqlbOpts...).Where(sqlb.Col("id").Eq(1)).Build(sqlb.Postgres())
	require.NoError(t, errA)

	b := qb.NewSqlBuilder(qb.PostgreSQLDialect())
	sqlB, argsB, errB := b.Update("t", rec).WithOptions(qbOpts).Where(qb.Eq("id", 1)).Build()
	require.NoError(t, errB)

	assert.Equal(t, sqlB, sqlA)
	assert.Equal(t, argsB, argsA)
}

func TestUpdateParityWithQB(t *testing.T) {
	rec := ParityRecord{
		ID: 1, Name: "n", Email: "", Age: intPtr(5), Note: nil,
		Tags: nil, Count: 0, Hidden: "h", Created: fixedTime, Data: "d",
	}

	t.Run("none", func(t *testing.T) {
		assertUpdateParity(t, rec, nil, &qb.UpdateOptions{IncludeZeroValues: true})
	})
	t.Run("SkipZeroValues", func(t *testing.T) {
		assertUpdateParity(t, rec,
			[]sqlb.RecordOption{sqlb.SkipZeroValues()},
			&qb.UpdateOptions{IncludeZeroValues: false})
	})
	t.Run("IncludeFields", func(t *testing.T) {
		assertUpdateParity(t, rec,
			[]sqlb.RecordOption{sqlb.IncludeFields("name")},
			&qb.UpdateOptions{IncludeZeroValues: true, IncludeFields: []string{"name"}})
	})
	t.Run("ExcludeFields", func(t *testing.T) {
		assertUpdateParity(t, rec,
			[]sqlb.RecordOption{sqlb.ExcludeFields("age")},
			&qb.UpdateOptions{IncludeZeroValues: true, ExcludeFields: []string{"age"}})
	})
	t.Run("WithAutoFields", func(t *testing.T) {
		assertUpdateParity(t, rec,
			[]sqlb.RecordOption{sqlb.WithAutoFields()},
			&qb.UpdateOptions{IncludeZeroValues: true, UpdateAutoFields: true})
	})
}
