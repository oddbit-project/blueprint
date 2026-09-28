// Package optional provides Optional[T], a tri-state value for PATCH-style
// request bodies, where "field absent", "field explicitly null" and "field
// set to a value" mean different things:
//
//	type UserPatch struct {
//		Name  optional.Optional[string] `json:"name,omitzero"`
//		Phone optional.Optional[string] `json:"phone,omitzero"`
//	}
//
// Decoding {"phone": null} leaves Name as None (do not touch the column) and
// sets Phone to Null (clear the column). The zero value of Optional[T] is
// None, so an absent JSON field needs no special handling.
package optional

import (
	"bytes"
	"encoding/json"
)

type state uint8

const (
	stateNone state = iota
	stateNull
	stateSome
)

// Optional holds one of three states: None (absent), Null (explicitly
// null) or Some(v) (a value). The zero value is None.
type Optional[T any] struct {
	value T
	state state
}

// None returns an absent Optional (the zero value).
func None[T any]() Optional[T] { return Optional[T]{} }

// Null returns an Optional that is present but explicitly null.
func Null[T any]() Optional[T] { return Optional[T]{state: stateNull} }

// Some returns an Optional holding v. Some of a zero value (e.g. Some(0) or
// Some("")) is still a value, not None.
func Some[T any](v T) Optional[T] { return Optional[T]{value: v, state: stateSome} }

// IsSet reports whether the Optional is present, either as Null or as a
// value.
func (o Optional[T]) IsSet() bool { return o.state != stateNone }

// IsNull reports whether the Optional is present and explicitly null.
func (o Optional[T]) IsNull() bool { return o.state == stateNull }

// IsSome reports whether the Optional holds a value.
func (o Optional[T]) IsSome() bool { return o.state == stateSome }

// IsZero reports whether the Optional is None. encoding/json uses it to
// omit None fields tagged `json:",omitzero"`.
func (o Optional[T]) IsZero() bool { return o.state == stateNone }

// Get returns the held value and true for Some; for None and Null it
// returns T's zero value and false.
func (o Optional[T]) Get() (T, bool) {
	if o.state != stateSome {
		var zero T
		return zero, false
	}
	return o.value, true
}

// MarshalJSON encodes Some(v) as v and Null as null. None also encodes as
// null; tag the field `json:",omitzero"` to leave it out instead.
func (o Optional[T]) MarshalJSON() ([]byte, error) {
	if o.state != stateSome {
		return []byte("null"), nil
	}
	return json.Marshal(o.value)
}

var jsonNull = []byte("null")

// UnmarshalJSON decodes a JSON null as Null and any other value as Some.
// A field missing from the input is never passed to UnmarshalJSON, so it
// keeps its zero value, None. On a decoding error the Optional is left
// unchanged.
func (o *Optional[T]) UnmarshalJSON(data []byte) error {
	if bytes.Equal(bytes.TrimSpace(data), jsonNull) {
		*o = Null[T]()
		return nil
	}
	var v T
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*o = Some(v)
	return nil
}
