package sqlb

import (
	"fmt"
	"reflect"

	"github.com/oddbit-project/blueprint/db/field"
)

// recordOptions configures how recordValues maps a record's fields to
// columns and values. The zero value is the insert-time behaviour: no
// include/exclude filter, Auto fields skipped, zero values kept.
type recordOptions struct {
	include  []string
	exclude  []string
	skipZero bool
	withAuto bool
}

// RecordOption configures UpdateBuilder.SetRecord.
type RecordOption func(*recordOptions)

// IncludeFields restricts SetRecord to only the named fields (struct field
// name or db name). When set, ExcludeFields is ignored.
func IncludeFields(names ...string) RecordOption {
	return func(o *recordOptions) { o.include = append(o.include, names...) }
}

// ExcludeFields removes the named fields (struct field name or db name)
// from SetRecord.
func ExcludeFields(names ...string) RecordOption {
	return func(o *recordOptions) { o.exclude = append(o.exclude, names...) }
}

// SkipZeroValues skips fields holding their type's zero value.
func SkipZeroValues() RecordOption {
	return func(o *recordOptions) { o.skipZero = true }
}

// WithAutoFields includes fields marked Auto, which are otherwise always
// skipped by SetRecord.
func WithAutoFields() RecordOption {
	return func(o *recordOptions) { o.withAuto = true }
}

// recordStructValue dereferences rec (a struct or pointer to struct) to its
// struct Value, failing with ErrInvalidRecord for nil, a nil pointer, or a
// non-struct.
func recordStructValue(rec any) (reflect.Value, error) {
	if rec == nil {
		return reflect.Value{}, ErrInvalidRecord
	}
	v := reflect.ValueOf(rec)
	for v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return reflect.Value{}, ErrInvalidRecord
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return reflect.Value{}, ErrInvalidRecord
	}
	return v, nil
}

// fieldPaths walks t's fields the same way field.GetStructMeta does
// (recursing into anonymous struct fields, honouring db:"-"/ch:"-"), so its
// result lines up index-for-index with field.GetStructMeta(t)'s output. It
// additionally rejects shapes field.GetStructMeta accepts but maps
// unsafely: an anonymous pointer field, an anonymous unexported field, and
// an anonymous struct field carrying a db/ch tag (silently ignored by
// field.GetStructMeta, since flattening happens before tag parsing).
func fieldPaths(t reflect.Type, prefix []int) ([][]int, error) {
	var out [][]int
	for i := 0; i < t.NumField(); i++ {
		sf := t.Field(i)
		idx := make([]int, len(prefix)+1)
		copy(idx, prefix)
		idx[len(prefix)] = i

		if !sf.IsExported() {
			if sf.Anonymous {
				return nil, fmt.Errorf("%w: unexported embedded field %q", ErrRecordShape, sf.Name)
			}
			continue
		}

		if sf.Anonymous {
			if sf.Type.Kind() == reflect.Ptr {
				return nil, fmt.Errorf("%w: embedded pointer field %q", ErrRecordShape, sf.Name)
			}
			reserved := field.IsReservedType(sf.Type.String())
			if sf.Type.Kind() == reflect.Struct && !reserved {
				if sf.Tag.Get("db") != "" || sf.Tag.Get("ch") != "" {
					return nil, fmt.Errorf("%w: tagged embedded field %q", ErrRecordShape, sf.Name)
				}
				sub, err := fieldPaths(sf.Type, idx)
				if err != nil {
					return nil, err
				}
				out = append(out, sub...)
				continue
			}
		}

		dbTag := sf.Tag.Get("db")
		if dbTag == "" {
			dbTag = sf.Tag.Get("ch")
		}
		if dbTag == "-" {
			continue
		}
		out = append(out, idx)
	}
	return out, nil
}

// recordMeta returns t's field metadata and matching field-index paths,
// after the shape checks in fieldPaths and two global checks
// field.GetStructMeta does not perform: two fields promoted to the same Go
// name (ErrRecordShape - qb resolves this ambiguously via FieldByName) and
// two fields mapped to the same db column (ErrDuplicateColumn), wherever in
// the struct they occur.
func recordMeta(t reflect.Type) ([]field.Metadata, [][]int, error) {
	paths, err := fieldPaths(t, nil)
	if err != nil {
		return nil, nil, err
	}
	metas, err := field.GetStructMeta(t)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %v", ErrRecordShape, err)
	}
	if len(metas) != len(paths) {
		return nil, nil, fmt.Errorf("%w: field count mismatch", ErrRecordShape)
	}

	seenNames := make(map[string]bool, len(metas))
	seenDb := make(map[string]bool, len(metas))
	for _, m := range metas {
		if seenNames[m.Name] {
			return nil, nil, fmt.Errorf("%w: ambiguous promoted field %q", ErrRecordShape, m.Name)
		}
		seenNames[m.Name] = true
		if seenDb[m.DbName] {
			return nil, nil, fmt.Errorf("%w: column %q set more than once", ErrDuplicateColumn, m.DbName)
		}
		seenDb[m.DbName] = true
	}
	return metas, paths, nil
}

// matchesAny reports whether names contains structName or dbName.
func matchesAny(names []string, structName, dbName string) bool {
	for _, n := range names {
		if n == structName || n == dbName {
			return true
		}
	}
	return false
}

// validateFieldNames fails with ErrUnknownField for any name in names that
// matches no field's struct name or db name.
func validateFieldNames(names []string, metas []field.Metadata) error {
	for _, n := range names {
		found := false
		for _, m := range metas {
			if m.Name == n || m.DbName == n {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("%w: %q", ErrUnknownField, n)
		}
	}
	return nil
}

// extractValue resolves fv's bindable value: a non-nil pointer binds its
// pointee, a nil pointer binds nil.
func extractValue(fv reflect.Value) any {
	if fv.Kind() == reflect.Ptr {
		if fv.IsNil() {
			return nil
		}
		return fv.Elem().Interface()
	}
	return fv.Interface()
}

// recordValues resolves rec's insertable (forInsert) or updatable columns
// and values, per the rules in plans/003: Auto fields are skipped (insert
// always; update unless WithAutoFields); update additionally applies
// include/exclude (checked after Auto, before OmitNil/OmitEmpty) and
// SkipZeroValues. Insert returns ErrNoColumns when nothing is left to
// write; update may legitimately return zero columns (the caller
// aggregates across Set/SetMap/SetRecord before checking).
func recordValues(rec any, forInsert bool, o recordOptions) ([]string, []any, error) {
	v, err := recordStructValue(rec)
	if err != nil {
		return nil, nil, err
	}
	metas, paths, err := recordMeta(v.Type())
	if err != nil {
		return nil, nil, err
	}

	if !forInsert {
		if err := validateFieldNames(o.include, metas); err != nil {
			return nil, nil, err
		}
		if err := validateFieldNames(o.exclude, metas); err != nil {
			return nil, nil, err
		}
	}

	var cols []string
	var vals []any
	for i, meta := range metas {
		if meta.Auto {
			if forInsert || !o.withAuto {
				continue
			}
		}

		if !forInsert {
			skip := false
			if len(o.include) > 0 {
				if !matchesAny(o.include, meta.Name, meta.DbName) {
					skip = true
				}
			} else if matchesAny(o.exclude, meta.Name, meta.DbName) {
				skip = true
			}
			if skip {
				continue
			}
		}

		fv := v.FieldByIndex(paths[i])
		if meta.OmitNil && fv.Kind() == reflect.Ptr && fv.IsNil() {
			continue
		}
		if meta.OmitEmpty && fv.IsZero() {
			continue
		}
		if !forInsert && o.skipZero && fv.IsZero() {
			continue
		}

		cols = append(cols, meta.DbName)
		vals = append(vals, extractValue(fv))
	}

	if forInsert && len(cols) == 0 {
		return nil, nil, ErrNoColumns
	}
	return cols, vals, nil
}

// RecordColumns returns every column t maps (Auto fields included, in
// field.GetStructMeta order), after the same shape and duplicate checks as
// recordValues. t may be a struct type or a pointer to one.
func RecordColumns(t reflect.Type) ([]string, error) {
	for t != nil && t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t == nil || t.Kind() != reflect.Struct {
		return nil, ErrInvalidRecord
	}
	metas, _, err := recordMeta(t)
	if err != nil {
		return nil, err
	}
	cols := make([]string, len(metas))
	for i, m := range metas {
		cols[i] = m.DbName
	}
	return cols, nil
}

// InsertColumns returns the columns an INSERT of rec would write: every
// mapped column except Auto fields, a nil pointer with OmitNil, and a zero
// value with OmitEmpty.
func InsertColumns(rec any) ([]string, error) {
	cols, _, err := recordValues(rec, true, recordOptions{})
	return cols, err
}
