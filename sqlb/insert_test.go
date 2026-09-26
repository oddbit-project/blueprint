package sqlb

import (
	"errors"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInsertGolden(t *testing.T) {
	type insUser struct {
		ID   int    `db:"id" auto:"true"`
		Name string `db:"name"`
	}
	u1 := insUser{Name: "a"}
	u2 := insUser{Name: "b"}

	tests := []struct {
		name    string
		dialect Dialect
		build   *InsertBuilder
		sql     string
		args    []any
	}{
		{"columns values", Postgres(),
			Insert("t").Columns("a", "b").Values(1, "x"),
			`INSERT INTO "t" ("a", "b") VALUES ($1, $2)`, []any{1, "x"}},
		{"multi values", Postgres(),
			Insert("t").Columns("a", "b").Values(1, "x").Values(2, "y"),
			`INSERT INTO "t" ("a", "b") VALUES ($1, $2), ($3, $4)`, []any{1, "x", 2, "y"}},
		{"slice value one arg", Postgres(),
			Insert("t").Columns("tags").Values([]string{"a", "b"}),
			`INSERT INTO "t" ("tags") VALUES ($1)`, []any{[]string{"a", "b"}}},
		{"setmap sorted", Postgres(),
			Insert("t").SetMap(map[string]any{"b": 2, "a": 1}),
			`INSERT INTO "t" ("a", "b") VALUES ($1, $2)`, []any{1, 2}},
		{"rows", Postgres(),
			Insert("t").Rows(u1, u2),
			`INSERT INTO "t" ("name") VALUES ($1), ($2)`, []any{"a", "b"}},
		{"from select", Postgres(),
			Insert("t").Columns("a").FromSelect(Select("x").From("s").Where(Col("k").Eq(1))),
			`INSERT INTO "t" ("a") SELECT "x" FROM "s" WHERE "k" = $1`, []any{1}},
		{"from select sqlite upsert where true", SQLite(),
			Insert("t").Columns("a").FromSelect(Select("x").From("s")).OnConflict("a").DoNothing(),
			"INSERT INTO `t` (`a`) SELECT `x` FROM `s` WHERE true ON CONFLICT (`a`) DO NOTHING", []any{}},
		{"from select union sqlite upsert where true on last member", SQLite(),
			Insert("t").Columns("a").
				FromSelect(Select("x").From("s").UnionAll(Select("y").From("u"))).
				OnConflict("a").DoNothing(),
			"INSERT INTO `t` (`a`) SELECT `x` FROM `s` UNION ALL SELECT `y` FROM `u` WHERE true ON CONFLICT (`a`) DO NOTHING",
			[]any{}},
		{"from select union sqlite upsert last member already has where", SQLite(),
			Insert("t").Columns("a").
				FromSelect(Select("x").From("s").UnionAll(Select("y").From("u").Where(Col("y").Gt(0)))).
				OnConflict("a").DoNothing(),
			"INSERT INTO `t` (`a`) SELECT `x` FROM `s` UNION ALL SELECT `y` FROM `u` WHERE `y` > ? ON CONFLICT (`a`) DO NOTHING",
			[]any{0}},
		{"do nothing with target", Postgres(),
			Insert("t").Columns("a").Values(1).OnConflict("id").DoNothing(),
			`INSERT INTO "t" ("a") VALUES ($1) ON CONFLICT ("id") DO NOTHING`, []any{1}},
		{"do nothing no target", Postgres(),
			Insert("t").Columns("a").Values(1).OnConflict().DoNothing(),
			`INSERT INTO "t" ("a") VALUES ($1) ON CONFLICT DO NOTHING`, []any{1}},
		{"do update excluded", Postgres(),
			Insert("t").Columns("id", "name", "age").Values(1, "n", 2).OnConflict("id").DoUpdateExcluded("name", "age"),
			`INSERT INTO "t" ("id", "name", "age") VALUES ($1, $2, $3) ON CONFLICT ("id") DO UPDATE SET "name" = excluded."name", "age" = excluded."age"`,
			[]any{1, "n", 2}},
		{"do update raw", Postgres(),
			Insert("t").Columns("n").Values(1).OnConflict("id").DoUpdate(map[string]any{"n": Raw("? + 1", Col("t.n"))}),
			`INSERT INTO "t" ("n") VALUES ($1) ON CONFLICT ("id") DO UPDATE SET "n" = "t"."n" + 1`, []any{1}},
		{"returning", Postgres(),
			Insert("t").Columns("id").Values(1).Returning("id"),
			`INSERT INTO "t" ("id") VALUES ($1) RETURNING "id"`, []any{1}},
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

func TestInsertErrors(t *testing.T) {
	tests := []struct {
		name  string
		build *InsertBuilder
		err   error
	}{
		{"value count mismatch", Insert("t").Columns("a").Values(1, 2), ErrValueCount},
		{"no rows", Insert("t"), ErrNoColumns},
		{"mixed rows and values", Insert("t").Rows(struct {
			Name string `db:"name"`
		}{"x"}).Values(1), ErrInsertMixed},
		{"invalid identifier", Insert("t").Columns("a.b").Values(1), ErrInvalidIdentifier},
		{"conflict no target on doupdate excluded", Insert("t").Columns("a").Values(1).OnConflict().DoUpdateExcluded("a"), ErrConflictTarget},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := tt.build.Build(Postgres())
			assert.True(t, errors.Is(err, tt.err), "got %v", err)
		})
	}
}

func TestInsertImmutability(t *testing.T) {
	base := Insert("t").Columns("a").Values(1)
	x := base.Values(2)

	baseSQL, baseArgs, err := base.Build(Postgres())
	require.NoError(t, err)
	assert.Equal(t, `INSERT INTO "t" ("a") VALUES ($1)`, baseSQL)
	assert.Equal(t, []any{1}, baseArgs)

	xSQL, xArgs, err := x.Build(Postgres())
	require.NoError(t, err)
	assert.Equal(t, `INSERT INTO "t" ("a") VALUES ($1), ($2)`, xSQL)
	assert.Equal(t, []any{1, 2}, xArgs)

	y := base.Returning("a")
	baseSQL2, _, _ := base.Build(Postgres())
	assert.Equal(t, baseSQL, baseSQL2)
	ySQL, _, err := y.Build(Postgres())
	require.NoError(t, err)
	assert.Equal(t, `INSERT INTO "t" ("a") VALUES ($1) RETURNING "a"`, ySQL)

	// The sqlite upsert WHERE-true rewrite must not mutate the
	// *SelectBuilder the caller passed to FromSelect.
	inner := Select("x").From("s").UnionAll(Select("y").From("u"))
	_, _, err = Insert("t").Columns("a").FromSelect(inner).OnConflict("a").DoNothing().Build(SQLite())
	require.NoError(t, err)
	innerSQL, _, err := inner.Build(SQLite())
	require.NoError(t, err)
	assert.Equal(t, "SELECT `x` FROM `s` UNION ALL SELECT `y` FROM `u`", innerSQL)
}

func TestInsertUnsupportedClickHouse(t *testing.T) {
	_, _, err := Insert("t").Columns("a").Values(1).OnConflict("a").DoNothing().Build(ClickHouse())
	assert.True(t, errors.Is(err, ErrUnsupported), "conflict: got %v", err)

	_, _, err = Insert("t").Columns("a").Values(1).Returning("a").Build(ClickHouse())
	assert.True(t, errors.Is(err, ErrUnsupported), "returning: got %v", err)

	_, _, err = render(ClickHouse(), Col("a").Eq(Excluded("a")))
	assert.True(t, errors.Is(err, ErrUnsupported), "excluded: got %v", err)
}

func TestInsertClickHouse(t *testing.T) {
	sql, args, err := Insert("t").Columns("a", "b").Values(1, "x").Build(ClickHouse())
	require.NoError(t, err)
	assert.Equal(t, `INSERT INTO "t" ("a", "b") VALUES (?, ?)`, sql)
	assert.Equal(t, []any{1, "x"}, args)
}

func TestUpsertValidation(t *testing.T) {
	_, _, err := Insert("t").Columns("a").Values(1).OnConflict("id").DoUpdate(map[string]any{}).Build(Postgres())
	assert.True(t, errors.Is(err, ErrNoColumns), "empty do update: got %v", err)

	_, _, err = Insert("t").Columns("a").Values(1).OnConflict("id").DoUpdateExcluded().Build(Postgres())
	assert.True(t, errors.Is(err, ErrNoColumns), "empty do update excluded: got %v", err)

	_, _, err = Insert("t").Columns("a").Values(1).OnConflict("id").DoUpdateExcluded("b").Build(Postgres())
	assert.True(t, errors.Is(err, ErrUnknownField), "excluded col not inserted: got %v", err)
}

func TestInsertSubqueryValue(t *testing.T) {
	sql, args, err := Insert("t").Columns("a").Values(Sub(Select("x").From("s").Where(Col("k").Eq(1)))).Build(Postgres())
	require.NoError(t, err)
	assert.Equal(t, `INSERT INTO "t" ("a") VALUES ((SELECT "x" FROM "s" WHERE "k" = $1))`, sql)
	assert.Equal(t, []any{1}, args)
}

func TestInsertNotAnExpr(t *testing.T) {
	exprType := reflect.TypeOf((*Expr)(nil)).Elem()
	assert.False(t, reflect.TypeOf((*InsertBuilder)(nil)).Implements(exprType))
}
