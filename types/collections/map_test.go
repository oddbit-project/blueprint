package collections

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMap_BasicOperations(t *testing.T) {
	t.Run("string to int map", func(t *testing.T) {
		m := NewMap[string, int]()

		// Test Add and Get
		m.Add("one", 1)
		m.Add("two", 2)
		m.Add("three", 3)

		val, err := m.Get("two")
		require.NoError(t, err)
		assert.Equal(t, 2, val)

		// Test Contains
		assert.True(t, m.Contains("one"))
		assert.False(t, m.Contains("four"))

		// Test Len
		assert.Equal(t, 3, m.Len())

		// Test Delete
		m.Delete("two")
		assert.False(t, m.Contains("two"))
		assert.Equal(t, 2, m.Len())

		// Test GetKeys
		keys := m.GetKeys()
		assert.Len(t, keys, 2)
		assert.Contains(t, keys, "one")
		assert.Contains(t, keys, "three")
	})

	t.Run("int to string map", func(t *testing.T) {
		m := NewMap[int, string]()

		m.Add(1, "one")
		m.Add(2, "two")

		val, err := m.Get(1)
		require.NoError(t, err)
		assert.Equal(t, "one", val)

		assert.True(t, m.Contains(2))
		assert.False(t, m.Contains(3))
	})
}

func TestMap_ErrorCases(t *testing.T) {
	m := NewMap[string, int]()

	// Test Get with non-existent key
	_, err := m.Get("missing")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot find item with key missing")

	// Test MustGet with non-existent key
	val := m.MustGet("missing")
	assert.Equal(t, 0, val) // Should return zero value
}

func TestMap_Purge(t *testing.T) {
	m := NewMap[string, string]()

	m.Add("key1", "value1")
	m.Add("key2", "value2")
	m.Add("key3", "value3")

	assert.Equal(t, 3, m.Len())

	m.Purge()

	assert.Equal(t, 0, m.Len())
	assert.False(t, m.Contains("key1"))
	assert.False(t, m.Contains("key2"))
	assert.False(t, m.Contains("key3"))
}

func TestMap_ConcurrentOperations(t *testing.T) {
	m := NewMap[int, string]()
	const numGoroutines = 100
	const numOperations = 100

	var wg sync.WaitGroup
	wg.Add(numGoroutines * 4) // 4 types of operations

	// Concurrent writes
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				key := id*numOperations + j
				m.Add(key, fmt.Sprintf("value-%d-%d", id, j))
			}
		}(i)
	}

	// Concurrent reads
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				m.GetKeys()
			}
		}()
	}

	// Concurrent contains checks
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				key := id*numOperations + j
				m.Contains(key)
			}
		}(i)
	}

	// Concurrent deletes
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations/2; j++ {
				key := id*numOperations + j
				m.Delete(key)
			}
		}(i)
	}

	wg.Wait()

	// Verify data integrity
	assert.True(t, m.Len() > 0)
	assert.True(t, m.Len() <= numGoroutines*numOperations)
}

func TestMap_TypeAliases(t *testing.T) {
	t.Run("StringListMap", func(t *testing.T) {
		m := NewStringListMap()

		m.Add("fruits", []string{"apple", "banana", "orange"})
		m.Add("vegetables", []string{"carrot", "lettuce"})

		fruits, err := m.Get("fruits")
		require.NoError(t, err)
		assert.Len(t, fruits, 3)
		assert.Contains(t, fruits, "apple")

		// Test MustGet
		veggies := m.MustGet("vegetables")
		assert.Len(t, veggies, 2)

		// Test MustGet with missing key
		missing := m.MustGet("missing")
		assert.Nil(t, missing)
		assert.Len(t, missing, 0)
	})

	t.Run("StringMap", func(t *testing.T) {
		m := NewStringMap()

		m.Add("name", "John")
		m.Add("city", "New York")

		name, err := m.Get("name")
		require.NoError(t, err)
		assert.Equal(t, "John", name)

		assert.True(t, m.Contains("city"))
		assert.Equal(t, 2, m.Len())
	})

	t.Run("IntMap", func(t *testing.T) {
		m := NewIntMap()

		m.Add(1, "first")
		m.Add(2, 42)
		m.Add(3, []int{1, 2, 3})

		val1, err := m.Get(1)
		require.NoError(t, err)
		assert.Equal(t, "first", val1)

		val2, err := m.Get(2)
		require.NoError(t, err)
		assert.Equal(t, 42, val2)

		val3, err := m.Get(3)
		require.NoError(t, err)
		slice, ok := val3.([]int)
		require.True(t, ok)
		assert.Len(t, slice, 3)
	})
}

func TestMap_ComplexTypes(t *testing.T) {
	type Person struct {
		Name string
		Age  int
	}

	t.Run("struct values", func(t *testing.T) {
		m := NewMap[string, Person]()

		m.Add("john", Person{Name: "John Doe", Age: 30})
		m.Add("jane", Person{Name: "Jane Smith", Age: 25})

		person, err := m.Get("john")
		require.NoError(t, err)
		assert.Equal(t, "John Doe", person.Name)
		assert.Equal(t, 30, person.Age)

		// Test zero value return on error
		missing, err := m.Get("missing")
		assert.Error(t, err)
		assert.Equal(t, Person{}, missing)
	})

	t.Run("map of maps", func(t *testing.T) {
		m := NewMap[string, map[string]int]()

		scores := map[string]int{
			"math":    95,
			"science": 87,
			"english": 92,
		}

		m.Add("student1", scores)

		retrieved, err := m.Get("student1")
		require.NoError(t, err)
		assert.Equal(t, 95, retrieved["math"])
		assert.Equal(t, 87, retrieved["science"])
	})
}

func TestMap_EdgeCases(t *testing.T) {
	t.Run("empty map operations", func(t *testing.T) {
		m := NewMap[string, int]()

		// Operations on empty map
		assert.Equal(t, 0, m.Len())
		assert.Empty(t, m.GetKeys())
		assert.False(t, m.Contains("any"))

		_, err := m.Get("any")
		assert.Error(t, err)

		// Delete on empty map shouldn't panic
		m.Delete("any")

		// Purge on empty map shouldn't panic
		m.Purge()
	})

	t.Run("overwrite existing key", func(t *testing.T) {
		m := NewMap[string, int]()

		m.Add("key", 100)
		assert.Equal(t, 100, m.MustGet("key"))

		m.Add("key", 200)
		assert.Equal(t, 200, m.MustGet("key"))
		assert.Equal(t, 1, m.Len())
	})

	t.Run("nil and zero values", func(t *testing.T) {
		m := NewMap[string, *int]()

		// Add nil value
		m.Add("nil", nil)
		assert.True(t, m.Contains("nil"))

		val, err := m.Get("nil")
		require.NoError(t, err)
		assert.Nil(t, val)

		// Zero value for non-existent key
		missing, err := m.Get("missing")
		assert.Error(t, err)
		assert.Nil(t, missing)
	})
}

func BenchmarkMap_Add(b *testing.B) {
	m := NewMap[int, string]()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		m.Add(i, fmt.Sprintf("value-%d", i))
	}
}

func BenchmarkMap_Get(b *testing.B) {
	m := NewMap[int, string]()
	for i := 0; i < 1000; i++ {
		m.Add(i, fmt.Sprintf("value-%d", i))
	}
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		m.Get(i % 1000)
	}
}

func BenchmarkMap_Contains(b *testing.B) {
	m := NewMap[int, string]()
	for i := 0; i < 1000; i++ {
		m.Add(i, fmt.Sprintf("value-%d", i))
	}
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		m.Contains(i % 1000)
	}
}

func BenchmarkMap_ConcurrentReadWrite(b *testing.B) {
	m := NewMap[int, string]()
	for i := 0; i < 100; i++ {
		m.Add(i, fmt.Sprintf("value-%d", i))
	}
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			if i%2 == 0 {
				m.Add(i%100, fmt.Sprintf("value-%d", i))
			} else {
				m.Get(i % 100)
			}
			i++
		}
	})
}
