package qb

import (
	"errors"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSqlError_Error(t *testing.T) {
	t.Run("basic error", func(t *testing.T) {
		err := NewError("something went wrong", nil)
		assert.Equal(t, "something went wrong", err.Error())
	})

	t.Run("error with cause", func(t *testing.T) {
		cause := errors.New("root cause")
		err := NewError("something went wrong", cause)
		assert.Equal(t, "something went wrong: root cause", err.Error())
	})
}

func TestSqlError_Unwrap(t *testing.T) {
	cause := errors.New("root cause")
	err := NewError("something went wrong", cause)

	unwrapped := errors.Unwrap(err)
	assert.Equal(t, cause, unwrapped)
}

func TestSqlError_WithContext(t *testing.T) {
	err := NewError("test error", nil)
	err.WithContext("table", "users").WithContext("operation", "insert")

	assert.Equal(t, "users", err.Context["table"])
	assert.Equal(t, "insert", err.Context["operation"])
}

func TestErrorConstructors(t *testing.T) {
	t.Run("InvalidInputError", func(t *testing.T) {
		err := InvalidInputError("data is nil", nil)
		assert.Contains(t, err.Error(), "invalid input")
		assert.Contains(t, err.Error(), "data is nil")
	})

	t.Run("ValidationError", func(t *testing.T) {
		err := ValidationError("field cannot be empty")
		assert.Contains(t, err.Error(), "validation failed")
		assert.Contains(t, err.Error(), "field cannot be empty")
	})

	t.Run("StructParsingError", func(t *testing.T) {
		structType := reflect.TypeOf(struct{ ID int }{})
		cause := errors.New("parsing failed")
		err := StructParsingError(structType, cause)

		assert.Contains(t, err.Error(), "failed to parse struct")
		assert.Contains(t, err.Error(), structType.String())
		assert.Contains(t, err.Error(), "parsing failed")
	})

	t.Run("FieldMappingError", func(t *testing.T) {
		err := FieldMappingError("Name", "name", errors.New("mapping failed"))
		assert.Contains(t, err.Error(), "failed to map field Name to column name")
		assert.Contains(t, err.Error(), "mapping failed")
	})

	t.Run("DialectError", func(t *testing.T) {
		err := DialectError("invalid table name", errors.New("contains special chars"))
		assert.Contains(t, err.Error(), "dialect error")
		assert.Contains(t, err.Error(), "invalid table name")
		assert.Contains(t, err.Error(), "contains special chars")
	})

	t.Run("BatchProcessingError", func(t *testing.T) {
		err := BatchProcessingError(5, errors.New("record is nil"))
		assert.Contains(t, err.Error(), "failed to process record at index 5")
		assert.Contains(t, err.Error(), "record is nil")
	})
}

func TestValidationHelpers(t *testing.T) {
	t.Run("validateNotNil", func(t *testing.T) {
		// Test nil value
		err := validateNotNil(nil, "data")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "data cannot be nil")

		// Test non-nil value
		err = validateNotNil("not nil", "data")
		assert.NoError(t, err)
	})

	t.Run("validateNotEmpty", func(t *testing.T) {
		// Test empty string
		err := validateNotEmpty("", "table name")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "table name cannot be empty")

		// Test non-empty string
		err = validateNotEmpty("users", "table name")
		assert.NoError(t, err)
	})

	t.Run("validateSliceNotEmpty", func(t *testing.T) {
		// Test nil slice
		err := validateSliceNotEmpty(nil, "data")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "data cannot be nil")

		// Test non-slice
		err = validateSliceNotEmpty("not a slice", "data")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "data must be a slice")

		// Test empty slice
		var emptySlice []int
		err = validateSliceNotEmpty(emptySlice, "data")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "data cannot be empty")

		// Test non-empty slice
		nonEmptySlice := []int{1, 2, 3}
		err = validateSliceNotEmpty(nonEmptySlice, "data")
		assert.NoError(t, err)
	})

	t.Run("validateStructType", func(t *testing.T) {
		// Test struct type
		structType := reflect.TypeOf(struct{ ID int }{})
		err := validateStructType(structType, "data")
		assert.NoError(t, err)

		// Test pointer to struct type
		ptrStructType := reflect.TypeOf(&struct{ ID int }{})
		err = validateStructType(ptrStructType, "data")
		assert.NoError(t, err)

		// Test non-struct type
		stringType := reflect.TypeOf("string")
		err = validateStructType(stringType, "data")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "data must be a struct")
	})

	t.Run("validateTableName", func(t *testing.T) {
		// Test empty table name
		err := validateTableName("")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "table name cannot be empty")

		// Test table name with semicolon
		err = validateTableName("users; DROP TABLE users;")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "contains invalid characters")

		// Test table name with comment
		err = validateTableName("users--comment")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "contains invalid characters")

		// Test valid table name
		err = validateTableName("users")
		assert.NoError(t, err)

		// Test valid schema.table name
		err = validateTableName("public.users")
		assert.NoError(t, err)
	})
}

func TestErrorChaining(t *testing.T) {
	// Create a chain of errors
	rootCause := errors.New("database connection failed")
	wrappedErr := InvalidInputError("query failed", rootCause)

	// Test that we can unwrap to the root cause
	unwrapped := errors.Unwrap(wrappedErr)
	assert.Equal(t, rootCause, unwrapped)

	// Test that errors.Is works
	assert.True(t, errors.Is(wrappedErr, rootCause))
}

func TestErrorContext(t *testing.T) {
	err := NewError("test error", nil)
	err.WithContext("table", "users")
	err.WithContext("operation", "insert")
	err.WithContext("field", "name")

	assert.Equal(t, 3, len(err.Context))
	assert.Equal(t, "users", err.Context["table"])
	assert.Equal(t, "insert", err.Context["operation"])
	assert.Equal(t, "name", err.Context["field"])
}
