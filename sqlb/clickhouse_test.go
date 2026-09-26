package sqlb

import (
	"errors"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClickHouseClauseOrder(t *testing.T) {
	q := From(Table("r").As("q")).
		Final().
		Sample(0.1).
		Prewhere(Col("x").Eq(1)).
		Where(Col("y").Eq(2)).
		Limit(1).
		Settings(map[string]any{"max_threads": 1})
	sql, args, err := q.Build(ClickHouse())
	require.NoError(t, err)
	assert.Equal(t, `SELECT * FROM "r" AS "q" FINAL SAMPLE 0.1 PREWHERE "x" = ? WHERE "y" = ? LIMIT 1 SETTINGS max_threads = ?`, sql)
	assert.Equal(t, []any{1, 2, 1}, args)
}

func TestClickHouseSampleRows(t *testing.T) {
	sql, args, err := From("r").SampleRows(1000).Build(ClickHouse())
	require.NoError(t, err)
	assert.Equal(t, `SELECT * FROM "r" SAMPLE 1000`, sql)
	assert.Equal(t, []any{}, args)
}

func TestClickHouseSampleNoExponent(t *testing.T) {
	sql, _, err := From("r").Sample(0.00001).Build(ClickHouse())
	require.NoError(t, err)
	assert.Equal(t, `SELECT * FROM "r" SAMPLE 0.00001`, sql)
}

func TestClickHouseArrayJoin(t *testing.T) {
	tests := []struct {
		name  string
		build *SelectBuilder
		sql   string
	}{
		{"array join", Select("x", "a").From("t").ArrayJoin(Col("arr").As("a")),
			`SELECT "x", "a" FROM "t" ARRAY JOIN "arr" AS "a"`},
		{"left array join", Select("x", "a").From("t").LeftArrayJoin(Col("arr").As("a")),
			`SELECT "x", "a" FROM "t" LEFT ARRAY JOIN "arr" AS "a"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql, _, err := tt.build.Build(ClickHouse())
			require.NoError(t, err)
			assert.Equal(t, tt.sql, sql)
		})
	}

	t.Run("zero columns", func(t *testing.T) {
		_, _, err := Select("x").From("t").ArrayJoin().Build(ClickHouse())
		assert.True(t, errors.Is(err, ErrEmptyClause), "got %v", err)
	})

	t.Run("both called", func(t *testing.T) {
		_, _, err := Select("x").From("t").ArrayJoin(Col("a")).LeftArrayJoin(Col("b")).Build(ClickHouse())
		assert.True(t, errors.Is(err, ErrUnsupported), "got %v", err)
	})
}

func TestClickHouseOnlyClauses(t *testing.T) {
	tests := []struct {
		name  string
		build *SelectBuilder
	}{
		{"final", From("t").Final()},
		{"sample", From("t").Sample(0.5)},
		{"sample rows", From("t").SampleRows(10)},
		{"array join", Select("a").From("t").ArrayJoin(Col("a"))},
		{"prewhere", From("t").Prewhere(Col("x").Eq(1))},
		{"settings", From("t").Settings(map[string]any{"a": 1})},
		{"final on subquery from", From(Select("a").From("u").As("s")).Final()},
		{"sample on subquery from", From(Select("a").From("u").As("s")).Sample(0.5)},
	}
	for _, dialect := range []Dialect{Postgres(), SQLite()} {
		for _, tt := range tests {
			t.Run(dialect.Name()+"/"+tt.name, func(t *testing.T) {
				_, _, err := tt.build.Build(dialect)
				assert.True(t, errors.Is(err, ErrUnsupported), "got %v", err)
			})
		}
	}
}

func TestSampleValidation(t *testing.T) {
	tests := []struct {
		name  string
		build *SelectBuilder
		err   error
		sql   string
	}{
		{"zero", From("t").Sample(0), ErrInvalidSample, ""},
		{"negative", From("t").Sample(-1), ErrInvalidSample, ""},
		{"above one", From("t").Sample(1.5), ErrInvalidSample, ""},
		{"nan", From("t").Sample(math.NaN()), ErrInvalidSample, ""},
		{"inf", From("t").Sample(math.Inf(1)), ErrInvalidSample, ""},
		{"sample rows zero", From("t").SampleRows(0), ErrInvalidSample, ""},
		{"accept 0.1", From("t").Sample(0.1), nil, `SELECT * FROM "t" SAMPLE 0.1`},
		{"accept 1", From("t").Sample(1), nil, `SELECT * FROM "t" SAMPLE 1`},
		{"accept 0.00001 no exponent", From("t").Sample(0.00001), nil, `SELECT * FROM "t" SAMPLE 0.00001`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql, _, err := tt.build.Build(ClickHouse())
			if tt.err != nil {
				assert.True(t, errors.Is(err, tt.err), "got %v", err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.sql, sql)
		})
	}
}

func TestSettingsValidation(t *testing.T) {
	t.Run("bad key", func(t *testing.T) {
		_, _, err := From("t").Settings(map[string]any{"a b": 1}).Build(ClickHouse())
		assert.True(t, errors.Is(err, ErrInvalidSetting), "got %v", err)
	})

	t.Run("keys sorted and values bound", func(t *testing.T) {
		sql, args, err := From("t").Settings(map[string]any{"z": 1, "a": 2}).Build(ClickHouse())
		require.NoError(t, err)
		assert.Equal(t, `SELECT * FROM "t" SETTINGS a = ?, z = ?`, sql)
		assert.Equal(t, []any{2, 1}, args)
	})
}

func TestSettingsImmutability(t *testing.T) {
	m1 := map[string]any{"a": 1}
	m2 := map[string]any{"b": 2}
	base := From("t").Settings(m1)
	x := base.Settings(m2)

	m1["c"] = 3 // mutate after the call

	baseSQL, baseArgs, err := base.Build(ClickHouse())
	require.NoError(t, err)
	xSQL, xArgs, err := x.Build(ClickHouse())
	require.NoError(t, err)

	assert.Equal(t, `SELECT * FROM "t" SETTINGS a = ?`, baseSQL)
	assert.Equal(t, []any{1}, baseArgs)
	assert.Equal(t, `SELECT * FROM "t" SETTINGS a = ?, b = ?`, xSQL)
	assert.Equal(t, []any{1, 2}, xArgs)
}
