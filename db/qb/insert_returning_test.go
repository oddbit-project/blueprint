package qb

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildSQLInsertReturning(t *testing.T) {
	t.Run("basic insert returning single field", func(t *testing.T) {
		builder := NewSqlBuilder(DefaultSqlDialect())

		type User struct {
			ID    int    `db:"id" auto:"true"`
			Name  string `db:"name"`
			Email string `db:"email"`
		}

		user := User{
			Name:  "John Doe",
			Email: "john@example.com",
		}

		sql, args, err := builder.BuildSQLInsertReturning("users", user, []string{"id"})
		require.NoError(t, err)

		expectedSQL := `INSERT INTO "users" ("name", "email") VALUES (?, ?) RETURNING "id"`
		expectedArgs := []any{"John Doe", "john@example.com"}

		assert.Equal(t, expectedSQL, sql)
		assert.Equal(t, expectedArgs, args)
	})

	t.Run("insert returning multiple fields", func(t *testing.T) {
		builder := NewSqlBuilder(DefaultSqlDialect())

		type User struct {
			ID        int       `db:"id" auto:"true"`
			Name      string    `db:"name"`
			Email     string    `db:"email"`
			CreatedAt time.Time `db:"created_at" auto:"true"`
		}

		user := User{
			Name:  "Jane Smith",
			Email: "jane@example.com",
		}

		sql, args, err := builder.BuildSQLInsertReturning("users", user, []string{"id", "created_at"})
		require.NoError(t, err)

		expectedSQL := `INSERT INTO "users" ("name", "email") VALUES (?, ?) RETURNING "id", "created_at"`
		expectedArgs := []any{"Jane Smith", "jane@example.com"}

		assert.Equal(t, expectedSQL, sql)
		assert.Equal(t, expectedArgs, args)
	})

	t.Run("insert returning all fields", func(t *testing.T) {
		builder := NewSqlBuilder(DefaultSqlDialect())

		type Product struct {
			ID          int     `db:"id" auto:"true"`
			Name        string  `db:"name"`
			Price       float64 `db:"price"`
			Description string  `db:"description"`
		}

		product := Product{
			Name:        "Wireless Headphones",
			Price:       99.99,
			Description: "High-quality wireless headphones",
		}

		sql, args, err := builder.BuildSQLInsertReturning("products", product, []string{"id", "name", "price", "description"})
		require.NoError(t, err)

		expectedSQL := `INSERT INTO "products" ("name", "price", "description") VALUES (?, ?, ?) RETURNING "id", "name", "price", "description"`
		expectedArgs := []any{"Wireless Headphones", 99.99, "High-quality wireless headphones"}

		assert.Equal(t, expectedSQL, sql)
		assert.Equal(t, expectedArgs, args)
	})

	t.Run("insert returning with custom mapper", func(t *testing.T) {
		builder := NewSqlBuilder(DefaultSqlDialect())

		type User struct {
			ID       int                    `db:"id" auto:"true"`
			Name     string                 `db:"name"`
			Settings map[string]interface{} `db:"settings" goqu:"omitempty"`
		}

		user := User{
			Name: "Admin User",
		}

		sql, args, err := builder.BuildSQLInsertReturning("users", user, []string{"id", "name"})
		require.NoError(t, err)

		expectedSQL := `INSERT INTO "users" ("name") VALUES (?) RETURNING "id", "name"`
		expectedArgs := []any{"Admin User"}

		assert.Equal(t, expectedSQL, sql)
		assert.Equal(t, expectedArgs, args)
	})

	t.Run("insert returning with PostgreSQL dialect", func(t *testing.T) {
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

		type User struct {
			ID    int    `db:"id" auto:"true"`
			Name  string `db:"name"`
			Email string `db:"email"`
		}

		user := User{
			Name:  "PostgreSQL User",
			Email: "postgres@example.com",
		}

		sql, args, err := builder.BuildSQLInsertReturning("users", user, []string{"id", "name"})
		require.NoError(t, err)

		expectedSQL := `INSERT INTO "users" ("name", "email") VALUES ($1, $2) RETURNING "id", "name"`
		expectedArgs := []any{"PostgreSQL User", "postgres@example.com"}

		assert.Equal(t, expectedSQL, sql)
		assert.Equal(t, expectedArgs, args)
	})

	t.Run("insert returning with schema-qualified table", func(t *testing.T) {
		builder := NewSqlBuilder(DefaultSqlDialect())

		type User struct {
			ID    int    `db:"id" auto:"true"`
			Name  string `db:"name"`
			Email string `db:"email"`
		}

		user := User{
			Name:  "Schema User",
			Email: "schema@example.com",
		}

		sql, args, err := builder.BuildSQLInsertReturning("public.users", user, []string{"id"})
		require.NoError(t, err)

		expectedSQL := `INSERT INTO "public"."users" ("name", "email") VALUES (?, ?) RETURNING "id"`
		expectedArgs := []any{"Schema User", "schema@example.com"}

		assert.Equal(t, expectedSQL, sql)
		assert.Equal(t, expectedArgs, args)
	})

	t.Run("insert returning with omitempty and omitnil", func(t *testing.T) {
		builder := NewSqlBuilder(DefaultSqlDialect())

		type User struct {
			ID          int    `db:"id" auto:"true"`
			Name        string `db:"name"`
			Email       string `db:"email"`
			Description string `db:"description" goqu:"omitempty"`
			Age         *int   `db:"age" goqu:"omitnil"`
		}

		user := User{
			Name:        "Minimal User",
			Email:       "minimal@example.com",
			Description: "",  // Will be omitted
			Age:         nil, // Will be omitted
		}

		sql, args, err := builder.BuildSQLInsertReturning("users", user, []string{"id", "name", "email"})
		require.NoError(t, err)

		expectedSQL := `INSERT INTO "users" ("name", "email") VALUES (?, ?) RETURNING "id", "name", "email"`
		expectedArgs := []any{"Minimal User", "minimal@example.com"}

		assert.Equal(t, expectedSQL, sql)
		assert.Equal(t, expectedArgs, args)
	})

	t.Run("insert returning with pointer struct", func(t *testing.T) {
		builder := NewSqlBuilder(DefaultSqlDialect())

		type User struct {
			ID    int    `db:"id" auto:"true"`
			Name  string `db:"name"`
			Email string `db:"email"`
		}

		user := &User{
			Name:  "Pointer User",
			Email: "pointer@example.com",
		}

		sql, args, err := builder.BuildSQLInsertReturning("users", user, []string{"id", "name"})
		require.NoError(t, err)

		expectedSQL := `INSERT INTO "users" ("name", "email") VALUES (?, ?) RETURNING "id", "name"`
		expectedArgs := []any{"Pointer User", "pointer@example.com"}

		assert.Equal(t, expectedSQL, sql)
		assert.Equal(t, expectedArgs, args)
	})
}

func TestBuildSQLInsertReturning_ErrorCases(t *testing.T) {
	t.Run("empty returning fields", func(t *testing.T) {
		builder := NewSqlBuilder(DefaultSqlDialect())

		type User struct {
			ID   int    `db:"id" auto:"true"`
			Name string `db:"name"`
		}

		user := User{Name: "Test User"}

		_, _, err := builder.BuildSQLInsertReturning("users", user, []string{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "empty return fields")
	})

	t.Run("nil returning fields", func(t *testing.T) {
		builder := NewSqlBuilder(DefaultSqlDialect())

		type User struct {
			ID   int    `db:"id" auto:"true"`
			Name string `db:"name"`
		}

		user := User{Name: "Test User"}

		_, _, err := builder.BuildSQLInsertReturning("users", user, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "empty return fields")
	})

	t.Run("propagates insert errors", func(t *testing.T) {
		builder := NewSqlBuilder(DefaultSqlDialect())

		// Test with nil data
		_, _, err := builder.BuildSQLInsertReturning("users", nil, []string{"id"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "data cannot be nil")
	})

	t.Run("propagates empty table name error", func(t *testing.T) {
		builder := NewSqlBuilder(DefaultSqlDialect())

		type User struct {
			ID   int    `db:"id" auto:"true"`
			Name string `db:"name"`
		}

		user := User{Name: "Test User"}

		_, _, err := builder.BuildSQLInsertReturning("", user, []string{"id"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "table name cannot be empty")
	})

	t.Run("propagates invalid table name error", func(t *testing.T) {
		builder := NewSqlBuilder(DefaultSqlDialect())

		type User struct {
			ID   int    `db:"id" auto:"true"`
			Name string `db:"name"`
		}

		user := User{Name: "Test User"}

		_, _, err := builder.BuildSQLInsertReturning("invalid.table.name", user, []string{"id"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid table name format")
	})

	t.Run("propagates mapper errors", func(t *testing.T) {
		builder := NewSqlBuilder(DefaultSqlDialect())

		type User struct {
			ID   int    `db:"id" auto:"true"`
			Name string `db:"name"`
			Data string `db:"data"`
		}

		user := User{
			Name: "Test User",
			Data: "some data",
		}

		_, _, err := builder.BuildSQLInsertReturning("users", user, []string{"id"})
		require.NoError(t, err) // Should succeed without mapper
	})

	t.Run("propagates struct validation errors", func(t *testing.T) {
		builder := NewSqlBuilder(DefaultSqlDialect())

		// Test with non-struct data
		_, _, err := builder.BuildSQLInsertReturning("users", "not a struct", []string{"id"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must be a struct")
	})
}

func TestBuildSQLInsertReturning_Integration(t *testing.T) {
	t.Run("real-world user registration scenario", func(t *testing.T) {
		builder := NewSqlBuilder(DefaultSqlDialect())

		type User struct {
			ID          int                    `db:"id" auto:"true"`
			Username    string                 `db:"username"`
			Email       string                 `db:"email"`
			FullName    string                 `db:"full_name"`
			IsActive    bool                   `db:"is_active"`
			Preferences map[string]interface{} `db:"preferences" goqu:"omitempty"`
			CreatedAt   time.Time              `db:"created_at" auto:"true"`
			UpdatedAt   time.Time              `db:"updated_at" auto:"true"`
		}

		user := User{
			Username: "johndoe",
			Email:    "john.doe@example.com",
			FullName: "John Doe",
			IsActive: true,
		}

		sql, args, err := builder.BuildSQLInsertReturning("users", user, []string{"id", "created_at", "updated_at"})
		require.NoError(t, err)

		expectedSQL := `INSERT INTO "users" ("username", "email", "full_name", "is_active") VALUES (?, ?, ?, ?) RETURNING "id", "created_at", "updated_at"`
		expectedArgs := []any{"johndoe", "john.doe@example.com", "John Doe", true}

		assert.Equal(t, expectedSQL, sql)
		assert.Equal(t, expectedArgs, args)
	})

	t.Run("product creation with inventory tracking", func(t *testing.T) {
		builder := NewSqlBuilder(DefaultSqlDialect())

		type Product struct {
			ID          int       `db:"id" auto:"true"`
			SKU         string    `db:"sku"`
			Name        string    `db:"name"`
			Description string    `db:"description"`
			Price       float64   `db:"price"`
			Stock       int       `db:"stock"`
			Tags        []string  `db:"tags" goqu:"omitempty"`
			IsActive    bool      `db:"is_active"`
			CreatedAt   time.Time `db:"created_at" auto:"true"`
		}

		product := Product{
			SKU:         "WHD-001",
			Name:        "Wireless Headphones",
			Description: "Premium wireless headphones with noise cancellation",
			Price:       199.99,
			Stock:       50,
			IsActive:    true,
		}

		sql, args, err := builder.BuildSQLInsertReturning("products", product, []string{"id", "sku", "created_at"})
		require.NoError(t, err)

		expectedSQL := `INSERT INTO "products" ("sku", "name", "description", "price", "stock", "is_active") VALUES (?, ?, ?, ?, ?, ?) RETURNING "id", "sku", "created_at"`
		expectedArgs := []any{"WHD-001", "Wireless Headphones", "Premium wireless headphones with noise cancellation", 199.99, 50, true}

		assert.Equal(t, expectedSQL, sql)
		assert.Equal(t, expectedArgs, args)
	})

	t.Run("audit log entry creation", func(t *testing.T) {
		builder := NewSqlBuilder(DefaultSqlDialect())

		type AuditLog struct {
			ID        int                    `db:"id" auto:"true"`
			UserID    int                    `db:"user_id"`
			Action    string                 `db:"action"`
			Resource  string                 `db:"resource"`
			Details   map[string]interface{} `db:"details" goqu:"omitempty"`
			IPAddress string                 `db:"ip_address"`
			UserAgent string                 `db:"user_agent"`
			CreatedAt time.Time              `db:"created_at" auto:"true"`
		}

		logEntry := AuditLog{
			UserID:    123,
			Action:    "CREATE",
			Resource:  "users",
			IPAddress: "192.168.1.100",
			UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
		}

		sql, args, err := builder.BuildSQLInsertReturning("audit_logs", logEntry, []string{"id", "created_at"})
		require.NoError(t, err)

		expectedSQL := `INSERT INTO "audit_logs" ("user_id", "action", "resource", "ip_address", "user_agent") VALUES (?, ?, ?, ?, ?) RETURNING "id", "created_at"`
		expectedArgs := []any{123, "CREATE", "users", "192.168.1.100", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"}

		assert.Equal(t, expectedSQL, sql)
		assert.Equal(t, expectedArgs, args)
	})
}

func TestBuildSQLInsertReturning_FieldQuoting(t *testing.T) {
	t.Run("properly quotes returning field names", func(t *testing.T) {
		builder := NewSqlBuilder(DefaultSqlDialect())

		type User struct {
			ID        int    `db:"id" auto:"true"`
			Name      string `db:"name"`
			UserOrder int    `db:"order"` // 'order' is a reserved keyword
		}

		user := User{
			Name:      "Test User",
			UserOrder: 1,
		}

		sql, args, err := builder.BuildSQLInsertReturning("users", user, []string{"id", "order"})
		require.NoError(t, err)

		expectedSQL := `INSERT INTO "users" ("name", "order") VALUES (?, ?) RETURNING "id", "order"`
		expectedArgs := []any{"Test User", 1}

		assert.Equal(t, expectedSQL, sql)
		assert.Equal(t, expectedArgs, args)
	})

	t.Run("handles field names with special characters", func(t *testing.T) {
		builder := NewSqlBuilder(DefaultSqlDialect())

		type User struct {
			ID           int    `db:"id" auto:"true"`
			Name         string `db:"name"`
			SpecialField string `db:"special-field"`
		}

		user := User{
			Name:         "Test User",
			SpecialField: "special value",
		}

		sql, args, err := builder.BuildSQLInsertReturning("users", user, []string{"id", "special-field"})
		require.NoError(t, err)

		expectedSQL := `INSERT INTO "users" ("name", "special-field") VALUES (?, ?) RETURNING "id", "special-field"`
		expectedArgs := []any{"Test User", "special value"}

		assert.Equal(t, expectedSQL, sql)
		assert.Equal(t, expectedArgs, args)
	})
}

func TestBuildSQLInsertReturning_Performance(t *testing.T) {
	t.Run("performance with many returning fields", func(t *testing.T) {
		builder := NewSqlBuilder(DefaultSqlDialect())

		type LargeStruct struct {
			ID      int    `db:"id" auto:"true"`
			Field1  string `db:"field1"`
			Field2  string `db:"field2"`
			Field3  string `db:"field3"`
			Field4  string `db:"field4"`
			Field5  string `db:"field5"`
			Field6  string `db:"field6"`
			Field7  string `db:"field7"`
			Field8  string `db:"field8"`
			Field9  string `db:"field9"`
			Field10 string `db:"field10"`
		}

		data := LargeStruct{
			Field1: "value1", Field2: "value2", Field3: "value3",
			Field4: "value4", Field5: "value5", Field6: "value6",
			Field7: "value7", Field8: "value8", Field9: "value9",
			Field10: "value10",
		}

		returningFields := []string{"id", "field1", "field2", "field3", "field4", "field5", "field6", "field7", "field8", "field9", "field10"}

		sql, args, err := builder.BuildSQLInsertReturning("large_table", data, returningFields)
		require.NoError(t, err)

		// Verify the structure is correct
		assert.Contains(t, sql, "INSERT INTO \"large_table\"")
		assert.Contains(t, sql, "RETURNING")
		assert.Len(t, args, 10) // All non-auto fields

		// Count the number of returning fields
		returningPart := sql[strings.Index(sql, "RETURNING")+len("RETURNING "):]
		fieldCount := len(returningFields)
		expectedCommas := fieldCount - 1
		actualCommas := 0
		for _, char := range returningPart {
			if char == ',' {
				actualCommas++
			}
		}
		assert.Equal(t, expectedCommas, actualCommas)
	})
}
