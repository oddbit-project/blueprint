package clickhouse

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
	"sync"

	"github.com/oddbit-project/blueprint/dbx"
	"github.com/oddbit-project/blueprint/utils"
	"github.com/oddbit-project/gohan"
	"github.com/oddbit-project/gohan/field"
)

// ErrRecordMapping is returned by Querier.CheckRecord (and so by
// dbx.NewRepository over a Querier) when a column gohan builds from a
// record's `db` tags would not be scanned into, or appended from, the same
// field by clickhouse-go's `ch`-tag struct mapper.
const ErrRecordMapping = utils.Error("clickhouse: record columns do not map to the same fields in clickhouse-go")

var _ dbx.RecordChecker = (*Querier)(nil)

// recordChecks records the reflect.Types that passed CheckRecord (failures
// are not cached).
var recordChecks sync.Map // map[reflect.Type]bool

// CheckRecord implements dbx.RecordChecker. It fails with ErrRecordMapping,
// naming each offending column, field and reason, when any column
// gohan.RecordColumns reports for t would be mapped by clickhouse-go's
// struct mapper (used by ScanStruct and AppendStruct, so by Get, Select and
// InsertBatch) to a different field or to none; errors from
// gohan.RecordColumns are returned unchanged. Fields the driver maps but
// gohan does not (e.g. `db:"-"` without a `ch` tag) are allowed: dbx only
// selects and inserts its own column list, so the driver never looks them
// up. An embedded non-struct field is rejected even when gohan skips it,
// because clickhouse-go's mapper panics on it.
//
// The rules mirror clickhouse-go v2.40.3's struct_map.go (via this
// package's copy, structIdx) and must be re-verified on every driver
// upgrade. A passing result is cached per type; a failure is not, so a
// record type that becomes valid later (e.g. after field.AddReservedType)
// is re-checked. Register reserved types before creating repositories.
func (q *Querier) CheckRecord(t reflect.Type) error {
	if _, ok := recordChecks.Load(t); ok {
		return nil
	}
	err := checkRecord(t)
	if err == nil {
		recordChecks.Store(t, true)
	}
	return err
}

func checkRecord(t reflect.Type) error {
	cols, err := gohan.RecordColumns(t)
	if err != nil {
		return err
	}
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	var problems []string
	for _, p := range embeddedNonStructs(t, nil) {
		f := t.FieldByIndex(p)
		problems = append(problems, fmt.Sprintf("field %s: embedded non-struct type %s, on which clickhouse-go's struct mapper panics", fieldPathName(t, p), f.Type))
	}
	if len(problems) > 0 {
		return fmt.Errorf("%w: %s: %s", ErrRecordMapping, t, strings.Join(problems, "; "))
	}

	paths := gohanFieldPaths(t, nil)
	if len(paths) != len(cols) {
		return fmt.Errorf("%w: %s: %d columns but %d mapped fields", ErrRecordMapping, t, len(cols), len(paths))
	}
	chIdx := structIdx(t)
	for i, col := range cols {
		p := paths[i]
		if got, ok := chIdx[col]; ok && slices.Equal(got, p) {
			continue
		}
		problems = append(problems, fmt.Sprintf("column %q (field %s): %s", col, fieldPathName(t, p), mismatchReason(t, p, col, chIdx)))
	}
	if len(problems) > 0 {
		return fmt.Errorf("%w: %s: %s", ErrRecordMapping, t, strings.Join(problems, "; "))
	}
	return nil
}

// gohanFieldPaths returns the index path of every field gohan maps to a
// column, in gohan.RecordColumns order. It assumes t already passed
// gohan.RecordColumns: exported fields only, recursing into embedded
// non-reserved structs, skipping `db:"-"` (or `ch:"-"` when there is no db
// tag).
func gohanFieldPaths(t reflect.Type, prefix []int) [][]int {
	var out [][]int
	for i := 0; i < t.NumField(); i++ {
		sf := t.Field(i)
		if !sf.IsExported() {
			continue
		}
		p := append(slices.Clone(prefix), i)
		if sf.Anonymous && sf.Type.Kind() == reflect.Struct && !field.IsReservedType(sf.Type.String()) {
			out = append(out, gohanFieldPaths(sf.Type, p)...)
			continue
		}
		name := sf.Tag.Get("db")
		if name == "" {
			name = sf.Tag.Get("ch")
		}
		if name == "-" {
			continue
		}
		out = append(out, p)
	}
	return out
}

// embeddedNonStructs returns the index paths of the embedded fields
// clickhouse-go's mapper would recurse into although they are not structs
// (it calls NumField on them, which panics): anonymous, non-pointer, not
// `ch:"-"`, reached through the same embedded structs the mapper descends.
func embeddedNonStructs(t reflect.Type, prefix []int) [][]int {
	var out [][]int
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.Anonymous || f.Tag.Get("ch") == "-" || f.Type.Kind() == reflect.Pointer {
			continue
		}
		p := append(slices.Clone(prefix), i)
		if f.Type.Kind() != reflect.Struct {
			out = append(out, p)
			continue
		}
		out = append(out, embeddedNonStructs(f.Type, p)...)
	}
	return out
}

// mismatchReason explains why the field at path p, which gohan maps to
// col, is not the field clickhouse-go maps col to.
func mismatchReason(t reflect.Type, p []int, col string, chIdx map[string][]int) string {
	f := t.FieldByIndex(p)
	tag := f.Tag.Get("ch")
	switch {
	case f.Anonymous:
		return fmt.Sprintf("embedded %s is one column for dbx, but clickhouse-go maps the fields inside it, not the field itself", f.Type)
	case tag == "-":
		return `tagged ch:"-", so clickhouse-go does not map it`
	case tag != "" && tag != col:
		return fmt.Sprintf("clickhouse-go maps it as %q (its ch tag)", tag)
	case tag == "" && f.Name != col:
		return fmt.Sprintf("clickhouse-go maps it as %q (its Go field name; no ch tag)", f.Name)
	}
	return fmt.Sprintf("clickhouse-go maps %q to field %s instead (the later field with that name wins)", col, fieldPathName(t, chIdx[col]))
}

// fieldPathName renders the index path p in t as dotted Go field names.
func fieldPathName(t reflect.Type, p []int) string {
	names := make([]string, len(p))
	for i, idx := range p {
		f := t.Field(idx)
		names[i] = f.Name
		t = f.Type
	}
	return strings.Join(names, ".")
}
