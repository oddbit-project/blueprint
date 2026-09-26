package sqlb

import (
	"errors"
	"math"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSelectGolden(t *testing.T) {
	tests := []struct {
		name    string
		dialect Dialect
		build   *SelectBuilder
		sql     string
		args    []any
	}{
		{"from", Postgres(), From("users"), `SELECT * FROM "users"`, []any{}},
		{"cols schema", Postgres(), Select("id", "name").From("public.users"),
			`SELECT "id", "name" FROM "public"."users"`, []any{}},
		{"int as", Postgres(), Select(Int(1).As("k")), `SELECT 1 AS "k"`, []any{}},
		{"count all as", Postgres(), Select(CountAll().As("n")).From("t"),
			`SELECT COUNT(*) AS "n" FROM "t"`, []any{}},
		{"two wheres", Postgres(), From("t").Where(Col("a").Eq(1)).Where(Col("b").Eq(2)),
			`SELECT * FROM "t" WHERE ("a" = $1 AND "b" = $2)`, []any{1, 2}},
		{"group having", Postgres(), From("t").GroupBy("a").Having(CountAll().Gt(1)),
			`SELECT * FROM "t" GROUP BY "a" HAVING COUNT(*) > $1`, []any{1}},
		{"order limit offset", Postgres(), From("t").OrderBy("a", Col("b").Desc()).Limit(10).Offset(5),
			`SELECT * FROM "t" ORDER BY "a" ASC, "b" DESC LIMIT 10 OFFSET 5`, []any{}},
		{"limit zero", Postgres(), From("t").Limit(0), `SELECT * FROM "t" LIMIT 0`, []any{}},
		{"in subquery", Postgres(),
			From("t").Where(Col("id").In(Select("uid").From("x").Where(Col("k").Eq(7)))).Where(Col("z").Eq(8)),
			`SELECT * FROM "t" WHERE ("id" IN (SELECT "uid" FROM "x" WHERE "k" = $1) AND "z" = $2)`,
			[]any{7, 8}},
		{"scalar subquery", Postgres(), From("t").Where(Col("a").Eq(Sub(Select(Max("a")).From("t")))),
			`SELECT * FROM "t" WHERE "a" = (SELECT MAX("a") FROM "t")`, []any{}},
		{"subquery from", Postgres(), From(Select("a").From("t").As("s")),
			`SELECT * FROM (SELECT "a" FROM "t") AS "s"`, []any{}},
		{"tableref alias", Postgres(), From(Table("t").As("x")).Where(Table("t").As("x").Col("a").Eq(1)),
			`SELECT * FROM "t" AS "x" WHERE "x"."a" = $1`, []any{1}},
		{"exists", Postgres(), From("t").Where(Exists(Select(Int(1)).From("u"))),
			`SELECT * FROM "t" WHERE EXISTS (SELECT 1 FROM "u")`, []any{}},
		{"distinct", Postgres(), From("t").Distinct(), `SELECT DISTINCT * FROM "t"`, []any{}},
		{"maxuint64 clickhouse limit", ClickHouse(), From("t").Limit(math.MaxUint64),
			`SELECT * FROM "t" LIMIT 18446744073709551615`, []any{}},
		{"ident allowed postgres", Postgres(), Select("a?b").From("t"), `SELECT "a?b" FROM "t"`, []any{}},

		// SQLite: backtick identifiers, `?` placeholders.
		{"from sqlite", SQLite(), From("t"), "SELECT * FROM `t`", []any{}},
		{"where sqlite", SQLite(), From("t").Where(Col("a").Eq(1)), "SELECT * FROM `t` WHERE `a` = ?", []any{1}},
		{"offset sqlite", SQLite(), From("t").Offset(5), "SELECT * FROM `t` LIMIT -1 OFFSET 5", []any{}},
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

func TestSelectErrors(t *testing.T) {
	tests := []struct {
		name    string
		dialect Dialect
		build   *SelectBuilder
		err     error
	}{
		{"no cols no from", Postgres(), Select(), ErrNoTable},
		{"invalid column", Postgres(), Select(1).From("t"), ErrInvalidColumn},
		{"from nil", Postgres(), Select("a").From(nil), ErrNoTable},
		{"from empty string", Postgres(), Select("a").From(""), ErrNoTable},
		{"from bare select", Postgres(), Select("a").From(Select("a")), ErrNeedAlias},
		{"limit too big postgres", Postgres(), From("t").Limit(math.MaxUint64), ErrInvalidLimit},
		{"where nil", Postgres(), From("t").Where(nil), ErrNilExpr},
		{"zero dialect", Dialect{}, From("t"), ErrUnknownDialect},
		{"clickhouse ident ban", ClickHouse(), Select("a?b").From("t"), ErrInvalidIdentifier},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := tt.build.Build(tt.dialect)
			assert.True(t, errors.Is(err, tt.err), "got %v", err)
		})
	}
}

func TestSelectImmutability(t *testing.T) {
	a, b, c := Col("a").Eq(1), Col("b").Eq(2), Col("c").Eq(3)
	d, e := Col("d").Eq(4), Col("e").Eq(5)
	base := From("t").Where(a).Where(b).Where(c)

	x := base.Where(d)
	y := base.Where(e)

	baseSQL, _, _ := base.Build(Postgres())
	xSQL, _, _ := x.Build(Postgres())
	ySQL, _, _ := y.Build(Postgres())

	assert.NotContains(t, xSQL, `"e"`)
	assert.NotContains(t, ySQL, `"d"`)
	assert.NotContains(t, baseSQL, `"d"`)
	assert.NotContains(t, baseSQL, `"e"`)

	// AddColumns
	baseCols := Select("a").AddColumns("b")
	x2 := baseCols.AddColumns("c")
	y2 := baseCols.AddColumns("d")
	x2SQL, _, _ := x2.From("t").Build(Postgres())
	y2SQL, _, _ := y2.From("t").Build(Postgres())
	baseColsSQL, _, _ := baseCols.From("t").Build(Postgres())
	assert.NotContains(t, x2SQL, `"d"`)
	assert.NotContains(t, y2SQL, `"c"`)
	assert.NotContains(t, baseColsSQL, `"c"`)
	assert.NotContains(t, baseColsSQL, `"d"`)

	// OrderBy
	baseOrder := From("t").OrderBy("a").OrderBy("b")
	x3 := baseOrder.OrderBy("c")
	y3 := baseOrder.OrderBy("d")
	x3SQL, _, _ := x3.Build(Postgres())
	y3SQL, _, _ := y3.Build(Postgres())
	baseOrderSQL, _, _ := baseOrder.Build(Postgres())
	assert.NotContains(t, x3SQL, `"d"`)
	assert.NotContains(t, y3SQL, `"c"`)
	assert.NotContains(t, baseOrderSQL, `"c"`)
	assert.NotContains(t, baseOrderSQL, `"d"`)

	// GroupBy
	baseGroup := From("t").GroupBy("a").GroupBy("b")
	x4 := baseGroup.GroupBy("c")
	y4 := baseGroup.GroupBy("d")
	x4SQL, _, _ := x4.Build(Postgres())
	y4SQL, _, _ := y4.Build(Postgres())
	baseGroupSQL, _, _ := baseGroup.Build(Postgres())
	assert.NotContains(t, x4SQL, `"d"`)
	assert.NotContains(t, y4SQL, `"c"`)
	assert.NotContains(t, baseGroupSQL, `"c"`)
	assert.NotContains(t, baseGroupSQL, `"d"`)

	// Having
	baseHaving := From("t").GroupBy("a").Having(a).Having(b)
	x5 := baseHaving.Having(d)
	y5 := baseHaving.Having(e)
	x5SQL, _, _ := x5.Build(Postgres())
	y5SQL, _, _ := y5.Build(Postgres())
	baseHavingSQL, _, _ := baseHaving.Build(Postgres())
	assert.NotContains(t, x5SQL, `"e"`)
	assert.NotContains(t, y5SQL, `"d"`)
	assert.NotContains(t, baseHavingSQL, `"d"`)
	assert.NotContains(t, baseHavingSQL, `"e"`)

	// Columns(cols...) does not alias the caller's backing array.
	callerCols := []any{"a", "b"}
	built := Select().Columns(callerCols...)
	callerCols[0] = "z"
	builtSQL, _, _ := built.From("t").Build(Postgres())
	assert.Contains(t, builtSQL, `"a"`)
	assert.NotContains(t, builtSQL, `"z"`)
}

func TestSelectPlaceholderNumbering(t *testing.T) {
	inner := Select("uid").From("x").Where(Col("k").Eq(1))
	q := From("t").
		Where(Col("id").In(inner)).
		Where(Col("a").Eq(Sub(Select(Max("v")).From("t").Where(Col("m").Eq(2))))).
		Where(Exists(Select(Int(1)).From("u").Where(Col("n").Eq(3))))
	sql, args, err := q.Build(Postgres())
	require.NoError(t, err)
	assert.Equal(t,
		`SELECT * FROM "t" WHERE ("id" IN (SELECT "uid" FROM "x" WHERE "k" = $1) AND `+
			`"a" = (SELECT MAX("v") FROM "t" WHERE "m" = $2) AND `+
			`EXISTS (SELECT 1 FROM "u" WHERE "n" = $3))`, sql)
	assert.Equal(t, []any{1, 2, 3}, args)
}

func TestSelectNotAnExpr(t *testing.T) {
	exprType := reflect.TypeOf((*Expr)(nil)).Elem()
	assert.False(t, reflect.TypeOf((*SelectBuilder)(nil)).Implements(exprType))
	assert.False(t, reflect.TypeOf(Subquery{}).Implements(exprType))
}
