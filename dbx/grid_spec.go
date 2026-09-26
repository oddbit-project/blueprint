package dbx

import (
	"database/sql/driver"
	"fmt"
	"reflect"
	"sync"

	"github.com/oddbit-project/blueprint/db/field"
)

// gridSpec is the compiled, per-type shape a Grid[T] needs: which aliases
// are addressable, and which underlying db columns are sortable,
// filterable and searchable. It is built once per reflect.Type (unlike
// db.Grid's spec, which is keyed by t.Name() and can therefore be shared by
// two differently-shaped types with the same name) and cached.
type gridSpec struct {
	aliasField   map[string]string // alias -> db column, addressable fields only
	sortFields   []string          // sortable db columns
	filterFields []string          // filterable db columns
	searchFields []string          // searchable db columns, in struct field order
}

var gridSpecCache sync.Map // map[reflect.Type]*gridSpec

var valuerType = reflect.TypeOf((*driver.Valuer)(nil)).Elem()

// getGridSpec returns the cached gridSpec for t, building it on first use.
func getGridSpec(t reflect.Type) (*gridSpec, error) {
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if cached, ok := gridSpecCache.Load(t); ok {
		return cached.(*gridSpec), nil
	}
	spec, err := newGridSpec(t)
	if err != nil {
		return nil, err
	}
	gridSpecCache.Store(t, spec)
	return spec, nil
}

// isSearchableType reports whether t (dereferenced through pointers) is
// string-kind or implements driver.Valuer (either as t or *t). LIKE against
// anything else fails on PostgreSQL and ClickHouse.
func isSearchableType(t reflect.Type) bool {
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() == reflect.String {
		return true
	}
	if t.Implements(valuerType) {
		return true
	}
	if reflect.PointerTo(t).Implements(valuerType) {
		return true
	}
	return false
}

// newGridSpec builds a gridSpec from t's field metadata. Only fields
// carrying at least one of grid:"sort"/"filter"/"search" are addressable;
// everything else (including untagged fields) is invisible to the grid.
// Among addressable fields, an empty or "-" alias, or a duplicate alias,
// fails NewGrid outright rather than silently overwriting (db.Grid's
// last-wins behaviour).
func newGridSpec(t reflect.Type) (*gridSpec, error) {
	metas, err := field.GetStructMeta(t)
	if err != nil {
		return nil, err
	}

	spec := &gridSpec{
		aliasField:   make(map[string]string),
		sortFields:   make([]string, 0),
		filterFields: make([]string, 0),
		searchFields: make([]string, 0),
	}

	for _, meta := range metas {
		if !meta.Sortable && !meta.Filterable && !meta.Searchable {
			continue
		}
		if meta.Alias == "" || meta.Alias == "-" {
			return nil, fmt.Errorf("dbx: grid field %q has an empty or reserved alias", meta.Name)
		}
		if _, dup := spec.aliasField[meta.Alias]; dup {
			return nil, fmt.Errorf("dbx: duplicate grid alias %q", meta.Alias)
		}
		spec.aliasField[meta.Alias] = meta.DbName

		if meta.Sortable {
			spec.sortFields = append(spec.sortFields, meta.DbName)
		}
		if meta.Filterable {
			spec.filterFields = append(spec.filterFields, meta.DbName)
		}
		if meta.Searchable {
			if !isSearchableType(meta.Type) {
				return nil, fmt.Errorf("dbx: grid field %q is searchable but is neither string-kind nor a driver.Valuer", meta.Name)
			}
			spec.searchFields = append(spec.searchFields, meta.DbName)
		}
	}

	return spec, nil
}
