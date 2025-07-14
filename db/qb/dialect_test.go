package qb

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultSqlDialect(t *testing.T) {
	d := DefaultSqlDialect()

	assert.Equal(t, "?", d.PlaceHolderFragment)
	assert.False(t, d.IncludePlaceholderNum)
	assert.Equal(t, `"%s"`, d.QuoteTable)
	assert.Equal(t, `"%s"`, d.QuoteField)
	assert.Equal(t, `"%s"`, d.QuoteSchema)
	assert.Equal(t, `"%s"`, d.QuoteDatabase)
	assert.Equal(t, `.`, d.QuoteSeparator)
}

func TestSqlDialect_Placeholder(t *testing.T) {
	tests := []struct {
		name                string
		dialect             SqlDialect
		count               int
		expectedPlaceholder string
	}{
		{
			name:                "default dialect with count",
			dialect:             DefaultSqlDialect(),
			count:               1,
			expectedPlaceholder: "?",
		},
		{
			name:                "default dialect negative count",
			dialect:             DefaultSqlDialect(),
			count:               -1,
			expectedPlaceholder: "?",
		},
		{
			name: "postgres-style dialect",
			dialect: SqlDialect{
				PlaceHolderFragment:   "$",
				IncludePlaceholderNum: true,
			},
			count:               3,
			expectedPlaceholder: "$3",
		},
		{
			name: "postgres-style dialect with zero",
			dialect: SqlDialect{
				PlaceHolderFragment:   "$",
				IncludePlaceholderNum: true,
			},
			count:               0,
			expectedPlaceholder: "$0",
		},
		{
			name: "postgres-style dialect negative (should not include number)",
			dialect: SqlDialect{
				PlaceHolderFragment:   "$",
				IncludePlaceholderNum: true,
			},
			count:               -1,
			expectedPlaceholder: "$",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.dialect.Placeholder(tt.count)
			assert.Equal(t, tt.expectedPlaceholder, result)
		})
	}
}

func TestSqlDialect_Table(t *testing.T) {
	d := DefaultSqlDialect()

	tests := []struct {
		name          string
		tableName     string
		expectedSQL   string
		expectedError bool
		errorMessage  string
	}{
		{
			name:          "simple table name",
			tableName:     "users",
			expectedSQL:   `"users"`,
			expectedError: false,
		},
		{
			name:          "table with schema",
			tableName:     "public.users",
			expectedSQL:   `"public"."users"`,
			expectedError: false,
		},
		{
			name:          "invalid table name with multiple dots",
			tableName:     "db.schema.table",
			expectedSQL:   "",
			expectedError: true,
			errorMessage:  "invalid table name format",
		},
		{
			name:          "empty table name",
			tableName:     "",
			expectedSQL:   "",
			expectedError: true,
			errorMessage:  "table name cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := d.Table(tt.tableName)

			if tt.expectedError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMessage)
				assert.Equal(t, tt.expectedSQL, result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedSQL, result)
			}
		})
	}
}

func TestSqlDialect_TableSchema(t *testing.T) {
	d := DefaultSqlDialect()

	tests := []struct {
		name        string
		schema      string
		table       string
		expectedSQL string
	}{
		{
			name:        "standard schema and table",
			schema:      "public",
			table:       "users",
			expectedSQL: `"public"."users"`,
		},
		{
			name:        "empty schema",
			schema:      "",
			table:       "users",
			expectedSQL: `""."users"`,
		},
		{
			name:        "empty table",
			schema:      "public",
			table:       "",
			expectedSQL: `"public".""`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := d.TableSchema(tt.schema, tt.table)
			assert.Equal(t, tt.expectedSQL, result)
		})
	}
}

func TestSqlDialect_Field(t *testing.T) {
	d := DefaultSqlDialect()

	tests := []struct {
		name        string
		fieldName   string
		expectedSQL string
	}{
		{
			name:        "simple field",
			fieldName:   "name",
			expectedSQL: `"name"`,
		},
		{
			name:        "field with underscore",
			fieldName:   "user_id",
			expectedSQL: `"user_id"`,
		},
		{
			name:        "empty field",
			fieldName:   "",
			expectedSQL: `""`,
		},
		{
			name:        "field with special chars",
			fieldName:   "field.name",
			expectedSQL: `"field.name"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := d.Field(tt.fieldName)
			assert.Equal(t, tt.expectedSQL, result)
		})
	}
}

func TestSqlDialect_CustomDialects(t *testing.T) {
	// Test MySQL-style dialect
	mysqlDialect := SqlDialect{
		PlaceHolderFragment:   "?",
		IncludePlaceholderNum: false,
		QuoteTable:            "`%s`",
		QuoteField:            "`%s`",
		QuoteSchema:           "`%s`",
		QuoteDatabase:         "`%s`",
		QuoteSeparator:        `.`,
	}

	// Test PostgreSQL-style dialect
	postgresDialect := SqlDialect{
		PlaceHolderFragment:   "$",
		IncludePlaceholderNum: true,
		QuoteTable:            `"%s"`,
		QuoteField:            `"%s"`,
		QuoteSchema:           `"%s"`,
		QuoteDatabase:         `"%s"`,
		QuoteSeparator:        `.`,
	}

	// Test SQL Server-style dialect
	sqlServerDialect := SqlDialect{
		PlaceHolderFragment:   "@p",
		IncludePlaceholderNum: true,
		QuoteTable:            "[%s]",
		QuoteField:            "[%s]",
		QuoteSchema:           "[%s]",
		QuoteDatabase:         "[%s]",
		QuoteSeparator:        "].[",
	}

	t.Run("MySQL dialect", func(t *testing.T) {
		assert.Equal(t, "`users`", mysqlDialect.Field("users"))
		table, _ := mysqlDialect.Table("users")
		assert.Equal(t, "`users`", table)
		assert.Equal(t, "?", mysqlDialect.Placeholder(1))
	})

	t.Run("PostgreSQL dialect", func(t *testing.T) {
		assert.Equal(t, `"users"`, postgresDialect.Field("users"))
		table, _ := postgresDialect.Table("users")
		assert.Equal(t, `"users"`, table)
		assert.Equal(t, "$5", postgresDialect.Placeholder(5))
	})

	t.Run("SQL Server dialect", func(t *testing.T) {
		assert.Equal(t, "[users]", sqlServerDialect.Field("users"))
		table, _ := sqlServerDialect.Table("users")
		assert.Equal(t, "[users]", table)
		assert.Equal(t, "@p10", sqlServerDialect.Placeholder(10))
	})
}
