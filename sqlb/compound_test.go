package sqlb

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWithGolden(t *testing.T) {
	q := From("s").With("s", Select("a").From("t").Where(Col("k").Eq(1))).Where(Col("a").Gt(2))
	sql, args, err := q.Build(Postgres())
	require.NoError(t, err)
	assert.Equal(t, `WITH "s" AS (SELECT "a" FROM "t" WHERE "k" = $1) SELECT * FROM "s" WHERE "a" > $2`, sql)
	assert.Equal(t, []any{1, 2}, args)
}

func TestWithRecursive(t *testing.T) {
	q := Select("k").From("r").WithRecursive("r",
		Select(Int(1).As("k")).UnionAll(Select(Raw("k + 1")).From("r").Where(Col("k").Lt(3))))
	sql, args, err := q.Build(SQLite())
	require.NoError(t, err)
	// renderColumn renders an Expr via renderExpr, which parenthesizes a
	// Raw operand for precedence safety (see expr.go); this predates plan
	// 004 and is unrelated to it.
	assert.Equal(t,
		"WITH RECURSIVE `r` AS (SELECT 1 AS `k` UNION ALL SELECT (k + 1) FROM `r` WHERE `k` < ?) SELECT `k` FROM `r`",
		sql)
	assert.Equal(t, []any{3}, args)
}

func TestUnionGolden(t *testing.T) {
	tests := []struct {
		name    string
		dialect Dialect
		build   *SelectBuilder
		sql     string
		args    []any
	}{
		{"union pg", Postgres(),
			Select("a").From("t").Union(Select("a").From("u")).OrderBy("a").Limit(5),
			`SELECT "a" FROM "t" UNION SELECT "a" FROM "u" ORDER BY "a" ASC LIMIT 5`, []any{}},
		{"union sqlite", SQLite(), Select("a").From("t").Union(Select("a").From("u")),
			"SELECT `a` FROM `t` UNION SELECT `a` FROM `u`", []any{}},
		{"union clickhouse wraps with order/limit", ClickHouse(),
			Select("a").From("t").Union(Select("a").From("u")).OrderBy("a").Limit(5),
			`SELECT * FROM (SELECT "a" FROM "t" UNION DISTINCT SELECT "a" FROM "u") ORDER BY "a" ASC LIMIT 5`, []any{}},
		{"union all clickhouse no wrap without order/limit", ClickHouse(),
			Select("a").From("t").UnionAll(Select("a").From("u")),
			`SELECT "a" FROM "t" UNION ALL SELECT "a" FROM "u"`, []any{}},
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

func TestUnionMemberRestrictions(t *testing.T) {
	tests := []struct {
		name   string
		member *SelectBuilder
	}{
		{"order by", Select("a").From("u").OrderBy("a")},
		{"limit", Select("a").From("u").Limit(1)},
		{"offset", Select("a").From("u").Offset(1)},
		{"with", Select("a").From("u").With("x", Select("b").From("v"))},
		{"settings", Select("a").From("u").Settings(map[string]any{"max_threads": 1})},
		{"nested union", Select("a").From("u").Union(Select("a").From("v"))},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := Select("a").From("t").Union(tt.member)
			_, _, err := q.Build(ClickHouse())
			assert.True(t, errors.Is(err, ErrCompoundPart), "got %v", err)
		})
	}
}

func TestCompoundPlaceholderOrder(t *testing.T) {
	q := Select("a").
		With("s", Select("x").From("y").Where(Col("k").Eq(1))).
		From("s").
		Where(Col("a").Eq(2)).
		UnionAll(Select("a").From("z").Where(Col("m").Eq(3)))
	sql, args, err := q.Build(Postgres())
	require.NoError(t, err)
	assert.Equal(t,
		`WITH "s" AS (SELECT "x" FROM "y" WHERE "k" = $1) SELECT "a" FROM "s" WHERE "a" = $2 `+
			`UNION ALL SELECT "a" FROM "z" WHERE "m" = $3`, sql)
	assert.Equal(t, []any{1, 2, 3}, args)
}

func TestCompoundNames(t *testing.T) {
	_, _, err := From("s").With("a.b", Select("x").From("y")).Build(Postgres())
	assert.True(t, errors.Is(err, ErrInvalidIdentifier), "got %v", err)
}
