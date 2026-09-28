package dbx

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/oddbit-project/gohan"
)

// ksGridRow is the grid record type for the keyset grid tests.
type ksGridRow struct {
	ID    int64   `db:"id" json:"id" grid:"sort,filter"`
	Name  string  `db:"name" json:"name" grid:"search,sort"`
	Score float64 `db:"score" json:"score" grid:"sort"`
	Tag   string  `db:"tag" json:"tag" grid:"filter"`
	Ratio float64 `db:"ratio" json:"ratio"`
}

// ksWideRow has more sortable fields than MaxKeysetKeys.
type ksWideRow struct {
	ID int64 `db:"id" json:"id" grid:"sort"`
	A  int64 `db:"a" json:"a" grid:"sort"`
	B  int64 `db:"b" json:"b" grid:"sort"`
	C  int64 `db:"c" json:"c" grid:"sort"`
	D  int64 `db:"d" json:"d" grid:"sort"`
	E  int64 `db:"e" json:"e" grid:"sort"`
	F  int64 `db:"f" json:"f" grid:"sort"`
	G  int64 `db:"g" json:"g" grid:"sort"`
	H  int64 `db:"h" json:"h" grid:"sort"`
}

func ksGrid(t *testing.T) *Grid[ksGridRow] {
	t.Helper()
	g, err := NewGrid[ksGridRow]()
	require.NoError(t, err)
	return g.WithTiebreaker("id")
}

func ksGridRepo(t *testing.T, d gohan.Dialect) (*Repository[ksGridRow], *recordingQuerier) {
	t.Helper()
	rq := &recordingQuerier{d: d}
	r, err := NewRepository[ksGridRow](rq, "rows")
	require.NoError(t, err)
	return r, rq
}

// gridKeysetSQL runs QueryGridKeyset and returns the statement it issued.
func gridKeysetSQL(t *testing.T, g *Grid[ksGridRow], q *GridQuery, cursor string) (string, []any) {
	t.Helper()
	r, rq := ksGridRepo(t, gohan.Postgres())
	_, err := r.QueryGridKeyset(context.Background(), g, q, cursor)
	require.NoError(t, err)
	require.Len(t, rq.calls, 1)
	return rq.calls[0].sql, rq.calls[0].args
}

// orderBy extracts the ORDER BY list of a statement.
func orderBy(t *testing.T, sqlStr string) string {
	t.Helper()
	_, rest, ok := strings.Cut(sqlStr, " ORDER BY ")
	require.True(t, ok, sqlStr)
	list, _, _ := strings.Cut(rest, " LIMIT ")
	return list
}

// gridCursor mints the cursor QueryGridKeyset would return after a row with
// the given key values, for q's sort.
func gridCursor(t *testing.T, g *Grid[ksGridRow], q *GridQuery, vals ...any) string {
	t.Helper()
	r, _ := ksGridRepo(t, gohan.Postgres())
	keys := make([]KeysetKey, 0)
	for _, k := range g.orderKeys(q) {
		keys = append(keys, k.KeysetKey)
	}
	return ksCursor(t, r, keys, vals...)
}

func TestGridKeysetOffsetRejected(t *testing.T) {
	for _, q := range []*GridQuery{{Offset: 1}, {Offset: 100, Limit: 10}} {
		cq := &countingQuerier{d: gohan.Postgres()}
		r, err := NewRepository[ksGridRow](cq, "rows")
		require.NoError(t, err)
		_, err = r.QueryGridKeyset(context.Background(), ksGrid(t), q, "")
		assert.Equal(t, GridError{Scope: "query", Message: "offset is not allowed with cursor pagination"}, err)
		assert.Zero(t, cq.calls)
	}

	t.Run("Page(1, n) is allowed", func(t *testing.T) {
		q := &GridQuery{}
		q.Page(1, 5)
		sqlStr, _ := gridKeysetSQL(t, ksGrid(t), q, "")
		assert.True(t, strings.HasSuffix(sqlStr, "LIMIT 6"), sqlStr)
		assert.NotContains(t, sqlStr, "OFFSET")
	})
}

func TestGridKeysetNoTiebreaker(t *testing.T) {
	g, err := NewGrid[ksGridRow]()
	require.NoError(t, err)
	cq := &countingQuerier{d: gohan.Postgres()}
	r, err := NewRepository[ksGridRow](cq, "rows")
	require.NoError(t, err)

	_, err = r.QueryGridKeyset(context.Background(), g, &GridQuery{Sort: []SortField{{Field: "id"}}}, "")
	require.ErrorIs(t, err, ErrInvalidKeysetKey)
	assert.Contains(t, err.Error(), "WithTiebreaker")
	var gerr GridError
	assert.False(t, errors.As(err, &gerr))
	assert.Zero(t, cq.calls)
}

func TestGridKeysetIneligibleKeys(t *testing.T) {
	cq := &countingQuerier{d: gohan.Postgres()}
	r, err := NewRepository[ksGridRow](cq, "rows")
	require.NoError(t, err)

	t.Run("client sort", func(t *testing.T) {
		_, err := r.QueryGridKeyset(context.Background(), ksGrid(t), &GridQuery{Sort: []SortField{{Field: "score"}}}, "")
		assert.Equal(t, GridError{Scope: "sort", Field: "score", Message: "field cannot be used with cursor pagination"}, err)
	})

	t.Run("tiebreaker", func(t *testing.T) {
		g, err := NewGrid[ksGridRow]()
		require.NoError(t, err)
		_, err = r.QueryGridKeyset(context.Background(), g.WithTiebreaker("ratio"), &GridQuery{}, "")
		require.ErrorIs(t, err, ErrInvalidKeysetKey)
		var gerr GridError
		assert.False(t, errors.As(err, &gerr))
	})
	assert.Zero(t, cq.calls)
}

func TestGridKeysetOrderParity(t *testing.T) {
	queries := map[string]*GridQuery{
		"no sort":              {},
		"default desc":         {Sort: []SortField{{Field: "name"}}},
		"asc":                  {Sort: []SortField{{Field: "name", Order: SortAscending}}},
		"desc then asc":        {Sort: []SortField{{Field: "name", Order: SortDescending}, {Field: "id", Order: SortAscending}}},
		"map form":             {SortFields: map[string]string{"name": "asc", "id": "desc"}},
		"sort on tiebreaker":   {Sort: []SortField{{Field: "id", Order: SortDescending}}},
		"tiebreaker then name": {Sort: []SortField{{Field: "id"}, {Field: "name"}}},
	}
	for name, q := range queries {
		t.Run(name, func(t *testing.T) {
			g := ksGrid(t)
			sb, err := g.Build(gohan.Select("id").From("rows"), q)
			require.NoError(t, err)
			buildSQL, _, err := sb.Build(gohan.Postgres())
			require.NoError(t, err)

			keysetSQL, _ := gridKeysetSQL(t, g, q, "")
			assert.Equal(t, orderBy(t, buildSQL), orderBy(t, keysetSQL))
		})
	}

	t.Run("two tiebreaker columns", func(t *testing.T) {
		g := ksGrid(t).WithTiebreaker("name", "id")
		keysetSQL, _ := gridKeysetSQL(t, g, &GridQuery{Sort: []SortField{{Field: "name"}}}, "")
		assert.Equal(t, `"name" DESC, "id" ASC`, orderBy(t, keysetSQL))
	})
}

func TestGridKeysetFiltersAndSeek(t *testing.T) {
	g := ksGrid(t)
	q := &GridQuery{
		FilterFields: map[string]any{"tag": "a"},
		SearchType:   SearchAny,
		SearchText:   "bo",
		Sort:         []SortField{{Field: "name", Order: SortAscending}},
		Limit:        5,
	}
	cursor := gridCursor(t, g, q, "bob", int64(7))
	sqlStr, args := gridKeysetSQL(t, g, q, cursor)
	assert.Equal(t, `SELECT "id", "name", "score", "tag", "ratio" FROM "rows" WHERE ("tag" = $1 AND "name" LIKE $2 ESCAPE '!' AND ("name" >= $3 AND ("name" > $4 OR ("name" = $5 AND "id" > $6)))) ORDER BY "name" ASC, "id" ASC LIMIT 6`, sqlStr)
	assert.Equal(t, []any{"a", "%bo%", "bob", "bob", "bob", int64(7)}, args)
}

func TestGridKeysetCursorErrors(t *testing.T) {
	g := ksGrid(t)
	byName := &GridQuery{Sort: []SortField{{Field: "name", Order: SortAscending}}}
	cursor := gridCursor(t, g, byName, "bob", int64(7))

	cases := map[string]struct {
		q      *GridQuery
		cursor string
	}{
		"other sort field":     {&GridQuery{Sort: []SortField{{Field: "id", Order: SortAscending}}}, cursor},
		"other sort direction": {&GridQuery{Sort: []SortField{{Field: "name", Order: SortDescending}}}, cursor},
		"garbage":              {byName, "garbage!"},
		"too large":            {byName, strings.Repeat("A", MaxCursorBytes+1)},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			cq := &countingQuerier{d: gohan.Postgres()}
			r, err := NewRepository[ksGridRow](cq, "rows")
			require.NoError(t, err)
			_, err = r.QueryGridKeyset(context.Background(), g, c.q, c.cursor)
			assert.Equal(t, GridError{Scope: "cursor", Message: "cursor is not valid"}, err)
			require.ErrorIs(t, err, ErrInvalidCursor)
			body, jerr := json.Marshal(err)
			require.NoError(t, jerr)
			assert.JSONEq(t, `{"scope":"cursor","field":"","message":"cursor is not valid"}`, string(body))
			assert.Zero(t, cq.calls)
		})
	}

	t.Run("changed filters keep the cursor", func(t *testing.T) {
		q := &GridQuery{Sort: byName.Sort, FilterFields: map[string]any{"tag": "b"}, Limit: 3}
		sqlStr, args := gridKeysetSQL(t, g, q, cursor)
		assert.Equal(t, `SELECT "id", "name", "score", "tag", "ratio" FROM "rows" WHERE ("tag" = $1 AND ("name" >= $2 AND ("name" > $3 OR ("name" = $4 AND "id" > $5)))) ORDER BY "name" ASC, "id" ASC LIMIT 4`, sqlStr)
		assert.Equal(t, []any{"b", "bob", "bob", "bob", int64(7)}, args)
	})
}

func TestGridErrorIs(t *testing.T) {
	assert.True(t, errors.Is(GridError{Scope: "cursor", Message: "cursor is not valid"}, ErrInvalidCursor))
	assert.False(t, errors.Is(GridError{Scope: "sort", Field: "x", Message: "field is not valid"}, ErrInvalidCursor))
	assert.False(t, errors.Is(GridError{Scope: "cursor"}, ErrInvalidKeysetKey))
	var gerr GridError
	assert.True(t, errors.As(error(GridError{Scope: "cursor"}), &gerr))
}

func TestGridKeysetLimit(t *testing.T) {
	cases := []struct {
		name  string
		max   *uint
		limit uint
		want  string
	}{
		{"default cap, no limit", nil, 0, "LIMIT 1001"},
		{"default cap, small limit", nil, 5, "LIMIT 6"},
		{"default cap, over the cap", nil, 5000, "LIMIT 1001"},
		{"own cap, no limit", ptr(uint(10)), 0, "LIMIT 11"},
		{"own cap, over", ptr(uint(10)), 50, "LIMIT 11"},
		{"no cap, no limit", ptr(uint(0)), 0, "LIMIT 101"},
		{"no cap, big limit", ptr(uint(0)), 5000, "LIMIT 5001"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			g := ksGrid(t)
			if c.max != nil {
				g.WithMaxLimit(*c.max)
			}
			sqlStr, _ := gridKeysetSQL(t, g, &GridQuery{Limit: c.limit}, "")
			assert.True(t, strings.HasSuffix(sqlStr, c.want), sqlStr)
		})
	}
}

// TestGridKeysetLimitOutOfRange checks that an uncapped grid rejects a
// client Limit whose look-ahead row (Limit+1) would not fit an int64 LIMIT,
// before any query.
func TestGridKeysetLimitOutOfRange(t *testing.T) {
	for _, limit := range []uint{math.MaxInt64, math.MaxUint64} {
		cq := &countingQuerier{d: gohan.Postgres()}
		r, err := NewRepository[ksGridRow](cq, "rows")
		require.NoError(t, err)
		g := ksGrid(t)
		g.WithMaxLimit(0)
		_, err = r.QueryGridKeyset(context.Background(), g, &GridQuery{Limit: limit}, "")
		assert.Equal(t, GridError{Scope: "query", Message: "limit is out of range"}, err, "limit %d", limit)
		assert.Zero(t, cq.calls)
	}

	t.Run("largest accepted limit", func(t *testing.T) {
		g := ksGrid(t)
		g.WithMaxLimit(0)
		sqlStr, _ := gridKeysetSQL(t, g, &GridQuery{Limit: math.MaxInt64 - 1}, "")
		assert.True(t, strings.HasSuffix(sqlStr, "LIMIT 9223372036854775807"), sqlStr)
	})
}

// TestGridKeysetLongStringKey pins what a stored string too long (or not
// valid UTF-8) for a cursor does to a client-chosen sort: the page fails
// with a server-side error, no panic and no silently broken pagination.
func TestGridKeysetLongStringKey(t *testing.T) {
	cols := []string{"id", "name", "score", "tag", "ratio"}
	sqlFor := `SELECT "id", "name", "score", "tag", "ratio" FROM "rows" ORDER BY "name" ASC, "id" ASC LIMIT 2`
	q := &GridQuery{Sort: []SortField{{Field: "name", Order: SortAscending}}, Limit: 1}
	cases := []struct {
		name    string
		key     string
		wantErr error
	}{
		{"too long for a cursor", strings.Repeat("x", MaxCursorBytes), ErrCursorTooLarge},
		{"not valid UTF-8", "a\xff", ErrInvalidKeysetKey},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sq, mock := newMockQuerier(t)
			r, err := NewRepository[ksGridRow](sq, "rows")
			require.NoError(t, err)
			mock.ExpectQuery(sqlFor).WillReturnRows(sqlmock.NewRows(cols).
				AddRow(int64(1), c.key, 0.0, "", 0.0).
				AddRow(int64(2), c.key+"z", 0.0, "", 0.0))

			var page *KeysetPage[ksGridRow]
			require.NotPanics(t, func() { page, err = r.QueryGridKeyset(context.Background(), ksGrid(t), q, "") })
			require.ErrorIs(t, err, c.wantErr)
			var gerr GridError
			assert.False(t, errors.As(err, &gerr), "a server-side failure, not a client error")
			assert.Nil(t, page)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestGridKeysetTooManyKeys(t *testing.T) {
	g, err := NewGrid[ksWideRow]()
	require.NoError(t, err)
	g.WithTiebreaker("id")
	cq := &countingQuerier{d: gohan.Postgres()}
	r, err := NewRepository[ksWideRow](cq, "wide")
	require.NoError(t, err)

	sorts := []SortField{{Field: "a"}, {Field: "b"}, {Field: "c"}, {Field: "d"}, {Field: "e"}, {Field: "f"}, {Field: "g"}}
	_, err = r.QueryGridKeyset(context.Background(), g, &GridQuery{Sort: sorts}, "")
	require.NoError(t, err, "seven sort fields plus the tiebreaker is the maximum")

	_, err = r.QueryGridKeyset(context.Background(), g, &GridQuery{Sort: append(sorts, SortField{Field: "h"})}, "")
	assert.Equal(t, GridError{Scope: "sort", Message: "too many sort fields for cursor pagination"}, err)
	assert.Equal(t, 1, cq.calls)
}

func TestGridKeysetInvalidQueryParity(t *testing.T) {
	queries := map[string]*GridQuery{
		"nil query":        nil,
		"unknown filter":   {FilterFields: map[string]any{"nope": 1}},
		"not filterable":   {FilterFields: map[string]any{"name": "x"}},
		"bad filter value": {FilterFields: map[string]any{"tag": map[string]any{"a": 1}}},
		"unknown sort":     {Sort: []SortField{{Field: "nope"}}},
		"bad order":        {Sort: []SortField{{Field: "name", Order: "up"}}},
		"both sort forms":  {Sort: []SortField{{Field: "name"}}, SortFields: map[string]string{"id": "asc"}},
		"search too long":  {SearchType: SearchAny, SearchText: strings.Repeat("x", MaxSearchText+1)},
		"search type":      {SearchType: 9},
		"search not set":   {SearchText: "x"},
		"limit range":      {Limit: math.MaxInt64 + 1},
	}
	for name, q := range queries {
		t.Run(name, func(t *testing.T) {
			cq := &countingQuerier{d: gohan.Postgres()}
			r, err := NewRepository[ksGridRow](cq, "rows")
			require.NoError(t, err)
			g := ksGrid(t)
			_, want := r.QueryGrid(context.Background(), g, q)
			require.Error(t, want)
			_, got := r.QueryGridKeyset(context.Background(), g, q, "")
			assert.Equal(t, want, got)
			assert.Zero(t, cq.calls)
		})
	}

	t.Run("config errors", func(t *testing.T) {
		cq := &countingQuerier{d: gohan.Postgres()}
		r, err := NewRepository[ksGridRow](cq, "rows")
		require.NoError(t, err)
		for _, g := range []*Grid[ksGridRow]{
			ksGrid(t).WithMaxLimit(math.MaxInt64 + 1),
			ksGrid(t).WithTiebreaker("nope"),
		} {
			for _, q := range []*GridQuery{{}, {Offset: 5}} {
				_, want := r.QueryGrid(context.Background(), g, q)
				require.Error(t, want)
				var gerr GridError
				require.False(t, errors.As(want, &gerr))
				_, got := r.QueryGridKeyset(context.Background(), g, q, "")
				assert.Equal(t, want, got)
			}
		}
		assert.Zero(t, cq.calls)
	})
}
