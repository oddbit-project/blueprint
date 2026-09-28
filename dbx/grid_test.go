package dbx

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"math"
	"net"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/oddbit-project/blueprint/db"
	"github.com/oddbit-project/gohan"
)

// row is the golden record type used across this file: normative aliases
// and grid flags for TestGridBuildGolden and friends.
type row struct {
	ID    int    `db:"id" json:"id" grid:"sort,filter"`
	Name  string `db:"name" json:"name" grid:"search,sort"`
	Email string `db:"email" json:"email" grid:"search"`
	Tag   string `db:"tag" json:"tag" grid:"filter"`
	Note  string `db:"note" json:"note"` // not addressable
}

func rowBase() *gohan.SelectBuilder {
	return gohan.Select("id", "name", "email", "tag").From("rows")
}

func buildRow(t *testing.T, q *GridQuery) (string, []any, error) {
	t.Helper()
	g, err := NewGrid[row]()
	require.NoError(t, err)
	// no row cap, so goldens show exactly the paging the query asked for
	sb, err := g.WithMaxLimit(0).Build(rowBase(), q)
	if err != nil {
		return "", nil, err
	}
	return sb.Build(gohan.Postgres())
}

// --- Step 1: spec and types ---

func TestGridSpecCacheByType(t *testing.T) {
	type typA struct {
		ID int `db:"id" grid:"sort"`
	}
	type typB struct {
		ID   int    `db:"id" grid:"sort"`
		Name string `db:"name" grid:"filter"`
	}

	// Two anonymous struct types (Name() == "" for both) with different
	// grid-flagged fields must not share a spec: proves the cache is keyed
	// by reflect.Type, not t.Name() (db.Grid's defect).
	specA, err := getGridSpec(reflect.TypeOf(struct {
		ID int `db:"id" grid:"sort"`
	}{}))
	require.NoError(t, err)
	specB, err := getGridSpec(reflect.TypeOf(struct {
		ID   int    `db:"id" grid:"sort"`
		Name string `db:"name" grid:"filter"`
	}{}))
	require.NoError(t, err)

	assert.Len(t, specA.filterFields, 0)
	assert.Len(t, specB.filterFields, 1)

	// same call twice returns the identical cached spec
	specA2, err := getGridSpec(reflect.TypeOf(struct {
		ID int `db:"id" grid:"sort"`
	}{}))
	require.NoError(t, err)
	assert.Same(t, specA, specA2)

	_ = typA{}
	_ = typB{}
}

func TestNewGridRejects(t *testing.T) {
	t.Run("duplicate alias", func(t *testing.T) {
		type dup struct {
			A int `db:"a" alias:"x" grid:"filter"`
			B int `db:"b" alias:"x" grid:"filter"`
		}
		_, err := NewGrid[dup]()
		assert.Error(t, err)
	})

	t.Run("empty alias from omitempty", func(t *testing.T) {
		type emptyAlias struct {
			A int `db:"a" json:",omitempty" grid:"filter"`
		}
		_, err := NewGrid[emptyAlias]()
		assert.Error(t, err)
	})

	t.Run("dash alias", func(t *testing.T) {
		type dashAlias struct {
			A int `db:"a" json:"-" grid:"filter"`
		}
		_, err := NewGrid[dashAlias]()
		assert.Error(t, err)
	})

	t.Run("searchable int field", func(t *testing.T) {
		type badSearch struct {
			A int `db:"a" grid:"search"`
		}
		_, err := NewGrid[badSearch]()
		assert.Error(t, err)
	})

	t.Run("valid row", func(t *testing.T) {
		_, err := NewGrid[row]()
		assert.NoError(t, err)
	})
}

func TestNewGridQuery(t *testing.T) {
	q, err := NewGridQuery(SearchAny, 10, 0)
	require.NoError(t, err)
	assert.Equal(t, uint(SearchAny), q.SearchType)
	assert.Equal(t, uint(10), q.Limit)

	_, err = NewGridQuery(99, 10, 0)
	require.Error(t, err)
	var gerr GridError
	require.True(t, errors.As(err, &gerr))
	assert.Equal(t, "search", gerr.Scope)
	assert.Equal(t, "invalid search type", gerr.Message)
}

func TestGridQueryPage(t *testing.T) {
	q := &GridQuery{}
	q.Page(1, 20)
	assert.Equal(t, uint(0), q.Offset)
	assert.Equal(t, uint(20), q.Limit)

	q.Page(3, 20)
	assert.Equal(t, uint(40), q.Offset)
	assert.Equal(t, uint(20), q.Limit)

	// page < 1 defaults to 1; itemsPerPage < 1 defaults to DefaultPageSize
	q.Page(0, 0)
	assert.Equal(t, uint(0), q.Offset)
	assert.Equal(t, uint(DefaultPageSize), q.Limit)
}

func TestGridErrorMessage(t *testing.T) {
	withField := GridError{Scope: "filter", Field: "tag", Message: "field is not valid"}
	assert.Equal(t, "error on filter with field tag: field is not valid", withField.Error())

	noField := GridError{Scope: "search", Message: "search not allowed"}
	assert.Equal(t, "error on search: search not allowed", noField.Error())
}

func TestGridQueryJSON(t *testing.T) {
	var q1, q2 GridQuery
	require.NoError(t, json.Unmarshal([]byte(`{"SearchType":1}`), &q1))
	require.NoError(t, json.Unmarshal([]byte(`{"searchType":1}`), &q2))
	assert.Equal(t, uint(1), q1.SearchType)
	assert.Equal(t, uint(1), q2.SearchType)

	out, err := json.Marshal(&GridQuery{SearchType: 2})
	require.NoError(t, err)
	assert.Contains(t, string(out), `"searchType"`)
}

// --- Step 2: ValidQuery and Build ---

func TestGridBuildGolden(t *testing.T) {
	cases := []struct {
		name    string
		q       *GridQuery
		wantSQL string
		wantArg []any
		wantErr *GridError
	}{
		{
			name:    "empty",
			q:       &GridQuery{},
			wantSQL: `SELECT "id", "name", "email", "tag" FROM "rows"`,
			wantArg: []any{},
		},
		{
			name:    "filters sorted alias order",
			q:       &GridQuery{FilterFields: map[string]any{"tag": "a", "id": float64(3)}},
			wantSQL: `SELECT "id", "name", "email", "tag" FROM "rows" WHERE ("id" = $1 AND "tag" = $2)`,
			wantArg: []any{float64(3), "a"},
		},
		{
			name:    "filter list uses IN",
			q:       &GridQuery{FilterFields: map[string]any{"id": []any{float64(1), float64(2)}}},
			wantSQL: `SELECT "id", "name", "email", "tag" FROM "rows" WHERE "id" IN ($1, $2)`,
			wantArg: []any{float64(1), float64(2)},
		},
		{
			name:    "nil filter value is IS NULL",
			q:       &GridQuery{FilterFields: map[string]any{"tag": nil}},
			wantSQL: `SELECT "id", "name", "email", "tag" FROM "rows" WHERE "tag" IS NULL`,
			wantArg: []any{},
		},
		{
			name:    "search any",
			q:       &GridQuery{SearchType: SearchAny, SearchText: "5%"},
			wantSQL: `SELECT "id", "name", "email", "tag" FROM "rows" WHERE ("name" LIKE $1 ESCAPE '!' OR "email" LIKE $2 ESCAPE '!')`,
			wantArg: []any{"%5!%%", "%5!%%"},
		},
		{
			name:    "search start",
			q:       &GridQuery{SearchType: SearchStart, SearchText: "ab"},
			wantSQL: `SELECT "id", "name", "email", "tag" FROM "rows" WHERE ("name" LIKE $1 ESCAPE '!' OR "email" LIKE $2 ESCAPE '!')`,
			wantArg: []any{"ab%", "ab%"},
		},
		{
			name:    "search end",
			q:       &GridQuery{SearchType: SearchEnd, SearchText: "ab"},
			wantSQL: `SELECT "id", "name", "email", "tag" FROM "rows" WHERE ("name" LIKE $1 ESCAPE '!' OR "email" LIKE $2 ESCAPE '!')`,
			wantArg: []any{"%ab", "%ab"},
		},
		{
			name: "combined filter+search+sort+paging",
			q: &GridQuery{
				FilterFields: map[string]any{"tag": "a"},
				SearchType:   SearchAny,
				SearchText:   "x",
				SortFields:   map[string]string{"name": "asc", "id": ""},
				Limit:        10,
				Offset:       20,
			},
			wantSQL: `SELECT "id", "name", "email", "tag" FROM "rows" WHERE ("tag" = $1 AND ("name" LIKE $2 ESCAPE '!' OR "email" LIKE $3 ESCAPE '!')) ORDER BY "id" DESC, "name" ASC LIMIT 10 OFFSET 20`,
			wantArg: []any{"a", "%x%", "%x%"},
		},
		{
			name:    "offset only",
			q:       &GridQuery{Offset: 5},
			wantSQL: `SELECT "id", "name", "email", "tag" FROM "rows" OFFSET 5`,
			wantArg: []any{},
		},
		{
			name:    "not filterable",
			q:       &GridQuery{FilterFields: map[string]any{"name": "x"}},
			wantErr: &GridError{Scope: "filter", Field: "name", Message: "field is not filterable"},
		},
		{
			name:    "not addressable",
			q:       &GridQuery{FilterFields: map[string]any{"note": "x"}},
			wantErr: &GridError{Scope: "filter", Field: "note", Message: "field is not valid"},
		},
		{
			name:    "nested list rejected",
			q:       &GridQuery{FilterFields: map[string]any{"tag": []any{[]any{map[string]any{"k') OR 1=1 --": 1}}}}},
			wantErr: &GridError{Scope: "filter", Field: "tag", Message: "value is not valid"},
		},
		{
			name:    "nested list of scalars rejected",
			q:       &GridQuery{FilterFields: map[string]any{"tag": []any{[]any{float64(1), float64(2)}}}},
			wantErr: &GridError{Scope: "filter", Field: "tag", Message: "value is not valid"},
		},
		{
			name:    "map rejected",
			q:       &GridQuery{FilterFields: map[string]any{"tag": map[string]any{"a": 1}}},
			wantErr: &GridError{Scope: "filter", Field: "tag", Message: "value is not valid"},
		},
		{
			name:    "invalid search type",
			q:       &GridQuery{SearchType: 99, SearchText: "x"},
			wantErr: &GridError{Scope: "search", Message: "invalid search type"},
		},
		{
			name:    "invalid sort order",
			q:       &GridQuery{SortFields: map[string]string{"id": "sideways"}},
			wantErr: &GridError{Scope: "sort", Field: "id", Message: "sort order is not valid"},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gotSQL, gotArgs, err := buildRow(t, c.q)
			if c.wantErr != nil {
				require.Error(t, err)
				var gerr GridError
				require.True(t, errors.As(err, &gerr))
				assert.Equal(t, *c.wantErr, gerr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, c.wantSQL, gotSQL)
			assert.Equal(t, c.wantArg, gotArgs)
		})
	}
}

func TestGridRejectsCompoundBase(t *testing.T) {
	g, err := NewGrid[row]()
	require.NoError(t, err)
	compound := rowBase().Union(gohan.Select("id", "name", "email", "tag").From("more_rows"))
	_, err = g.Build(compound, &GridQuery{})
	require.Error(t, err)
	var gerr GridError
	require.True(t, errors.As(err, &gerr))
	assert.Equal(t, GridError{Scope: "query", Message: "base query must not be a UNION; wrap it with gohan.From(q.As(...))"}, gerr)
}

func TestGridCompoundBaseWorkaround(t *testing.T) {
	g, err := NewGrid[row]()
	require.NoError(t, err)
	compound := gohan.Select("id", "name", "email", "tag").From("rows").
		Union(gohan.Select("id", "name", "email", "tag").From("more_rows"))
	base := gohan.From(compound.As("u"))
	sb, err := g.Build(base, &GridQuery{FilterFields: map[string]any{"tag": "a"}})
	require.NoError(t, err)
	gotSQL, gotArgs, err := sb.Build(gohan.Postgres())
	require.NoError(t, err)
	assert.Equal(t,
		`SELECT * FROM (SELECT "id", "name", "email", "tag" FROM "rows" UNION SELECT "id", "name", "email", "tag" FROM "more_rows") AS "u" WHERE "tag" = $1 LIMIT 1000 OFFSET 0`,
		gotSQL)
	assert.Equal(t, []any{"a"}, gotArgs)
}

// TestGridBuildGoldenSQLite proves the SQLite-specific "LIMIT -1 OFFSET n"
// rendering for an offset-only query, exercised through gohan rather than
// re-implemented in Build.
func TestGridBuildGoldenSQLite(t *testing.T) {
	g, err := NewGrid[row]()
	require.NoError(t, err)
	sb, err := g.Build(rowBase(), &GridQuery{Offset: 5})
	require.NoError(t, err)
	gotSQL, gotArgs, err := sb.Build(gohan.SQLite())
	require.NoError(t, err)
	assert.Equal(t, "SELECT `id`, `name`, `email`, `tag` FROM `rows` LIMIT 1000 OFFSET 5", gotSQL)
	assert.Equal(t, []any{}, gotArgs)
}

func TestGridSortDeterministic(t *testing.T) {
	type multi struct {
		Zeta  int    `db:"zeta" json:"a_zeta" grid:"sort,filter"`
		Alpha int    `db:"alpha" json:"z_alpha" grid:"sort,filter"`
		Mid   string `db:"mid" json:"m_mid" grid:"sort,filter"`
	}
	g, err := NewGrid[multi]()
	require.NoError(t, err)
	base := gohan.Select("zeta", "alpha", "mid").From("multi")
	q := &GridQuery{
		FilterFields: map[string]any{"a_zeta": float64(1), "z_alpha": float64(2), "m_mid": "x"},
		SortFields:   map[string]string{"a_zeta": "asc", "z_alpha": "desc", "m_mid": "asc"},
	}

	var first string
	var firstArgs []any
	for i := 0; i < 300; i++ {
		sb, err := g.Build(base, q)
		require.NoError(t, err)
		gotSQL, gotArgs, err := sb.Build(gohan.Postgres())
		require.NoError(t, err)
		if i == 0 {
			first = gotSQL
			firstArgs = gotArgs
			continue
		}
		assert.Equal(t, first, gotSQL)
		assert.Equal(t, firstArgs, gotArgs)
	}
	// aliases (a_zeta, m_mid, z_alpha) sort differently than db names
	// (alpha, mid, zeta), so this also proves iteration is by alias.
	assert.Contains(t, first, `"zeta" = $1 AND "mid" = $2 AND "alpha" = $3`)
	assert.Contains(t, first, `ORDER BY "zeta" ASC, "mid" ASC, "alpha" DESC`)
}

func TestGridFilterFunc(t *testing.T) {
	g, err := NewGrid[row]()
	require.NoError(t, err)
	g.AddFilterFunc("tag", func(v any) (any, error) {
		if v == "yes" {
			return true, nil
		}
		return nil, errors.New("boom")
	})

	sb, err := g.Build(rowBase(), &GridQuery{FilterFields: map[string]any{"tag": "yes"}})
	require.NoError(t, err)
	gotSQL, gotArgs, err := sb.Build(gohan.Postgres())
	require.NoError(t, err)
	assert.Equal(t, `SELECT "id", "name", "email", "tag" FROM "rows" WHERE "tag" = $1 LIMIT 1000 OFFSET 0`, gotSQL)
	assert.Equal(t, []any{true}, gotArgs)

	_, err = g.Build(rowBase(), &GridQuery{FilterFields: map[string]any{"tag": "no"}})
	require.Error(t, err)
	var gerr GridError
	assert.False(t, errors.As(err, &gerr))
}

func TestGridValueCaps(t *testing.T) {
	t.Run("too many filter values", func(t *testing.T) {
		list := make([]any, MaxFilterValues+1)
		for i := range list {
			list[i] = float64(i)
		}
		_, _, err := buildRow(t, &GridQuery{FilterFields: map[string]any{"id": list}})
		require.Error(t, err)
		var gerr GridError
		require.True(t, errors.As(err, &gerr))
		assert.Equal(t, "value is not valid", gerr.Message)
	})

	t.Run("search text too long", func(t *testing.T) {
		text := make([]byte, MaxSearchText+1)
		for i := range text {
			text[i] = 'a'
		}
		_, _, err := buildRow(t, &GridQuery{SearchType: SearchAny, SearchText: string(text)})
		require.Error(t, err)
		var gerr GridError
		require.True(t, errors.As(err, &gerr))
		assert.Equal(t, "search text too long", gerr.Message)
	})
}

func TestGridMaxLimit(t *testing.T) {
	g, err := NewGrid[row]()
	require.NoError(t, err)
	g.WithMaxLimit(5)

	sb, err := g.Build(rowBase(), &GridQuery{})
	require.NoError(t, err)
	gotSQL, _, err := sb.Build(gohan.Postgres())
	require.NoError(t, err)
	assert.Contains(t, gotSQL, "LIMIT 5")

	sb, err = g.Build(rowBase(), &GridQuery{Limit: 100})
	require.NoError(t, err)
	gotSQL, _, err = sb.Build(gohan.Postgres())
	require.NoError(t, err)
	assert.Contains(t, gotSQL, "LIMIT 5")

	sb, err = g.Build(rowBase(), &GridQuery{Limit: 3})
	require.NoError(t, err)
	gotSQL, _, err = sb.Build(gohan.Postgres())
	require.NoError(t, err)
	assert.Contains(t, gotSQL, "LIMIT 3")
}

func TestGridNilInputs(t *testing.T) {
	g, err := NewGrid[row]()
	require.NoError(t, err)

	_, err = g.Build(rowBase(), nil)
	require.Error(t, err)
	var gerr GridError
	require.True(t, errors.As(err, &gerr))
	assert.Equal(t, GridError{Scope: "query", Message: "query is required"}, gerr)

	_, err = g.Build(nil, &GridQuery{})
	require.Error(t, err)
	require.True(t, errors.As(err, &gerr))
	assert.Equal(t, GridError{Scope: "query", Message: "base query is required"}, gerr)

	verr := g.ValidQuery(nil)
	require.Error(t, verr)
	require.True(t, errors.As(verr, &gerr))
	assert.Equal(t, GridError{Scope: "query", Message: "query is required"}, gerr)
}

func TestGridNoSearchableFields(t *testing.T) {
	type noSearch struct {
		ID  int `db:"id" json:"id" grid:"sort,filter"`
		Tag int `db:"tag" json:"tag" grid:"filter"`
	}
	g, err := NewGrid[noSearch]()
	require.NoError(t, err)
	base := gohan.Select("id", "tag").From("no_search")
	_, err = g.Build(base, &GridQuery{SearchType: SearchAny, SearchText: "x"})
	require.Error(t, err)
	var gerr GridError
	require.True(t, errors.As(err, &gerr))
	assert.Equal(t, GridError{Scope: "search", Message: "no searchable fields"}, gerr)

	// no search text: still valid
	_, err = g.Build(base, &GridQuery{SearchType: SearchAny})
	require.NoError(t, err)
}

func TestGridDefaultMaxLimit(t *testing.T) {
	g, err := NewGrid[row]()
	require.NoError(t, err)

	cases := []struct {
		name  string
		limit uint
		want  string
	}{
		{"zero limit is capped", 0, "LIMIT 1000"},
		{"large limit is capped", 5000, "LIMIT 1000"},
		{"small limit kept", 10, "LIMIT 10"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sb, err := g.Build(rowBase(), &GridQuery{Limit: c.limit})
			require.NoError(t, err)
			gotSQL, _, err := sb.Build(gohan.Postgres())
			require.NoError(t, err)
			assert.Contains(t, gotSQL, c.want)
		})
	}

	t.Run("WithMaxLimit(0) removes the cap", func(t *testing.T) {
		g, err := NewGrid[row]()
		require.NoError(t, err)
		sb, err := g.WithMaxLimit(0).Build(rowBase(), &GridQuery{})
		require.NoError(t, err)
		gotSQL, _, err := sb.Build(gohan.Postgres())
		require.NoError(t, err)
		assert.NotContains(t, gotSQL, "LIMIT")
	})
}

func TestGridFilterFuncResultValidated(t *testing.T) {
	build := func(t *testing.T, fn GridFilterFunc) (string, []any, error) {
		t.Helper()
		g, err := NewGrid[row]()
		require.NoError(t, err)
		g.AddFilterFunc("tag", fn)
		sb, err := g.WithMaxLimit(0).Build(rowBase(), &GridQuery{FilterFields: map[string]any{"tag": "x"}})
		if err != nil {
			return "", nil, err
		}
		return sb.Build(gohan.Postgres())
	}

	t.Run("expression result is rejected", func(t *testing.T) {
		_, _, err := build(t, func(any) (any, error) { return gohan.Raw("1=1"), nil })
		require.Error(t, err)
		var gerr GridError
		require.True(t, errors.As(err, &gerr))
		assert.Equal(t, GridError{Scope: "filter", Field: "tag", Message: "value is not valid"}, gerr)
	})

	t.Run("map result is rejected", func(t *testing.T) {
		_, _, err := build(t, func(any) (any, error) { return map[string]any{"a": 1}, nil })
		var gerr GridError
		require.True(t, errors.As(err, &gerr))
	})

	t.Run("typed slice result becomes IN", func(t *testing.T) {
		gotSQL, gotArgs, err := build(t, func(any) (any, error) { return []string{"a", "b"}, nil })
		require.NoError(t, err)
		assert.Equal(t, `SELECT "id", "name", "email", "tag" FROM "rows" WHERE "tag" IN ($1, $2)`, gotSQL)
		assert.Equal(t, []any{"a", "b"}, gotArgs)
	})

	t.Run("too many slice elements is rejected", func(t *testing.T) {
		_, _, err := build(t, func(any) (any, error) { return make([]int, MaxFilterValues+1), nil })
		var gerr GridError
		require.True(t, errors.As(err, &gerr))
	})

	t.Run("non-JSON scalar result is accepted", func(t *testing.T) {
		ts := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
		gotSQL, gotArgs, err := build(t, func(any) (any, error) { return ts, nil })
		require.NoError(t, err)
		assert.Equal(t, `SELECT "id", "name", "email", "tag" FROM "rows" WHERE "tag" = $1`, gotSQL)
		assert.Equal(t, []any{ts}, gotArgs)
	})

	t.Run("byte slice result stays scalar", func(t *testing.T) {
		gotSQL, _, err := build(t, func(any) (any, error) { return []byte("ab"), nil })
		require.NoError(t, err)
		assert.Contains(t, gotSQL, `"tag" = $1`)
	})
}

func TestGridQueryPageOverflow(t *testing.T) {
	q := &GridQuery{}
	q.Page(math.MaxInt, 1000)
	assert.Equal(t, uint(math.MaxInt), q.Offset)
	assert.Equal(t, uint(1000), q.Limit)

	g, err := NewGrid[row]()
	require.NoError(t, err)
	sb, err := g.Build(rowBase(), q)
	require.NoError(t, err)
	_, _, err = sb.Build(gohan.Postgres())
	require.NoError(t, err)
}

func TestGridRejectsOutOfRangePaging(t *testing.T) {
	g, err := NewGrid[row]()
	require.NoError(t, err)
	big := uint64(math.MaxInt64) + 1
	for _, q := range []*GridQuery{
		{Offset: uint(big)},
		{Limit: uint(big)},
	} {
		err := g.ValidQuery(q)
		var gerr GridError
		require.True(t, errors.As(err, &gerr), "%+v", q)
		assert.Equal(t, "query", gerr.Scope)
	}
}

func TestGridOrderedSort(t *testing.T) {
	t.Run("caller order is kept", func(t *testing.T) {
		gotSQL, _, err := buildRow(t, &GridQuery{Sort: []SortField{{Field: "name", Order: "asc"}, {Field: "id", Order: "desc"}}})
		require.NoError(t, err)
		assert.Contains(t, gotSQL, `ORDER BY "name" ASC, "id" DESC`)
	})

	t.Run("default order is desc", func(t *testing.T) {
		gotSQL, _, err := buildRow(t, &GridQuery{Sort: []SortField{{Field: "name"}}})
		require.NoError(t, err)
		assert.Contains(t, gotSQL, `ORDER BY "name" DESC`)
	})

	t.Run("json array decodes", func(t *testing.T) {
		var q GridQuery
		require.NoError(t, json.Unmarshal([]byte(`{"sort":[{"field":"id","order":"asc"},{"field":"name","order":"desc"}]}`), &q))
		gotSQL, _, err := buildRow(t, &q)
		require.NoError(t, err)
		assert.Contains(t, gotSQL, `ORDER BY "id" ASC, "name" DESC`)
	})

	errCases := []struct {
		name string
		q    *GridQuery
		want GridError
	}{
		{"unknown field", &GridQuery{Sort: []SortField{{Field: "nope"}}}, GridError{Scope: "sort", Field: "nope", Message: "field is not valid"}},
		{"not sortable", &GridQuery{Sort: []SortField{{Field: "tag"}}}, GridError{Scope: "sort", Field: "tag", Message: "field is not sortable"}},
		{"bad order", &GridQuery{Sort: []SortField{{Field: "id", Order: "sideways"}}}, GridError{Scope: "sort", Field: "id", Message: "sort order is not valid"}},
		{"duplicate field", &GridQuery{Sort: []SortField{{Field: "id"}, {Field: "id", Order: "asc"}}}, GridError{Scope: "sort", Field: "id", Message: "field is repeated"}},
		{"both sort forms", &GridQuery{Sort: []SortField{{Field: "id"}}, SortFields: map[string]string{"name": "asc"}}, GridError{Scope: "sort", Message: "use either sort or sortFields, not both"}},
	}
	for _, c := range errCases {
		t.Run(c.name, func(t *testing.T) {
			_, _, err := buildRow(t, c.q)
			var gerr GridError
			require.True(t, errors.As(err, &gerr), "err=%v", err)
			assert.Equal(t, c.want, gerr)
		})
	}
}

func TestGridTiebreaker(t *testing.T) {
	newGrid := func(t *testing.T, cols ...string) *Grid[row] {
		t.Helper()
		g, err := NewGrid[row]()
		require.NoError(t, err)
		return g.WithTiebreaker(cols...)
	}
	build := func(t *testing.T, g *Grid[row], q *GridQuery) (string, error) {
		t.Helper()
		sb, err := g.Build(rowBase(), q)
		if err != nil {
			return "", err
		}
		gotSQL, _, err := sb.Build(gohan.Postgres())
		return gotSQL, err
	}

	t.Run("appended after client sort", func(t *testing.T) {
		gotSQL, err := build(t, newGrid(t, "id"), &GridQuery{SortFields: map[string]string{"name": "asc"}})
		require.NoError(t, err)
		assert.Contains(t, gotSQL, `ORDER BY "name" ASC, "id" ASC`)
	})

	t.Run("used alone without client sort", func(t *testing.T) {
		gotSQL, err := build(t, newGrid(t, "id"), &GridQuery{})
		require.NoError(t, err)
		assert.Contains(t, gotSQL, `ORDER BY "id" ASC`)
	})

	t.Run("not repeated when already sorted", func(t *testing.T) {
		gotSQL, err := build(t, newGrid(t, "id"), &GridQuery{Sort: []SortField{{Field: "id", Order: "desc"}}})
		require.NoError(t, err)
		assert.Contains(t, gotSQL, `ORDER BY "id" DESC LIMIT`)
	})

	t.Run("any mapped column may be used", func(t *testing.T) {
		gotSQL, err := build(t, newGrid(t, "note"), &GridQuery{})
		require.NoError(t, err)
		assert.Contains(t, gotSQL, `ORDER BY "note" ASC`)
	})

	t.Run("unmapped column is an error", func(t *testing.T) {
		_, err := build(t, newGrid(t, "nope"), &GridQuery{})
		require.Error(t, err)
		var gerr GridError
		assert.False(t, errors.As(err, &gerr), "configuration error, not a client error")
	})
}

// parityRow is used only here, to compare dbx's ValidQuery against
// db.Grid's, for everything not a documented, intentional divergence. It
// must never be reused elsewhere in the package: db.Grid's spec cache used to
// be keyed by t.Name(), so a second type named parityRow would have shared its
// spec.
type parityRow struct {
	ID    int    `db:"id" json:"id" grid:"sort,filter"`
	Name  string `db:"name" json:"name" grid:"search,sort"`
	Email string `db:"email" json:"email" grid:"search"`
	Tag   string `db:"tag" json:"tag" grid:"filter"`
	Note  string `db:"note" json:"note"`
}

func toDbGridQuery(q *GridQuery) *db.GridQuery {
	return &db.GridQuery{
		SearchType:   q.SearchType,
		SearchText:   q.SearchText,
		FilterFields: q.FilterFields,
		SortFields:   q.SortFields,
		Offset:       q.Offset,
		Limit:        q.Limit,
	}
}

func TestGridValidQueryParity(t *testing.T) {
	dg, err := NewGrid[parityRow]()
	require.NoError(t, err)
	bg, err := db.NewGrid("parity_rows", &parityRow{})
	require.NoError(t, err)

	cases := []struct {
		name string
		q    *GridQuery
	}{
		{"empty", &GridQuery{}},
		{"valid scalar filter", &GridQuery{FilterFields: map[string]any{"tag": "a"}}},
		{"valid int filter", &GridQuery{FilterFields: map[string]any{"id": float64(3)}}},
		{"filter not filterable", &GridQuery{FilterFields: map[string]any{"name": "x"}}},
		{"nil filter value", &GridQuery{FilterFields: map[string]any{"tag": nil}}},
		{"valid list filter", &GridQuery{FilterFields: map[string]any{"id": []any{float64(1), float64(2)}}}},
		{"valid sort", &GridQuery{SortFields: map[string]string{"id": "asc"}}},
		{"invalid sort order", &GridQuery{SortFields: map[string]string{"id": "sideways"}}},
		{"sort not sortable", &GridQuery{SortFields: map[string]string{"tag": "asc"}}},
		{"valid search", &GridQuery{SearchType: SearchAny, SearchText: "x"}},
		{"search not allowed", &GridQuery{SearchText: "x", SearchType: SearchNone}},
		{"filter+sort multi-scope", &GridQuery{FilterFields: map[string]any{"tag": "a"}, SortFields: map[string]string{"id": "sideways"}}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dxErr := dg.ValidQuery(c.q)
			dbErr := bg.ValidQuery(toDbGridQuery(c.q))

			if dbErr == nil {
				assert.NoError(t, dxErr)
				return
			}
			require.Error(t, dxErr)
			var dxGridErr, dbGridErr GridError
			var dbErrTyped db.GridError
			require.True(t, errors.As(dbErr, &dbErrTyped), "db error must be a GridError")
			// dbx's GridError and db's GridError have the same shape; compare
			// field by field rather than by type.
			require.True(t, errors.As(dxErr, &dxGridErr))
			dbGridErr = GridError{Scope: dbErrTyped.Scope, Field: dbErrTyped.Field, Message: dbErrTyped.Message}
			assert.Equal(t, dbGridErr, dxGridErr)
		})
	}
}

// uuidLike mimics uuid.UUID: a byte array that is a driver.Valuer.
type uuidLike [16]byte

func (u uuidLike) Value() (driver.Value, error) { return u[:], nil }

// pgArray mimics pq.StringArray: a slice that is a driver.Valuer.
type pgArray []string

func (a pgArray) Value() (driver.Value, error) { return "{a,b}", nil }

func TestGridFilterFuncResultShapes(t *testing.T) {
	build := func(t *testing.T, v any) (string, []any, error) {
		t.Helper()
		g, err := NewGrid[row]()
		require.NoError(t, err)
		g.AddFilterFunc("tag", func(any) (any, error) { return v, nil })
		sb, err := g.WithMaxLimit(0).Build(rowBase(), &GridQuery{FilterFields: map[string]any{"tag": "x"}})
		if err != nil {
			return "", nil, err
		}
		return sb.Build(gohan.Postgres())
	}
	const eq = `SELECT "id", "name", "email", "tag" FROM "rows" WHERE "tag" = $1`
	const in2 = `SELECT "id", "name", "email", "tag" FROM "rows" WHERE "tag" IN ($1, $2)`

	scalars := []struct {
		name string
		v    any
	}{
		{"byte-array Valuer", uuidLike{1, 2, 3}},
		{"slice Valuer", pgArray{"a", "b"}},
		{"named byte slice", json.RawMessage(`{"a":1}`)},
		{"net.IP", net.IPv4(10, 0, 0, 1).To4()},
		{"plain array", [2]int{1, 2}},
	}
	for _, c := range scalars {
		t.Run(c.name+" binds as one value", func(t *testing.T) {
			gotSQL, gotArgs, err := build(t, c.v)
			require.NoError(t, err)
			assert.Equal(t, eq, gotSQL)
			assert.Equal(t, []any{c.v}, gotArgs)
		})
	}

	t.Run("list of byte-array Valuers becomes IN", func(t *testing.T) {
		gotSQL, gotArgs, err := build(t, []any{uuidLike{1}, uuidLike{2}})
		require.NoError(t, err)
		assert.Equal(t, in2, gotSQL)
		assert.Equal(t, []any{uuidLike{1}, uuidLike{2}}, gotArgs)
	})

	t.Run("typed slice of byte-array Valuers becomes IN", func(t *testing.T) {
		gotSQL, _, err := build(t, []uuidLike{{1}, {2}})
		require.NoError(t, err)
		assert.Equal(t, in2, gotSQL)
	})

	t.Run("subquery in a list is rejected", func(t *testing.T) {
		_, _, err := build(t, []any{gohan.Select("id").From("other")})
		var gerr GridError
		require.True(t, errors.As(err, &gerr), "err=%v", err)
	})

	t.Run("subquery is rejected", func(t *testing.T) {
		_, _, err := build(t, gohan.Select("id").From("other"))
		var gerr GridError
		require.True(t, errors.As(err, &gerr), "err=%v", err)
	})
}

func TestGridTiebreakerConfig(t *testing.T) {
	t.Run("repeated column is emitted once", func(t *testing.T) {
		g, err := NewGrid[row]()
		require.NoError(t, err)
		sb, err := g.WithTiebreaker("id", "id").Build(rowBase(), &GridQuery{})
		require.NoError(t, err)
		gotSQL, _, err := sb.Build(gohan.Postgres())
		require.NoError(t, err)
		assert.Contains(t, gotSQL, `ORDER BY "id" ASC LIMIT`)
	})

	t.Run("a later valid call replaces an invalid one", func(t *testing.T) {
		g, err := NewGrid[row]()
		require.NoError(t, err)
		_, err = g.WithTiebreaker("bogus").WithTiebreaker("id").Build(rowBase(), &GridQuery{})
		require.NoError(t, err)
	})

	t.Run("ValidQuery reports a configuration error", func(t *testing.T) {
		g, err := NewGrid[row]()
		require.NoError(t, err)
		err = g.WithTiebreaker("bogus").ValidQuery(&GridQuery{})
		require.Error(t, err)
		var gerr GridError
		assert.False(t, errors.As(err, &gerr))
	})

	t.Run("max limit above MaxInt64 is a configuration error", func(t *testing.T) {
		g, err := NewGrid[row]()
		require.NoError(t, err)
		big := uint64(math.MaxInt64) + 1
		g.WithMaxLimit(uint(big))
		require.Error(t, g.ValidQuery(&GridQuery{}))
		_, err = g.Build(rowBase(), &GridQuery{})
		require.Error(t, err)
		var gerr GridError
		assert.False(t, errors.As(err, &gerr))
	})
}

func TestGridCaseInsensitiveSearch(t *testing.T) {
	build := func(t *testing.T, g *Grid[row], d gohan.Dialect, st uint) string {
		t.Helper()
		sb, err := g.Build(rowBase(), &GridQuery{SearchType: st, SearchText: "Ali"})
		require.NoError(t, err)
		gotSQL, _, err := sb.Build(d)
		require.NoError(t, err)
		return gotSQL
	}
	newGrid := func(t *testing.T) *Grid[row] {
		g, err := NewGrid[row]()
		require.NoError(t, err)
		return g
	}

	t.Run("default search is case-sensitive LIKE", func(t *testing.T) {
		assert.Contains(t, build(t, newGrid(t), gohan.Postgres(), SearchAny), `"name" LIKE $1`)
	})

	cases := []struct {
		name string
		d    gohan.Dialect
		st   uint
		want string
	}{
		{"postgres any", gohan.Postgres(), SearchAny, `"name" ILIKE $1`},
		{"postgres start", gohan.Postgres(), SearchStart, `"name" ILIKE $1`},
		{"postgres end", gohan.Postgres(), SearchEnd, `"name" ILIKE $1`},
		// SQLite renders LIKE with or without folding (its LIKE already ignores
		// ASCII case), so this case pins the rendering; it cannot detect
		// WithCaseInsensitiveSearch being ignored.
		{"sqlite", gohan.SQLite(), SearchAny, "`name` LIKE ?"},
		{"clickhouse", gohan.ClickHouse(), SearchAny, `"name" ILIKE ?`},
	}
	for _, c := range cases {
		t.Run("fold "+c.name, func(t *testing.T) {
			assert.Contains(t, build(t, newGrid(t).WithCaseInsensitiveSearch(), c.d, c.st), c.want)
		})
	}

	t.Run("fold keeps prefix and suffix semantics", func(t *testing.T) {
		g := newGrid(t).WithCaseInsensitiveSearch()
		for st, want := range map[uint]string{SearchStart: "Ali%", SearchEnd: "%Ali", SearchAny: "%Ali%"} {
			sb, err := g.Build(rowBase(), &GridQuery{SearchType: st, SearchText: "Ali"})
			require.NoError(t, err)
			_, args, err := sb.Build(gohan.Postgres())
			require.NoError(t, err)
			assert.Equal(t, want, args[0], "search type %d", st)
		}
	})
}
