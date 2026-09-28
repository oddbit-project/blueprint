package sqlite

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/oddbit-project/blueprint/dbx"
	"github.com/oddbit-project/gohan"
)

// ksRow is the record type for the keyset pagination tests. created holds
// UTC times, as keyset pagination on SQLite requires.
type ksRow struct {
	ID      int64     `db:"id,auto" json:"id" grid:"sort,filter"`
	Score   int32     `db:"score" json:"score" grid:"sort,filter"`
	Name    string    `db:"name" json:"name" grid:"sort,search"`
	Created time.Time `db:"created" json:"created" grid:"sort"`
	UID     uuid.UUID `db:"uid" json:"uid" grid:"sort"`
}

const ksRowsDDL = `CREATE TABLE %s (id INTEGER PRIMARY KEY AUTOINCREMENT, score INTEGER NOT NULL,
	name TEXT NOT NULL, created DATETIME NOT NULL, uid TEXT NOT NULL)`

var ksNames = []string{"alice", "Alice", "ALICE", "Ålund", "ábaco", "zebra", "Zoë", "日本", "b", "émile", "Émile"}

// ksSeed builds n rows: scores repeat every 13 rows, created takes 20
// distinct UTC values with nanoseconds.
func ksSeed(n int) []*ksRow {
	base := time.Date(2024, 3, 4, 5, 6, 7, 0, time.UTC)
	rows := make([]*ksRow, n)
	for i := range rows {
		rows[i] = &ksRow{
			Score:   int32(i % 13),
			Name:    ksNames[i%len(ksNames)],
			Created: base.Add(time.Duration(i%20) * 123456789),
			UID:     uuid.New(),
		}
	}
	return rows
}

// ksWalk fetches pages until HasMore is false, checking every page but the
// last is full and that only the last has no cursor.
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

func ksIDs(rows []*ksRow) []int64 {
	ids := make([]int64, len(rows))
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

func (s *SQLiteIntegrationTestSuite) TestDbxKeysetPagination() {
	const table = "dbx_keyset_rows"
	const total = 257
	s.execDDL(fmt.Sprintf(ksRowsDDL, table))
	defer s.dropTable(table)
	t := s.T()

	r, err := dbx.NewRepository[ksRow](s.dbxQuerier(), table)
	require.NoError(t, err)
	require.NoError(t, r.Insert(s.ctx, ksSeed(total)...))

	// I1
	keySets := [][]dbx.KeysetKey{
		{dbx.KeyAsc("score"), dbx.KeyAsc("id")},
		{dbx.KeyDesc("score"), dbx.KeyAsc("id")},
		{dbx.KeyDesc("created"), dbx.KeyDesc("id")},
		{dbx.KeyAsc("created"), dbx.KeyDesc("score"), dbx.KeyAsc("id")},
		{dbx.KeyAsc("name"), dbx.KeyDesc("id")},
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

	// times read back are the UTC instants written, nanoseconds included
	list, err := r.List(s.ctx, r.Select().Where(gohan.Col("id").Eq(2)))
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.True(t, list[0].Created.Equal(ksSeed(2)[1].Created), "got %v", list[0].Created)

	// I2
	keys := keySets[0]
	first, err := r.ListKeyset(s.ctx, nil, keys, 10, "")
	require.NoError(t, err)
	ordered, err := r.List(s.ctx, ksOrderBy(r.Select(), keys))
	require.NoError(t, err)
	doomed := ordered[len(ordered)-1].ID
	now := time.Now().UTC()
	require.NoError(t, r.Insert(s.ctx,
		&ksRow{Score: -1, Name: "before", Created: now, UID: uuid.New()},
		&ksRow{Score: 6, Name: "after", Created: now, UID: uuid.New()}))
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
		Sort:         []dbx.SortField{{Field: "created", Order: dbx.SortDescending}, {Field: "name", Order: dbx.SortAscending}},
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

	// I4
	forged := []struct {
		keys []dbx.KeysetKey
		vals []string
	}{
		{[]dbx.KeysetKey{dbx.KeyAsc("score"), dbx.KeyAsc("id")}, []string{"2147483647", "9223372036854775807"}},
		{[]dbx.KeysetKey{dbx.KeyAsc("score"), dbx.KeyAsc("id")}, []string{"-2147483648", "-9223372036854775808"}},
		{[]dbx.KeysetKey{dbx.KeyDesc("created"), dbx.KeyDesc("id")}, []string{"0001-01-01T00:00:00Z", "1"}},
		{[]dbx.KeysetKey{dbx.KeyDesc("created"), dbx.KeyDesc("id")}, []string{"9999-12-31T23:59:59.999999999Z", "1"}},
		{[]dbx.KeysetKey{dbx.KeyAsc("uid")}, []string{"ffffffff-ffff-ffff-ffff-ffffffffffff"}},
		{[]dbx.KeysetKey{dbx.KeyAsc("name"), dbx.KeyDesc("id")}, []string{"\u0000", "1"}},
	}
	for _, f := range forged {
		page, err := r.ListKeyset(s.ctx, nil, f.keys, 1, "")
		require.NoError(t, err)
		_, err = r.ListKeyset(s.ctx, nil, f.keys, 5, ksForge(t, page.NextCursor, f.vals...))
		if err != nil {
			assert.ErrorIs(t, err, dbx.ErrInvalidCursor, "keys %v vals %v", f.keys, f.vals)
		}
	}
}

// TestDbxKeysetRandomWalk walks random key directions and page sizes over
// rows with many duplicate score and created values.
func (s *SQLiteIntegrationTestSuite) TestDbxKeysetRandomWalk() {
	const table = "dbx_keyset_prop"
	s.execDDL(fmt.Sprintf(ksRowsDDL, table))
	defer s.dropTable(table)
	t := s.T()

	rnd := rand.New(rand.NewSource(20240304))
	base := time.Date(2024, 3, 4, 5, 6, 7, 0, time.UTC)
	rows := make([]*ksRow, 500)
	for i := range rows {
		rows[i] = &ksRow{
			Score:   int32(rnd.Intn(5)),
			Name:    ksNames[rnd.Intn(len(ksNames))],
			Created: base.Add(time.Duration(rnd.Intn(8)) * 123456789),
			UID:     uuid.New(),
		}
	}
	r, err := dbx.NewRepository[ksRow](s.dbxQuerier(), table)
	require.NoError(t, err)
	require.NoError(t, r.Insert(s.ctx, rows...))

	cols := []string{"score", "created", "name"}
	for iter := 0; iter < 20; iter++ {
		rnd.Shuffle(len(cols), func(i, j int) { cols[i], cols[j] = cols[j], cols[i] })
		keys := make([]dbx.KeysetKey, 0, 4)
		for _, c := range cols {
			keys = append(keys, dbx.KeysetKey{Column: c, Desc: rnd.Intn(2) == 0})
		}
		keys = append(keys, dbx.KeysetKey{Column: "id", Desc: rnd.Intn(2) == 0})
		size := 1 + rnd.Intn(60)

		want, err := r.List(s.ctx, ksOrderBy(r.Select(), keys))
		require.NoError(t, err)
		got := ksWalk(t, size, func(c string) (*dbx.KeysetPage[ksRow], error) {
			return r.ListKeyset(s.ctx, nil, keys, size, c)
		})
		assert.Equal(t, ksIDs(want), ksIDs(got), "keys %v, page size %d", keys, size)
	}
}
