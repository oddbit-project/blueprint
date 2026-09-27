package dbx

import (
	"context"
	"database/sql/driver"
	"errors"
	"strings"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/oddbit-project/gohan"
)

// --- shared record types ---

type user struct {
	ID    int64  `db:"id,auto"`
	Name  string `db:"name"`
	Email string `db:"email"`
}

type userOpt struct {
	ID    int64   `db:"id,auto"`
	Name  string  `db:"name"`
	Email string  `db:"email"`
	Bio   *string `db:"bio" goqu:"omitnil"`
}

// gridUser is the record type for TestRepositoryQueryGrid.
type gridUser struct {
	ID    int64  `db:"id,auto" json:"id" grid:"sort,filter"`
	Name  string `db:"name" json:"name" grid:"search,sort"`
	Email string `db:"email" json:"email" grid:"filter"`
}

// --- shapes for TestNewRepositoryShapes ---

type EmbeddedBase struct {
	Name string `db:"name"`
}

type shapeEmbedded struct {
	EmbeddedBase
	Age int64 `db:"age"`
}

type shapeTime struct {
	When time.Time `db:"when"`
}

type shapePtrEmbed struct {
	*EmbeddedBase
	Age int64 `db:"age"`
}

type lowerBase struct {
	Name string `db:"name"`
}

type shapeUnexportedEmbed struct {
	lowerBase
	Age int64 `db:"age"`
}

type shapeTaggedEmbed struct {
	EmbeddedBase `db:"-"`
	Age          int64 `db:"age"`
}

type DupBase struct {
	ID2 int64 `db:"id"`
}

type shapeDupCols struct {
	ID int64 `db:"id"`
	DupBase
}

type shapeNoColumns struct {
	X string `db:"-"`
}

// --- test doubles ---

// countingQuerier is a Querier double that counts every I/O method call, to
// prove a guard returns before any query is issued (go-sqlmock's
// ExpectationsWereMet does not detect unexpected calls, only unmet
// expected ones). It implements neither TxQuerier nor TxBeginner.
type countingQuerier struct {
	d     gohan.Dialect
	calls int
}

func (c *countingQuerier) Dialect() gohan.Dialect { return c.d }
func (c *countingQuerier) Exec(ctx context.Context, query string, args ...any) (int64, error) {
	c.calls++
	return 0, nil
}
func (c *countingQuerier) Get(ctx context.Context, dest any, query string, args ...any) error {
	c.calls++
	return nil
}
func (c *countingQuerier) Select(ctx context.Context, dest any, query string, args ...any) error {
	c.calls++
	return nil
}
func (c *countingQuerier) QueryInt64(ctx context.Context, query string, args ...any) (int64, error) {
	c.calls++
	return 0, nil
}

var _ Querier = (*countingQuerier)(nil)

// batchInserterDouble is a Querier + BatchInserter double that records the
// InsertBatch call instead of issuing SQL.
type batchInserterDouble struct {
	countingQuerier
	batchCalls int
	gotTable   string
	gotRows    []any
}

func (b *batchInserterDouble) InsertBatch(ctx context.Context, table string, rows []any) error {
	b.batchCalls++
	b.gotTable = table
	b.gotRows = rows
	return nil
}

var _ BatchInserter = (*batchInserterDouble)(nil)

// --- helpers ---

// chunkInsertSQL builds the literal SQL text an INSERT of n rows of cols
// into table renders to, without going through gohan, so tests pinning
// chunk sizes are not tautological.
func chunkInsertSQL(table string, cols []string, n int) string {
	quoted := make([]string, len(cols))
	for i, c := range cols {
		quoted[i] = `"` + c + `"`
	}
	row := "(" + strings.Repeat("?, ", len(cols)-1) + "?)"
	rows := make([]string, n)
	for i := range rows {
		rows[i] = row
	}
	return `INSERT INTO "` + table + `" (` + strings.Join(quoted, ", ") + `) VALUES ` + strings.Join(rows, ", ")
}

func repeatAnyArg(n int) []driver.Value {
	args := make([]driver.Value, n)
	for i := range args {
		args[i] = sqlmock.AnyArg()
	}
	return args
}

// --- NewRepository ---

func TestNewRepositoryNotStruct(t *testing.T) {
	q, _ := newMockQuerier(t)
	_, err := NewRepository[int](q, "t")
	assert.True(t, errors.Is(err, ErrNotStruct))
}

func TestNewRepositoryInvalidTable(t *testing.T) {
	q, _ := newMockQuerier(t)
	_, err := NewRepository[user](q, "a\x00")
	assert.True(t, errors.Is(err, gohan.ErrInvalidIdentifier))
}

func TestNewRepositoryShapes(t *testing.T) {
	q, _ := newMockQuerier(t)

	t.Run("embedded exported struct accepted", func(t *testing.T) {
		r, err := NewRepository[shapeEmbedded](q, "t")
		require.NoError(t, err)
		assert.Equal(t, []string{"name", "age"}, r.cols)
	})

	t.Run("time.Time field accepted", func(t *testing.T) {
		r, err := NewRepository[shapeTime](q, "t")
		require.NoError(t, err)
		assert.Equal(t, []string{"when"}, r.cols)
	})

	t.Run("embedded pointer rejected", func(t *testing.T) {
		_, err := NewRepository[shapePtrEmbed](q, "t")
		assert.True(t, errors.Is(err, gohan.ErrRecordShape), "got %v", err)
	})

	t.Run("unexported embedded struct rejected", func(t *testing.T) {
		_, err := NewRepository[shapeUnexportedEmbed](q, "t")
		assert.True(t, errors.Is(err, gohan.ErrRecordShape), "got %v", err)
	})

	t.Run("tagged anonymous struct rejected", func(t *testing.T) {
		_, err := NewRepository[shapeTaggedEmbed](q, "t")
		assert.True(t, errors.Is(err, gohan.ErrRecordShape), "got %v", err)
	})

	t.Run("duplicate column rejected", func(t *testing.T) {
		_, err := NewRepository[shapeDupCols](q, "t")
		assert.True(t, errors.Is(err, gohan.ErrDuplicateColumn), "got %v", err)
	})

	t.Run("no columns rejected", func(t *testing.T) {
		_, err := NewRepository[shapeNoColumns](q, "t")
		assert.True(t, errors.Is(err, ErrNoColumns), "got %v", err)
	})
}

func TestRepositoryScanEmbedded(t *testing.T) {
	q, mock := newMockQuerier(t)
	r, err := NewRepository[shapeEmbedded](q, "widgets")
	require.NoError(t, err)

	mock.ExpectQuery(`SELECT "name", "age" FROM "widgets" LIMIT 1`).
		WillReturnRows(sqlmock.NewRows([]string{"name", "age"}).AddRow("bob", int64(5)))

	got, err := r.Get(context.Background(), nil)
	require.NoError(t, err)
	assert.Equal(t, "bob", got.Name)
	assert.Equal(t, int64(5), got.Age)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRepositorySelectColumns(t *testing.T) {
	q, _ := newMockQuerier(t)
	r, err := NewRepository[user](q, "users")
	require.NoError(t, err)

	sqlStr, args, err := r.Select().Build(gohan.Postgres())
	require.NoError(t, err)
	assert.Equal(t, `SELECT "id", "name", "email" FROM "users"`, sqlStr)
	assert.Empty(t, args)
}

func TestRepositoryGet(t *testing.T) {
	q, mock := newMockQuerier(t)
	r, err := NewRepository[user](q, "users")
	require.NoError(t, err)

	mock.ExpectQuery(`SELECT "id", "name", "email" FROM "users" WHERE "id" = $1 LIMIT 1`).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "email"}).AddRow(int64(7), "Bob", "bob@x.com"))

	got, err := r.Get(context.Background(), r.Select().Where(gohan.Col("id").Eq(int64(7))))
	require.NoError(t, err)
	assert.Equal(t, int64(7), got.ID)
	assert.Equal(t, "Bob", got.Name)
	assert.Equal(t, "bob@x.com", got.Email)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRepositoryGetNotFound(t *testing.T) {
	q, mock := newMockQuerier(t)
	r, err := NewRepository[user](q, "users")
	require.NoError(t, err)

	mock.ExpectQuery(`SELECT "id", "name", "email" FROM "users" LIMIT 1`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "email"}))

	got, err := r.Get(context.Background(), nil)
	assert.Nil(t, got)
	assert.True(t, errors.Is(err, ErrNotFound))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRepositoryListEmpty(t *testing.T) {
	q, mock := newMockQuerier(t)
	r, err := NewRepository[user](q, "users")
	require.NoError(t, err)

	mock.ExpectQuery(`SELECT "id", "name", "email" FROM "users"`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "email"}))

	got, err := r.List(context.Background(), nil)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Len(t, got, 0)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRepositoryGetByUnknownColumn(t *testing.T) {
	cq := &countingQuerier{d: gohan.Postgres()}
	r, err := NewRepository[user](cq, "users")
	require.NoError(t, err)

	got, err := r.GetBy(context.Background(), map[string]any{"bogus": 1})
	assert.Nil(t, got)
	assert.True(t, errors.Is(err, ErrUnknownColumn))
	assert.Equal(t, 0, cq.calls)
}

func TestRepositoryCountAndExists(t *testing.T) {
	q, mock := newMockQuerier(t)
	r, err := NewRepository[user](q, "users")
	require.NoError(t, err)

	mock.ExpectQuery(`SELECT COUNT(*) FROM "users" WHERE "id" = $1`).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(3)))
	n, err := r.Count(context.Background(), gohan.Col("id").Eq(int64(7)))
	require.NoError(t, err)
	assert.Equal(t, int64(3), n)

	existsSQL := `SELECT COUNT(*) FROM (SELECT 1 FROM "users" WHERE "id" = $1 LIMIT 1) AS "e"`

	mock.ExpectQuery(existsSQL).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(0)))
	ok, err := r.Exists(context.Background(), gohan.Col("id").Eq(int64(7)))
	require.NoError(t, err)
	assert.False(t, ok)

	mock.ExpectQuery(existsSQL).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(1)))
	ok, err = r.Exists(context.Background(), gohan.Col("id").Eq(int64(7)))
	require.NoError(t, err)
	assert.True(t, ok)

	require.NoError(t, mock.ExpectationsWereMet())
}

// --- writes ---

func TestRepositoryInsertSingle(t *testing.T) {
	q, mock := newMockQuerier(t)
	r, err := NewRepository[user](q, "users")
	require.NoError(t, err)

	mock.ExpectExec(`INSERT INTO "users" ("name", "email") VALUES ($1, $2)`).
		WithArgs("Bob", "bob@x.com").
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = r.Insert(context.Background(), &user{Name: "Bob", Email: "bob@x.com"})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRepositoryInsertChunked(t *testing.T) {
	mockDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer func() { _ = mockDB.Close() }()
	conn := sqlx.NewDb(mockDB, "sqlmock")
	q := NewSQL(conn, gohan.Generic())
	r, err := NewRepository[user](q, "users")
	require.NoError(t, err)

	records := make([]*user, 700)
	for i := range records {
		records[i] = &user{Name: "n", Email: "e"}
	}

	mock.ExpectBegin()
	mock.ExpectExec(chunkInsertSQL("users", []string{"name", "email"}, 333)).
		WithArgs(repeatAnyArg(666)...).
		WillReturnResult(sqlmock.NewResult(0, 333))
	mock.ExpectExec(chunkInsertSQL("users", []string{"name", "email"}, 333)).
		WithArgs(repeatAnyArg(666)...).
		WillReturnResult(sqlmock.NewResult(0, 333))
	mock.ExpectExec(chunkInsertSQL("users", []string{"name", "email"}, 34)).
		WithArgs(repeatAnyArg(68)...).
		WillReturnResult(sqlmock.NewResult(0, 34))
	mock.ExpectCommit()

	err = r.Insert(context.Background(), records...)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRepositoryInsertChunkedNoTx(t *testing.T) {
	cq := &countingQuerier{d: gohan.Generic()}
	r, err := NewRepository[user](cq, "users")
	require.NoError(t, err)

	records := make([]*user, 700)
	for i := range records {
		records[i] = &user{Name: "n", Email: "e"}
	}

	err = r.Insert(context.Background(), records...)
	assert.True(t, errors.Is(err, ErrTxUnsupported))
	assert.Equal(t, 0, cq.calls)
}

func TestRepositoryInsertBatchInserter(t *testing.T) {
	bi := &batchInserterDouble{countingQuerier: countingQuerier{d: gohan.Postgres()}}
	r, err := NewRepository[user](bi, "users")
	require.NoError(t, err)

	records := []*user{{Name: "a", Email: "a@x.com"}, {Name: "b", Email: "b@x.com"}}
	err = r.Insert(context.Background(), records...)
	require.NoError(t, err)
	assert.Equal(t, 1, bi.batchCalls)
	assert.Equal(t, "users", bi.gotTable)
	assert.Len(t, bi.gotRows, 2)
	assert.Equal(t, 0, bi.calls)
}

func TestRepositoryInsertEmpty(t *testing.T) {
	cq := &countingQuerier{d: gohan.Postgres()}
	r, err := NewRepository[user](cq, "users")
	require.NoError(t, err)

	err = r.Insert(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, cq.calls)
}

func TestRepositoryDeleteRequiresWhere(t *testing.T) {
	cq := &countingQuerier{d: gohan.Postgres()}
	r, err := NewRepository[user](cq, "users")
	require.NoError(t, err)

	_, err = r.Delete(context.Background(), nil)
	assert.True(t, errors.Is(err, gohan.ErrNoWhere))
	assert.Equal(t, 0, cq.calls)

	_, err = r.Delete(context.Background(), gohan.And())
	assert.True(t, errors.Is(err, gohan.ErrNoWhere), "got %v", err)
	assert.Equal(t, 0, cq.calls)

	_, err = r.Delete(context.Background(), gohan.Col("id").NotIn())
	assert.True(t, errors.Is(err, gohan.ErrNoWhere), "got %v", err)
	assert.Equal(t, 0, cq.calls)
}

func TestRepositoryUpdateRequiresWhere(t *testing.T) {
	cq := &countingQuerier{d: gohan.Postgres()}
	r, err := NewRepository[user](cq, "users")
	require.NoError(t, err)

	_, err = r.UpdateFields(context.Background(), map[string]any{"name": "x"}, nil)
	assert.True(t, errors.Is(err, gohan.ErrNoWhere))
	assert.Equal(t, 0, cq.calls)

	_, err = r.UpdateFields(context.Background(), map[string]any{"name": "x"}, gohan.And())
	assert.True(t, errors.Is(err, gohan.ErrNoWhere), "got %v", err)
	assert.Equal(t, 0, cq.calls)

	_, err = r.UpdateFields(context.Background(), map[string]any{"name": "x"}, gohan.Col("id").NotIn())
	assert.True(t, errors.Is(err, gohan.ErrNoWhere), "got %v", err)
	assert.Equal(t, 0, cq.calls)

	_, err = r.Update(context.Background(), &user{Name: "x"}, nil)
	assert.True(t, errors.Is(err, gohan.ErrNoWhere))
	assert.Equal(t, 0, cq.calls)
}

func TestRepositoryUpdateFieldsUnknownColumn(t *testing.T) {
	cq := &countingQuerier{d: gohan.Postgres()}
	r, err := NewRepository[user](cq, "users")
	require.NoError(t, err)

	_, err = r.UpdateFields(context.Background(), map[string]any{"bogus": "x"}, gohan.Col("id").Eq(int64(1)))
	assert.True(t, errors.Is(err, ErrUnknownColumn))
	assert.Equal(t, 0, cq.calls)
}

func TestRepositoryUpsertOmittedNotUpdated(t *testing.T) {
	q, mock := newMockQuerier(t)
	r, err := NewRepository[userOpt](q, "users")
	require.NoError(t, err)

	mock.ExpectExec(`INSERT INTO "users" ("name", "email") VALUES ($1, $2) ON CONFLICT ("email") DO UPDATE SET "name" = excluded."name"`).
		WithArgs("Bob", "bob@x.com").
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = r.Upsert(context.Background(), &userOpt{Name: "Bob", Email: "bob@x.com"}, []string{"email"})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRepositoryUpsertUnknownColumn(t *testing.T) {
	cq := &countingQuerier{d: gohan.Postgres()}
	r, err := NewRepository[user](cq, "users")
	require.NoError(t, err)

	err = r.Upsert(context.Background(), &user{Name: "a", Email: "b"}, []string{"bogus"})
	assert.True(t, errors.Is(err, ErrUnknownColumn))
	assert.Equal(t, 0, cq.calls)
}

func TestRepositoryUpsertDefaultColumns(t *testing.T) {
	q, mock := newMockQuerier(t)
	r, err := NewRepository[user](q, "users")
	require.NoError(t, err)

	mock.ExpectExec(`INSERT INTO "users" ("name", "email") VALUES ($1, $2) ON CONFLICT ("email") DO UPDATE SET "name" = excluded."name"`).
		WithArgs("Bob", "bob@x.com").
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = r.Upsert(context.Background(), &user{Name: "Bob", Email: "bob@x.com"}, []string{"email"})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRepositoryInsertReturning(t *testing.T) {
	q, mock := newMockQuerier(t)
	r, err := NewRepository[user](q, "users")
	require.NoError(t, err)

	mock.ExpectQuery(`INSERT INTO "users" ("name", "email") VALUES ($1, $2) RETURNING "id", "name", "email"`).
		WithArgs("Bob", "bob@x.com").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "email"}).AddRow(int64(1), "Bob", "bob@x.com"))

	got, err := r.InsertReturning(context.Background(), &user{Name: "Bob", Email: "bob@x.com"})
	require.NoError(t, err)
	assert.Equal(t, int64(1), got.ID)
	assert.Equal(t, "Bob", got.Name)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRepositoryWithTx(t *testing.T) {
	q, mock := newMockQuerier(t)
	r, err := NewRepository[user](q, "users")
	require.NoError(t, err)

	mock.ExpectBegin()
	mock.ExpectExec(`INSERT INTO "users" ("name", "email") VALUES ($1, $2)`).
		WithArgs("Bob", "bob@x.com").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	u := &user{Name: "Bob", Email: "bob@x.com"}
	err = WithTx(context.Background(), q, nil, func(tx Querier) error {
		return r.With(tx).Insert(context.Background(), u)
	})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRepositoryQueryGrid(t *testing.T) {
	q, mock := newMockQuerier(t)
	r, err := NewRepository[gridUser](q, "users")
	require.NoError(t, err)
	g, err := NewGrid[gridUser]()
	require.NoError(t, err)

	mock.ExpectQuery(`SELECT "id", "name", "email" FROM "users" WHERE "email" = $1 ORDER BY "name" ASC`).
		WithArgs("bob@x.com").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "email"}).AddRow(int64(1), "Bob", "bob@x.com"))

	got, err := r.QueryGrid(context.Background(), g, &GridQuery{
		FilterFields: map[string]any{"email": "bob@x.com"},
		SortFields:   map[string]string{"name": "asc"},
	})
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "Bob", got[0].Name)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRepositoryQueryGridInvalidQueryNeverHitsDB(t *testing.T) {
	cq := &countingQuerier{d: gohan.Postgres()}
	r, err := NewRepository[gridUser](cq, "users")
	require.NoError(t, err)
	g, err := NewGrid[gridUser]()
	require.NoError(t, err)

	_, err = r.QueryGrid(context.Background(), g, &GridQuery{FilterFields: map[string]any{"bogus": "x"}})
	require.Error(t, err)
	assert.Equal(t, 0, cq.calls)
}
