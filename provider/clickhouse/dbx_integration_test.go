package clickhouse

import (
	"encoding/json"
	"errors"
	"fmt"

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
