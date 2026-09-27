package sqlb

import (
	"database/sql"
	"database/sql/driver"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIdentQuoting(t *testing.T) {
	tests := []struct {
		dialect Dialect
		name    string
		in      string
		want    string
	}{
		{Postgres(), "postgres a", "a", `"a"`},
		{Postgres(), "postgres s.t", "s.t", `"s"."t"`},
		{Postgres(), "postgres t.*", "t.*", `"t".*`},
		{Postgres(), `postgres a"b`, `a"b`, `"a""b"`},
		{Postgres(), "postgres c\\d", `c\d`, `"c\d"`},
		{SQLite(), "sqlite a", "a", "`a`"},
		{SQLite(), "sqlite s.t", "s.t", "`s`.`t`"},
		{SQLite(), "sqlite t.*", "t.*", "`t`.*"},
		{SQLite(), `sqlite a"b`, `a"b`, "`a\"b`"},
		{SQLite(), "sqlite a`b", "a`b", "`a``b`"},
		{ClickHouse(), "clickhouse a", "a", `"a"`},
		{ClickHouse(), "clickhouse s.t", "s.t", `"s"."t"`},
		{ClickHouse(), `clickhouse a"b`, `a"b`, `"a""b"`},
		{ClickHouse(), "clickhouse c\\d", `c\d`, `"c\\d"`},
		{Generic(), "generic a", "a", `"a"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql, _, err := render(tt.dialect, Col(tt.in))
			require.NoError(t, err)
			assert.Equal(t, tt.want, sql)
		})
	}
}

func TestIdentRejects(t *testing.T) {
	common := []string{"", "a..b", "a\x00b"}
	for _, dialect := range []Dialect{Postgres(), SQLite(), ClickHouse(), Generic()} {
		for _, in := range common {
			t.Run(dialect.Name()+"/"+in, func(t *testing.T) {
				_, _, err := render(dialect, Col(in))
				assert.True(t, errors.Is(err, ErrInvalidIdentifier), "got %v", err)
			})
		}
	}

	for _, in := range []string{"a?b", "a@b", "a$1", "a{b", "a}b", "{a:b}"} {
		t.Run("clickhouse/"+in, func(t *testing.T) {
			_, _, err := render(ClickHouse(), Col(in))
			assert.True(t, errors.Is(err, ErrInvalidIdentifier), "got %v", err)
		})
		t.Run("postgres/"+in, func(t *testing.T) {
			_, _, err := render(Postgres(), Col(in))
			assert.NoError(t, err)
		})
	}
}

func TestPlaceholders(t *testing.T) {
	tests := []struct {
		dialect Dialect
		want    string
	}{
		{Postgres(), `"a" = $1`},
		{SQLite(), "`a` = ?"},
		{ClickHouse(), `"a" = ?`},
		{Generic(), `"a" = ?`},
	}
	for _, tt := range tests {
		t.Run(tt.dialect.Name(), func(t *testing.T) {
			w := &writer{d: tt.dialect}
			w.ident("a")
			w.keyword(" = ")
			w.arg(1)
			sql, args, err := w.finish()
			require.NoError(t, err)
			assert.Equal(t, tt.want, sql)
			assert.Equal(t, []any{1}, args)
		})
	}
}

type validValuer struct{ v int }

func (v validValuer) Value() (driver.Value, error) { return v.v, nil }

func TestUnsafeValuesClickHouse(t *testing.T) {
	unsafe := []any{
		map[string]int{"k": 1},
		struct{ X int }{1},
		sql.Named("x", 1),
		[]any{map[string]any{"k'": 1}},
		[]any{[]any{map[string]any{"k": 1}}},
	}
	for i, v := range unsafe {
		t.Run("unsafe", func(t *testing.T) {
			w := &writer{d: ClickHouse()}
			w.arg(v)
			_, _, err := w.finish()
			assert.True(t, errors.Is(err, ErrUnsafeValue), "case %d: got %v", i, err)
		})
	}

	safe := []any{
		time.Now(),
		validValuer{1},
		func() *int { n := 1; return &n }(),
		[]string{"a", "b"},
		[][]int{{1, 2}, {3, 4}},
	}
	for i, v := range safe {
		t.Run("safe", func(t *testing.T) {
			w := &writer{d: ClickHouse()}
			w.arg(v)
			_, _, err := w.finish()
			assert.NoError(t, err, "case %d", i)
		})
	}

	t.Run("map accepted on postgres", func(t *testing.T) {
		w := &writer{d: Postgres()}
		w.arg(map[string]int{"k": 1})
		_, _, err := w.finish()
		assert.NoError(t, err)
	})
}

type stringerErr string

func (s stringerErr) String() string { return string(s) }

type ptrStringer struct{ v string }

func (p *ptrStringer) String() string { return p.v }

type badErr int

func (b badErr) Error() string { return "bad" }

func TestNestedValuersClickHouse(t *testing.T) {
	t.Run("nested NullString in slice is unsafe", func(t *testing.T) {
		w := &writer{d: ClickHouse()}
		w.arg([]sql.NullString{{String: "x') OR 1=1 --", Valid: true}})
		_, _, err := w.finish()
		assert.True(t, errors.Is(err, ErrUnsafeValue))
	})

	t.Run("In expands the slice so each element is top-level", func(t *testing.T) {
		_, _, err := render(ClickHouse(), Col("x").In([]sql.NullInt64{{Int64: 1, Valid: true}}))
		assert.NoError(t, err)
	})

	t.Run("top-level NullString is safe", func(t *testing.T) {
		w := &writer{d: ClickHouse()}
		w.arg(sql.NullString{String: "x", Valid: true})
		_, _, err := w.finish()
		assert.NoError(t, err)
	})

	t.Run("top-level double pointer is unsafe", func(t *testing.T) {
		w := &writer{d: ClickHouse()}
		ns := sql.NullString{String: "x", Valid: true}
		p := &ns
		w.arg(&p)
		_, _, err := w.finish()
		assert.True(t, errors.Is(err, ErrUnsafeValue))
	})

	t.Run("nested struct implementing Stringer is safe", func(t *testing.T) {
		w := &writer{d: ClickHouse()}
		w.arg([]stringerErr{"a"})
		_, _, err := w.finish()
		assert.NoError(t, err)
	})

	t.Run("slice of pointer-receiver Stringer is safe", func(t *testing.T) {
		w := &writer{d: ClickHouse()}
		w.arg([]*ptrStringer{{v: "a"}})
		_, _, err := w.finish()
		assert.NoError(t, err)
	})

	t.Run("nested time.Time slice is safe", func(t *testing.T) {
		w := &writer{d: ClickHouse()}
		w.arg([]time.Time{time.Now()})
		_, _, err := w.finish()
		assert.NoError(t, err)
	})

	t.Run("nested NullString slice on postgres is safe", func(t *testing.T) {
		w := &writer{d: Postgres()}
		w.arg([]sql.NullString{{String: "x", Valid: true}})
		_, _, err := w.finish()
		assert.NoError(t, err)
	})

	t.Run("interface pointer to map is unsafe", func(t *testing.T) {
		x := any(map[string]any{"k') OR 1=1 --": 1})
		w := &writer{d: ClickHouse()}
		w.arg(&x)
		_, _, err := w.finish()
		assert.True(t, errors.Is(err, ErrUnsafeValue))
	})

	t.Run("interface pointer to NullString is unsafe", func(t *testing.T) {
		y := any(sql.NullString{String: "z", Valid: true})
		w := &writer{d: ClickHouse()}
		w.arg(&y)
		_, _, err := w.finish()
		assert.True(t, errors.Is(err, ErrUnsafeValue))
	})

	t.Run("slice of interface pointers to map is unsafe", func(t *testing.T) {
		m := any(map[string]any{"k": 1})
		w := &writer{d: ClickHouse()}
		w.arg([]*any{&m})
		_, _, err := w.finish()
		assert.True(t, errors.Is(err, ErrUnsafeValue))
	})

	t.Run("error without Stringer nested in slice is unsafe", func(t *testing.T) {
		w := &writer{d: ClickHouse()}
		w.arg([]badErr{badErr(1)})
		_, _, err := w.finish()
		assert.True(t, errors.Is(err, ErrUnsafeValue))
	})

	t.Run("top-level nil pointer to Valuer is safe and bound as nil", func(t *testing.T) {
		w := &writer{d: ClickHouse()}
		var p *sql.NullString
		w.arg(p)
		_, args, err := w.finish()
		assert.NoError(t, err)
		assert.Equal(t, []any{nil}, args)
	})
}

func TestNestedSliceViaIn(t *testing.T) {
	// Col("a").In([]any{[]any{map[string]any{"k": 1}}}) on ClickHouse -> ErrUnsafeValue.
	// In() is defined in expr.go (step 3); here we exercise the writer's
	// recursive check directly on the equivalent nested value.
	w := &writer{d: ClickHouse()}
	w.arg([]any{[]any{map[string]any{"k": 1}}})
	_, _, err := w.finish()
	assert.True(t, errors.Is(err, ErrUnsafeValue))
}

func TestArgRejectsBuilders(t *testing.T) {
	builders := []any{
		Select("a").From("t"),
		Delete("t"),
		Insert("t").Columns("a").Values(1),
		Update("t").Set("a", 1).All(),
		Select("a").From("t").As("s"),
		Table("t"),
	}
	for _, d := range []Dialect{Postgres(), SQLite(), ClickHouse(), Generic()} {
		for i, b := range builders {
			t.Run(d.Name(), func(t *testing.T) {
				w := &writer{d: d}
				w.arg(b)
				_, _, err := w.finish()
				assert.True(t, errors.Is(err, ErrInvalidColumn), "case %d: got %v", i, err)
			})
		}
	}
}

func TestTooManyArgs(t *testing.T) {
	w := &writer{d: Generic()}
	for i := 0; i < 1000; i++ {
		w.arg(i)
	}
	_, _, err := w.finish()
	assert.True(t, errors.Is(err, ErrTooManyArgs))
}

func TestFinishTwicePanics(t *testing.T) {
	w := &writer{d: Postgres()}
	w.keyword("x")
	_, _, _ = w.finish()
	assert.Panics(t, func() {
		_, _, _ = w.finish()
	})
}

func TestZeroDialect(t *testing.T) {
	_, _, err := render(Dialect{}, Col("a"))
	assert.True(t, errors.Is(err, ErrUnknownDialect))
}

func TestFirstErrorWins(t *testing.T) {
	w := &writer{d: Postgres()}
	w.ident("")               // sets ErrInvalidIdentifier
	w.fail(ErrUnknownDialect) // must not overwrite
	_, _, err := w.finish()
	assert.True(t, errors.Is(err, ErrInvalidIdentifier))
}

func TestQuoteIdent(t *testing.T) {
	got, err := Postgres().QuoteIdent("s.t")
	require.NoError(t, err)
	assert.Equal(t, `"s"."t"`, got)

	got, err = ClickHouse().QuoteIdent(`c\d`)
	require.NoError(t, err)
	assert.Equal(t, `"c\\d"`, got)

	got, err = SQLite().QuoteIdent("t")
	require.NoError(t, err)
	assert.Equal(t, "`t`", got)

	_, err = Postgres().QuoteIdent("")
	assert.True(t, errors.Is(err, ErrInvalidIdentifier))
}
