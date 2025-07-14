package qb

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type ReturningTestUser struct {
	ID        int    `db:"id" auto:"true"`
	Name      string `db:"name"`
	Email     string `db:"email"`
	Age       int    `db:"age"`
	Active    bool   `db:"active"`
	UpdatedAt string `db:"updated_at"`
}

func TestUpdateBuilder_Returning(t *testing.T) {
	builder := NewSqlBuilder(DefaultSqlDialect())
	user := ReturningTestUser{
		Name:   "John Doe",
		Email:  "john@example.com",
		Age:    30,
		Active: true,
	}

	t.Run("returning single field", func(t *testing.T) {
		sql, args, err := builder.Update("users", user).
			ByID(1).
			Returning("id").
			Build()

		require.NoError(t, err)
		assert.Contains(t, sql, "UPDATE \"users\" SET")
		assert.Contains(t, sql, "WHERE \"id\" = ?")
		assert.Contains(t, sql, "RETURNING \"id\"")
		assert.Equal(t, []any{"John Doe", "john@example.com", 30, true, "", 1}, args)
	})

	t.Run("returning multiple fields", func(t *testing.T) {
		sql, args, err := builder.Update("users", user).
			ByID(1).
			Returning("id", "name", "updated_at").
			Build()

		require.NoError(t, err)
		assert.Contains(t, sql, "UPDATE \"users\" SET")
		assert.Contains(t, sql, "WHERE \"id\" = ?")
		assert.Contains(t, sql, "RETURNING \"id\", \"name\", \"updated_at\"")
		assert.Equal(t, []any{"John Doe", "john@example.com", 30, true, "", 1}, args)
	})

	t.Run("returning all fields", func(t *testing.T) {
		sql, args, err := builder.Update("users", user).
			ByID(1).
			ReturningAll().
			Build()

		require.NoError(t, err)
		assert.Contains(t, sql, "UPDATE \"users\" SET")
		assert.Contains(t, sql, "WHERE \"id\" = ?")
		assert.Contains(t, sql, "RETURNING *")
		assert.Equal(t, []any{"John Doe", "john@example.com", 30, true, "", 1}, args)
	})

	t.Run("add returning fields", func(t *testing.T) {
		sql, args, err := builder.Update("users", user).
			ByID(1).
			Returning("id", "name").
			AddReturning("email", "updated_at").
			Build()

		require.NoError(t, err)
		assert.Contains(t, sql, "UPDATE \"users\" SET")
		assert.Contains(t, sql, "WHERE \"id\" = ?")
		assert.Contains(t, sql, "RETURNING \"id\", \"name\", \"email\", \"updated_at\"")
		assert.Equal(t, []any{"John Doe", "john@example.com", 30, true, "", 1}, args)
	})

	t.Run("returning with complex WHERE", func(t *testing.T) {
		sql, args, err := builder.Update("users", user).
			WhereAnd(
				Eq("department", "IT"),
				Gt("age", 25),
			).
			Returning("id", "name", "updated_at").
			Build()

		require.NoError(t, err)
		assert.Contains(t, sql, "UPDATE \"users\" SET")
		assert.Contains(t, sql, "WHERE (\"department\" = ? AND \"age\" > ?)")
		assert.Contains(t, sql, "RETURNING \"id\", \"name\", \"updated_at\"")
		assert.Equal(t, []any{"John Doe", "john@example.com", 30, true, "", "IT", 25}, args)
	})

	t.Run("returning with field exclusion", func(t *testing.T) {
		sql, args, err := builder.Update("users", user).
			ByID(1).
			ExcludeFields("email", "age").
			Returning("id", "name", "updated_at").
			Build()

		require.NoError(t, err)
		assert.Contains(t, sql, "UPDATE \"users\" SET")
		assert.Contains(t, sql, "\"name\" = ?")
		assert.Contains(t, sql, "\"active\" = ?")
		assert.Contains(t, sql, "\"updated_at\" = ?")
		assert.NotContains(t, sql, "\"email\" = ?")
		assert.NotContains(t, sql, "\"age\" = ?")
		assert.Contains(t, sql, "RETURNING \"id\", \"name\", \"updated_at\"")
		assert.Equal(t, []any{"John Doe", true, "", 1}, args)
	})

	t.Run("no returning clause when not specified", func(t *testing.T) {
		sql, args, err := builder.Update("users", user).
			ByID(1).
			Build()

		require.NoError(t, err)
		assert.Contains(t, sql, "UPDATE \"users\" SET")
		assert.Contains(t, sql, "WHERE \"id\" = ?")
		assert.NotContains(t, sql, "RETURNING")
		assert.Equal(t, []any{"John Doe", "john@example.com", 30, true, "", 1}, args)
	})
}

func TestUpdateBuilder_ReturningWithOptions(t *testing.T) {
	builder := NewSqlBuilder(DefaultSqlDialect())
	user := ReturningTestUser{
		Name:   "Jane Doe",
		Email:  "jane@example.com",
		Age:    25,
		Active: false,
	}

	t.Run("returning with options", func(t *testing.T) {
		options := UpdateOptions{
			IncludeZeroValues: false,
			ReturningFields:   []string{"id", "name", "updated_at"},
		}

		sql, args, err := builder.Update("users", user).
			ByID(1).
			WithOptions(&options).
			Build()

		require.NoError(t, err)
		assert.Contains(t, sql, "UPDATE \"users\" SET")
		assert.Contains(t, sql, "\"name\" = ?")
		assert.Contains(t, sql, "\"email\" = ?")
		assert.Contains(t, sql, "\"age\" = ?")       // 25 is not zero value, so included
		assert.NotContains(t, sql, "\"active\" = ?") // false is zero value, so excluded
		assert.Contains(t, sql, "RETURNING \"id\", \"name\", \"updated_at\"")
		assert.Equal(t, []any{"Jane Doe", "jane@example.com", 25, 1}, args)
	})
}

func TestUpdateBuilder_ReturningWithLegacyAPI(t *testing.T) {
	builder := NewSqlBuilder(DefaultSqlDialect())
	user := ReturningTestUser{
		Name:   "Legacy User",
		Email:  "legacy@example.com",
		Age:    35,
		Active: true,
	}

	t.Run("returning with legacy BuildSQLUpdate", func(t *testing.T) {
		options := UpdateOptions{
			IncludeZeroValues: true,
			ReturningFields:   []string{"id", "updated_at"},
		}

		sql, args, err := builder.Update("users", user).Where(Eq("id", 1)).WithOptions(&options).Build()

		require.NoError(t, err)
		assert.Contains(t, sql, "UPDATE \"users\" SET")
		assert.Contains(t, sql, "WHERE \"id\" = ?")
		assert.Contains(t, sql, "RETURNING \"id\", \"updated_at\"")
		assert.Equal(t, []any{"Legacy User", "legacy@example.com", 35, true, "", 1}, args)
	})

	t.Run("returning with legacy BuildSQLUpdateByID", func(t *testing.T) {
		options := UpdateOptions{
			IncludeZeroValues: true,
			ReturningFields:   []string{"*"},
		}

		sql, args, err := builder.Update("users", user).WhereEq("id", 42).WithOptions(&options).Build()

		require.NoError(t, err)
		assert.Contains(t, sql, "UPDATE \"users\" SET")
		assert.Contains(t, sql, "WHERE \"id\" = ?")
		assert.Contains(t, sql, "RETURNING *")
		assert.Equal(t, []any{"Legacy User", "legacy@example.com", 35, true, "", 42}, args)
	})
}

func TestUpdateBuilder_ReturningValidation(t *testing.T) {
	builder := NewSqlBuilder(DefaultSqlDialect())
	user := ReturningTestUser{Name: "Test User"}

	t.Run("returning with empty fields list", func(t *testing.T) {
		sql, args, err := builder.Update("users", user).
			ByID(1).
			Returning().
			Build()

		require.NoError(t, err)
		assert.Contains(t, sql, "UPDATE \"users\" SET")
		assert.Contains(t, sql, "WHERE \"id\" = ?")
		assert.NotContains(t, sql, "RETURNING")
		assert.Equal(t, []any{"Test User", "", 0, false, "", 1}, args)
	})

	t.Run("returning fields are properly quoted", func(t *testing.T) {
		sql, args, err := builder.Update("users", user).
			ByID(1).
			Returning("user_id", "full_name", "created_at").
			Build()

		require.NoError(t, err)
		assert.Contains(t, sql, "RETURNING \"user_id\", \"full_name\", \"created_at\"")
		assert.Equal(t, []any{"Test User", "", 0, false, "", 1}, args)
	})

	t.Run("returning asterisk is not quoted", func(t *testing.T) {
		sql, args, err := builder.Update("users", user).
			ByID(1).
			ReturningAll().
			Build()

		require.NoError(t, err)
		assert.Contains(t, sql, "RETURNING *")
		assert.NotContains(t, sql, "RETURNING \"*\"")
		assert.Equal(t, []any{"Test User", "", 0, false, "", 1}, args)
	})
}
