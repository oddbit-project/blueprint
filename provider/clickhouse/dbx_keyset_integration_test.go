package clickhouse

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/oddbit-project/blueprint/dbx"
	"github.com/oddbit-project/gohan"
)

// ksRow is the record type for the keyset pagination tests.
type ksRow struct {
	ID    uint32    `ch:"id" db:"id" json:"id" grid:"sort,filter"`
	Score uint8     `ch:"score" db:"score" json:"score" grid:"sort,filter"`
	Name  string    `ch:"name" db:"name" json:"name" grid:"sort,search"`
	T0    time.Time `ch:"t0" db:"t0" json:"t0" grid:"sort"`
	T3    time.Time `ch:"t3" db:"t3" json:"t3" grid:"sort"`
	T6    time.Time `ch:"t6" db:"t6" json:"t6" grid:"sort"`
	T9    time.Time `ch:"t9" db:"t9" json:"t9" grid:"sort"`
	UID   uuid.UUID `ch:"uid" db:"uid" json:"uid" grid:"sort"`
}

const ksRowsDDL = `
CREATE TABLE %s (
	id    UInt32,
	score UInt8,
	name  String,
	t0    DateTime('UTC'),
	t3    DateTime64(3, 'UTC'),
	t6    DateTime64(6),
	t9    DateTime64(9, 'UTC'),
	uid   UUID
) ENGINE = MergeTree ORDER BY id
`

var ksNames = []string{"alice", "Alice", "ALICE", "Ålund", "ábaco", "zebra", "Zoë", "日本", "b", "émile", "Émile"}

// ksSeed builds rows with ids from first: scores repeat every 13 rows and
// each time column takes 20 distinct values at its own precision.
func ksSeed(first, n int) []*ksRow {
	base := time.Date(2024, 3, 4, 5, 6, 7, 0, time.UTC)
	rows := make([]*ksRow, n)
	for i := range rows {
		k := time.Duration((first + i) % 20)
		rows[i] = &ksRow{
			ID:    uint32(first + i),
			Score: uint8((first + i) % 13),
			Name:  ksNames[(first+i)%len(ksNames)],
			T0:    base.Add(k * 1234 * time.Second),
			T3:    base.Add(k * 1234 * time.Millisecond),
			T6:    base.Add(k * 1234567 * time.Microsecond),
			T9:    base.Add(k * 123456789),
			UID:   uuid.New(),
		}
	}
	return rows
}

func ksWalk[T any](t *testing.T, pageSize int, fetch func(cursor string) (*dbx.KeysetPage[T], error)) []*T {
	t.Helper()
	var all []*T
	cursor := ""
	for i := 0; ; i++ {
		require.Less(t, i, 10000, "walk does not end")
		page, err := fetch(cursor)
		require.NoError(t, err)
		require.NotNil(t, page.Items)
		assert.Equal(t, page.HasMore, page.NextCursor != "")
		all = append(all, page.Items...)
		if !page.HasMore {
			assert.LessOrEqual(t, len(page.Items), pageSize)
			return all
		}
		require.Len(t, page.Items, pageSize)
		cursor = page.NextCursor
	}
}

func ksOrderBy(sel *gohan.SelectBuilder, keys []dbx.KeysetKey) *gohan.SelectBuilder {
	for _, k := range keys {
		if k.Desc {
			sel = sel.OrderBy(gohan.Col(k.Column).Desc())
		} else {
			sel = sel.OrderBy(gohan.Col(k.Column).Asc())
		}
	}
	return sel
}

func ksIDs(rows []*ksRow) []uint32 {
	ids := make([]uint32, len(rows))
	for i, r := range rows {
		ids[i] = r.ID
	}
	return ids
}

// ksForge replaces cursor's key values, keeping its fingerprint.
func ksForge(t *testing.T, cursor string, keys ...string) string {
	t.Helper()
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	require.NoError(t, err)
	var pl struct {
		V int      `json:"v"`
		F string   `json:"f"`
		K []string `json:"k"`
	}
	require.NoError(t, json.Unmarshal(raw, &pl))
	pl.K = keys
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	require.NoError(t, enc.Encode(pl))
	return base64.RawURLEncoding.EncodeToString(bytes.TrimSuffix(buf.Bytes(), []byte("\n")))
}

func (s *ClickhouseRepositoryTestSuite) TestDbxKeysetPagination() {
	const table = "dbx_keyset_rows"
	const total = 257
	s.createTable(table, ksRowsDDL)
	defer s.dropTable(table)
	t := s.T()

	r, err := dbx.NewRepository[ksRow](s.client.Querier(), table)
	require.NoError(t, err)
	require.NoError(t, r.Insert(s.ctx, ksSeed(1, total)...))

	// I1
	keySets := [][]dbx.KeysetKey{
		{dbx.KeyDesc("score"), dbx.KeyAsc("id")},
		{dbx.KeyAsc("t0"), dbx.KeyAsc("id")},
		{dbx.KeyDesc("t3"), dbx.KeyAsc("id")},
		{dbx.KeyAsc("t6"), dbx.KeyDesc("id")},
		{dbx.KeyDesc("t9"), dbx.KeyAsc("id")},
		{dbx.KeyAsc("name"), dbx.KeyAsc("id")},
		{dbx.KeyAsc("uid")},
	}
	for _, keys := range keySets {
		want, err := r.List(s.ctx, ksOrderBy(r.Select(), keys))
		require.NoError(t, err)
		require.Len(t, want, total)
		for _, size := range []int{1, 7, 50, 300} {
			got := ksWalk(t, size, func(c string) (*dbx.KeysetPage[ksRow], error) {
				return r.ListKeyset(s.ctx, nil, keys, size, c)
			})
			assert.Equal(t, ksIDs(want), ksIDs(got), "keys %v, page size %d", keys, size)
		}
	}

	// I2
	keys := []dbx.KeysetKey{dbx.KeyAsc("score"), dbx.KeyAsc("id")}
	first, err := r.ListKeyset(s.ctx, nil, keys, 10, "")
	require.NoError(t, err)
	cur := first.Items[len(first.Items)-1]
	before := ksSeed(1000, 1)[0]
	before.Score, before.ID, before.Name = cur.Score, cur.ID-1, "before" // sorts before the cursor
	after := ksSeed(1001, 1)[0]
	after.Score, after.Name = 6, "after"
	require.NoError(t, r.Insert(s.ctx, before, after))
	doomed := uint32(total) // score 257 % 13 = 10, not on the first page
	_, err = r.Delete(s.ctx, gohan.Col("id").Eq(doomed))
	require.NoError(t, err)
	rest := ksWalk(t, 10, func(c string) (*dbx.KeysetPage[ksRow], error) {
		if c == "" {
			c = first.NextCursor
		}
		return r.ListKeyset(s.ctx, nil, keys, 10, c)
	})
	seen := map[string]int{}
	for _, row := range append(first.Items, rest...) {
		seen[row.Name]++
		assert.NotEqual(t, doomed, row.ID)
	}
	assert.Zero(t, seen["before"])
	assert.Equal(t, 1, seen["after"])
	assert.Len(t, append(first.Items, rest...), total)

	// I3
	g, err := dbx.NewGrid[ksRow]()
	require.NoError(t, err)
	g.WithTiebreaker("id").WithMaxLimit(0)
	gq := &dbx.GridQuery{
		FilterFields: map[string]any{"score": []any{float64(1), float64(2), float64(3), float64(6)}},
		Sort:         []dbx.SortField{{Field: "t6", Order: dbx.SortDescending}, {Field: "name", Order: dbx.SortAscending}},
	}
	wantGrid, err := r.QueryGrid(s.ctx, g, gq)
	require.NoError(t, err)
	require.NotEmpty(t, wantGrid)
	pq := *gq
	pq.Limit = 7
	gotGrid := ksWalk(t, 7, func(c string) (*dbx.KeysetPage[ksRow], error) {
		return r.QueryGridKeyset(s.ctx, g, &pq, c)
	})
	assert.Equal(t, ksIDs(wantGrid), ksIDs(gotGrid))

	// I4: forged extremes give a page or ErrInvalidCursor, never a database
	// error.
	type forgedCase struct {
		keys []dbx.KeysetKey
		vals []string
	}
	forged := []forgedCase{
		{keys: []dbx.KeysetKey{dbx.KeyDesc("score"), dbx.KeyAsc("id")}, vals: []string{"255", "4294967295"}},
		{keys: []dbx.KeysetKey{dbx.KeyDesc("score"), dbx.KeyAsc("id")}, vals: []string{"0", "18446744073709551615"}},
		{keys: []dbx.KeysetKey{dbx.KeyAsc("uid")}, vals: []string{"ffffffff-ffff-ffff-ffff-ffffffffffff"}},
		{keys: []dbx.KeysetKey{dbx.KeyAsc("uid")}, vals: []string{"00000000-0000-0000-0000-000000000000"}},
	}
	for _, col := range []string{"t0", "t3", "t6", "t9"} {
		for _, v := range []string{
			"0001-01-01T00:00:00Z", "1899-12-31T23:59:59.999999999Z", "1900-01-01T00:00:00Z",
			"2262-04-11T23:47:16.854775807Z", "2262-04-11T23:47:16.854775808Z", "2263-01-01T00:00:00.000000001Z",
			"2299-12-31T23:59:59.999Z", "2299-12-31T23:59:59.999999999Z", "2300-01-01T00:00:00Z",
			"9999-12-31T23:59:59.999999999Z",
		} {
			forged = append(forged, forgedCase{keys: []dbx.KeysetKey{dbx.KeyDesc(col), dbx.KeyAsc("id")}, vals: []string{v, "1"}})
		}
	}
	for _, f := range forged {
		t.Run(fmt.Sprintf("forged %v %v", f.keys, f.vals), func(t *testing.T) {
			page, err := r.ListKeyset(s.ctx, nil, f.keys, 1, "")
			require.NoError(t, err)
			_, err = r.ListKeyset(s.ctx, nil, f.keys, 5, ksForge(t, page.NextCursor, f.vals...))
			if err != nil {
				assert.ErrorIs(t, err, dbx.ErrInvalidCursor)
			}
		})
	}
}
