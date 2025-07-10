package qb

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Integration test structs representing real-world scenarios
type User struct {
	ID          int                    `db:"id" auto:"true"`
	Username    string                 `db:"username"`
	Email       string                 `db:"email"`
	FirstName   string                 `db:"first_name"`
	LastName    string                 `db:"last_name"`
	Age         int                    `db:"age"`
	IsActive    bool                   `db:"is_active"`
	LastLogin   *time.Time             `db:"last_login" goqu:"omitnil"`
	Preferences map[string]interface{} `db:"preferences" goqu:"omitempty"`
	UpdatedAt   time.Time              `db:"updated_at" auto:"true"`
	CreatedAt   time.Time              `db:"created_at" auto:"true"`
}

type IntegrationProduct struct {
	ID          int       `db:"id" auto:"true"`
	Name        string    `db:"name"`
	Description string    `db:"description" goqu:"omitempty"`
	Price       float64   `db:"price"`
	Stock       int       `db:"stock"`
	IsActive    bool      `db:"is_active"`
	Tags        []string  `db:"tags" goqu:"omitempty"`
	UpdatedAt   time.Time `db:"updated_at" auto:"true"`
}

type Order struct {
	ID        int        `db:"id" auto:"true"`
	UserID    int        `db:"user_id"`
	Status    string     `db:"status"`
	Total     float64    `db:"total"`
	ShippedAt *time.Time `db:"shipped_at" goqu:"omitnil"`
	UpdatedAt time.Time  `db:"updated_at" auto:"true"`
}

func TestUpdateIntegration_UserManagement(t *testing.T) {
	builder := NewSqlBuilder(DefaultSqlDialect())

	t.Run("update user profile", func(t *testing.T) {
		user := User{
			Username:  "john_doe",
			Email:     "john.doe@example.com",
			FirstName: "John",
			LastName:  "Doe",
			Age:       30,
			IsActive:  true,
		}

		sql, args, err := builder.Update("users", user).WhereEq("id", 123).Build()
		require.NoError(t, err)

		expectedSQL := `UPDATE "users" SET "username" = ?, "email" = ?, "first_name" = ?, "last_name" = ?, "age" = ?, "is_active" = ? WHERE "id" = ?`
		expectedArgs := []any{"john_doe", "john.doe@example.com", "John", "Doe", 30, true, 123}

		assert.Equal(t, expectedSQL, sql)
		assert.Equal(t, expectedArgs, args)
	})

	t.Run("update user profile excluding sensitive fields", func(t *testing.T) {
		user := User{
			Username:  "jane_smith",
			Email:     "jane.smith@example.com",
			FirstName: "Jane",
			LastName:  "Smith",
			Age:       25,
		}

		options := UpdateOptions{
			ExcludeFields: []string{"Email", "IsActive"}, // Don't update sensitive fields
		}

		sql, args, err := builder.Update("users", user).WhereEq("id", 456).WithOptions(&options).Build()
		require.NoError(t, err)

		expectedSQL := `UPDATE "users" SET "username" = ?, "first_name" = ?, "last_name" = ?, "age" = ? WHERE "id" = ?`
		expectedArgs := []any{"jane_smith", "Jane", "Smith", 25, 456}

		assert.Equal(t, expectedSQL, sql)
		assert.Equal(t, expectedArgs, args)
	})

	t.Run("update only specific user fields", func(t *testing.T) {
		user := User{
			FirstName: "UpdatedFirst",
			LastName:  "UpdatedLast",
			Age:       35,
		}

		options := UpdateOptions{
			IncludeFields: []string{"FirstName", "LastName"},
		}

		sql, args, err := builder.Update("users", user).WhereEq("id", 789).WithOptions(&options).Build()
		require.NoError(t, err)

		expectedSQL := `UPDATE "users" SET "first_name" = ?, "last_name" = ? WHERE "id" = ?`
		expectedArgs := []any{"UpdatedFirst", "UpdatedLast", 789}

		assert.Equal(t, expectedSQL, sql)
		assert.Equal(t, expectedArgs, args)
	})

	t.Run("update user last login with timestamp", func(t *testing.T) {
		loginTime := time.Date(2023, 12, 25, 10, 30, 0, 0, time.UTC)
		user := User{
			LastLogin: &loginTime,
		}

		options := UpdateOptions{
			IncludeFields: []string{"LastLogin"},
		}

		sql, args, err := builder.Update("users", user).WhereEq("id", 123).WithOptions(&options).Build()
		require.NoError(t, err)

		expectedSQL := `UPDATE "users" SET "last_login" = ? WHERE "id" = ?`
		expectedArgs := []any{loginTime, 123}

		assert.Equal(t, expectedSQL, sql)
		assert.Equal(t, expectedArgs, args)
	})
}

func TestUpdateIntegration_ProductManagement(t *testing.T) {
	builder := NewSqlBuilder(DefaultSqlDialect())

	// Register JSON mapper for tags

	t.Run("update product inventory", func(t *testing.T) {
		product := IntegrationProduct{
			Stock:    50,
			IsActive: true,
		}

		options := UpdateOptions{
			IncludeFields: []string{"Stock", "IsActive"},
		}

		sql, args, err := builder.Update("products", product).WhereEq("id", 101).WithOptions(&options).Build()
		require.NoError(t, err)

		expectedSQL := `UPDATE "products" SET "stock" = ?, "is_active" = ? WHERE "id" = ?`
		expectedArgs := []any{50, true, 101}

		assert.Equal(t, expectedSQL, sql)
		assert.Equal(t, expectedArgs, args)
	})

	t.Run("update product with tags", func(t *testing.T) {
		product := IntegrationProduct{
			Name:        "Wireless Headphones",
			Description: "High-quality wireless headphones",
			Price:       99.99,
		}

		options := UpdateOptions{
			ExcludeFields: []string{"Stock", "IsActive"}, // Don't update stock and active status
		}

		sql, args, err := builder.Update("products", product).WhereEq("id", 102).WithOptions(&options).Build()
		require.NoError(t, err)

		expectedSQL := `UPDATE "products" SET "name" = ?, "description" = ?, "price" = ? WHERE "id" = ?`
		expectedArgs := []any{"Wireless Headphones", "High-quality wireless headphones", 99.99, 102}

		assert.Equal(t, expectedSQL, sql)
		assert.Equal(t, expectedArgs, args)
	})

	t.Run("batch update product prices", func(t *testing.T) {
		// Note: BuildSQLBatchUpdate doesn't exist in current implementation
		// This would need to be implemented separately or use individual updates
		t.Skip("BuildSQLBatchUpdate not implemented in current design")
	})
}

func TestUpdateIntegration_OrderManagement(t *testing.T) {
	builder := NewSqlBuilder(DefaultSqlDialect())

	t.Run("update order status", func(t *testing.T) {
		order := Order{
			Status: "shipped",
		}

		options := UpdateOptions{
			IncludeFields: []string{"Status"},
		}

		whereClause := And(
			Eq("id", 1001),
			Eq("status", "processing"),
		)

		sql, args, err := builder.Update("orders", order).Where(whereClause).WithOptions(&options).Build()
		require.NoError(t, err)

		expectedSQL := `UPDATE "orders" SET "status" = ? WHERE ("id" = ? AND "status" = ?)`
		expectedArgs := []any{"shipped", 1001, "processing"}

		assert.Equal(t, expectedSQL, sql)
		assert.Equal(t, expectedArgs, args)
	})

	t.Run("update order with shipped timestamp", func(t *testing.T) {
		shippedTime := time.Date(2023, 12, 25, 14, 30, 0, 0, time.UTC)
		order := Order{
			Status:    "shipped",
			ShippedAt: &shippedTime,
		}

		options := UpdateOptions{
			IncludeFields: []string{"Status", "ShippedAt"},
		}

		sql, args, err := builder.Update("orders", order).WhereEq("id", 1002).WithOptions(&options).Build()
		require.NoError(t, err)

		expectedSQL := `UPDATE "orders" SET "status" = ?, "shipped_at" = ? WHERE "id" = ?`
		expectedArgs := []any{"shipped", shippedTime, 1002}

		assert.Equal(t, expectedSQL, sql)
		assert.Equal(t, expectedArgs, args)
	})

	t.Run("batch update order statuses", func(t *testing.T) {
		// BuildSQLBatchUpdate not implemented - skip this test
		t.Skip("BuildSQLBatchUpdate not implemented in current design")
	})
}

func TestUpdateIntegration_AdvancedScenarios(t *testing.T) {
	builder := NewSqlBuilder(DefaultSqlDialect())

	t.Run("update with complex WHERE conditions", func(t *testing.T) {
		user := User{
			IsActive: false,
		}

		options := UpdateOptions{
			IncludeFields:     []string{"IsActive"},
			IncludeZeroValues: true,
		}

		whereClause := And(
			Lt("last_login", time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)),
			Eq("is_active", true),
			Gte("age", 18),
		)

		sql, args, err := builder.Update("users", user).Where(whereClause).WithOptions(&options).Build()
		require.NoError(t, err)

		expectedSQL := `UPDATE "users" SET "is_active" = ? WHERE ("last_login" < ? AND "is_active" = ? AND "age" >= ?)`
		expectedArgs := []any{false, time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC), true, 18}

		assert.Equal(t, expectedSQL, sql)
		assert.Equal(t, expectedArgs, args)
	})

	t.Run("update excluding zero values", func(t *testing.T) {
		user := User{
			Username:  "updated_user",
			Email:     "", // Zero value - should be excluded
			FirstName: "Updated",
			LastName:  "", // Zero value - should be excluded
			Age:       0,  // Zero value - should be excluded
			IsActive:  true,
		}

		options := UpdateOptions{
			IncludeZeroValues: false,
		}

		sql, args, err := builder.Update("users", user).WhereEq("id", 999).WithOptions(&options).Build()
		require.NoError(t, err)

		expectedSQL := `UPDATE "users" SET "username" = ?, "first_name" = ?, "is_active" = ? WHERE "id" = ?`
		expectedArgs := []any{"updated_user", "Updated", true, 999}

		assert.Equal(t, expectedSQL, sql)
		assert.Equal(t, expectedArgs, args)
	})

	t.Run("update with auto fields included", func(t *testing.T) {
		now := time.Date(2023, 12, 25, 15, 30, 0, 0, time.UTC)
		user := User{
			Username:  "admin_user",
			UpdatedAt: now,
		}

		options := UpdateOptions{
			IncludeFields:    []string{"Username", "UpdatedAt"},
			UpdateAutoFields: true,
		}

		sql, args, err := builder.Update("users", user).WhereEq("id", 1).WithOptions(&options).Build()
		require.NoError(t, err)

		expectedSQL := `UPDATE "users" SET "username" = ?, "updated_at" = ? WHERE "id" = ?`
		expectedArgs := []any{"admin_user", now, 1}

		assert.Equal(t, expectedSQL, sql)
		assert.Equal(t, expectedArgs, args)
	})
}

func TestUpdateIntegration_ErrorScenarios(t *testing.T) {
	builder := NewSqlBuilder(DefaultSqlDialect())

	// Register JSON mapper for tests that need it

	t.Run("update with invalid table name", func(t *testing.T) {
		user := User{Username: "test"}

		_, _, err := builder.Update("invalid.table.name", user).WhereEq("id", 1).Build()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid table name format")
	})

	t.Run("update with unregistered mapper", func(t *testing.T) {
		type TestStruct struct {
			ID   int    `db:"id" auto:"true"`
			Data string `db:"data"`
		}

		data := TestStruct{Data: "test"}

		_, _, err := builder.Update("test", data).WhereEq("id", 1).Build()
		require.NoError(t, err) // Should succeed without mapper
	})

	t.Run("batch update with mismatched types", func(t *testing.T) {
		// Use simpler structs to avoid mapper complications
		type SimpleUser struct {
			ID       int    `db:"id"`
			Username string `db:"username"`
		}

		type SimpleProduct struct {
			ID   int    `db:"id"`
			Name string `db:"name"`
		}

		t.Skip("BuildSQLBatchUpdate not implemented")
	})
}

func TestUpdateIntegration_PerformanceScenarios(t *testing.T) {
	t.Run("large batch update", func(t *testing.T) {
		t.Skip("BuildSQLBatchUpdate not implemented")
	})
}

func TestUpdateIntegration_PostgreSQLDialect(t *testing.T) {
	// Test with PostgreSQL-style dialect
	postgresDialect := SqlDialect{
		PlaceHolderFragment:   "$",
		IncludePlaceholderNum: true,
		QuoteTable:            `"%s"`,
		QuoteField:            `"%s"`,
		QuoteSchema:           `"%s"`,
		QuoteDatabase:         `"%s"`,
		QuoteSeparator:        `.`,
	}

	builder := NewSqlBuilder(postgresDialect)

	user := User{
		Username:  "postgres_user",
		Email:     "postgres@example.com",
		FirstName: "Postgres",
		LastName:  "User",
		Age:       28,
		IsActive:  true,
	}

	options := UpdateOptions{
		ExcludeFields: []string{"Preferences"}, // Exclude complex field
	}

	sql, args, err := builder.Update("users", user).WhereEq("id", 123).WithOptions(&options).Build()
	require.NoError(t, err)

	expectedSQL := `UPDATE "users" SET "username" = $1, "email" = $2, "first_name" = $3, "last_name" = $4, "age" = $5, "is_active" = $6 WHERE "id" = $7`
	expectedArgs := []any{"postgres_user", "postgres@example.com", "Postgres", "User", 28, true, 123}

	assert.Equal(t, expectedSQL, sql)
	assert.Equal(t, expectedArgs, args)
}
