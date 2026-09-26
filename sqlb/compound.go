package sqlb

import (
	"fmt"
	"strings"
)

// cte is a single WITH entry.
type cte struct {
	name  string
	query *SelectBuilder
}

// unionKind selects the UNION keyword rendered for a unionMember.
type unionKind int

const (
	unionAll unionKind = iota
	unionDistinct
)

// unionMember is a single UNION operand.
type unionMember struct {
	kind  unionKind
	query *SelectBuilder
}

// addCTE appends a WITH entry, copying it first (immutability).
func (s *SelectBuilder) addCTE(name string, q *SelectBuilder, recursive bool) *SelectBuilder {
	c := s.clone()
	c.ctes = append(c.ctes, cte{name: name, query: q})
	if recursive {
		c.recursive = true
	}
	return c
}

// With adds a WITH entry: "name AS (q)". name must be a single-part
// identifier.
func (s *SelectBuilder) With(name string, q *SelectBuilder) *SelectBuilder {
	return s.addCTE(name, q, false)
}

// WithRecursive adds a WITH RECURSIVE entry: "name AS (q)". name must be a
// single-part identifier. If any CTE on the builder was added with
// WithRecursive, the whole WITH clause renders RECURSIVE.
func (s *SelectBuilder) WithRecursive(name string, q *SelectBuilder) *SelectBuilder {
	return s.addCTE(name, q, true)
}

// addUnion appends a UNION member, copying it first (immutability).
func (s *SelectBuilder) addUnion(kind unionKind, q *SelectBuilder) *SelectBuilder {
	c := s.clone()
	c.unions = append(c.unions, unionMember{kind: kind, query: q})
	return c
}

// Union adds a UNION member: UNION DISTINCT on ClickHouse, UNION
// elsewhere. q may not have its own ORDER BY, LIMIT, OFFSET, WITH,
// SETTINGS or UNION — such a member fails with ErrCompoundPart at Build.
func (s *SelectBuilder) Union(q *SelectBuilder) *SelectBuilder {
	return s.addUnion(unionDistinct, q)
}

// UnionAll adds a UNION ALL member. q may not have its own ORDER BY,
// LIMIT, OFFSET, WITH, SETTINGS or UNION — such a member fails with
// ErrCompoundPart at Build.
func (s *SelectBuilder) UnionAll(q *SelectBuilder) *SelectBuilder {
	return s.addUnion(unionAll, q)
}

// checkCompoundMember reports ErrCompoundPart if q cannot be used as a
// UNION member, or ErrNilExpr if q is nil.
func checkCompoundMember(q *SelectBuilder) error {
	if q == nil {
		return ErrNilExpr
	}
	if len(q.ctes) > 0 || len(q.unions) > 0 || len(q.orderBy) > 0 ||
		q.hasLimit || q.hasOffset || len(q.settings) > 0 {
		return ErrCompoundPart
	}
	return nil
}

// renderUnionKeyword renders the keyword for a union member of kind on ch
// (ClickHouse) or another dialect.
func renderUnionKeyword(w *writer, kind unionKind, ch bool) {
	switch kind {
	case unionAll:
		w.keyword(" UNION ALL ")
	case unionDistinct:
		if ch {
			w.keyword(" UNION DISTINCT ")
		} else {
			w.keyword(" UNION ")
		}
	}
}

// renderWith renders the WITH clause, if any: "WITH [RECURSIVE ]n AS
// (<select>)[, m AS (<select>)] ".
func (s *SelectBuilder) renderWith(w *writer) {
	if len(s.ctes) == 0 {
		return
	}
	w.keyword("WITH ")
	if s.recursive {
		w.keyword("RECURSIVE ")
	}
	for i, c := range s.ctes {
		if i > 0 {
			w.keyword(", ")
		}
		if c.query == nil {
			w.fail(ErrNilExpr)
			return
		}
		if c.name == "" || strings.Contains(c.name, ".") {
			w.fail(fmt.Errorf("%w: %q", ErrInvalidIdentifier, c.name))
			return
		}
		w.ident(c.name)
		w.keyword(" AS (")
		c.query.renderSelect(w)
		w.keyword(")")
	}
	w.keyword(" ")
}
