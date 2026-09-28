package dbx

import (
	"context"
	"database/sql"
	"encoding/json"
	"math"
	"strings"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/oddbit-project/blueprint/types/jsoncol"
	"github.com/oddbit-project/blueprint/types/optional"
	"github.com/oddbit-project/gohan"
)

type csStatus string

// csByte is a named byte type: reflect cannot convert a string to or from
// a slice of it.
type csByte byte

type CsBase struct {
	ID int64 `db:"id,auto"`
}

// csRecord covers every field kind the changeset and diff handle.
type csRecord struct {
	CsBase
	Name    string                       `db:"name"`
	Age     int                          `db:"age"`
	Small   int8                         `db:"small"`
	Count   uint32                       `db:"count"`
	Big     uint64                       `db:"big"`
	Score   float64                      `db:"score"`
	Ratio   float32                      `db:"ratio"`
	Active  bool                         `db:"active"`
	Status  csStatus                     `db:"status"`
	Nick    *string                      `db:"nick"`
	Level   *int                         `db:"level"`
	Tags    []string                     `db:"tags"`
	Data    []byte                       `db:"data"`
	Raw     []csByte                     `db:"raw"`
	Attrs   map[string]any               `db:"attrs"`
	Any     any                          `db:"any"`
	Born    time.Time                    `db:"born"`
	Seen    *time.Time                   `db:"seen"`
	Label   sql.NullString               `db:"label"`
	NullN   sql.Null[int64]              `db:"null_n"`
	UID     uuid.UUID                    `db:"uid"`
	Doc     jsoncol.JSON[map[string]any] `db:"doc"`
	Created time.Time                    `db:"created_at" auto:"true"`
	Updated time.Time                    `db:"updated_at" goqu:"skipupdate"`
	Ignored string                       `db:"-"`
}

func newCsRepo(t *testing.T) *Repository[csRecord] {
	t.Helper()
	r, err := NewRepository[csRecord](&countingQuerier{d: gohan.Postgres()}, "records")
	require.NoError(t, err)
	return r
}

func ptr[V any](v V) *V { return &v }

func TestChangesetSetRejects(t *testing.T) {
	tests := []struct {
		name    string
		col     string
		value   any
		wantErr error
	}{
		{"unknown column", "bogus", "x", ErrUnknownColumn},
		{"Go field name is not a column", "Name", "x", ErrUnknownColumn},
		{"db:\"-\" field", "ignored", "x", ErrUnknownColumn},
		{"auto via db tag (embedded)", "id", int64(1), ErrAutoColumn},
		{"auto via auto tag", "created_at", time.Now(), ErrAutoColumn},
		{"auto via goqu skipupdate", "updated_at", time.Now(), ErrAutoColumn},
		{"fractional float into int", "age", 30.7, ErrValueType},
		{"fractional float into *int", "level", 2.5, ErrValueType},
		{"NaN into int", "age", math.NaN(), ErrValueType},
		{"Inf into int", "age", math.Inf(1), ErrValueType},
		{"float beyond int64 into int", "age", 1e20, ErrValueType},
		{"int overflowing int8", "small", 300, ErrValueType},
		{"float overflowing int8", "small", float64(-129), ErrValueType},
		{"negative int into uint", "count", -1, ErrValueType},
		{"negative float into uint", "count", float64(-1), ErrValueType},
		{"negative int into uint64", "big", -1, ErrValueType},
		{"negative float into uint64", "big", float64(-1), ErrValueType},
		{"float at 2^64 into uint64", "big", float64(1<<63) * 2, ErrValueType},
		{"uint overflowing uint32", "count", uint64(1 << 40), ErrValueType},
		{"uint beyond int64 into int", "age", uint64(math.MaxUint64), ErrValueType},
		{"float64 overflowing float32", "ratio", 1e300, ErrValueType},
		{"int losing precision in float64", "score", int64(1<<53 + 1), ErrValueType},
		{"int losing precision in float32", "ratio", 1<<24 + 1, ErrValueType},
		{"string into int", "age", "30", ErrValueType},
		{"int into string (no rune conversion)", "name", 65, ErrValueType},
		{"int into named string", "status", 1, ErrValueType},
		{"string into time.Time", "born", "2024-01-01T00:00:00Z", ErrValueType},
		{"wrong slice element type", "tags", []int{1}, ErrValueType},
		{"int into *string", "nick", 5, ErrValueType},
		{"bool into int", "age", true, ErrValueType},
		{"nil into int", "age", nil, ErrValueType},
		{"nil into string", "name", nil, ErrValueType},
		{"nil into bool", "active", nil, ErrValueType},
		{"nil into time.Time", "born", nil, ErrValueType},
		{"nil pointer into int", "age", (*int)(nil), ErrValueType},
		{"string into []namedByte", "raw", "abc", ErrValueType},
		{"[]namedByte into string", "name", []csByte("abc"), ErrValueType},
		{"json.Number with fraction into int", "age", json.Number("30.5"), ErrValueType},
		{"json.Number with exponent into int", "age", json.Number("3e1"), ErrValueType},
		{"json.Number beyond int64 into int", "age", json.Number("9223372036854775808"), ErrValueType},
		{"json.Number overflowing int8", "small", json.Number("300"), ErrValueType},
		{"negative json.Number into uint", "count", json.Number("-1"), ErrValueType},
		{"json.Number beyond uint64", "big", json.Number("18446744073709551616"), ErrValueType},
		{"json.Number overflowing float64", "score", json.Number("1e400"), ErrValueType},
		{"json.Number overflowing float32", "ratio", json.Number("1e300"), ErrValueType},
		{"non-numeric json.Number into int", "age", json.Number("abc"), ErrValueType},
		{"non-numeric json.Number into float", "score", json.Number("abc"), ErrValueType},
		{"NaN json.Number into float", "score", json.Number("NaN"), ErrValueType},
		{"Inf json.Number into float", "score", json.Number("Inf"), ErrValueType},
		{"hex json.Number into float", "score", json.Number("0x1p3"), ErrValueType},
		{"json.Number into string", "name", json.Number("12"), ErrValueType},
		{"json.Number into named string", "status", json.Number("12"), ErrValueType},
		{"nil into uuid.UUID (Scanner, not NULL-style)", "uid", nil, ErrValueType},
		{"nil into jsoncol.JSON (Scanner, not NULL-style)", "doc", nil, ErrValueType},
		{"Optional passed to Set", "name", optional.Some("x"), ErrValueType},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cs := NewChangeset(newCsRepo(t))
			require.NoError(t, cs.Set("name", "before"))
			err := cs.Set(tt.col, tt.value)
			require.Error(t, err)
			assert.ErrorIs(t, err, tt.wantErr)
			assert.Equal(t, map[string]any{"name": "before"}, cs.Changes(), "a rejected Set must not change the changeset")
		})
	}
}

// TestChangesetSetErrorOmitsValue checks that an ErrValueType message names
// the column and the types but never echoes the (client-supplied) value.
func TestChangesetSetErrorOmitsValue(t *testing.T) {
	cs := NewChangeset(newCsRepo(t))
	err := cs.Set("age", "<script>secret</script>")
	require.ErrorIs(t, err, ErrValueType)
	assert.NotContains(t, err.Error(), "secret")
	assert.Contains(t, err.Error(), `"age"`)
	assert.Contains(t, err.Error(), "int")
	assert.Contains(t, err.Error(), "string")

	err = cs.Set("small", 12345)
	require.ErrorIs(t, err, ErrValueType)
	assert.NotContains(t, err.Error(), "12345")
}

func TestChangesetSetAccepts(t *testing.T) {
	born := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	tests := []struct {
		name  string
		col   string
		value any
		want  any
	}{
		{"string", "name", "bob", "bob"},
		{"empty string", "name", "", ""},
		{"integral float64 into int", "age", float64(30), 30},
		{"int64 into int", "age", int64(30), 30},
		{"negative integral float into int", "age", float64(-4), -4},
		{"int at int8 bound", "small", -128, int8(-128)},
		{"uint into uint32", "count", uint(7), uint32(7)},
		{"integral float into uint32", "count", float64(7), uint32(7)},
		{"large integral float into uint64", "big", float64(1 << 63), uint64(1 << 63)},
		{"max uint32", "count", int64(math.MaxUint32), uint32(math.MaxUint32)},
		{"int into float64", "score", 3, float64(3)},
		{"float64 into float32 (rounded)", "ratio", 0.1, float32(0.1)},
		{"bool", "active", true, true},
		{"string into named string", "status", "ok", csStatus("ok")},
		{"named string", "status", csStatus("ok"), csStatus("ok")},
		{"value into *string", "nick", "x", "x"},
		{"pointer into *string is dereferenced", "nick", ptr("x"), "x"},
		{"nil into *string", "nick", nil, nil},
		{"nil pointer into *string", "nick", (*string)(nil), nil},
		{"integral float into *int", "level", float64(2), 2},
		{"slice", "tags", []string{"a"}, []string{"a"}},
		{"nil into slice", "tags", nil, nil},
		{"string into []byte", "data", "abc", []byte("abc")},
		{"nil into []byte", "data", nil, nil},
		{"map", "attrs", map[string]any{"k": 1}, map[string]any{"k": 1}},
		{"nil into map", "attrs", nil, nil},
		{"anything into any", "any", 5, 5},
		{"nil into any", "any", nil, nil},
		{"time.Time", "born", born, born},
		{"time.Time into *time.Time", "seen", born, born},
		{"nil into *time.Time", "seen", nil, nil},
		{"nil into sql.Scanner type", "label", nil, nil},
		{"nil into sql.Null[int64]", "null_n", nil, nil},
		{"sql.Null[int64]", "null_n", sql.Null[int64]{V: 3, Valid: true}, sql.Null[int64]{V: 3, Valid: true}},
		{"uuid.UUID", "uid", uuid.Nil, uuid.Nil},
		{"jsoncol.JSON", "doc", jsoncol.JSON[map[string]any]{V: map[string]any{"a": 1}}, jsoncol.JSON[map[string]any]{V: map[string]any{"a": 1}}},
		{"json.Number into int", "age", json.Number("30"), 30},
		{"json.Number above 2^53 into int stays exact", "age", json.Number("9007199254740993"), 9007199254740993},
		{"negative json.Number into int8", "small", json.Number("-128"), int8(-128)},
		{"json.Number max uint64", "big", json.Number("18446744073709551615"), uint64(math.MaxUint64)},
		{"json.Number into *int", "level", json.Number("-5"), -5},
		{"*json.Number into int", "age", ptr(json.Number("7")), 7},
		{"json.Number into float64", "score", json.Number("2.5e1"), 25.0},
		{"json.Number into float32 (rounded)", "ratio", json.Number("0.1"), float32(0.1)},
		{"json.Number into any is kept", "any", json.Number("12"), json.Number("12")},
		{"sql.NullString", "label", sql.NullString{String: "l", Valid: true}, sql.NullString{String: "l", Valid: true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cs := NewChangeset(newCsRepo(t))
			require.NoError(t, cs.Set(tt.col, tt.value))
			got := cs.Changes()
			require.Contains(t, got, tt.col)
			assert.Equal(t, tt.want, got[tt.col])
			assert.IsType(t, tt.want, got[tt.col])
		})
	}
}

// TestChangesetJSONUseNumber applies a body decoded with UseNumber: an
// integer above 2^53 reaches the int field exactly.
func TestChangesetJSONUseNumber(t *testing.T) {
	dec := json.NewDecoder(strings.NewReader(`{"age": 9007199254740993, "score": 1.5}`))
	dec.UseNumber()
	var body map[string]any
	require.NoError(t, dec.Decode(&body))

	cs := NewChangeset(newCsRepo(t))
	for k, v := range body {
		require.NoError(t, cs.Set(k, v), k)
	}
	assert.Equal(t, map[string]any{"age": 9007199254740993, "score": 1.5}, cs.Changes())
}

func TestChangesetOverwriteAndCopy(t *testing.T) {
	cs := NewChangeset(newCsRepo(t))
	assert.Empty(t, cs.Changes())

	require.NoError(t, cs.Set("name", "a"))
	require.NoError(t, cs.Set("name", "b"))
	require.NoError(t, cs.Set("age", 1))

	got := cs.Changes()
	assert.Equal(t, map[string]any{"name": "b", "age": 1}, got)

	got["name"] = "mutated"
	delete(got, "age")
	assert.Equal(t, map[string]any{"name": "b", "age": 1}, cs.Changes(), "Changes must return a copy")
}

func TestSetOptional(t *testing.T) {
	cs := NewChangeset(newCsRepo(t))

	require.NoError(t, SetOptional(cs, "name", optional.None[string]()))
	require.NoError(t, SetOptional(cs, "nick", optional.Null[string]()))
	require.NoError(t, SetOptional(cs, "age", optional.Some(float64(30))))
	require.NoError(t, SetOptional(cs, "bogus", optional.None[string]()), "None skips validation entirely")

	assert.Equal(t, map[string]any{"nick": nil, "age": 30}, cs.Changes())

	assert.ErrorIs(t, SetOptional(cs, "age", optional.Some(30.7)), ErrValueType)
	assert.ErrorIs(t, SetOptional(cs, "age", optional.Null[int]()), ErrValueType)
	assert.ErrorIs(t, SetOptional(cs, "id", optional.Some(int64(1))), ErrAutoColumn)
	assert.ErrorIs(t, SetOptional(cs, "bogus", optional.Some("x")), ErrUnknownColumn)
}

// TestChangesetPatchFlow decodes a PATCH body, applies it through a
// changeset and checks the UPDATE UpdateFields sends.
func TestChangesetPatchFlow(t *testing.T) {
	type patch struct {
		Name optional.Optional[string] `json:"name,omitzero"`
		Nick optional.Optional[string] `json:"nick,omitzero"`
		Age  optional.Optional[int]    `json:"age,omitzero"`
	}
	var body patch
	require.NoError(t, json.Unmarshal([]byte(`{"nick":null,"age":41}`), &body))

	q, mock := newMockQuerier(t)
	r, err := NewRepository[csRecord](q, "records")
	require.NoError(t, err)

	cs := NewChangeset(r)
	require.NoError(t, SetOptional(cs, "name", body.Name))
	require.NoError(t, SetOptional(cs, "nick", body.Nick))
	require.NoError(t, SetOptional(cs, "age", body.Age))

	mock.ExpectExec(`UPDATE "records" SET "age" = $1, "nick" = $2 WHERE "id" = $3`).
		WithArgs(41, nil, int64(9)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	n, err := r.UpdateFields(context.Background(), cs.Changes(), gohan.Col("id").Eq(int64(9)))
	require.NoError(t, err)
	assert.Equal(t, int64(1), n)
	require.NoError(t, mock.ExpectationsWereMet())
}

func baseCsRecord() csRecord {
	born := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	return csRecord{
		CsBase:  CsBase{ID: 1},
		Name:    "alice",
		Age:     30,
		Small:   1,
		Count:   2,
		Score:   1.5,
		Ratio:   0.5,
		Active:  true,
		Status:  "ok",
		Nick:    ptr("al"),
		Level:   nil,
		Tags:    []string{"a", "b"},
		Data:    []byte("xy"),
		Attrs:   map[string]any{"k": 1},
		Any:     "v",
		Born:    born,
		Seen:    ptr(born),
		Label:   sql.NullString{String: "l", Valid: true},
		Created: born,
		Updated: born,
		Ignored: "i",
	}
}

func TestChangesDiff(t *testing.T) {
	born := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	tests := []struct {
		name   string
		modify func(r *csRecord)
		want   map[string]any
	}{
		{"identical", func(r *csRecord) {}, map[string]any{}},
		{"string", func(r *csRecord) { r.Name = "bob" }, map[string]any{"name": "bob"}},
		{"int", func(r *csRecord) { r.Age = 31 }, map[string]any{"age": 31}},
		{"int to zero", func(r *csRecord) { r.Age = 0 }, map[string]any{"age": 0}},
		{"int8", func(r *csRecord) { r.Small = -1 }, map[string]any{"small": int8(-1)}},
		{"uint32", func(r *csRecord) { r.Count = 3 }, map[string]any{"count": uint32(3)}},
		{"uint64", func(r *csRecord) { r.Big = 1 << 63 }, map[string]any{"big": uint64(1 << 63)}},
		{"float64", func(r *csRecord) { r.Score = 2.5 }, map[string]any{"score": 2.5}},
		{"float32", func(r *csRecord) { r.Ratio = 0.25 }, map[string]any{"ratio": float32(0.25)}},
		{"bool", func(r *csRecord) { r.Active = false }, map[string]any{"active": false}},
		{"named string", func(r *csRecord) { r.Status = "no" }, map[string]any{"status": csStatus("no")}},
		{"pointer, same pointee in new pointer", func(r *csRecord) { r.Nick = ptr("al") }, map[string]any{}},
		{"pointer, changed pointee", func(r *csRecord) { r.Nick = ptr("ally") }, map[string]any{"nick": "ally"}},
		{"pointer to nil", func(r *csRecord) { r.Nick = nil }, map[string]any{"nick": nil}},
		{"nil to pointer", func(r *csRecord) { r.Level = ptr(3) }, map[string]any{"level": 3}},
		{"slice element", func(r *csRecord) { r.Tags = []string{"a", "c"} }, map[string]any{"tags": []string{"a", "c"}}},
		{"slice equal copy", func(r *csRecord) { r.Tags = []string{"a", "b"} }, map[string]any{}},
		{"slice to nil", func(r *csRecord) { r.Tags = nil }, map[string]any{"tags": []string(nil)}},
		{"bytes", func(r *csRecord) { r.Data = []byte("xz") }, map[string]any{"data": []byte("xz")}},
		{"map value", func(r *csRecord) { r.Attrs = map[string]any{"k": 2} }, map[string]any{"attrs": map[string]any{"k": 2}}},
		{"interface", func(r *csRecord) { r.Any = 5 }, map[string]any{"any": 5}},
		{"time.Time", func(r *csRecord) { r.Born = born.Add(time.Second) }, map[string]any{"born": born.Add(time.Second)}},
		{"time.Time same instant, other location", func(r *csRecord) { r.Born = born.In(time.FixedZone("X", 3600)) },
			map[string]any{"born": born.In(time.FixedZone("X", 3600))}},
		{"*time.Time same instant in new pointer", func(r *csRecord) { r.Seen = ptr(born) }, map[string]any{}},
		{"*time.Time changed", func(r *csRecord) { r.Seen = ptr(born.Add(time.Hour)) }, map[string]any{"seen": born.Add(time.Hour)}},
		{"sql.NullString", func(r *csRecord) { r.Label = sql.NullString{} }, map[string]any{"label": sql.NullString{}}},
		{"auto columns ignored", func(r *csRecord) {
			r.ID = 2
			r.Created = born.Add(time.Hour)
			r.Updated = born.Add(time.Hour)
		}, map[string]any{}},
		{"db:\"-\" ignored", func(r *csRecord) { r.Ignored = "changed" }, map[string]any{}},
		{"several", func(r *csRecord) {
			r.Name = "bob"
			r.Age = 40
			r.Level = ptr(1)
		}, map[string]any{"name": "bob", "age": 40, "level": 1}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			old := baseCsRecord()
			updated := baseCsRecord()
			tt.modify(&updated)
			got, err := Changes(&old, &updated)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
			for k, v := range tt.want {
				assert.IsType(t, v, got[k], k)
			}
		})
	}
}

func TestChangesErrors(t *testing.T) {
	rec := baseCsRecord()
	_, err := Changes(nil, &rec)
	assert.ErrorIs(t, err, ErrNilRecord)
	_, err = Changes(&rec, nil)
	assert.ErrorIs(t, err, ErrNilRecord)

	a, b := shapeDupCols{}, shapeDupCols{}
	_, err = Changes(&a, &b)
	assert.ErrorIs(t, err, gohan.ErrDuplicateColumn)

	x, y := 1, 2
	_, err = Changes(&x, &y)
	assert.ErrorIs(t, err, ErrNotStruct)
}
