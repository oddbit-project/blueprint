package clickhouse

import (
	"context"
	"testing"

	"github.com/doug-martin/goqu/v9"
	"github.com/oddbit-project/blueprint/db"
	"github.com/stretchr/testify/assert"
)

// Test record struct
type TestRecord struct {
	ID    int    `ch:"id"`
	Name  string `ch:"name"`
	Value int    `ch:"value"`
}

func TestRepositorySqlBuilders(t *testing.T) {
	ctx := context.Background()
	repo := NewRepository(ctx, nil, "test_table")

	// Test SQL builders
	assert.NotNil(t, repo.Sql())
	assert.NotNil(t, repo.SqlSelect())
	assert.NotNil(t, repo.SqlInsert())
	assert.Nil(t, repo.SqlUpdate())    // not supported
	assert.NotNil(t, repo.SqlDelete()) // not supported

	// Verify correct table name for select and insert
	// Note: Update and Delete need values to be set before generating SQL
	selectSQL, _, err := repo.SqlSelect().ToSQL()
	assert.NoError(t, err)
	assert.Contains(t, selectSQL, "test_table")

	insertSQL, _, err := repo.SqlInsert().Rows(map[string]interface{}{"test": 1}).ToSQL()
	assert.NoError(t, err)
	assert.Contains(t, insertSQL, "test_table")
}

func TestRepositoryName(t *testing.T) {
	ctx := context.Background()
	repo := NewRepository(ctx, nil, "test_table")

	assert.Equal(t, "test_table", repo.Name())
}

func TestRepositoryDB(t *testing.T) {
	ctx := context.Background()
	repo := NewRepository(ctx, nil, "test_table")

	// Not supported for clickhouse
	assert.Nil(t, repo.Db())
}

func TestRepositoryConn(t *testing.T) {
	ctx := context.Background()
	repo := NewRepository(ctx, nil, "test_table")

	assert.Nil(t, repo.Conn())
}

func TestRepositoryParameterValidation(t *testing.T) {
	ctx := context.Background()
	repo := NewRepository(ctx, nil, "test_table")

	// Test with nil parameters
	err := repo.FetchOne(nil, &TestRecord{})
	assert.Equal(t, ErrInvalidParameters, err)

	err = repo.FetchOne(repo.SqlSelect(), nil)
	assert.Equal(t, ErrInvalidParameters, err)

	err = repo.Fetch(nil, &[]TestRecord{})
	assert.Equal(t, ErrInvalidParameters, err)

	err = repo.Fetch(repo.SqlSelect(), nil)
	assert.Equal(t, ErrInvalidParameters, err)

	err = repo.FetchRecord(nil, &TestRecord{})
	assert.Equal(t, ErrInvalidParameters, err)

	err = repo.FetchByKey("id", 1, nil)
	assert.Equal(t, ErrInvalidParameters, err)

	records := make([]TestRecord, 0)
	err = repo.FetchWhere(nil, &records)
	assert.Equal(t, ErrInvalidParameters, err)

	err = repo.Exec(nil)
	assert.Equal(t, ErrInvalidParameters, err)

	_, err = repo.Exists("id", 1, "name") // Only one skip param
	assert.Equal(t, ErrInvalidParameters, err)

	err = repo.Delete(nil)
	assert.Equal(t, ErrInvalidParameters, err)

	err = repo.DeleteWhere(nil)
	assert.Equal(t, ErrInvalidParameters, err)

	// Test InsertReturning - not supported
	record := &TestRecord{ID: 1, Name: "test", Value: 100}
	err = repo.InsertReturning(record, []string{"id"})
	assert.Equal(t, ErrNotSupported, err)
}

func TestDialectEscapesBackslash(t *testing.T) {
	qry := goqu.Dialect("clickhouse").From("t").Where(goqu.C("name").Eq(`x\' OR 1=1 --`))
	sqlQry, args, err := qry.ToSQL()
	assert.NoError(t, err)
	assert.Empty(t, args)
	assert.Equal(t, `SELECT * FROM "t" WHERE ("name" = 'x\\'' OR 1=1 --')`, sqlQry)
}

func TestRepositoryRejectsInvalidIdentifier(t *testing.T) {
	repo := NewRepository(context.Background(), nil, "test_table")
	bad := `a" = 1 OR "b`
	var rows []TestRecord

	assert.ErrorIs(t, repo.FetchRecord(map[string]any{bad: 1}, &TestRecord{}), db.ErrInvalidIdentifier)
	assert.ErrorIs(t, repo.FetchByKey(bad, 1, &TestRecord{}), db.ErrInvalidIdentifier)
	assert.ErrorIs(t, repo.FetchWhere(map[string]any{bad: 1}, &rows), db.ErrInvalidIdentifier)
	assert.ErrorIs(t, repo.DeleteWhere(map[string]any{bad: 1}), db.ErrInvalidIdentifier)
	assert.ErrorIs(t, repo.DeleteByKey(bad, 1), db.ErrInvalidIdentifier)

	_, err := repo.Exists(bad, 1)
	assert.ErrorIs(t, err, db.ErrInvalidIdentifier)
	_, err = repo.Exists("name", 1, `a\`, 1)
	assert.ErrorIs(t, err, db.ErrInvalidIdentifier)
	_, err = repo.Exists("name", 1, 1, 1)
	assert.ErrorIs(t, err, ErrInvalidParameters)
	_, err = repo.CountWhere(map[string]any{bad: 1})
	assert.ErrorIs(t, err, db.ErrInvalidIdentifier)
}

func TestRepositoryDeleteWhereEmptyMap(t *testing.T) {
	repo := NewRepository(context.Background(), nil, "test_table")
	assert.ErrorIs(t, repo.DeleteWhere(map[string]any{}), ErrInvalidParameters)
}
