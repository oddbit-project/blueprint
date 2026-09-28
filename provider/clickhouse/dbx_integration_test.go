package clickhouse

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/oddbit-project/blueprint/dbx"
	"github.com/oddbit-project/gohan"
)

// dbxEvent is the record type used by most of this file's tests. The
// `created` column (DEFAULT now()) is deliberately absent from the
// struct: InsertBatch must send an explicit column list, or a batch
// against a table with a DEFAULT column not present in T fails with
// "missing destination name".
type dbxEvent struct {
	ID   uint32   `ch:"id" db:"id" json:"id" grid:"sort,filter"`
	Name string   `ch:"name" db:"name" json:"name" grid:"search,sort"`
	Tags []string `ch:"tags" db:"tags" json:"tags"`
}

const dbxEventDDL = `
CREATE TABLE %s (
	id      UInt32,
	name    String,
	tags    Array(String),
	created DateTime DEFAULT now()
) ENGINE = MergeTree ORDER BY id
`

const dbxClausesDDL = `
CREATE TABLE %s (
	id      UInt32,
	name    String,
	tags    Array(String),
	created DateTime DEFAULT now()
) ENGINE = ReplacingMergeTree ORDER BY (id, intHash32(id)) SAMPLE BY intHash32(id)
`

// createTable runs ddl (a "CREATE TABLE %s (...) ..." format string)
// against table and registers a drop for cleanup.
func (s *ClickhouseRepositoryTestSuite) createTable(table, ddlFmt string) {
	err := s.client.Conn.Exec(s.ctx, fmt.Sprintf(ddlFmt, table))
	require.NoError(s.T(), err, "create table %s", table)
}

func (s *ClickhouseRepositoryTestSuite) dropTable(table string) {
	err := s.client.Conn.Exec(s.ctx, "DROP TABLE IF EXISTS "+table)
	if err != nil {
		s.T().Logf("failed to drop table %s: %v", table, err)
	}
}

func (s *ClickhouseRepositoryTestSuite) TestDbxRoundTrip() {
	const table = "dbx_roundtrip"
	s.createTable(table, dbxEventDDL)
	defer s.dropTable(table)

	q := s.client.Querier()
	r, err := dbx.NewRepository[dbxEvent](q, table)
	require.NoError(s.T(), err)

	rows := []*dbxEvent{
		{ID: 1, Name: "alice", Tags: []string{"x"}},
		{ID: 2, Name: "bob", Tags: []string{"y", "z"}},
		{ID: 3, Name: "carol", Tags: nil},
	}
	require.NoError(s.T(), r.Insert(s.ctx, rows...))

	// List: the *[]*T path.
	list, err := r.List(s.ctx, nil)
	require.NoError(s.T(), err)
	require.Len(s.T(), list, 3)

	got, err := r.Get(s.ctx, r.Select().Where(gohan.Col("id").Eq(uint32(1))))
	require.NoError(s.T(), err)
	assert.Equal(s.T(), "alice", got.Name)

	gotBy, err := r.GetBy(s.ctx, map[string]any{"name": "bob"})
	require.NoError(s.T(), err)
	assert.Equal(s.T(), uint32(2), gotBy.ID)

	cnt, err := r.Count(s.ctx, nil)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(3), cnt)

	exists, err := r.Exists(s.ctx, gohan.Col("id").Eq(uint32(1)))
	require.NoError(s.T(), err)
	assert.True(s.T(), exists)

	notExists, err := r.Exists(s.ctx, gohan.Col("id").Eq(uint32(999)))
	require.NoError(s.T(), err)
	assert.False(s.T(), notExists)

	// Querier.Exec always reports 0 rows affected on ClickHouse.
	n, err := r.Delete(s.ctx, gohan.Col("id").Eq(uint32(1)))
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(0), n)

	cnt, err = r.Count(s.ctx, nil)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(2), cnt)
}

func (s *ClickhouseRepositoryTestSuite) TestDbxGetNotFound() {
	const table = "dbx_notfound"
	s.createTable(table, dbxEventDDL)
	defer s.dropTable(table)

	q := s.client.Querier()
	r, err := dbx.NewRepository[dbxEvent](q, table)
	require.NoError(s.T(), err)

	_, err = r.Get(s.ctx, r.Select().Where(gohan.Col("id").Eq(uint32(999))))
	require.Error(s.T(), err)
	assert.True(s.T(), errors.Is(err, dbx.ErrNotFound), "got %v", err)
}

func (s *ClickhouseRepositoryTestSuite) TestDbxSelectValueSlice() {
	const table = "dbx_selectvalue"
	s.createTable(table, dbxEventDDL)
	defer s.dropTable(table)

	q := s.client.Querier()
	r, err := dbx.NewRepository[dbxEvent](q, table)
	require.NoError(s.T(), err)
	require.NoError(s.T(), r.Insert(s.ctx,
		&dbxEvent{ID: 1, Name: "a"},
		&dbxEvent{ID: 2, Name: "b"},
	))

	sqlStr, args, err := r.Select().Build(q.Dialect())
	require.NoError(s.T(), err)

	var dest []dbxEvent
	require.NoError(s.T(), q.Select(s.ctx, &dest, sqlStr, args...))
	assert.Len(s.T(), dest, 2)
}

func (s *ClickhouseRepositoryTestSuite) TestDbxValuesRoundTrip() {
	const table = "dbx_values"
	s.createTable(table, dbxEventDDL)
	defer s.dropTable(table)

	q := s.client.Querier()
	r, err := dbx.NewRepository[dbxEvent](q, table)
	require.NoError(s.T(), err)

	names := []string{`a\'b`, `a\nb`, `a\\b`, "what?", "plain"}
	for i, n := range names {
		require.NoError(s.T(), r.Insert(s.ctx, &dbxEvent{ID: uint32(i + 1), Name: n}), n)
	}

	for _, n := range names {
		got, err := r.GetBy(s.ctx, map[string]any{"name": n})
		require.NoError(s.T(), err, n)
		assert.Equal(s.T(), n, got.Name, n)
	}

	const payload = `x\' OR 1=1 --`
	list, err := r.ListBy(s.ctx, map[string]any{"name": payload})
	require.NoError(s.T(), err)
	assert.Len(s.T(), list, 0)

	_, err = r.Delete(s.ctx, gohan.Col("name").Eq(payload))
	require.NoError(s.T(), err)

	cnt, err := r.Count(s.ctx, nil)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(len(names)), cnt)
}

// dbxBackslashCol maps the single-backslash column `c\d`. The tag must be
// written with a doubled backslash in Go source: struct tags are
// Go-unquoted by reflect.StructTag.Get, and a single backslash there is
// an invalid escape (Get would silently return "").
type dbxBackslashCol struct {
	Val string `ch:"c\\d" db:"c\\d"`
}

func (s *ClickhouseRepositoryTestSuite) TestDbxBackslashIdentifier() {
	const table = "dbx_backslash"
	err := s.client.Conn.Exec(s.ctx, `CREATE TABLE `+table+` (id Int32, "c\\d" String) ENGINE = MergeTree ORDER BY id`)
	require.NoError(s.T(), err)
	defer s.dropTable(table)

	require.NoError(s.T(), s.client.Conn.Exec(s.ctx, "INSERT INTO "+table+" VALUES (?, ?)", 1, "val1"))

	q := s.client.Querier()
	sqlStr, args, err := gohan.Select(`c\d`).From(table).Where(gohan.Col("id").Eq(1)).Build(q.Dialect())
	require.NoError(s.T(), err)

	var out dbxBackslashCol
	require.NoError(s.T(), q.Get(s.ctx, &out, sqlStr, args...))
	assert.Equal(s.T(), "val1", out.Val)

	// A trailing single backslash is the sharper case: dropping the \->\\
	// escape here doesn't just fetch the wrong value, it makes the
	// rendered identifier's closing quote look escaped ("d\" instead of
	// "d\\"), so the statement fails to parse at all.
	const table2 = "dbx_backslash_trailing"
	err = s.client.Conn.Exec(s.ctx, `CREATE TABLE `+table2+` (id Int32, "d\\" String) ENGINE = MergeTree ORDER BY id`)
	require.NoError(s.T(), err)
	defer s.dropTable(table2)
	require.NoError(s.T(), s.client.Conn.Exec(s.ctx, "INSERT INTO "+table2+" VALUES (?, ?)", 1, "val2"))

	sqlStr2, args2, err := gohan.Select(`d\`).From(table2).Where(gohan.Col("id").Eq(1)).Build(q.Dialect())
	require.NoError(s.T(), err)

	type trailingCol struct {
		Val string `ch:"d\\"`
	}
	var out2 trailingCol
	require.NoError(s.T(), q.Get(s.ctx, &out2, sqlStr2, args2...))
	assert.Equal(s.T(), "val2", out2.Val)
}

func (s *ClickhouseRepositoryTestSuite) TestDbxQuestionMarkRejected() {
	_, _, err := gohan.Select("q?x").From("t").Build(gohan.ClickHouse())
	assert.True(s.T(), errors.Is(err, gohan.ErrInvalidIdentifier), "got %v", err)

	_, _, err = gohan.Select(gohan.Raw("'??'")).From("t").Build(gohan.ClickHouse())
	assert.True(s.T(), errors.Is(err, gohan.ErrRawPlaceholder), "got %v", err)
}

func (s *ClickhouseRepositoryTestSuite) TestDbxUnsafeValueRejected() {
	_, _, err := gohan.Select("*").From("t").
		Where(gohan.Col("name").Eq(map[string]int{"k": 1})).
		Build(gohan.ClickHouse())
	assert.True(s.T(), errors.Is(err, gohan.ErrUnsafeValue), "got %v", err)
}

// qpRow is the destination type for TestDbxQueryParamTrigger.
type qpRow struct {
	ID uint32 `ch:"id"`
	J  string `ch:"j"`
}

func (s *ClickhouseRepositoryTestSuite) TestDbxQueryParamTrigger() {
	const table = "dbx_queryparam"
	s.createTable(table, dbxEventDDL)
	defer s.dropTable(table)

	q := s.client.Querier()
	r, err := dbx.NewRepository[dbxEvent](q, table)
	require.NoError(s.T(), err)
	require.NoError(s.T(), r.Insert(s.ctx, &dbxEvent{ID: 1, Name: "x"}))

	sel := gohan.Select(gohan.Col("id"), gohan.Raw(`'{"a":1}'`).As("j")).
		From(table).Where(gohan.Col("id").Eq(1))
	sqlStr, args, err := sel.Build(gohan.ClickHouse())
	require.NoError(s.T(), err)

	var rows []*qpRow
	err = q.Select(s.ctx, &rows, sqlStr, args...)
	if err != nil {
		require.ErrorIs(s.T(), err, clickhouse.ErrUnsupportedQueryParameter,
			"driver rejected the query, but not with the documented error: %v", err)
		return
	}
	require.Len(s.T(), rows, 1, "must be exactly the id=1 row, not more/fewer")
	assert.Equal(s.T(), uint32(1), rows[0].ID)
}

func (s *ClickhouseRepositoryTestSuite) TestDbxContainsLiteral() {
	const table = "dbx_contains"
	s.createTable(table, dbxEventDDL)
	defer s.dropTable(table)

	q := s.client.Querier()
	r, err := dbx.NewRepository[dbxEvent](q, table)
	require.NoError(s.T(), err)

	names := []string{"50%", "50x", "a_b", "axb", `c\d`}
	for i, n := range names {
		require.NoError(s.T(), r.Insert(s.ctx, &dbxEvent{ID: uint32(i + 1), Name: n}), n)
	}

	for _, tc := range []string{"50%", "a_b", `c\d`} {
		list, err := r.List(s.ctx, r.Select().Where(gohan.Col("name").Contains(tc)))
		require.NoError(s.T(), err, tc)
		require.Len(s.T(), list, 1, tc)
		assert.Equal(s.T(), tc, list[0].Name, tc)
	}
}

func (s *ClickhouseRepositoryTestSuite) TestDbxClickHouseClauses() {
	const table = "dbx_clauses"
	s.createTable(table, dbxClausesDDL)
	defer s.dropTable(table)

	q := s.client.Querier()
	r, err := dbx.NewRepository[dbxEvent](q, table)
	require.NoError(s.T(), err)

	rows := []*dbxEvent{
		{ID: 1, Name: "a", Tags: []string{"t1"}},
		{ID: 2, Name: "b", Tags: []string{"t1", "t2"}},
		{ID: 3, Name: "c", Tags: nil},
	}
	require.NoError(s.T(), r.Insert(s.ctx, rows...))
	const n = int64(3)

	countOf := func(sel *gohan.SelectBuilder) int64 {
		s.T().Helper()
		sqlStr, args, err := sel.Build(q.Dialect())
		require.NoError(s.T(), err)
		cnt, err := q.QueryInt64(s.ctx, sqlStr, args...)
		require.NoError(s.T(), err)
		return cnt
	}

	// Final()
	assert.Equal(s.T(), n, countOf(gohan.Select(gohan.CountAll()).From(table).Final()))

	// Sample(1) - full ratio.
	assert.Equal(s.T(), n, countOf(gohan.Select(gohan.CountAll()).From(table).Sample(1)))

	// Prewhere
	assert.Equal(s.T(), int64(1), countOf(gohan.Select(gohan.CountAll()).From(table).Prewhere(gohan.Col("id").Eq(uint32(1)))))

	// ArrayJoin: 1+2+0 = 3 tag rows.
	assert.Equal(s.T(), int64(3), countOf(gohan.Select(gohan.CountAll()).From(table).ArrayJoin(gohan.Col("tags").As("tag"))))

	// Settings
	assert.Equal(s.T(), n, countOf(gohan.Select(gohan.CountAll()).From(table).Settings(map[string]any{"max_threads": 1})))

	// Combined clause order.
	combined := gohan.Select(gohan.CountAll()).From(table).
		Final().Sample(1).
		Prewhere(gohan.Col("id").Eq(uint32(1))).
		Where(gohan.Col("name").Eq("a")).
		Limit(1).
		Settings(map[string]any{"max_threads": 1})
	assert.Equal(s.T(), int64(1), countOf(combined))

	// Union (renders UNION DISTINCT): union of the table with itself
	// dedups back to n rows.
	unionSel := gohan.Select("id").From(table).Union(gohan.Select("id").From(table))
	assert.Equal(s.T(), n, countOf(gohan.Select(gohan.CountAll()).From(unionSel.As("u"))))

	// With: a CTE selecting from the table.
	withSel := gohan.Select("n").From("w").
		With("w", gohan.Select(gohan.CountAll().As("n")).From(table))
	assert.Equal(s.T(), n, countOf(withSel))

	// Join: self-join on id, one-to-one.
	joinSel := gohan.Select(gohan.CountAll()).
		From(gohan.Table(table).As("a")).
		Join(gohan.Table(table).As("b"), gohan.Table(table).As("a").Col("id").Eq(gohan.Table(table).As("b").Col("id")))
	assert.Equal(s.T(), n, countOf(joinSel))

	// Offset without Limit.
	assert.Equal(s.T(), n-1, countOf(gohan.Select(gohan.CountAll()).From(gohan.Select("id").From(table).Offset(1).As("o"))))

	// Compound-limit: gohan must wrap the compound on ClickHouse so the
	// outer LIMIT 1 applies to the whole union, not just its last member.
	const tableB = "dbx_clauses_b"
	s.createTable(tableB, dbxClausesDDL)
	defer s.dropTable(tableB)
	require.NoError(s.T(), func() error {
		rb, err := dbx.NewRepository[dbxEvent](q, tableB)
		if err != nil {
			return err
		}
		return rb.Insert(s.ctx, &dbxEvent{ID: 1, Name: "x"}, &dbxEvent{ID: 2, Name: "y"})
	}())

	compound := gohan.Select("id").From(table).UnionAll(gohan.Select("id").From(tableB)).OrderBy("id").Limit(1)
	assert.Equal(s.T(), int64(1), countOf(gohan.Select(gohan.CountAll()).From(compound.As("c"))))

	// Exec(ctx, gohan.Delete(t).All()) renders WHERE 1 and empties the
	// table (lightweight DELETE).
	delSQL, delArgs, err := gohan.Delete(table).All().Build(q.Dialect())
	require.NoError(s.T(), err)
	_, err = q.Exec(s.ctx, delSQL, delArgs...)
	require.NoError(s.T(), err)
}

func (s *ClickhouseRepositoryTestSuite) TestDbxGrid() {
	const table = "dbx_grid"
	s.createTable(table, dbxEventDDL)
	defer s.dropTable(table)

	q := s.client.Querier()
	r, err := dbx.NewRepository[dbxEvent](q, table)
	require.NoError(s.T(), err)

	names := []string{"50%", "50x", "a_b"}
	for i, n := range names {
		require.NoError(s.T(), r.Insert(s.ctx, &dbxEvent{ID: uint32(i + 1), Name: n}), n)
	}

	g, err := dbx.NewGrid[dbxEvent]()
	require.NoError(s.T(), err)

	list, err := r.QueryGrid(s.ctx, g, &dbx.GridQuery{SearchType: dbx.SearchAny, SearchText: "50%"})
	require.NoError(s.T(), err)
	require.Len(s.T(), list, 1)
	assert.Equal(s.T(), "50%", list[0].Name)

	var jq dbx.GridQuery
	body := []byte(`{"filterFields":{"id":[[{"k') OR 1=1 --":1}]]}}`)
	require.NoError(s.T(), json.Unmarshal(body, &jq))

	_, err = r.QueryGrid(s.ctx, g, &jq)
	require.Error(s.T(), err)
	var gerr dbx.GridError
	require.True(s.T(), errors.As(err, &gerr), "got %v", err)
	assert.Equal(s.T(), "value is not valid", gerr.Message)

	// The same nested value, passed directly, must be rejected by gohan
	// too (defense in depth against clickhouse-go's unsafe formatting).
	raw := jq.FilterFields["id"]
	_, _, err = gohan.Select(gohan.CountAll()).From(table).
		Where(gohan.Col("id").In(raw)).
		Build(gohan.ClickHouse())
	assert.True(s.T(), errors.Is(err, gohan.ErrUnsafeValue), "got %v", err)
}

func (s *ClickhouseRepositoryTestSuite) TestDbxUnsupported() {
	const table = "dbx_unsupported"
	s.createTable(table, dbxEventDDL)
	defer s.dropTable(table)

	q := s.client.Querier()
	r, err := dbx.NewRepository[dbxEvent](q, table)
	require.NoError(s.T(), err)

	_, err = r.Update(s.ctx, &dbxEvent{ID: 1, Name: "x"}, gohan.Col("id").Eq(uint32(1)))
	assert.True(s.T(), errors.Is(err, gohan.ErrUnsupported), "got %v", err)

	err = r.Upsert(s.ctx, &dbxEvent{ID: 1, Name: "x"}, []string{"id"})
	assert.True(s.T(), errors.Is(err, gohan.ErrUnsupported), "got %v", err)

	_, err = r.InsertReturning(s.ctx, &dbxEvent{ID: 1, Name: "x"})
	assert.True(s.T(), errors.Is(err, gohan.ErrUnsupported), "got %v", err)

	err = dbx.WithTx(s.ctx, q, nil, func(tx dbx.Querier) error { return nil })
	assert.True(s.T(), errors.Is(err, dbx.ErrTxUnsupported), "got %v", err)
}

// dbxTimeRecord is the record type for TestDbxTimePrecision: three
// DateTime64 columns at different scales, one in a non-UTC timezone, and a
// Date column, used to check that a dbx call binding a time.Time argument
// through the Querier does not truncate it to whole seconds. See
// querier.go's namedArgs.
type dbxTimeRecord struct {
	ID uint32    `ch:"id" db:"id"`
	T3 time.Time `ch:"t3" db:"t3"`
	T9 time.Time `ch:"t9" db:"t9"`
	TL time.Time `ch:"tl" db:"tl"`
	D  time.Time `ch:"d" db:"d"`
}

const dbxTimeDDL = `
CREATE TABLE %s (
	id UInt32,
	t3 DateTime64(3, 'UTC'),
	t9 DateTime64(9, 'UTC'),
	tl DateTime64(6, 'Europe/Lisbon'),
	d  Date
) ENGINE = MergeTree ORDER BY id
`

// TestDbxTimePrecision reproduces the wrong query results caused by
// clickhouse-go's native time.Time binding, which formats a time.Time as
// "toDateTime('YYYY-MM-DD hh:mm:ss')" (whole seconds only), for every dbx
// call that passes args through the Querier (Exec/Get/Select/QueryInt64;
// InsertBatch is columnar and unaffected).
func (s *ClickhouseRepositoryTestSuite) TestDbxTimePrecision() {
	const table = "dbx_time_precision"
	s.createTable(table, dbxTimeDDL)
	defer s.dropTable(table)

	q := s.client.Querier()
	r, err := dbx.NewRepository[dbxTimeRecord](q, table)
	require.NoError(s.T(), err)

	tm := time.Date(2026, 9, 27, 12, 34, 56, 123456789, time.UTC)
	tmBefore := tm.Add(-500 * time.Millisecond)
	midnight := time.Date(2026, 9, 27, 0, 0, 0, 0, time.UTC)

	// id 1: InsertBatch (columnar) - already full precision, the baseline.
	require.NoError(s.T(), r.Insert(s.ctx, &dbxTimeRecord{ID: 1, T3: tm, T9: tm, TL: tm, D: midnight}))

	// id 2: Repository.Exec / gohan.Insert - the path under test, at tm.
	_, err = r.Exec(s.ctx, gohan.Insert(table).Columns("id", "t3", "t9", "tl", "d").
		Values(uint32(2), tm, tm, tm, midnight))
	require.NoError(s.T(), err)

	// id 3: same path, an instant strictly before tm.
	_, err = r.Exec(s.ctx, gohan.Insert(table).Columns("id", "t3", "t9", "tl", "d").
		Values(uint32(3), tmBefore, tmBefore, tmBefore, midnight))
	require.NoError(s.T(), err)

	got, err := r.Get(s.ctx, r.Select().Where(gohan.Col("id").Eq(uint32(2))))
	require.NoError(s.T(), err)
	assert.True(s.T(), got.T9.Equal(tm), "id 2 t9 = %s, want %s (same instant as tm)", got.T9, tm)
	wantT3 := tm.Truncate(time.Millisecond)
	assert.True(s.T(), got.T3.Equal(wantT3),
		"id 2 t3 = %s, want %s (tm truncated to milliseconds, same instant)", got.T3, wantT3)
	wantTL := tm.Truncate(time.Microsecond)
	assert.True(s.T(), got.TL.Equal(wantTL),
		"id 2 tl = %s, want %s (tm truncated to microseconds, same instant)", got.TL, wantTL)

	cnt, err := r.Count(s.ctx, gohan.Col("t9").Eq(tm))
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(2), cnt, "Count(t9 = tm)")

	cnt, err = r.Count(s.ctx, gohan.Col("t3").Eq(tm.Truncate(time.Millisecond)))
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(2), cnt, "Count(t3 = tm.Truncate(ms))")

	cnt, err = r.Count(s.ctx, gohan.Col("t3").Eq(tm))
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(0), cnt, "Count(t3 = tm) exact-instant: t3 stores only milliseconds")

	cnt, err = r.Count(s.ctx, gohan.Col("tl").Eq(tm.Truncate(time.Microsecond)))
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(2), cnt, "Count(tl = tm.Truncate(us))")

	cnt, err = r.Count(s.ctx, gohan.Col("d").Eq(midnight))
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(3), cnt, "Count(d = midnight)")

	list, err := r.List(s.ctx, r.Select().Where(gohan.Col("t9").Gte(tm)))
	require.NoError(s.T(), err)
	gotIDs := make([]uint32, 0, len(list))
	for _, row := range list {
		gotIDs = append(gotIDs, row.ID)
	}
	assert.ElementsMatch(s.T(), []uint32{1, 2}, gotIDs, "List(t9 >= tm)")

	cnt, err = r.Count(s.ctx, gohan.Col("t9").Lt(tm))
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(1), cnt, "Count(t9 < tm)")

	_, err = r.Delete(s.ctx, gohan.Col("t9").Lt(tm))
	require.NoError(s.T(), err)

	list, err = r.List(s.ctx, nil)
	require.NoError(s.T(), err)
	gotIDs = gotIDs[:0]
	for _, row := range list {
		gotIDs = append(gotIDs, row.ID)
	}
	assert.ElementsMatch(s.T(), []uint32{1, 2}, gotIDs, "ids remaining after Delete(t9 < tm)")
}

// TestDbxTimeOutsideUnixNanoRange checks that bound time.Time args outside
// the range of time.Time.UnixNano (roughly 1678-2262) keep their instant, in
// UTC and in other locations. See querier.go's dateNamed. Stored values are
// read back with toString: clickhouse-go v2.40.3's columnar insert and
// time.Time scanning themselves go wrong past 2262, so neither is a usable
// baseline here.
func (s *ClickhouseRepositoryTestSuite) TestDbxTimeOutsideUnixNanoRange() {
	const table = "dbx_time_range"
	s.createTable(table, `
CREATE TABLE %s (
	id UInt32,
	t3 DateTime64(3, 'UTC')
) ENGINE = MergeTree ORDER BY id
`)
	defer s.dropTable(table)

	type rec struct {
		ID uint32    `ch:"id" db:"id"`
		T3 time.Time `ch:"t3" db:"t3"`
	}
	q := s.client.Querier()
	r, err := dbx.NewRepository[rec](q, table)
	require.NoError(s.T(), err)

	zone := time.FixedZone("UTC+5", 5*3600)
	lisbon, err := time.LoadLocation("Europe/Lisbon")
	require.NoError(s.T(), err)
	cases := []struct {
		name string
		at   time.Time
	}{
		{"far future UTC", time.Date(2290, 1, 2, 3, 4, 5, 123000000, time.UTC)},
		{"far future non-UTC", time.Date(2290, 1, 2, 3, 4, 5, 123000000, time.UTC).In(zone)},
		{"early UTC", time.Date(1901, 1, 2, 3, 4, 5, 123000000, time.UTC)},
		{"early non-UTC", time.Date(1901, 1, 2, 3, 4, 5, 123000000, time.UTC).In(zone)},
		{"in range non-UTC", time.Date(2026, 1, 2, 3, 4, 5, 123000000, time.UTC).In(zone)},
		{"far future named zone", time.Date(2290, 1, 2, 3, 4, 5, 123000000, time.UTC).In(lisbon)},
		{"far future Local", time.Date(2290, 1, 2, 3, 4, 5, 123000000, time.UTC).Local()},
		{"early Local", time.Date(1901, 1, 2, 3, 4, 5, 123000000, time.UTC).Local()},
		{"in range Local", time.Date(2026, 1, 2, 3, 4, 5, 123000000, time.UTC).Local()},
		{"pre-2001 Local", time.Date(1990, 1, 2, 3, 4, 5, 123000000, time.UTC).Local()},
		{"epoch Local", time.Unix(0, 500000000).Local()},
	}
	for i, c := range cases {
		id := uint32(i + 1)
		want := c.at.UTC().Format("2006-01-02 15:04:05.000")

		// baseline written as a plain literal, no time binding involved
		_, err := q.Exec(s.ctx, fmt.Sprintf("INSERT INTO %s VALUES (%d, '%s')", table, id, want))
		require.NoError(s.T(), err, c.name)

		// a bound time.Time must compare equal to the stored instant
		cnt, err := r.Count(s.ctx, gohan.And(gohan.Col("id").Eq(id), gohan.Col("t3").Eq(c.at)))
		require.NoError(s.T(), err, c.name)
		assert.Equal(s.T(), int64(1), cnt, "%s: bound arg must match the stored instant", c.name)

		// and a bound time.Time must be stored as that instant
		_, err = r.Exec(s.ctx, gohan.Insert(table).Columns("id", "t3").Values(id+100, c.at))
		require.NoError(s.T(), err, c.name)
		var got []struct {
			S string `ch:"s" db:"s"`
		}
		require.NoError(s.T(), q.Select(s.ctx, &got, fmt.Sprintf("SELECT toString(t3) AS s FROM %s WHERE id = %d", table, id+100)))
		require.Len(s.T(), got, 1, c.name)
		assert.Equal(s.T(), want, got[0].S, "%s: stored value", c.name)
	}
}

// TestDbxTimeZoneDateSemantics checks that a bound time.Time in a named zone
// is compared to a Date column (and passed to toDate) as the calendar day in
// that zone, not in UTC. See querier.go's dateNamed.
func (s *ClickhouseRepositoryTestSuite) TestDbxTimeZoneDateSemantics() {
	const table = "dbx_time_zone_date"
	s.createTable(table, `
CREATE TABLE %s (
	id UInt32,
	d  Date
) ENGINE = MergeTree ORDER BY id
`)
	defer s.dropTable(table)

	q := s.client.Querier()
	_, err := q.Exec(s.ctx, fmt.Sprintf("INSERT INTO %s VALUES (1, '2024-06-01')", table))
	require.NoError(s.T(), err)

	lisbon, err := time.LoadLocation("Europe/Lisbon")
	require.NoError(s.T(), err)
	midnight := time.Date(2024, 6, 1, 0, 0, 0, 0, lisbon) // 2024-05-31 23:00 UTC

	type rec struct {
		ID uint32    `ch:"id" db:"id"`
		D  time.Time `ch:"d" db:"d"`
	}
	r, err := dbx.NewRepository[rec](q, table)
	require.NoError(s.T(), err)
	cnt, err := r.Count(s.ctx, gohan.Col("d").Eq(midnight))
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(1), cnt, "Date column compared with a Lisbon-midnight time")
}
