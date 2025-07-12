package db

import (
	"github.com/oddbit-project/blueprint/db/field"
	"reflect"
	"sync"
)

type fieldSpec struct {
	fieldAlias   map[string]string // maps db fields to alias
	aliasField   map[string]string // maps alias to db fields
	sortFields   []string          // sortable db fields
	filterFields []string          // filterable db fields
	searchFields []string          // searchable db fields
}

var specCache = sync.Map{}

func getFieldSpec(from any) (*fieldSpec, error) {
	t := reflect.TypeOf(from)
	if t == nil {
		return nil, field.ErrInvalidStruct
	}
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	if cached, ok := specCache.Load(t.Name()); ok {
		return cached.(*fieldSpec), nil
	}
	v, err := newFieldSpecFromType(t)
	if err != nil {
		return nil, err
	}
	specCache.Store(t.Name(), v)

	return v, err
}

func newFieldSpecFromType(t reflect.Type) (*fieldSpec, error) {
	if t == nil {
		return nil, field.ErrInvalidStruct
	}

	structMeta, err := field.GetStructMeta(t)
	if err != nil {
		return nil, err
	}

	spec := &fieldSpec{
		fieldAlias:   make(map[string]string),
		aliasField:   make(map[string]string),
		sortFields:   make([]string, 0),
		filterFields: make([]string, 0),
		searchFields: make([]string, 0),
	}

	// Build the maps and lists from structMeta
	for _, meta := range structMeta {
		// Skip fields without db tags
		if meta.DbName == meta.Name && len(meta.DbOptions) == 0 {
			// No db tag present, skip this field
			continue
		}

		spec.fieldAlias[meta.DbName] = meta.Alias
		spec.aliasField[meta.Alias] = meta.DbName

		if meta.Sortable {
			spec.sortFields = append(spec.sortFields, meta.DbName)
		}
		if meta.Filterable {
			spec.filterFields = append(spec.filterFields, meta.DbName)
		}
		if meta.Searchable {
			spec.searchFields = append(spec.searchFields, meta.DbName)
		}
	}

	return spec, nil
}
