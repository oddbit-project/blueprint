package qb

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test structs
type TestUser struct {
	ID         int       `db:"id" auto:"true" goqu:"skipinsert"`
	Name       string    `db:"name"`
	Email      string    `db:"email"`
	Age        *int      `db:"age" goqu:"omitnil"`
	IsActive   bool      `db:"is_active"`
	EmptyField string    `db:"empty_field" goqu:"omitempty"`
	CreatedAt  time.Time `db:"created_at" auto:"true" goqu:"skipinsert"`
	UpdatedAt  time.Time `db:"updated_at"`
}

type SimpleStruct struct {
	Name  string `db:"name"`
	Value int    `db:"value"`
}

func TestBuildSQLInsert(t *testing.T) {
	builder := NewSqlBuilder(DefaultSqlDialect())

	tests := []struct {
		name          string
		tableName     string
		data          any
		expectedSQL   string
		expectedArgs  []any
		expectedError bool
		errorContains string
	}{
		{
			name:      "simple insert with all fields",
			tableName: "users",
			data: TestUser{
				ID:         1,
				Name:       "John Doe",
				Email:      "john@example.com",
				Age:        intPtr(30),
				IsActive:   true,
				EmptyField: "not empty",
				CreatedAt:  time.Now(),
				UpdatedAt:  time.Now(),
			},
			expectedSQL:   `INSERT INTO "users" ("name", "email", "age", "is_active", "empty_field", "updated_at") VALUES (?, ?, ?, ?, ?, ?)`,
			expectedArgs:  []any{"John Doe", "john@example.com", 30, true, "not empty", nil}, // UpdatedAt will be different
			expectedError: false,
		},
		{
			name:      "insert with nil age (omitnil)",
			tableName: "users",
			data: TestUser{
				Name:       "Jane Doe",
				Email:      "jane@example.com",
				Age:        nil, // Should be omitted
				IsActive:   false,
				EmptyField: "data",
				UpdatedAt:  time.Now(),
			},
			expectedSQL:   `INSERT INTO "users" ("name", "email", "is_active", "empty_field", "updated_at") VALUES (?, ?, ?, ?, ?)`,
			expectedArgs:  []any{"Jane Doe", "jane@example.com", false, "data", nil},
			expectedError: false,
		},
		{
			name:      "insert with empty field (omitempty)",
			tableName: "users",
			data: TestUser{
				Name:       "Bob Smith",
				Email:      "bob@example.com",
				Age:        intPtr(25),
				IsActive:   true,
				EmptyField: "", // Should be omitted
				UpdatedAt:  time.Now(),
			},
			expectedSQL:   `INSERT INTO "users" ("name", "email", "age", "is_active", "updated_at") VALUES (?, ?, ?, ?, ?)`,
			expectedArgs:  []any{"Bob Smith", "bob@example.com", 25, true, nil},
			expectedError: false,
		},
		{
			name:      "insert with schema-qualified table",
			tableName: "public.users",
			data: SimpleStruct{
				Name:  "Test",
				Value: 42,
			},
			expectedSQL:   `INSERT INTO "public"."users" ("name", "value") VALUES (?, ?)`,
			expectedArgs:  []any{"Test", 42},
			expectedError: false,
		},
		{
			name:      "insert with pointer to struct",
			tableName: "simple",
			data: &SimpleStruct{
				Name:  "Pointer Test",
				Value: 100,
			},
			expectedSQL:   `INSERT INTO "simple" ("name", "value") VALUES (?, ?)`,
			expectedArgs:  []any{"Pointer Test", 100},
			expectedError: false,
		},
		{
			name:          "invalid table name",
			tableName:     "db.schema.table",
			data:          SimpleStruct{Name: "Test", Value: 1},
			expectedSQL:   "",
			expectedArgs:  nil,
			expectedError: true,
			errorContains: "invalid table name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql, args, err := builder.BuildSQLInsert(tt.tableName, tt.data)

			if tt.expectedError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
				assert.Empty(t, sql)
				assert.Nil(t, args)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedSQL, sql)
				// For time fields, just check the count
				assert.Len(t, args, len(tt.expectedArgs))
				// Check non-time fields
				for i, expectedArg := range tt.expectedArgs {
					if expectedArg != nil {
						assert.Equal(t, expectedArg, args[i])
					}
				}
			}
		})
	}
}

func TestBuildSQLBatchInsert(t *testing.T) {
	builder := NewSqlBuilder(DefaultSqlDialect())

	tests := []struct {
		name          string
		tableName     string
		data          []any
		expectedSQL   string
		expectedArgs  int // Just count args due to time fields
		expectedError bool
		errorContains string
	}{
		{
			name:      "batch insert with 3 records",
			tableName: "simple",
			data: []any{
				SimpleStruct{Name: "User 1", Value: 10},
				SimpleStruct{Name: "User 2", Value: 20},
				SimpleStruct{Name: "User 3", Value: 30},
			},
			expectedSQL:   `INSERT INTO "simple" ("name", "value") VALUES (?, ?), (?, ?), (?, ?)`,
			expectedArgs:  6,
			expectedError: false,
		},
		{
			name:      "batch insert with consistent non-nil values",
			tableName: "users",
			data: []any{
				TestUser{Name: "User 1", Email: "user1@example.com", Age: intPtr(25), IsActive: true, EmptyField: "data1"},
				TestUser{Name: "User 2", Email: "user2@example.com", Age: intPtr(30), IsActive: false, EmptyField: "data2"},
				TestUser{Name: "User 3", Email: "user3@example.com", Age: intPtr(35), IsActive: true, EmptyField: "data3"},
			},
			expectedSQL:   `INSERT INTO "users" ("name", "email", "age", "is_active", "empty_field", "updated_at") VALUES (?, ?, ?, ?, ?, ?), (?, ?, ?, ?, ?, ?), (?, ?, ?, ?, ?, ?)`,
			expectedArgs:  18,
			expectedError: false,
		},
		{
			name:          "empty batch",
			tableName:     "users",
			data:          []any{},
			expectedSQL:   "",
			expectedArgs:  0,
			expectedError: true,
			errorContains: "data cannot be empty",
		},
		{
			name:      "batch with pointer structs",
			tableName: "simple",
			data: []any{
				&SimpleStruct{Name: "Ptr 1", Value: 1},
				&SimpleStruct{Name: "Ptr 2", Value: 2},
			},
			expectedSQL:   `INSERT INTO "simple" ("name", "value") VALUES (?, ?), (?, ?)`,
			expectedArgs:  4,
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql, args, err := builder.BuildSQLBatchInsert(tt.tableName, tt.data)

			if tt.expectedError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
				assert.Empty(t, sql)
				assert.Nil(t, args)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedSQL, sql)
				assert.Len(t, args, tt.expectedArgs)
			}
		})
	}
}

func TestBuildSQLInsert_DifferentDialects(t *testing.T) {
	// Test with PostgreSQL-style dialect
	postgresDialect := SqlDialect{
		PlaceHolderFragment:   "$",
		IncludePlaceholderNum: true,
		QuoteTable:            `"%s"`,
		QuoteField:            `"%s"`,
		QuoteSchema:           `"%s"`,
		QuoteDatabase:         `"%s"`,
		QuoteSeparator:        `"."`,
	}

	builder := NewSqlBuilder(postgresDialect)

	data := SimpleStruct{
		Name:  "Test",
		Value: 42,
	}

	sql, args, err := builder.BuildSQLInsert("users", data)
	require.NoError(t, err)
	assert.Equal(t, `INSERT INTO "users" ("name", "value") VALUES ($1, $2)`, sql)
	assert.Equal(t, []any{"Test", 42}, args)
}

func TestBuildSQLInsert_StructWithNoInsertableFields(t *testing.T) {
	type AllAutoFields struct {
		ID        int       `db:"id" auto:"true"`
		CreatedAt time.Time `db:"created_at" auto:"true"`
	}

	builder := NewSqlBuilder(DefaultSqlDialect())

	data := AllAutoFields{
		ID:        1,
		CreatedAt: time.Now(),
	}

	_, _, err := builder.BuildSQLInsert("test_table", data)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no insertable fields found")
}

func TestBuildSQLInsert_ComplexFieldTypes(t *testing.T) {
	type ComplexStruct struct {
		ID       int                    `db:"id" auto:"true"`
		Name     string                 `db:"name"`
		Tags     []string               `db:"tags" goqu:"omitempty"`
		Metadata map[string]interface{} `db:"metadata" goqu:"omitnil"`
		Config   *string                `db:"config" goqu:"omitnil"`
	}

	builder := NewSqlBuilder(DefaultSqlDialect())

	tests := []struct {
		name         string
		data         ComplexStruct
		expectedSQL  string
		expectedArgs []any
	}{
		{
			name: "with all fields populated",
			data: ComplexStruct{
				Name:     "Complex",
				Tags:     []string{"tag1", "tag2"},
				Metadata: map[string]interface{}{"key": "value"},
				Config:   stringPtr("config data"),
			},
			expectedSQL:  `INSERT INTO "complex" ("name", "tags", "metadata", "config") VALUES (?, ?, ?, ?)`,
			expectedArgs: []any{"Complex", []string{"tag1", "tag2"}, map[string]interface{}{"key": "value"}, "config data"},
		},
		{
			name: "with empty tags and nil config",
			data: ComplexStruct{
				Name:     "Simple",
				Tags:     []string{}, // Empty slice is NOT zero value - will be included
				Metadata: nil,        // Map is not a pointer, omitnil doesn't apply
				Config:   nil,        // Pointer is nil, omitnil should skip
			},
			expectedSQL:  `INSERT INTO "complex" ("name", "tags", "metadata") VALUES (?, ?, ?)`,
			expectedArgs: []any{"Simple", []string{}, map[string]interface{}(nil)},
		},
		{
			name: "with nil tags (zero value)",
			data: ComplexStruct{
				Name:     "Zero Test",
				Tags:     nil, // nil slice IS zero value - will be omitted
				Metadata: nil,
				Config:   nil,
			},
			expectedSQL:  `INSERT INTO "complex" ("name", "metadata") VALUES (?, ?)`,
			expectedArgs: []any{"Zero Test", map[string]interface{}(nil)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql, args, err := builder.BuildSQLInsert("complex", tt.data)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedSQL, sql)
			assert.Equal(t, tt.expectedArgs, args)
		})
	}
}

// Helper functions
func intPtr(i int) *int {
	return &i
}

func stringPtr(s string) *string {
	return &s
}
