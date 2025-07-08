package field

import (
	"fmt"
	"github.com/oddbit-project/blueprint/runtime"
	"github.com/oddbit-project/blueprint/utils"
	"reflect"
	"sync"
)

const (
	ErrNilPointer    = utils.Error("ptr to struct to be parsed by field spec is nil")
	ErrInvalidStruct = utils.Error("field spec requires a pointer to a struct; invalid type")

	// tags
	tagDb   = "db"   // database/sql tag
	tagCh   = "ch"   // clickhouse tag
	tagAuto = "auto" // optional tag that defines if field is automatically generated (skipupdate/skipinsert produce the same result)
	tagGrid = "grid" // grid options
	tagGoqu = "goqu" // possible tag names: skipupdate, skipinsert, defaultifempty, omitnil, omitempty

	// alias tags - used to search for a possible Alias value
	tagJson  = "json"
	tagXml   = "xml"
	tagAlias = "alias"

	// misc tag values
	optAuto   = "auto" // grid tag alias for auto:true
	optTrue   = "true"
	optSort   = "sort"
	optSearch = "search"
	optFilter = "filter"

	// goqu tag values
	optSkipUpdate = "skipupdate"
	optSkipInsert = "skipinsert"
	optOmitNil    = "omitnil"
	optOmitEmpty  = "omitempty"
)

type Metadata struct {
	Name       string `json:"fieldName"`  // actual field name
	DbName     string `json:"dbName"`     // database field name
	Alias      string `json:"fieldAlias"` // serialized name (json,xml, etc)
	Sortable   bool   `json:"sortable"`   // true if field is sortable
	Filterable bool   `json:"filterable"` // true if field is filterable
	Searchable bool   `json:"searchable"` // true if field is searchable
	Auto       bool   `json:"auto"`       // true if field is automatically generated
	OmitNil    bool   `json:"omitNil"`    // true if field is skipped on insert if nil
	OmitEmpty  bool   `json:"omitEmpty"`  // true if field is skipped on insert if empty
	TypeName   string `json:"typeName"`   // field type name (from reflect.Type.Name())
	Type       reflect.Type
	DbOptions  []string `json:"dbOptions"` // additional db options like goqu, etc
}

var (
	dbTagList    = []string{tagDb, tagCh}              // valid database tags
	aliasTagList = []string{tagAlias, tagJson, tagXml} // valid alias tags
	fieldCache   = sync.Map{}                          // map[reflect.Type]FieldMetadata
)

func GetStructMeta(t reflect.Type) ([]Metadata, error) {
	if cached, ok := fieldCache.Load(t); ok {
		return cached.([]Metadata), nil
	}
	// Create a new instance of the type to scan
	v := reflect.New(t).Elem()
	meta, err := scanStruct(v.Interface())
	if err == nil {
		fieldCache.Store(t, meta)
	}
	return meta, err
}

func scanStruct(arg any) ([]Metadata, error) {
	v := reflect.ValueOf(arg)

	// if ptr, unwrap
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return nil, ErrNilPointer
		}
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		return nil, ErrInvalidStruct
	}
	if cached, exists := fieldCache.Load(v.Type()); exists {
		return cached.([]Metadata), nil
	}

	result := make([]Metadata, 0, v.NumField())
	for i := 0; i < v.NumField(); i++ {
		field := v.Type().Field(i)
		// Skip unexported fields
		if !field.IsExported() {
			continue
		}
		
		reserved := IsReservedType(field.Type.String())
		if field.Type.Kind() == reflect.Struct && !reserved {
			// resolve embedded structs
			structMap, err := scanStruct(v.Field(i).Interface())
			if err != nil {
				return nil, err
			}
			result = append(result, structMap...)
		} else {
			// resolve fields
			meta := Metadata{
				Name:      field.Name,
				DbName:    field.Name, // default is field.Name
				Alias:     field.Name, // default is field.Name
				Type:      field.Type,
				TypeName:  field.Type.String(),
				DbOptions: make([]string, 0),
			}

			// attempt to parse dbName
			// additional tags other than name are added to DbOptions
			tagContent := runtime.ParseTagList(field, dbTagList)
			if len(tagContent) > 0 {
				meta.DbName = tagContent[0]
				if len(tagContent) > 1 {
					meta.DbOptions = tagContent[1:]
				}
			}

			// attempt to parse aliasName
			// additional tags other than name are ignored
			tagContent = runtime.ParseTagList(field, aliasTagList)
			if len(tagContent) > 0 {
				meta.Alias = tagContent[0]
			}

			// attempt to parse auto
			// the ony valid value is "true"
			tagContent = runtime.ParseTag(field, tagAuto)
			if len(tagContent) > 0 && tagContent[0] == optTrue {
				meta.Auto = true
			}

			// attempt to parse grid flags
			tagContent = runtime.ParseTag(field, tagGrid)
			for _, tag := range tagContent {
				switch tag {
				case optSearch:
					meta.Searchable = true
				case optFilter:
					meta.Filterable = true
				case optSort:
					meta.Sortable = true
				case optAuto:
					meta.Auto = true
				default:
					meta.DbOptions = append(meta.DbOptions, tag)
				}
			}

			// attempt to parse goqu flags
			tagContent = runtime.ParseTag(field, tagGoqu)
			for _, tag := range tagContent {
				switch tag {
				case optSkipInsert, optSkipUpdate:
					meta.Auto = true
				case optOmitEmpty:
					meta.OmitEmpty = true
				case optOmitNil:
					meta.OmitNil = true
				default:
					meta.DbOptions = append(meta.DbOptions, tag)
				}
			}

			for _, f := range result {
				if f.DbName == meta.DbName {
					return nil, fmt.Errorf("duplicate field name for field %s", f.DbName)
				}
			}

			result = append(result, meta)
		}
	}
	return result, nil
}
