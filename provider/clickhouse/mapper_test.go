package clickhouse

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

type testStruct struct {
	ID   int    `ch:"id"`
	Name string `ch:"name"`
	Age  int    `ch:"age"`
}

type testStructWithPrivate struct {
	ID       int    `ch:"id"`
	Name     string `ch:"name"`
	Age      int    `ch:"age"`
	private  string
	internal int `ch:"-"`
}

type testNestedStruct struct {
	Base     testStruct `ch:"base"`
	Extra    string     `ch:"extra"`
	internal int        `ch:"-"`
}

func TestStructMap(t *testing.T) {
	m := &structMap{}

	t.Run("Valid struct pointer", func(t *testing.T) {
		s := &testStruct{
			ID:   1,
			Name: "test",
			Age:  30,
		}

		columns, values, err := m.Map("Test", s, false)
		assert.NoError(t, err)
		assert.Len(t, columns, 3)
		assert.Len(t, values, 3)
		
		// Verify all expected columns are present
		assert.Contains(t, columns, "id")
		assert.Contains(t, columns, "name")
		assert.Contains(t, columns, "age")
		
		// Create a map to associate columns with values
		valueMap := make(map[string]interface{})
		for i, col := range columns {
			valueMap[col] = values[i]
		}
		
		// Verify each column has the expected value
		assert.Equal(t, 1, valueMap["id"])
		assert.Equal(t, "test", valueMap["name"])
		assert.Equal(t, 30, valueMap["age"])
	})

	t.Run("Valid struct pointer with addr", func(t *testing.T) {
		s := &testStruct{
			ID:   1,
			Name: "test",
			Age:  30,
		}

		columns, values, err := m.Map("Test", s, true)
		assert.NoError(t, err)
		assert.Len(t, columns, 3)
		assert.Len(t, values, 3)
		
		// Verify all expected columns are present
		assert.Contains(t, columns, "id")
		assert.Contains(t, columns, "name")
		assert.Contains(t, columns, "age")
		
		// Create a map to associate columns with values (which are pointers in this case)
		valueMap := make(map[string]interface{})
		for i, col := range columns {
			valueMap[col] = values[i]
		}
		
		// Verify each pointer contains the expected value
		idPtr, ok := valueMap["id"].(*int)
		assert.True(t, ok, "ID should be an *int")
		assert.Equal(t, 1, *idPtr)
		
		namePtr, ok := valueMap["name"].(*string)
		assert.True(t, ok, "Name should be a *string")
		assert.Equal(t, "test", *namePtr)
		
		agePtr, ok := valueMap["age"].(*int)
		assert.True(t, ok, "Age should be an *int")
		assert.Equal(t, 30, *agePtr)
	})

	t.Run("Struct with private fields", func(t *testing.T) {
		s := &testStructWithPrivate{
			ID:       1,
			Name:     "test",
			Age:      30,
			private:  "hidden",
			internal: 42,
		}

		columns, values, err := m.Map("Test", s, false)
		assert.NoError(t, err)
		
		// Should only include the exported fields with CH tags
		assert.Len(t, columns, 3)
		assert.Len(t, values, 3)
		
		// Verify the expected columns and no unexpected ones
		assert.Contains(t, columns, "id")
		assert.Contains(t, columns, "name")
		assert.Contains(t, columns, "age")
		assert.NotContains(t, columns, "private")
		assert.NotContains(t, columns, "internal")
	})

	t.Run("Nested struct", func(t *testing.T) {
		s := &testNestedStruct{
			Base: testStruct{
				ID:   1,
				Name: "test",
				Age:  30,
			},
			Extra:    "extra value",
			internal: 42,
		}

		columns, values, err := m.Map("Test", s, false)
		assert.NoError(t, err)
		
		// Verify we got columns and values
		assert.NotEmpty(t, columns)
		assert.NotEmpty(t, values)
		assert.Equal(t, len(columns), len(values))
		
		// Check for expected fields - the current implementation doesn't expand nested structs
		assert.Contains(t, columns, "base")
		assert.Contains(t, columns, "extra")
		assert.NotContains(t, columns, "internal")
	})

	t.Run("Not a pointer", func(t *testing.T) {
		s := testStruct{
			ID:   1,
			Name: "test",
			Age:  30,
		}

		columns, values, err := m.Map("Test", s, false)
		assert.Error(t, err)
		assert.Nil(t, columns)
		assert.Nil(t, values)
		assert.Contains(t, err.Error(), "must pass a pointer")
	})

	t.Run("Nil pointer", func(t *testing.T) {
		var s *testStruct = nil

		columns, values, err := m.Map("Test", s, false)
		assert.Error(t, err)
		assert.Nil(t, columns)
		assert.Nil(t, values)
		assert.Contains(t, err.Error(), "nil pointer")
	})

	t.Run("Not a struct", func(t *testing.T) {
		s := "not a struct"

		columns, values, err := m.Map("Test", &s, false)
		assert.Error(t, err)
		assert.Nil(t, columns)
		assert.Nil(t, values)
		assert.Contains(t, err.Error(), "expects a struct")
	})
}

func TestStructIdx(t *testing.T) {
	t.Run("Basic struct", func(t *testing.T) {
		typ := (testStruct{}).Type()
		idx := structIdx(typ)
		
		assert.Len(t, idx, 3)
		assert.Contains(t, idx, "id")
		assert.Contains(t, idx, "name")
		assert.Contains(t, idx, "age")
	})

	t.Run("Struct with private and ignored fields", func(t *testing.T) {
		typ := (testStructWithPrivate{}).Type()
		idx := structIdx(typ)
		
		assert.Len(t, idx, 3)
		assert.Contains(t, idx, "id")
		assert.Contains(t, idx, "name")
		assert.Contains(t, idx, "age")
		assert.NotContains(t, idx, "private")
		assert.NotContains(t, idx, "internal")
	})
	
	t.Run("Nested struct", func(t *testing.T) {
		typ := (testNestedStruct{}).Type()
		idx := structIdx(typ)
		
		// Current implementation puts nested structs as fields themselves, not expanded
		assert.Contains(t, idx, "base")
		assert.Contains(t, idx, "extra")
		assert.NotContains(t, idx, "internal")
	})
}

// Helper method to get type safely
func (testStruct) Type() reflect.Type {
	return reflect.TypeOf(testStruct{})
}

func (testStructWithPrivate) Type() reflect.Type {
	return reflect.TypeOf(testStructWithPrivate{})
}

func (testNestedStruct) Type() reflect.Type {
	return reflect.TypeOf(testNestedStruct{})
}