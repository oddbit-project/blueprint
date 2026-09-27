package sqlb

import (
	"errors"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateGolden(t *testing.T) {
	tests := []struct {
		name    string
		dialect Dialect
		build   *UpdateBuilder
		sql     string
		args    []any
	}{
		{"set chain", Postgres(),
			Update("t").Set("a", 1).Set("b", "x").Where(Col("id").Eq(9)),
			`UPDATE "t" SET "a" = $1, "b" = $2 WHERE "id" = $3`, []any{1, "x", 9}},
		{"setmap all", Postgres(),
			Update("t").SetMap(map[string]any{"b": 2, "a": 1}).All(),
			`UPDATE "t" SET "a" = $1, "b" = $2`, []any{1, 2}},
		{"returning postgres", Postgres(),
			Update("t").Set("a", 1).Where(Col("id").Eq(1)).Returning("a"),
			`UPDATE "t" SET "a" = $1 WHERE "id" = $2 RETURNING "a"`, []any{1, 1}},
		{"returning sqlite", SQLite(),
			Update("t").Set("a", 1).Where(Col("id").Eq(1)).Returning("a"),
			"UPDATE `t` SET `a` = ? WHERE `id` = ? RETURNING `a`", []any{1, 1}},
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

func TestUpdateErrors(t *testing.T) {
	tests := []struct {
		name  string
		build *UpdateBuilder
		err   error
	}{
		{"no where", Update("t").Set("a", 1), ErrNoWhere},
		{"where empty", Update("t").Set("a", 1).Where(), ErrNoWhere},
		{"no columns", Update("t").Where(Col("id").Eq(1)), ErrNoColumns},
		{"duplicate column", Update("t").Set("a", 1).Set("a", 2).All(), ErrDuplicateColumn},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := tt.build.Build(Postgres())
			assert.True(t, errors.Is(err, tt.err), "got %v", err)
		})
	}
}

func TestUpdateImmutability(t *testing.T) {
	base := Update("t").Set("a", 1).Where(Col("id").Eq(1))
	x := base.Set("b", 2)

	baseSQL, baseArgs, err := base.Build(Postgres())
	require.NoError(t, err)
	assert.Equal(t, `UPDATE "t" SET "a" = $1 WHERE "id" = $2`, baseSQL)
	assert.Equal(t, []any{1, 1}, baseArgs)

	xSQL, xArgs, err := x.Build(Postgres())
	require.NoError(t, err)
	assert.Equal(t, `UPDATE "t" SET "a" = $1, "b" = $2 WHERE "id" = $3`, xSQL)
	assert.Equal(t, []any{1, 2, 1}, xArgs)
}

func TestUpdateRequiresWhere(t *testing.T) {
	trivialCases := []*UpdateBuilder{
		Update("t").Set("a", 1),
		Update("t").Set("a", 1).Where(),
		Update("t").Set("a", 1).Where(And()),
		Update("t").Set("a", 1).Where(Col("id").NotIn()),
		Update("t").Set("a", 1).Where(Not(Col("id").In())),
		Update("t").Set("a", 1).Where(Or(Col("a").Eq(1), Col("id").NotIn())),
	}
	for i, b := range trivialCases {
		_, _, err := b.Build(Postgres())
		assert.True(t, errors.Is(err, ErrNoWhere), "case %d: got %v", i, err)
	}

	sql, args, err := Update("t").Set("a", 1).Where(And()).Where(Col("id").Eq(1)).Build(Postgres())
	require.NoError(t, err)
	assert.Equal(t, `UPDATE "t" SET "a" = $1 WHERE ((1=1) AND "id" = $2)`, sql)
	assert.Equal(t, []any{1, 1}, args)

	sql, args, err = Update("t").Set("a", 1).Where(Not(Col("id").In(1))).Build(Postgres())
	require.NoError(t, err)
	assert.Equal(t, `UPDATE "t" SET "a" = $1 WHERE NOT ("id" IN ($2))`, sql)
	assert.Equal(t, []any{1, 1}, args)
}

func TestUpdateReturning(t *testing.T) {
	sql, _, err := Update("t").Set("a", 1).Where(Col("id").Eq(1)).Returning("a").Build(Postgres())
	require.NoError(t, err)
	assert.Equal(t, `UPDATE "t" SET "a" = $1 WHERE "id" = $2 RETURNING "a"`, sql)

	sql, _, err = Update("t").Set("a", 1).Where(Col("id").Eq(1)).Returning("a").Build(SQLite())
	require.NoError(t, err)
	assert.Equal(t, "UPDATE `t` SET `a` = ? WHERE `id` = ? RETURNING `a`", sql)

	_, _, err = Update("t").Set("a", 1).All().Build(ClickHouse())
	assert.True(t, errors.Is(err, ErrUnsupported))
}

func TestUpdateDuplicateAcrossSetters(t *testing.T) {
	type rec struct {
		Name string `db:"name"`
	}
	_, _, err := Update("t").SetRecord(rec{Name: "n"}).Set("name", "x").Where(Col("id").Eq(1)).Build(Postgres())
	assert.True(t, errors.Is(err, ErrDuplicateColumn), "got %v", err)
}

func TestUpdateSetRecord(t *testing.T) {
	type rec struct {
		ID   int    `db:"id" auto:"true"`
		Name string `db:"name"`
		Age  int    `db:"age"`
	}
	sql, args, err := Update("t").SetRecord(rec{ID: 1, Name: "n", Age: 0}).Where(Col("id").Eq(1)).Build(Postgres())
	require.NoError(t, err)
	assert.Equal(t, `UPDATE "t" SET "name" = $1, "age" = $2 WHERE "id" = $3`, sql)
	assert.Equal(t, []any{"n", 0, 1}, args)

	sql, args, err = Update("t").SetRecord(rec{ID: 1, Name: "n", Age: 0}, SkipZeroValues()).Where(Col("id").Eq(1)).Build(Postgres())
	require.NoError(t, err)
	assert.Equal(t, `UPDATE "t" SET "name" = $1 WHERE "id" = $2`, sql)
	assert.Equal(t, []any{"n", 1}, args)

	sql, args, err = Update("t").SetRecord(rec{ID: 1, Name: "n", Age: 5}, WithAutoFields()).Where(Col("id").Eq(1)).Build(Postgres())
	require.NoError(t, err)
	assert.Equal(t, `UPDATE "t" SET "id" = $1, "name" = $2, "age" = $3 WHERE "id" = $4`, sql)
	assert.Equal(t, []any{1, "n", 5, 1}, args)
}

func TestUpdateNotAnExpr(t *testing.T) {
	exprType := reflect.TypeOf((*Expr)(nil)).Elem()
	assert.False(t, reflect.TypeOf((*UpdateBuilder)(nil)).Implements(exprType))
}
