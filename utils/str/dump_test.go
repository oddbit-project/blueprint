package str

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDumpJSON(t *testing.T) {
	tests := []struct {
		name     string
		src      any
		expected string
	}{
		{"scalar", 42, "42"},
		{"flat map", map[string]int{"a": 1}, "{\n \"a\": 1\n}"},
		{"nested", map[string]any{"a": []int{1}}, "{\n \"a\": [\n  1\n ]\n}"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, DumpJSON(tt.src))
		})
	}
}
