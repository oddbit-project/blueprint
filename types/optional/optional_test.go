package optional

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type patchBody struct {
	Name  Optional[string] `json:"name,omitzero"`
	Age   Optional[int]    `json:"age,omitzero"`
	Email Optional[string] `json:"email"` // no omitzero: None marshals as null
}

func TestStates(t *testing.T) {
	tests := []struct {
		name   string
		o      Optional[int]
		isSet  bool
		isNull bool
		isZero bool
		wantV  int
		wantOK bool
	}{
		{"zero value is None", Optional[int]{}, false, false, true, 0, false},
		{"None", None[int](), false, false, true, 0, false},
		{"Null", Null[int](), true, true, false, 0, false},
		{"Some", Some(42), true, false, false, 42, true},
		{"Some zero value", Some(0), true, false, false, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.isSet, tt.o.IsSet())
			assert.Equal(t, tt.isNull, tt.o.IsNull())
			assert.Equal(t, tt.isZero, tt.o.IsZero())
			v, ok := tt.o.Get()
			assert.Equal(t, tt.wantV, v)
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.wantOK, tt.o.IsSome())
		})
	}
}

func TestMarshalJSON(t *testing.T) {
	tests := []struct {
		name string
		in   patchBody
		want string
	}{
		{"all None", patchBody{}, `{"email":null}`},
		{"Null is written", patchBody{Name: Null[string](), Email: Null[string]()}, `{"name":null,"email":null}`},
		{"Some is written", patchBody{Name: Some("alice"), Age: Some(0), Email: Some("a@x")}, `{"name":"alice","age":0,"email":"a@x"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := json.Marshal(tt.in)
			require.NoError(t, err)
			assert.JSONEq(t, tt.want, string(b))
		})
	}
}

func TestUnmarshalJSON(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want patchBody
	}{
		{"absent fields stay None", `{}`, patchBody{}},
		{"null becomes Null", `{"name":null,"age":null}`, patchBody{Name: Null[string](), Age: Null[int]()}},
		{"value becomes Some", `{"name":"bob","age":0}`, patchBody{Name: Some("bob"), Age: Some(0)}},
		{"mixed", `{"name":"bob","email":null}`, patchBody{Name: Some("bob"), Email: Null[string]()}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got patchBody
			require.NoError(t, json.Unmarshal([]byte(tt.in), &got))
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestUnmarshalJSON_Reuse(t *testing.T) {
	o := Some(5)
	require.NoError(t, json.Unmarshal([]byte(`null`), &o))
	assert.True(t, o.IsNull())
	v, ok := o.Get()
	assert.False(t, ok)
	assert.Equal(t, 0, v, "Null must not keep a previous value")
}

func TestUnmarshalJSON_TypeError(t *testing.T) {
	var got patchBody
	err := json.Unmarshal([]byte(`{"age":"x"}`), &got)
	require.Error(t, err)
	assert.False(t, got.Age.IsSet())
}

func TestRoundTrip(t *testing.T) {
	for _, in := range []patchBody{
		{},
		{Name: Null[string](), Age: Null[int]()},
		{Name: Some("carol"), Age: Some(33)},
	} {
		b, err := json.Marshal(in)
		require.NoError(t, err)
		var out patchBody
		require.NoError(t, json.Unmarshal(b, &out))
		// Email has no omitzero, so a None Email comes back as Null.
		if !in.Email.IsSet() {
			assert.True(t, out.Email.IsNull())
			out.Email = None[string]()
		}
		assert.Equal(t, in, out)
	}
}
