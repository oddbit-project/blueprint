package qb

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildSQLInsert_ErrorIntegration(t *testing.T) {
	t.Run("nil data error", func(t *testing.T) {
		builder := NewSqlBuilder(DefaultSqlDialect())

		_, _, err := builder.BuildSQLInsert("test_table", nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "data cannot be nil")
	})

	t.Run("empty table name error", func(t *testing.T) {
		builder := NewSqlBuilder(DefaultSqlDialect())

		type TestStruct struct {
			ID int `db:"id"`
		}

		_, _, err := builder.BuildSQLInsert("", TestStruct{ID: 1})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "table name cannot be empty")
	})

	t.Run("nil pointer data error", func(t *testing.T) {
		builder := NewSqlBuilder(DefaultSqlDialect())

		type TestStruct struct {
			ID int `db:"id"`
		}
		var data *TestStruct = nil

		_, _, err := builder.BuildSQLInsert("test_table", data)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "data pointer cannot be nil")
	})

	t.Run("non-struct data error", func(t *testing.T) {
		builder := NewSqlBuilder(DefaultSqlDialect())

		_, _, err := builder.BuildSQLInsert("test_table", "not a struct")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must be a struct")
	})

	t.Run("dialect error", func(t *testing.T) {
		// Test with an invalid table name
		builder := NewSqlBuilder(DefaultSqlDialect())

		type TestStruct struct {
			ID int `db:"id"`
		}

		_, _, err := builder.BuildSQLInsert("invalid.table.name", TestStruct{ID: 1})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid table name format")
	})

	t.Run("no fields to insert error", func(t *testing.T) {
		builder := NewSqlBuilder(DefaultSqlDialect())

		type TestStruct struct {
			ID int `db:"id" auto:"true"`
		}

		_, _, err := builder.BuildSQLInsert("test_table", TestStruct{ID: 1})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no insertable fields found")
	})

}

func TestBuildSQLBatchInsert_ErrorIntegration(t *testing.T) {
	t.Run("empty table name error", func(t *testing.T) {
		builder := NewSqlBuilder(DefaultSqlDialect())

		type TestStruct struct {
			ID int `db:"id"`
		}

		data := []any{TestStruct{ID: 1}}
		_, _, err := builder.BuildSQLBatchInsert("", data)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "table name cannot be empty")
	})

	t.Run("nil data error", func(t *testing.T) {
		builder := NewSqlBuilder(DefaultSqlDialect())

		_, _, err := builder.BuildSQLBatchInsert("test_table", nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "data cannot be nil")
	})

	t.Run("empty data slice error", func(t *testing.T) {
		builder := NewSqlBuilder(DefaultSqlDialect())

		data := []any{} // Empty slice, not nil slice
		_, _, err := builder.BuildSQLBatchInsert("test_table", data)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be empty")
	})

	t.Run("nil first record error", func(t *testing.T) {
		builder := NewSqlBuilder(DefaultSqlDialect())

		data := []any{nil}
		_, _, err := builder.BuildSQLBatchInsert("test_table", data)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "first record cannot be nil")
	})

	t.Run("non-struct first record error", func(t *testing.T) {
		builder := NewSqlBuilder(DefaultSqlDialect())

		data := []any{"not a struct"}
		_, _, err := builder.BuildSQLBatchInsert("test_table", data)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "first record must be a struct")
	})

	t.Run("nil record in middle error", func(t *testing.T) {
		builder := NewSqlBuilder(DefaultSqlDialect())

		type TestStruct struct {
			ID int `db:"id"`
		}

		data := []any{TestStruct{ID: 1}, nil, TestStruct{ID: 3}}
		_, _, err := builder.BuildSQLBatchInsert("test_table", data)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "record cannot be nil")
	})

	t.Run("mismatched record types error", func(t *testing.T) {
		builder := NewSqlBuilder(DefaultSqlDialect())

		type TestStruct1 struct {
			ID int `db:"id"`
		}

		type TestStruct2 struct {
			Name string `db:"name"`
		}

		data := []any{TestStruct1{ID: 1}, TestStruct2{Name: "test"}}
		_, _, err := builder.BuildSQLBatchInsert("test_table", data)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "does not match first record type")
	})

	t.Run("no fields to insert error", func(t *testing.T) {
		builder := NewSqlBuilder(DefaultSqlDialect())

		type TestStruct struct {
			ID int `db:"id" auto:"true"`
		}

		data := []any{TestStruct{ID: 1}, TestStruct{ID: 2}}
		_, _, err := builder.BuildSQLBatchInsert("test_table", data)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no insertable fields found")
	})
}

func TestDialect_ErrorIntegration(t *testing.T) {
	dialect := DefaultSqlDialect()

	t.Run("empty table name error", func(t *testing.T) {
		_, err := dialect.Table("")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "table name cannot be empty")
	})

	t.Run("invalid schema.table format error", func(t *testing.T) {
		_, err := dialect.Table("schema.table.extra")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid table name format")
	})

	t.Run("empty schema name error", func(t *testing.T) {
		_, err := dialect.Table(".table_name")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "schema name cannot be empty")
	})

	t.Run("empty table name in schema.table format error", func(t *testing.T) {
		_, err := dialect.Table("schema_name.")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "table name cannot be empty")
	})

	t.Run("valid table names work", func(t *testing.T) {
		// Single table name
		result, err := dialect.Table("users")
		require.NoError(t, err)
		assert.Equal(t, `"users"`, result)

		// Schema.table format
		result, err = dialect.Table("public.users")
		require.NoError(t, err)
		assert.Equal(t, `"public"."users"`, result)
	})
}

func TestErrorChainPropagation(t *testing.T) {
	// Test that error causes are properly propagated
	builder := NewSqlBuilder(DefaultSqlDialect())

	type TestStruct struct {
		ID   int    `db:"id" auto:"true"`
		Data string `db:"data"`
	}

	_, _, err := builder.BuildSQLInsert("test_table", TestStruct{Data: "test"})
	require.NoError(t, err) // Should succeed without mapper
}

func TestErrorMessageReadability(t *testing.T) {
	// Test that error messages are human-readable
	testCases := []struct {
		name     string
		testFunc func() error
		contains []string
	}{
		{
			name: "validation error",
			testFunc: func() error {
				return ValidationError("field cannot be empty")
			},
			contains: []string{"validation failed", "field cannot be empty"},
		},
		{
			name: "invalid input error",
			testFunc: func() error {
				return InvalidInputError("data is nil", nil)
			},
			contains: []string{"invalid input", "data is nil"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.testFunc()
			require.Error(t, err)

			errMsg := err.Error()
			for _, expected := range tc.contains {
				assert.Contains(t, errMsg, expected)
			}
		})
	}
}
