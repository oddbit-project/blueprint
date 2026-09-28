package runtime

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

type tagSample struct {
	Plain  int `db:"id,auto" opt:"id"`
	Spaced int `db:"id, auto" opt:" id , omitempty "`
	None   int
}

func sampleField(t *testing.T, name string) reflect.StructField {
	t.Helper()
	f, ok := reflect.TypeOf(tagSample{}).FieldByName(name)
	assert.True(t, ok)
	return f
}

func TestParseTag_TrimsParts(t *testing.T) {
	tests := []struct {
		name     string
		field    string
		tag      string
		expected []string
	}{
		{"plain", "Plain", "db", []string{"id", "auto"}},
		{"spaced", "Spaced", "db", []string{"id", "auto"}},
		{"spaced both sides", "Spaced", "opt", []string{"id", "omitempty"}},
		{"missing tag", "None", "db", []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := sampleField(t, tt.field)
			assert.Equal(t, tt.expected, ParseTag(f, tt.tag))
			assert.Equal(t, tt.expected, ParseTagList(f, []string{"missing", tt.tag}))
			if len(tt.expected) > 0 {
				assert.Equal(t, tt.expected, MustParseTag(f, tt.tag, []string{"default"}))
			} else {
				assert.Equal(t, []string{"default"}, MustParseTag(f, tt.tag, []string{"default"}))
			}
		})
	}
}
