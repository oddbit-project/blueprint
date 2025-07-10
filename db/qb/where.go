package qb

import (
	"fmt"
	"strings"
)

// WhereClause represents a complex WHERE clause with AND/OR support
type WhereClause interface {
	// Build generates the SQL string and collects values
	// Returns: SQL string, values array, next placeholder number
	Build(dialect interface {
		Field(string) string
		Placeholder(int) string
	}, startPlaceholder int) (sql string, values []any, nextPlaceholder int)
}

// SimpleCondition represents a single WHERE condition (replaces WhereCondition)
type SimpleCondition struct {
	Field    string
	Operator string
	Value    any
}

func (c SimpleCondition) Build(dialect interface {
	Field(string) string
	Placeholder(int) string
}, start int) (string, []any, int) {
	sql := fmt.Sprintf("%s %s %s",
		dialect.Field(c.Field),
		c.Operator,
		dialect.Placeholder(start))
	return sql, []any{c.Value}, start + 1
}

// AndCondition represents multiple conditions joined by AND
type AndCondition struct {
	Conditions []WhereClause
}

func (c AndCondition) Build(dialect interface {
	Field(string) string
	Placeholder(int) string
}, start int) (string, []any, int) {
	if len(c.Conditions) == 0 {
		return "1=1", nil, start
	}

	if len(c.Conditions) == 1 {
		return c.Conditions[0].Build(dialect, start)
	}

	var parts []string
	var values []any
	current := start

	for _, cond := range c.Conditions {
		sql, vals, next := cond.Build(dialect, current)
		parts = append(parts, sql)
		values = append(values, vals...)
		current = next
	}

	return "(" + strings.Join(parts, " AND ") + ")", values, current
}

// OrCondition represents multiple conditions joined by OR
type OrCondition struct {
	Conditions []WhereClause
}

func (c OrCondition) Build(dialect interface {
	Field(string) string
	Placeholder(int) string
}, start int) (string, []any, int) {
	if len(c.Conditions) == 0 {
		return "1=0", nil, start
	}

	if len(c.Conditions) == 1 {
		return c.Conditions[0].Build(dialect, start)
	}

	var parts []string
	var values []any
	current := start

	for _, cond := range c.Conditions {
		sql, vals, next := cond.Build(dialect, current)
		parts = append(parts, sql)
		values = append(values, vals...)
		current = next
	}

	return "(" + strings.Join(parts, " OR ") + ")", values, current
}

// LiteralCondition represents a raw SQL fragment with placeholders
type LiteralCondition struct {
	SQL    string
	Values []any
}

func (c LiteralCondition) Build(dialect interface {
	Field(string) string
	Placeholder(int) string
}, start int) (string, []any, int) {
	sql := c.SQL
	current := start

	// Replace ? placeholders with dialect-specific placeholders
	for i := 0; i < len(c.Values); i++ {
		sql = strings.Replace(sql, "?", dialect.Placeholder(current), 1)
		current++
	}

	return sql, c.Values, current
}

// RawCondition for completely raw SQL (no placeholder processing)
type RawCondition struct {
	SQL string
}

func (c RawCondition) Build(dialect interface {
	Field(string) string
	Placeholder(int) string
}, start int) (string, []any, int) {
	return c.SQL, nil, start
}

// InCondition for IN operator
type InCondition struct {
	Field  string
	Values []any
}

func (c InCondition) Build(dialect interface {
	Field(string) string
	Placeholder(int) string
}, start int) (string, []any, int) {
	if len(c.Values) == 0 {
		return "1=0", nil, start // IN () is always false
	}

	placeholders := make([]string, len(c.Values))
	for i := range c.Values {
		placeholders[i] = dialect.Placeholder(start + i)
	}

	sql := fmt.Sprintf("%s IN (%s)",
		dialect.Field(c.Field),
		strings.Join(placeholders, ", "))

	return sql, c.Values, start + len(c.Values)
}

// NullCondition for IS NULL / IS NOT NULL
type NullCondition struct {
	Field  string
	IsNull bool
}

func (c NullCondition) Build(dialect interface {
	Field(string) string
	Placeholder(int) string
}, start int) (string, []any, int) {
	op := "IS NULL"
	if !c.IsNull {
		op = "IS NOT NULL"
	}
	sql := fmt.Sprintf("%s %s", dialect.Field(c.Field), op)
	return sql, nil, start
}

// BetweenCondition for BETWEEN operator
type BetweenCondition struct {
	Field string
	Start any
	End   any
}

func (c BetweenCondition) Build(dialect interface {
	Field(string) string
	Placeholder(int) string
}, start int) (string, []any, int) {
	sql := fmt.Sprintf("%s BETWEEN %s AND %s",
		dialect.Field(c.Field),
		dialect.Placeholder(start),
		dialect.Placeholder(start+1))
	return sql, []any{c.Start, c.End}, start + 2
}

// ComparisonCondition for field-to-field comparisons
type ComparisonCondition struct {
	LeftField  string
	Operator   string
	RightField string
}

func (c ComparisonCondition) Build(dialect interface {
	Field(string) string
	Placeholder(int) string
}, start int) (string, []any, int) {
	sql := fmt.Sprintf("%s %s %s",
		dialect.Field(c.LeftField),
		c.Operator,
		dialect.Field(c.RightField))
	return sql, nil, start
}

// Helper functions for building WHERE clauses
func And(conditions ...WhereClause) WhereClause {
	return AndCondition{Conditions: conditions}
}

func Or(conditions ...WhereClause) WhereClause {
	return OrCondition{Conditions: conditions}
}

func Cond(field, operator string, value any) WhereClause {
	return SimpleCondition{Field: field, Operator: operator, Value: value}
}

func Literal(sql string, values ...any) WhereClause {
	return LiteralCondition{SQL: sql, Values: values}
}

func Raw(sql string) WhereClause {
	return RawCondition{SQL: sql}
}

func In(field string, values ...any) WhereClause {
	return InCondition{Field: field, Values: values}
}

func IsNull(field string) WhereClause {
	return NullCondition{Field: field, IsNull: true}
}

func IsNotNull(field string) WhereClause {
	return NullCondition{Field: field, IsNull: false}
}

func Between(field string, start, end any) WhereClause {
	return BetweenCondition{Field: field, Start: start, End: end}
}

func Compare(leftField, operator, rightField string) WhereClause {
	return ComparisonCondition{
		LeftField:  leftField,
		Operator:   operator,
		RightField: rightField,
	}
}

// Convenience functions for common patterns
func Eq(field string, value any) WhereClause {
	return Cond(field, "=", value)
}

func NotEq(field string, value any) WhereClause {
	return Cond(field, "!=", value)
}

func Gt(field string, value any) WhereClause {
	return Cond(field, ">", value)
}

func Gte(field string, value any) WhereClause {
	return Cond(field, ">=", value)
}

func Lt(field string, value any) WhereClause {
	return Cond(field, "<", value)
}

func Lte(field string, value any) WhereClause {
	return Cond(field, "<=", value)
}

func Like(field string, pattern string) WhereClause {
	return Cond(field, "LIKE", pattern)
}

func NotLike(field string, pattern string) WhereClause {
	return Cond(field, "NOT LIKE", pattern)
}
