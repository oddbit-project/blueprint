package sqlb

import (
	"database/sql/driver"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

// writer accumulates SQL text and bound arguments for a single rendering
// pass. The first error recorded wins; later writes become no-ops.
type writer struct {
	d    Dialect
	buf  strings.Builder
	args []any
	err  error
	done bool
}

// keyword appends a trusted constant verbatim. It must never receive
// caller-supplied data.
func (w *writer) keyword(s string) {
	if w.err != nil {
		return
	}
	w.buf.WriteString(s)
}

// fail records the first error encountered during rendering.
func (w *writer) fail(err error) {
	if w.err == nil {
		w.err = err
	}
}

var clickHouseIdentBan = regexp.MustCompile(`\$[0-9]`)

// ident writes a quoted, escaped identifier. See sqlb/dialect.go for the
// rules by dialect.
func (w *writer) ident(name string) {
	if w.err != nil {
		return
	}
	if name == "" || strings.ContainsRune(name, 0) {
		w.fail(fmt.Errorf("%w: %q", ErrInvalidIdentifier, name))
		return
	}
	parts := strings.Split(name, ".")
	for _, p := range parts {
		if p == "" {
			w.fail(fmt.Errorf("%w: %q", ErrInvalidIdentifier, name))
			return
		}
	}
	if w.d.quote == quoteClickHouse {
		if strings.ContainsAny(name, "?@{}") || clickHouseIdentBan.MatchString(name) {
			w.fail(fmt.Errorf("%w: %q", ErrInvalidIdentifier, name))
			return
		}
	}
	for i, p := range parts {
		if i > 0 {
			w.buf.WriteByte('.')
		}
		if p == "*" {
			w.buf.WriteByte('*')
			continue
		}
		w.buf.WriteString(w.quotePart(p))
	}
}

// quotePart quotes a single, already-validated identifier part.
func (w *writer) quotePart(p string) string {
	switch w.d.quote {
	case quoteBacktick:
		return "`" + strings.ReplaceAll(p, "`", "``") + "`"
	case quoteClickHouse:
		p = strings.ReplaceAll(p, `\`, `\\`)
		return `"` + strings.ReplaceAll(p, `"`, `""`) + `"`
	default: // quoteDouble
		return `"` + strings.ReplaceAll(p, `"`, `""`) + `"`
	}
}

var clickHouseNamedValueTypes = map[string]bool{
	"NamedValue":     true,
	"NamedDateValue": true,
}

// isUnsafeClickHouseValue reports whether v is a value clickhouse-go's
// formatter would render unsafely (see plan 001's facts table). It
// recurses into slices and arrays other than []byte.
func isUnsafeClickHouseValue(v any) bool {
	if v == nil {
		return false
	}
	rv := reflect.ValueOf(v)
	for rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return false
		}
		rv = rv.Elem()
	}
	if !rv.IsValid() {
		return false
	}

	switch rv.Kind() {
	case reflect.Slice, reflect.Array:
		if rv.Type().Elem().Kind() == reflect.Uint8 {
			return false
		}
		for i := 0; i < rv.Len(); i++ {
			if isUnsafeClickHouseValue(rv.Index(i).Interface()) {
				return true
			}
		}
		return false
	case reflect.Map:
		return true
	case reflect.Struct:
		if _, ok := rv.Interface().(driver.Valuer); ok {
			return false
		}
		t := rv.Type()
		if t.PkgPath() == "time" && t.Name() == "Time" {
			return false
		}
		if t.Name() == "NamedArg" && strings.Contains(t.PkgPath(), "database/sql") {
			return true
		}
		if strings.Contains(t.PkgPath(), "clickhouse-go") && clickHouseNamedValueTypes[t.Name()] {
			return true
		}
		return true
	default:
		return false
	}
}

// arg binds v as a placeholder, or renders it inline if it implements
// Expr.
func (w *writer) arg(v any) {
	if w.err != nil {
		return
	}
	switch v.(type) {
	case *SelectBuilder, *DeleteBuilder, *InsertBuilder, *UpdateBuilder, Subquery, TableRef:
		w.fail(fmt.Errorf("%w: %T", ErrInvalidColumn, v))
		return
	}
	if e, ok := v.(Expr); ok {
		e.render(w)
		return
	}
	if w.d.quote == quoteClickHouse && isUnsafeClickHouseValue(v) {
		w.fail(fmt.Errorf("%w: %T", ErrUnsafeValue, v))
		return
	}
	w.args = append(w.args, v)
	if w.d.name == "postgres" {
		w.buf.WriteString("$" + strconv.Itoa(len(w.args)))
	} else {
		w.buf.WriteByte('?')
	}
}

// finish returns the accumulated SQL and arguments, or the first error
// encountered. Calling finish twice is a programming error and panics.
func (w *writer) finish() (string, []any, error) {
	if w.done {
		panic("sqlb: writer.finish called twice")
	}
	w.done = true
	if w.err != nil {
		return "", nil, w.err
	}
	if w.d.MaxArgs() > 0 && len(w.args) > w.d.MaxArgs() {
		return "", nil, ErrTooManyArgs
	}
	args := w.args
	if args == nil {
		args = []any{}
	}
	return w.buf.String(), args, nil
}

// render builds SQL and arguments for e under dialect d using a fresh
// writer.
func render(d Dialect, e Expr) (string, []any, error) {
	w := &writer{d: d}
	if d.Name() == "" {
		w.fail(ErrUnknownDialect)
	} else if e == nil {
		w.fail(ErrNilExpr)
	} else {
		e.render(w)
	}
	return w.finish()
}

// QuoteIdent quotes and escapes name per the dialect's identifier rules.
// It is for adapters that need to name an identifier outside a statement.
func (d Dialect) QuoteIdent(name string) (string, error) {
	w := &writer{d: d}
	if d.Name() == "" {
		return "", ErrUnknownDialect
	}
	w.ident(name)
	sql, _, err := w.finish()
	return sql, err
}
