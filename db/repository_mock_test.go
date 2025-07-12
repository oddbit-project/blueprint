package db

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/oddbit-project/blueprint/db/qb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/doug-martin/goqu/v9"
)

// TestUser for testing repository methods
type TestUser struct {
	ID        int64     `db:"id"`
	Name      string    `db:"name"`
	Email     string    `db:"email"`
	Status    string    `db:"status"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

// MockRepositoryHelper provides helper functions for creating mock repositories
type MockRepositoryHelper struct {
	repo    *repository
	mock    sqlmock.Sqlmock
	cleanup func()
}

// CreateMockRepository creates a repository with a mocked database connection
func CreateMockRepository(t *testing.T, tableName string) *MockRepositoryHelper {
	// Create mock database
	mockDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)

	// Create sqlx DB from mock
	db := sqlx.NewDb(mockDB, "sqlmock")

	// Create repository
	repo := &repository{
		conn:       db,
		ctx:        context.Background(),
		tableName:  tableName,
		dialect:    goqu.Dialect("postgres"),
		sqlBuilder: qb.NewSqlBuilder(qb.PostgreSQLDialect()),
	}

	// Return cleanup function
	cleanup := func() {
		db.Close()
		// Verify all expectations were met
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unmet mock expectations: %v", err)
		}
	}

	return &MockRepositoryHelper{
		repo:    repo,
		mock:    mock,
		cleanup: cleanup,
	}
}

// TestRepository_InsertReturning_StructTarget tests InsertReturning with struct target
func TestRepository_InsertReturning_StructTarget(t *testing.T) {
	helper := CreateMockRepository(t, "users")
	defer helper.cleanup()

	user := &TestUser{
		Name:   "John Doe",
		Email:  "john@example.com",
		Status: "active",
	}

	// Expected SQL (simplified pattern)
	expectedSQL := `INSERT INTO "users" .* RETURNING "id", "name", "created_at"`
	expectedTime := time.Now()

	// Mock the query execution
	rows := sqlmock.NewRows([]string{"id", "name", "created_at"}).
		AddRow(1, "John Doe", expectedTime)

	helper.mock.ExpectQuery(expectedSQL).
		WithArgs(sqlmock.AnyArg(), "John Doe", "john@example.com", "active", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(rows)

	// Test struct target
	result := &TestUser{}
	err := helper.repo.InsertReturning(user, []string{"id", "name", "created_at"}, result)

	// Assertions
	assert.NoError(t, err)
	assert.Equal(t, int64(1), result.ID)
	assert.Equal(t, "John Doe", result.Name)
	assert.Equal(t, expectedTime, result.CreatedAt)
}

// TestRepository_InsertReturning_MultipleVariables tests InsertReturning with multiple variables
func TestRepository_InsertReturning_MultipleVariables(t *testing.T) {
	helper := CreateMockRepository(t, "users")
	defer helper.cleanup()

	user := &TestUser{
		Name:   "Jane Doe",
		Email:  "jane@example.com",
		Status: "pending",
	}

	// Expected SQL
	expectedSQL := `INSERT INTO "users" .* RETURNING "id", "name", "status"`

	// Mock the query execution
	rows := sqlmock.NewRows([]string{"id", "name", "status"}).
		AddRow(2, "Jane Doe", "pending")

	helper.mock.ExpectQuery(expectedSQL).
		WithArgs(sqlmock.AnyArg(), "Jane Doe", "jane@example.com", "pending", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(rows)

	// Test multiple variables target
	var id int64
	var name string
	var status string
	err := helper.repo.InsertReturning(user, []string{"id", "name", "status"}, &id, &name, &status)

	// Assertions
	assert.NoError(t, err)
	assert.Equal(t, int64(2), id)
	assert.Equal(t, "Jane Doe", name)
	assert.Equal(t, "pending", status)
}

// TestRepository_InsertReturning_SingleVariable tests InsertReturning with single variable
func TestRepository_InsertReturning_SingleVariable(t *testing.T) {
	helper := CreateMockRepository(t, "users")
	defer helper.cleanup()

	user := &TestUser{
		Name:   "Bob Smith",
		Email:  "bob@example.com",
		Status: "active",
	}

	// Expected SQL
	expectedSQL := `INSERT INTO "users" .* RETURNING "id"`

	// Mock the query execution
	rows := sqlmock.NewRows([]string{"id"}).
		AddRow(3)

	helper.mock.ExpectQuery(expectedSQL).
		WithArgs(sqlmock.AnyArg(), "Bob Smith", "bob@example.com", "active", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(rows)

	// Test single variable target
	var id int64
	err := helper.repo.InsertReturning(user, []string{"id"}, &id)

	// Assertions
	assert.NoError(t, err)
	assert.Equal(t, int64(3), id)
}

// TestRepository_UpdateReturning_StructTarget tests UpdateReturning with struct target
func TestRepository_UpdateReturning_StructTarget(t *testing.T) {
	helper := CreateMockRepository(t, "users")
	defer helper.cleanup()

	user := &TestUser{
		Name:   "John Updated",
		Email:  "john.updated@example.com",
		Status: "verified",
	}

	whereConditions := map[string]any{
		"id": 1,
	}

	// Expected SQL pattern
	expectedSQL := `UPDATE "users" SET .* WHERE .* RETURNING "id", "name", "status", "updated_at"`
	expectedTime := time.Now()

	// Mock the query execution
	rows := sqlmock.NewRows([]string{"id", "name", "status", "updated_at"}).
		AddRow(1, "John Updated", "verified", expectedTime)

	helper.mock.ExpectQuery(expectedSQL).WillReturnRows(rows)

	// Test struct target
	result := &TestUser{}
	err := helper.repo.UpdateReturning(user, whereConditions, []string{"id", "name", "status", "updated_at"}, result)

	// Assertions
	assert.NoError(t, err)
	assert.Equal(t, int64(1), result.ID)
	assert.Equal(t, "John Updated", result.Name)
	assert.Equal(t, "verified", result.Status)
	assert.Equal(t, expectedTime, result.UpdatedAt)
}

// TestRepository_UpdateFieldsReturning_MultipleVariables tests UpdateFieldsReturning with multiple variables
func TestRepository_UpdateFieldsReturning_MultipleVariables(t *testing.T) {
	helper := CreateMockRepository(t, "users")
	defer helper.cleanup()

	fieldsToUpdate := map[string]any{
		"name":   "Updated Name",
		"status": "active",
	}

	whereConditions := map[string]any{
		"id": 1,
	}

	// Expected SQL pattern
	expectedSQL := `UPDATE "users" SET .* WHERE .* RETURNING "id", "name", "status"`

	// Mock the query execution
	rows := sqlmock.NewRows([]string{"id", "name", "status"}).
		AddRow(1, "Updated Name", "active")

	helper.mock.ExpectQuery(expectedSQL).WillReturnRows(rows)

	// Test with multiple variables
	var id int64
	var name string
	var status string
	err := helper.repo.UpdateFieldsReturning(&TestUser{}, fieldsToUpdate, whereConditions, []string{"id", "name", "status"}, &id, &name, &status)

	// Assertions
	assert.NoError(t, err)
	assert.Equal(t, int64(1), id)
	assert.Equal(t, "Updated Name", name)
	assert.Equal(t, "active", status)
}

// TestRepository_FetchOne tests the FetchOne method
func TestRepository_FetchOne(t *testing.T) {
	helper := CreateMockRepository(t, "users")
	defer helper.cleanup()

	// Expected SQL - The actual SQL generated by goqu
	expectedSQL := `SELECT \* FROM "users" LIMIT 1`
	expectedTime := time.Now()

	// Mock the query execution
	rows := sqlmock.NewRows([]string{"id", "name", "email", "status", "created_at", "updated_at"}).
		AddRow(1, "John Doe", "john@example.com", "active", expectedTime, expectedTime)

	helper.mock.ExpectQuery(expectedSQL).WillReturnRows(rows)

	// Test
	result := &TestUser{}
	qry := helper.repo.SqlSelect()
	err := helper.repo.FetchOne(qry, result)

	// Assertions
	assert.NoError(t, err)
	assert.Equal(t, int64(1), result.ID)
	assert.Equal(t, "John Doe", result.Name)
	assert.Equal(t, "john@example.com", result.Email)
	assert.Equal(t, "active", result.Status)
}

// TestRepository_ErrorHandling tests error cases
func TestRepository_ErrorHandling(t *testing.T) {
	helper := CreateMockRepository(t, "users")
	defer helper.cleanup()

	// Test InsertReturning with no targets
	err := helper.repo.InsertReturning(&TestUser{}, []string{"id"})
	assert.Equal(t, ErrInvalidParameters, err)

	// Test UpdateReturning with no targets  
	err = helper.repo.UpdateReturning(&TestUser{}, map[string]any{"id": 1}, []string{"id"})
	assert.Equal(t, ErrInvalidParameters, err)

	// Test UpdateFieldsReturning with no targets
	err = helper.repo.UpdateFieldsReturning(&TestUser{}, map[string]any{"name": "test"}, map[string]any{"id": 1}, []string{"id"})
	assert.Equal(t, ErrInvalidParameters, err)
}

// TestRepository_Transaction tests transaction creation and basic operations
func TestRepository_Transaction(t *testing.T) {
	helper := CreateMockRepository(t, "users")
	defer helper.cleanup()

	// Mock transaction begin
	helper.mock.ExpectBegin()

	// Create transaction
	tx, err := helper.repo.NewTransaction(nil)
	require.NoError(t, err)
	require.NotNil(t, tx)

	// Test transaction methods exist and are properly typed
	assert.Equal(t, "users", tx.Name())
	assert.NotNil(t, tx.SqlSelect())
	assert.NotNil(t, tx.SqlInsert())
	assert.NotNil(t, tx.SqlUpdate())
	assert.NotNil(t, tx.SqlDelete())

	// Mock rollback for cleanup
	helper.mock.ExpectRollback()
	err = tx.Rollback()
	assert.NoError(t, err)
}

// TestTransaction_InsertReturning tests transaction InsertReturning
func TestTransaction_InsertReturning(t *testing.T) {
	helper := CreateMockRepository(t, "users")
	defer helper.cleanup()

	// Mock transaction begin
	helper.mock.ExpectBegin()

	// Create transaction
	tx, err := helper.repo.NewTransaction(nil)
	require.NoError(t, err)

	user := &TestUser{
		Name:   "Transaction User",
		Email:  "tx@example.com",
		Status: "pending",
	}

	// Expected SQL
	expectedSQL := `INSERT INTO "users" .* RETURNING "id", "name"`

	// Mock the query execution
	rows := sqlmock.NewRows([]string{"id", "name"}).
		AddRow(4, "Transaction User")

	helper.mock.ExpectQuery(expectedSQL).
		WithArgs(sqlmock.AnyArg(), "Transaction User", "tx@example.com", "pending", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(rows)

	// Test multiple variables in transaction
	var id int64
	var name string
	err = tx.InsertReturning(user, []string{"id", "name"}, &id, &name)

	// Assertions
	assert.NoError(t, err)
	assert.Equal(t, int64(4), id)
	assert.Equal(t, "Transaction User", name)

	// Mock commit
	helper.mock.ExpectCommit()
	err = tx.Commit()
	assert.NoError(t, err)
}