package field

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReservedTypes_Initial(t *testing.T) {
	// Test initial reserved types
	types := GetReservedTypes()
	assert.Contains(t, types, "time.Time", "time.Time should be in initial reserved types")
}

func TestAddReservedType(t *testing.T) {
	// Get initial count
	initialTypes := GetReservedTypes()
	initialCount := len(initialTypes)

	// Add new type
	AddReservedType("custom.Type")
	
	// Verify it was added
	types := GetReservedTypes()
	assert.Len(t, types, initialCount+1)
	assert.Contains(t, types, "custom.Type")
	
	// Add duplicate - should not increase count
	AddReservedType("custom.Type")
	types = GetReservedTypes()
	assert.Len(t, types, initialCount+1)
}

func TestIsReservedType(t *testing.T) {
	// Test default reserved type
	assert.True(t, IsReservedType("time.Time"))
	
	// Test non-reserved type
	assert.False(t, IsReservedType("string"))
	assert.False(t, IsReservedType("int"))
	
	// Add custom type and test
	AddReservedType("myapp.CustomTime")
	assert.True(t, IsReservedType("myapp.CustomTime"))
}

func TestGetReservedTypes_ReturnsCopy(t *testing.T) {
	// Get types
	types1 := GetReservedTypes()
	types2 := GetReservedTypes()
	
	// Verify they are different slices (copies)
	require.NotSame(t, &types1, &types2)
	
	// Modify one slice
	if len(types1) > 0 {
		types1[0] = "modified"
		
		// Verify original is unchanged
		types3 := GetReservedTypes()
		assert.NotEqual(t, types1[0], types3[0])
	}
}

func TestReservedTypes_Concurrent(t *testing.T) {
	const numGoroutines = 100
	const numOperations = 100
	
	var wg sync.WaitGroup
	wg.Add(numGoroutines * 3)
	
	// Concurrent adds
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				AddReservedType(string(rune('A' + id%26)))
			}
		}(i)
	}
	
	// Concurrent reads
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				GetReservedTypes()
			}
		}()
	}
	
	// Concurrent checks
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				IsReservedType(string(rune('A' + id%26)))
			}
		}(i)
	}
	
	wg.Wait()
	
	// Verify data integrity
	types := GetReservedTypes()
	assert.NotEmpty(t, types)
	
	// Check for duplicates
	seen := make(map[string]bool)
	for _, typ := range types {
		assert.False(t, seen[typ], "Found duplicate type: %s", typ)
		seen[typ] = true
	}
}

func TestAddReservedType_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		typeStr  string
		expected bool
	}{
		{"empty string", "", true},
		{"whitespace", "  ", true},
		{"special chars", "type.with-special_chars$", true},
		{"unicode", "类型.时间", true},
		{"very long name", "com.example.very.long.package.name.with.many.dots.CustomType", true},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := len(GetReservedTypes())
			AddReservedType(tt.typeStr)
			after := len(GetReservedTypes())
			
			if tt.expected {
				assert.Equal(t, before+1, after, "Type should have been added")
				assert.True(t, IsReservedType(tt.typeStr))
			}
		})
	}
}

func BenchmarkAddReservedType(b *testing.B) {
	for i := 0; i < b.N; i++ {
		AddReservedType("benchmark.Type")
	}
}

func BenchmarkIsReservedType(b *testing.B) {
	// Add some types first
	for i := 0; i < 10; i++ {
		AddReservedType(string(rune('A' + i)))
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		IsReservedType("time.Time")
	}
}

func BenchmarkGetReservedTypes(b *testing.B) {
	// Add some types first
	for i := 0; i < 10; i++ {
		AddReservedType(string(rune('A' + i)))
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		GetReservedTypes()
	}
}