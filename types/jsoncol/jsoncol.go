// Package jsoncol provides JSON[T], a generic database column type that
// stores T as JSON text: PostgreSQL json/jsonb, or SQLite TEXT.
//
//	type Account struct {
//		ID       int64                  `db:"id,auto"`
//		Settings jsoncol.JSON[Settings] `db:"settings"`
//		Extra    *jsoncol.JSON[Extra]   `db:"extra"` // nullable column
//	}
//
// JSON[T] is not intended for ClickHouse, whose native driver does not use
// driver.Valuer/sql.Scanner for its JSON type.
package jsoncol

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"

	"github.com/oddbit-project/blueprint/utils"
)

// ErrScanType is returned by JSON.Scan when the driver hands it a value
// that is not []byte, string or nil.
const ErrScanType = utils.Error("jsoncol: unsupported scan source type")

// JSON wraps a value of type T that is stored in the database as JSON.
//
// Value always marshals V, so a nil map, slice or pointer T is written as
// the JSON literal null, not as SQL NULL. For a nullable column use
// *JSON[T]: a nil *JSON[T] binds SQL NULL, and scanning SQL NULL into it
// leaves it nil.
type JSON[T any] struct {
	V T
}

// Value implements driver.Valuer. It returns the JSON encoding of V as a
// string: text is accepted by PostgreSQL json/jsonb under both the simple
// and extended protocols and is stored as TEXT (not BLOB) by SQLite.
func (j JSON[T]) Value() (driver.Value, error) {
	b, err := json.Marshal(j.V)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

// Scan implements sql.Scanner. src may be []byte or string holding JSON,
// or nil (SQL NULL). V is reset to T's zero value before decoding, so SQL
// NULL and the JSON literal null both leave V at its zero value.
func (j *JSON[T]) Scan(src any) error {
	var zero T
	j.V = zero
	var data []byte
	switch s := src.(type) {
	case nil:
		return nil
	case []byte:
		data = s
	case string:
		data = []byte(s)
	default:
		return fmt.Errorf("%w: %T", ErrScanType, src)
	}
	return json.Unmarshal(data, &j.V)
}

// MarshalJSON encodes the JSON as V alone, without a wrapping object, so a
// record holding a JSON[T] field serializes the same as one holding T.
func (j JSON[T]) MarshalJSON() ([]byte, error) {
	return json.Marshal(j.V)
}

// UnmarshalJSON decodes data into V.
func (j *JSON[T]) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &j.V)
}
