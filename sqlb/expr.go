package sqlb

import (
	"database/sql/driver"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

// isNilValue reports whether v is untyped nil, a nil pointer, or a nil
// interface.
func isNilValue(v any) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Ptr, reflect.Interface:
		return rv.IsNil()
	default:
		return false
	}
}

// Expr is any renderable SQL fragment. It is sealed: only this package can
// implement it.
type Expr interface{ render(w *writer) }

// Value is any scalar or boolean expression. All comparison methods live
// on Value. The zero Value is invalid and fails to render with
// ErrNilExpr.
type Value struct {
	fn      func(w *writer)
	isRaw   bool
	trivial bool
	never   bool
}

func (v Value) render(w *writer) {
	if v.fn == nil {
		w.fail(ErrNilExpr)
		return
	}
	v.fn(w)
}

// renderExpr renders e, wrapping it in parentheses if it is a Raw Value,
// so a Raw operand cannot change the precedence of its surrounding
// expression.
func renderExpr(w *writer, e Expr) {
	if v, ok := e.(Value); ok && v.isRaw {
		w.keyword("(")
		v.render(w)
		w.keyword(")")
		return
	}
	e.render(w)
}

// renderOperand renders x: an Expr is rendered (wrapped if Raw), anything
// else is bound via arg.
func renderOperand(w *writer, x any) {
	if e, ok := x.(Expr); ok {
		renderExpr(w, e)
		return
	}
	w.arg(x)
}

// Col renders name as a quoted, escaped identifier.
func Col(name string) Value {
	return Value{fn: func(w *writer) {
		w.ident(name)
	}}
}

// Val binds v as an argument. Val(nil) renders the keyword NULL.
// Val(expr) renders expr in place.
func Val(v any) Value {
	if vv, ok := v.(Value); ok {
		return vv
	}
	if e, ok := v.(Expr); ok {
		return Value{fn: func(w *writer) {
			e.render(w)
		}}
	}
	if isNilValue(v) {
		return Value{fn: func(w *writer) {
			w.keyword("NULL")
		}}
	}
	return Value{fn: func(w *writer) {
		w.arg(v)
	}}
}

// Int renders n as an inline integer literal. Negative values are
// parenthesized.
func Int(n int64) Value {
	return Value{fn: func(w *writer) {
		s := strconv.FormatInt(n, 10)
		if n < 0 {
			w.keyword("(" + s + ")")
			return
		}
		w.keyword(s)
	}}
}

func isDigit(b byte) bool { return b >= '0' && b <= '9' }

// validateRaw checks sql text against the forbidden-placeholder rules and
// returns the number of `?` markers.
func validateRaw(d Dialect, s string) (int, error) {
	markers := 0
	n := len(s)
	i := 0
	for i < n {
		c := s[i]
		switch c {
		case '?':
			if i+1 < n && s[i+1] == '?' {
				if d.quote == quoteClickHouse {
					return 0, ErrRawPlaceholder
				}
				// A literal '?' immediately followed by a digit would
				// read back as an ordinal placeholder (SQLite binds `?N`
				// by ordinal).
				if i+2 < n && isDigit(s[i+2]) {
					return 0, ErrRawPlaceholder
				}
				i += 2
				continue
			}
			if i+1 < n && isDigit(s[i+1]) {
				return 0, ErrRawPlaceholder
			}
			if i > 0 {
				switch s[i-1] {
				case '-', '$', '\\':
					return 0, ErrRawPlaceholder
				}
			}
			markers++
			i++
		case '$':
			if i+1 < n && isDigit(s[i+1]) {
				return 0, ErrRawPlaceholder
			}
			i++
		default:
			i++
		}
	}
	return markers, nil
}

// Raw renders sql text verbatim, substituting each `?` marker with the
// corresponding argument (bound, or rendered inline if it is an Expr).
// `??` writes a literal `?`, except on ClickHouse where it is rejected.
// sql is trusted input — never build it from request data.
func Raw(sql string, args ...any) Value {
	return Value{
		isRaw: true,
		fn: func(w *writer) {
			markers, err := validateRaw(w.d, sql)
			if err != nil {
				w.fail(err)
				return
			}
			if markers != len(args) {
				w.fail(ErrRawArgs)
				return
			}
			n := len(sql)
			i := 0
			argIdx := 0
			for i < n {
				c := sql[i]
				if c == '?' && i+1 < n && sql[i+1] == '?' {
					w.buf.WriteByte('?')
					i += 2
					continue
				}
				if c == '?' {
					w.arg(args[argIdx])
					argIdx++
					i++
					continue
				}
				w.buf.WriteByte(c)
				i++
			}
		},
	}
}

type star struct{}

func (star) render(w *writer) { w.keyword("*") }

// Star renders the unquoted `*`.
func Star() Expr { return star{} }

// Sub renders a scalar subquery: "(<select>)". It shares w with the outer
// statement so placeholders keep counting across nesting.
func Sub(q *SelectBuilder) Value {
	return Value{fn: func(w *writer) {
		if q == nil {
			w.fail(ErrNilExpr)
			return
		}
		w.keyword("(")
		q.renderSelect(w)
		w.keyword(")")
	}}
}

// Exists renders "EXISTS (<select>)". If q's combined WHERE is
// syntactically never (an empty result set) and q has no UNION members,
// the returned Value is flagged never for the trivially-true/false guards
// in delete.go/update.go: this over-approximates (e.g. an aggregate
// subquery over an empty set still returns one row), which only makes the
// guard refuse more often, never less.
func Exists(q *SelectBuilder) Value {
	return Value{
		never: q != nil && q.whereIsNever(),
		fn: func(w *writer) {
			if q == nil {
				w.fail(ErrNilExpr)
				return
			}
			w.keyword("EXISTS (")
			q.renderSelect(w)
			w.keyword(")")
		},
	}
}

// NotExists renders "NOT EXISTS (<select>)". See Exists for the trivial
// flag.
func NotExists(q *SelectBuilder) Value {
	return Value{
		trivial: q != nil && q.whereIsNever(),
		fn: func(w *writer) {
			if q == nil {
				w.fail(ErrNilExpr)
				return
			}
			w.keyword("NOT EXISTS (")
			q.renderSelect(w)
			w.keyword(")")
		},
	}
}

// renderBoolList renders exprs joined by sep, wrapped in parentheses,
// except when there is exactly one element (rendered bare) or none
// (emptyLit, wrapped in parentheses).
func renderBoolList(w *writer, exprs []Expr, sep, emptyLit string) {
	if len(exprs) == 0 {
		w.keyword("(" + emptyLit + ")")
		return
	}
	if len(exprs) == 1 {
		if exprs[0] == nil {
			w.fail(ErrNilExpr)
			return
		}
		renderExpr(w, exprs[0])
		return
	}
	w.keyword("(")
	for i, e := range exprs {
		if e == nil {
			w.fail(ErrNilExpr)
			return
		}
		if i > 0 {
			w.keyword(" " + sep + " ")
		}
		renderExpr(w, e)
	}
	w.keyword(")")
}

// And renders exprs joined by AND, in parentheses (unless there is exactly
// one). And() renders (1=1).
func And(exprs ...Expr) Value {
	trivial := true
	never := false
	for _, e := range exprs {
		v, ok := e.(Value)
		if !ok || !v.trivial {
			trivial = false
		}
		if ok && v.never {
			never = true
		}
	}
	return Value{
		trivial: trivial,
		never:   never,
		fn: func(w *writer) {
			renderBoolList(w, exprs, "AND", "1=1")
		},
	}
}

// Or renders exprs joined by OR, in parentheses (unless there is exactly
// one). Or() renders (1=0).
func Or(exprs ...Expr) Value {
	trivial := false
	never := true
	for _, e := range exprs {
		v, ok := e.(Value)
		if ok && v.trivial {
			trivial = true
		}
		if !ok || !v.never {
			never = false
		}
	}
	return Value{
		trivial: trivial,
		never:   never,
		fn: func(w *writer) {
			renderBoolList(w, exprs, "OR", "1=0")
		},
	}
}

// Not renders NOT (e). If e is a Value, Not swaps its trivial/never flags:
// the negation of an always-true condition is always-false and vice
// versa.
func Not(e Expr) Value {
	var trivial, never bool
	if v, ok := e.(Value); ok {
		trivial, never = v.never, v.trivial
	}
	return Value{
		trivial: trivial,
		never:   never,
		fn: func(w *writer) {
			if e == nil {
				w.fail(ErrNilExpr)
				return
			}
			w.keyword("NOT (")
			e.render(w)
			w.keyword(")")
		},
	}
}

// Match renders an AND of equality comparisons, one per field, with keys
// sorted for deterministic output. Match(nil) or an empty map fails with
// ErrEmptyMatch.
func Match(fields map[string]any) Value {
	if len(fields) == 0 {
		return Value{fn: func(w *writer) {
			w.fail(ErrEmptyMatch)
		}}
	}
	keys := make([]string, 0, len(fields))
	for k := range fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	exprs := make([]Expr, len(keys))
	for i, k := range keys {
		exprs[i] = Col(k).Eq(fields[k])
	}
	return And(exprs...)
}

func compare(v Value, op string, x any) Value {
	return Value{fn: func(w *writer) {
		renderExpr(w, v)
		w.keyword(" " + op + " ")
		renderOperand(w, x)
	}}
}

// Eq renders "v = x", or "v IS NULL" when x is nil.
func (v Value) Eq(x any) Value {
	if isNilValue(x) {
		return Value{fn: func(w *writer) {
			renderExpr(w, v)
			w.keyword(" IS NULL")
		}}
	}
	return compare(v, "=", x)
}

// Neq renders "v <> x", or "v IS NOT NULL" when x is nil.
func (v Value) Neq(x any) Value {
	if isNilValue(x) {
		return Value{fn: func(w *writer) {
			renderExpr(w, v)
			w.keyword(" IS NOT NULL")
		}}
	}
	return compare(v, "<>", x)
}

func (v Value) Gt(x any) Value  { return compare(v, ">", x) }
func (v Value) Gte(x any) Value { return compare(v, ">=", x) }
func (v Value) Lt(x any) Value  { return compare(v, "<", x) }
func (v Value) Lte(x any) Value { return compare(v, "<=", x) }

// expandInValues expands a single slice argument (other than []byte) into
// its elements, unless it implements driver.Valuer.
func expandInValues(values []any) []any {
	if len(values) != 1 || values[0] == nil {
		return values
	}
	v := values[0]
	if _, ok := v.(driver.Valuer); ok {
		return values
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Slice || rv.Type().Elem().Kind() == reflect.Uint8 {
		return values
	}
	out := make([]any, rv.Len())
	for i := range out {
		out[i] = rv.Index(i).Interface()
	}
	return out
}

func inExpr(v Value, values []any, negate bool) Value {
	expanded := expandInValues(values)
	empty := len(expanded) == 0
	return Value{
		trivial: negate && empty,
		never:   !negate && empty,
		fn: func(w *writer) {
			if len(expanded) == 0 {
				if negate {
					w.keyword("1=1")
				} else {
					w.keyword("1=0")
				}
				return
			}
			renderExpr(w, v)
			if negate {
				w.keyword(" NOT IN (")
			} else {
				w.keyword(" IN (")
			}
			for i, e := range expanded {
				if i > 0 {
					w.keyword(", ")
				}
				renderOperand(w, e)
			}
			w.keyword(")")
		},
	}
}

// subqueryInExpr renders "v IN (<select>)" / "v NOT IN (<select>)",
// sharing w with the outer statement so placeholders keep counting across
// nesting. See Exists for the trivial/never propagation.
func subqueryInExpr(v Value, sb *SelectBuilder, negate bool) Value {
	never := sb != nil && sb.whereIsNever()
	trivial := negate && never
	never = !negate && never
	return Value{
		trivial: trivial,
		never:   never,
		fn: func(w *writer) {
			if sb == nil {
				w.fail(ErrNilExpr)
				return
			}
			renderExpr(w, v)
			if negate {
				w.keyword(" NOT IN (")
			} else {
				w.keyword(" IN (")
			}
			sb.renderSelect(w)
			w.keyword(")")
		},
	}
}

// In renders "v IN (values...)", expanding a single slice argument. With
// no values (after expansion) it renders 1=0. A single *SelectBuilder
// argument renders "v IN (<select>)".
func (v Value) In(values ...any) Value {
	if len(values) == 1 {
		if sb, ok := values[0].(*SelectBuilder); ok {
			return subqueryInExpr(v, sb, false)
		}
	}
	return inExpr(v, values, false)
}

// NotIn renders "v NOT IN (values...)", expanding a single slice
// argument. With no values (after expansion) it renders 1=1. A single
// *SelectBuilder argument renders "v NOT IN (<select>)".
func (v Value) NotIn(values ...any) Value {
	if len(values) == 1 {
		if sb, ok := values[0].(*SelectBuilder); ok {
			return subqueryInExpr(v, sb, true)
		}
	}
	return inExpr(v, values, true)
}

// Like renders "v LIKE p".
func (v Value) Like(p any) Value { return compare(v, "LIKE", p) }

// NotLike renders "v NOT LIKE p".
func (v Value) NotLike(p any) Value { return compare(v, "NOT LIKE", p) }

func iLikeExpr(v Value, op string, p any) Value {
	return Value{fn: func(w *writer) {
		if !w.d.Has(FeatureILike) {
			w.fail(fmt.Errorf("%w: %s", ErrUnsupported, op))
			return
		}
		renderExpr(w, v)
		w.keyword(" " + op + " ")
		renderOperand(w, p)
	}}
}

// ILike renders "v ILIKE p". ErrUnsupported on dialects without ILIKE.
func (v Value) ILike(p any) Value { return iLikeExpr(v, "ILIKE", p) }

// NotILike renders "v NOT ILIKE p". ErrUnsupported on dialects without
// ILIKE.
func (v Value) NotILike(p any) Value { return iLikeExpr(v, "NOT ILIKE", p) }

var (
	likeEscapeStd        = strings.NewReplacer("!", "!!", "%", "!%", "_", "!_")
	likeEscapeClickHouse = strings.NewReplacer(`\`, `\\`, "%", `\%`, "_", `\_`)
)

func likeHelper(v Value, s string, leading, trailing bool) Value {
	return Value{fn: func(w *writer) {
		var escaped string
		clickhouse := w.d.quote == quoteClickHouse
		if clickhouse {
			escaped = likeEscapeClickHouse.Replace(s)
		} else {
			escaped = likeEscapeStd.Replace(s)
		}
		pattern := escaped
		if leading {
			pattern = "%" + pattern
		}
		if trailing {
			pattern += "%"
		}
		renderExpr(w, v)
		w.keyword(" LIKE ")
		w.arg(pattern)
		if !clickhouse {
			w.keyword(" ESCAPE '!'")
		}
	}}
}

// Contains renders a LIKE match for "%s%", with LIKE wildcards in s
// escaped.
func (v Value) Contains(s string) Value { return likeHelper(v, s, true, true) }

// HasPrefix renders a LIKE match for "s%", with LIKE wildcards in s
// escaped.
func (v Value) HasPrefix(s string) Value { return likeHelper(v, s, false, true) }

// HasSuffix renders a LIKE match for "%s", with LIKE wildcards in s
// escaped.
func (v Value) HasSuffix(s string) Value { return likeHelper(v, s, true, false) }

// Between renders "v BETWEEN lo AND hi".
func (v Value) Between(lo, hi any) Value {
	return Value{fn: func(w *writer) {
		renderExpr(w, v)
		w.keyword(" BETWEEN ")
		renderOperand(w, lo)
		w.keyword(" AND ")
		renderOperand(w, hi)
	}}
}

// NotBetween renders "v NOT BETWEEN lo AND hi".
func (v Value) NotBetween(lo, hi any) Value {
	return Value{fn: func(w *writer) {
		renderExpr(w, v)
		w.keyword(" NOT BETWEEN ")
		renderOperand(w, lo)
		w.keyword(" AND ")
		renderOperand(w, hi)
	}}
}

// IsNull renders "v IS NULL".
func (v Value) IsNull() Value {
	return Value{fn: func(w *writer) {
		renderExpr(w, v)
		w.keyword(" IS NULL")
	}}
}

// IsNotNull renders "v IS NOT NULL".
func (v Value) IsNotNull() Value {
	return Value{fn: func(w *writer) {
		renderExpr(w, v)
		w.keyword(" IS NOT NULL")
	}}
}

// As renders "v AS alias". alias must be a single-part identifier.
func (v Value) As(alias string) Expr {
	return Value{fn: func(w *writer) {
		if alias == "*" || strings.Contains(alias, ".") {
			w.fail(fmt.Errorf("%w: %q", ErrInvalidIdentifier, alias))
			return
		}
		renderExpr(w, v)
		w.keyword(" AS ")
		w.ident(alias)
	}}
}

// Order is an ORDER BY item. It implements Expr.
type Order struct {
	fn func(w *writer)
}

func (o Order) render(w *writer) {
	if o.fn == nil {
		w.fail(ErrNilExpr)
		return
	}
	o.fn(w)
}

// Asc renders "v ASC".
func (v Value) Asc() Order {
	return Order{fn: func(w *writer) {
		renderExpr(w, v)
		w.keyword(" ASC")
	}}
}

// Desc renders "v DESC".
func (v Value) Desc() Order {
	return Order{fn: func(w *writer) {
		renderExpr(w, v)
		w.keyword(" DESC")
	}}
}

// NullsFirst appends "NULLS FIRST".
func (o Order) NullsFirst() Order {
	prev := o.fn
	return Order{fn: func(w *writer) {
		if prev == nil {
			w.fail(ErrNilExpr)
			return
		}
		prev(w)
		w.keyword(" NULLS FIRST")
	}}
}

// NullsLast appends "NULLS LAST".
func (o Order) NullsLast() Order {
	prev := o.fn
	return Order{fn: func(w *writer) {
		if prev == nil {
			w.fail(ErrNilExpr)
			return
		}
		prev(w)
		w.keyword(" NULLS LAST")
	}}
}
