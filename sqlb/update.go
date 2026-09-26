package sqlb

import (
	"fmt"
	"sort"
	"strings"
)

// UpdateBuilder builds an UPDATE statement. Builders are immutable: every
// method returns a new value and never modifies the receiver.
type UpdateBuilder struct {
	table any

	setCols []string
	setVals []any
	err     error

	where []Expr
	all   bool

	returning []any
}

var _ Statement = (*UpdateBuilder)(nil)

// Update starts an UPDATE statement against table: a string or a TableRef
// without an alias.
func Update(table any) *UpdateBuilder {
	return &UpdateBuilder{table: table}
}

func (b *UpdateBuilder) clone() *UpdateBuilder {
	c := *b
	c.setCols = append([]string(nil), b.setCols...)
	c.setVals = append([]any(nil), b.setVals...)
	c.where = append([]Expr(nil), b.where...)
	c.returning = append([]any(nil), b.returning...)
	return &c
}

// Set appends "col = v" to the SET list.
func (b *UpdateBuilder) Set(col string, v any) *UpdateBuilder {
	c := b.clone()
	c.setCols = append(c.setCols, col)
	c.setVals = append(c.setVals, v)
	return c
}

// SetMap appends one "col = v" per entry of m, in sorted key order.
func (b *UpdateBuilder) SetMap(m map[string]any) *UpdateBuilder {
	c := b.clone()
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		c.setCols = append(c.setCols, k)
		c.setVals = append(c.setVals, m[k])
	}
	return c
}

// SetRecord appends one "col = v" per updatable field of rec (a struct or
// pointer to struct), per opts. Values are extracted immediately (a
// snapshot): mutating rec afterwards does not change the statement. A
// shape or option error is deferred and reported at Build.
func (b *UpdateBuilder) SetRecord(rec any, opts ...RecordOption) *UpdateBuilder {
	c := b.clone()
	if c.err != nil {
		return c
	}
	var o recordOptions
	for _, opt := range opts {
		opt(&o)
	}
	cols, vals, err := recordValues(rec, false, o)
	if err != nil {
		c.err = err
		return c
	}
	c.setCols = append(c.setCols, cols...)
	c.setVals = append(c.setVals, vals...)
	return c
}

// Where appends conds, ANDed with any existing conditions.
func (b *UpdateBuilder) Where(conds ...Expr) *UpdateBuilder {
	c := b.clone()
	c.where = append(c.where, conds...)
	return c
}

// All allows the UPDATE to build with no WHERE clause, or a trivially true
// one, affecting every row.
func (b *UpdateBuilder) All() *UpdateBuilder {
	c := b.clone()
	c.all = true
	return c
}

// Returning sets the RETURNING column list.
func (b *UpdateBuilder) Returning(cols ...any) *UpdateBuilder {
	c := b.clone()
	c.returning = append([]any(nil), cols...)
	return c
}

// Build renders the UPDATE statement under dialect d using a fresh writer.
func (b *UpdateBuilder) Build(d Dialect) (string, []any, error) {
	w := &writer{d: d}
	if d.Name() == "" {
		w.fail(ErrUnknownDialect)
		return w.finish()
	}
	b.renderUpdate(w)
	return w.finish()
}

// renderUpdate renders the UPDATE statement into w. It is not named render
// so *UpdateBuilder does not satisfy Expr.
func (b *UpdateBuilder) renderUpdate(w *writer) {
	if !w.d.Has(FeatureUpdate) {
		w.fail(fmt.Errorf("%w: UPDATE", ErrUnsupported))
		return
	}
	if b.err != nil {
		w.fail(b.err)
		return
	}

	w.keyword("UPDATE ")
	renderWriteTable(w, b.table)

	if len(b.setCols) == 0 {
		w.fail(ErrNoColumns)
		return
	}
	seen := make(map[string]bool, len(b.setCols))
	for _, c := range b.setCols {
		if strings.Contains(c, ".") {
			w.fail(fmt.Errorf("%w: %q", ErrInvalidIdentifier, c))
			return
		}
		if seen[c] {
			w.fail(fmt.Errorf("%w: %q", ErrDuplicateColumn, c))
			return
		}
		seen[c] = true
	}

	w.keyword(" SET ")
	for i, c := range b.setCols {
		if i > 0 {
			w.keyword(", ")
		}
		w.ident(c)
		w.keyword(" = ")
		renderAssignValue(w, b.setVals[i])
	}

	hasWhere := len(b.where) > 0
	var whereExpr Value
	if hasWhere {
		whereExpr = And(b.where...)
	}
	switch {
	case hasWhere && (b.all || !whereExpr.trivial):
		w.keyword(" WHERE ")
		renderExpr(w, whereExpr)
	case hasWhere:
		w.fail(ErrNoWhere)
		return
	case b.all:
		// All() with no WHERE conditions at all: affect every row, no
		// WHERE clause rendered.
	default:
		w.fail(ErrNoWhere)
		return
	}

	if len(b.returning) > 0 {
		if !w.d.Has(FeatureReturning) {
			w.fail(fmt.Errorf("%w: RETURNING", ErrUnsupported))
			return
		}
		w.keyword(" RETURNING ")
		for i, c := range b.returning {
			if i > 0 {
				w.keyword(", ")
			}
			renderColumn(w, c)
		}
	}
}
