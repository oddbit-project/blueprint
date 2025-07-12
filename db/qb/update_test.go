package qb

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test structs for UPDATE operations
type UpdateTestStruct struct {
	ID        int       `db:"id" auto:"true"`
	Name      string    `db:"name"`
	Email     string    `db:"email"`
	Age       int       `db:"age"`
	IsActive  bool      `db:"is_active"`
	Score     *float64  `db:"score" goqu:"omitnil"`
	UpdatedAt time.Time `db:"updated_at" auto:"true"`
}

func TestBuildSQLUpdate_Basic(t *testing.T) {
	builder := NewSqlBuilder(DefaultSqlDialect())

	tests := []struct {
		name          string
		data          any
		whereClause   WhereClause
		options       *UpdateOptions
		expectedSQL   string
		expectedArgs  []any
		expectedError bool
		errorContains string
	}{
		{
			name: "simple update with single WHERE condition",
			data: UpdateTestStruct{
				Name:     "John Doe",
				Email:    "john@example.com",
				Age:      30,
				IsActive: true,
			},
			whereClause:  Eq("id", 1),
			options:      DefaultUpdateOptions(),
			expectedSQL:  `UPDATE "update_test" SET "name" = ?, "email" = ?, "age" = ?, "is_active" = ? WHERE "id" = ?`,
			expectedArgs: []any{"John Doe", "john@example.com", 30, true, 1},
		},
		{
			name: "update with multiple WHERE conditions",
			data: UpdateTestStruct{
				Name:  "Jane Smith",
				Email: "jane@example.com",
			},
			whereClause: And(
				Eq("id", 2),
				Eq("status", "active"),
			),
			options:      DefaultUpdateOptions(),
			expectedSQL:  `UPDATE "update_test" SET "name" = ?, "email" = ?, "age" = ?, "is_active" = ? WHERE ("id" = ? AND "status" = ?)`,
			expectedArgs: []any{"Jane Smith", "jane@example.com", 0, false, 2, "active"},
		},
		{
			name: "update excluding zero values",
			data: UpdateTestStruct{
				Name:     "Bob Johnson",
				Email:    "", // Zero value
				Age:      0,  // Zero value
				IsActive: true,
			},
			whereClause: Eq("id", 4),
			options: &UpdateOptions{
				IncludeZeroValues: false,
			},
			expectedSQL:  `UPDATE "update_test" SET "name" = ?, "is_active" = ? WHERE "id" = ?`,
			expectedArgs: []any{"Bob Johnson", true, 4},
		},
		{
			name: "update with field exclusion",
			data: UpdateTestStruct{
				Name:     "Alice Brown",
				Email:    "alice@example.com",
				Age:      25,
				IsActive: false,
			},
			whereClause: Eq("id", 5),
			options: &UpdateOptions{
				ExcludeFields: []string{"Email", "Age", "IsActive"},
			},
			expectedSQL:  `UPDATE "update_test" SET "name" = ? WHERE "id" = ?`,
			expectedArgs: []any{"Alice Brown", 5},
		},
		{
			name: "update with field inclusion",
			data: UpdateTestStruct{
				Name:     "Charlie Davis",
				Email:    "charlie@example.com",
				Age:      35,
				IsActive: true,
			},
			whereClause: Eq("id", 6),
			options: &UpdateOptions{
				IncludeFields: []string{"Name", "Email"},
			},
			expectedSQL:  `UPDATE "update_test" SET "name" = ?, "email" = ? WHERE "id" = ?`,
			expectedArgs: []any{"Charlie Davis", "charlie@example.com", 6},
		},
		{
			name: "update with auto fields included",
			data: UpdateTestStruct{
				Name:      "Diana Wilson",
				Email:     "diana@example.com",
				UpdatedAt: time.Date(2023, 12, 25, 10, 0, 0, 0, time.UTC),
			},
			whereClause: Eq("id", 7),
			options: &UpdateOptions{
				UpdateAutoFields: true,
				IncludeFields:    []string{"Name", "Email", "UpdatedAt"},
			},
			expectedSQL:  `UPDATE "update_test" SET "name" = ?, "email" = ?, "updated_at" = ? WHERE "id" = ?`,
			expectedArgs: []any{"Diana Wilson", "diana@example.com", time.Date(2023, 12, 25, 10, 0, 0, 0, time.UTC), 7},
		},
		{
			name: "update with different operators",
			data: UpdateTestStruct{
				Name:  "Eve Taylor",
				Email: "eve@example.com",
			},
			whereClause: And(
				Gt("age", 18),
				Cond("status", "IN", "('active', 'pending')"),
			),
			options:      DefaultUpdateOptions(),
			expectedSQL:  `UPDATE "update_test" SET "name" = ?, "email" = ?, "age" = ?, "is_active" = ? WHERE ("age" > ? AND "status" IN ?)`,
			expectedArgs: []any{"Eve Taylor", "eve@example.com", 0, false, 18, "('active', 'pending')"},
		},
		{
			name: "update with pointer struct",
			data: &UpdateTestStruct{
				Name:     "Frank Miller",
				Email:    "frank@example.com",
				Age:      40,
				IsActive: true,
			},
			whereClause:  Eq("id", 8),
			options:      DefaultUpdateOptions(),
			expectedSQL:  `UPDATE "update_test" SET "name" = ?, "email" = ?, "age" = ?, "is_active" = ? WHERE "id" = ?`,
			expectedArgs: []any{"Frank Miller", "frank@example.com", 40, true, 8},
		},
		{
			name: "update with omitnil field",
			data: UpdateTestStruct{
				Name:  "Grace Lee",
				Email: "grace@example.com",
				Score: nil, // Should be omitted
			},
			whereClause:  Eq("id", 9),
			options:      DefaultUpdateOptions(),
			expectedSQL:  `UPDATE "update_test" SET "name" = ?, "email" = ?, "age" = ?, "is_active" = ? WHERE "id" = ?`,
			expectedArgs: []any{"Grace Lee", "grace@example.com", 0, false, 9},
		},
		{
			name: "error: empty WHERE conditions",
			data: UpdateTestStruct{
				Name: "Test User",
			},
			whereClause:   nil,
			options:       DefaultUpdateOptions(),
			expectedError: true,
			errorContains: "WHERE clause is required",
		},
		{
			name:          "error: nil data",
			data:          nil,
			whereClause:   Eq("id", 1),
			options:       DefaultUpdateOptions(),
			expectedError: true,
			errorContains: "record cannot be nil",
		},
		{
			name:          "error: empty table name",
			data:          UpdateTestStruct{Name: "Test"},
			whereClause:   Eq("id", 1),
			options:       DefaultUpdateOptions(),
			expectedError: true,
			errorContains: "table name cannot be empty",
		},
		{
			name: "error: no updatable fields",
			data: UpdateTestStruct{
				ID: 1, // Only auto field
			},
			whereClause: Eq("id", 1),
			options: &UpdateOptions{
				IncludeFields: []string{"NonExistentField"},
			},
			expectedError: true,
			errorContains: "no updatable fields found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tableName := "update_test"
			if tt.name == "error: empty table name" {
				tableName = ""
			}

			sql, args, err := builder.Update(tableName, tt.data).Where(tt.whereClause).WithOptions(tt.options).Build()

			if tt.expectedError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expectedSQL, sql)

			assert.Equal(t, tt.expectedArgs, args)
		})
	}
}

func TestBuildSQLUpdateByID(t *testing.T) {
	builder := NewSqlBuilder(DefaultSqlDialect())

	tests := []struct {
		name         string
		data         any
		id           any
		options      *UpdateOptions
		expectedSQL  string
		expectedArgs []any
	}{
		{
			name: "update by integer ID",
			data: UpdateTestStruct{
				Name:     "John Doe",
				Email:    "john@example.com",
				Age:      30,
				IsActive: true,
			},
			id:           1,
			options:      DefaultUpdateOptions(),
			expectedSQL:  `UPDATE "users" SET "name" = ?, "email" = ?, "age" = ?, "is_active" = ? WHERE "id" = ?`,
			expectedArgs: []any{"John Doe", "john@example.com", 30, true, 1},
		},
		{
			name: "update by string ID",
			data: UpdateTestStruct{
				Name:  "Jane Smith",
				Email: "jane@example.com",
			},
			id:           "user-123",
			options:      DefaultUpdateOptions(),
			expectedSQL:  `UPDATE "users" SET "name" = ?, "email" = ?, "age" = ?, "is_active" = ? WHERE "id" = ?`,
			expectedArgs: []any{"Jane Smith", "jane@example.com", 0, false, "user-123"},
		},
		{
			name: "update by ID with options",
			data: UpdateTestStruct{
				Name:     "Bob Johnson",
				Email:    "bob@example.com",
				Age:      25,
				IsActive: true,
			},
			id: 2,
			options: &UpdateOptions{
				IncludeFields: []string{"Name", "Age"},
			},
			expectedSQL:  `UPDATE "users" SET "name" = ?, "age" = ? WHERE "id" = ?`,
			expectedArgs: []any{"Bob Johnson", 25, 2},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql, args, err := builder.Update("users", tt.data).WhereEq("id", tt.id).WithOptions(tt.options).Build()
			require.NoError(t, err)
			assert.Equal(t, tt.expectedSQL, sql)
			assert.Equal(t, tt.expectedArgs, args)
		})
	}
}

func TestUpdateOptions_Default(t *testing.T) {
	opts := DefaultUpdateOptions()

	assert.False(t, opts.OnlyChanged)
	assert.True(t, opts.IncludeZeroValues)
	assert.Nil(t, opts.ExcludeFields)
	assert.Nil(t, opts.IncludeFields)
	assert.False(t, opts.UpdateAutoFields)
}

func TestShouldSkipField(t *testing.T) {
	tests := []struct {
		name      string
		fieldName string
		options   *UpdateOptions
		expected  bool
	}{
		{
			name:      "no restrictions",
			fieldName: "Name",
			options:   DefaultUpdateOptions(),
			expected:  false,
		},
		{
			name:      "field in exclude list",
			fieldName: "Email",
			options: &UpdateOptions{
				ExcludeFields: []string{"Email", "Age"},
			},
			expected: true,
		},
		{
			name:      "field not in exclude list",
			fieldName: "Name",
			options: &UpdateOptions{
				ExcludeFields: []string{"Email", "Age"},
			},
			expected: false,
		},
		{
			name:      "field in include list",
			fieldName: "Name",
			options: &UpdateOptions{
				IncludeFields: []string{"Name", "Email"},
			},
			expected: false,
		},
		{
			name:      "field not in include list",
			fieldName: "Age",
			options: &UpdateOptions{
				IncludeFields: []string{"Name", "Email"},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.options.ShouldSkipField(tt.fieldName, tt.fieldName)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBuildSQLUpdate_WithDifferentDialects(t *testing.T) {
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

	data := UpdateTestStruct{
		Name:     "Test User",
		Email:    "test@example.com",
		Age:      25,
		IsActive: true,
	}

	whereClause := Eq("id", 1)

	sql, args, err := builder.Update("users", data).Where(whereClause).Build()
	require.NoError(t, err)

	expectedSQL := `UPDATE "users" SET "name" = $1, "email" = $2, "age" = $3, "is_active" = $4 WHERE "id" = $5`
	expectedArgs := []any{"Test User", "test@example.com", 25, true, 1}

	assert.Equal(t, expectedSQL, sql)
	assert.Equal(t, expectedArgs, args)
}

// Benchmark tests
func BenchmarkBuildSQLUpdate(b *testing.B) {
	builder := NewSqlBuilder(DefaultSqlDialect())

	data := UpdateTestStruct{
		Name:     "Benchmark User",
		Email:    "benchmark@example.com",
		Age:      30,
		IsActive: true,
	}

	whereClause := Eq("id", 1)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, err := builder.Update("users", data).Where(whereClause).Build()
		if err != nil {
			b.Fatal(err)
		}
	}
}
