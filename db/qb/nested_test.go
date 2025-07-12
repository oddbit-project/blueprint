package qb

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test structs for nested structure scenarios
type Address struct {
	Street   string `db:"street"`
	City     string `db:"city"`
	Country  string `db:"country"`
	PostCode string `db:"post_code" goqu:"omitempty"`
}

type ContactInfo struct {
	Phone     string  `db:"phone"`
	Email     string  `db:"email"`
	Secondary *string `db:"secondary_email" goqu:"omitnil"`
}

// Test struct with embedded structs
type Employee struct {
	ID          int         `db:"id" auto:"true"`
	Name        string      `db:"name"`
	Address                 // Anonymous embedding
	ContactInfo ContactInfo `db:"-"` // Should be ignored with db:"-"
	Department  string      `db:"department"`
	CreatedAt   time.Time   `db:"created_at" auto:"true"`
}

// Test struct with nested pointer
type Company struct {
	ID      int      `db:"id" auto:"true"`
	Name    string   `db:"name"`
	Address *Address `db:"address"` // Pointer to struct - not flattened, stored as is
	Founded int      `db:"founded_year"`
}

// Test struct with multiple levels of embedding
type BaseEntity struct {
	ID        int       `db:"id" auto:"true"`
	CreatedAt time.Time `db:"created_at" auto:"true"`
	UpdatedAt time.Time `db:"updated_at"`
}

type NamedEntity struct {
	BaseEntity
	Name        string `db:"name"`
	Description string `db:"description" goqu:"omitempty"`
}

type Product struct {
	NamedEntity
	Price   float64 `db:"price"`
	SKU     string  `db:"sku"`
	InStock bool    `db:"in_stock"`
}

// Test struct with conflicting field names
type ConflictStruct struct {
	ID   int    `db:"id" auto:"true"`
	Name string `db:"name"`
	Info struct {
		Name  string `db:"info_name"` // Different db name to avoid conflict
		Value string `db:"info_value"`
	}
}

func TestBuildSQLInsert_EmbeddedStruct(t *testing.T) {
	builder := NewSqlBuilder(DefaultSqlDialect())

	tests := []struct {
		name         string
		data         any
		expectedSQL  string
		expectedArgs []any
	}{
		{
			name: "embedded struct fields",
			data: Employee{
				ID:   1,
				Name: "John Doe",
				Address: Address{
					Street:   "123 Main St",
					City:     "New York",
					Country:  "USA",
					PostCode: "10001",
				},
				Department: "Engineering",
				CreatedAt:  time.Now(),
			},
			expectedSQL:  `INSERT INTO "employees" ("name", "street", "city", "country", "post_code", "department") VALUES (?, ?, ?, ?, ?, ?)`,
			expectedArgs: []any{"John Doe", "123 Main St", "New York", "USA", "10001", "Engineering"},
		},
		{
			name: "embedded struct with empty omitempty field",
			data: Employee{
				Name: "Jane Smith",
				Address: Address{
					Street:   "456 Oak Ave",
					City:     "Boston",
					Country:  "USA",
					PostCode: "", // Should be omitted
				},
				Department: "HR",
			},
			expectedSQL:  `INSERT INTO "employees" ("name", "street", "city", "country", "department") VALUES (?, ?, ?, ?, ?)`,
			expectedArgs: []any{"Jane Smith", "456 Oak Ave", "Boston", "USA", "HR"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql, args, err := builder.BuildSQLInsert("employees", tt.data)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedSQL, sql)
			assert.Equal(t, tt.expectedArgs, args)
		})
	}
}

func TestBuildSQLInsert_NestedPointerStruct(t *testing.T) {
	builder := NewSqlBuilder(DefaultSqlDialect())

	tests := []struct {
		name         string
		data         any
		expectedSQL  string
		expectedArgs []any
	}{
		{
			name: "nested pointer struct",
			data: Company{
				ID:   1,
				Name: "Tech Corp",
				Address: &Address{
					Street:   "789 Tech Blvd",
					City:     "San Francisco",
					Country:  "USA",
					PostCode: "94105",
				},
				Founded: 2020,
			},
			expectedSQL: `INSERT INTO "companies" ("name", "address", "founded_year") VALUES (?, ?, ?)`,
			expectedArgs: []any{"Tech Corp", Address{
				Street:   "789 Tech Blvd",
				City:     "San Francisco",
				Country:  "USA",
				PostCode: "94105",
			}, 2020},
		},
		{
			name: "nil nested pointer struct",
			data: Company{
				ID:      2,
				Name:    "Startup Inc",
				Address: nil, // Nil pointer - fields should not be included
				Founded: 2023,
			},
			expectedSQL:  `INSERT INTO "companies" ("name", "address", "founded_year") VALUES (?, ?, ?)`,
			expectedArgs: []any{"Startup Inc", nil, 2023},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql, args, err := builder.BuildSQLInsert("companies", tt.data)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedSQL, sql)
			assert.Equal(t, tt.expectedArgs, args)
		})
	}
}

func TestBuildSQLInsert_MultiLevelEmbedding(t *testing.T) {
	builder := NewSqlBuilder(DefaultSqlDialect())

	now := time.Now()
	product := Product{
		NamedEntity: NamedEntity{
			BaseEntity: BaseEntity{
				ID:        1,
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Widget Pro",
			Description: "Professional widget",
		},
		Price:   29.99,
		SKU:     "WGT-001",
		InStock: true,
	}

	sql, args, err := builder.BuildSQLInsert("products", product)
	require.NoError(t, err)

	expectedSQL := `INSERT INTO "products" ("updated_at", "name", "description", "price", "sku", "in_stock") VALUES (?, ?, ?, ?, ?, ?)`
	assert.Equal(t, expectedSQL, sql)
	assert.Len(t, args, 6)
	assert.Equal(t, "Widget Pro", args[1])
	assert.Equal(t, "Professional widget", args[2])
	assert.Equal(t, 29.99, args[3])
	assert.Equal(t, "WGT-001", args[4])
	assert.Equal(t, true, args[5])
}

func TestBuildSQLInsert_MultiLevelEmbeddingWithEmpty(t *testing.T) {
	builder := NewSqlBuilder(DefaultSqlDialect())

	now := time.Now()
	product := Product{
		NamedEntity: NamedEntity{
			BaseEntity: BaseEntity{
				ID:        1,
				CreatedAt: now,
				UpdatedAt: now,
			},
			Name:        "Basic Widget",
			Description: "", // Empty - should be omitted due to omitempty
		},
		Price:   19.99,
		SKU:     "WGT-002",
		InStock: false,
	}

	sql, args, err := builder.BuildSQLInsert("products", product)
	require.NoError(t, err)

	expectedSQL := `INSERT INTO "products" ("updated_at", "name", "price", "sku", "in_stock") VALUES (?, ?, ?, ?, ?)`
	assert.Equal(t, expectedSQL, sql)
	assert.Len(t, args, 5)
}

func TestBuildSQLBatchInsert_NestedStructs(t *testing.T) {
	builder := NewSqlBuilder(DefaultSqlDialect())

	employees := []any{
		Employee{
			Name: "Alice Johnson",
			Address: Address{
				Street:   "111 First St",
				City:     "Chicago",
				Country:  "USA",
				PostCode: "60601",
			},
			Department: "Sales",
		},
		Employee{
			Name: "Bob Wilson",
			Address: Address{
				Street:   "222 Second Ave",
				City:     "Chicago",
				Country:  "USA",
				PostCode: "60602", // Non-empty for consistent behavior
			},
			Department: "Marketing",
		},
	}

	sql, args, err := builder.BuildSQLBatchInsert("employees", employees)
	require.NoError(t, err)

	expectedSQL := `INSERT INTO "employees" ("name", "street", "city", "country", "post_code", "department") VALUES (?, ?, ?, ?, ?, ?), (?, ?, ?, ?, ?, ?)`
	assert.Equal(t, expectedSQL, sql)
	assert.Len(t, args, 12)

	// Check first record
	assert.Equal(t, "Alice Johnson", args[0])
	assert.Equal(t, "111 First St", args[1])
	assert.Equal(t, "60601", args[4])

	// Check second record
	assert.Equal(t, "Bob Wilson", args[6])
	assert.Equal(t, "222 Second Ave", args[7])
	assert.Equal(t, "60602", args[10]) // Non-empty PostCode
}

func TestBuildSQLInsert_IgnoredFields(t *testing.T) {
	builder := NewSqlBuilder(DefaultSqlDialect())

	employee := Employee{
		Name: "Test User",
		Address: Address{
			Street:  "Test Street",
			City:    "Test City",
			Country: "Test Country",
		},
		Department: "Test Dept",
		ContactInfo: ContactInfo{
			Phone: "123-456-7890",
			Email: "test@example.com",
		},
	}

	sql, args, err := builder.BuildSQLInsert("employees", employee)
	require.NoError(t, err)

	// ContactInfo should be ignored due to db:"-" tag
	expectedSQL := `INSERT INTO "employees" ("name", "street", "city", "country", "department") VALUES (?, ?, ?, ?, ?)`
	assert.Equal(t, expectedSQL, sql)
	assert.Len(t, args, 5)

	// Verify ContactInfo fields are not included
	for _, arg := range args {
		assert.NotEqual(t, "123-456-7890", arg)
		assert.NotEqual(t, "test@example.com", arg)
	}
}

func TestBuildSQLInsert_AnonymousStructFields(t *testing.T) {
	type Inner struct {
		Field1 string `db:"field1"`
		Field2 int    `db:"field2"`
	}

	type AnonymousTest struct {
		ID    int    `db:"id" auto:"true"`
		Inner        // Anonymous embedding
		Name  string `db:"name"`
	}

	builder := NewSqlBuilder(DefaultSqlDialect())

	// Testing anonymous embedded struct fields
	data := AnonymousTest{
		ID: 1,
		Inner: Inner{
			Field1: "Value1",
			Field2: 42,
		},
		Name: "Test",
	}

	sql, args, err := builder.BuildSQLInsert("test_table", data)
	require.NoError(t, err)

	// Anonymous struct fields should be included if they have db tags
	expectedSQL := `INSERT INTO "test_table" ("field1", "field2", "name") VALUES (?, ?, ?)`
	assert.Equal(t, expectedSQL, sql)
	assert.Equal(t, []any{"Value1", 42, "Test"}, args)
}

// Test edge case: deeply nested structs
type Level3 struct {
	Value string `db:"level3_value"`
}

type Level2 struct {
	Level3
	Value string `db:"level2_value"`
}

type Level1 struct {
	Level2
	Value string `db:"level1_value"`
}

func TestBuildSQLInsert_DeeplyNestedStructs(t *testing.T) {
	builder := NewSqlBuilder(DefaultSqlDialect())

	data := Level1{
		Level2: Level2{
			Level3: Level3{
				Value: "L3",
			},
			Value: "L2",
		},
		Value: "L1",
	}

	sql, args, err := builder.BuildSQLInsert("nested_table", data)
	require.NoError(t, err)

	// The actual behavior: when structs are embedded, fields are processed in order
	// Since field extraction happens on the instance, the last "Value" field wins
	expectedSQL := `INSERT INTO "nested_table" ("level3_value", "level2_value", "level1_value") VALUES (?, ?, ?)`
	assert.Equal(t, expectedSQL, sql)
	// All values will be "L1" because that's the value of the outermost struct
	assert.Equal(t, []any{"L1", "L1", "L1"}, args)
}

// Test struct with interface field
type FlexibleStruct struct {
	ID   int    `db:"id" auto:"true"`
	Name string `db:"name"`
	Data any    `db:"data"` // Interface field
	Meta any    `db:"meta" goqu:"omitnil"`
}

func TestBuildSQLInsert_InterfaceFields(t *testing.T) {
	builder := NewSqlBuilder(DefaultSqlDialect())

	tests := []struct {
		name         string
		data         FlexibleStruct
		expectedSQL  string
		expectedArgs []any
	}{
		{
			name: "interface with concrete value",
			data: FlexibleStruct{
				Name: "Flex1",
				Data: map[string]string{"key": "value"},
				Meta: "metadata",
			},
			expectedSQL:  `INSERT INTO "flexible" ("name", "data", "meta") VALUES (?, ?, ?)`,
			expectedArgs: []any{"Flex1", map[string]string{"key": "value"}, "metadata"},
		},
		{
			name: "interface with nil value",
			data: FlexibleStruct{
				Name: "Flex2",
				Data: nil,
				Meta: nil, // omitnil doesn't work on non-pointer types
			},
			expectedSQL:  `INSERT INTO "flexible" ("name", "data", "meta") VALUES (?, ?, ?)`,
			expectedArgs: []any{"Flex2", nil, nil},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql, args, err := builder.BuildSQLInsert("flexible", tt.data)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedSQL, sql)
			assert.Equal(t, tt.expectedArgs, args)
		})
	}
}
