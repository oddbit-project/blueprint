package dbx

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/oddbit-project/gohan"
)

const (
	// MaxCursorBytes caps a pagination cursor's length, both the cursors
	// ListKeyset/QueryGridKeyset accept and the ones they mint.
	MaxCursorBytes = 4096
	// MaxKeysetKeys caps the number of keyset keys (the seek predicate grows
	// with the square of the key count).
	MaxKeysetKeys = 8
)

// KeysetKey is one key of a keyset (cursor) pagination order: a column of
// the record type and its direction.
type KeysetKey struct {
	Column string // T's db column
	Desc   bool
}

// KeyAsc returns an ascending KeysetKey on col.
func KeyAsc(col string) KeysetKey { return KeysetKey{Column: col} }

// KeyDesc returns a descending KeysetKey on col.
func KeyDesc(col string) KeysetKey { return KeysetKey{Column: col, Desc: true} }

// KeysetPage is one page of a keyset (cursor) paginated listing. Items is
// never nil. NextCursor fetches the next page and is "" when HasMore is
// false.
type KeysetPage[T any] struct {
	Items      []*T   `json:"items"`
	NextCursor string `json:"nextCursor"`
	HasMore    bool   `json:"hasMore"`
}

// keysetCol is one resolved keyset key.
type keysetCol struct {
	col  string
	desc bool
	typ  keyType
	path []int
}

// keysetPlan is a validated keyset key list, with the fingerprint its
// cursors carry.
type keysetPlan struct {
	keys []keysetCol
	fp   string
	// clickhouse is set on dialects with gohan.FeatureClickHouse, whose
	// unsigned columns hold values above math.MaxInt64.
	clickhouse bool
}

// keysetColumn resolves k against fields: the column must be mapped and have
// an eligible Go type.
func keysetColumn(fields map[string]keysetField, k KeysetKey) (keysetCol, error) {
	f, ok := fields[k.Column]
	if !ok {
		return keysetCol{}, fmt.Errorf("%w: %q", ErrUnknownColumn, k.Column)
	}
	kt, err := keyTypeOf(f.typ)
	if err != nil {
		return keysetCol{}, fmt.Errorf("%w: column %q: %v", ErrInvalidKeysetKey, k.Column, err)
	}
	return keysetCol{col: k.Column, desc: k.Desc, typ: kt, path: f.path}, nil
}

// newKeysetPlan validates keys against fields (1 to MaxKeysetKeys keys,
// mapped, not repeated, of an eligible type) and computes the cursor
// fingerprint for table.
func newKeysetPlan(fields map[string]keysetField, table string, keys []KeysetKey, clickhouse bool) (*keysetPlan, error) {
	if len(keys) == 0 || len(keys) > MaxKeysetKeys {
		return nil, fmt.Errorf("%w: %d keys, want 1 to %d", ErrInvalidKeysetKey, len(keys), MaxKeysetKeys)
	}
	cols := make([]keysetCol, len(keys))
	seen := make(map[string]bool, len(keys))
	for i, k := range keys {
		if seen[k.Column] {
			return nil, fmt.Errorf("%w: column %q: repeated", ErrInvalidKeysetKey, k.Column)
		}
		seen[k.Column] = true
		c, err := keysetColumn(fields, k)
		if err != nil {
			return nil, err
		}
		cols[i] = c
	}
	return &keysetPlan{keys: cols, fp: cursorFingerprint(table, cols), clickhouse: clickhouse}, nil
}

// keysetPlan builds the plan for keys on r's table, before any query.
func (r *Repository[T]) keysetPlan(keys []KeysetKey) (*keysetPlan, error) {
	fields, err := keysetFieldsFor(reflect.TypeFor[T]())
	if err != nil {
		return nil, err
	}
	return newKeysetPlan(fields, r.table, keys, r.q.Dialect().Has(gohan.FeatureClickHouse))
}

// values returns rec's canonical key values (rec is a T, not a pointer).
func (p *keysetPlan) values(rec reflect.Value) []any {
	out := make([]any, len(p.keys))
	for i, k := range p.keys {
		out[i] = keyValue(rec.FieldByIndex(k.path), k.typ)
	}
	return out
}

// columns returns the key columns, for error messages.
func (p *keysetPlan) columns() string {
	names := make([]string, len(p.keys))
	for i, k := range p.keys {
		names[i] = k.col
	}
	return strings.Join(names, ", ")
}

// seekOp is a comparison in a seek predicate.
type seekOp uint8

const (
	seekEq seekOp = iota
	seekGt
	seekLt
	seekGte
	seekLte
)

// seekCmp compares key number key with its cursor value.
type seekCmp struct {
	key int
	op  seekOp
}

// seekPred is the predicate selecting the rows after a cursor, for keys k1…kn
// with cursor values v1…vn and opI = ">" (ascending) or "<" (descending):
//
//	lead AND (k1 op1 v1 OR (k1 = v1 AND k2 op2 v2) OR … OR (k1 = v1 AND … AND kn opn vn))
//
// lead is the redundant bound "k1 >= v1" ("<=" when descending), present only
// when n > 1, so every dialect gets an index range on k1.
type seekPred struct {
	lead []seekCmp
	any  [][]seekCmp
}

// newSeekPred builds the seek predicate for keys with the given directions.
func newSeekPred(desc []bool) seekPred {
	var p seekPred
	if len(desc) > 1 {
		op := seekGte
		if desc[0] {
			op = seekLte
		}
		p.lead = []seekCmp{{key: 0, op: op}}
	}
	for i := range desc {
		and := make([]seekCmp, 0, i+1)
		for j := 0; j < i; j++ {
			and = append(and, seekCmp{key: j, op: seekEq})
		}
		op := seekGt
		if desc[i] {
			op = seekLt
		}
		p.any = append(p.any, append(and, seekCmp{key: i, op: op}))
	}
	return p
}

// cmpExpr renders c against cols and vals.
func cmpExpr(c seekCmp, cols []string, vals []any) gohan.Expr {
	col, v := gohan.Col(cols[c.key]), vals[c.key]
	switch c.op {
	case seekGt:
		return col.Gt(v)
	case seekLt:
		return col.Lt(v)
	case seekGte:
		return col.Gte(v)
	case seekLte:
		return col.Lte(v)
	default:
		return col.Eq(v)
	}
}

// expr renders p with gohan comparisons, And and Or only.
func (p seekPred) expr(cols []string, vals []any) gohan.Expr {
	ors := make([]gohan.Expr, len(p.any))
	for i, and := range p.any {
		exprs := make([]gohan.Expr, len(and))
		for j, c := range and {
			exprs[j] = cmpExpr(c, cols, vals)
		}
		ors[i] = gohan.And(exprs...)
	}
	if len(p.lead) == 0 {
		return gohan.Or(ors...)
	}
	return gohan.And(cmpExpr(p.lead[0], cols, vals), gohan.Or(ors...))
}

// seek returns the predicate selecting the rows after the cursor values vals.
func (p *keysetPlan) seek(vals []any) gohan.Expr {
	cols := make([]string, len(p.keys))
	desc := make([]bool, len(p.keys))
	for i, k := range p.keys {
		cols[i], desc[i] = k.col, k.desc
	}
	return newSeekPred(desc).expr(cols, vals)
}

// sameKey reports whether two canonical key tuples are equal.
func sameKey(a, b []any) bool {
	for i := range a {
		if ta, ok := a[i].(time.Time); ok {
			if !ta.Equal(b[i].(time.Time)) {
				return false
			}
			continue
		}
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// keysetPage runs sel (filters only) after the cursor values after (nil for
// the first page), ordered by the plan's keys, fetching one probe row past
// pageSize to learn whether another page follows.
func (r *Repository[T]) keysetPage(ctx context.Context, sel *gohan.SelectBuilder, p *keysetPlan, pageSize int, after []any) (*KeysetPage[T], error) {
	if after != nil {
		sel = sel.Where(p.seek(after))
	}
	for _, k := range p.keys {
		if k.desc {
			sel = sel.OrderBy(gohan.Col(k.col).Desc())
		} else {
			sel = sel.OrderBy(gohan.Col(k.col).Asc())
		}
	}
	rows, err := r.List(ctx, sel.Limit(uint64(pageSize)+1))
	if err != nil {
		return nil, err
	}

	var prev []any
	for _, rec := range rows {
		vals := p.values(reflect.ValueOf(rec).Elem())
		if prev != nil && sameKey(prev, vals) {
			return nil, fmt.Errorf("%w: (%s)", ErrKeysetNotUnique, p.columns())
		}
		prev = vals
	}

	page := &KeysetPage[T]{Items: rows}
	if len(rows) > pageSize {
		page.Items = rows[:pageSize]
		page.HasMore = true
		last := page.Items[pageSize-1]
		page.NextCursor, err = p.mint(p.values(reflect.ValueOf(last).Elem()))
		if err != nil {
			return nil, err
		}
	}
	return page, nil
}

// ListKeyset returns one page of the rows matching where (nil or
// gohan.IsEmpty for every row), ordered by keys, starting after cursor (""
// for the first page). keys must identify rows uniquely (end them with a
// unique column), their columns must be NOT NULL, and each field's Go type
// must be an integer (not uintptr), a string, a time.Time or a uuid.UUID
// (named types by their kind; no pointers and no other driver.Valuer or
// sql.Scanner types), else ErrInvalidKeysetKey. A pageSize below 1 means
// DefaultPageSize; above DefaultMaxLimit it is clamped to DefaultMaxLimit.
//
// Keys and cursor are checked before any query: an unknown column fails with
// ErrUnknownColumn, a malformed cursor, or one minted for another table, key
// list or key type, with ErrInvalidCursor. The cursor is not signed or
// encrypted: its key values are visible to clients, so never use a secret
// column as a key. Two fetched rows with the same key values fail with
// ErrKeysetNotUnique, and a next-page cursor longer than MaxCursorBytes with
// ErrCursorTooLarge.
func (r *Repository[T]) ListKeyset(ctx context.Context, where gohan.Expr, keys []KeysetKey, pageSize int, cursor string) (*KeysetPage[T], error) {
	p, err := r.keysetPlan(keys)
	if err != nil {
		return nil, err
	}
	if pageSize < 1 {
		pageSize = DefaultPageSize
	} else if pageSize > DefaultMaxLimit {
		pageSize = DefaultMaxLimit
	}
	after, err := p.decode(cursor)
	if err != nil {
		return nil, err
	}
	sel := r.sel
	if !gohan.IsEmpty(where) {
		sel = sel.Where(where)
	}
	return r.keysetPage(ctx, sel, p, pageSize, after)
}
