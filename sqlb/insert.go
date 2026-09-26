package sqlb

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
)

// conflictAction is the ON CONFLICT clause chosen by ConflictBuilder.
type conflictAction int

const (
	conflictDoNothing conflictAction = iota
	conflictDoUpdate
	conflictDoUpdateExcluded
)

// conflictState holds an INSERT's ON CONFLICT clause.
type conflictState struct {
	cols         []string
	action       conflictAction
	doUpdateSet  map[string]any
	excludedCols []string
}

// InsertBuilder builds an INSERT statement. Builders are immutable: every
// method returns a new value and never modifies the receiver.
type InsertBuilder struct {
	table any

	cols []string

	hasValues bool
	valueRows [][]any

	hasRows  bool
	rowsCols []string
	rowsData [][]any
	rowsErr  error

	hasSetMap  bool
	setMapCols []string
	setMapVals []any
	setMapErr  error

	hasFromSelect bool
	fromSelect    *SelectBuilder

	conflict *conflictState

	returning []any
}

var _ Statement = (*InsertBuilder)(nil)

// Insert starts an INSERT statement against table: a string or a TableRef
// without an alias.
func Insert(table any) *InsertBuilder {
	return &InsertBuilder{table: table}
}

func (b *InsertBuilder) clone() *InsertBuilder {
	c := *b
	c.cols = append([]string(nil), b.cols...)
	c.valueRows = append([][]any(nil), b.valueRows...)
	c.rowsCols = append([]string(nil), b.rowsCols...)
	c.rowsData = append([][]any(nil), b.rowsData...)
	c.setMapCols = append([]string(nil), b.setMapCols...)
	c.setMapVals = append([]any(nil), b.setMapVals...)
	c.returning = append([]any(nil), b.returning...)
	return &c
}

// Columns replaces the column list used with Values or FromSelect.
func (b *InsertBuilder) Columns(cols ...string) *InsertBuilder {
	c := b.clone()
	c.cols = append([]string(nil), cols...)
	return c
}

// Values appends one row of values, in column position.
func (b *InsertBuilder) Values(vals ...any) *InsertBuilder {
	c := b.clone()
	c.hasValues = true
	c.valueRows = append(c.valueRows, append([]any(nil), vals...))
	return c
}

// Rows appends one row per record (a struct or pointer to struct). Values
// are extracted immediately (a snapshot): mutating a record afterwards does
// not change the statement. The first record decides the column set; a
// later record whose omit decision (OmitNil/OmitEmpty) differs for any
// field fails at Build with ErrInconsistentOmit.
func (b *InsertBuilder) Rows(records ...any) *InsertBuilder {
	c := b.clone()
	c.hasRows = true
	c.rowsCols, c.rowsData, c.rowsErr = recordRows(records)
	return c
}

// SetMap sets a single row from m, with columns in sorted key order.
func (b *InsertBuilder) SetMap(m map[string]any) *InsertBuilder {
	c := b.clone()
	c.hasSetMap = true
	c.setMapCols, c.setMapVals, c.setMapErr = setMapRow(m)
	return c
}

// FromSelect makes the INSERT an "INSERT INTO ... SELECT ...", using the
// column list set by Columns.
func (b *InsertBuilder) FromSelect(q *SelectBuilder) *InsertBuilder {
	c := b.clone()
	c.hasFromSelect = true
	c.fromSelect = q
	return c
}

// OnConflict starts an ON CONFLICT clause targeting cols (which may be
// empty for DoNothing).
func (b *InsertBuilder) OnConflict(cols ...string) ConflictBuilder {
	return ConflictBuilder{b: b, cols: append([]string(nil), cols...)}
}

// Returning sets the RETURNING column list.
func (b *InsertBuilder) Returning(cols ...any) *InsertBuilder {
	c := b.clone()
	c.returning = append([]any(nil), cols...)
	return c
}

// ConflictBuilder configures the action of an ON CONFLICT clause.
type ConflictBuilder struct {
	b    *InsertBuilder
	cols []string
}

// DoNothing renders "ON CONFLICT [(cols)] DO NOTHING".
func (c ConflictBuilder) DoNothing() *InsertBuilder {
	nb := c.b.clone()
	nb.conflict = &conflictState{cols: c.cols, action: conflictDoNothing}
	return nb
}

// DoUpdate renders "ON CONFLICT (cols) DO UPDATE SET ...", with set's keys
// sorted. A value is rendered in place if it is an Expr (e.g. Excluded or
// Raw), otherwise bound.
func (c ConflictBuilder) DoUpdate(set map[string]any) *InsertBuilder {
	nb := c.b.clone()
	m := make(map[string]any, len(set))
	for k, v := range set {
		m[k] = v
	}
	nb.conflict = &conflictState{cols: c.cols, action: conflictDoUpdate, doUpdateSet: m}
	return nb
}

// DoUpdateExcluded renders "ON CONFLICT (cols) DO UPDATE SET c = excluded.c"
// for each c in cols. Each c must be one of the INSERT's own columns, else
// Build fails with ErrUnknownField.
func (c ConflictBuilder) DoUpdateExcluded(cols ...string) *InsertBuilder {
	nb := c.b.clone()
	nb.conflict = &conflictState{cols: c.cols, action: conflictDoUpdateExcluded, excludedCols: append([]string(nil), cols...)}
	return nb
}

// Excluded renders "excluded."col"": the proposed-for-insertion value of
// col, for use inside DoUpdate. ErrUnsupported on dialects without
// FeatureUpsert.
func Excluded(col string) Value {
	return Value{fn: func(w *writer) {
		if !w.d.Has(FeatureUpsert) {
			w.fail(fmt.Errorf("%w: excluded", ErrUnsupported))
			return
		}
		if strings.Contains(col, ".") {
			w.fail(fmt.Errorf("%w: %q", ErrInvalidIdentifier, col))
			return
		}
		w.keyword("excluded.")
		w.ident(col)
	}}
}

// setMapRow turns m into a single row, with columns in sorted key order.
// An empty map fails with ErrNoColumns.
func setMapRow(m map[string]any) ([]string, []any, error) {
	if len(m) == 0 {
		return nil, nil, ErrNoColumns
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	vals := make([]any, len(keys))
	for i, k := range keys {
		vals[i] = m[k]
	}
	return keys, vals, nil
}

// recordRows resolves records into a shared column list (from the first
// record) and one value row per record. Records may be T or *T (compared
// after dereference); a later record of a different type fails with
// ErrRecordType, and a later record whose resolved column set differs from
// the first (any omitted field included, or vice versa) fails with
// ErrInconsistentOmit.
func recordRows(records []any) ([]string, [][]any, error) {
	if len(records) == 0 {
		return nil, nil, ErrNoColumns
	}
	var firstType reflect.Type
	var cols []string
	rows := make([][]any, 0, len(records))
	for i, rec := range records {
		v, err := recordStructValue(rec)
		if err != nil {
			return nil, nil, err
		}
		if i == 0 {
			firstType = v.Type()
		} else if v.Type() != firstType {
			return nil, nil, ErrRecordType
		}

		c, vals, err := recordValues(rec, true, recordOptions{})
		if err != nil {
			return nil, nil, err
		}
		if i == 0 {
			cols = c
		} else if !equalStrSlices(c, cols) {
			return nil, nil, ErrInconsistentOmit
		}
		rows = append(rows, vals)
	}
	return cols, rows, nil
}

func equalStrSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// renderWriteTable renders table as an INSERT/UPDATE target: a string, or a
// TableRef without an alias (a table alias makes no sense on a write
// target).
func renderWriteTable(w *writer, table any) {
	switch t := table.(type) {
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
		if t.alias != "" {
			w.fail(fmt.Errorf("%w: table alias in write statement", ErrUnsupported))
			return
		}
		w.ident(t.name)
	default:
		w.fail(ErrNoTable)
	}
}

// sqliteTrueCond renders the unquoted, unparenthesized literal "true".
var sqliteTrueCond = Value{fn: func(w *writer) { w.keyword("true") }}

// Build renders the INSERT statement under dialect d using a fresh writer.
func (b *InsertBuilder) Build(d Dialect) (string, []any, error) {
	w := &writer{d: d}
	if d.Name() == "" {
		w.fail(ErrUnknownDialect)
		return w.finish()
	}
	b.renderInsert(w)
	return w.finish()
}

// renderInsert renders the INSERT statement into w. It is not named render
// so *InsertBuilder does not satisfy Expr.
func (b *InsertBuilder) renderInsert(w *writer) {
	sourceCount := 0
	if b.hasValues {
		sourceCount++
	}
	if b.hasRows {
		sourceCount++
	}
	if b.hasSetMap {
		sourceCount++
	}
	if b.hasFromSelect {
		sourceCount++
	}
	if sourceCount > 1 {
		w.fail(ErrInsertMixed)
		return
	}

	var cols []string
	var rows [][]any
	switch {
	case b.hasRows:
		if b.rowsErr != nil {
			w.fail(b.rowsErr)
			return
		}
		cols, rows = b.rowsCols, b.rowsData
	case b.hasSetMap:
		if b.setMapErr != nil {
			w.fail(b.setMapErr)
			return
		}
		cols, rows = b.setMapCols, [][]any{b.setMapVals}
	case b.hasValues:
		if len(b.cols) == 0 {
			w.fail(ErrValueCount)
			return
		}
		for _, row := range b.valueRows {
			if len(row) != len(b.cols) {
				w.fail(ErrValueCount)
				return
			}
		}
		cols, rows = b.cols, b.valueRows
	case b.hasFromSelect:
		if len(b.cols) == 0 {
			w.fail(ErrNoColumns)
			return
		}
		cols = b.cols
	default:
		w.fail(ErrNoColumns)
		return
	}

	if !b.hasFromSelect && len(cols) == 0 {
		w.fail(ErrNoColumns)
		return
	}
	for _, c := range cols {
		if strings.Contains(c, ".") {
			w.fail(fmt.Errorf("%w: %q", ErrInvalidIdentifier, c))
			return
		}
	}

	w.keyword("INSERT INTO ")
	renderWriteTable(w, b.table)
	w.keyword(" (")
	for i, c := range cols {
		if i > 0 {
			w.keyword(", ")
		}
		w.ident(c)
	}
	w.keyword(")")

	if b.hasFromSelect {
		w.keyword(" ")
		q := b.fromSelect
		if q == nil {
			w.fail(ErrNoTable)
			return
		}
		if w.d.name == "sqlite" && b.conflict != nil && len(q.where) == 0 {
			q = q.Where(sqliteTrueCond)
		}
		q.renderSelect(w)
	} else {
		w.keyword(" VALUES ")
		for i, row := range rows {
			if i > 0 {
				w.keyword(", ")
			}
			w.keyword("(")
			for j, v := range row {
				if j > 0 {
					w.keyword(", ")
				}
				w.arg(v)
			}
			w.keyword(")")
		}
	}

	if b.conflict != nil {
		if !w.d.Has(FeatureUpsert) {
			w.fail(fmt.Errorf("%w: ON CONFLICT", ErrUnsupported))
			return
		}
		for _, c := range b.conflict.cols {
			if strings.Contains(c, ".") {
				w.fail(fmt.Errorf("%w: %q", ErrInvalidIdentifier, c))
				return
			}
		}
		w.keyword(" ON CONFLICT")
		if len(b.conflict.cols) > 0 {
			w.keyword(" (")
			for i, c := range b.conflict.cols {
				if i > 0 {
					w.keyword(", ")
				}
				w.ident(c)
			}
			w.keyword(")")
		}

		switch b.conflict.action {
		case conflictDoNothing:
			w.keyword(" DO NOTHING")
		case conflictDoUpdate:
			if len(b.conflict.cols) == 0 {
				w.fail(ErrConflictTarget)
				return
			}
			if len(b.conflict.doUpdateSet) == 0 {
				w.fail(ErrNoColumns)
				return
			}
			keys := make([]string, 0, len(b.conflict.doUpdateSet))
			for k := range b.conflict.doUpdateSet {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			w.keyword(" DO UPDATE SET ")
			for i, k := range keys {
				if i > 0 {
					w.keyword(", ")
				}
				if strings.Contains(k, ".") {
					w.fail(fmt.Errorf("%w: %q", ErrInvalidIdentifier, k))
					return
				}
				w.ident(k)
				w.keyword(" = ")
				renderAssignValue(w, b.conflict.doUpdateSet[k])
			}
		case conflictDoUpdateExcluded:
			if len(b.conflict.cols) == 0 {
				w.fail(ErrConflictTarget)
				return
			}
			if len(b.conflict.excludedCols) == 0 {
				w.fail(ErrNoColumns)
				return
			}
			w.keyword(" DO UPDATE SET ")
			for i, c := range b.conflict.excludedCols {
				if !containsStr(cols, c) {
					w.fail(fmt.Errorf("%w: %q", ErrUnknownField, c))
					return
				}
				if i > 0 {
					w.keyword(", ")
				}
				w.ident(c)
				w.keyword(" = excluded.")
				w.ident(c)
			}
		}
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

func containsStr(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
