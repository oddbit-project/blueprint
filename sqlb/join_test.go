package sqlb

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJoinGolden(t *testing.T) {
	tests := []struct {
		name    string
		dialect Dialect
		build   *SelectBuilder
		sql     string
		args    []any
	}{
		{"inner join on", Postgres(),
			From(Table("t").As("a")).Join(Table("u").As("b"), Table("t").As("a").Col("id").Eq(Table("u").As("b").Col("tid"))),
			`SELECT * FROM "t" AS "a" INNER JOIN "u" AS "b" ON "a"."id" = "b"."tid"`, []any{}},
		{"left join on", Postgres(), From("t").LeftJoin("u", Col("t.id").Eq(Col("u.tid"))),
			`SELECT * FROM "t" LEFT JOIN "u" ON "t"."id" = "u"."tid"`, []any{}},
		{"right join on", Postgres(), From("t").RightJoin("u", Col("t.id").Eq(Col("u.tid"))),
			`SELECT * FROM "t" RIGHT JOIN "u" ON "t"."id" = "u"."tid"`, []any{}},
		{"full join on", Postgres(), From("t").FullJoin("u", Col("t.id").Eq(Col("u.tid"))),
			`SELECT * FROM "t" FULL JOIN "u" ON "t"."id" = "u"."tid"`, []any{}},
		{"cross join", Postgres(), From("t").CrossJoin("u"), `SELECT * FROM "t" CROSS JOIN "u"`, []any{}},
		{"join using", Postgres(), From("t").JoinUsing("u", "id"), `SELECT * FROM "t" INNER JOIN "u" USING ("id")`, []any{}},
		{"left join using", Postgres(), From("t").LeftJoinUsing("u", "id"),
			`SELECT * FROM "t" LEFT JOIN "u" USING ("id")`, []any{}},
		{"join using multi", Postgres(), From("t").JoinUsing("u", "a", "b"),
			`SELECT * FROM "t" INNER JOIN "u" USING ("a", "b")`, []any{}},
		{"subquery source", Postgres(), From("t").Join(Select("id").From("u").As("s"), Col("t.id").Eq(Col("s.id"))),
			`SELECT * FROM "t" INNER JOIN (SELECT "id" FROM "u") AS "s" ON "t"."id" = "s"."id"`, []any{}},

		// SQLite: backtick identifiers, `?` placeholders.
		{"cross join sqlite", SQLite(), From("t").CrossJoin("u"), "SELECT * FROM `t` CROSS JOIN `u`", []any{}},
		{"left join using sqlite", SQLite(), From("t").LeftJoinUsing("u", "id"),
			"SELECT * FROM `t` LEFT JOIN `u` USING (`id`)", []any{}},
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

func TestJoinErrors(t *testing.T) {
	tests := []struct {
		name  string
		build *SelectBuilder
		err   error
	}{
		{"empty using", From("t").JoinUsing("u"), ErrEmptyClause},
		{"nil on", From("t").Join("u", nil), ErrNilExpr},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := tt.build.Build(Postgres())
			assert.True(t, errors.Is(err, tt.err), "got %v", err)
		})
	}
}

func TestJoinImmutability(t *testing.T) {
	base := From("t").
		Join("a", Col("t.x").Eq(Col("a.x"))).
		Join("b", Col("t.x").Eq(Col("b.x"))).
		Join("c", Col("t.x").Eq(Col("c.x")))

	x := base.Join("d", Col("t.x").Eq(Col("d.x")))
	y := base.Join("e", Col("t.x").Eq(Col("e.x")))

	baseSQL, _, err := base.Build(Postgres())
	require.NoError(t, err)
	xSQL, _, err := x.Build(Postgres())
	require.NoError(t, err)
	ySQL, _, err := y.Build(Postgres())
	require.NoError(t, err)

	assert.NotContains(t, xSQL, `"e"`)
	assert.NotContains(t, ySQL, `"d"`)
	assert.NotContains(t, baseSQL, `"d"`)
	assert.NotContains(t, baseSQL, `"e"`)
	assert.Contains(t, xSQL, `"d"`)
	assert.Contains(t, ySQL, `"e"`)
}

func TestJoinPlaceholderOrder(t *testing.T) {
	inner := Select("id").From("u").Where(Col("k").Eq(1)).As("s")
	q := From("t").Join(inner, Col("t.id").Eq(Col("s.id"))).Where(Col("t.a").Eq(2))
	sql, args, err := q.Build(Postgres())
	require.NoError(t, err)
	assert.Equal(t,
		`SELECT * FROM "t" INNER JOIN (SELECT "id" FROM "u" WHERE "k" = $1) AS "s" `+
			`ON "t"."id" = "s"."id" WHERE "t"."a" = $2`, sql)
	assert.Equal(t, []any{1, 2}, args)
}
