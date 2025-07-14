package qb

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStruct for FieldsValues validation - it must match the fields being updated
type FieldsValuesTestStruct struct {
	ID       int    `db:"id" auto:"true"`
	Name     string `db:"name"`
	Email    string `db:"email"`
	Age      int    `db:"age"`
	Active   bool   `db:"active"`
	Score    float64 `db:"score"`
	Category string `db:"category"`
}

func TestUpdateBuilder_FieldsValues_Basic(t *testing.T) {
	builder := NewSqlBuilder(DefaultSqlDialect())
	
	// Empty struct is used for field validation
	testStruct := FieldsValuesTestStruct{}

	t.Run("simple field values update", func(t *testing.T) {
		fieldValues := map[string]any{
			"name":   "John Doe",
			"email":  "john@example.com",
			"age":    30,
			"active": true,
		}

		sql, args, err := builder.Update("users", testStruct).
			FieldsValues(fieldValues).
			WhereEq("id", 1).
			Build()

		require.NoError(t, err)
		assert.Contains(t, sql, "UPDATE \"users\" SET")
		assert.Contains(t, sql, "\"name\" = ?")
		assert.Contains(t, sql, "\"email\" = ?")
		assert.Contains(t, sql, "\"age\" = ?")
		assert.Contains(t, sql, "\"active\" = ?")
		assert.Contains(t, sql, "WHERE \"id\" = ?")
		// Args contain field values + WHERE value, order varies due to map iteration
		assert.Len(t, args, 5)
		assert.Contains(t, args, "John Doe")
		assert.Contains(t, args, "john@example.com")
		assert.Contains(t, args, 30)
		assert.Contains(t, args, true)
		assert.Contains(t, args, 1) // WHERE value
	})

	t.Run("single field update", func(t *testing.T) {
		fieldValues := map[string]any{
			"name": "Updated Name",
		}

		sql, args, err := builder.Update("users", testStruct).
			FieldsValues(fieldValues).
			WhereEq("id", 42).
			Build()

		require.NoError(t, err)
		assert.Contains(t, sql, "UPDATE \"users\" SET \"name\" = ?")
		assert.Contains(t, sql, "WHERE \"id\" = ?")
		assert.Equal(t, []any{"Updated Name", 42}, args)
	})

	t.Run("multiple data types", func(t *testing.T) {
		fieldValues := map[string]any{
			"name":     "Test User",
			"age":      25,
			"active":   false,
			"score":    98.5,
			"category": "premium",
		}

		sql, args, err := builder.Update("users", testStruct).
			FieldsValues(fieldValues).
			WhereEq("id", 1).
			Build()

		require.NoError(t, err)
		assert.Contains(t, sql, "UPDATE \"users\" SET")
		assert.Contains(t, sql, "\"name\" = ?")
		assert.Contains(t, sql, "\"age\" = ?")
		assert.Contains(t, sql, "\"active\" = ?")
		assert.Contains(t, sql, "\"score\" = ?")
		assert.Contains(t, sql, "\"category\" = ?")
		assert.Contains(t, sql, "WHERE \"id\" = ?")
		// Args contain field values + WHERE value, order varies due to map iteration
		assert.Len(t, args, 6)
		assert.Contains(t, args, "Test User")
		assert.Contains(t, args, 25)
		assert.Contains(t, args, false)
		assert.Contains(t, args, 98.5)
		assert.Contains(t, args, "premium")
		assert.Contains(t, args, 1) // WHERE value
	})

	t.Run("nil and zero values", func(t *testing.T) {
		fieldValues := map[string]any{
			"name":   "",     // Empty string
			"age":    0,      // Zero int
			"active": false,  // False boolean
			"score":  0.0,    // Zero float
		}

		sql, args, err := builder.Update("users", testStruct).
			FieldsValues(fieldValues).
			WhereEq("id", 1).
			Build()

		require.NoError(t, err)
		assert.Contains(t, sql, "UPDATE \"users\" SET")
		assert.Contains(t, sql, "\"name\" = ?")
		assert.Contains(t, sql, "\"age\" = ?")
		assert.Contains(t, sql, "\"active\" = ?")
		assert.Contains(t, sql, "\"score\" = ?")
		// Args contain field values + WHERE value, order varies due to map iteration
		assert.Len(t, args, 5)
		assert.Contains(t, args, "")
		assert.Contains(t, args, 0)
		assert.Contains(t, args, false)
		assert.Contains(t, args, 0.0)
		assert.Contains(t, args, 1) // WHERE value
	})
}

func TestUpdateBuilder_FieldsValues_WithComplexWhere(t *testing.T) {
	builder := NewSqlBuilder(DefaultSqlDialect())
	testStruct := FieldsValuesTestStruct{}

	t.Run("with AND conditions", func(t *testing.T) {
		fieldValues := map[string]any{
			"active": false,
			"score":  85.5,
		}

		sql, args, err := builder.Update("users", testStruct).
			FieldsValues(fieldValues).
			WhereAnd(
				Eq("category", "premium"),
				Gt("age", 18),
			).
			Build()

		require.NoError(t, err)
		assert.Contains(t, sql, "UPDATE \"users\" SET")
		assert.Contains(t, sql, "\"active\" = ?")
		assert.Contains(t, sql, "\"score\" = ?")
		assert.Contains(t, sql, "WHERE (\"category\" = ? AND \"age\" > ?)")
		// Args contain field values + WHERE values, order varies due to map iteration
		assert.Len(t, args, 4)
		assert.Contains(t, args, false)
		assert.Contains(t, args, 85.5)
		assert.Contains(t, args, "premium")
		assert.Contains(t, args, 18)
	})

	t.Run("with OR conditions", func(t *testing.T) {
		fieldValues := map[string]any{
			"category": "standard",
		}

		sql, args, err := builder.Update("users", testStruct).
			FieldsValues(fieldValues).
			WhereOr(
				Eq("email", "old@example.com"),
				Eq("email", "legacy@example.com"),
			).
			Build()

		require.NoError(t, err)
		assert.Contains(t, sql, "UPDATE \"users\" SET \"category\" = ?")
		assert.Contains(t, sql, "WHERE (\"email\" = ? OR \"email\" = ?)")
		// Args contain field values + WHERE values, order varies due to map iteration
		assert.Len(t, args, 3)
		assert.Contains(t, args, "standard")
		assert.Contains(t, args, "old@example.com")
		assert.Contains(t, args, "legacy@example.com")
	})

	t.Run("with complex nested conditions", func(t *testing.T) {
		fieldValues := map[string]any{
			"active": true,
			"score":  100.0,
		}

		sql, args, err := builder.Update("users", testStruct).
			FieldsValues(fieldValues).
			WhereAnd(
				Or(
					Eq("category", "premium"),
					Eq("category", "vip"),
				),
				Gte("age", 21),
				Lt("score", 90.0),
			).
			Build()

		require.NoError(t, err)
		assert.Contains(t, sql, "UPDATE \"users\" SET")
		assert.Contains(t, sql, "\"active\" = ?")
		assert.Contains(t, sql, "\"score\" = ?")
		assert.Contains(t, sql, "WHERE ((\"category\" = ? OR \"category\" = ?) AND \"age\" >= ? AND \"score\" < ?)")
		// Args contain field values + WHERE values, order varies due to map iteration
		assert.Len(t, args, 6)
		assert.Contains(t, args, true)
		assert.Contains(t, args, 100.0)
		assert.Contains(t, args, "premium")
		assert.Contains(t, args, "vip")
		assert.Contains(t, args, 21)
		assert.Contains(t, args, 90.0)
	})
}

func TestUpdateBuilder_FieldsValues_WithReturning(t *testing.T) {
	builder := NewSqlBuilder(DefaultSqlDialect())
	testStruct := FieldsValuesTestStruct{}

	t.Run("with returning fields", func(t *testing.T) {
		fieldValues := map[string]any{
			"name":  "Updated User",
			"email": "updated@example.com",
		}

		sql, args, err := builder.Update("users", testStruct).
			FieldsValues(fieldValues).
			WhereEq("id", 1).
			Returning("id", "name", "email").
			Build()

		require.NoError(t, err)
		assert.Contains(t, sql, "UPDATE \"users\" SET")
		assert.Contains(t, sql, "\"name\" = ?")
		assert.Contains(t, sql, "\"email\" = ?")
		assert.Contains(t, sql, "WHERE \"id\" = ?")
		assert.Contains(t, sql, "RETURNING \"id\", \"name\", \"email\"")
		// Args contain field values + WHERE value, order varies due to map iteration
		assert.Len(t, args, 3)
		assert.Contains(t, args, "Updated User")
		assert.Contains(t, args, "updated@example.com")
		assert.Contains(t, args, 1)
	})

	t.Run("with returning all", func(t *testing.T) {
		fieldValues := map[string]any{
			"score": 95.0,
		}

		sql, args, err := builder.Update("users", testStruct).
			FieldsValues(fieldValues).
			WhereEq("id", 1).
			ReturningAll().
			Build()

		require.NoError(t, err)
		assert.Contains(t, sql, "UPDATE \"users\" SET \"score\" = ?")
		assert.Contains(t, sql, "WHERE \"id\" = ?")
		assert.Contains(t, sql, "RETURNING *")
		// Args contain field values + WHERE value, order varies due to map iteration
		assert.Len(t, args, 2)
		assert.Contains(t, args, 95.0)
		assert.Contains(t, args, 1)
	})
}

func TestUpdateBuilder_FieldsValues_WithOptions(t *testing.T) {
	builder := NewSqlBuilder(DefaultSqlDialect())
	testStruct := FieldsValuesTestStruct{}

	t.Run("with update options", func(t *testing.T) {
		fieldValues := map[string]any{
			"name":   "Test User",
			"email":  "test@example.com",
			"active": true,
		}

		options := &UpdateOptions{
			ReturningFields: []string{"id", "name"},
		}

		sql, args, err := builder.Update("users", testStruct).
			FieldsValues(fieldValues).
			WhereEq("id", 1).
			WithOptions(options).
			Build()

		require.NoError(t, err)
		assert.Contains(t, sql, "UPDATE \"users\" SET")
		assert.Contains(t, sql, "\"name\" = ?")
		assert.Contains(t, sql, "\"email\" = ?")
		assert.Contains(t, sql, "\"active\" = ?")
		assert.Contains(t, sql, "WHERE \"id\" = ?")
		assert.Contains(t, sql, "RETURNING \"id\", \"name\"")
		// Args contain field values + WHERE value, order varies due to map iteration
		assert.Len(t, args, 4)
		assert.Contains(t, args, "Test User")
		assert.Contains(t, args, "test@example.com")
		assert.Contains(t, args, true)
		assert.Contains(t, args, 1)
	})

	t.Run("fluent API overrides options", func(t *testing.T) {
		fieldValues := map[string]any{
			"name": "Override Test",
		}

		sql, args, err := builder.Update("users", testStruct).
			FieldsValues(fieldValues).
			WhereEq("id", 1).
			WithOptions(&UpdateOptions{
				ReturningFields: []string{"name"}, // This gets overridden
			}).
			Returning("id").          // This should override any options
			Build()

		require.NoError(t, err)
		assert.Contains(t, sql, "RETURNING \"id\"") // Should be "id", not "name"
		// Args contain field values + WHERE value, order varies due to map iteration
		assert.Len(t, args, 2)
		assert.Contains(t, args, "Override Test")
		assert.Contains(t, args, 1)
	})
}

func TestUpdateBuilder_FieldsValues_ErrorCases(t *testing.T) {
	builder := NewSqlBuilder(DefaultSqlDialect())
	testStruct := FieldsValuesTestStruct{}

	t.Run("nil field values falls back to struct", func(t *testing.T) {
		sql, args, err := builder.Update("users", testStruct).
			FieldsValues(nil).
			WhereEq("id", 1).
			Build()

		// nil fieldValues should fall back to struct-based update (this is valid behavior)
		require.NoError(t, err)
		assert.Contains(t, sql, "UPDATE \"users\" SET")
		assert.Contains(t, sql, "WHERE \"id\" = ?")
		// Should update all struct fields since it falls back to struct-based update
		assert.Greater(t, len(args), 1) // At least the WHERE argument
	})

	t.Run("empty field values", func(t *testing.T) {
		fieldValues := map[string]any{}

		sql, _, err := builder.Update("users", testStruct).
			FieldsValues(fieldValues).
			WhereEq("id", 1).
			Build()

		require.NoError(t, err) // Empty map should be allowed
		assert.Contains(t, sql, "UPDATE \"users\" SET")
		assert.Contains(t, sql, "WHERE \"id\" = ?")
	})

	t.Run("invalid field name", func(t *testing.T) {
		fieldValues := map[string]any{
			"invalid_field": "some value",
		}

		sql, args, err := builder.Update("users", testStruct).
			FieldsValues(fieldValues).
			WhereEq("id", 1).
			Build()

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "db field not found in struct")
		assert.Empty(t, sql)
		assert.Nil(t, args)
	})

	t.Run("mixed valid and invalid fields", func(t *testing.T) {
		fieldValues := map[string]any{
			"name":          "Valid Name",
			"invalid_field": "Invalid",
		}

		sql, args, err := builder.Update("users", testStruct).
			FieldsValues(fieldValues).
			WhereEq("id", 1).
			Build()

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "db field not found in struct")
		assert.Empty(t, sql)
		assert.Nil(t, args)
	})

	t.Run("nil WHERE clause", func(t *testing.T) {
		fieldValues := map[string]any{
			"name": "Test",
		}

		sql, args, err := builder.Update("users", testStruct).
			FieldsValues(fieldValues).
			Build()

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "WHERE clause is required")
		assert.Empty(t, sql)
		assert.Nil(t, args)
	})

	t.Run("empty table name", func(t *testing.T) {
		fieldValues := map[string]any{
			"name": "Test",
		}

		sql, args, err := builder.Update("", testStruct).
			FieldsValues(fieldValues).
			WhereEq("id", 1).
			Build()

		assert.Error(t, err)
		assert.Empty(t, sql)
		assert.Nil(t, args)
	})

	t.Run("nil record for field validation", func(t *testing.T) {
		fieldValues := map[string]any{
			"name": "Test",
		}

		sql, args, err := builder.Update("users", nil).
			FieldsValues(fieldValues).
			WhereEq("id", 1).
			Build()

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "record cannot be nil")
		assert.Empty(t, sql)
		assert.Nil(t, args)
	})
}

func TestUpdateBuilder_FieldsValues_Dialects(t *testing.T) {
	testStruct := FieldsValuesTestStruct{}

	t.Run("MySQL dialect", func(t *testing.T) {
		dialect := SqlDialect{
			PlaceHolderFragment: "?",
			QuoteField:         "`%s`",
			QuoteTable:         "`%s`",
		}
		builder := NewSqlBuilder(dialect)

		fieldValues := map[string]any{
			"name": "MySQL User",
			"age":  25,
		}

		sql, args, err := builder.Update("users", testStruct).
			FieldsValues(fieldValues).
			WhereEq("id", 1).
			Build()

		require.NoError(t, err)
		assert.Contains(t, sql, "UPDATE `users` SET")
		assert.Contains(t, sql, "`name` = ?")
		assert.Contains(t, sql, "`age` = ?")
		assert.Contains(t, sql, "WHERE `id` = ?")
		// Args contain field values + WHERE value, order varies due to map iteration
		assert.Len(t, args, 3)
		assert.Contains(t, args, "MySQL User")
		assert.Contains(t, args, 25)
		assert.Contains(t, args, 1)
	})

	t.Run("PostgreSQL dialect", func(t *testing.T) {
		dialect := SqlDialect{
			PlaceHolderFragment:   "$",
			IncludePlaceholderNum: true,
			QuoteField:           "\"%s\"",
			QuoteTable:           "\"%s\"",
		}
		builder := NewSqlBuilder(dialect)

		fieldValues := map[string]any{
			"email":  "postgres@example.com",
			"active": true,
		}

		sql, args, err := builder.Update("users", testStruct).
			FieldsValues(fieldValues).
			WhereEq("id", 42).
			Build()

		require.NoError(t, err)
		assert.Contains(t, sql, "UPDATE \"users\" SET")
		// PostgreSQL placeholders can be in any order due to map iteration
		assert.Regexp(t, `"email" = \$[12]`, sql)
		assert.Regexp(t, `"active" = \$[12]`, sql)
		assert.Contains(t, sql, "WHERE \"id\" = $3")
		// Args contain field values + WHERE value, order varies due to map iteration
		assert.Len(t, args, 3)
		assert.Contains(t, args, "postgres@example.com")
		assert.Contains(t, args, true)
		assert.Contains(t, args, 42)
	})
}

func TestUpdateBuilder_FieldsValues_ChainedMethods(t *testing.T) {
	builder := NewSqlBuilder(DefaultSqlDialect())
	testStruct := FieldsValuesTestStruct{}

	t.Run("chained fluent interface", func(t *testing.T) {
		fieldValues := map[string]any{
			"name":     "Chained User",
			"category": "gold",
		}

		sql, args, err := builder.Update("users", testStruct).
			FieldsValues(fieldValues).
			WhereAnd(
				Eq("active", true),
				Gt("score", 80),
			).
			Returning("id", "name").
			IncludeZeroValues(false).
			Build()

		require.NoError(t, err)
		assert.Contains(t, sql, "UPDATE \"users\" SET")
		assert.Contains(t, sql, "\"name\" = ?")
		assert.Contains(t, sql, "\"category\" = ?")
		assert.Contains(t, sql, "WHERE (\"active\" = ? AND \"score\" > ?)")
		assert.Contains(t, sql, "RETURNING \"id\", \"name\"")
		// Args contain field values + WHERE values, order varies due to map iteration
		assert.Len(t, args, 4)
		assert.Contains(t, args, "Chained User")
		assert.Contains(t, args, "gold")
		assert.Contains(t, args, true)
		assert.Contains(t, args, 80)
	})

	t.Run("method order independence", func(t *testing.T) {
		fieldValues := map[string]any{
			"score": 95.0,
		}

		// Different order should produce same result
		sql1, args1, err1 := builder.Update("users", testStruct).
			WhereEq("id", 1).
			FieldsValues(fieldValues).
			Returning("score").
			Build()

		sql2, args2, err2 := builder.Update("users", testStruct).
			Returning("score").
			FieldsValues(fieldValues).
			WhereEq("id", 1).
			Build()

		require.NoError(t, err1)
		require.NoError(t, err2)
		assert.Equal(t, sql1, sql2)
		assert.Equal(t, args1, args2)
	})
}

func TestUpdateBuilder_FieldsValues_Integration(t *testing.T) {
	builder := NewSqlBuilder(DefaultSqlDialect())
	testStruct := FieldsValuesTestStruct{}

	t.Run("real world scenario - user profile update", func(t *testing.T) {
		// Simulate partial profile update
		fieldValues := map[string]any{
			"name":     "John Updated",
			"email":    "john.updated@example.com", 
			"category": "premium",
		}

		sql, args, err := builder.Update("user_profiles", testStruct).
			FieldsValues(fieldValues).
			WhereEq("id", 12345).
			Returning("id", "name", "email", "category").
			Build()

		require.NoError(t, err)
		assert.Contains(t, sql, "UPDATE \"user_profiles\" SET")
		assert.Contains(t, sql, "\"name\" = ?")
		assert.Contains(t, sql, "\"email\" = ?")
		assert.Contains(t, sql, "\"category\" = ?")
		assert.Contains(t, sql, "WHERE \"id\" = ?")
		assert.Contains(t, sql, "RETURNING \"id\", \"name\", \"email\", \"category\"")
		// Args contain field values + WHERE value, order varies due to map iteration
		assert.Len(t, args, 4)
		assert.Contains(t, args, "John Updated")
		assert.Contains(t, args, "john.updated@example.com")
		assert.Contains(t, args, "premium")
		assert.Contains(t, args, 12345)
	})

	t.Run("bulk field update with complex conditions", func(t *testing.T) {
		fieldValues := map[string]any{
			"active":   false,
			"category": "inactive",
		}

		sql, args, err := builder.Update("users", testStruct).
			FieldsValues(fieldValues).
			WhereAnd(
				Lt("score", 50),
				Eq("active", true),
				In("category", "basic", "standard"),
			).
			Returning("id").
			Build()

		require.NoError(t, err)
		assert.Contains(t, sql, "UPDATE \"users\" SET")
		assert.Contains(t, sql, "\"active\" = ?")
		assert.Contains(t, sql, "\"category\" = ?")
		assert.Contains(t, sql, "WHERE (\"score\" < ? AND \"active\" = ? AND \"category\" IN (?, ?))")
		assert.Contains(t, sql, "RETURNING \"id\"")
		// Args contain field values + WHERE values, order varies due to map iteration
		assert.Len(t, args, 6)
		assert.Contains(t, args, false)
		assert.Contains(t, args, "inactive")
		assert.Contains(t, args, 50)
		assert.Contains(t, args, true)
		assert.Contains(t, args, "basic")
		assert.Contains(t, args, "standard")
	})
}

func BenchmarkUpdateBuilder_FieldsValues(b *testing.B) {
	builder := NewSqlBuilder(DefaultSqlDialect())
	testStruct := FieldsValuesTestStruct{}

	fieldValues := map[string]any{
		"name":     "Benchmark User",
		"email":    "benchmark@example.com",
		"age":      30,
		"active":   true,
		"score":    85.5,
		"category": "premium",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, err := builder.Update("users", testStruct).
			FieldsValues(fieldValues).
			WhereEq("id", i).
			Build()
		if err != nil {
			b.Fatal(err)
		}
	}
}