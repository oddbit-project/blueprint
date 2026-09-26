package sqlb

import (
	"fmt"
	"regexp"
)

// colOrVal renders v as a column when it is a string, or as a value
// operand otherwise (an Expr is rendered, anything else is bound).
func colOrVal(v any) Expr {
	if s, ok := v.(string); ok {
		return Col(s)
	}
	return Val(v)
}

func aggregate(name string, col any) Value {
	return Value{fn: func(w *writer) {
		w.keyword(name + "(")
		renderExpr(w, colOrVal(col))
		w.keyword(")")
	}}
}

// CountAll renders COUNT(*).
func CountAll() Value {
	return Value{fn: func(w *writer) {
		w.keyword("COUNT(*)")
	}}
}

// Count renders COUNT(col). col is a column name if it is a string.
func Count(col any) Value { return aggregate("COUNT", col) }

// Sum renders SUM(col). col is a column name if it is a string.
func Sum(col any) Value { return aggregate("SUM", col) }

// Avg renders AVG(col). col is a column name if it is a string.
func Avg(col any) Value { return aggregate("AVG", col) }

// Min renders MIN(col). col is a column name if it is a string.
func Min(col any) Value { return aggregate("MIN", col) }

// Max renders MAX(col). col is a column name if it is a string.
func Max(col any) Value { return aggregate("MAX", col) }

var fnNameRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// Fn renders name(args...). name is trusted input — never build it from
// request data — and must match ^[A-Za-z_][A-Za-z0-9_]*$.
func Fn(name string, args ...any) Value {
	return Value{fn: func(w *writer) {
		if !fnNameRe.MatchString(name) {
			w.fail(fmt.Errorf("%w: %q", ErrInvalidFunction, name))
			return
		}
		w.keyword(name + "(")
		for i, a := range args {
			if i > 0 {
				w.keyword(", ")
			}
			renderOperand(w, a)
		}
		w.keyword(")")
	}}
}

var castTypeRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_ ,()\[\]]*$`)

func validCastType(typ string) bool {
	if !castTypeRe.MatchString(typ) {
		return false
	}
	depth := 0
	for _, c := range typ {
		switch c {
		case '(':
			depth++
		case ')':
			depth--
			if depth < 0 {
				return false
			}
		}
	}
	return depth == 0
}

// Cast renders CAST(v AS typ). typ is trusted input — never build it from
// request data.
func Cast(v any, typ string) Value {
	return Value{fn: func(w *writer) {
		if !validCastType(typ) {
			w.fail(fmt.Errorf("%w: %q", ErrInvalidType, typ))
			return
		}
		w.keyword("CAST(")
		renderOperand(w, v)
		w.keyword(" AS " + typ + ")")
	}}
}

type caseWhen struct {
	cond Expr
	then any
}

// CaseBuilder builds a CASE WHEN ... THEN ... [ELSE ...] END expression.
type CaseBuilder struct {
	whens []caseWhen
}

// Case starts a CASE expression.
func Case() CaseBuilder {
	return CaseBuilder{}
}

// When adds a WHEN cond THEN then clause.
func (c CaseBuilder) When(cond Expr, then any) CaseBuilder {
	whens := make([]caseWhen, len(c.whens), len(c.whens)+1)
	copy(whens, c.whens)
	whens = append(whens, caseWhen{cond: cond, then: then})
	return CaseBuilder{whens: whens}
}

func (c CaseBuilder) render(w *writer, elseVal any, hasElse bool) {
	if len(c.whens) == 0 {
		w.fail(ErrEmptyCase)
		return
	}
	w.keyword("CASE")
	for _, wc := range c.whens {
		w.keyword(" WHEN ")
		if wc.cond == nil {
			w.fail(ErrNilExpr)
			return
		}
		renderExpr(w, wc.cond)
		w.keyword(" THEN ")
		renderOperand(w, wc.then)
	}
	if hasElse {
		w.keyword(" ELSE ")
		renderOperand(w, elseVal)
	}
	w.keyword(" END")
}

// Else adds an ELSE v clause and finishes the CASE expression.
func (c CaseBuilder) Else(v any) Value {
	return Value{fn: func(w *writer) {
		c.render(w, v, true)
	}}
}

// End finishes the CASE expression without an ELSE clause. No When calls
// fails with ErrEmptyCase at render.
func (c CaseBuilder) End() Value {
	return Value{fn: func(w *writer) {
		c.render(w, nil, false)
	}}
}
