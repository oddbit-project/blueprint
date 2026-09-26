package sqlb

import (
	"fmt"
	"strings"
)

// Statement is anything that renders to a complete SQL string and its
// bound arguments.
type Statement interface {
	Build(d Dialect) (string, []any, error)
}

// TableRef names a table, with an optional alias.
type TableRef struct {
	name  string
	alias string
}

// Table names table for use as a FROM/DELETE target.
func Table(name string) TableRef {
	return TableRef{name: name}
}

// As sets a single-part alias for the table.
func (t TableRef) As(alias string) TableRef {
	t.alias = alias
	return t
}

// Col renders name qualified by the table's alias (if set) or its name.
func (t TableRef) Col(name string) Value {
	qualifier := t.alias
	if qualifier == "" {
		qualifier = t.name
	}
	return Col(qualifier + "." + name)
}

// Subquery is a SELECT usable as a FROM/JOIN source. It is not an Expr and
// cannot appear where an expression is expected.
type Subquery struct {
	sb    *SelectBuilder
	alias string
}

// writeAlias writes alias as a single-part identifier, failing with
// ErrInvalidIdentifier for "*" or a multi-part name.
func writeAlias(w *writer, alias string) {
	if alias == "*" || strings.Contains(alias, ".") {
		w.fail(fmt.Errorf("%w: %q", ErrInvalidIdentifier, alias))
		return
	}
	w.ident(alias)
}

// renderColumn renders c: a string is a column name, an Expr is rendered
// as-is, anything else fails with ErrInvalidColumn.
func renderColumn(w *writer, c any) {
	switch v := c.(type) {
	case string:
		Col(v).render(w)
	case Expr:
		renderExpr(w, v)
	default:
		w.fail(ErrInvalidColumn)
	}
}

// renderSource renders src as a FROM/DELETE-target: a string (dotted names
// allowed), a TableRef, or a Subquery (which requires an alias). A bare
// *SelectBuilder fails with ErrNeedAlias; nil, "" or anything else fails
// with ErrNoTable.
func renderSource(w *writer, src any) {
	switch v := src.(type) {
	case nil:
		w.fail(ErrNoTable)
	case string:
		if v == "" {
			w.fail(ErrNoTable)
			return
		}
		w.ident(v)
	case TableRef:
		if v.name == "" {
			w.fail(ErrNoTable)
			return
		}
		w.ident(v.name)
		if v.alias != "" {
			w.keyword(" AS ")
			writeAlias(w, v.alias)
		}
	case Subquery:
		if v.sb == nil {
			w.fail(ErrNoTable)
			return
		}
		if v.alias == "" {
			w.fail(ErrNeedAlias)
			return
		}
		w.keyword("(")
		v.sb.renderSelect(w)
		w.keyword(") AS ")
		writeAlias(w, v.alias)
	case *SelectBuilder:
		w.fail(ErrNeedAlias)
	default:
		w.fail(ErrNoTable)
	}
}
