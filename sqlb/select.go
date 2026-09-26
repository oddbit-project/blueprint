package sqlb

import (
	"math"
	"strconv"
)

// SelectBuilder builds a SELECT statement. Builders are immutable: every
// method returns a new value and never modifies the receiver.
type SelectBuilder struct {
	cols      []any
	from      any
	fromSet   bool
	distinct  bool
	where     []Expr
	groupBy   []any
	having    []Expr
	orderBy   []any
	hasLimit  bool
	limit     uint64
	hasOffset bool
	offset    uint64

	joins []join

	ctes      []cte
	recursive bool
	unions    []unionMember

	final          bool
	hasSample      bool
	sampleIsRows   bool
	sampleRatio    float64
	sampleRows     uint64
	hasArrayJoin   bool
	arrayJoinLeft  bool
	arrayJoinCalls int
	arrayJoin      []any
	prewhere       []Expr
	settings       []kv
}

var _ Statement = (*SelectBuilder)(nil)

// Select starts a SELECT statement with the given select-list columns. A
// string is a column name; anything else that is not an Expr fails with
// ErrInvalidColumn at Build.
func Select(cols ...any) *SelectBuilder {
	return &SelectBuilder{cols: append([]any(nil), cols...)}
}

// From is shorthand for Select().From(table).
func From(table any) *SelectBuilder {
	return Select().From(table)
}

func (s *SelectBuilder) clone() *SelectBuilder {
	c := *s
	c.cols = append([]any(nil), s.cols...)
	c.where = append([]Expr(nil), s.where...)
	c.groupBy = append([]any(nil), s.groupBy...)
	c.having = append([]Expr(nil), s.having...)
	c.orderBy = append([]any(nil), s.orderBy...)
	c.joins = append([]join(nil), s.joins...)
	c.ctes = append([]cte(nil), s.ctes...)
	c.unions = append([]unionMember(nil), s.unions...)
	c.arrayJoin = append([]any(nil), s.arrayJoin...)
	c.prewhere = append([]Expr(nil), s.prewhere...)
	c.settings = append([]kv(nil), s.settings...)
	return &c
}

// From sets the FROM source: a string ("schema.table" allowed), a
// TableRef, or a Subquery.
func (s *SelectBuilder) From(table any) *SelectBuilder {
	c := s.clone()
	c.from = table
	c.fromSet = true
	return c
}

// Columns replaces the select list.
func (s *SelectBuilder) Columns(cols ...any) *SelectBuilder {
	c := s.clone()
	c.cols = append([]any(nil), cols...)
	return c
}

// AddColumns appends to the select list.
func (s *SelectBuilder) AddColumns(cols ...any) *SelectBuilder {
	c := s.clone()
	c.cols = append(c.cols, cols...)
	return c
}

// Distinct adds DISTINCT to the select list.
func (s *SelectBuilder) Distinct() *SelectBuilder {
	c := s.clone()
	c.distinct = true
	return c
}

// Where appends conds, ANDed with any existing conditions.
func (s *SelectBuilder) Where(conds ...Expr) *SelectBuilder {
	c := s.clone()
	c.where = append(c.where, conds...)
	return c
}

// GroupBy appends to the GROUP BY list.
func (s *SelectBuilder) GroupBy(cols ...any) *SelectBuilder {
	c := s.clone()
	c.groupBy = append(c.groupBy, cols...)
	return c
}

// Having appends conds, ANDed with any existing HAVING conditions.
func (s *SelectBuilder) Having(conds ...Expr) *SelectBuilder {
	c := s.clone()
	c.having = append(c.having, conds...)
	return c
}

// OrderBy appends to the ORDER BY list. An Order is used as-is; a string or
// Value is rendered ascending.
func (s *SelectBuilder) OrderBy(o ...any) *SelectBuilder {
	c := s.clone()
	c.orderBy = append(c.orderBy, o...)
	return c
}

// Limit sets the LIMIT clause. Limit(0) renders LIMIT 0.
func (s *SelectBuilder) Limit(n uint64) *SelectBuilder {
	c := s.clone()
	c.hasLimit = true
	c.limit = n
	return c
}

// Offset sets the OFFSET clause.
func (s *SelectBuilder) Offset(n uint64) *SelectBuilder {
	c := s.clone()
	c.hasOffset = true
	c.offset = n
	return c
}

// As turns the SELECT into a Subquery usable as a FROM source, aliased by
// alias (a single-part identifier).
func (s *SelectBuilder) As(alias string) Subquery {
	return Subquery{sb: s, alias: alias}
}

// Build renders the SELECT statement under dialect d using a fresh writer.
func (s *SelectBuilder) Build(d Dialect) (string, []any, error) {
	w := &writer{d: d}
	if d.Name() == "" {
		w.fail(ErrUnknownDialect)
		return w.finish()
	}
	s.renderSelect(w)
	return w.finish()
}

// renderSelect renders the full SELECT statement (WITH, core, UNION
// members, and the trailing ORDER BY/LIMIT/OFFSET/SETTINGS) into w. It is
// not named render so *SelectBuilder does not satisfy Expr.
//
// On ClickHouse, a compound (a builder with UNION members) that also has
// ORDER BY/LIMIT/OFFSET is wrapped as "SELECT * FROM (<with><core><unions>)
// <tail>", because ClickHouse binds a trailing ORDER BY/LIMIT to the last
// UNION member only. The CTE list stays inside the wrap parentheses.
func (s *SelectBuilder) renderSelect(w *writer) {
	ch := w.d.quote == quoteClickHouse
	wrap := ch && len(s.unions) > 0 && (len(s.orderBy) > 0 || s.hasLimit || s.hasOffset)

	if wrap {
		w.keyword("SELECT * FROM (")
	}
	s.renderWith(w)
	s.renderSelectCore(w)
	for _, u := range s.unions {
		if err := checkCompoundMember(u.query); err != nil {
			w.fail(err)
			return
		}
		renderUnionKeyword(w, u.kind, ch)
		u.query.renderSelectCore(w)
	}
	if wrap {
		w.keyword(")")
	}
	s.renderTail(w)
}

// renderSelectCore renders "SELECT ... [FROM ...] ... HAVING ..." — the
// part shared by a top-level SELECT and a UNION member (which may not have
// its own WITH, UNION, ORDER BY, LIMIT, OFFSET or SETTINGS).
func (s *SelectBuilder) renderSelectCore(w *writer) {
	w.keyword("SELECT ")
	if s.distinct {
		w.keyword("DISTINCT ")
	}
	if len(s.cols) == 0 {
		if !s.fromSet {
			w.fail(ErrNoTable)
			return
		}
		w.keyword("*")
	} else {
		for i, c := range s.cols {
			if i > 0 {
				w.keyword(", ")
			}
			renderColumn(w, c)
		}
	}

	if s.fromSet {
		w.keyword(" FROM ")
		renderSource(w, s.from)
		s.renderFinalSample(w)
	}

	s.renderArrayJoin(w)
	for _, j := range s.joins {
		renderJoin(w, j)
	}

	if len(s.prewhere) > 0 {
		if !w.d.Has(FeatureClickHouse) {
			w.fail(ErrUnsupported)
			return
		}
		w.keyword(" PREWHERE ")
		renderExpr(w, And(s.prewhere...))
	}

	if len(s.where) > 0 {
		w.keyword(" WHERE ")
		renderExpr(w, And(s.where...))
	}

	if len(s.groupBy) > 0 {
		w.keyword(" GROUP BY ")
		for i, c := range s.groupBy {
			if i > 0 {
				w.keyword(", ")
			}
			renderColumn(w, c)
		}
	}

	if len(s.having) > 0 {
		w.keyword(" HAVING ")
		renderExpr(w, And(s.having...))
	}
}

// renderTail renders the ORDER BY/LIMIT/OFFSET/SETTINGS clauses, which
// apply to the whole compound (see renderSelect).
func (s *SelectBuilder) renderTail(w *writer) {
	if len(s.orderBy) > 0 {
		w.keyword(" ORDER BY ")
		for i, o := range s.orderBy {
			if i > 0 {
				w.keyword(", ")
			}
			renderOrderItem(w, o)
		}
	}

	ch := w.d.quote == quoteClickHouse
	if s.hasLimit {
		if !ch && s.limit > math.MaxInt64 {
			w.fail(ErrInvalidLimit)
			return
		}
		w.keyword(" LIMIT " + strconv.FormatUint(s.limit, 10))
	} else if s.hasOffset && w.d.name == "sqlite" {
		w.keyword(" LIMIT -1")
	}
	if s.hasOffset {
		if !ch && s.offset > math.MaxInt64 {
			w.fail(ErrInvalidLimit)
			return
		}
		w.keyword(" OFFSET " + strconv.FormatUint(s.offset, 10))
	}
	s.renderSettings(w)
}

// renderOrderItem renders o: an Order is used as-is, a string or Value is
// rendered ascending, anything else fails with ErrInvalidColumn.
func renderOrderItem(w *writer, o any) {
	switch v := o.(type) {
	case Order:
		v.render(w)
	case string:
		Col(v).Asc().render(w)
	case Value:
		v.Asc().render(w)
	default:
		w.fail(ErrInvalidColumn)
	}
}
