package dbx

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/oddbit-project/gohan"
)

// --- record types ---

type ksStatus string
type ksLevel int16

// ksEligible has one field of every Go type a keyset key may have.
type ksEligible struct {
	ID     int64     `db:"id"`
	I      int       `db:"i"`
	I8     int8      `db:"i8"`
	I16    int16     `db:"i16"`
	I32    int32     `db:"i32"`
	U      uint      `db:"u"`
	U8     uint8     `db:"u8"`
	U16    uint16    `db:"u16"`
	U32    uint32    `db:"u32"`
	U64    uint64    `db:"u64"`
	S      string    `db:"s"`
	Status ksStatus  `db:"status"`
	Level  ksLevel   `db:"level"`
	When   time.Time `db:"when"`
	UID    uuid.UUID `db:"uid"`
}

type ksValuerString string

func (v ksValuerString) Value() (driver.Value, error) { return string(v), nil }

type ksScannerInt int64

func (v *ksScannerInt) Scan(src any) error { return nil }

type ksPoint struct{ X, Y int }

// ksRejected has one field of every Go type a keyset key must not have.
type ksRejected struct {
	ID     int64          `db:"id"`
	Ptr    *int64         `db:"ptr"`
	PtrT   *time.Time     `db:"ptr_t"`
	Null   sql.NullInt64  `db:"null"`
	NullS  sql.NullString `db:"null_s"`
	Valuer ksValuerString `db:"valuer"`
	Scan   ksScannerInt   `db:"scan"`
	F32    float32        `db:"f32"`
	F64    float64        `db:"f64"`
	B      bool           `db:"b"`
	Bytes  []byte         `db:"bytes"`
	Arr    [4]byte        `db:"arr"`
	Point  ksPoint        `db:"point"`
	Map    map[string]int `db:"map"`
	Slice  []string       `db:"slice"`
	Any    any            `db:"any"`
	UPtr   uintptr        `db:"uptr"`
}

// KsShadowBase is embedded by ksShadow, whose shallower Name field (db:"-")
// shadows the mapped KsShadowBase.Name: reflect's FieldByName("Name") picks
// the unmapped outer field.
type KsShadowBase struct {
	ID   int64  `db:"id"`
	Name string `db:"name"`
}

type ksShadow struct {
	KsShadowBase
	Name string `db:"-"`
}

// ksChTagged maps its columns through ch tags only.
type ksChTagged struct {
	ID   uint32 `ch:"event_id"`
	Name string `ch:"name,opt"`
	Low  string
}

// --- step 1: key eligibility and field index ---

func TestKeysetKeyTypes(t *testing.T) {
	fields, err := keysetFieldsFor(reflect.TypeFor[ksEligible]())
	require.NoError(t, err)

	want := map[string]string{
		"id": "int64", "i": "int" + strconv.Itoa(strconv.IntSize), "i8": "int8", "i16": "int16", "i32": "int32",
		"u": "uint" + strconv.Itoa(strconv.IntSize), "u8": "uint8", "u16": "uint16", "u32": "uint32", "u64": "uint64",
		"s": "string", "status": "string", "level": "int16", "when": "time", "uid": "uuid",
	}
	require.Len(t, fields, len(want))
	for col, name := range want {
		f, ok := fields[col]
		require.True(t, ok, col)
		kt, err := keyTypeOf(f.typ)
		require.NoError(t, err, col)
		assert.Equal(t, name, kt.name(), col)
	}
}

func TestKeysetRejectedKeyTypes(t *testing.T) {
	cq := &countingQuerier{d: gohan.Postgres()}
	r, err := NewRepository[ksRejected](cq, "rejected")
	require.NoError(t, err)

	for _, col := range []string{"ptr", "ptr_t", "null", "null_s", "valuer", "scan", "f32", "f64", "b", "bytes", "arr", "point", "map", "slice", "any", "uptr"} {
		t.Run(col, func(t *testing.T) {
			_, err := r.ListKeyset(context.Background(), nil, []KeysetKey{KeyAsc(col), KeyAsc("id")}, 10, "")
			require.ErrorIs(t, err, ErrInvalidKeysetKey)
			assert.Contains(t, err.Error(), strconv.Quote(col))
			if col == "ptr" || col == "ptr_t" {
				assert.Contains(t, err.Error(), "nullable")
			}
		})
	}
	assert.Zero(t, cq.calls)
}

func TestKeysetValuesCanonical(t *testing.T) {
	fields, err := keysetFieldsFor(reflect.TypeFor[ksEligible]())
	require.NoError(t, err)
	p, err := newKeysetPlan(fields, "t", []KeysetKey{KeyAsc("when"), KeyAsc("i8"), KeyAsc("u16"), KeyAsc("status"), KeyAsc("uid")}, false)
	require.NoError(t, err)

	now := time.Now().In(time.FixedZone("x", -7*3600)) // monotonic reading, not UTC
	u := uuid.New()
	got := p.values(reflect.ValueOf(ksEligible{When: now, I8: -3, U16: 9, Status: "on", UID: u}))
	require.Len(t, got, 5)
	tm := got[0].(time.Time)
	assert.Equal(t, time.UTC, tm.Location())
	assert.True(t, tm.Equal(now))
	assert.NotContains(t, tm.String(), "m=", "monotonic reading stripped")
	assert.Equal(t, []any{int64(-3), uint64(9), "on", u}, got[1:])
}

func TestKeysetKeyListRejectedBeforeQuery(t *testing.T) {
	cq := &countingQuerier{d: gohan.Postgres()}
	r, err := NewRepository[ksEligible](cq, "eligible")
	require.NoError(t, err)

	tooMany := make([]KeysetKey, 0, MaxKeysetKeys+1)
	for _, c := range []string{"id", "i", "i8", "i16", "i32", "u", "u8", "u16", "u32"} {
		tooMany = append(tooMany, KeyAsc(c))
	}
	require.Len(t, tooMany, MaxKeysetKeys+1)

	cases := []struct {
		name   string
		keys   []KeysetKey
		cursor string
		want   error
	}{
		{"no keys", nil, "", ErrInvalidKeysetKey},
		{"empty keys", []KeysetKey{}, "", ErrInvalidKeysetKey},
		{"too many keys", tooMany, "", ErrInvalidKeysetKey},
		{"duplicate key", []KeysetKey{KeyAsc("id"), KeyDesc("id")}, "", ErrInvalidKeysetKey},
		{"unknown column", []KeysetKey{KeyAsc("nope")}, "", ErrUnknownColumn},
		{"invalid cursor", []KeysetKey{KeyAsc("id")}, "not-a-cursor", ErrInvalidCursor},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := r.ListKeyset(context.Background(), nil, c.keys, 10, c.cursor)
			require.ErrorIs(t, err, c.want)
		})
	}
	assert.Zero(t, cq.calls)

	t.Run("exactly MaxKeysetKeys is allowed", func(t *testing.T) {
		_, err := r.ListKeyset(context.Background(), nil, tooMany[:MaxKeysetKeys], 10, "")
		require.NoError(t, err)
		assert.Equal(t, 1, cq.calls)
	})
}

func TestKeysetFieldShadowing(t *testing.T) {
	fields, err := keysetFieldsFor(reflect.TypeFor[ksShadow]())
	require.NoError(t, err)
	assert.Equal(t, []int{0, 1}, fields["name"].path)

	rec := ksShadow{KsShadowBase: KsShadowBase{ID: 1, Name: "inner"}, Name: "outer"}
	p, err := newKeysetPlan(fields, "t", []KeysetKey{KeyAsc("name"), KeyAsc("id")}, false)
	require.NoError(t, err)
	assert.Equal(t, []any{"inner", int64(1)}, p.values(reflect.ValueOf(rec)))
}

func TestKeysetFieldsChTags(t *testing.T) {
	fields, err := keysetFieldsFor(reflect.TypeFor[ksChTagged]())
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"event_id", "name", "low"}, mapKeysOf(fields))
}

// TestKeysetFieldsMatchRecordColumns cross-checks the keyset field walker
// against gohan's column list for every record type the package's tests
// use.
func TestKeysetFieldsMatchRecordColumns(t *testing.T) {
	types := []reflect.Type{
		reflect.TypeFor[user](), reflect.TypeFor[userOpt](), reflect.TypeFor[gridUser](),
		reflect.TypeFor[shapeEmbedded](), reflect.TypeFor[shapeTime](), reflect.TypeFor[row](),
		reflect.TypeFor[gridRow](), reflect.TypeFor[dbxUser](), reflect.TypeFor[allOmit](),
		reflect.TypeFor[csRecord](), reflect.TypeFor[parityRow](), reflect.TypeFor[retRow](), reflect.TypeFor[grpRow](),
		reflect.TypeFor[userPostCount](), reflect.TypeFor[ksEligible](), reflect.TypeFor[ksRejected](),
		reflect.TypeFor[ksShadow](), reflect.TypeFor[ksChTagged](),
	}
	for _, typ := range types {
		t.Run(typ.Name(), func(t *testing.T) {
			cols, err := gohan.RecordColumns(typ)
			require.NoError(t, err)
			fields, err := keysetFieldsFor(typ)
			require.NoError(t, err)
			assert.ElementsMatch(t, cols, mapKeysOf(fields))
			for col, f := range fields {
				assert.Equal(t, f.typ, typ.FieldByIndex(f.path).Type, col)
			}
		})
	}
}

func mapKeysOf[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// --- step 3: statement, page mechanics, predicate ---

// ksItem is the record type for the statement and page tests.
type ksItem struct {
	ID      int64     `db:"id"`
	Score   int32     `db:"score"`
	Name    string    `db:"name"`
	Created time.Time `db:"created"`
}

var ksItemCols = []string{"id", "score", "name", "created"}

// ksCursor mints the cursor r's plan for keys would mint for vals.
func ksCursor[T any](t *testing.T, r *Repository[T], keys []KeysetKey, vals ...any) string {
	t.Helper()
	p, err := r.keysetPlan(keys)
	require.NoError(t, err)
	c, err := p.mint(vals)
	require.NoError(t, err)
	return c
}

func named(vals ...any) []any {
	out := make([]any, len(vals))
	for i, v := range vals {
		out[i] = sql.Named("p"+strconv.Itoa(i+1), v)
	}
	return out
}

func TestListKeysetSQL(t *testing.T) {
	ts := time.Date(2024, 5, 6, 7, 8, 9, 0, time.UTC)
	type keysCase struct {
		name   string
		keys   []KeysetKey
		vals   []any // cursor values; nil for the first page
		where  gohan.Expr
		pg     string
		sqlite string
		ch     string
		args   []any
	}
	cases := []keysCase{
		{
			name:   "first page",
			keys:   []KeysetKey{KeyAsc("id")},
			pg:     `SELECT "id", "score", "name", "created" FROM "items" ORDER BY "id" ASC LIMIT 11`,
			sqlite: "SELECT `id`, `score`, `name`, `created` FROM `items` ORDER BY `id` ASC LIMIT 11",
			ch:     `SELECT "id", "score", "name", "created" FROM "items" ORDER BY "id" ASC LIMIT 11`,
			args:   []any{},
		},
		{
			name:   "one key asc",
			keys:   []KeysetKey{KeyAsc("id")},
			vals:   []any{int64(7)},
			pg:     `SELECT "id", "score", "name", "created" FROM "items" WHERE "id" > $1 ORDER BY "id" ASC LIMIT 11`,
			sqlite: "SELECT `id`, `score`, `name`, `created` FROM `items` WHERE `id` > ? ORDER BY `id` ASC LIMIT 11",
			ch:     `SELECT "id", "score", "name", "created" FROM "items" WHERE "id" > @p1 ORDER BY "id" ASC LIMIT 11`,
			args:   []any{int64(7)},
		},
		{
			name:   "one key desc",
			keys:   []KeysetKey{KeyDesc("created")},
			vals:   []any{ts},
			pg:     `SELECT "id", "score", "name", "created" FROM "items" WHERE "created" < $1 ORDER BY "created" DESC LIMIT 11`,
			sqlite: "SELECT `id`, `score`, `name`, `created` FROM `items` WHERE `created` < ? ORDER BY `created` DESC LIMIT 11",
			ch:     `SELECT "id", "score", "name", "created" FROM "items" WHERE "created" < @p1 ORDER BY "created" DESC LIMIT 11`,
			args:   []any{ts},
		},
		{
			name:   "two keys asc",
			keys:   []KeysetKey{KeyAsc("score"), KeyAsc("id")},
			vals:   []any{int64(5), int64(7)},
			pg:     `SELECT "id", "score", "name", "created" FROM "items" WHERE ("score" >= $1 AND ("score" > $2 OR ("score" = $3 AND "id" > $4))) ORDER BY "score" ASC, "id" ASC LIMIT 11`,
			sqlite: "SELECT `id`, `score`, `name`, `created` FROM `items` WHERE (`score` >= ? AND (`score` > ? OR (`score` = ? AND `id` > ?))) ORDER BY `score` ASC, `id` ASC LIMIT 11",
			ch:     `SELECT "id", "score", "name", "created" FROM "items" WHERE ("score" >= @p1 AND ("score" > @p2 OR ("score" = @p3 AND "id" > @p4))) ORDER BY "score" ASC, "id" ASC LIMIT 11`,
			args:   []any{int64(5), int64(5), int64(5), int64(7)},
		},
		{
			name:   "two keys desc",
			keys:   []KeysetKey{KeyDesc("score"), KeyDesc("id")},
			vals:   []any{int64(5), int64(7)},
			pg:     `SELECT "id", "score", "name", "created" FROM "items" WHERE ("score" <= $1 AND ("score" < $2 OR ("score" = $3 AND "id" < $4))) ORDER BY "score" DESC, "id" DESC LIMIT 11`,
			sqlite: "SELECT `id`, `score`, `name`, `created` FROM `items` WHERE (`score` <= ? AND (`score` < ? OR (`score` = ? AND `id` < ?))) ORDER BY `score` DESC, `id` DESC LIMIT 11",
			ch:     `SELECT "id", "score", "name", "created" FROM "items" WHERE ("score" <= @p1 AND ("score" < @p2 OR ("score" = @p3 AND "id" < @p4))) ORDER BY "score" DESC, "id" DESC LIMIT 11`,
			args:   []any{int64(5), int64(5), int64(5), int64(7)},
		},
		{
			name:   "two keys mixed",
			keys:   []KeysetKey{KeyDesc("created"), KeyAsc("id")},
			vals:   []any{ts, int64(7)},
			pg:     `SELECT "id", "score", "name", "created" FROM "items" WHERE ("created" <= $1 AND ("created" < $2 OR ("created" = $3 AND "id" > $4))) ORDER BY "created" DESC, "id" ASC LIMIT 11`,
			sqlite: "SELECT `id`, `score`, `name`, `created` FROM `items` WHERE (`created` <= ? AND (`created` < ? OR (`created` = ? AND `id` > ?))) ORDER BY `created` DESC, `id` ASC LIMIT 11",
			ch:     `SELECT "id", "score", "name", "created" FROM "items" WHERE ("created" <= @p1 AND ("created" < @p2 OR ("created" = @p3 AND "id" > @p4))) ORDER BY "created" DESC, "id" ASC LIMIT 11`,
			args:   []any{ts, ts, ts, int64(7)},
		},
		{
			name:   "three keys mixed",
			keys:   []KeysetKey{KeyAsc("name"), KeyDesc("score"), KeyAsc("id")},
			vals:   []any{"bob", int64(5), int64(7)},
			pg:     `SELECT "id", "score", "name", "created" FROM "items" WHERE ("name" >= $1 AND ("name" > $2 OR ("name" = $3 AND "score" < $4) OR ("name" = $5 AND "score" = $6 AND "id" > $7))) ORDER BY "name" ASC, "score" DESC, "id" ASC LIMIT 11`,
			sqlite: "SELECT `id`, `score`, `name`, `created` FROM `items` WHERE (`name` >= ? AND (`name` > ? OR (`name` = ? AND `score` < ?) OR (`name` = ? AND `score` = ? AND `id` > ?))) ORDER BY `name` ASC, `score` DESC, `id` ASC LIMIT 11",
			ch:     `SELECT "id", "score", "name", "created" FROM "items" WHERE ("name" >= @p1 AND ("name" > @p2 OR ("name" = @p3 AND "score" < @p4) OR ("name" = @p5 AND "score" = @p6 AND "id" > @p7))) ORDER BY "name" ASC, "score" DESC, "id" ASC LIMIT 11`,
			args:   []any{"bob", "bob", "bob", int64(5), "bob", int64(5), int64(7)},
		},
		{
			name:   "caller where keeps its parentheses",
			keys:   []KeysetKey{KeyAsc("score"), KeyAsc("id")},
			vals:   []any{int64(5), int64(7)},
			where:  gohan.Or(gohan.Col("name").Eq("a"), gohan.Raw("score > ? OR score < ?", 1, 0)),
			pg:     `SELECT "id", "score", "name", "created" FROM "items" WHERE (("name" = $1 OR (score > $2 OR score < $3)) AND ("score" >= $4 AND ("score" > $5 OR ("score" = $6 AND "id" > $7)))) ORDER BY "score" ASC, "id" ASC LIMIT 11`,
			sqlite: "SELECT `id`, `score`, `name`, `created` FROM `items` WHERE ((`name` = ? OR (score > ? OR score < ?)) AND (`score` >= ? AND (`score` > ? OR (`score` = ? AND `id` > ?)))) ORDER BY `score` ASC, `id` ASC LIMIT 11",
			ch:     `SELECT "id", "score", "name", "created" FROM "items" WHERE (("name" = @p1 OR (score > @p2 OR score < @p3)) AND ("score" >= @p4 AND ("score" > @p5 OR ("score" = @p6 AND "id" > @p7)))) ORDER BY "score" ASC, "id" ASC LIMIT 11`,
			args:   []any{"a", 1, 0, int64(5), int64(5), int64(5), int64(7)},
		},
		{
			name:   "raw where alone is wrapped",
			keys:   []KeysetKey{KeyAsc("id")},
			vals:   []any{int64(7)},
			where:  gohan.Raw("score > ? OR score < ?", 1, 0),
			pg:     `SELECT "id", "score", "name", "created" FROM "items" WHERE ((score > $1 OR score < $2) AND "id" > $3) ORDER BY "id" ASC LIMIT 11`,
			sqlite: "SELECT `id`, `score`, `name`, `created` FROM `items` WHERE ((score > ? OR score < ?) AND `id` > ?) ORDER BY `id` ASC LIMIT 11",
			ch:     `SELECT "id", "score", "name", "created" FROM "items" WHERE ((score > @p1 OR score < @p2) AND "id" > @p3) ORDER BY "id" ASC LIMIT 11`,
			args:   []any{1, 0, int64(7)},
		},
		{
			name:   "empty where is no filter",
			keys:   []KeysetKey{KeyAsc("id")},
			where:  gohan.And(),
			pg:     `SELECT "id", "score", "name", "created" FROM "items" ORDER BY "id" ASC LIMIT 11`,
			sqlite: "SELECT `id`, `score`, `name`, `created` FROM `items` ORDER BY `id` ASC LIMIT 11",
			ch:     `SELECT "id", "score", "name", "created" FROM "items" ORDER BY "id" ASC LIMIT 11`,
			args:   []any{},
		},
	}
	dialects := []struct {
		name string
		d    gohan.Dialect
		want func(keysCase) (string, []any)
	}{
		{"postgres", gohan.Postgres(), func(c keysCase) (string, []any) { return c.pg, c.args }},
		{"sqlite", gohan.SQLite(), func(c keysCase) (string, []any) { return c.sqlite, c.args }},
		{"clickhouse", gohan.ClickHouseNamed(), func(c keysCase) (string, []any) { return c.ch, named(c.args...) }},
	}
	for _, d := range dialects {
		for _, c := range cases {
			t.Run(d.name+"/"+c.name, func(t *testing.T) {
				rq := &recordingQuerier{d: d.d}
				r, err := NewRepository[ksItem](rq, "items")
				require.NoError(t, err)
				cursor := ""
				if c.vals != nil {
					cursor = ksCursor(t, r, c.keys, c.vals...)
				}
				page, err := r.ListKeyset(context.Background(), c.where, c.keys, 10, cursor)
				require.NoError(t, err)
				assert.NotNil(t, page.Items)
				require.Len(t, rq.calls, 1)
				wantSQL, wantArgs := d.want(c)
				assert.Equal(t, "Select", rq.calls[0].method)
				assert.Equal(t, wantSQL, rq.calls[0].sql)
				assert.Equal(t, wantArgs, append([]any{}, rq.calls[0].args...))
			})
		}
	}
}

// ksMock returns a sqlmock-backed repository over ksItem on table "items".
func ksMock(t *testing.T) (*Repository[ksItem], sqlmock.Sqlmock) {
	t.Helper()
	q, mock := newMockQuerier(t)
	r, err := NewRepository[ksItem](q, "items")
	require.NoError(t, err)
	return r, mock
}

// ksRows returns sqlmock rows for items with ids ids, score = id / 10,
// name "n<id>".
func ksRows(ids ...int64) *sqlmock.Rows {
	rows := sqlmock.NewRows(ksItemCols)
	for _, id := range ids {
		rows.AddRow(id, int32(id/10), "n"+strconv.FormatInt(id, 10), time.Unix(id, 0).UTC())
	}
	return rows
}

const ksFirstPageSQL = `SELECT "id", "score", "name", "created" FROM "items" ORDER BY "id" ASC LIMIT `

func TestListKeysetPage(t *testing.T) {
	ctx := context.Background()
	keys := []KeysetKey{KeyAsc("id")}

	t.Run("probe row means another page", func(t *testing.T) {
		r, mock := ksMock(t)
		mock.ExpectQuery(ksFirstPageSQL + "4").WillReturnRows(ksRows(1, 2, 3, 4))
		page, err := r.ListKeyset(ctx, nil, keys, 3, "")
		require.NoError(t, err)
		require.NoError(t, mock.ExpectationsWereMet())
		assert.True(t, page.HasMore)
		require.Len(t, page.Items, 3)
		assert.Equal(t, int64(3), page.Items[2].ID)
		assert.Equal(t, ksCursor(t, r, keys, int64(3)), page.NextCursor)

		mock.ExpectQuery(`SELECT "id", "score", "name", "created" FROM "items" WHERE "id" > $1 ORDER BY "id" ASC LIMIT 4`).
			WithArgs(int64(3)).WillReturnRows(ksRows(4))
		page, err = r.ListKeyset(ctx, nil, keys, 3, page.NextCursor)
		require.NoError(t, err)
		require.NoError(t, mock.ExpectationsWereMet())
		assert.False(t, page.HasMore)
		assert.Empty(t, page.NextCursor)
		require.Len(t, page.Items, 1)
	})

	t.Run("exactly pageSize rows is the last page", func(t *testing.T) {
		r, mock := ksMock(t)
		mock.ExpectQuery(ksFirstPageSQL + "4").WillReturnRows(ksRows(1, 2, 3))
		page, err := r.ListKeyset(ctx, nil, keys, 3, "")
		require.NoError(t, err)
		assert.False(t, page.HasMore)
		assert.Empty(t, page.NextCursor)
		assert.Len(t, page.Items, 3)
	})

	t.Run("zero rows", func(t *testing.T) {
		r, mock := ksMock(t)
		mock.ExpectQuery(ksFirstPageSQL + "4").WillReturnRows(ksRows())
		page, err := r.ListKeyset(ctx, nil, keys, 3, "")
		require.NoError(t, err)
		assert.False(t, page.HasMore)
		assert.Empty(t, page.NextCursor)
		require.NotNil(t, page.Items)
		assert.Empty(t, page.Items)
	})

	for _, c := range []struct {
		pageSize int
		limit    string
	}{{0, "101"}, {-5, "101"}, {1, "2"}, {DefaultMaxLimit, "1001"}, {5000, "1001"}} {
		t.Run("page size "+strconv.Itoa(c.pageSize), func(t *testing.T) {
			r, mock := ksMock(t)
			mock.ExpectQuery(ksFirstPageSQL + c.limit).WillReturnRows(ksRows())
			_, err := r.ListKeyset(ctx, nil, keys, c.pageSize, "")
			require.NoError(t, err)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}

	t.Run("query error is returned", func(t *testing.T) {
		r, mock := ksMock(t)
		boom := errors.New("boom")
		mock.ExpectQuery(ksFirstPageSQL + "4").WillReturnError(boom)
		_, err := r.ListKeyset(ctx, nil, keys, 3, "")
		require.ErrorIs(t, err, boom)
	})
}

func TestListKeysetNotUnique(t *testing.T) {
	ctx := context.Background()
	keys := []KeysetKey{KeyAsc("score")}
	sqlFor := `SELECT "id", "score", "name", "created" FROM "items" ORDER BY "score" ASC LIMIT 4`

	t.Run("duplicate straddling the page boundary", func(t *testing.T) {
		r, mock := ksMock(t)
		mock.ExpectQuery(sqlFor).WillReturnRows(ksRows(10, 20, 30, 31))
		_, err := r.ListKeyset(ctx, nil, keys, 3, "")
		require.ErrorIs(t, err, ErrKeysetNotUnique)
	})

	t.Run("duplicate inside the page", func(t *testing.T) {
		r, mock := ksMock(t)
		mock.ExpectQuery(sqlFor).WillReturnRows(ksRows(10, 11))
		_, err := r.ListKeyset(ctx, nil, keys, 3, "")
		require.ErrorIs(t, err, ErrKeysetNotUnique)
	})

	t.Run("distinct keys", func(t *testing.T) {
		r, mock := ksMock(t)
		mock.ExpectQuery(sqlFor).WillReturnRows(ksRows(10, 20, 30, 40))
		page, err := r.ListKeyset(ctx, nil, keys, 3, "")
		require.NoError(t, err)
		assert.True(t, page.HasMore)
	})

	t.Run("same instant in another zone", func(t *testing.T) {
		r, mock := ksMock(t)
		at := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
		rows := sqlmock.NewRows(ksItemCols).
			AddRow(int64(1), int32(1), "a", at).
			AddRow(int64(2), int32(2), "b", at.In(time.FixedZone("x", 3600)))
		mock.ExpectQuery(`SELECT "id", "score", "name", "created" FROM "items" ORDER BY "created" ASC LIMIT 4`).WillReturnRows(rows)
		_, err := r.ListKeyset(ctx, nil, []KeysetKey{KeyAsc("created")}, 3, "")
		require.ErrorIs(t, err, ErrKeysetNotUnique)
	})

	t.Run("equal on a prefix only", func(t *testing.T) {
		r, mock := ksMock(t)
		mock.ExpectQuery(`SELECT "id", "score", "name", "created" FROM "items" ORDER BY "score" ASC, "id" ASC LIMIT 4`).
			WillReturnRows(ksRows(10, 11, 12, 13))
		page, err := r.ListKeyset(ctx, nil, []KeysetKey{KeyAsc("score"), KeyAsc("id")}, 3, "")
		require.NoError(t, err)
		assert.Len(t, page.Items, 3)
	})
}

func TestListKeysetMintFailures(t *testing.T) {
	ctx := context.Background()
	keys := []KeysetKey{KeyAsc("name")}
	sqlFor := `SELECT "id", "score", "name", "created" FROM "items" ORDER BY "name" ASC LIMIT 2`

	t.Run("cursor over the cap", func(t *testing.T) {
		r, mock := ksMock(t)
		long := strings.Repeat("x", 4000)
		rows := sqlmock.NewRows(ksItemCols).
			AddRow(int64(1), int32(1), long, time.Unix(0, 0)).
			AddRow(int64(2), int32(1), long+"y", time.Unix(0, 0))
		mock.ExpectQuery(sqlFor).WillReturnRows(rows)
		page, err := r.ListKeyset(ctx, nil, keys, 1, "")
		require.ErrorIs(t, err, ErrCursorTooLarge)
		assert.NotErrorIs(t, err, ErrInvalidCursor)
		assert.Nil(t, page)
	})

	t.Run("invalid UTF-8 key", func(t *testing.T) {
		r, mock := ksMock(t)
		rows := sqlmock.NewRows(ksItemCols).
			AddRow(int64(1), int32(1), "a\xff", time.Unix(0, 0)).
			AddRow(int64(2), int32(1), "b", time.Unix(0, 0))
		mock.ExpectQuery(sqlFor).WillReturnRows(rows)
		_, err := r.ListKeyset(ctx, nil, keys, 1, "")
		require.ErrorIs(t, err, ErrInvalidKeysetKey)
	})

	t.Run("last page mints nothing", func(t *testing.T) {
		r, mock := ksMock(t)
		rows := sqlmock.NewRows(ksItemCols).AddRow(int64(1), int32(1), "a\xff", time.Unix(0, 0))
		mock.ExpectQuery(sqlFor).WillReturnRows(rows)
		page, err := r.ListKeyset(ctx, nil, keys, 1, "")
		require.NoError(t, err)
		assert.False(t, page.HasMore)
	})
}

// evalSeek evaluates p for a row and cursor of small integers.
func evalSeek(p seekPred, row, cur []int) bool {
	cmp := func(c seekCmp) bool {
		a, b := row[c.key], cur[c.key]
		switch c.op {
		case seekGt:
			return a > b
		case seekLt:
			return a < b
		case seekGte:
			return a >= b
		case seekLte:
			return a <= b
		default:
			return a == b
		}
	}
	for _, c := range p.lead {
		if !cmp(c) {
			return false
		}
	}
	for _, and := range p.any {
		ok := true
		for _, c := range and {
			ok = ok && cmp(c)
		}
		if ok {
			return true
		}
	}
	return false
}

// after reports whether row sorts strictly after cur under directions desc.
func after(row, cur []int, desc []bool) bool {
	for i := range row {
		if row[i] == cur[i] {
			continue
		}
		return (row[i] > cur[i]) != desc[i]
	}
	return false
}

// tuples returns every n-tuple over {0, 1, 2}.
func tuples(n int) [][]int {
	if n == 0 {
		return [][]int{{}}
	}
	var out [][]int
	for _, rest := range tuples(n - 1) {
		for v := 0; v < 3; v++ {
			out = append(out, append(append([]int{}, rest...), v))
		}
	}
	return out
}

func TestSeekPredicateTruthTable(t *testing.T) {
	for n := 1; n <= 3; n++ {
		for mask := 0; mask < 1<<n; mask++ {
			desc := make([]bool, n)
			for i := range desc {
				desc[i] = mask&(1<<i) != 0
			}
			p := newSeekPred(desc)
			if n == 1 {
				assert.Empty(t, p.lead)
			} else {
				require.Len(t, p.lead, 1)
			}
			for _, cur := range tuples(n) {
				for _, row := range tuples(n) {
					assert.Equal(t, after(row, cur, desc), evalSeek(p, row, cur), "desc=%v row=%v cur=%v", desc, row, cur)
				}
			}
		}
	}
}
