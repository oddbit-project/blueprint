package sqlb

import "fmt"

// DeleteBuilder builds a DELETE statement. Builders are immutable: every
// method returns a new value and never modifies the receiver.
type DeleteBuilder struct {
	table     any
	where     []Expr
	all       bool
	returning []any
}

var _ Statement = (*DeleteBuilder)(nil)

// Delete starts a DELETE statement against table: a string or TableRef. A
// Subquery fails with ErrNoTable at Build.
func Delete(table any) *DeleteBuilder {
	return &DeleteBuilder{table: table}
}

func (d *DeleteBuilder) clone() *DeleteBuilder {
	c := *d
	c.where = append([]Expr(nil), d.where...)
	c.returning = append([]any(nil), d.returning...)
	return &c
}

// Where appends conds, ANDed with any existing conditions.
func (d *DeleteBuilder) Where(conds ...Expr) *DeleteBuilder {
	c := d.clone()
	c.where = append(c.where, conds...)
	return c
}

// All allows the DELETE to build with no WHERE clause, or a trivially true
// one, affecting every row.
func (d *DeleteBuilder) All() *DeleteBuilder {
	c := d.clone()
	c.all = true
	return c
}

// Returning sets the RETURNING column list. Zero cols means no RETURNING
// clause.
func (d *DeleteBuilder) Returning(cols ...any) *DeleteBuilder {
	c := d.clone()
	c.returning = append([]any(nil), cols...)
	return c
}

// Build renders the DELETE statement under dialect dl using a fresh
// writer.
func (d *DeleteBuilder) Build(dl Dialect) (string, []any, error) {
	w := &writer{d: dl}
	if dl.Name() == "" {
		w.fail(ErrUnknownDialect)
		return w.finish()
	}
	d.render(w)
	return w.finish()
}

func (d *DeleteBuilder) render(w *writer) {
	w.keyword("DELETE FROM ")
	switch t := d.table.(type) {
	case string:
		if t == "" {
			w.fail(ErrNoTable)
			return
		}
		w.ident(t)
	case TableRef:
		if t.name == "" {
			w.fail(ErrNoTable)
			return
		}
		if t.alias != "" && w.d.quote == quoteClickHouse {
			w.fail(fmt.Errorf("%w: table alias in DELETE", ErrUnsupported))
			return
		}
		w.ident(t.name)
		if t.alias != "" {
			w.keyword(" AS ")
			writeAlias(w, t.alias)
		}
	default:
		w.fail(ErrNoTable)
		return
	}

	hasWhere := len(d.where) > 0
	var whereExpr Value
	if hasWhere {
		whereExpr = And(d.where...)
	}
	switch {
	case hasWhere && (d.all || !whereExpr.trivial):
		w.keyword(" WHERE ")
		renderExpr(w, whereExpr)
	case hasWhere:
		w.fail(ErrNoWhere)
		return
	case d.all:
		if w.d.quote == quoteClickHouse {
			w.keyword(" WHERE 1")
		}
	default:
		w.fail(ErrNoWhere)
		return
	}

	if len(d.returning) > 0 {
		if !w.d.Has(FeatureReturning) {
			w.fail(fmt.Errorf("%w: RETURNING", ErrUnsupported))
			return
		}
		w.keyword(" RETURNING ")
		for i, c := range d.returning {
			if i > 0 {
				w.keyword(", ")
			}
			renderColumn(w, c)
		}
	}
}
