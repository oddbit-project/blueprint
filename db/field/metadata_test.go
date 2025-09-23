package field

import (
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test structures for metadata extraction
type SimpleStruct struct {
	ID   int    `db:"id"`
	Name string `db:"name" json:"user_name"`
}

type StructWithTags struct {
	ID          int       `db:"id" auto:"true" goqu:"skipinsert"`
	Name        string    `db:"name" json:"full_name" grid:"sort,search"`
	Email       string    `db:"email" ch:"email_addr" grid:"filter"`
	Age         int       `db:"age" goqu:"omitempty"`
	Score       *float64  `db:"score" goqu:"omitnil"`
	CreatedAt   time.Time `db:"created_at" auto:"true"`
	UpdatedAt   time.Time `db:"updated_at" goqu:"skipupdate"`
	Description string    `db:"description" grid:"search" alias:"desc"`
}

type EmbeddedStruct struct {
	SimpleStruct
	ExtraField string `db:"extra"`
}

type NestedStruct struct {
	ID     int `db:"id"`
	Nested struct {
		Field1 string `db:"field1"`
		Field2 int    `db:"field2"`
	}
}

type ReservedTypeStruct struct {
	ID        int       `db:"id"`
	Timestamp time.Time `db:"timestamp"`
	Custom    string    `db:"custom"`
}

type DuplicateFieldStruct struct {
	Field1 string `db:"name"`
	Field2 string `db:"name"` // Duplicate db name
}

func TestGetStructMeta_Simple(t *testing.T) {
	meta, err := GetStructMeta(reflect.TypeOf(SimpleStruct{}))
	require.NoError(t, err)
	require.Len(t, meta, 2)

	// Check ID field
	idField := findFieldByName(meta, "ID")
	require.NotNil(t, idField)
	assert.Equal(t, "ID", idField.Name)
	assert.Equal(t, "id", idField.DbName)
	assert.Equal(t, "ID", idField.Alias) // Default to field name
	assert.Equal(t, "int", idField.TypeName)
	assert.False(t, idField.Auto)
	assert.False(t, idField.Sortable)
	assert.False(t, idField.Filterable)
	assert.False(t, idField.Searchable)

	// Check Name field
	nameField := findFieldByName(meta, "Name")
	require.NotNil(t, nameField)
	assert.Equal(t, "Name", nameField.Name)
	assert.Equal(t, "name", nameField.DbName)
	assert.Equal(t, "user_name", nameField.Alias) // From json tag
	assert.Equal(t, "string", nameField.TypeName)
}

func TestGetStructMeta_ComplexTags(t *testing.T) {
	meta, err := GetStructMeta(reflect.TypeOf(StructWithTags{}))
	require.NoError(t, err)
	require.Len(t, meta, 8)

	// Test auto field
	idField := findFieldByName(meta, "ID")
	require.NotNil(t, idField)
	assert.True(t, idField.Auto)

	// Test grid tags
	nameField := findFieldByName(meta, "Name")
	require.NotNil(t, nameField)
	assert.True(t, nameField.Sortable)
	assert.True(t, nameField.Searchable)
	assert.False(t, nameField.Filterable)
	assert.Equal(t, "full_name", nameField.Alias)

	emailField := findFieldByName(meta, "Email")
	require.NotNil(t, emailField)
	assert.True(t, emailField.Filterable)
	assert.False(t, emailField.Sortable)
	assert.False(t, emailField.Searchable)

	// Test goqu tags
	ageField := findFieldByName(meta, "Age")
	require.NotNil(t, ageField)
	assert.True(t, ageField.OmitEmpty)
	assert.False(t, ageField.OmitNil)

	scoreField := findFieldByName(meta, "Score")
	require.NotNil(t, scoreField)
	assert.True(t, scoreField.OmitNil)
	assert.False(t, scoreField.OmitEmpty)
	assert.Equal(t, "*float64", scoreField.TypeName)

	// Test auto detection from goqu tags
	updatedAtField := findFieldByName(meta, "UpdatedAt")
	require.NotNil(t, updatedAtField)
	assert.True(t, updatedAtField.Auto)

	// Test alias tag
	descField := findFieldByName(meta, "Description")
	require.NotNil(t, descField)
	assert.Equal(t, "desc", descField.Alias)
}

func TestGetStructMeta_EmbeddedStruct(t *testing.T) {
	meta, err := GetStructMeta(reflect.TypeOf(EmbeddedStruct{}))
	require.NoError(t, err)
	require.Len(t, meta, 3) // 2 from SimpleStruct + 1 ExtraField

	// Verify embedded fields are included
	idField := findFieldByName(meta, "ID")
	require.NotNil(t, idField)
	assert.Equal(t, "id", idField.DbName)

	nameField := findFieldByName(meta, "Name")
	require.NotNil(t, nameField)
	assert.Equal(t, "name", nameField.DbName)

	extraField := findFieldByName(meta, "ExtraField")
	require.NotNil(t, extraField)
	assert.Equal(t, "extra", extraField.DbName)
}

func TestGetStructMeta_NestedStruct(t *testing.T) {
	meta, err := GetStructMeta(reflect.TypeOf(NestedStruct{}))
	require.NoError(t, err)
	require.Len(t, meta, 2) // ID + Nested field (not flattened since it's named)

	// Check that nested struct is treated as a single field (not flattened)
	idField := findFieldByDbName(meta, "id")
	require.NotNil(t, idField)

	nestedField := findFieldByDbName(meta, "nested")
	require.NotNil(t, nestedField)
	assert.Equal(t, "nested", nestedField.DbName)
}

func TestGetStructMeta_ReservedTypes(t *testing.T) {
	// Ensure time.Time is reserved
	assert.True(t, IsReservedType("time.Time"))

	meta, err := GetStructMeta(reflect.TypeOf(ReservedTypeStruct{}))
	require.NoError(t, err)
	require.Len(t, meta, 3)

	// time.Time should not be recursively parsed
	timestampField := findFieldByName(meta, "Timestamp")
	require.NotNil(t, timestampField)
	assert.Equal(t, "time.Time", timestampField.TypeName)
}

func TestGetStructMeta_Errors(t *testing.T) {
	// Test with nil pointer
	var nilPtr *SimpleStruct
	_, err := scanStruct(nilPtr)
	assert.ErrorIs(t, err, ErrNilPointer)

	// Test with non-struct
	_, err = scanStruct("not a struct")
	assert.ErrorIs(t, err, ErrInvalidStruct)

	// Test with duplicate field names
	_, err = GetStructMeta(reflect.TypeOf(DuplicateFieldStruct{}))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate field name")
}

func TestGetStructMeta_Caching(t *testing.T) {
	// Clear cache first
	fieldCache = sync.Map{}

	// First call should scan
	meta1, err := GetStructMeta(reflect.TypeOf(SimpleStruct{}))
	require.NoError(t, err)

	// Second call should use cache
	meta2, err := GetStructMeta(reflect.TypeOf(SimpleStruct{}))
	require.NoError(t, err)

	// Should return same data
	assert.Equal(t, meta1, meta2)
}

func TestGetStructMeta_ConcurrentAccess(t *testing.T) {
	// Clear cache
	fieldCache = sync.Map{}

	const numGoroutines = 100
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	errors := make(chan error, numGoroutines)
	results := make(chan []Metadata, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			meta, err := GetStructMeta(reflect.TypeOf(StructWithTags{}))
			if err != nil {
				errors <- err
			} else {
				results <- meta
			}
		}()
	}

	wg.Wait()
	close(errors)
	close(results)

	// Check no errors
	for err := range errors {
		t.Errorf("Unexpected error: %v", err)
	}

	// Verify all results are identical
	var firstResult []Metadata
	for result := range results {
		if firstResult == nil {
			firstResult = result
		} else {
			assert.Equal(t, firstResult, result)
		}
	}
}

func TestGetStructMeta_DbOptions(t *testing.T) {
	type OptionsStruct struct {
		Field1 string `db:"field1,option1,option2"`
		Field2 string `grid:"sort,custom1,custom2"`
		Field3 string `goqu:"skipinsert,custom3"`
	}

	meta, err := GetStructMeta(reflect.TypeOf(OptionsStruct{}))
	require.NoError(t, err)

	field1 := findFieldByName(meta, "Field1")
	require.NotNil(t, field1)
	assert.Contains(t, field1.DbOptions, "option1")
	assert.Contains(t, field1.DbOptions, "option2")

	field2 := findFieldByName(meta, "Field2")
	require.NotNil(t, field2)
	assert.True(t, field2.Sortable)
	assert.Contains(t, field2.DbOptions, "custom1")
	assert.Contains(t, field2.DbOptions, "custom2")

	field3 := findFieldByName(meta, "Field3")
	require.NotNil(t, field3)
	assert.True(t, field3.Auto) // skipinsert sets Auto to true
	assert.Contains(t, field3.DbOptions, "custom3")
}

func TestGetStructMeta_AllTagCombinations(t *testing.T) {
	type AllTagsStruct struct {
		Field1 string `db:"f1" ch:"cf1" auto:"true" grid:"sort,search,filter,auto" goqu:"skipupdate,omitnil,omitempty" json:"field_1" xml:"Field1" alias:"f_1"`
	}

	meta, err := GetStructMeta(reflect.TypeOf(AllTagsStruct{}))
	require.NoError(t, err)
	require.Len(t, meta, 1)

	field := meta[0]
	assert.Equal(t, "Field1", field.Name)
	assert.Equal(t, "f1", field.DbName)
	assert.Equal(t, "f_1", field.Alias) // alias tag takes precedence
	assert.True(t, field.Auto)
	assert.True(t, field.Sortable)
	assert.True(t, field.Searchable)
	assert.True(t, field.Filterable)
	assert.True(t, field.OmitNil)
	assert.True(t, field.OmitEmpty)
}

// Helper functions
func findFieldByName(meta []Metadata, name string) *Metadata {
	for i := range meta {
		if meta[i].Name == name {
			return &meta[i]
		}
	}
	return nil
}

func findFieldByDbName(meta []Metadata, dbName string) *Metadata {
	for i := range meta {
		if meta[i].DbName == dbName {
			return &meta[i]
		}
	}
	return nil
}

func BenchmarkGetStructMeta_Simple(b *testing.B) {
	for i := 0; i < b.N; i++ {
		GetStructMeta(reflect.TypeOf(SimpleStruct{}))
	}
}

func BenchmarkGetStructMeta_Complex(b *testing.B) {
	for i := 0; i < b.N; i++ {
		GetStructMeta(reflect.TypeOf(StructWithTags{}))
	}
}

func BenchmarkGetStructMeta_Cached(b *testing.B) {
	// Ensure it's cached first
	GetStructMeta(reflect.TypeOf(StructWithTags{}))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		GetStructMeta(reflect.TypeOf(StructWithTags{}))
	}
}
