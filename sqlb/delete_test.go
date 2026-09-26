package sqlb

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeleteGolden(t *testing.T) {
	tests := []struct {
		name    string
		dialect Dialect
		build   *DeleteBuilder
		sql     string
		args    []any
	}{
		{"where", Postgres(), Delete("t").Where(Col("a").Eq(1)), `DELETE FROM "t" WHERE "a" = $1`, []any{1}},
		{"all", Postgres(), Delete("t").All(), `DELETE FROM "t"`, []any{}},
		{"all clickhouse", ClickHouse(), Delete("t").All(), `DELETE FROM "t" WHERE 1`, []any{}},
		{"returning", Postgres(), Delete("t").Where(Col("a").Eq(1)).Returning("id"),
			`DELETE FROM "t" WHERE "a" = $1 RETURNING "id"`, []any{1}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql, args, err := tt.build.Build(tt.dialect)
			require.NoError(t, err)
			assert.Equal(t, tt.sql, sql)
			assert.Equal(t, tt.args, args)
		})
	}
}

func TestDeleteErrors(t *testing.T) {
	tests := []struct {
		name    string
		dialect Dialect
		build   *DeleteBuilder
		err     error
	}{
		{"no where", Postgres(), Delete("t"), ErrNoWhere},
		{"where empty", Postgres(), Delete("t").Where(), ErrNoWhere},
		{"where and empty", Postgres(), Delete("t").Where(And()), ErrNoWhere},
		{"where notin empty", Postgres(), Delete("t").Where(Col("id").NotIn()), ErrNoWhere},
		{"where empty match", Postgres(), Delete("t").Where(Match(map[string]any{})), ErrEmptyMatch},
		{"delete subquery", Postgres(), Delete(Select("a").From("t").As("s")), ErrNoTable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := tt.build.Build(tt.dialect)
			assert.True(t, errors.Is(err, tt.err), "got %v", err)
		})
	}
}

func TestDeleteTrivialWhere(t *testing.T) {
	// Only trivial conditions -> ErrNoWhere.
	trivialCases := []*DeleteBuilder{
		Delete("t").Where(And()),
		Delete("t").Where(And(And())),
		Delete("t").Where(Col("id").NotIn()),
		Delete("t").Where(And()).Where(Col("id").NotIn()),
	}
	for i, b := range trivialCases {
		_, _, err := b.Build(Postgres())
		assert.True(t, errors.Is(err, ErrNoWhere), "case %d: got %v", i, err)
	}

	// A trivial condition plus a real one builds.
	sql, args, err := Delete("t").Where(And()).Where(Col("a").Eq(1)).Build(Postgres())
	require.NoError(t, err)
	assert.Equal(t, `DELETE FROM "t" WHERE ((1=1) AND "a" = $1)`, sql)
	assert.Equal(t, []any{1}, args)

	// Raw is explicit and trusted: it is never treated as trivial.
	sql, args, err = Delete("t").Where(Raw("1=1")).Build(Postgres())
	require.NoError(t, err)
	assert.Equal(t, `DELETE FROM "t" WHERE (1=1)`, sql)
	assert.Equal(t, []any{}, args)
}

func TestDeleteImmutability(t *testing.T) {
	base := Delete("t").Where(Col("a").Eq(1))
	x := base.Where(Col("b").Eq(2))

	baseSQL, _, _ := base.Build(Postgres())
	xSQL, _, _ := x.Build(Postgres())
	assert.Equal(t, `DELETE FROM "t" WHERE "a" = $1`, baseSQL)
	assert.Equal(t, `DELETE FROM "t" WHERE ("a" = $1 AND "b" = $2)`, xSQL)

	allBase := Delete("t").Where(Col("a").Eq(1))
	_ = allBase.All()
	baseAfterAllSQL, _, err := allBase.Build(Postgres())
	require.NoError(t, err)
	assert.Equal(t, `DELETE FROM "t" WHERE "a" = $1`, baseAfterAllSQL)
}

func TestDeleteClickHouse(t *testing.T) {
	sql, args, err := Delete("t").All().Build(ClickHouse())
	require.NoError(t, err)
	assert.Equal(t, `DELETE FROM "t" WHERE 1`, sql)
	assert.Equal(t, []any{}, args)

	_, _, err = Delete(Table("t").As("x")).Where(Col("a").Eq(1)).Build(ClickHouse())
	assert.True(t, errors.Is(err, ErrUnsupported))

	_, _, err = Delete("t").Where(Col("a").Eq(1)).Returning("id").Build(ClickHouse())
	assert.True(t, errors.Is(err, ErrUnsupported))
}
