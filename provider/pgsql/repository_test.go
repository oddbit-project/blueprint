package pgsql

import (
	"context"
	"fmt"
	"github.com/doug-martin/goqu/v9"
	"github.com/oddbit-project/blueprint/db"
	"github.com/stretchr/testify/assert"
	"strconv"
	"testing"
	"time"
)

const (
	sampleTable    = "sample_table"
	sampleTableDDL = `create table sample_table(id_sample_table serial not null primary key, created_at timestamp with time zone, label text);`
)

type sampleRecord struct {
	Id        int       `db:"id_sample_table" goqu:"skipinsert"`
	CreatedAt time.Time `db:"created_at"`
	Label     string
}

type testRepository interface {
	db.Builder
	db.Reader
	db.Executor
	db.Writer
	db.Deleter
	db.Updater
}

func dbCleanup(t *testing.T, client *db.SqlClient) {
	_, err := client.Db().Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s", sampleTable))
	assert.Nil(t, err)
	_, err = client.Db().Exec(sampleTableDDL)
	assert.Nil(t, err)
}

func TestRepository(t *testing.T) {
	client := dbClient(t)
	dbCleanup(t, client)

	repo := db.NewRepository(context.Background(), client, sampleTable)
	testFunctions(t, repo)
}

func TestTransaction(t *testing.T) {
	client := dbClient(t)
	dbCleanup(t, client)

	repo := db.NewRepository(context.Background(), client, sampleTable)

	// test transaction with rollback
	tx, err := repo.NewTransaction(nil)
	assert.Nil(t, err)
	testFunctions(t, tx)
	assert.Nil(t, tx.Rollback())

	dbCleanup(t, client)

	// test transaction with commit
	tx, err = repo.NewTransaction(nil)
	assert.Nil(t, err)
	testFunctions(t, tx)
	assert.Nil(t, tx.Commit())

}

func testFunctions(t *testing.T, repo testRepository) {

	// Read non-existing record
	record := &sampleRecord{}
	err := repo.FetchOne(repo.SqlSelect(), record)
	assert.Error(t, err)
	assert.True(t, db.EmptyResult(err))

	newRecord1 := &sampleRecord{
		CreatedAt: time.Now(),
		Label:     "record 1",
	}
	// insert single record
	assert.Nil(t, repo.Insert(newRecord1))

	// read single existing record
	err = repo.FetchOne(repo.SqlSelect(), record)
	assert.Nil(t, err)
	assert.Equal(t, newRecord1.Label, record.Label)
	assert.Equal(t, newRecord1.CreatedAt.Unix(), record.CreatedAt.Unix())
	assert.True(t, record.Id > 0)

	// add several records
	records := make([]*sampleRecord, 0)
	for i := 2; i < 10; i++ {
		row := &sampleRecord{
			CreatedAt: time.Now(),
			Label:     "record " + strconv.Itoa(i),
		}
		records = append(records, row)
	}
	// insert multiple records
	assert.Nil(t, repo.Insert(records))

	// read multiple records
	records = make([]*sampleRecord, 0)
	assert.Nil(t, repo.Fetch(repo.SqlSelect(), &records))
	assert.Len(t, records, 9)
	// check that labels were read
	for i := 1; i < 9; i++ {
		assert.Equal(t, "record "+strconv.Itoa(i), records[i-1].Label)
	}

	// read record number 3
	assert.Nil(t, repo.FetchRecord(map[string]any{"label": "record 3"}, record))
	assert.Equal(t, "record 3", record.Label)

	// read record number 4, but to a slice
	records = make([]*sampleRecord, 0)
	assert.Nil(t, repo.FetchWhere(map[string]any{"label": "record 4"}, &records))
	assert.Equal(t, "record 4", records[0].Label)

	// use exists
	exists := false
	// non-existing record
	exists, err = repo.Exists("label", "record 999")
	assert.Nil(t, err)
	assert.False(t, exists)
	// existing record
	exists, err = repo.Exists("label", "record 8")
	assert.Nil(t, err)
	assert.True(t, exists)

	// existing record with skip value
	exists, err = repo.Exists("label", "record 4", "id_sample_table", records[0].Id)
	assert.Nil(t, err)
	assert.False(t, exists)

	// delete record number 4
	assert.Nil(t, repo.Delete(repo.SqlDelete().Where(goqu.C("id_sample_table").Eq(4))))
	// try to read deleted item, should fail
	record.Label = ""
	err = repo.FetchRecord(map[string]any{"label": "record 4"}, record)
	assert.Error(t, err)
	assert.True(t, db.EmptyResult(err))

	// delete record number 3
	assert.Nil(t, repo.DeleteWhere(map[string]any{"label": "record 3"}))
	// record number 3 deleted, should fail
	err = repo.FetchRecord(map[string]any{"label": "record 3"}, record)
	assert.Error(t, err)
	assert.True(t, db.EmptyResult(err))

	// insert returning
	record = &sampleRecord{
		CreatedAt: time.Now(),
		Label:     "something different",
	}
	assert.Nil(t, repo.InsertReturning(record, []any{"id_sample_table"}, &record.Id))
	assert.True(t, record.Id > 0)

	// update last insert using whole record
	record.Label = "foo"
	assert.Nil(t, repo.UpdateRecord(record, map[string]any{"label": "something different"}))
	// re-fetch using new label
	assert.Nil(t, repo.FetchRecord(map[string]any{"label": "foo"}, record))

	// update last insert using just label
	assert.Nil(t, repo.UpdateRecord(map[string]any{"label": "bar"}, map[string]any{"label": "foo"}))
	// re-fetch using new label
	assert.Nil(t, repo.FetchRecord(map[string]any{"label": "bar"}, record))

	// select via exec
	assert.Nil(t, repo.Exec(repo.SqlSelect().Where(goqu.C("label").Eq("bar"))))

	// delete cascade
	assert.Nil(t, repo.DeleteCascade(repo.SqlDelete()))

}
