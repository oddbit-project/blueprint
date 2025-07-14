package qb

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type WhereTestUser struct {
	ID       int                    `db:"id" auto:"true"`
	Name     string                 `db:"name"`
	Email    string                 `db:"email"`
	Age      int                    `db:"age"`
	Active   bool                   `db:"active"`
	Settings map[string]interface{} `db:"settings" goqu:"omitempty"`
}

func TestBuildSQLUpdateWhere_SimpleCondition(t *testing.T) {
	builder := NewSqlBuilder(DefaultSqlDialect())
	user := WhereTestUser{
		Name:   "John Doe",
		Email:  "john@example.com",
		Age:    30,
		Active: true,
	}

	whereClause := Eq("id", 1)
	sql, args, err := builder.Update("users", user).Where(whereClause).Build()

	require.NoError(t, err)
	assert.Contains(t, sql, "UPDATE \"users\"")
	assert.Contains(t, sql, "\"name\" = ?")
	assert.Contains(t, sql, "\"email\" = ?")
	assert.Contains(t, sql, "\"age\" = ?")
	assert.Contains(t, sql, "\"active\" = ?")
	assert.Contains(t, sql, "WHERE \"id\" = ?")
	assert.Equal(t, []any{"John Doe", "john@example.com", 30, true, 1}, args)
}

func TestBuildSQLUpdateWhere_AndCondition(t *testing.T) {
	builder := NewSqlBuilder(DefaultSqlDialect())
	user := WhereTestUser{Name: "Jane Doe", Email: "jane@example.com"}

	whereClause := And(
		Eq("id", 1),
		Gt("age", 18),
	)
	sql, args, err := builder.Update("users", user).Where(whereClause).Build()

	require.NoError(t, err)
	assert.Contains(t, sql, "WHERE (\"id\" = ? AND \"age\" > ?)")
	assert.Equal(t, []any{"Jane Doe", "jane@example.com", 0, false, 1, 18}, args)
}

func TestBuildSQLUpdateWhere_OrCondition(t *testing.T) {
	builder := NewSqlBuilder(DefaultSqlDialect())
	user := WhereTestUser{Name: "Bob Smith"}

	whereClause := Or(
		Eq("email", "bob@example.com"),
		Eq("email", "bob.smith@example.com"),
	)
	sql, args, err := builder.Update("users", user).Where(whereClause).Build()

	require.NoError(t, err)
	assert.Contains(t, sql, "WHERE (\"email\" = ? OR \"email\" = ?)")
	assert.Equal(t, []any{"Bob Smith", "", 0, false, "bob@example.com", "bob.smith@example.com"}, args)
}

func TestBuildSQLUpdateWhere_ComplexCondition(t *testing.T) {
	builder := NewSqlBuilder(DefaultSqlDialect())
	user := WhereTestUser{Active: false}

	whereClause := And(
		Or(
			Eq("department", "IT"),
			Eq("department", "Engineering"),
		),
		Gt("age", 25),
		Lt("age", 65),
	)
	sql, args, err := builder.Update("users", user).Where(whereClause).Build()

	require.NoError(t, err)
	assert.Contains(t, sql, "WHERE ((\"department\" = ? OR \"department\" = ?) AND \"age\" > ? AND \"age\" < ?)")
	assert.Equal(t, []any{"", "", 0, false, "IT", "Engineering", 25, 65}, args)
}

func TestBuildSQLUpdateWhere_InCondition(t *testing.T) {
	builder := NewSqlBuilder(DefaultSqlDialect())
	user := WhereTestUser{Active: true}

	whereClause := In("status", "active", "pending", "verified")
	sql, args, err := builder.Update("users", user).Where(whereClause).Build()

	require.NoError(t, err)
	assert.Contains(t, sql, "WHERE \"status\" IN (?, ?, ?)")
	assert.Equal(t, []any{"", "", 0, true, "active", "pending", "verified"}, args)
}

func TestBuildSQLUpdateWhere_BetweenCondition(t *testing.T) {
	builder := NewSqlBuilder(DefaultSqlDialect())
	user := WhereTestUser{Name: "Updated Name"}

	whereClause := Between("created_at", "2023-01-01", "2023-12-31")
	sql, args, err := builder.Update("users", user).Where(whereClause).Build()

	require.NoError(t, err)
	assert.Contains(t, sql, "WHERE \"created_at\" BETWEEN ? AND ?")
	assert.Equal(t, []any{"Updated Name", "", 0, false, "2023-01-01", "2023-12-31"}, args)
}

func TestBuildSQLUpdateWhere_NullConditions(t *testing.T) {
	builder := NewSqlBuilder(DefaultSqlDialect())
	user := WhereTestUser{Email: "new@example.com"}

	// Test IS NULL
	whereClause := IsNull("deleted_at")
	sql, args, err := builder.Update("users", user).Where(whereClause).Build()

	require.NoError(t, err)
	assert.Contains(t, sql, "WHERE \"deleted_at\" IS NULL")
	assert.Equal(t, []any{"", "new@example.com", 0, false}, args)

	// Test IS NOT NULL
	whereClause = IsNotNull("verified_at")
	sql, args, err = builder.Update("users", user).Where(whereClause).Build()

	require.NoError(t, err)
	assert.Contains(t, sql, "WHERE \"verified_at\" IS NOT NULL")
	assert.Equal(t, []any{"", "new@example.com", 0, false}, args)
}

func TestBuildSQLUpdateWhere_LiteralCondition(t *testing.T) {
	builder := NewSqlBuilder(DefaultSqlDialect())
	user := WhereTestUser{Name: "Updated"}

	whereClause := Literal("age > ? AND status = ?", 18, "active")
	sql, args, err := builder.Update("users", user).Where(whereClause).Build()

	require.NoError(t, err)
	assert.Contains(t, sql, "WHERE age > ? AND status = ?")
	assert.Equal(t, []any{"Updated", "", 0, false, 18, "active"}, args)
}

func TestBuildSQLUpdateWhere_RawCondition(t *testing.T) {
	builder := NewSqlBuilder(DefaultSqlDialect())
	user := WhereTestUser{Active: false}

	whereClause := Raw("ST_DWithin(location, ST_GeomFromText('POINT(-122.4194 37.7749)'), 1000)")
	sql, args, err := builder.Update("users", user).Where(whereClause).Build()

	require.NoError(t, err)
	assert.Contains(t, sql, "WHERE ST_DWithin(location, ST_GeomFromText('POINT(-122.4194 37.7749)'), 1000)")
	assert.Equal(t, []any{"", "", 0, false}, args)
}

func TestBuildSQLUpdateWhere_ComparisonCondition(t *testing.T) {
	builder := NewSqlBuilder(DefaultSqlDialect())
	user := WhereTestUser{Name: "Updated"}

	whereClause := Compare("created_at", "<", "updated_at")
	sql, args, err := builder.Update("users", user).Where(whereClause).Build()

	require.NoError(t, err)
	assert.Contains(t, sql, "WHERE \"created_at\" < \"updated_at\"")
	assert.Equal(t, []any{"Updated", "", 0, false}, args)
}

func TestUpdateBuilder_FluentInterface(t *testing.T) {
	builder := NewSqlBuilder(DefaultSqlDialect())
	user := WhereTestUser{
		Name:   "John Updated",
		Email:  "john.updated@example.com",
		Active: true,
	}

	sql, args, err := builder.Update("users", user).
		WhereAnd(
			Eq("id", 1),
			Gt("age", 18),
		).
		ExcludeFields("created_at", "updated_at").
		Build()

	require.NoError(t, err)
	assert.Contains(t, sql, "UPDATE \"users\"")
	assert.Contains(t, sql, "WHERE (\"id\" = ? AND \"age\" > ?)")
	assert.Equal(t, []any{"John Updated", "john.updated@example.com", 0, true, 1, 18}, args)
}

func TestUpdateBuilder_ChainedMethods(t *testing.T) {
	builder := NewSqlBuilder(DefaultSqlDialect())
	user := WhereTestUser{Name: "Test User"}

	sql, args, err := builder.Update("users", user).
		WhereOr(
			Eq("email", "test1@example.com"),
			Eq("email", "test2@example.com"),
		).
		IncludeZeroValues(false).
		UpdateAutoFields(true).
		Build()

	require.NoError(t, err)
	assert.Contains(t, sql, "WHERE (\"email\" = ? OR \"email\" = ?)")
	assert.Equal(t, []any{"Test User", "test1@example.com", "test2@example.com"}, args)
}

func TestUpdateBuilder_ConvenienceMethods(t *testing.T) {
	builder := NewSqlBuilder(DefaultSqlDialect())
	user := WhereTestUser{Active: false}

	// Test WhereEq
	sql, args, err := builder.Update("users", user).
		WhereEq("id", 1).
		Build()

	require.NoError(t, err)
	assert.Contains(t, sql, "WHERE \"id\" = ?")
	assert.Equal(t, []any{"", "", 0, false, 1}, args)

	// Test WhereIn
	sql, args, err = builder.Update("users", user).
		WhereIn("status", "active", "pending").
		Build()

	require.NoError(t, err)
	assert.Contains(t, sql, "WHERE \"status\" IN (?, ?)")
	assert.Equal(t, []any{"", "", 0, false, "active", "pending"}, args)

	// Test WhereBetween
	sql, args, err = builder.Update("users", user).
		WhereBetween("age", 18, 65).
		Build()

	require.NoError(t, err)
	assert.Contains(t, sql, "WHERE \"age\" BETWEEN ? AND ?")
	assert.Equal(t, []any{"", "", 0, false, 18, 65}, args)

	// Test WhereNull
	sql, args, err = builder.Update("users", user).
		WhereNull("deleted_at").
		Build()

	require.NoError(t, err)
	assert.Contains(t, sql, "WHERE \"deleted_at\" IS NULL")
	assert.Equal(t, []any{"", "", 0, false}, args)

	// Test WhereLiteral
	sql, args, err = builder.Update("users", user).
		WhereLiteral("age > ? AND status = ?", 18, "active").
		Build()

	require.NoError(t, err)
	assert.Contains(t, sql, "WHERE age > ? AND status = ?")
	assert.Equal(t, []any{"", "", 0, false, 18, "active"}, args)

	// Test WhereRaw
	sql, args, err = builder.Update("users", user).
		WhereRaw("EXTRACT(YEAR FROM created_at) = 2023").
		Build()

	require.NoError(t, err)
	assert.Contains(t, sql, "WHERE EXTRACT(YEAR FROM created_at) = 2023")
	assert.Equal(t, []any{"", "", 0, false}, args)
}

func TestBuildSQLUpdateByID_NewImplementation(t *testing.T) {
	builder := NewSqlBuilder(DefaultSqlDialect())
	user := WhereTestUser{
		Name:   "Updated User",
		Email:  "updated@example.com",
		Active: true,
	}

	sql, args, err := builder.Update("users", user).WhereEq("id", 42).Build()

	require.NoError(t, err)
	assert.Contains(t, sql, "UPDATE \"users\"")
	assert.Contains(t, sql, "\"name\" = ?")
	assert.Contains(t, sql, "\"email\" = ?")
	assert.Contains(t, sql, "\"active\" = ?")
	assert.Contains(t, sql, "WHERE \"id\" = ?")
	assert.Equal(t, []any{"Updated User", "updated@example.com", 0, true, 42}, args)
}

func TestBuildSQLUpdateWhere_ErrorCases(t *testing.T) {
	builder := NewSqlBuilder(DefaultSqlDialect())

	// Test nil WHERE clause
	user := WhereTestUser{Name: "Test"}
	sql, args, err := builder.Update("users", user).Where(nil).Build()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "WHERE clause is required")
	assert.Empty(t, sql)
	assert.Nil(t, args)

	// Test empty table name
	whereClause := Eq("id", 1)
	sql, args, err = builder.Update("", user).Where(whereClause).Build()
	assert.Error(t, err)
	assert.Empty(t, sql)
	assert.Nil(t, args)

	// Test nil record
	sql, args, err = builder.Update("users", nil).Where(whereClause).Build()
	assert.Error(t, err)
	assert.Empty(t, sql)
	assert.Nil(t, args)
}

func TestBuildSQLUpdateWhere_MySQL(t *testing.T) {
	builder := NewSqlBuilder(DefaultSqlDialect())
	user := WhereTestUser{Name: "MySQL User", Email: "mysql@example.com"}

	whereClause := And(
		Eq("id", 1),
		Like("name", "%John%"),
	)
	sql, args, err := builder.Update("users", user).Where(whereClause).Build()

	require.NoError(t, err)
	assert.Contains(t, sql, "UPDATE \"users\"")
	assert.Contains(t, sql, "\"name\" = ?")
	assert.Contains(t, sql, "\"email\" = ?")
	assert.Contains(t, sql, "WHERE (\"id\" = ? AND \"name\" LIKE ?)")
	assert.Equal(t, []any{"MySQL User", "mysql@example.com", 0, false, 1, "%John%"}, args)
}

func TestUpdateBuilder_NoWhere_Error(t *testing.T) {
	builder := NewSqlBuilder(DefaultSqlDialect())
	user := WhereTestUser{Name: "Test"}

	sql, args, err := builder.Update("users", user).Build()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "WHERE clause is required")
	assert.Empty(t, sql)
	assert.Nil(t, args)
}
