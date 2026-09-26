package sqlb

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAggregates(t *testing.T) {
	sql, args, err := render(Postgres(), CountAll())
	require.NoError(t, err)
	assert.Equal(t, "COUNT(*)", sql)
	assert.Equal(t, []any{}, args)

	sql, _, err = render(Postgres(), Count("id"))
	require.NoError(t, err)
	assert.Equal(t, `COUNT("id")`, sql)

	sql, _, err = render(Postgres(), Count(Col("id")))
	require.NoError(t, err)
	assert.Equal(t, `COUNT("id")`, sql)

	sql, _, err = render(Postgres(), Sum("amount"))
	require.NoError(t, err)
	assert.Equal(t, `SUM("amount")`, sql)

	sql, _, err = render(Postgres(), Avg("amount"))
	require.NoError(t, err)
	assert.Equal(t, `AVG("amount")`, sql)

	sql, _, err = render(Postgres(), Min("amount"))
	require.NoError(t, err)
	assert.Equal(t, `MIN("amount")`, sql)

	sql, _, err = render(Postgres(), Max("amount"))
	require.NoError(t, err)
	assert.Equal(t, `MAX("amount")`, sql)
}

func TestFn(t *testing.T) {
	sql, args, err := render(Postgres(), Fn("lower", Col("a")).Eq("x"))
	require.NoError(t, err)
	assert.Equal(t, `lower("a") = $1`, sql)
	assert.Equal(t, []any{"x"}, args)

	for _, name := range []string{"lower(", "a b", ""} {
		t.Run(name, func(t *testing.T) {
			_, _, err := render(Postgres(), Fn(name))
			assert.True(t, errors.Is(err, ErrInvalidFunction), "got %v", err)
		})
	}
}

func TestCast(t *testing.T) {
	accepted := []string{"INTEGER", "numeric(10,2)", "text[]", "Nullable(String)", "LowCardinality(Nullable(String))", "DateTime64(3)"}
	for _, typ := range accepted {
		t.Run(typ, func(t *testing.T) {
			sql, _, err := render(Postgres(), Cast(Col("a"), typ))
			require.NoError(t, err)
			assert.Equal(t, `CAST("a" AS `+typ+`)`, sql)
		})
	}

	rejected := []string{"int; DROP", "text'", "a)("}
	for _, typ := range rejected {
		t.Run(typ, func(t *testing.T) {
			_, _, err := render(Postgres(), Cast(Col("a"), typ))
			assert.True(t, errors.Is(err, ErrInvalidType), "got %v", err)
		})
	}
}

func TestCase(t *testing.T) {
	sql, args, err := render(Postgres(), Sum(Case().When(Col("ok").Eq(true), Int(1)).Else(Int(0))))
	require.NoError(t, err)
	assert.Equal(t, `SUM(CASE WHEN "ok" = $1 THEN 1 ELSE 0 END)`, sql)
	assert.Equal(t, []any{true}, args)

	sql, args, err = render(Postgres(), Case().
		When(Col("a").Eq(1), "one").
		When(Col("a").Eq(2), "two").
		Else("other"))
	require.NoError(t, err)
	assert.Equal(t, `CASE WHEN "a" = $1 THEN $2 WHEN "a" = $3 THEN $4 ELSE $5 END`, sql)
	assert.Equal(t, []any{1, "one", 2, "two", "other"}, args)

	sql, args, err = render(Postgres(), Case().When(Col("a").Eq(1), 1).End())
	require.NoError(t, err)
	assert.Equal(t, `CASE WHEN "a" = $1 THEN $2 END`, sql)
	assert.Equal(t, []any{1, 1}, args)

	_, _, err = render(Postgres(), Case().End())
	assert.True(t, errors.Is(err, ErrEmptyCase))
}
