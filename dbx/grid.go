package dbx

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"sort"

	"github.com/oddbit-project/blueprint/sqlb"
)

const (
	SortAscending  = "asc"
	SortDescending = "desc"

	SearchNone  = 0
	SearchStart = 1
	SearchEnd   = 2
	SearchAny   = 3

	DefaultPageSize = 100

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
// input. Limit == 0 returns every row, unless Grid.WithMaxLimit is set, in
// which case it is treated like Limit == n. JSON numbers decode as
// float64, so {"id": 3.9} matches id 3 on PostgreSQL (pgx truncates);
// register a filter func on integer id columns to reject non-integer
// input if that matters.
type GridQuery struct {
	SearchType   uint              `json:"searchType"`
	SearchText   string            `json:"searchText,omitempty"`
	FilterFields map[string]any    `json:"filterFields,omitempty"`
	SortFields   map[string]string `json:"sortFields,omitempty"`
	Offset       uint              `json:"offset,omitempty"`
	Limit        uint              `json:"limit,omitempty"`
}

// GridError is returned by ValidQuery/Build/NewGridQuery for an invalid
// request. Scope is "query", "filter", "sort" or "search"; Field is the
// offending alias (empty for a query- or search-scoped error).
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

// Page sets Offset and Limit from a 1-based page number and page size.
func (g *GridQuery) Page(page, itemsPerPage int) {
	if page < 1 {
		page = 1
	}
	if itemsPerPage < 1 {
		itemsPerPage = DefaultPageSize
	}
	g.Offset = uint((page - 1) * itemsPerPage)
	g.Limit = uint(itemsPerPage)
}

// Grid[T] builds sqlb queries from a GridQuery against T's grid-flagged
// fields (grid:"sort"/"filter"/"search"). Only such fields are addressable
// by alias; every other field, tagged or not, answers "field is not valid".
type Grid[T any] struct {
	spec       *gridSpec
	filterFunc map[string]GridFilterFunc
	maxLimit   uint
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
	}, nil
}

// AddFilterFunc registers a filter function for dbField (T's db column
// name, not its alias).
func (g *Grid[T]) AddFilterFunc(dbField string, f GridFilterFunc) *Grid[T] {
	g.filterFunc[dbField] = f
	return g
}

// WithMaxLimit caps GridQuery.Limit: n == 0 (the default) applies no cap;
// otherwise Limit == 0 or Limit > n is treated as Limit == n.
func (g *Grid[T]) WithMaxLimit(n uint) *Grid[T] {
	g.maxLimit = n
	return g
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
// formatting (sqlb's recursive isUnsafeClickHouseValue check is the
// primary defence); it also rejects plain maps and nested lists outright,
// which sqlb would otherwise accept for non-ClickHouse dialects.
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

// ValidQuery validates q against g's spec: filters (sorted alias order),
// then sorts (sorted alias order), then search. Filters and sorts report
// "field is not valid" for an unaddressable alias, then "field is not
// filterable"/"not sortable"; filter values are checked against the
// allowlist before any filter func runs.
func (g *Grid[T]) ValidQuery(q *GridQuery) error {
	if q == nil {
		return GridError{Scope: "query", Message: "query is required"}
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
			if fn, ok := g.filterFunc[fname]; ok {
				if _, err := fn(q.FilterFields[alias]); err != nil {
					return err
				}
			}
		}
	}

	if q.SortFields != nil {
		for _, alias := range sortedStringKeys(q.SortFields) {
			fname, ok := g.spec.aliasField[alias]
			if !ok {
				return GridError{Scope: "sort", Field: alias, Message: "field is not valid"}
			}
			if !slices.Contains(g.spec.sortFields, fname) {
				return GridError{Scope: "sort", Field: alias, Message: "field is not sortable"}
			}
			if dir := q.SortFields[alias]; len(dir) > 0 {
				if !slices.Contains(validSortFields, dir) {
					return GridError{Scope: "sort", Field: alias, Message: "sort order is not valid"}
				}
			}
		}
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

	return nil
}

// Build validates q, then applies it to base: filters (sorted alias order,
// ANDed with base's own conditions), search (one HasPrefix/HasSuffix/
// Contains per searchable field, ORed — no searchable fields means no
// search condition at all), sort (sorted alias order, default direction
// desc), then paging (after WithMaxLimit's cap).
func (g *Grid[T]) Build(base *sqlb.SelectBuilder, q *GridQuery) (*sqlb.SelectBuilder, error) {
	if q == nil {
		return nil, GridError{Scope: "query", Message: "query is required"}
	}
	if base == nil {
		return nil, GridError{Scope: "query", Message: "base query is required"}
	}
	if base.IsCompound() {
		return nil, GridError{Scope: "query", Message: "base query must not be a UNION; wrap it with sqlb.From(q.As(...))"}
	}
	if err := g.ValidQuery(q); err != nil {
		return nil, err
	}

	qry := base

	if len(q.FilterFields) > 0 {
		for _, alias := range sortedStringKeys(q.FilterFields) {
			fname := g.spec.aliasField[alias]
			v := q.FilterFields[alias]
			if fn, ok := g.filterFunc[fname]; ok {
				nv, err := fn(v)
				if err != nil {
					return nil, err
				}
				v = nv
			}
			if list, ok := v.([]any); ok {
				qry = qry.Where(sqlb.Col(fname).In(list...))
			} else {
				qry = qry.Where(sqlb.Col(fname).Eq(v))
			}
		}
	}

	if len(q.SearchText) > 0 && len(g.spec.searchFields) > 0 {
		exprs := make([]sqlb.Expr, len(g.spec.searchFields))
		for i, fname := range g.spec.searchFields {
			col := sqlb.Col(fname)
			switch q.SearchType {
			case SearchStart:
				exprs[i] = col.HasPrefix(q.SearchText)
			case SearchEnd:
				exprs[i] = col.HasSuffix(q.SearchText)
			default: // SearchAny (ValidQuery already rejected anything else with search text set)
				exprs[i] = col.Contains(q.SearchText)
			}
		}
		qry = qry.Where(sqlb.Or(exprs...))
	}

	if len(q.SortFields) > 0 {
		for _, alias := range sortedStringKeys(q.SortFields) {
			fname := g.spec.aliasField[alias]
			dir := q.SortFields[alias]
			if len(dir) == 0 {
				dir = SortDescending
			}
			var order sqlb.Order
			if dir == SortAscending {
				order = sqlb.Col(fname).Asc()
			} else {
				order = sqlb.Col(fname).Desc()
			}
			qry = qry.OrderBy(order)
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
