package sqlb

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- TestRecordInsertRules ---

type autoVariants struct {
	A1 int `db:",auto"`
	A2 int `auto:"true"`
	A3 int `grid:"auto"`
	A4 int `goqu:"skipinsert"`
	A5 int `goqu:"skipupdate"`
	X  int `db:"x"`
}

func TestRecordInsertRules_AutoTagForms(t *testing.T) {
	cols, vals, err := recordValues(autoVariants{X: 9}, true, recordOptions{})
	require.NoError(t, err)
	assert.Equal(t, []string{"x"}, cols)
	assert.Equal(t, []any{9}, vals)
}

type omitnilStruct struct {
	Set *string `db:"set_field" goqu:"omitnil"`
	Nil *string `db:"nil_field" goqu:"omitnil"`
}

func TestRecordInsertRules_OmitNil(t *testing.T) {
	s := "hello"
	cols, vals, err := recordValues(omitnilStruct{Set: &s, Nil: nil}, true, recordOptions{})
	require.NoError(t, err)
	assert.Equal(t, []string{"set_field"}, cols)
	assert.Equal(t, []any{"hello"}, vals)
}

type omitemptyStruct struct {
	S string  `db:"s" goqu:"omitempty"`
	I int     `db:"i" goqu:"omitempty"`
	L []int   `db:"l" goqu:"omitempty"`
	X *string `db:"x"`
}

func TestRecordInsertRules_OmitEmpty(t *testing.T) {
	// All empty: everything skipped except X (a plain pointer field with no
	// omit option, nil, so it binds nil).
	cols, vals, err := recordValues(omitemptyStruct{}, true, recordOptions{})
	require.NoError(t, err)
	assert.Equal(t, []string{"x"}, cols)
	assert.Equal(t, []any{nil}, vals)

	xv := "v"
	cols, vals, err = recordValues(omitemptyStruct{S: "a", I: 1, L: []int{1}, X: &xv}, true, recordOptions{})
	require.NoError(t, err)
	assert.Equal(t, []string{"s", "i", "l", "x"}, cols)
	assert.Equal(t, []any{"a", 1, []int{1}, "v"}, vals)
}

type pointerStruct struct {
	P *int `db:"p"`
}

func TestRecordInsertRules_PointerDeref(t *testing.T) {
	n := 42
	cols, vals, err := recordValues(pointerStruct{P: &n}, true, recordOptions{})
	require.NoError(t, err)
	assert.Equal(t, []string{"p"}, cols)
	assert.Equal(t, []any{42}, vals)

	cols, vals, err = recordValues(pointerStruct{}, true, recordOptions{})
	require.NoError(t, err)
	assert.Equal(t, []string{"p"}, cols)
	assert.Equal(t, []any{nil}, vals)
}

type EmbeddedBase struct {
	Name string `db:"name"`
}

type embeddedStruct struct {
	EmbeddedBase
	Age int `db:"age"`
}

func TestRecordInsertRules_ExportedEmbeddedFlattened(t *testing.T) {
	cols, vals, err := recordValues(embeddedStruct{EmbeddedBase{"n"}, 5}, true, recordOptions{})
	require.NoError(t, err)
	assert.Equal(t, []string{"name", "age"}, cols)
	assert.Equal(t, []any{"n", 5}, vals)
}

type timeStruct struct {
	When time.Time `db:"when"`
}

func TestRecordInsertRules_TimeIsOneColumn(t *testing.T) {
	now := time.Now()
	cols, vals, err := recordValues(timeStruct{When: now}, true, recordOptions{})
	require.NoError(t, err)
	assert.Equal(t, []string{"when"}, cols)
	assert.Equal(t, []any{now}, vals)
}

type dashStruct struct {
	Keep string `db:"keep"`
	Drop string `db:"-"`
}

func TestRecordInsertRules_DashSkipped(t *testing.T) {
	cols, vals, err := recordValues(dashStruct{Keep: "k", Drop: "d"}, true, recordOptions{})
	require.NoError(t, err)
	assert.Equal(t, []string{"keep"}, cols)
	assert.Equal(t, []any{"k"}, vals)
}

type dbOptionsIgnoredStruct struct {
	X string `db:"x,omitempty"`
}

func TestRecordInsertRules_DbTagOptionsIgnored(t *testing.T) {
	// Only "auto" is honoured inside the db tag; "omitempty" here is not.
	cols, vals, err := recordValues(dbOptionsIgnoredStruct{X: ""}, true, recordOptions{})
	require.NoError(t, err)
	assert.Equal(t, []string{"x"}, cols)
	assert.Equal(t, []any{""}, vals)
}

// --- TestRecordShapes ---

type withPtrBase struct {
	*EmbeddedBase
	Age int `db:"age"`
}

type unexportedEmbedded struct {
	lowerBase
	Age int `db:"age"`
}

type lowerBase struct {
	Name string `db:"name"`
}

type taggedAnon struct {
	EmbeddedBase `db:"-"`
	Age          int `db:"age"`
}

type Level3 struct {
	Value string `db:"level3_value"`
}

type Level2 struct {
	Level3
	Value string `db:"level2_value"`
}

type Level1 struct {
	Level2
	Value string `db:"level1_value"`
}

type dupDbNameDirect struct {
	ID int `db:"id"`
	DupDbNameBase
}

type DupDbNameBase struct {
	ID2 int `db:"id"`
}

func TestRecordShapes(t *testing.T) {
	_, _, err := recordValues(withPtrBase{}, true, recordOptions{})
	assert.True(t, errors.Is(err, ErrRecordShape), "embedded pointer: got %v", err)

	_, _, err = recordValues(unexportedEmbedded{}, true, recordOptions{})
	assert.True(t, errors.Is(err, ErrRecordShape), "unexported embedded: got %v", err)

	_, _, err = recordValues(taggedAnon{}, true, recordOptions{})
	assert.True(t, errors.Is(err, ErrRecordShape), "tagged embedded: got %v", err)

	_, _, err = recordValues(Level1{}, true, recordOptions{})
	assert.True(t, errors.Is(err, ErrRecordShape), "ambiguous promoted name: got %v", err)

	_, _, err = recordValues(dupDbNameDirect{}, true, recordOptions{})
	assert.True(t, errors.Is(err, ErrDuplicateColumn), "duplicate db name: got %v", err)

	_, _, err = recordValues(42, true, recordOptions{})
	assert.True(t, errors.Is(err, ErrInvalidRecord), "non-struct: got %v", err)

	var nilPtr *EmbeddedBase
	_, _, err = recordValues(nilPtr, true, recordOptions{})
	assert.True(t, errors.Is(err, ErrInvalidRecord), "nil pointer: got %v", err)
}

// --- TestRecordUpdateRules ---

type updateStruct struct {
	ID   int    `db:"id" auto:"true"`
	Name string `db:"name"`
	Age  int    `db:"age"`
}

func TestRecordUpdateRules(t *testing.T) {
	rec := updateStruct{ID: 1, Name: "n", Age: 0}

	cols, vals, err := recordValues(rec, false, recordOptions{})
	require.NoError(t, err)
	assert.Equal(t, []string{"name", "age"}, cols)
	assert.Equal(t, []any{"n", 0}, vals)

	cols, _, err = recordValues(rec, false, recordOptions{withAuto: true})
	require.NoError(t, err)
	assert.Equal(t, []string{"id", "name", "age"}, cols)

	cols, _, err = recordValues(rec, false, recordOptions{include: []string{"Name"}})
	require.NoError(t, err)
	assert.Equal(t, []string{"name"}, cols)

	cols, _, err = recordValues(rec, false, recordOptions{exclude: []string{"age"}})
	require.NoError(t, err)
	assert.Equal(t, []string{"name"}, cols)

	// include wins over exclude
	cols, _, err = recordValues(rec, false, recordOptions{include: []string{"Name"}, exclude: []string{"name"}})
	require.NoError(t, err)
	assert.Equal(t, []string{"name"}, cols)

	// IncludeFields naming an Auto field still skips it unless WithAutoFields.
	cols, _, err = recordValues(rec, false, recordOptions{include: []string{"id", "name"}})
	require.NoError(t, err)
	assert.Equal(t, []string{"name"}, cols)

	cols, _, err = recordValues(rec, false, recordOptions{include: []string{"id", "name"}, withAuto: true})
	require.NoError(t, err)
	assert.Equal(t, []string{"id", "name"}, cols)

	_, _, err = recordValues(rec, false, recordOptions{include: []string{"nope"}})
	assert.True(t, errors.Is(err, ErrUnknownField))

	_, _, err = recordValues(rec, false, recordOptions{exclude: []string{"nope"}})
	assert.True(t, errors.Is(err, ErrUnknownField))
}

// --- TestRecordBatchConsistency ---

type batchStruct struct {
	Name string  `db:"name"`
	Opt  *string `db:"opt" goqu:"omitnil"`
}

func TestRecordBatchConsistency(t *testing.T) {
	v := "x"
	r1 := batchStruct{Name: "a", Opt: &v}
	r2 := &batchStruct{Name: "b", Opt: &v}

	cols, rows, err := recordRows([]any{r1, r2})
	require.NoError(t, err)
	assert.Equal(t, []string{"name", "opt"}, cols)
	assert.Equal(t, [][]any{{"a", "x"}, {"b", "x"}}, rows)

	type other struct {
		Name string `db:"name"`
	}
	_, _, err = recordRows([]any{r1, other{Name: "c"}})
	assert.True(t, errors.Is(err, ErrRecordType))

	// first included Opt, second omits it (nil) -> inconsistent
	_, _, err = recordRows([]any{r1, batchStruct{Name: "b", Opt: nil}})
	assert.True(t, errors.Is(err, ErrInconsistentOmit))

	// first omits Opt, second includes it -> inconsistent
	_, _, err = recordRows([]any{batchStruct{Name: "a", Opt: nil}, r2})
	assert.True(t, errors.Is(err, ErrInconsistentOmit))
}

// --- TestRecordSnapshot ---

type snapshotStruct struct {
	Name string `db:"name"`
}

func TestRecordSnapshot(t *testing.T) {
	rec := &snapshotStruct{Name: "before"}
	b := Insert("t").Columns("name").Rows(rec)
	rec.Name = "after"

	sql, args, err := b.Build(Postgres())
	require.NoError(t, err)
	assert.Equal(t, `INSERT INTO "t" ("name") VALUES ($1)`, sql)
	assert.Equal(t, []any{"before"}, args)
}

// --- TestRecordColumns ---

func TestRecordColumns(t *testing.T) {
	cols, err := RecordColumns(reflect.TypeOf(updateStruct{}))
	require.NoError(t, err)
	assert.Equal(t, []string{"id", "name", "age"}, cols)

	cols, err = RecordColumns(reflect.TypeOf(&updateStruct{}))
	require.NoError(t, err)
	assert.Equal(t, []string{"id", "name", "age"}, cols)

	_, err = RecordColumns(reflect.TypeOf(Level1{}))
	assert.True(t, errors.Is(err, ErrRecordShape))

	cols, err = InsertColumns(updateStruct{ID: 1, Name: "n", Age: 2})
	require.NoError(t, err)
	assert.Equal(t, []string{"name", "age"}, cols)

	oe := omitemptyStruct{}
	cols, err = InsertColumns(oe)
	require.NoError(t, err)
	assert.Equal(t, []string{"x"}, cols)
}
