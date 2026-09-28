package console

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultColors(t *testing.T) {
	tests := []struct {
		name     string
		value    int
		expected int
	}{
		{"FgDefault", FgDefault, 39},
		{"BgDefault", BgDefault, 49},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.value)
		})
	}
	assert.Equal(t, "\033[49m", Color(BgDefault))
}
