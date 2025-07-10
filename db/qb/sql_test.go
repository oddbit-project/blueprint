package qb

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSqlBuilder(t *testing.T) {
	// Test with default dialect
	builder := NewSqlBuilder(DefaultSqlDialect())
	require.NotNil(t, builder)
	assert.Equal(t, DefaultSqlDialect(), builder.dialect)

	// Test with custom dialect
	customDialect := SqlDialect{
		PlaceHolderFragment:   "$",
		IncludePlaceholderNum: true,
		QuoteTable:            `"%s"`,
		QuoteField:            `"%s"`,
		QuoteSchema:           `"%s"`,
		QuoteDatabase:         `"%s"`,
		QuoteSeparator:        `"."`,
	}

	customBuilder := NewSqlBuilder(customDialect)
	require.NotNil(t, customBuilder)
	assert.Equal(t, customDialect, customBuilder.dialect)
}

func TestSqlBuilder_Integration(t *testing.T) {
	// Test that SqlBuilder properly uses the dialect for SQL generation
	type TestRecord struct {
		ID    int    `db:"id" auto:"true"`
		Name  string `db:"name"`
		Email string `db:"email"`
	}

	tests := []struct {
		name        string
		dialect     SqlDialect
		tableName   string
		expectedSQL string
	}{
		{
			name:        "default dialect",
			dialect:     DefaultSqlDialect(),
			tableName:   "users",
			expectedSQL: `INSERT INTO "users" ("name", "email") VALUES (?, ?)`,
		},
		{
			name: "postgres dialect",
			dialect: SqlDialect{
				PlaceHolderFragment:   "$",
				IncludePlaceholderNum: true,
				QuoteTable:            `"%s"`,
				QuoteField:            `"%s"`,
				QuoteSchema:           `"%s"`,
				QuoteDatabase:         `"%s"`,
				QuoteSeparator:        `"."`,
			},
			tableName:   "users",
			expectedSQL: `INSERT INTO "users" ("name", "email") VALUES ($1, $2)`,
		},
		{
			name: "mysql dialect",
			dialect: SqlDialect{
				PlaceHolderFragment:   "?",
				IncludePlaceholderNum: false,
				QuoteTable:            "`%s`",
				QuoteField:            "`%s`",
				QuoteSchema:           "`%s`",
				QuoteDatabase:         "`%s`",
				QuoteSeparator:        "`.`",
			},
			tableName:   "users",
			expectedSQL: "INSERT INTO `users` (`name`, `email`) VALUES (?, ?)",
		},
		{
			name: "sql server dialect",
			dialect: SqlDialect{
				PlaceHolderFragment:   "@p",
				IncludePlaceholderNum: true,
				QuoteTable:            "[%s]",
				QuoteField:            "[%s]",
				QuoteSchema:           "[%s]",
				QuoteDatabase:         "[%s]",
				QuoteSeparator:        "].[",
			},
			tableName:   "users",
			expectedSQL: "INSERT INTO [users] ([name], [email]) VALUES (@p1, @p2)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := NewSqlBuilder(tt.dialect)

			data := TestRecord{
				ID:    1, // Should be ignored due to auto tag
				Name:  "John Doe",
				Email: "john@example.com",
			}

			sql, args, err := builder.BuildSQLInsert(tt.tableName, data)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedSQL, sql)
			assert.Equal(t, []any{"John Doe", "john@example.com"}, args)
		})
	}
}

func TestSqlBuilder_BatchIntegration(t *testing.T) {
	// Test batch operations with different dialects
	type SimpleRecord struct {
		Name  string `db:"name"`
		Value int    `db:"value"`
	}

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

	data := []any{
		SimpleRecord{Name: "Item 1", Value: 10},
		SimpleRecord{Name: "Item 2", Value: 20},
		SimpleRecord{Name: "Item 3", Value: 30},
	}

	sql, args, err := builder.BuildSQLBatchInsert("items", data)
	require.NoError(t, err)

	expectedSQL := `INSERT INTO "items" ("name", "value") VALUES ($1, $2), ($3, $4), ($5, $6)`
	assert.Equal(t, expectedSQL, sql)
	assert.Equal(t, []any{"Item 1", 10, "Item 2", 20, "Item 3", 30}, args)
}
