package jsoncol

import (
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type settings struct {
	Theme string   `json:"theme"`
	Tags  []string `json:"tags,omitempty"`
}

// compile-time interface checks
var (
	_ driver.Valuer    = JSON[settings]{}
	_ sql.Scanner      = (*JSON[settings])(nil)
	_ json.Marshaler   = JSON[settings]{}
	_ json.Unmarshaler = (*JSON[settings])(nil)
)

func TestValue(t *testing.T) {
	tests := []struct {
		name string
		in   driver.Valuer
		want driver.Value
	}{
		{"struct", JSON[settings]{V: settings{Theme: "dark", Tags: []string{"a"}}}, `{"theme":"dark","tags":["a"]}`},
		{"zero struct", JSON[settings]{}, `{"theme":""}`},
		{"map", JSON[map[string]int]{V: map[string]int{"a": 1}}, `{"a":1}`},
		{"nil map is JSON null", JSON[map[string]int]{}, `null`},
		{"nil slice is JSON null", JSON[[]int]{}, `null`},
		{"empty slice", JSON[[]int]{V: []int{}}, `[]`},
		{"nil pointer is JSON null", JSON[*settings]{}, `null`},
		{"scalar", JSON[int]{V: 7}, `7`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.in.Value()
			require.NoError(t, err)
			assert.IsType(t, "", got, "Value must return a string (TEXT/jsonb-friendly), not []byte")
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestValue_MarshalError(t *testing.T) {
	_, err := JSON[chan int]{V: make(chan int)}.Value()
	require.Error(t, err)
}

func TestScan(t *testing.T) {
	tests := []struct {
		name string
		src  any
		want settings
	}{
		{"bytes", []byte(`{"theme":"dark","tags":["x","y"]}`), settings{Theme: "dark", Tags: []string{"x", "y"}}},
		{"string", `{"theme":"light"}`, settings{Theme: "light"}},
		{"SQL NULL resets to zero", nil, settings{}},
		{"JSON null resets to zero", `null`, settings{}},
		{"JSON null bytes", []byte(`null`), settings{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// start from a non-zero value: Scan must not merge into it
			j := JSON[settings]{V: settings{Theme: "old", Tags: []string{"old"}}}
			require.NoError(t, j.Scan(tt.src))
			assert.Equal(t, tt.want, j.V)
		})
	}
}

func TestScan_Errors(t *testing.T) {
	tests := []struct {
		name    string
		src     any
		wantErr error
	}{
		{"int64", int64(1), ErrScanType},
		{"bool", true, ErrScanType},
		{"invalid JSON", `{"theme":`, nil},
		{"wrong JSON type", `{"theme":1}`, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var j JSON[settings]
			err := j.Scan(tt.src)
			require.Error(t, err)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			}
		})
	}
}

func TestJSONMarshaling(t *testing.T) {
	type rec struct {
		Settings JSON[settings] `json:"settings"`
	}
	in := rec{Settings: JSON[settings]{V: settings{Theme: "dark"}}}
	b, err := json.Marshal(in)
	require.NoError(t, err)
	assert.JSONEq(t, `{"settings":{"theme":"dark"}}`, string(b))

	var out rec
	require.NoError(t, json.Unmarshal(b, &out))
	assert.Equal(t, in, out)
}

func TestValueScanRoundTrip(t *testing.T) {
	in := JSON[map[string]any]{V: map[string]any{"n": 1.5, "s": "x", "l": []any{true, nil}}}
	v, err := in.Value()
	require.NoError(t, err)
	var out JSON[map[string]any]
	require.NoError(t, out.Scan(v))
	assert.Equal(t, in, out)
}
