package clickhouse

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/oddbit-project/blueprint/dbx"
	"github.com/oddbit-project/gohan"
)

type ChkBase struct {
	Created time.Time `ch:"created" db:"created"`
}

type chkEmbedded struct {
	ID uint32 `ch:"id" db:"id"`
	ChkBase
}

type ChkNestedBase struct {
	ChkBase
	Owner string `ch:"owner" db:"owner"`
}

type chkNested struct {
	ChkNestedBase
	ID uint32 `ch:"id" db:"id"`
}

type chkExtras struct {
	ID       uint32 `ch:"id" db:"id,auto"`
	Hidden   string `db:"-"`
	Both     string `ch:"-" db:"-"`
	internal string //nolint:unused
}

type ChkUntaggedBase struct {
	Created time.Time `db:"created"`
}

type chkEmbedUntagged struct {
	ID uint32 `ch:"id" db:"id"`
	ChkUntaggedBase
}

type ChkShadowBase struct {
	Ref string `ch:"id" db:"ref"`
}

type chkShadow struct {
	ID uint32 `ch:"id" db:"id"`
	ChkShadowBase
}

type chkEmbedTime struct {
	ID        uint32 `ch:"id" db:"id"`
	time.Time `ch:"ts" db:"ts"`
}

type ChkCode string

type chkEmbedNonStruct struct {
	ID      uint32 `ch:"id" db:"id"`
	ChkCode `ch:"code" db:"code"`
}

type chkEmbedNonStructHidden struct {
	ID      uint32 `ch:"id" db:"id"`
	ChkCode `db:"-"`
}

type chkPtrEmbed struct {
	ID uint32 `ch:"id" db:"id"`
	*ChkBase
}

func TestCheckRecordAccepts(t *testing.T) {
	cases := []struct {
		name string
		typ  reflect.Type
	}{
		{"ch equals db", reflect.TypeFor[dbxEvent]()},
		{"ch tags only", reflect.TypeFor[struct {
			ID   uint32 `ch:"id"`
			Name string `ch:"name"`
		}]()},
		{"db tag with options", reflect.TypeFor[struct {
			ID uint32 `ch:"id" db:"id,auto"`
		}]()},
		{"db tag equal to the Go field name", reflect.TypeFor[struct {
			Name string `db:"Name"`
		}]()},
		{"time.Time fields", reflect.TypeFor[dbxTimeRecord]()},
		{"backslash column", reflect.TypeFor[dbxBackslashCol]()},
		{"embedded struct", reflect.TypeFor[chkEmbedded]()},
		{"nested embedded struct", reflect.TypeFor[chkNested]()},
		{"driver-only and unexported fields", reflect.TypeFor[chkExtras]()},
		{"embedded non-struct hidden with ch:\"-\"", reflect.TypeFor[struct {
			ID      uint32 `ch:"id" db:"id"`
			ChkCode `ch:"-" db:"-"`
		}]()},
		{"pointer to struct", reflect.TypeFor[*dbxEvent]()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.NoError(t, NewQuerier(nil).CheckRecord(tc.typ))
		})
	}
}

func TestCheckRecordRejects(t *testing.T) {
	cases := []struct {
		name string
		typ  reflect.Type
		want []string
	}{
		{
			name: "missing ch tag, field name differs",
			typ: reflect.TypeFor[struct {
				ID   uint32 `ch:"id" db:"id"`
				Name string `db:"name"`
			}](),
			want: []string{`column "name" (field Name)`, `"Name"`, "Go field name"},
		},
		{
			name: "untagged field",
			typ: reflect.TypeFor[struct {
				Name string
			}](),
			want: []string{`column "name" (field Name)`, `"Name"`},
		},
		{
			name: "different ch name",
			typ: reflect.TypeFor[struct {
				Name string `ch:"title" db:"name"`
			}](),
			want: []string{`column "name" (field Name)`, `"title"`, "ch tag"},
		},
		{
			name: "ch tag with options",
			typ: reflect.TypeFor[struct {
				Name string `ch:"name,omitempty"`
			}](),
			want: []string{`column "name" (field Name)`, `"name,omitempty"`},
		},
		{
			name: `ch:"-"`,
			typ: reflect.TypeFor[struct {
				ID   uint32 `ch:"id" db:"id"`
				Name string `ch:"-" db:"name"`
			}](),
			want: []string{`column "name" (field Name)`, `ch:"-"`},
		},
		{
			name: "embedded struct field without ch tag",
			typ:  reflect.TypeFor[chkEmbedUntagged](),
			want: []string{`column "created" (field ChkUntaggedBase.Created)`, `"Created"`},
		},
		{
			name: "embedded struct field shadows an outer column",
			typ:  reflect.TypeFor[chkShadow](),
			want: []string{
				`column "id" (field ID)`, "ChkShadowBase.Ref",
				`column "ref" (field ChkShadowBase.Ref)`,
			},
		},
		{
			name: "embedded time.Time",
			typ:  reflect.TypeFor[chkEmbedTime](),
			want: []string{`column "ts" (field Time)`, "embedded"},
		},
		{
			name: "embedded non-struct",
			typ:  reflect.TypeFor[chkEmbedNonStruct](),
			want: []string{"ChkCode", "panics"},
		},
		{
			name: "embedded non-struct excluded only from dbx",
			typ:  reflect.TypeFor[chkEmbedNonStructHidden](),
			want: []string{"ChkCode", "panics"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := NewQuerier(nil).CheckRecord(tc.typ)
			require.Error(t, err)
			assert.True(t, errors.Is(err, ErrRecordMapping), "got %v", err)
			for _, w := range tc.want {
				assert.Contains(t, err.Error(), w)
			}
		})
	}
}

func TestCheckRecordReportsEveryColumn(t *testing.T) {
	err := NewQuerier(nil).CheckRecord(reflect.TypeFor[struct {
		ID   uint32 `ch:"id" db:"id"`
		Name string `db:"name"`
		Tags string `ch:"labels" db:"tags"`
	}]())
	require.Error(t, err)
	assert.Contains(t, err.Error(), `column "name"`)
	assert.Contains(t, err.Error(), `column "tags"`)
	assert.NotContains(t, err.Error(), `column "id"`)
}

func TestCheckRecordShapeErrors(t *testing.T) {
	err := NewQuerier(nil).CheckRecord(reflect.TypeFor[chkPtrEmbed]())
	assert.True(t, errors.Is(err, gohan.ErrRecordShape), "got %v", err)
	assert.Contains(t, err.Error(), "ChkBase")

	err = NewQuerier(nil).CheckRecord(reflect.TypeFor[int]())
	assert.True(t, errors.Is(err, gohan.ErrInvalidRecord), "got %v", err)
}

func TestCheckRecordCached(t *testing.T) {
	type cached struct {
		Name string `db:"name"`
	}
	type good struct {
		Name string `ch:"name" db:"name"`
	}
	bad, ok := reflect.TypeFor[cached](), reflect.TypeFor[good]()
	recordChecks.Delete(bad)
	recordChecks.Delete(ok)

	// a failing check is not cached, so a later fix (e.g. registering a
	// reserved type) takes effect
	require.Error(t, NewQuerier(nil).CheckRecord(bad))
	_, found := recordChecks.Load(bad)
	assert.False(t, found, "a failing check is not cached")
	require.Error(t, NewQuerier(nil).CheckRecord(bad))

	require.NoError(t, NewQuerier(nil).CheckRecord(ok))
	_, found = recordChecks.Load(ok)
	assert.True(t, found, "a passing check is cached")
	assert.NoError(t, NewQuerier(nil).CheckRecord(ok))
}

func TestNewRepositoryChecksRecord(t *testing.T) {
	var _ dbx.RecordChecker = (*Querier)(nil)

	type bad struct {
		ID   uint32 `ch:"id" db:"id"`
		Name string `db:"name"`
	}
	r, err := dbx.NewRepository[bad](NewQuerier(nil), "events")
	assert.Nil(t, r)
	assert.True(t, errors.Is(err, ErrRecordMapping), "got %v", err)
	assert.Contains(t, err.Error(), `column "name" (field Name)`)

	good, err := dbx.NewRepository[dbxEvent](NewQuerier(nil), "events")
	require.NoError(t, err)
	assert.Equal(t, "events", good.Table())
}

// TestStructIdxPanicsOnEmbeddedNonStruct pins the driver behaviour
// CheckRecord reports for an embedded non-struct field (structIdx is this
// package's verbatim copy of clickhouse-go's mapper).
func TestStructIdxPanicsOnEmbeddedNonStruct(t *testing.T) {
	assert.Panics(t, func() { structIdx(reflect.TypeFor[chkEmbedNonStructHidden]()) })
}
