package qb

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestBuildSQLBatchInsert_OmitNilBehavior(t *testing.T) {
	t.Run("batch insert with consistent omitnil behavior", func(t *testing.T) {
		builder := NewSqlBuilder(DefaultSqlDialect())

		type User struct {
			ID       int    `db:"id" auto:"true"`
			Name     string `db:"name"`
			Email    string `db:"email"`
			Age      *int   `db:"age" goqu:"omitnil"`
			IsActive *bool  `db:"is_active" goqu:"omitnil"`
		}

		// All records have nil age and nil isActive (consistent omit behavior)
		users := []any{
			User{
				Name:     "John Doe",
				Email:    "john@example.com",
				Age:      nil, // Omitted due to omitnil
				IsActive: nil, // Omitted due to omitnil
			},
			User{
				Name:     "Jane Smith",
				Email:    "jane@example.com",
				Age:      nil, // Omitted due to omitnil
				IsActive: nil, // Omitted due to omitnil
			},
			User{
				Name:     "Bob Johnson",
				Email:    "bob@example.com",
				Age:      nil, // Omitted due to omitnil
				IsActive: nil, // Omitted due to omitnil
			},
		}

		sql, args, err := builder.BuildSQLBatchInsert("users", users)
		require.NoError(t, err)

		// Only name and email columns should be included
		expectedSQL := `INSERT INTO "users" ("name", "email") VALUES (?, ?), (?, ?), (?, ?)`
		expectedArgs := []any{
			"John Doe", "john@example.com",
			"Jane Smith", "jane@example.com",
			"Bob Johnson", "bob@example.com",
		}

		assert.Equal(t, expectedSQL, sql)
		assert.Equal(t, expectedArgs, args)
	})

	t.Run("batch insert fails with inconsistent omitnil", func(t *testing.T) {
		builder := NewSqlBuilder(DefaultSqlDialect())

		type User struct {
			ID    int    `db:"id" auto:"true"`
			Name  string `db:"name"`
			Email string `db:"email"`
			Age   *int   `db:"age" goqu:"omitnil"`
		}

		age := 30
		users := []any{
			User{
				Name:  "John Doe",
				Email: "john@example.com",
				Age:   nil, // Omitted due to omitnil
			},
			User{
				Name:  "Jane Smith",
				Email: "jane@example.com",
				Age:   &age, // NOT omitted - inconsistent!
			},
		}

		_, _, err := builder.BuildSQLBatchInsert("users", users)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "field 'Age' has inconsistent omit behavior")
		assert.Contains(t, err.Error(), "omitted in first record but would be included in record 2")
	})

	t.Run("batch insert with consistent omitempty fields", func(t *testing.T) {
		builder := NewSqlBuilder(DefaultSqlDialect())

		type Product struct {
			ID          int    `db:"id" auto:"true"`
			Name        string `db:"name"`
			Description string `db:"description" goqu:"omitempty"`
			Category    string `db:"category" goqu:"omitempty"`
		}

		// All products have filled description and category (consistent)
		products := []any{
			Product{
				Name:        "Product 1",
				Description: "Full description",
				Category:    "Electronics",
			},
			Product{
				Name:        "Product 2",
				Description: "Another description",
				Category:    "Books",
			},
			Product{
				Name:        "Product 3",
				Description: "Third description",
				Category:    "Toys",
			},
		}

		sql, args, err := builder.BuildSQLBatchInsert("products", products)
		require.NoError(t, err)

		expectedSQL := `INSERT INTO "products" ("name", "description", "category") VALUES (?, ?, ?), (?, ?, ?), (?, ?, ?)`
		expectedArgs := []any{
			"Product 1", "Full description", "Electronics",
			"Product 2", "Another description", "Books",
			"Product 3", "Third description", "Toys",
		}

		assert.Equal(t, expectedSQL, sql)
		assert.Equal(t, expectedArgs, args)
	})

	t.Run("batch insert fails with inconsistent omitempty", func(t *testing.T) {
		builder := NewSqlBuilder(DefaultSqlDialect())

		type Product struct {
			ID          int    `db:"id" auto:"true"`
			Name        string `db:"name"`
			Description string `db:"description" goqu:"omitempty"`
		}

		products := []any{
			Product{
				Name:        "Product 1",
				Description: "", // Empty - omitted
			},
			Product{
				Name:        "Product 2",
				Description: "Has description", // Not empty - inconsistent!
			},
		}

		_, _, err := builder.BuildSQLBatchInsert("products", products)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "field 'Description' has inconsistent omit behavior")
	})

	t.Run("verify proper omit behavior in batch insert", func(t *testing.T) {
		builder := NewSqlBuilder(DefaultSqlDialect())

		type TestStruct struct {
			ID       int     `db:"id" auto:"true"`
			Required string  `db:"required"`
			Optional *string `db:"optional" goqu:"omitnil"`
		}

		// All records have non-nil optional values
		opt1 := "value1"
		opt2 := "value2"
		opt3 := "value3"
		data := []any{
			TestStruct{Required: "req1", Optional: &opt1},
			TestStruct{Required: "req2", Optional: &opt2},
			TestStruct{Required: "req3", Optional: &opt3},
		}

		sql, args, err := builder.BuildSQLBatchInsert("test_table", data)
		require.NoError(t, err)

		// All records have same columns
		expectedSQL := `INSERT INTO "test_table" ("required", "optional") VALUES (?, ?), (?, ?), (?, ?)`
		assert.Equal(t, expectedSQL, sql)

		// Verify values
		assert.Equal(t, args[0], "req1")
		assert.Equal(t, args[1], "value1")
		assert.Equal(t, args[2], "req2")
		assert.Equal(t, args[3], "value2")
		assert.Equal(t, args[4], "req3")
		assert.Equal(t, args[5], "value3")
	})
}

func TestBuildSQLBatchInsert_SingleInsertComparison(t *testing.T) {
	t.Run("omitnil behavior is consistent in batch insert", func(t *testing.T) {
		builder := NewSqlBuilder(DefaultSqlDialect())

		type User struct {
			ID    int    `db:"id" auto:"true"`
			Name  string `db:"name"`
			Email string `db:"email"`
			Age   *int   `db:"age" goqu:"omitnil"`
		}

		user := User{
			Name:  "John Doe",
			Email: "john@example.com",
			Age:   nil, // Should be omitted
		}

		// Single insert - omitnil fields are excluded from query
		singleSQL, singleArgs, err := builder.BuildSQLInsert("users", user)
		require.NoError(t, err)

		expectedSingleSQL := `INSERT INTO "users" ("name", "email") VALUES (?, ?)`
		expectedSingleArgs := []any{"John Doe", "john@example.com"}

		assert.Equal(t, expectedSingleSQL, singleSQL)
		assert.Equal(t, expectedSingleArgs, singleArgs)

		// Batch insert - now also respects omitnil (same as single insert)
		batchSQL, batchArgs, err := builder.BuildSQLBatchInsert("users", []any{user})
		require.NoError(t, err)

		expectedBatchSQL := `INSERT INTO "users" ("name", "email") VALUES (?, ?)`
		expectedBatchArgs := []any{"John Doe", "john@example.com"}

		assert.Equal(t, expectedBatchSQL, batchSQL)
		assert.Equal(t, expectedBatchArgs, batchArgs)
	})
}
