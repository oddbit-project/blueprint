package dbx

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/oddbit-project/gohan/field"
)

// keyKind is the family of Go types a keyset key can have.
type keyKind uint8

const (
	keyInt keyKind = iota + 1
	keyUint
	keyString
	keyTime
	keyUUID
)

// keyType is a keyset key's Go type, reduced to what the cursor encodes: its
// kind and, for integers, its width in bits.
type keyType struct {
	kind keyKind
	bits int
}

// name returns the type's name as recorded in the cursor fingerprint.
func (k keyType) name() string {
	switch k.kind {
	case keyInt:
		return "int" + strconv.Itoa(k.bits)
	case keyUint:
		return "uint" + strconv.Itoa(k.bits)
	case keyString:
		return "string"
	case keyTime:
		return "time"
	default:
		return "uuid"
	}
}

var (
	timeType = reflect.TypeFor[time.Time]()
	uuidType = reflect.TypeFor[uuid.UUID]()
)

// keyTypeOf classifies t as a keyset key type, or explains why it cannot be
// one. time.Time and uuid.UUID are accepted as is; pointers (nullable) and
// other driver.Valuer/sql.Scanner types (whose database representation dbx
// cannot know) are rejected; any other type is accepted by its kind when
// that is an integer (not uintptr) or string.
func keyTypeOf(t reflect.Type) (keyType, error) {
	switch {
	case t == timeType:
		return keyType{kind: keyTime}, nil
	case t == uuidType:
		return keyType{kind: keyUUID}, nil
	case t.Kind() == reflect.Pointer:
		return keyType{}, fmt.Errorf("type %s is nullable", t)
	case t.Implements(valuerType), reflect.PointerTo(t).Implements(valuerType),
		t.Implements(scannerType), reflect.PointerTo(t).Implements(scannerType):
		return keyType{}, fmt.Errorf("type %s has a custom database representation", t)
	}
	switch t.Kind() {
	case reflect.Int:
		return keyType{kind: keyInt, bits: strconv.IntSize}, nil
	case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return keyType{kind: keyInt, bits: t.Bits()}, nil
	case reflect.Uint:
		return keyType{kind: keyUint, bits: strconv.IntSize}, nil
	case reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return keyType{kind: keyUint, bits: t.Bits()}, nil
	case reflect.String:
		return keyType{kind: keyString}, nil
	}
	return keyType{}, fmt.Errorf("type %s is not supported", t)
}

// keysetField is one mapped field of a record type: its index path and Go
// type.
type keysetField struct {
	path []int
	typ  reflect.Type
}

// keysetFieldsResult is a keysetFieldCache entry.
type keysetFieldsResult struct {
	fields map[string]keysetField
	err    error
}

var keysetFieldCache sync.Map // map[reflect.Type]keysetFieldsResult

// keysetFieldsFor returns t's mapped fields by column, with the index path
// that reaches each one. reflect's FieldByName cannot be used instead: it
// would pick a shallower field shadowing an embedded mapped one even when
// the shallower field is unmapped (db:"-").
func keysetFieldsFor(t reflect.Type) (map[string]keysetField, error) {
	if cached, ok := keysetFieldCache.Load(t); ok {
		res := cached.(keysetFieldsResult)
		return res.fields, res.err
	}
	fields, err := buildKeysetFields(t)
	keysetFieldCache.Store(t, keysetFieldsResult{fields: fields, err: err})
	return fields, err
}

// walkedField is one field found by walkFields.
type walkedField struct {
	column string
	name   string
	keysetField
}

// walkFields lists t's mapped fields in the order field.GetStructMeta lists
// them: exported fields, recursing into anonymous struct fields that are not
// a reserved type, skipping db:"-" (or ch:"-"). The column is the first part
// of the db tag, else of the ch tag, else the lower-cased field name.
func walkFields(t reflect.Type, prefix []int) []walkedField {
	var out []walkedField
	for i := 0; i < t.NumField(); i++ {
		sf := t.Field(i)
		if !sf.IsExported() {
			continue
		}
		path := append(append(make([]int, 0, len(prefix)+1), prefix...), i)
		if sf.Anonymous && sf.Type.Kind() == reflect.Struct && !field.IsReservedType(sf.Type.String()) {
			out = append(out, walkFields(sf.Type, path)...)
			continue
		}
		tag := sf.Tag.Get("db")
		if tag == "" {
			tag = sf.Tag.Get("ch")
		}
		column := strings.ToLower(sf.Name)
		if tag != "" {
			column, _, _ = strings.Cut(tag, ",")
			if column == "-" {
				continue
			}
		}
		out = append(out, walkedField{column: column, name: sf.Name, keysetField: keysetField{path: path, typ: sf.Type}})
	}
	return out
}

// buildKeysetFields walks t and cross-checks the result against
// field.GetStructMeta, field by field, so the index paths address exactly
// the fields the rest of dbx maps.
func buildKeysetFields(t reflect.Type) (map[string]keysetField, error) {
	walked := walkFields(t, nil)
	metas, err := field.GetStructMeta(t)
	if err != nil {
		return nil, err
	}
	if len(metas) != len(walked) {
		return nil, fmt.Errorf("dbx: keyset field index of %s disagrees with its field metadata", t)
	}
	fields := make(map[string]keysetField, len(walked))
	for i, w := range walked {
		if metas[i].DbName != w.column || metas[i].Name != w.name {
			return nil, fmt.Errorf("dbx: keyset field index of %s disagrees with its field metadata", t)
		}
		fields[w.column] = w.keysetField
	}
	return fields, nil
}

// keyValue reads the canonical value of a keyset key of type kt from v:
// int64, uint64, string, a UTC time.Time (without monotonic reading) or a
// uuid.UUID. These are the values bound in the seek predicate.
func keyValue(v reflect.Value, kt keyType) any {
	switch kt.kind {
	case keyInt:
		return v.Int()
	case keyUint:
		return v.Uint()
	case keyString:
		return v.String()
	case keyTime:
		return v.Interface().(time.Time).UTC()
	default:
		return v.Interface().(uuid.UUID)
	}
}
