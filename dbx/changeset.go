package dbx

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"maps"
	"reflect"
	"strconv"
	"strings"

	"github.com/oddbit-project/blueprint/types/optional"
	"github.com/oddbit-project/blueprint/utils"
	"github.com/oddbit-project/gohan"
	"github.com/oddbit-project/gohan/field"
)

const (
	// ErrAutoColumn is returned by Changeset.Set for a column whose field is
	// marked auto (db:",auto", auto:"true", grid:"auto", or goqu
	// skipinsert/skipupdate): such columns are never written by an update.
	ErrAutoColumn = utils.Error("dbx: column is auto-generated and cannot be set")
	// ErrValueType is returned by Changeset.Set when a value cannot be stored
	// in the column's field type without loss.
	ErrValueType = utils.Error("dbx: value does not fit the column's field type")
	// ErrNilRecord is returned by Changes when either record is nil.
	ErrNilRecord = utils.Error("dbx: record is nil")
)

// Changeset collects validated column values for a partial update of a
// Repository[T]'s table. Each Set is checked against T before it is
// accepted, so Changes() can be passed to Repository.UpdateFields without
// the database being the first to notice a bad column or value. A
// Changeset is not safe for concurrent use.
type Changeset[T any] struct {
	repo    *Repository[T]
	fields  map[string]recordField
	err     error
	changes map[string]any
}

// NewChangeset returns an empty Changeset for repo's record type.
func NewChangeset[T any](repo *Repository[T]) *Changeset[T] {
	c := &Changeset[T]{repo: repo, changes: make(map[string]any)}
	fields, err := recordFields(reflect.TypeFor[T]())
	if err != nil {
		// unreachable for a repository built by NewRepository, which
		// already rejected T's shape; reported by every Set.
		c.err = err
		return c
	}
	c.fields = make(map[string]recordField, len(fields))
	for _, f := range fields {
		c.fields[f.meta.DbName] = f
	}
	return c
}

// Set records value for column, replacing any earlier value for the same
// column. column is a db column name (not a Go field name). Set fails,
// leaving the changeset unchanged, with:
//
//   - ErrUnknownColumn if column is not one of the repository's columns;
//   - ErrAutoColumn if the column's field is marked auto;
//   - ErrValueType if value does not fit the field's type.
//
// A value fits when it is assignable to the field type, or converts to it
// without loss:
//
//   - numbers convert between integer and float kinds only when the value
//     is exactly representable: float64(30) fits an int field, 30.7, NaN,
//     ±Inf, an out-of-range value or a negative value into an unsigned
//     field do not; an integer too large to be exact in the float field is
//     rejected; a float64 into a float32 field is rounded (only a
//     magnitude beyond float32's range is rejected);
//   - a json.Number (from json.Decoder.UseNumber) fits a numeric field
//     under the same rules, parsed exactly: an integer field takes only an
//     integer literal ("30", not "30.0" or "3e1"), so an integer above 2^53
//     is not rounded as a float64 would be; a non-numeric json.Number, or
//     one for a non-numeric field, does not fit;
//   - otherwise a conversion is allowed only between types of the same
//     kind (e.g. string into a named string type) or between string and
//     []byte, so an int is never turned into a one-rune string.
//
// For a pointer field the value may be given either as a pointer or as
// the pointee; the stored value is the pointee (nil for a nil pointer),
// which is what the driver binds. nil is accepted only for fields that can
// hold NULL: pointers, interfaces, maps, slices, and NULL-style structs
// whose pointer implements sql.Scanner and that have an exported bool
// field named Valid (sql.NullString, sql.Null[T], pgtype.Text, ...); other
// Scanner types such as uuid.UUID or jsoncol.JSON[T] reject nil (use a
// pointer field for a nullable column). A string is not parsed into a
// time.Time: decode such values before calling Set.
func (c *Changeset[T]) Set(column string, value any) error {
	if c.err != nil {
		return c.err
	}
	if !c.repo.colSet[column] {
		return fmt.Errorf("%w: %q", ErrUnknownColumn, column)
	}
	f := c.fields[column]
	if f.meta.Auto {
		return fmt.Errorf("%w: %q", ErrAutoColumn, column)
	}
	v, ok := fitValue(value, f.meta.Type)
	if !ok {
		return fmt.Errorf("%w: column %q is %s, got %T", ErrValueType, column, f.meta.Type, value)
	}
	c.changes[column] = v
	return nil
}

// Changes returns a copy of the accepted column values, keyed by column
// name, ready for Repository.UpdateFields. It is empty (not nil) when
// nothing was set; check its length before updating, since an UPDATE with
// no columns fails to build.
func (c *Changeset[T]) Changes() map[string]any {
	return maps.Clone(c.changes)
}

// SetOptional applies a tri-state value to c: None leaves column
// untouched (and is not validated), Null sets it to nil, and Some(v) sets
// it to v. Null and Some are validated exactly as by Changeset.Set.
func SetOptional[T, V any](c *Changeset[T], column string, o optional.Optional[V]) error {
	if !o.IsSet() {
		return nil
	}
	v, ok := o.Get()
	if !ok {
		return c.Set(column, nil)
	}
	return c.Set(column, v)
}

// Changes compares oldRec and newRec field by field and returns, keyed by
// column name, newRec's value for every non-auto column whose value
// differs, for Repository.UpdateFields ("load, modify, save the diff").
// Fields are compared with reflect.DeepEqual, so pointers compare by
// pointee and slices/maps by content; a time.Time compares strictly, so
// the same instant in another location (or with a monotonic reading
// stripped) counts as a change. A pointer field's value is its pointee
// (nil for a nil pointer). It fails with ErrNilRecord for a nil record,
// ErrNotStruct for a non-struct T, and gohan's shape errors for a T that
// NewRepository would reject.
func Changes[T any](oldRec, newRec *T) (map[string]any, error) {
	if oldRec == nil || newRec == nil {
		return nil, ErrNilRecord
	}
	t := reflect.TypeFor[T]()
	if t.Kind() != reflect.Struct {
		return nil, ErrNotStruct
	}
	fields, err := recordFields(t)
	if err != nil {
		return nil, err
	}
	ov := reflect.ValueOf(oldRec).Elem()
	nv := reflect.ValueOf(newRec).Elem()
	out := make(map[string]any)
	for _, f := range fields {
		if f.meta.Auto {
			continue
		}
		a := ov.FieldByIndex(f.index)
		b := nv.FieldByIndex(f.index)
		if reflect.DeepEqual(a.Interface(), b.Interface()) {
			continue
		}
		if b.Kind() == reflect.Pointer {
			if b.IsNil() {
				out[f.meta.DbName] = nil
				continue
			}
			b = b.Elem()
		}
		out[f.meta.DbName] = b.Interface()
	}
	return out, nil
}

// recordField pairs a mapped field's metadata with its index path in the
// record struct.
type recordField struct {
	meta  field.Metadata
	index []int
}

// recordFields returns t's mapped fields in field.GetStructMeta order,
// after gohan's record-shape checks (the same NewRepository applies).
func recordFields(t reflect.Type) ([]recordField, error) {
	if _, err := gohan.RecordColumns(t); err != nil {
		return nil, err
	}
	metas, err := field.GetStructMeta(t)
	if err != nil {
		return nil, err
	}
	paths := fieldIndexPaths(t, nil)
	if len(paths) != len(metas) {
		return nil, fmt.Errorf("%w: field count mismatch", gohan.ErrRecordShape)
	}
	out := make([]recordField, len(metas))
	for i, m := range metas {
		if t.FieldByIndex(paths[i]).Name != m.Name {
			return nil, fmt.Errorf("%w: field order mismatch at %q", gohan.ErrRecordShape, m.Name)
		}
		out[i] = recordField{meta: m, index: paths[i]}
	}
	return out, nil
}

// fieldIndexPaths walks t the way field.GetStructMeta does: exported
// fields only, anonymous non-reserved struct fields flattened, and fields
// whose db (or, failing that, ch) tag starts with "-" skipped.
func fieldIndexPaths(t reflect.Type, prefix []int) [][]int {
	var out [][]int
	for i := 0; i < t.NumField(); i++ {
		sf := t.Field(i)
		if !sf.IsExported() {
			continue
		}
		idx := append(append([]int(nil), prefix...), i)
		if sf.Anonymous && sf.Type.Kind() == reflect.Struct && !field.IsReservedType(sf.Type.String()) {
			out = append(out, fieldIndexPaths(sf.Type, idx)...)
			continue
		}
		tag := sf.Tag.Get("db")
		if tag == "" {
			tag = sf.Tag.Get("ch")
		}
		if name, _, _ := strings.Cut(tag, ","); name == "-" {
			continue
		}
		out = append(out, idx)
	}
	return out
}

var scannerType = reflect.TypeFor[sql.Scanner]()

// nullable reports whether a field of type t can be set to nil: a pointer,
// interface, map or slice, or a NULL-style struct (sql.NullString,
// sql.Null[T], pgtype.Text, ...) whose pointer implements sql.Scanner and
// which has an exported bool field named Valid. Other Scanner types
// (uuid.UUID, jsoncol.JSON[T], ...) do not represent NULL.
func nullable(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Pointer, reflect.Interface, reflect.Map, reflect.Slice:
		return true
	case reflect.Struct:
		if !reflect.PointerTo(t).Implements(scannerType) {
			return false
		}
		f, ok := t.FieldByName("Valid")
		return ok && f.IsExported() && f.Type.Kind() == reflect.Bool
	}
	return false
}

// fitValue returns value as it should be bound for a field of type ft, or
// false if it does not fit (see Changeset.Set).
func fitValue(value any, ft reflect.Type) (any, bool) {
	if ft.Kind() == reflect.Interface && value != nil && reflect.TypeOf(value).AssignableTo(ft) {
		return value, true
	}
	rv := reflect.ValueOf(value)
	for rv.IsValid() && rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			rv = reflect.Value{}
			break
		}
		rv = rv.Elem()
	}
	if !rv.IsValid() {
		return nil, nullable(ft)
	}

	tt := ft
	if tt.Kind() == reflect.Pointer {
		tt = tt.Elem()
	}
	vt := rv.Type()
	if vt == jsonNumberType && !vt.AssignableTo(tt) {
		return fitJSONNumber(json.Number(rv.String()), tt)
	}
	switch {
	case isNumberKind(vt.Kind()) && isNumberKind(tt.Kind()):
		out, ok := convertNumber(rv, tt)
		if !ok {
			return nil, false
		}
		return out.Interface(), true
	case vt.AssignableTo(tt):
		return rv.Interface(), true
	case vt.Kind() == tt.Kind() && vt.ConvertibleTo(tt),
		(isStringBytes(vt, tt) || isStringBytes(tt, vt)) && vt.ConvertibleTo(tt):
		return rv.Convert(tt).Interface(), true
	}
	return nil, false
}

var jsonNumberType = reflect.TypeFor[json.Number]()

// fitJSONNumber converts n (from json.Decoder.UseNumber) to the numeric
// type tt without loss: an integer field takes only an integer literal
// (parsed exactly, no fraction or exponent), a float field any JSON
// number. A non-numeric n, or a non-numeric tt, does not fit.
func fitJSONNumber(n json.Number, tt reflect.Type) (any, bool) {
	s := string(n)
	if s == "" || (s[0] != '-' && (s[0] < '0' || s[0] > '9')) || !json.Valid([]byte(s)) {
		return nil, false
	}
	var src reflect.Value
	switch k := tt.Kind(); {
	case isIntKind(k):
		i, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return nil, false
		}
		src = reflect.ValueOf(i)
	case isUintKind(k):
		u, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			return nil, false
		}
		src = reflect.ValueOf(u)
	case isFloatKind(k):
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return nil, false
		}
		src = reflect.ValueOf(f)
	default:
		return nil, false
	}
	out, ok := convertNumber(src, tt)
	if !ok {
		return nil, false
	}
	return out.Interface(), true
}

// isStringBytes reports whether a is a string kind and b a byte slice.
func isStringBytes(a, b reflect.Type) bool {
	return a.Kind() == reflect.String && b.Kind() == reflect.Slice && b.Elem().Kind() == reflect.Uint8
}

func isIntKind(k reflect.Kind) bool {
	return k >= reflect.Int && k <= reflect.Int64
}

func isUintKind(k reflect.Kind) bool {
	return k >= reflect.Uint && k <= reflect.Uintptr
}

func isFloatKind(k reflect.Kind) bool {
	return k == reflect.Float32 || k == reflect.Float64
}

func isNumberKind(k reflect.Kind) bool {
	return isIntKind(k) || isUintKind(k) || isFloatKind(k)
}

const (
	twoTo63 = float64(1 << 63)
	twoTo64 = twoTo63 * 2
)

// convertNumber converts the numeric rv to the numeric type tt, failing
// when the value is not exactly representable in tt (float to float only
// fails on overflow).
func convertNumber(rv reflect.Value, tt reflect.Type) (reflect.Value, bool) {
	out := reflect.New(tt).Elem()
	tk := tt.Kind()
	switch sk := rv.Kind(); {
	case isIntKind(sk):
		i := rv.Int()
		switch {
		case isIntKind(tk):
			if out.OverflowInt(i) {
				return out, false
			}
			out.SetInt(i)
		case isUintKind(tk):
			if i < 0 || out.OverflowUint(uint64(i)) {
				return out, false
			}
			out.SetUint(uint64(i))
		default:
			f := float64(i)
			if tk == reflect.Float32 {
				f = float64(float32(i))
			}
			if f >= twoTo63 || int64(f) != i {
				return out, false
			}
			out.SetFloat(f)
		}
	case isUintKind(sk):
		u := rv.Uint()
		switch {
		case isIntKind(tk):
			if u > 1<<63-1 || out.OverflowInt(int64(u)) {
				return out, false
			}
			out.SetInt(int64(u))
		case isUintKind(tk):
			if out.OverflowUint(u) {
				return out, false
			}
			out.SetUint(u)
		default:
			f := float64(u)
			if tk == reflect.Float32 {
				f = float64(float32(u))
			}
			if f >= twoTo64 || uint64(f) != u {
				return out, false
			}
			out.SetFloat(f)
		}
	default:
		f := rv.Float()
		switch {
		case isIntKind(tk):
			if f < -twoTo63 || f >= twoTo63 || f != float64(int64(f)) || out.OverflowInt(int64(f)) {
				return out, false
			}
			out.SetInt(int64(f))
		case isUintKind(tk):
			if f < 0 || f >= twoTo64 || f != float64(uint64(f)) || out.OverflowUint(uint64(f)) {
				return out, false
			}
			out.SetUint(uint64(f))
		default:
			if out.OverflowFloat(f) {
				return out, false
			}
			out.SetFloat(f)
		}
	}
	return out, true
}
