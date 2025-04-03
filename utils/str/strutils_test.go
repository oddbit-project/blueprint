package str

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestContains(t *testing.T) {
	// Test cases
	testCases := []struct {
		name     string
		needle   string
		haystack []string
		expected int
	}{
		{
			name:     "Exact match first element",
			needle:   "apple",
			haystack: []string{"apple", "banana", "orange"},
			expected: 0,
		},
		{
			name:     "Exact match middle element",
			needle:   "banana",
			haystack: []string{"apple", "banana", "orange"},
			expected: 1,
		},
		{
			name:     "Exact match last element",
			needle:   "orange",
			haystack: []string{"apple", "banana", "orange"},
			expected: 2,
		},
		{
			name:     "No match",
			needle:   "grape",
			haystack: []string{"apple", "banana", "orange"},
			expected: -1,
		},
		{
			name:     "Empty needle",
			needle:   "",
			haystack: []string{"apple", "banana", "orange"},
			expected: -1,
		},
		{
			name:     "Empty haystack",
			needle:   "apple",
			haystack: []string{},
			expected: -1,
		},
		{
			name:     "Nil haystack",
			needle:   "apple",
			haystack: nil,
			expected: -1,
		},
		{
			name:     "Case sensitive match",
			needle:   "Apple",
			haystack: []string{"apple", "banana", "orange"},
			expected: -1,
		},
		{
			name:     "Duplicate elements - returns first occurrence",
			needle:   "apple",
			haystack: []string{"orange", "apple", "banana", "apple"},
			expected: 1,
		},
	}

	// Run test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := Contains(tc.needle, tc.haystack)
			assert.Equal(t, tc.expected, result)
		})
	}
}