package db

import (
	"github.com/doug-martin/goqu/v9"
	"github.com/stretchr/testify/assert"
	"testing"
)

// TestGridRecord is a struct for testing grid functionality
type TestGridRecord struct {
	ID          int    `db:"id" json:"id" grid:"sort,filter"`
	Name        string `db:"name" json:"name" grid:"sort,search,filter"`
	Email       string `db:"email" json:"email" grid:"search,filter"`
	Description string `db:"description" json:"description" grid:"search"`
	Status      bool   `db:"status" json:"status" grid:"filter"`
	CreatedAt   string `db:"created_at" json:"createdAt" grid:"sort"`
}

func TestGridError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      GridError
		expected string
	}{
		{
			name: "with field",
			err: GridError{
				Scope:   "test",
				Field:   "field1",
				Message: "error message",
			},
			expected: "error on test with field field1: error message",
		},
		{
			name: "without field",
			err: GridError{
				Scope:   "test",
				Field:   "",
				Message: "error message",
			},
			expected: "error on test: error message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.err.Error()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNewGridQuery(t *testing.T) {
	tests := []struct {
		name       string
		searchType uint
		limit      uint
		offset     uint
		expectErr  bool
	}{
		{
			name:       "valid search type - none",
			searchType: SearchNone,
			limit:      10,
			offset:     0,
			expectErr:  false,
		},
		{
			name:       "valid search type - start",
			searchType: SearchStart,
			limit:      10,
			offset:     0,
			expectErr:  false,
		},
		{
			name:       "valid search type - end",
			searchType: SearchEnd,
			limit:      10,
			offset:     0,
			expectErr:  false,
		},
		{
			name:       "valid search type - any",
			searchType: SearchAny,
			limit:      10,
			offset:     0,
			expectErr:  false,
		},
		{
			name:       "invalid search type",
			searchType: 99,
			limit:      10,
			offset:     0,
			expectErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query, err := NewGridQuery(tt.searchType, tt.limit, tt.offset)
			if tt.expectErr {
				assert.Error(t, err)
				assert.Nil(t, query)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.searchType, query.SearchType)
				assert.Equal(t, tt.limit, query.Limit)
				assert.Equal(t, tt.offset, query.Offset)
				assert.Empty(t, query.SearchText)
				assert.Nil(t, query.FilterFields)
				assert.Nil(t, query.SortFields)
			}
		})
	}
}

func TestNewGrid(t *testing.T) {
	// Valid record
	grid, err := NewGrid("test_table", &TestGridRecord{})
	assert.NoError(t, err)
	assert.NotNil(t, grid)
	assert.Equal(t, "test_table", grid.tableName)
	assert.NotNil(t, grid.spec)
	assert.NotNil(t, grid.filterFunc)

	// Invalid record (should fail)
	grid, err = NewGrid("test_table", nil)
	assert.Error(t, err)
	assert.Nil(t, grid)
}

func TestNewGridWithSpec(t *testing.T) {
	spec, _ := NewFieldSpec(&TestGridRecord{})
	grid := NewGridWithSpec("test_table", spec)
	
	assert.NotNil(t, grid)
	assert.Equal(t, "test_table", grid.tableName)
	assert.Equal(t, spec, grid.spec)
	assert.NotNil(t, grid.filterFunc)
}

func TestGrid_AddFilterFunc(t *testing.T) {
	grid, _ := NewGrid("test_table", &TestGridRecord{})
	
	// Test filter function for boolean conversion
	filterFunc := func(value any) (any, error) {
		if value == "yes" {
			return true, nil
		}
		return false, nil
	}
	
	result := grid.AddFilterFunc("status", filterFunc)
	
	// Should return the grid for chaining
	assert.Equal(t, grid, result)
	
	// Verify filter function was added
	assert.Contains(t, grid.filterFunc, "status")
}

func TestGrid_ValidQuery(t *testing.T) {
	grid, _ := NewGrid("test_table", &TestGridRecord{})
	
	// Add a filter function for testing
	grid.AddFilterFunc("status", func(value any) (any, error) {
		if value == "invalid" {
			return nil, GridError{
				Scope:   "filter",
				Field:   "status",
				Message: "invalid value",
			}
		}
		return true, nil
	})
	
	tests := []struct {
		name      string
		query     GridQuery
		expectErr bool
	}{
		{
			name: "valid empty query",
			query: GridQuery{
				SearchType: SearchNone,
				Limit:      10,
				Offset:     0,
			},
			expectErr: false,
		},
		{
			name: "valid filter fields",
			query: GridQuery{
				SearchType:   SearchNone,
				FilterFields: map[string]any{"id": 1, "name": "test"},
				Limit:        10,
				Offset:       0,
			},
			expectErr: false,
		},
		{
			name: "invalid filter field name",
			query: GridQuery{
				SearchType:   SearchNone,
				FilterFields: map[string]any{"nonexistent": "value"},
				Limit:        10,
				Offset:       0,
			},
			expectErr: true,
		},
		{
			name: "non-filterable field",
			query: GridQuery{
				SearchType:   SearchNone,
				FilterFields: map[string]any{"createdAt": "2023-01-01"},
				Limit:        10,
				Offset:       0,
			},
			expectErr: true,
		},
		{
			name: "invalid filter value",
			query: GridQuery{
				SearchType:   SearchNone,
				FilterFields: map[string]any{"status": "invalid"},
				Limit:        10,
				Offset:       0,
			},
			expectErr: true,
		},
		{
			name: "valid sort fields",
			query: GridQuery{
				SearchType: SearchNone,
				SortFields: map[string]string{"id": SortAscending, "name": SortDescending},
				Limit:      10,
				Offset:     0,
			},
			expectErr: false,
		},
		{
			name: "invalid sort field name",
			query: GridQuery{
				SearchType: SearchNone,
				SortFields: map[string]string{"nonexistent": SortAscending},
				Limit:      10,
				Offset:     0,
			},
			expectErr: true,
		},
		{
			name: "non-sortable field",
			query: GridQuery{
				SearchType: SearchNone,
				SortFields: map[string]string{"email": SortAscending},
				Limit:      10,
				Offset:     0,
			},
			expectErr: true,
		},
		{
			name: "invalid sort order",
			query: GridQuery{
				SearchType: SearchNone,
				SortFields: map[string]string{"id": "invalid"},
				Limit:      10,
				Offset:     0,
			},
			expectErr: true,
		},
		{
			name: "search text with SearchNone",
			query: GridQuery{
				SearchType: SearchNone,
				SearchText: "test",
				Limit:      10,
				Offset:     0,
			},
			expectErr: true,
		},
		{
			name: "valid search",
			query: GridQuery{
				SearchType: SearchAny,
				SearchText: "test",
				Limit:      10,
				Offset:     0,
			},
			expectErr: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := grid.ValidQuery(&tt.query)
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGrid_Build(t *testing.T) {
	grid, _ := NewGrid("test_table", &TestGridRecord{})
	
	// Add a filter function for testing
	grid.AddFilterFunc("status", func(value any) (any, error) {
		if value == "yes" {
			return true, nil
		}
		if value == "invalid" {
			return nil, GridError{
				Scope:   "filter",
				Field:   "status",
				Message: "invalid value",
			}
		}
		return false, nil
	})
	
	tests := []struct {
		name      string
		qry       *goqu.SelectDataset
		args      GridQuery
		expectErr bool
	}{
		{
			name: "nil query",
			qry:  nil,
			args: GridQuery{
				SearchType: SearchNone,
				Limit:      10,
				Offset:     0,
			},
			expectErr: false,
		},
		{
			name: "filter fields",
			qry:  goqu.Select().From("test_table"),
			args: GridQuery{
				SearchType:   SearchNone,
				FilterFields: map[string]any{"id": 1, "name": "test"},
				Limit:        10,
				Offset:       0,
			},
			expectErr: false,
		},
		{
			name: "invalid filter field",
			qry:  goqu.Select().From("test_table"),
			args: GridQuery{
				SearchType:   SearchNone,
				FilterFields: map[string]any{"nonexistent": "value"},
				Limit:        10,
				Offset:       0,
			},
			expectErr: true,
		},
		{
			name: "invalid filter value",
			qry:  goqu.Select().From("test_table"),
			args: GridQuery{
				SearchType:   SearchNone,
				FilterFields: map[string]any{"status": "invalid"},
				Limit:        10,
				Offset:       0,
			},
			expectErr: true,
		},
		{
			name: "search - none",
			qry:  goqu.Select().From("test_table"),
			args: GridQuery{
				SearchType: SearchNone,
				SearchText: "test",
				Limit:      10,
				Offset:     0,
			},
			expectErr: true,
		},
		{
			name: "search - start",
			qry:  goqu.Select().From("test_table"),
			args: GridQuery{
				SearchType: SearchStart,
				SearchText: "test",
				Limit:      10,
				Offset:     0,
			},
			expectErr: false,
		},
		{
			name: "search - end",
			qry:  goqu.Select().From("test_table"),
			args: GridQuery{
				SearchType: SearchEnd,
				SearchText: "test",
				Limit:      10,
				Offset:     0,
			},
			expectErr: false,
		},
		{
			name: "search - any",
			qry:  goqu.Select().From("test_table"),
			args: GridQuery{
				SearchType: SearchAny,
				SearchText: "test",
				Limit:      10,
				Offset:     0,
			},
			expectErr: false,
		},
		{
			name: "invalid search type",
			qry:  goqu.Select().From("test_table"),
			args: GridQuery{
				SearchType: 99,
				SearchText: "test",
				Limit:      10,
				Offset:     0,
			},
			expectErr: true,
		},
		{
			name: "sort fields",
			qry:  goqu.Select().From("test_table"),
			args: GridQuery{
				SearchType: SearchNone,
				SortFields: map[string]string{"id": SortAscending, "name": SortDescending},
				Limit:      10,
				Offset:     0,
			},
			expectErr: false,
		},
		{
			name: "invalid sort field",
			qry:  goqu.Select().From("test_table"),
			args: GridQuery{
				SearchType: SearchNone,
				SortFields: map[string]string{"nonexistent": SortAscending},
				Limit:      10,
				Offset:     0,
			},
			expectErr: true,
		},
		{
			name: "invalid sort order",
			qry:  goqu.Select().From("test_table"),
			args: GridQuery{
				SearchType: SearchNone,
				SortFields: map[string]string{"id": "invalid"},
				Limit:      10,
				Offset:     0,
			},
			expectErr: true,
		},
		{
			name: "limit and offset",
			qry:  goqu.Select().From("test_table"),
			args: GridQuery{
				SearchType: SearchNone,
				Limit:      10,
				Offset:     20,
			},
			expectErr: false,
		},
		{
			name: "offset only",
			qry:  goqu.Select().From("test_table"),
			args: GridQuery{
				SearchType: SearchNone,
				Offset:     20,
			},
			expectErr: false,
		},
		{
			name: "complete query",
			qry:  goqu.Select().From("test_table"),
			args: GridQuery{
				SearchType:   SearchAny,
				SearchText:   "test",
				FilterFields: map[string]any{"id": 1, "status": "yes"},
				SortFields:   map[string]string{"id": SortAscending, "name": SortDescending},
				Limit:        10,
				Offset:       20,
			},
			expectErr: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := grid.Build(tt.qry, &tt.args)
			if tt.expectErr {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				
				// Verify that the result is a valid SelectDataset
				sql, _, err := result.ToSQL()
				assert.NoError(t, err)
				assert.NotEmpty(t, sql)
			}
		})
	}
}

func TestGrid_Build_Specific(t *testing.T) {
	// This test focuses on specific cases to verify the SQL generated
	
	grid, _ := NewGrid("test_table", &TestGridRecord{})
	
	// Test case: SearchStart
	query := GridQuery{
		SearchType: SearchStart,
		SearchText: "test",
	}
	
	result, err := grid.Build(nil, &query)
	assert.NoError(t, err)
	sql, _, err := result.ToSQL()
	assert.NoError(t, err)
	assert.Contains(t, sql, "LIKE")
	assert.Contains(t, sql, "'test%'") // SearchStart: matches beginning
	
	// Test case: SearchEnd
	query = GridQuery{
		SearchType: SearchEnd,
		SearchText: "test",
	}
	
	result, err = grid.Build(nil, &query)
	assert.NoError(t, err)
	sql, _, err = result.ToSQL()
	assert.NoError(t, err)
	assert.Contains(t, sql, "LIKE")
	assert.Contains(t, sql, "'%test'") // SearchEnd: matches end
	
	// Test case: SearchAny
	query = GridQuery{
		SearchType: SearchAny,
		SearchText: "test",
	}
	
	result, err = grid.Build(nil, &query)
	assert.NoError(t, err)
	sql, _, err = result.ToSQL()
	assert.NoError(t, err)
	assert.Contains(t, sql, "LIKE")
	assert.Contains(t, sql, "'%test%'") // For different SQL dialects, the value might be quoted
	
	// Test case: SortAscending and SortDescending
	query = GridQuery{
		SearchType: SearchNone,
		SortFields: map[string]string{"id": SortAscending, "name": SortDescending},
	}
	
	result, err = grid.Build(nil, &query)
	assert.NoError(t, err)
	sql, _, err = result.ToSQL()
	assert.NoError(t, err)
	assert.Contains(t, sql, "ORDER BY")
	assert.Contains(t, sql, "ASC")
	assert.Contains(t, sql, "DESC")
	
	// Test case: Limit and Offset
	query = GridQuery{
		SearchType: SearchNone,
		Limit:      10,
		Offset:     20,
	}
	
	result, err = grid.Build(nil, &query)
	assert.NoError(t, err)
	sql, _, err = result.ToSQL()
	assert.NoError(t, err)
	assert.Contains(t, sql, "LIMIT 10")
	assert.Contains(t, sql, "OFFSET 20")
}