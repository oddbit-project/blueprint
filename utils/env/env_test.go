package env

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetEnvVar(t *testing.T) {
	// Test case: environment variable exists
	testEnvName := "TEST_GET_ENV_VAR"
	testEnvValue := "test-value-123"

	err := os.Setenv(testEnvName, testEnvValue)
	assert.NoError(t, err)
	defer os.Unsetenv(testEnvName)

	// First call should get from environment
	result := GetEnvVar(testEnvName)
	assert.Equal(t, testEnvValue, result)

	// Change the environment variable
	newValue := "new-value-456"
	err = os.Setenv(testEnvName, newValue)
	assert.NoError(t, err)

	// Second call should return cached value, not new value
	cachedResult := GetEnvVar(testEnvName)
	assert.Equal(t, testEnvValue, cachedResult)
	assert.NotEqual(t, newValue, cachedResult)

	// Test case: environment variable doesn't exist
	nonExistentVar := "NON_EXISTENT_TEST_VAR"
	os.Unsetenv(nonExistentVar) // Make sure it doesn't exist

	emptyResult := GetEnvVar(nonExistentVar)
	assert.Equal(t, "", emptyResult)
}

func TestSetEnvVar(t *testing.T) {
	// Test case: set new environment variable
	testEnvName := "TEST_SET_ENV_VAR"
	testEnvValue := "test-value-789"

	// Clean up before and after test
	os.Unsetenv(testEnvName)
	defer os.Unsetenv(testEnvName)

	// Set environment variable
	err := SetEnvVar(testEnvName, testEnvValue)
	assert.NoError(t, err)

	// Verify it was set in the actual environment
	actualValue := os.Getenv(testEnvName)
	assert.Equal(t, testEnvValue, actualValue)

	// Verify it was set in the cache (by calling GetEnvVar)
	cachedValue := GetEnvVar(testEnvName)
	assert.Equal(t, testEnvValue, cachedValue)

	// Test case: update existing environment variable
	updatedValue := "updated-value-789"
	err = SetEnvVar(testEnvName, updatedValue)
	assert.NoError(t, err)

	// Verify it was updated in both environment and cache
	actualUpdatedValue := os.Getenv(testEnvName)
	assert.Equal(t, updatedValue, actualUpdatedValue)

	cachedUpdatedValue := GetEnvVar(testEnvName)
	assert.Equal(t, updatedValue, cachedUpdatedValue)
}
