package dbx

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/oddbit-project/blueprint/db"
	"github.com/oddbit-project/blueprint/sqlb"
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

func rowBase() *sqlb.SelectBuilder {
	return sqlb.Select("id", "name", "email", "tag").From("rows")
}

func buildRow(t *testing.T, q *GridQuery) (string, []any, error) {
	t.Helper()
	g, err := NewGrid[row]()
	require.NoError(t, err)
	sb, err := g.Build(rowBase(), q)
	if err != nil {
		return "", nil, err
	}
	return sb.Build(sqlb.Postgres())
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
	compound := rowBase().Union(sqlb.Select("id", "name", "email", "tag").From("more_rows"))
	_, err = g.Build(compound, &GridQuery{})
	require.Error(t, err)
	var gerr GridError
	require.True(t, errors.As(err, &gerr))
	assert.Equal(t, GridError{Scope: "query", Message: "base query must not be a UNION; wrap it with sqlb.From(q.As(...))"}, gerr)
}

func TestGridCompoundBaseWorkaround(t *testing.T) {
	g, err := NewGrid[row]()
	require.NoError(t, err)
	compound := sqlb.Select("id", "name", "email", "tag").From("rows").
		Union(sqlb.Select("id", "name", "email", "tag").From("more_rows"))
	base := sqlb.From(compound.As("u"))
	sb, err := g.Build(base, &GridQuery{FilterFields: map[string]any{"tag": "a"}})
	require.NoError(t, err)
	gotSQL, gotArgs, err := sb.Build(sqlb.Postgres())
	require.NoError(t, err)
	assert.Equal(t,
		`SELECT * FROM (SELECT "id", "name", "email", "tag" FROM "rows" UNION SELECT "id", "name", "email", "tag" FROM "more_rows") AS "u" WHERE "tag" = $1`,
		gotSQL)
	assert.Equal(t, []any{"a"}, gotArgs)
}

// TestGridBuildGoldenSQLite proves the SQLite-specific "LIMIT -1 OFFSET n"
// rendering for an offset-only query, exercised through sqlb rather than
// re-implemented in Build.
func TestGridBuildGoldenSQLite(t *testing.T) {
	g, err := NewGrid[row]()
	require.NoError(t, err)
	sb, err := g.Build(rowBase(), &GridQuery{Offset: 5})
	require.NoError(t, err)
	gotSQL, gotArgs, err := sb.Build(sqlb.SQLite())
	require.NoError(t, err)
	assert.Equal(t, "SELECT `id`, `name`, `email`, `tag` FROM `rows` LIMIT -1 OFFSET 5", gotSQL)
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
	base := sqlb.Select("zeta", "alpha", "mid").From("multi")
	q := &GridQuery{
		FilterFields: map[string]any{"a_zeta": float64(1), "z_alpha": float64(2), "m_mid": "x"},
		SortFields:   map[string]string{"a_zeta": "asc", "z_alpha": "desc", "m_mid": "asc"},
	}

	var first string
	var firstArgs []any
	for i := 0; i < 300; i++ {
		sb, err := g.Build(base, q)
		require.NoError(t, err)
		gotSQL, gotArgs, err := sb.Build(sqlb.Postgres())
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
	gotSQL, gotArgs, err := sb.Build(sqlb.Postgres())
	require.NoError(t, err)
	assert.Equal(t, `SELECT "id", "name", "email", "tag" FROM "rows" WHERE "tag" = $1`, gotSQL)
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
	gotSQL, _, err := sb.Build(sqlb.Postgres())
	require.NoError(t, err)
	assert.Contains(t, gotSQL, "LIMIT 5")

	sb, err = g.Build(rowBase(), &GridQuery{Limit: 100})
	require.NoError(t, err)
	gotSQL, _, err = sb.Build(sqlb.Postgres())
	require.NoError(t, err)
	assert.Contains(t, gotSQL, "LIMIT 5")

	sb, err = g.Build(rowBase(), &GridQuery{Limit: 3})
	require.NoError(t, err)
	gotSQL, _, err = sb.Build(sqlb.Postgres())
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
	base := sqlb.Select("id", "tag").From("no_search")
	sb, err := g.Build(base, &GridQuery{SearchType: SearchAny, SearchText: "x"})
	require.NoError(t, err)
	gotSQL, gotArgs, err := sb.Build(sqlb.Postgres())
	require.NoError(t, err)
	assert.Equal(t, `SELECT "id", "tag" FROM "no_search"`, gotSQL)
	assert.Equal(t, []any{}, gotArgs)
}

// parityRow is used only here, to compare dbx's ValidQuery against
// db.Grid's, for everything not a documented, intentional divergence. It
// must never be reused elsewhere in the package: db.Grid's spec cache is
// keyed by t.Name(), so a second, differently-shaped type named parityRow
// anywhere else in this package would silently corrupt this test.
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
