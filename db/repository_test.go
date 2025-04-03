package db

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
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