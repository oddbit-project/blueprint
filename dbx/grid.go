package dbx

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"slices"
	"sort"

	"github.com/oddbit-project/gohan"
)

const (
	SortAscending  = "asc"
	SortDescending = "desc"

	SearchNone  = 0
	SearchStart = 1
	SearchEnd   = 2
	SearchAny   = 3

	DefaultPageSize = 100

	// DefaultMaxLimit is the row cap a new Grid applies to GridQuery.Limit
	// (see Grid.WithMaxLimit).
	DefaultMaxLimit = 1000

	// MaxFilterValues caps the number of elements a []any filter value may
	// contain.
	MaxFilterValues = 1000
	// MaxSearchText caps SearchText's length in bytes.
	MaxSearchText = 256
)

var validSearchType = []uint{SearchNone, SearchStart, SearchEnd, SearchAny}
var validSortFields = []string{SortDescending, SortAscending}

// GridFilterFunc translates a GridQuery filter value to a db-compatible
// value. It is called both by ValidQuery and by Build, and must therefore
// be pure (no side effects, same input always yields the same output):
// Build re-runs it and uses its result rather than reusing ValidQuery's
// call.
type GridFilterFunc func(lookupValue any) (any, error)

// GridQuery is the client-supplied request a Grid turns into a query:
// filters, search text, sort and paging. Every field is attacker-controlled
// input. Limit == 0, or a Limit above the grid's cap (DefaultMaxLimit unless
// changed with Grid.WithMaxLimit), is treated as the cap. Sort takes the sort
// fields in precedence order; SortFields is the older map form, applied in
// alias order, and the two cannot be combined. JSON numbers decode as
// float64, so {"id": 3.9} matches id 3 on PostgreSQL (pgx truncates);
// register a filter func on integer id columns to reject non-integer
// input if that matters.
type GridQuery struct {
	SearchType   uint              `json:"searchType"`
	SearchText   string            `json:"searchText,omitempty"`
	FilterFields map[string]any    `json:"filterFields,omitempty"`
	SortFields   map[string]string `json:"sortFields,omitempty"`
	Sort         []SortField       `json:"sort,omitempty"`
	Offset       uint              `json:"offset,omitempty"`
	Limit        uint              `json:"limit,omitempty"`
}

// SortField is one entry of GridQuery.Sort: a field alias and its order
// (SortAscending or SortDescending; empty means SortDescending).
type SortField struct {
	Field string `json:"field"`
	Order string `json:"order,omitempty"`
}

// GridError is returned by ValidQuery/Build/NewGridQuery for an invalid
// request. Scope is "query", "filter", "sort" or "search"; Field is the
// offending alias, or empty when the error is not about one field.
type GridError struct {
	Scope   string `json:"scope"`
	Field   string `json:"field"`
	Message string `json:"message"`
}

// Error implements the error interface.
func (err GridError) Error() string {
	if err.Field != "" {
		return fmt.Sprintf("error on %s with field %s: %s", err.Scope, err.Field, err.Message)
	}
	return fmt.Sprintf("error on %s: %s", err.Scope, err.Message)
}

// NewGridQuery builds a GridQuery, rejecting an out-of-range searchType.
func NewGridQuery(searchType uint, limit uint, offset uint) (*GridQuery, error) {
	if !slices.Contains(validSearchType, searchType) {
		return nil, GridError{
			Scope:   "search",
			Field:   "",
			Message: "invalid search type",
		}
	}
	return &GridQuery{
		SearchType: searchType,
		Offset:     offset,
		Limit:      limit,
	}, nil
}

// Page sets Offset and Limit from a 1-based page number and page size. An
// offset that would overflow saturates at math.MaxInt.
func (g *GridQuery) Page(page, itemsPerPage int) {
	if page < 1 {
		page = 1
	}
	if itemsPerPage < 1 {
		itemsPerPage = DefaultPageSize
	}
	if page-1 > math.MaxInt/itemsPerPage {
		g.Offset = uint(math.MaxInt)
	} else {
		g.Offset = uint((page - 1) * itemsPerPage)
	}
	g.Limit = uint(itemsPerPage)
}

// Grid[T] builds gohan queries from a GridQuery against T's grid-flagged
// fields (grid:"sort"/"filter"/"search"). Only such fields are addressable
// by alias; every other field, tagged or not, answers "field is not valid".
type Grid[T any] struct {
	spec       *gridSpec
	filterFunc map[string]GridFilterFunc
	maxLimit   uint
	tiebreaker []string

	maxLimitErr   error
	tiebreakerErr error
}

// NewGrid builds a Grid[T]. It fails if any grid-flagged field of T has a
// duplicate, empty or "-" alias, or if a searchable field is neither
// string-kind nor a database/sql/driver.Valuer.
func NewGrid[T any]() (*Grid[T], error) {
	spec, err := getGridSpec(reflect.TypeFor[T]())
	if err != nil {
		return nil, err
	}
	return &Grid[T]{
		spec:       spec,
		filterFunc: make(map[string]GridFilterFunc),
		maxLimit:   DefaultMaxLimit,
	}, nil
}

// AddFilterFunc registers a filter function for dbField (T's db column
// name, not its alias).
func (g *Grid[T]) AddFilterFunc(dbField string, f GridFilterFunc) *Grid[T] {
	g.filterFunc[dbField] = f
	return g
}

// WithMaxLimit caps GridQuery.Limit: Limit == 0 or Limit > n is treated as
// Limit == n. A new Grid uses DefaultMaxLimit; n == 0 removes the cap, so
// Limit == 0 returns every row.
func (g *Grid[T]) WithMaxLimit(n uint) *Grid[T] {
	g.maxLimitErr = nil
	if uint64(n) > math.MaxInt64 {
		g.maxLimitErr = fmt.Errorf("dbx: max limit %d is above math.MaxInt64", n)
	}
	g.maxLimit = n
	return g
}

// WithTiebreaker appends cols (T's db column names, which need not be
// grid-flagged), ascending, to every query's ORDER BY after the client's sort
// fields, skipping any the client already sorts by. Use a unique key so that
// rows with equal sort values keep a stable order across LIMIT/OFFSET pages.
// A column T does not map makes ValidQuery and Build fail; each call replaces
// the previous one.
func (g *Grid[T]) WithTiebreaker(cols ...string) *Grid[T] {
	g.tiebreaker, g.tiebreakerErr = nil, nil
	known, err := gohan.RecordColumns(reflect.TypeFor[T]())
	if err != nil {
		g.tiebreakerErr = err
		return g
	}
	for _, c := range cols {
		if !slices.Contains(known, c) {
			g.tiebreakerErr = fmt.Errorf("dbx: tiebreaker column %q is not mapped by %s", c, reflect.TypeFor[T]())
			return g
		}
	}
	g.tiebreaker = cols
	return g
}

// configErr returns the first configuration error set by WithMaxLimit or
// WithTiebreaker, if any.
func (g *Grid[T]) configErr() error {
	if g.maxLimitErr != nil {
		return g.maxLimitErr
	}
	return g.tiebreakerErr
}

// sortedKeys[K] returns m's keys sorted, for deterministic iteration over
// client-supplied maps.
func sortedStringKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// validFilterScalar reports whether v is one of the JSON scalar types the
// filter value allowlist accepts: nil, bool, float64, string or
// json.Number.
func validFilterScalar(v any) bool {
	switch v.(type) {
	case nil:
		return true
	case bool, float64, string, json.Number:
		return true
	default:
		return false
	}
}

// validFilterValue reports whether v is an allowed filter value: a JSON
// scalar, or a flat (non-nested) []any of at most MaxFilterValues scalars.
// This is the grid's defence-in-depth against clickhouse-go's unsafe map
// formatting (gohan's recursive isUnsafeClickHouseValue check is the
// primary defence); it also rejects plain maps and nested lists outright,
// which gohan would otherwise accept for non-ClickHouse dialects.
func validFilterValue(v any) bool {
	if list, ok := v.([]any); ok {
		if len(list) > MaxFilterValues {
			return false
		}
		for _, e := range list {
			if !validFilterScalar(e) {
				return false
			}
		}
		return true
	}
	return validFilterScalar(v)
}

// gohanPkg is the import path of gohan's types, which a filter func must
// not return (gohan renders them as SQL rather than binding them).
var gohanPkg = reflect.TypeFor[gohan.Value]().PkgPath()

// filterScalar reports whether v binds as a single value: anything but a map,
// a gohan type, or a slice that is not a byte slice or a driver.Valuer
// (arrays, byte slices and Valuers such as uuid.UUID or pq.StringArray bind
// as one value, as gohan binds them).
func filterScalar(v any) bool {
	if v == nil {
		return true
	}
	t := reflect.TypeOf(v)
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.PkgPath() == gohanPkg {
		return false
	}
	if _, ok := v.(driver.Valuer); ok {
		return true
	}
	switch t.Kind() {
	case reflect.Map:
		return false
	case reflect.Slice:
		return t.Elem().Kind() == reflect.Uint8
	}
	return true
}

// filterFuncValue checks and normalises a filter func's result. Unlike
// client values it may be any value the driver binds (time.Time, int,
// []byte, uuid.UUID, ...), but not a gohan type (which gohan would render as
// SQL rather than bind) or a map; any other slice becomes a []any (an IN
// list) of at most MaxFilterValues such values.
func filterFuncValue(v any) (any, bool) {
	if filterScalar(v) {
		return v, true
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Slice || rv.Len() > MaxFilterValues {
		return nil, false
	}
	list := make([]any, rv.Len())
	for i := range list {
		e := rv.Index(i).Interface()
		if !filterScalar(e) {
			return nil, false
		}
		list[i] = e
	}
	return list, true
}

// filterValue returns the value to bind for the filter on alias/fname:
// v itself, or the checked result of fname's filter func.
func (g *Grid[T]) filterValue(alias, fname string, v any) (any, error) {
	fn, ok := g.filterFunc[fname]
	if !ok {
		return v, nil
	}
	nv, err := fn(v)
	if err != nil {
		return nil, err
	}
	nv, ok = filterFuncValue(nv)
	if !ok {
		return nil, GridError{Scope: "filter", Field: alias, Message: "value is not valid"}
	}
	return nv, nil
}

// sortOrder returns q's sort fields in the order they apply: Sort as given,
// or SortFields in alias order.
func sortOrder(q *GridQuery) []SortField {
	if len(q.Sort) > 0 {
		return q.Sort
	}
	out := make([]SortField, 0, len(q.SortFields))
	for _, alias := range sortedStringKeys(q.SortFields) {
		out = append(out, SortField{Field: alias, Order: q.SortFields[alias]})
	}
	return out
}

// ValidQuery validates q against g's spec: paging, then filters (sorted
// alias order), then sorts (in the order they apply), then search. Filters
// and sorts report "field is not valid" for an unaddressable alias, then
// "field is not filterable"/"not sortable"; filter values are checked
// against the allowlist before any filter func runs, and a filter func's
// result is checked too. A configuration error (WithMaxLimit,
// WithTiebreaker) is returned as is, not as a GridError.
func (g *Grid[T]) ValidQuery(q *GridQuery) error {
	if err := g.configErr(); err != nil {
		return err
	}
	if q == nil {
		return GridError{Scope: "query", Message: "query is required"}
	}
	if uint64(q.Offset) > math.MaxInt64 {
		return GridError{Scope: "query", Message: "offset is out of range"}
	}
	if uint64(q.Limit) > math.MaxInt64 {
		return GridError{Scope: "query", Message: "limit is out of range"}
	}

	if q.FilterFields != nil {
		for _, alias := range sortedStringKeys(q.FilterFields) {
			fname, ok := g.spec.aliasField[alias]
			if !ok {
				return GridError{Scope: "filter", Field: alias, Message: "field is not valid"}
			}
			if !slices.Contains(g.spec.filterFields, fname) {
				return GridError{Scope: "filter", Field: alias, Message: "field is not filterable"}
			}
			if !validFilterValue(q.FilterFields[alias]) {
				return GridError{Scope: "filter", Field: alias, Message: "value is not valid"}
			}
			if _, err := g.filterValue(alias, fname, q.FilterFields[alias]); err != nil {
				return err
			}
		}
	}

	if len(q.Sort) > 0 && len(q.SortFields) > 0 {
		return GridError{Scope: "sort", Message: "use either sort or sortFields, not both"}
	}
	seen := make(map[string]bool, len(q.Sort))
	for _, sf := range sortOrder(q) {
		alias := sf.Field
		fname, ok := g.spec.aliasField[alias]
		if !ok {
			return GridError{Scope: "sort", Field: alias, Message: "field is not valid"}
		}
		if !slices.Contains(g.spec.sortFields, fname) {
			return GridError{Scope: "sort", Field: alias, Message: "field is not sortable"}
		}
		if len(sf.Order) > 0 && !slices.Contains(validSortFields, sf.Order) {
			return GridError{Scope: "sort", Field: alias, Message: "sort order is not valid"}
		}
		if seen[alias] {
			return GridError{Scope: "sort", Field: alias, Message: "field is repeated"}
		}
		seen[alias] = true
	}

	if len(q.SearchText) > MaxSearchText {
		return GridError{Scope: "search", Message: "search text too long"}
	}
	if len(q.SearchText) > 0 && q.SearchType == SearchNone {
		return GridError{Scope: "search", Message: "search not allowed"}
	}
	if q.SearchType > SearchAny {
		return GridError{Scope: "search", Message: "invalid search type"}
	}
	if len(q.SearchText) > 0 && len(g.spec.searchFields) == 0 {
		return GridError{Scope: "search", Message: "no searchable fields"}
	}

	return nil
}

// Build validates q, then applies it to base: filters (sorted alias order,
// ANDed with base's own conditions), search (one HasPrefix/HasSuffix/
// Contains per searchable field, ORed), sort (Sort order, or SortFields in
// alias order; default direction desc), then WithTiebreaker's columns, then
// paging (after the WithMaxLimit cap).
func (g *Grid[T]) Build(base *gohan.SelectBuilder, q *GridQuery) (*gohan.SelectBuilder, error) {
	if q == nil {
		return nil, GridError{Scope: "query", Message: "query is required"}
	}
	if base == nil {
		return nil, GridError{Scope: "query", Message: "base query is required"}
	}
	if err := g.configErr(); err != nil {
		return nil, err
	}
	if base.IsCompound() {
		return nil, GridError{Scope: "query", Message: "base query must not be a UNION; wrap it with gohan.From(q.As(...))"}
	}
	if err := g.ValidQuery(q); err != nil {
		return nil, err
	}

	qry := base

	if len(q.FilterFields) > 0 {
		for _, alias := range sortedStringKeys(q.FilterFields) {
			fname := g.spec.aliasField[alias]
			v, err := g.filterValue(alias, fname, q.FilterFields[alias])
			if err != nil {
				return nil, err
			}
			if list, ok := v.([]any); ok {
				qry = qry.Where(gohan.Col(fname).In(list...))
			} else {
				qry = qry.Where(gohan.Col(fname).Eq(v))
			}
		}
	}

	if len(q.SearchText) > 0 && len(g.spec.searchFields) > 0 {
		exprs := make([]gohan.Expr, len(g.spec.searchFields))
		for i, fname := range g.spec.searchFields {
			col := gohan.Col(fname)
			switch q.SearchType {
			case SearchStart:
				exprs[i] = col.HasPrefix(q.SearchText)
			case SearchEnd:
				exprs[i] = col.HasSuffix(q.SearchText)
			default: // SearchAny (ValidQuery already rejected anything else with search text set)
				exprs[i] = col.Contains(q.SearchText)
			}
		}
		qry = qry.Where(gohan.Or(exprs...))
	}

	sorted := make(map[string]bool)
	for _, sf := range sortOrder(q) {
		fname := g.spec.aliasField[sf.Field]
		sorted[fname] = true
		if sf.Order == SortAscending {
			qry = qry.OrderBy(gohan.Col(fname).Asc())
		} else {
			qry = qry.OrderBy(gohan.Col(fname).Desc())
		}
	}
	for _, col := range g.tiebreaker {
		if !sorted[col] {
			sorted[col] = true
			qry = qry.OrderBy(gohan.Col(col).Asc())
		}
	}

	limit := q.Limit
	if g.maxLimit > 0 && (limit == 0 || limit > g.maxLimit) {
		limit = g.maxLimit
	}
	if limit > 0 {
		qry = qry.Limit(uint64(limit)).Offset(uint64(q.Offset))
	} else if q.Offset > 0 {
		qry = qry.Offset(uint64(q.Offset))
	}

	return qry, nil
}
