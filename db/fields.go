package db

import (
	"github.com/oddbit-project/blueprint/utils"
	"reflect"
	"slices"
	"strings"
)

const (
	ErrDuplicatedAlias  = utils.Error("alias already exists")
	ErrDuplicatedField  = utils.Error("field already exists")
	ErrInvalidStructPtr = utils.Error("field spec requires a pointer to a struct")
	ErrNilPointer       = utils.Error("ptr to struct to be parsed by field spec is nil")
	ErrInvalidStruct    = utils.Error("field spec requires a pointer to a struct; invalid type")

	// grid tag options
	tagGrid   = "grid"
	optSort   = "sort"
	optSearch = "search"
	optFilter = "filter"
)

type FieldSpec struct {
	fieldAlias   map[string]string // maps db fields to alias
	aliasField   map[string]string // maps alias to db fields
	sortFields   []string          // sortable db fields
	filterFields []string          // filterable db fields
	searchFields []string          // searchable db fields
}

var (
	// valid database tags
	validDbTags = []string{"db", "ch"}

	// valid alias tags
	validAliasTags = []string{"alias", "json"}
)

func NewFieldSpec(from any) (*FieldSpec, error) {
	spec := NewEmptyFieldSpec()
	return spec, spec.scanStruct(from)
}

func NewEmptyFieldSpec() *FieldSpec {
	return &FieldSpec{
		fieldAlias:   make(map[string]string),
		aliasField:   make(map[string]string),
		sortFields:   make([]string, 0),
		filterFields: make([]string, 0),
		searchFields: make([]string, 0),
	}
}

// AddField add a field to the field map; alias is the public name of the field
func (f *FieldSpec) AddField(dbField, alias string, searchable bool, sortable bool, filterable bool) error {
	// if no alias, use dbField
	if len(alias) == 0 {
		alias = dbField
	}

	if _, ok := f.fieldAlias[dbField]; ok {
		return ErrDuplicatedField
	}
	if _, ok := f.aliasField[alias]; ok {
		return ErrDuplicatedAlias
	}

	f.fieldAlias[dbField] = alias
	f.aliasField[alias] = dbField

	if searchable {
		f.searchFields = append(f.searchFields, dbField)
	}
	if sortable {
		f.sortFields = append(f.sortFields, dbField)
	}
	if filterable {
		f.filterFields = append(f.filterFields, dbField)
	}
	return nil
}

// LookupAlias lookup an alias and return the DbField
func (f *FieldSpec) LookupAlias(alias string) (string, bool) {
	if f, v := f.aliasField[alias]; v {
		return f, true
	}
	return "", false
}

// FieldAliasMap returns a copy of the fieldAlias map that maps db fields to alias
func (f *FieldSpec) FieldAliasMap() map[string]string {
	result := make(map[string]string)
	for k, v := range f.fieldAlias {
		result[k] = v
	}
	return result
}

// FilterFields return the list of filterable field alias
func (f *FieldSpec) FilterFields() []string {
	return f.filterFields
}

// SortFields return the list of filterable field alias
func (f *FieldSpec) SortFields() []string {
	return f.sortFields
}

// SearchFields return the list of filterable field alias
func (f *FieldSpec) SearchFields() []string {
	return f.searchFields
}

func (f *FieldSpec) scanStruct(s any) error {
	v := reflect.ValueOf(s)
	if v.Kind() != reflect.Ptr {
		return ErrInvalidStructPtr
	}
	if v.IsNil() {
		return ErrNilPointer
	}
	// unwrap element
	t := reflect.TypeOf(s)
	if v = reflect.Indirect(v); t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if v.Kind() != reflect.Struct {
		return ErrInvalidStruct
	}

	return f.scanStructFields(t)
}

func (f *FieldSpec) scanStructFields(t reflect.Type) error {
	// recursively scan struct fields
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		name := field.Name
		alias := field.Name
		sortable := false
		filterable := false
		searchable := false

		// attempt to find a valid db tag to replace the field name
		for _, tag := range validDbTags {
			if tv := field.Tag.Get(tag); len(tv) != 0 {
				tags := strings.Split(tv, ",")
				name = tags[0]
			}
		}

		// attempt to find a valid alias tag to use as alias
		for _, tag := range validAliasTags {
			if tv := field.Tag.Get(tag); len(tv) != 0 {
				tags := strings.Split(tv, ",")
				alias = tags[0]
			}
		}

		// get grid options
		if tv := field.Tag.Get(tagGrid); len(tv) != 0 {
			opts := strings.Split(tv, ",")
			sortable = slices.Index(opts, optSort) > -1
			searchable = slices.Index(opts, optSearch) > -1
			filterable = slices.Index(opts, optFilter) > -1
		}
		switch {
		case name == "-", len(field.PkgPath) != 0 && !field.Anonymous:
			continue
		}
		switch {
		case field.Anonymous:
			if field.Type.Kind() != reflect.Ptr {
				if err := f.scanStructFields(field.Type); err != nil {
					return err
				}
			}
		default:
			if err := f.AddField(name, alias, searchable, sortable, filterable); err != nil {
				return err
			}
		}
	}
	return nil
}
