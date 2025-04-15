package db

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

// TestStructForScanning is a test struct with various tag combinations
type TestStructForScanning struct {
	ID        int    `db:"id" json:"id" grid:"sort,filter"`
	Name      string `db:"name" json:"name" grid:"sort,search,filter"`
	Email     string `db:"email" json:"email" grid:"search,filter"`
	CreatedAt string `db:"created_at" alias:"createdAt" grid:"sort"`
	UpdatedAt string `db:"updated_at" alias:"updatedAt"`
	Ignored   string `db:"-"`
	Unexported string `db:"unexported"`
}

// TestEmbeddedStruct contains an anonymous embedded struct
type TestEmbeddedStruct struct {
	TestStructForScanning
	Description string `db:"description" json:"description" grid:"search"`
}

// TestStructWithPointerEmbedded contains an anonymous embedded pointer to a struct
type TestStructWithPointerEmbedded struct {
	*TestStructForScanning
	Extra string `db:"extra" json:"extra"`
}

func TestNewFieldSpec(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		wantErr  bool
		errorMsg string
	}{
		{
			name:     "valid struct pointer",
			input:    &TestStructForScanning{},
			wantErr:  false,
			errorMsg: "",
		},
		{
			name:     "nil pointer",
			input:    (*TestStructForScanning)(nil),
			wantErr:  true,
			errorMsg: ErrNilPointer.Error(),
		},
		{
			name:     "non-pointer",
			input:    TestStructForScanning{},
			wantErr:  true,
			errorMsg: ErrInvalidStructPtr.Error(),
		},
		{
			name:     "pointer to non-struct",
			input:    new(string),
			wantErr:  true,
			errorMsg: ErrInvalidStruct.Error(),
		},
		{
			name:     "embedded struct",
			input:    &TestEmbeddedStruct{},
			wantErr:  false,
			errorMsg: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec, err := NewFieldSpec(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Equal(t, tt.errorMsg, err.Error())
				// Spec is created before error is returned, so it's not nil
				// Just verify that it's in an empty state
				if spec != nil {
					assert.Empty(t, spec.fieldAlias)
					assert.Empty(t, spec.aliasField)
					assert.Empty(t, spec.sortFields)
					assert.Empty(t, spec.filterFields)
					assert.Empty(t, spec.searchFields)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, spec)
			}
		})
	}
}

func TestEmptyFieldSpec(t *testing.T) {
	spec := NewEmptyFieldSpec()
	assert.NotNil(t, spec)
	assert.Empty(t, spec.fieldAlias)
	assert.Empty(t, spec.aliasField)
	assert.Empty(t, spec.sortFields)
	assert.Empty(t, spec.filterFields)
	assert.Empty(t, spec.searchFields)
}

func TestFieldSpec_AddField(t *testing.T) {
	spec := NewEmptyFieldSpec()

	// Test adding a field with an alias
	err := spec.AddField("id", "ID", true, true, true)
	assert.NoError(t, err)
	assert.Equal(t, "ID", spec.fieldAlias["id"])
	assert.Equal(t, "id", spec.aliasField["ID"])
	assert.Contains(t, spec.searchFields, "id")
	assert.Contains(t, spec.sortFields, "id")
	assert.Contains(t, spec.filterFields, "id")

	// Test adding a field without an alias
	err = spec.AddField("name", "", false, true, false)
	assert.NoError(t, err)
	assert.Equal(t, "name", spec.fieldAlias["name"])
	assert.Equal(t, "name", spec.aliasField["name"])
	assert.NotContains(t, spec.searchFields, "name")
	assert.Contains(t, spec.sortFields, "name")
	assert.NotContains(t, spec.filterFields, "name")

	// Test duplicated field
	err = spec.AddField("id", "ID2", false, false, false)
	assert.Error(t, err)
	assert.Equal(t, ErrDuplicatedField.Error(), err.Error())

	// Test duplicated alias
	err = spec.AddField("id2", "ID", false, false, false)
	assert.Error(t, err)
	assert.Equal(t, ErrDuplicatedAlias.Error(), err.Error())
}

func TestFieldSpec_LookupAlias(t *testing.T) {
	spec := NewEmptyFieldSpec()
	_ = spec.AddField("id", "ID", false, false, false)

	// Test existing alias
	field, exists := spec.LookupAlias("ID")
	assert.True(t, exists)
	assert.Equal(t, "id", field)

	// Test non-existing alias
	field, exists = spec.LookupAlias("nonexistent")
	assert.False(t, exists)
	assert.Equal(t, "", field)
}

func TestFieldSpec_FieldAliasMap(t *testing.T) {
	spec := NewEmptyFieldSpec()
	_ = spec.AddField("id", "ID", false, false, false)
	_ = spec.AddField("name", "NAME", false, false, false)

	map1 := spec.FieldAliasMap()
	assert.Equal(t, 2, len(map1))
	assert.Equal(t, "ID", map1["id"])
	assert.Equal(t, "NAME", map1["name"])

	// Ensure the returned map is a copy
	map1["test"] = "TEST"
	map2 := spec.FieldAliasMap()
	assert.Equal(t, 2, len(map2))
	assert.NotContains(t, map2, "test")
}

func TestFieldSpec_AccessorMethods(t *testing.T) {
	spec := NewEmptyFieldSpec()
	_ = spec.AddField("id", "ID", false, true, true)
	_ = spec.AddField("name", "NAME", true, true, false)
	_ = spec.AddField("email", "EMAIL", true, false, true)

	// Test SortFields
	sortFields := spec.SortFields()
	assert.Equal(t, 2, len(sortFields))
	assert.Contains(t, sortFields, "id")
	assert.Contains(t, sortFields, "name")

	// Test FilterFields
	filterFields := spec.FilterFields()
	assert.Equal(t, 2, len(filterFields))
	assert.Contains(t, filterFields, "id")
	assert.Contains(t, filterFields, "email")

	// Test SearchFields
	searchFields := spec.SearchFields()
	assert.Equal(t, 2, len(searchFields))
	assert.Contains(t, searchFields, "name")
	assert.Contains(t, searchFields, "email")
}

func TestFieldSpec_ScanStruct(t *testing.T) {
	testStruct := &TestStructForScanning{}
	spec, err := NewFieldSpec(testStruct)
	assert.NoError(t, err)

	// Check field mapping
	assert.Equal(t, "id", spec.aliasField["id"])
	assert.Equal(t, "name", spec.aliasField["name"])
	assert.Equal(t, "email", spec.aliasField["email"])
	assert.Equal(t, "created_at", spec.aliasField["createdAt"])
	assert.Equal(t, "updated_at", spec.aliasField["updatedAt"])

	// Check that the ignored field is not present
	_, exists := spec.aliasField["Ignored"]
	assert.False(t, exists)
	_, exists = spec.fieldAlias["Ignored"]
	assert.False(t, exists)

	// Check sort fields
	sortFields := spec.SortFields()
	assert.Equal(t, 3, len(sortFields))
	assert.Contains(t, sortFields, "id")
	assert.Contains(t, sortFields, "name")
	assert.Contains(t, sortFields, "created_at")

	// Check filter fields
	filterFields := spec.FilterFields()
	assert.Equal(t, 3, len(filterFields))
	assert.Contains(t, filterFields, "id")
	assert.Contains(t, filterFields, "name")
	assert.Contains(t, filterFields, "email")

	// Check search fields
	searchFields := spec.SearchFields()
	assert.Equal(t, 2, len(searchFields))
	assert.Contains(t, searchFields, "name")
	assert.Contains(t, searchFields, "email")
}

func TestFieldSpec_ScanEmbeddedStruct(t *testing.T) {
	testStruct := &TestEmbeddedStruct{}
	spec, err := NewFieldSpec(testStruct)
	assert.NoError(t, err)

	// Check that fields from both the embedded struct and the parent struct are present
	assert.Equal(t, "id", spec.aliasField["id"])
	assert.Equal(t, "name", spec.aliasField["name"])
	assert.Equal(t, "description", spec.aliasField["description"])

	// Check search fields (should include fields from both structs)
	searchFields := spec.SearchFields()
	assert.Equal(t, 3, len(searchFields))
	assert.Contains(t, searchFields, "name")
	assert.Contains(t, searchFields, "email")
	assert.Contains(t, searchFields, "description")
}

func TestFieldSpec_ScanEmbeddedPointerStruct(t *testing.T) {
	base := TestStructForScanning{}
	testStruct := &TestStructWithPointerEmbedded{
		TestStructForScanning: &base,
		Extra:                 "test",
	}
	
	spec, err := NewFieldSpec(testStruct)
	assert.NoError(t, err)

	// Should only have the Extra field since the embedded pointer field is skipped
	assert.Equal(t, 1, len(spec.fieldAlias))
	assert.Equal(t, "extra", spec.aliasField["extra"])
}