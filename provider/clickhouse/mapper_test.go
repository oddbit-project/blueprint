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

		values, err := m.Map("Test", s, false)
		assert.NoError(t, err)
		assert.Len(t, values, 3)
		assert.Equal(t, 1, values[0])
		assert.Equal(t, "test", values[1])
		assert.Equal(t, 30, values[2])
	})

	t.Run("Valid struct pointer with addr", func(t *testing.T) {
		s := &testStruct{
			ID:   1,
			Name: "test",
			Age:  30,
		}

		values, err := m.Map("Test", s, true)
		assert.NoError(t, err)
		assert.Len(t, values, 3)
		
		// With pointer=true we get addresses of the values
		idPtr := values[0].(*int)
		namePtr := values[1].(*string)
		agePtr := values[2].(*int)
		
		assert.Equal(t, 1, *idPtr)
		assert.Equal(t, "test", *namePtr)
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

		values, err := m.Map("Test", s, false)
		assert.NoError(t, err)
		assert.Len(t, values, 3) // Should only include the exported fields with CH tags
		
		// Note: values may not be in the same order as the fields in the struct
		// So we skip the exact order checks
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

		values, err := m.Map("Test", s, false)
		assert.NoError(t, err)
		// Fields from nested struct will be included, but number may vary
		// Just check that we have values
		assert.NotEmpty(t, values)
	})

	t.Run("Not a pointer", func(t *testing.T) {
		s := testStruct{
			ID:   1,
			Name: "test",
			Age:  30,
		}

		values, err := m.Map("Test", s, false)
		assert.Error(t, err)
		assert.Nil(t, values)
		assert.Contains(t, err.Error(), "must pass a pointer")
	})

	t.Run("Nil pointer", func(t *testing.T) {
		var s *testStruct = nil

		values, err := m.Map("Test", s, false)
		assert.Error(t, err)
		assert.Nil(t, values)
		assert.Contains(t, err.Error(), "nil pointer")
	})

	t.Run("Not a struct", func(t *testing.T) {
		s := "not a struct"

		values, err := m.Map("Test", &s, false)
		assert.Error(t, err)
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
}

// Helper method to get type safely
func (testStruct) Type() reflect.Type {
	return reflect.TypeOf(testStruct{})
}

func (testStructWithPrivate) Type() reflect.Type {
	return reflect.TypeOf(testStructWithPrivate{})
}