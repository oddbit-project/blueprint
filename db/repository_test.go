package db

import (
	"database/sql"
	"github.com/stretchr/testify/assert"
	"testing"
)

// TestEmptyResult checks the EmptyResult helper function
func TestEmptyResult(t *testing.T) {
	// Test with sql.ErrNoRows
	assert.True(t, EmptyResult(sql.ErrNoRows))
	
	// Test with non-empty result error
	assert.False(t, EmptyResult(sql.ErrConnDone))
	
	// Test with nil error
	assert.False(t, EmptyResult(nil))
}

// TestRepositoryInterfaces ensures that the interfaces are properly defined
func TestRepositoryInterfaces(t *testing.T) {
	// Just verify that the interface definitions are correct
	// This is a compilation check rather than a runtime check
	var _ Repository = (*repository)(nil)
	var _ Transaction = (*tx)(nil)

	var _ Reader = (*repository)(nil)
	var _ Reader = (*tx)(nil)

	var _ Writer = (*repository)(nil)
	var _ Writer = (*tx)(nil)

	var _ Builder = (*repository)(nil)
	var _ Builder = (*tx)(nil)

	var _ Executor = (*repository)(nil)
	var _ Executor = (*tx)(nil)

	var _ Counter = (*repository)(nil)
	var _ Counter = (*tx)(nil)

	var _ Deleter = (*repository)(nil)
	var _ Deleter = (*tx)(nil)

	var _ Updater = (*repository)(nil)
	var _ Updater = (*tx)(nil)
	
	var _ GridOps = (*repository)(nil)
}

// TestFVAlias ensures that the FV alias works as expected
func TestFVAlias(t *testing.T) {
	// Test the FV alias for fieldValues maps
	fieldValues := FV{
		"name": "John",
		"age":  30,
	}

	assert.Equal(t, "John", fieldValues["name"])
	assert.Equal(t, 30, fieldValues["age"])
}

// TestRepositoryTypes verifies that types exist
func TestRepositoryTypes(t *testing.T) {
	// Check that repository struct types match expectations
	var r *repository
	assert.Nil(t, r)
	
	var transaction *tx
	assert.Nil(t, transaction)
}

// TestGridStruct is a test struct for GridOps tests
type TestGridStruct struct {
	ID      int    `db:"id" json:"id" grid:"sort,filter"`
	Name    string `db:"name" json:"name" grid:"sort,search,filter"`
	Email   string `db:"email" json:"email" grid:"sort,search,filter"`
	Active  bool   `db:"active" json:"active" grid:"filter"`
	Created string `db:"created" json:"created" grid:"sort"`
}

// TestRepositoryGrid tests the Grid method of the repository
func TestRepositoryGrid(t *testing.T) {
	// Create a repository with a nil database connection 
	// (we're only testing the Grid method which doesn't use the database)
	repo := &repository{
		tableName: "test_table",
	}
	
	// Test creating a grid with a new record type
	grid, err := repo.Grid(&TestGridStruct{})
	assert.NoError(t, err)
	assert.NotNil(t, grid)
	assert.Equal(t, "test_table", grid.tableName)
	assert.NotNil(t, grid.spec)
	
	// Test creating another grid with the same record type
	grid2, err := repo.Grid(&TestGridStruct{})
	assert.NoError(t, err)
	assert.NotNil(t, grid2)
	assert.Equal(t, "test_table", grid2.tableName)
	assert.NotNil(t, grid2.spec)
}

// TestRepositoryQueryGrid_InvalidRecord tests the QueryGrid method with an invalid record
func TestRepositoryQueryGrid_InvalidRecord(t *testing.T) {
	// Create a repository without a database connection
	// (we'll only test the error case that doesn't use the connection)
	repo := &repository{
		tableName: "test_users",
	}
	
	// Create a grid query
	query, err := NewGridQuery(SearchAny, 10, 0)
	assert.NoError(t, err)
	
	// Create a destination slice
	var users []*TestGridStruct
	
	// Test with an invalid record
	err = repo.QueryGrid(nil, query, &users)
	assert.Error(t, err)
}