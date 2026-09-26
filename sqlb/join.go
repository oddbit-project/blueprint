package sqlb

// join is a single JOIN clause: t: string, TableRef or Subquery (as From).
// Either on (an ON condition) or using (a USING column list) is set, never
// both; a CROSS JOIN has neither.
type join struct {
	kind     string
	source   any
	on       Expr
	using    []string
	hasUsing bool
}

// addJoin appends j to the join list, copying it first (immutability).
func (s *SelectBuilder) addJoin(j join) *SelectBuilder {
	c := s.clone()
	c.joins = append(c.joins, j)
	return c
}

// Join adds an INNER JOIN against t, matched by on.
func (s *SelectBuilder) Join(t any, on Expr) *SelectBuilder {
	return s.addJoin(join{kind: "INNER JOIN", source: t, on: on})
}

// LeftJoin adds a LEFT JOIN against t, matched by on.
func (s *SelectBuilder) LeftJoin(t any, on Expr) *SelectBuilder {
	return s.addJoin(join{kind: "LEFT JOIN", source: t, on: on})
}

// RightJoin adds a RIGHT JOIN against t, matched by on.
func (s *SelectBuilder) RightJoin(t any, on Expr) *SelectBuilder {
	return s.addJoin(join{kind: "RIGHT JOIN", source: t, on: on})
}

// FullJoin adds a FULL JOIN against t, matched by on.
func (s *SelectBuilder) FullJoin(t any, on Expr) *SelectBuilder {
	return s.addJoin(join{kind: "FULL JOIN", source: t, on: on})
}

// CrossJoin adds a CROSS JOIN against t.
func (s *SelectBuilder) CrossJoin(t any) *SelectBuilder {
	return s.addJoin(join{kind: "CROSS JOIN", source: t})
}

// JoinUsing adds an INNER JOIN against t, matched by the named columns.
// Zero columns fails with ErrEmptyClause at Build.
func (s *SelectBuilder) JoinUsing(t any, cols ...string) *SelectBuilder {
	return s.addJoin(join{kind: "INNER JOIN", source: t, hasUsing: true, using: append([]string(nil), cols...)})
}

// LeftJoinUsing adds a LEFT JOIN against t, matched by the named columns.
// Zero columns fails with ErrEmptyClause at Build.
func (s *SelectBuilder) LeftJoinUsing(t any, cols ...string) *SelectBuilder {
	return s.addJoin(join{kind: "LEFT JOIN", source: t, hasUsing: true, using: append([]string(nil), cols...)})
}

// renderJoin renders j: "<KIND> <source>[ ON <on> | USING (<cols>)]".
func renderJoin(w *writer, j join) {
	w.keyword(" " + j.kind + " ")
	renderSource(w, j.source)
	if j.kind == "CROSS JOIN" {
		return
	}
	if j.hasUsing {
		if len(j.using) == 0 {
			w.fail(ErrEmptyClause)
			return
		}
		w.keyword(" USING (")
		for i, c := range j.using {
			if i > 0 {
				w.keyword(", ")
			}
			writeAlias(w, c)
		}
		w.keyword(")")
		return
	}
	if j.on == nil {
		w.fail(ErrNilExpr)
		return
	}
	w.keyword(" ON ")
	renderExpr(w, j.on)
}
