package sqlb

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"net"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestComparisons(t *testing.T) {
	tests := []struct {
		name    string
		dialect Dialect
		expr    Expr
		sql     string
		args    []any
		err     error
	}{
		{"eq", Postgres(), Col("a").Eq(5), `"a" = $1`, []any{5}, nil},
		{"eq nil", Postgres(), Col("a").Eq(nil), `"a" IS NULL`, []any{}, nil},
		{"eq nil ptr", Postgres(), Col("a").Eq((*int)(nil)), `"a" IS NULL`, []any{}, nil},
		{"neq nil", Postgres(), Col("a").Neq(nil), `"a" IS NOT NULL`, []any{}, nil},
		{"eq col", Postgres(), Col("a").Eq(Col("b")), `"a" = "b"`, []any{}, nil},
		{"in ints", Postgres(), Col("a").In(1, 2), `"a" IN ($1, $2)`, []any{1, 2}, nil},
		{"in slice", Postgres(), Col("a").In([]int{1, 2}), `"a" IN ($1, $2)`, []any{1, 2}, nil},
		{"in empty", Postgres(), Col("a").In(), "1=0", []any{}, nil},
		{"notin empty", Postgres(), Col("a").NotIn(), "1=1", []any{}, nil},
		{"between", Postgres(), Col("a").Between(1, 9), `"a" BETWEEN $1 AND $2`, []any{1, 9}, nil},
		{"and two", Postgres(), And(Col("a").Eq(1), Col("b").Eq(2)), `("a" = $1 AND "b" = $2)`, []any{1, 2}, nil},
		{"and one", Postgres(), And(Col("a").Eq(1)), `"a" = $1`, []any{1}, nil},
		{"and raw", Postgres(), And(Col("t").Eq(1), Raw("a OR b")), `("t" = $1 AND (a OR b))`, []any{1}, nil},
		{"and empty", Postgres(), And(), "(1=1)", []any{}, nil},
		{"or empty", Postgres(), Or(), "(1=0)", []any{}, nil},
		{"not", Postgres(), Not(Col("a").Eq(1)), `NOT ("a" = $1)`, []any{1}, nil},
		{"match", Postgres(), Match(map[string]any{"b": 2, "a": 1}), `("a" = $1 AND "b" = $2)`, []any{1, 2}, nil},
		{"match empty", Postgres(), Match(map[string]any{}), "", nil, ErrEmptyMatch},
		{"val nil", Postgres(), Val(nil), "NULL", []any{}, nil},
		{"int pos", Postgres(), Int(7), "7", []any{}, nil},
		{"int neg", Postgres(), Int(-3), "(-3)", []any{}, nil},
		{"as", Postgres(), Col("a").As("x"), `"a" AS "x"`, []any{}, nil},
		{"order", Postgres(), Col("a").Desc().NullsLast(), `"a" DESC NULLS LAST`, []any{}, nil},
		{"eq sqlite", SQLite(), Col("a").Eq(1), "`a` = ?", []any{1}, nil},
		{"ilike sqlite", SQLite(), Col("a").ILike("x"), "", nil, ErrUnsupported},
		{"unsafe clickhouse", ClickHouse(), Col("a").Eq(map[string]int{"k": 1}), "", nil, ErrUnsafeValue},
		{"neq", Postgres(), Col("a").Neq(5), `"a" <> $1`, []any{5}, nil},
		{"gt", Postgres(), Col("a").Gt(5), `"a" > $1`, []any{5}, nil},
		{"gte", Postgres(), Col("a").Gte(5), `"a" >= $1`, []any{5}, nil},
		{"lt", Postgres(), Col("a").Lt(5), `"a" < $1`, []any{5}, nil},
		{"lte", Postgres(), Col("a").Lte(5), `"a" <= $1`, []any{5}, nil},
		{"notin", Postgres(), Col("a").NotIn(1, 2), `"a" NOT IN ($1, $2)`, []any{1, 2}, nil},
		{"notbetween", Postgres(), Col("a").NotBetween(1, 9), `"a" NOT BETWEEN $1 AND $2`, []any{1, 9}, nil},
		{"like", Postgres(), Col("a").Like("x%"), `"a" LIKE $1`, []any{"x%"}, nil},
		{"notlike", Postgres(), Col("a").NotLike("x%"), `"a" NOT LIKE $1`, []any{"x%"}, nil},
		{"ilike pg", Postgres(), Col("a").ILike("x"), `"a" ILIKE $1`, []any{"x"}, nil},
		{"notilike pg", Postgres(), Col("a").NotILike("x"), `"a" NOT ILIKE $1`, []any{"x"}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql, args, err := render(tt.dialect, tt.expr)
			if tt.err != nil {
				assert.True(t, errors.Is(err, tt.err), "got %v", err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.sql, sql)
			assert.Equal(t, tt.args, args)
		})
	}
}

func TestNilHandling(t *testing.T) {
	var nilIface any
	tests := []struct {
		name string
		x    any
	}{
		{"untyped nil", nil},
		{"nil ptr", (*int)(nil)},
		{"nil interface", nilIface},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql, args, err := render(Postgres(), Col("a").Eq(tt.x))
			require.NoError(t, err)
			assert.Equal(t, `"a" IS NULL`, sql)
			assert.Equal(t, []any{}, args)
		})
	}
	n := 5
	sql, args, err := render(Postgres(), Col("a").Eq(&n))
	require.NoError(t, err)
	assert.Equal(t, `"a" = $1`, sql)
	assert.Equal(t, []any{&n}, args)
}

type valuerSlice []int

func (v valuerSlice) Value() (driver.Value, error) { return "x", nil }

func TestInExpansion(t *testing.T) {
	u := uuid.New()
	tests := []struct {
		name string
		expr Expr
		sql  string
		args []any
	}{
		{"int slice", Col("a").In([]int{1, 2}), `"a" IN ($1, $2)`, []any{1, 2}},
		{"string slice", Col("a").In([]string{"x", "y"}), `"a" IN ($1, $2)`, []any{"x", "y"}},
		{"any slice", Col("a").In([]any{1, "y"}), `"a" IN ($1, $2)`, []any{1, "y"}},
		{"uuid", Col("a").In(u), `"a" IN ($1)`, []any{u}},
		{"bytes", Col("a").In([]byte("ab")), `"a" IN ($1)`, []any{[]byte("ab")}},
		{"json raw", Col("a").In(json.RawMessage(`{}`)), `"a" IN ($1)`, []any{json.RawMessage(`{}`)}},
		{"net ip", Col("a").In(net.ParseIP("127.0.0.1")), `"a" IN ($1)`, []any{net.ParseIP("127.0.0.1")}},
		{"valuer slice", Col("a").In(valuerSlice{1, 2}), `"a" IN ($1)`, []any{valuerSlice{1, 2}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql, args, err := render(Postgres(), tt.expr)
			require.NoError(t, err)
			assert.Equal(t, tt.sql, sql)
			assert.Equal(t, tt.args, args)
		})
	}
}

func TestBooleanLogic(t *testing.T) {
	sql, args, err := render(Postgres(), Or(And(Col("a").Eq(1), Col("b").Eq(2)), Col("c").Eq(3)))
	require.NoError(t, err)
	assert.Equal(t, `(("a" = $1 AND "b" = $2) OR "c" = $3)`, sql)
	assert.Equal(t, []any{1, 2, 3}, args)

	sql, args, err = render(Postgres(), And(Col("t").Eq(1), Raw("a OR b")))
	require.NoError(t, err)
	assert.Equal(t, `("t" = $1 AND (a OR b))`, sql)
	assert.Equal(t, []any{1}, args)

	sql, args, err = render(Postgres(), Not(Col("a").Eq(1)))
	require.NoError(t, err)
	assert.Equal(t, `NOT ("a" = $1)`, sql)
	assert.Equal(t, []any{1}, args)
}

func TestMatch(t *testing.T) {
	sql, args, err := render(Postgres(), Match(map[string]any{"b": 2, "a": 1}))
	require.NoError(t, err)
	assert.Equal(t, `("a" = $1 AND "b" = $2)`, sql)
	assert.Equal(t, []any{1, 2}, args)

	sql, args, err = render(Postgres(), Match(map[string]any{"a": nil}))
	require.NoError(t, err)
	assert.Equal(t, `"a" IS NULL`, sql)
	assert.Equal(t, []any{}, args)

	_, _, err = render(Postgres(), Match(map[string]any{}))
	assert.True(t, errors.Is(err, ErrEmptyMatch))
}

func TestLikeHelpers(t *testing.T) {
	sql, args, err := render(Postgres(), Col("a").Contains("50%_x!"))
	require.NoError(t, err)
	assert.Equal(t, `"a" LIKE $1 ESCAPE '!'`, sql)
	assert.Equal(t, []any{"%50!%!_x!!%"}, args)

	sql, args, err = render(Postgres(), Col("a").HasPrefix("50%_x!"))
	require.NoError(t, err)
	assert.Equal(t, `"a" LIKE $1 ESCAPE '!'`, sql)
	assert.Equal(t, []any{"50!%!_x!!%"}, args)

	sql, args, err = render(Postgres(), Col("a").HasSuffix("50%_x!"))
	require.NoError(t, err)
	assert.Equal(t, `"a" LIKE $1 ESCAPE '!'`, sql)
	assert.Equal(t, []any{"%50!%!_x!!"}, args)

	sql, args, err = render(ClickHouse(), Col("a").Contains(`50%_x\`))
	require.NoError(t, err)
	assert.Equal(t, `"a" LIKE ?`, sql)
	assert.Equal(t, []any{`%50\%\_x\\%`}, args)
}

func TestRaw(t *testing.T) {
	arr := []int{1, 2, 3}
	sql, args, err := render(Postgres(), Raw("x @> ?::int[] AND y = '??'", arr))
	require.NoError(t, err)
	assert.Equal(t, `x @> $1::int[] AND y = '?'`, sql)
	assert.Equal(t, []any{arr}, args)

	sql, args, err = render(Postgres(), Raw("??"))
	require.NoError(t, err)
	assert.Equal(t, "?", sql)
	assert.Equal(t, []any{}, args)

	_, _, err = render(ClickHouse(), Raw("??"))
	assert.True(t, errors.Is(err, ErrRawPlaceholder))

	sql, args, err = render(Postgres(), Raw("lower(?)", Col("a")))
	require.NoError(t, err)
	assert.Equal(t, `lower("a")`, sql)
	assert.Equal(t, []any{}, args)

	_, _, err = render(Postgres(), Raw("? ?", 1))
	assert.True(t, errors.Is(err, ErrRawArgs))

	forbidden := []string{"a $1 b", "a ?1 b", "a -? b", "a $? b", `a \? b`}
	for _, dialect := range []Dialect{Postgres(), SQLite(), ClickHouse(), Generic()} {
		for _, s := range forbidden {
			t.Run(dialect.Name()+"/"+s, func(t *testing.T) {
				_, _, err := render(dialect, Raw(s))
				assert.True(t, errors.Is(err, ErrRawPlaceholder), "got %v", err)
			})
		}
	}
}

func TestInt(t *testing.T) {
	sql, _, err := render(Postgres(), Int(7))
	require.NoError(t, err)
	assert.Equal(t, "7", sql)

	sql, _, err = render(Postgres(), Int(-3))
	require.NoError(t, err)
	assert.Equal(t, "(-3)", sql)

	_, _, err = render(Postgres(), Raw("x -?", Int(-3)))
	assert.True(t, errors.Is(err, ErrRawPlaceholder))
}

func TestNilExpr(t *testing.T) {
	_, _, err := render(Postgres(), And(nil))
	assert.True(t, errors.Is(err, ErrNilExpr))

	_, _, err = render(Postgres(), Not(nil))
	assert.True(t, errors.Is(err, ErrNilExpr))

	_, _, err = render(Postgres(), Col("a").Eq(Value{}))
	assert.True(t, errors.Is(err, ErrNilExpr))
}

func TestTrivialFlag(t *testing.T) {
	assert.True(t, And().trivial)
	assert.True(t, And(And()).trivial)
	assert.True(t, Col("a").NotIn().trivial)

	assert.False(t, And(Col("a").Eq(1)).trivial)
	assert.False(t, Or().trivial)
	assert.False(t, Col("a").In().trivial)
}

func TestAliasSinglePart(t *testing.T) {
	_, _, err := render(Postgres(), Col("a").As("x.y"))
	assert.True(t, errors.Is(err, ErrInvalidIdentifier))

	_, _, err = render(Postgres(), Col("a").As("*"))
	assert.True(t, errors.Is(err, ErrInvalidIdentifier))
}

func TestOrder(t *testing.T) {
	sql, _, err := render(Postgres(), Col("a").Asc())
	require.NoError(t, err)
	assert.Equal(t, `"a" ASC`, sql)

	sql, _, err = render(Postgres(), Col("a").Desc())
	require.NoError(t, err)
	assert.Equal(t, `"a" DESC`, sql)

	sql, _, err = render(Postgres(), Col("a").Asc().NullsFirst())
	require.NoError(t, err)
	assert.Equal(t, `"a" ASC NULLS FIRST`, sql)

	sql, _, err = render(Postgres(), Col("a").Desc().NullsLast())
	require.NoError(t, err)
	assert.Equal(t, `"a" DESC NULLS LAST`, sql)
}

func TestValuesNeverInlined(t *testing.T) {
	hostile := `x\' OR 1=1 --`
	for _, dialect := range []Dialect{Postgres(), SQLite(), ClickHouse(), Generic()} {
		t.Run(dialect.Name(), func(t *testing.T) {
			sql, args, err := render(dialect, Col("a").Eq(hostile))
			require.NoError(t, err)
			assert.NotContains(t, sql, "OR 1=1")
			assert.Equal(t, []any{hostile}, args)
		})
	}
}
