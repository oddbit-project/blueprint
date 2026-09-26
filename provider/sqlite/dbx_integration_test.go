package sqlite

import (
	"fmt"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/oddbit-project/blueprint/dbx"
	"github.com/oddbit-project/blueprint/sqlb"
)

// dbxSqliteItem is the record type for TestDbxRoundTrip.
type dbxSqliteItem struct {
	ID   int64  `db:"id,auto"`
	Name string `db:"name"`
	Note string `db:"note"`
}

// dbxSqliteChunkRow is a minimal two-field, no-auto-id record type for
// TestDbxChunkedInsert.
type dbxSqliteChunkRow struct {
	A int    `db:"a"`
	B string `db:"b"`
}

func (s *SQLiteIntegrationTestSuite) dbxQuerier() *dbx.SQLQuerier {
	client := s.getTestClient()
	q, err := dbx.FromClient(client)
	require.NoError(s.T(), err)
	return q
}

func (s *SQLiteIntegrationTestSuite) execDDL(ddl string) {
	_, err := s.client.Conn.ExecContext(s.ctx, ddl)
	require.NoError(s.T(), err)
}

func (s *SQLiteIntegrationTestSuite) dropTable(table string) {
	_, _ = s.client.Conn.ExecContext(s.ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s", table))
}

func (s *SQLiteIntegrationTestSuite) TestDbxRoundTrip() {
	const table = "dbx_sqlite_roundtrip"
	s.execDDL(fmt.Sprintf("CREATE TABLE %s (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT UNIQUE, note TEXT)", table))
	defer s.dropTable(table)

	q := s.dbxQuerier()
	r, err := dbx.NewRepository[dbxSqliteItem](q, table)
	require.NoError(s.T(), err)

	require.NoError(s.T(), r.Insert(s.ctx, &dbxSqliteItem{Name: "alice"}))

	got, err := r.GetBy(s.ctx, map[string]any{"name": "alice"})
	require.NoError(s.T(), err)
	assert.NotZero(s.T(), got.ID)

	list, err := r.List(s.ctx, nil)
	require.NoError(s.T(), err)
	assert.Len(s.T(), list, 1)

	cnt, err := r.Count(s.ctx, nil)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(1), cnt)

	exists, err := r.Exists(s.ctx, sqlb.Col("id").Eq(got.ID))
	require.NoError(s.T(), err)
	assert.True(s.T(), exists)

	n, err := r.Update(s.ctx, &dbxSqliteItem{Name: "alice2"}, sqlb.Col("id").Eq(got.ID))
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(1), n)

	n, err = r.UpdateFields(s.ctx, map[string]any{"name": "alice3"}, sqlb.Col("id").Eq(got.ID))
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(1), n)

	// Upsert: a conflicting name (unique) updates the existing row's note
	// instead of inserting a duplicate.
	require.NoError(s.T(), r.Upsert(s.ctx, &dbxSqliteItem{Name: "alice3", Note: "updated"}, []string{"name"}, "note"))
	cnt, err = r.Count(s.ctx, nil)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(1), cnt)
	after, err := r.GetBy(s.ctx, map[string]any{"id": got.ID})
	require.NoError(s.T(), err)
	assert.Equal(s.T(), "updated", after.Note)

	// InsertReturning: a new row, id populated from the database.
	returned, err := r.InsertReturning(s.ctx, &dbxSqliteItem{Name: "bob"})
	require.NoError(s.T(), err)
	assert.NotZero(s.T(), returned.ID)
	assert.Equal(s.T(), "bob", returned.Name)

	cnt, err = r.Count(s.ctx, nil)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(2), cnt)

	n, err = r.Delete(s.ctx, sqlb.Col("id").Eq(got.ID))
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(1), n)

	cnt, err = r.Count(s.ctx, nil)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(1), cnt)
}

func (s *SQLiteIntegrationTestSuite) TestDbxValuesRoundTrip() {
	const table = "dbx_sqlite_values"
	s.execDDL(fmt.Sprintf("CREATE TABLE %s (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT, note TEXT)", table))
	defer s.dropTable(table)

	q := s.dbxQuerier()
	r, err := dbx.NewRepository[dbxSqliteItem](q, table)
	require.NoError(s.T(), err)

	names := []string{`a\'b`, `a\nb`, `a\\b`, "what?", "plain"}
	for _, n := range names {
		require.NoError(s.T(), r.Insert(s.ctx, &dbxSqliteItem{Name: n}), n)
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

	n, err := r.Delete(s.ctx, sqlb.Col("name").Eq(payload))
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(0), n)

	cnt, err := r.Count(s.ctx, nil)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(len(names)), cnt)
}

// TestDbxUnknownColumnErrors proves sqlb's backtick-quoted SQLite
// identifiers defeat SQLite's double-quoted-string fallback: a Match on a
// column that doesn't exist must fail the query, not silently match every
// row (which is what would happen if the identifier were double-quoted
// and SQLite fell back to treating it as a string literal).
func (s *SQLiteIntegrationTestSuite) TestDbxUnknownColumnErrors() {
	const table = "dbx_sqlite_unknown"
	s.execDDL(fmt.Sprintf("CREATE TABLE %s (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT, note TEXT)", table))
	defer s.dropTable(table)

	q := s.dbxQuerier()
	r, err := dbx.NewRepository[dbxSqliteItem](q, table)
	require.NoError(s.T(), err)
	require.NoError(s.T(), r.Insert(s.ctx, &dbxSqliteItem{Name: "alice"}))

	sqlStr, args, err := sqlb.Select("id", "name").From(table).
		Where(sqlb.Match(map[string]any{"does_not_exist": "alice"})).
		Build(q.Dialect())
	require.NoError(s.T(), err)

	var dest []dbxSqliteItem
	err = q.Select(s.ctx, &dest, sqlStr, args...)
	require.Error(s.T(), err)
}

func (s *SQLiteIntegrationTestSuite) TestDbxContainsLiteral() {
	const table = "dbx_sqlite_contains"
	s.execDDL(fmt.Sprintf("CREATE TABLE %s (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT, note TEXT)", table))
	defer s.dropTable(table)

	q := s.dbxQuerier()
	r, err := dbx.NewRepository[dbxSqliteItem](q, table)
	require.NoError(s.T(), err)

	names := []string{"50%", "50x", "a_b", "axb", `c\d`}
	for _, n := range names {
		require.NoError(s.T(), r.Insert(s.ctx, &dbxSqliteItem{Name: n}), n)
	}

	for _, tc := range []string{"50%", "a_b", `c\d`} {
		list, err := r.List(s.ctx, r.Select().Where(sqlb.Col("name").Contains(tc)))
		require.NoError(s.T(), err, tc)
		require.Len(s.T(), list, 1, tc)
		assert.Equal(s.T(), tc, list[0].Name, tc)
	}
}

func (s *SQLiteIntegrationTestSuite) TestDbxSQLiteSyntax() {
	const table = "dbx_sqlite_syntax"
	s.execDDL(fmt.Sprintf("CREATE TABLE %s (id INTEGER PRIMARY KEY, name TEXT, note TEXT)", table))
	defer s.dropTable(table)
	const joinB = "dbx_sqlite_syntax_b"
	s.execDDL(fmt.Sprintf("CREATE TABLE %s (id INTEGER PRIMARY KEY, tag TEXT)", joinB))
	defer s.dropTable(joinB)

	q := s.dbxQuerier()
	r, err := dbx.NewRepository[dbxSqliteItem](q, table)
	require.NoError(s.T(), err)
	require.NoError(s.T(), r.Insert(s.ctx,
		&dbxSqliteItem{ID: 1, Name: "a"},
		&dbxSqliteItem{ID: 2, Name: "b"},
		&dbxSqliteItem{ID: 3, Name: "c"},
	))
	// Only id 1 and 2 have a matching row in joinB, to exercise FullJoin.
	_, err = s.client.Conn.ExecContext(s.ctx, fmt.Sprintf("INSERT INTO %s (id, tag) VALUES (1,'x'),(2,'y'),(4,'z')", joinB))
	require.NoError(s.T(), err)

	countOf := func(sel *sqlb.SelectBuilder) int64 {
		s.T().Helper()
		sqlStr, args, err := sel.Build(q.Dialect())
		require.NoError(s.T(), err)
		n, err := q.QueryInt64(s.ctx, sqlStr, args...)
		require.NoError(s.T(), err)
		return n
	}

	// Offset without Limit renders "LIMIT -1 OFFSET n" and executes.
	assert.Equal(s.T(), int64(2), countOf(
		sqlb.Select(sqlb.CountAll()).From(sqlb.Select("id").From(table).OrderBy("id").Offset(1).As("o"))))

	// Union renders plain UNION (dedup).
	assert.Equal(s.T(), int64(3), countOf(
		sqlb.Select(sqlb.CountAll()).From(
			sqlb.Select("id").From(table).Union(sqlb.Select("id").From(table)).As("u"))))

	// WithRecursive: generate 1..3.
	assert.Equal(s.T(), int64(3), countOf(
		sqlb.Select(sqlb.CountAll()).From(
			sqlb.Select("k").From("r").WithRecursive("r",
				sqlb.Select(sqlb.Int(1).As("k")).UnionAll(
					sqlb.Select(sqlb.Raw("k + 1")).From("r").Where(sqlb.Col("k").Lt(3)))).As("gen"))))

	// FullJoin: id 3 has no match in joinB, joinB's id 4 has no match in
	// table -> 4 rows (1,2 matched + 3 and 4 as one-sided NULLs).
	fullJoinSel := sqlb.Select(sqlb.CountAll()).
		From(sqlb.Table(table).As("t")).
		FullJoin(sqlb.Table(joinB).As("j"), sqlb.Table("t").Col("id").Eq(sqlb.Table("j").Col("id")))
	assert.Equal(s.T(), int64(4), countOf(fullJoinSel))

	// Upsert from FromSelect, plain select form.
	upsertTable := "dbx_sqlite_upsert"
	s.execDDL(fmt.Sprintf("CREATE TABLE %s (id INTEGER PRIMARY KEY, name TEXT, val INTEGER)", upsertTable))
	defer s.dropTable(upsertTable)

	plainSel := sqlb.Select(sqlb.Int(1).As("id"), sqlb.Val("x").As("name"), sqlb.Int(10).As("val"))
	ins1 := sqlb.Insert(upsertTable).Columns("id", "name", "val").
		FromSelect(plainSel).OnConflict("id").DoUpdateExcluded("name", "val")
	sqlStr, args, err := ins1.Build(q.Dialect())
	require.NoError(s.T(), err)
	_, err = q.Exec(s.ctx, sqlStr, args...)
	require.NoError(s.T(), err)

	// Compound (UnionAll) form: WHERE true must land on the last member,
	// not the first, or SQLite's upsert grammar rejects the statement.
	selA := sqlb.Select(sqlb.Int(2).As("id"), sqlb.Val("y").As("name"), sqlb.Int(20).As("val"))
	selB := sqlb.Select(sqlb.Int(3).As("id"), sqlb.Val("z").As("name"), sqlb.Int(30).As("val"))
	ins2 := sqlb.Insert(upsertTable).Columns("id", "name", "val").
		FromSelect(selA.UnionAll(selB)).OnConflict("id").DoUpdateExcluded("name", "val")
	sqlStr, args, err = ins2.Build(q.Dialect())
	require.NoError(s.T(), err)
	_, err = q.Exec(s.ctx, sqlStr, args...)
	require.NoError(s.T(), err)

	upRepo, err := dbx.NewRepository[struct {
		ID   int64  `db:"id"`
		Name string `db:"name"`
		Val  int64  `db:"val"`
	}](q, upsertTable)
	require.NoError(s.T(), err)
	cnt, err := upRepo.Count(s.ctx, nil)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(3), cnt)
}

func (s *SQLiteIntegrationTestSuite) TestDbxILikeUnsupported() {
	_, _, err := sqlb.Select("*").From("t").Where(sqlb.Col("name").ILike("%a%")).Build(sqlb.SQLite())
	assert.ErrorIs(s.T(), err, sqlb.ErrUnsupported)
}

func (s *SQLiteIntegrationTestSuite) TestDbxChunkedInsert() {
	const table = "dbx_sqlite_chunked"
	s.execDDL(fmt.Sprintf("CREATE TABLE %s (a INTEGER, b TEXT)", table))
	defer s.dropTable(table)

	q := s.dbxQuerier()
	r, err := dbx.NewRepository[dbxSqliteChunkRow](q, table)
	require.NoError(s.T(), err)

	const total = 40000
	records := make([]*dbxSqliteChunkRow, total)
	for i := 0; i < total; i++ {
		records[i] = &dbxSqliteChunkRow{A: i, B: "x"}
	}
	require.NoError(s.T(), r.Insert(s.ctx, records...))

	cnt, err := r.Count(s.ctx, nil)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(total), cnt)
}
