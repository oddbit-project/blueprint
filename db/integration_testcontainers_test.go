package db

import (
	"context"
	"testing"
	"time"

	"github.com/doug-martin/goqu/v9"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	_ "github.com/lib/pq" // PostgreSQL driver
	"github.com/oddbit-project/blueprint/db/qb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// IntegrationTestUser for integration testing
type IntegrationTestUser struct {
	ID        int64     `db:"id,auto" json:"id"`
	Name      string    `db:"name" json:"name"`
	Email     string    `db:"email" json:"email"`
	Status    string    `db:"status" json:"status"`
	CreatedAt time.Time `db:"created_at,auto" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at,auto" json:"updated_at"`
}

// ExtensiveTestUser - comprehensive test model with various field types
type ExtensiveTestUser struct {
	ID        int64          `db:"id,auto" json:"id" grid:"sort,filter"`
	Name      string         `db:"name" json:"name" grid:"sort,search,filter"`
	Email     string         `db:"email" json:"email" grid:"search,filter"`
	Age       *int           `db:"age,omitnil" json:"age,omitempty" grid:"sort,filter"`
	Status    string         `db:"status" json:"status" grid:"sort,filter"`
	Score     float64        `db:"score" json:"score" grid:"sort,filter"`
	IsActive  bool           `db:"is_active" json:"is_active" grid:"filter"`
	Bio       *string        `db:"bio,omitnil" json:"bio,omitempty" grid:"search"`
	Tags      pq.StringArray `db:"tags" json:"tags" grid:"filter"`
	CreatedAt time.Time      `db:"created_at,auto" json:"created_at" grid:"sort"`
	UpdatedAt time.Time      `db:"updated_at,auto" json:"updated_at" grid:"sort"`
	DeletedAt *time.Time     `db:"deleted_at,omitnil" json:"deleted_at,omitempty" grid:"sort,filter"`
}

// TestProfile - related table for complex queries
type TestProfile struct {
	ID       int64  `db:"id,auto" json:"id"`
	UserID   int64  `db:"user_id" json:"user_id"`
	Website  string `db:"website" json:"website"`
	Company  string `db:"company" json:"company"`
	Location string `db:"location" json:"location"`
}

// ExtensiveTestHelper provides comprehensive testing utilities
type ExtensiveTestHelper struct {
	userRepo    Repository
	profileRepo Repository
	cleanup     func()
	db          *sqlx.DB
}

// connOptions implements ConnectionOptions
type connOptions struct{}

func (c connOptions) Apply(db *sqlx.DB) error {
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	return nil
}

// DBIntegrationTestSuite manages the PostgreSQL testcontainer and provides comprehensive testing
type DBIntegrationTestSuite struct {
	suite.Suite
	ctx        context.Context
	cancel     context.CancelFunc
	container  testcontainers.Container
	pgInstance *postgres.PostgresContainer
	dsn        string
	client     *SqlClient
}

// SetupSuite prepares the test environment with testcontainers
func (s *DBIntegrationTestSuite) SetupSuite() {
	s.ctx, s.cancel = context.WithCancel(context.Background())

	// Create PostgreSQL testcontainer
	var err error
	s.pgInstance, err = postgres.Run(s.ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("blueprint"),
		postgres.WithUsername("blueprint"),
		postgres.WithPassword("password"),
		testcontainers.WithWaitStrategy(
			wait.ForListeningPort("5432/tcp").WithStartupTimeout(60*time.Second),
		),
	)
	require.NoError(s.T(), err, "Failed to start PostgreSQL container")
	s.container = s.pgInstance.Container

	// Get connection string
	s.dsn, err = s.pgInstance.ConnectionString(s.ctx, "sslmode=disable", "default_query_exec_mode=simple_protocol")
	require.NoError(s.T(), err, "Failed to get PostgreSQL connection string")

	s.T().Logf("PostgreSQL container started with DSN: %s", s.dsn)

	// Create client
	s.client = s.getTestClient()
	err = s.client.Connect()
	require.NoError(s.T(), err, "Failed to connect to PostgreSQL")

	// Register dialect
	RegisterDialect("pgx", PostgreSQLDialect())
}

// TearDownSuite cleans up after all tests
func (s *DBIntegrationTestSuite) TearDownSuite() {
	if s.client != nil {
		s.client.Disconnect()
	}

	// Terminate container first with its own context before cancelling the main context
	if s.container != nil {
		// Create a separate context for cleanup to avoid cancellation issues
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()

		err := s.container.Terminate(cleanupCtx)
		if err != nil {
			s.T().Logf("Failed to terminate PostgreSQL container: %v", err)
		}
	}

	// Cancel the main context after container cleanup
	if s.cancel != nil {
		s.cancel()
	}
}

// SetupTest runs before each test to clean up any existing state
func (s *DBIntegrationTestSuite) SetupTest() {
	// Clean up test tables to ensure clean state between tests
	if s.client != nil && s.client.Conn != nil {
		_, _ = s.client.Conn.ExecContext(s.ctx, "DROP TABLE IF EXISTS test_profiles CASCADE")
		_, _ = s.client.Conn.ExecContext(s.ctx, "DROP TABLE IF EXISTS extensive_test_users CASCADE")
		_, _ = s.client.Conn.ExecContext(s.ctx, "DROP TABLE IF EXISTS test_users CASCADE")
	}
}

// getTestClient creates a client using the testcontainer DSN
func (s *DBIntegrationTestSuite) getTestClient() *SqlClient {
	opts := connOptions{}
	return NewSqlClient(s.dsn, "pgx", opts)
}

// pgClientFromUrl creates a SQL client from URL (from original integration_test.go)
func pgClientFromUrl(url string) *SqlClient {
	opts := connOptions{}
	return NewSqlClient(url, "pgx", opts)
}

// setupBasicRepository creates a basic test repository with test_users table
func (s *DBIntegrationTestSuite) setupBasicRepository() Repository {
	// Create test table
	createTableSQL := `
		CREATE TABLE IF NOT EXISTS test_users (
			id SERIAL PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			email VARCHAR(255) UNIQUE NOT NULL,
			status VARCHAR(50) DEFAULT 'pending',
			created_at TIMESTAMP DEFAULT NOW(),
			updated_at TIMESTAMP DEFAULT NOW()
		)
	`
	_, err := s.client.Db().Exec(createTableSQL)
	require.NoError(s.T(), err)

	// Clean up any existing test data
	_, err = s.client.Db().Exec("DELETE FROM test_users")
	require.NoError(s.T(), err)

	// Create repository
	return NewRepository(s.ctx, s.client, "test_users")
}

// setupExtensiveRepository creates a comprehensive test environment
func (s *DBIntegrationTestSuite) setupExtensiveRepository() (*ExtensiveTestHelper, Repository, Repository) {
	// Create comprehensive test tables
	createTablesSQL := `
		-- Drop tables if they exist (in correct order due to foreign keys)
		DROP TABLE IF EXISTS test_profiles CASCADE;
		DROP TABLE IF EXISTS extensive_test_users CASCADE;

		-- Create users table with comprehensive field types
		CREATE TABLE extensive_test_users (
			id SERIAL PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			email VARCHAR(255) UNIQUE NOT NULL,
			age INTEGER,
			status VARCHAR(50) DEFAULT 'pending',
			score DECIMAL(10,2) DEFAULT 0.0,
			is_active BOOLEAN DEFAULT true,
			bio TEXT,
			tags TEXT[], -- PostgreSQL array type
			created_at TIMESTAMP DEFAULT NOW(),
			updated_at TIMESTAMP DEFAULT NOW(),
			deleted_at TIMESTAMP NULL
		);

		-- Create profiles table for relationship testing
		CREATE TABLE test_profiles (
			id SERIAL PRIMARY KEY,
			user_id INTEGER NOT NULL REFERENCES extensive_test_users(id) ON DELETE CASCADE,
			website VARCHAR(255),
			company VARCHAR(255),
			location VARCHAR(255)
		);

		-- Create indexes for performance testing
		CREATE INDEX idx_extensive_test_users_email ON extensive_test_users(email);
		CREATE INDEX idx_extensive_test_users_status ON extensive_test_users(status);
		CREATE INDEX idx_extensive_test_users_created_at ON extensive_test_users(created_at);
		CREATE INDEX idx_test_profiles_user_id ON test_profiles(user_id);
	`
	_, err := s.client.Db().Exec(createTablesSQL)
	require.NoError(s.T(), err)

	// Create repositories
	userRepo := NewRepository(s.ctx, s.client, "extensive_test_users")
	profileRepo := NewRepository(s.ctx, s.client, "test_profiles")

	helper := &ExtensiveTestHelper{
		userRepo:    userRepo,
		profileRepo: profileRepo,
		cleanup:     func() {}, // No-op since we handle cleanup in TearDown
		db:          s.client.Db(),
	}

	return helper, userRepo, profileRepo
}

// createSampleUsers creates a set of sample users for testing
func (h *ExtensiveTestHelper) createSampleUsers(t *testing.T) []ExtensiveTestUser {
	age25 := 25
	age30 := 30
	age35 := 35
	bio1 := "Software developer passionate about Go"
	bio2 := "Data scientist with ML expertise"

	users := []ExtensiveTestUser{
		{
			Name:     "Alice Johnson",
			Email:    "alice@example.com",
			Age:      &age25,
			Status:   "active",
			Score:    95.5,
			IsActive: true,
			Bio:      &bio1,
			Tags:     pq.StringArray{"developer", "golang", "backend"},
		},
		{
			Name:     "Bob Smith",
			Email:    "bob@example.com",
			Age:      &age30,
			Status:   "pending",
			Score:    87.2,
			IsActive: false,
			Bio:      nil, // Test omitnil
			Tags:     pq.StringArray{"manager", "product"},
		},
		{
			Name:     "Carol Williams",
			Email:    "carol@example.com",
			Age:      &age35,
			Status:   "active",
			Score:    92.8,
			IsActive: true,
			Bio:      &bio2,
			Tags:     pq.StringArray{"data-science", "python", "machine-learning"},
		},
		{
			Name:     "David Brown",
			Email:    "david@example.com",
			Age:      nil, // Test omitnil
			Status:   "inactive",
			Score:    76.1,
			IsActive: false,
			Bio:      nil,
			Tags:     pq.StringArray{"sales"},
		},
	}

	// Insert users and capture their IDs
	for i := range users {
		err := h.userRepo.InsertReturning(&users[i], pq.StringArray{"id", "created_at", "updated_at"}, &users[i])
		require.NoError(t, err, "Failed to insert user %d", i)
		require.NotZero(t, users[i].ID, "User %d should have non-zero ID", i)
	}

	return users
}

// createSampleProfiles creates profiles for the test users
func (h *ExtensiveTestHelper) createSampleProfiles(t *testing.T, users []ExtensiveTestUser) []TestProfile {
	profiles := []TestProfile{
		{
			UserID:   users[0].ID,
			Website:  "https://alice.dev",
			Company:  "TechCorp",
			Location: "San Francisco, CA",
		},
		{
			UserID:   users[1].ID,
			Website:  "https://bobsmith.com",
			Company:  "StartupXYZ",
			Location: "New York, NY",
		},
		{
			UserID:   users[2].ID,
			Website:  "https://carolml.io",
			Company:  "DataCorp",
			Location: "Seattle, WA",
		},
	}

	// Insert profiles
	for i := range profiles {
		err := h.profileRepo.InsertReturning(&profiles[i], pq.StringArray{"id"}, &profiles[i].ID)
		require.NoError(t, err, "Failed to insert profile %d", i)
		require.NotZero(t, profiles[i].ID, "Profile %d should have non-zero ID", i)
	}

	return profiles
}

// cleanupTestData removes all test data from tables
func (h *ExtensiveTestHelper) cleanupTestData(t *testing.T) {
	_, err := h.db.Exec("DELETE FROM test_profiles")
	require.NoError(t, err)
	_, err = h.db.Exec("DELETE FROM extensive_test_users")
	require.NoError(t, err)
	// Reset sequences
	_, err = h.db.Exec("ALTER SEQUENCE extensive_test_users_id_seq RESTART WITH 1")
	require.NoError(t, err)
	_, err = h.db.Exec("ALTER SEQUENCE test_profiles_id_seq RESTART WITH 1")
	require.NoError(t, err)
}

// PostgreSQLDialect creates a PostgreSQL dialect for the tests
func PostgreSQLDialect() qb.SqlDialect {
	return qb.PostgreSQLDialect()
}

// Test basic repository functionality
func (s *DBIntegrationTestSuite) TestBasicRepository() {
	repo := s.setupBasicRepository()

	// Test 1: Insert with struct target
	user1 := &IntegrationTestUser{
		Name:   "John Doe",
		Email:  "john@example.com",
		Status: "active",
	}

	result1 := &IntegrationTestUser{}
	err := repo.InsertReturning(user1, []string{"id", "name", "email", "status", "created_at"}, result1)
	require.NoError(s.T(), err)

	assert.NotZero(s.T(), result1.ID)
	assert.Equal(s.T(), "John Doe", result1.Name)
	assert.Equal(s.T(), "john@example.com", result1.Email)
	assert.Equal(s.T(), "active", result1.Status)
	assert.False(s.T(), result1.CreatedAt.IsZero())

	// Test 2: Insert with multiple variables
	user2 := &IntegrationTestUser{
		Name:   "Jane Smith",
		Email:  "jane@example.com",
		Status: "pending",
	}

	var id2 int64
	var name2 string
	var email2 string
	var status2 string
	err = repo.InsertReturning(user2, []string{"id", "name", "email", "status"}, &id2, &name2, &email2, &status2)
	require.NoError(s.T(), err)

	assert.NotZero(s.T(), id2)
	assert.Equal(s.T(), "Jane Smith", name2)
	assert.Equal(s.T(), "jane@example.com", email2)
	assert.Equal(s.T(), "pending", status2)

	// Verify count
	var count int64
	err = repo.Db().QueryRow("SELECT COUNT(*) FROM test_users").Scan(&count)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(2), count)
}

// Test update functionality
func (s *DBIntegrationTestSuite) TestUpdateReturning() {
	repo := s.setupBasicRepository()

	// Insert a user to update
	user := &IntegrationTestUser{
		Name:   "Original Name",
		Email:  "original@example.com",
		Status: "pending",
	}

	var originalID int64
	err := repo.InsertReturning(user, []string{"id"}, &originalID)
	require.NoError(s.T(), err)

	// Test UpdateReturning with struct target
	updateUser := &IntegrationTestUser{
		Name:   "Updated Name",
		Email:  "updated@example.com",
		Status: "active",
	}

	whereConditions := map[string]any{"id": originalID}
	result := &IntegrationTestUser{}
	err = repo.UpdateReturning(updateUser, whereConditions, []string{"id", "name", "email", "status", "updated_at"}, result)
	require.NoError(s.T(), err)

	assert.Equal(s.T(), originalID, result.ID)
	assert.Equal(s.T(), "Updated Name", result.Name)
	assert.Equal(s.T(), "updated@example.com", result.Email)
	assert.Equal(s.T(), "active", result.Status)
	assert.False(s.T(), result.UpdatedAt.IsZero())
}

// Test UpdateFieldsReturning functionality
func (s *DBIntegrationTestSuite) TestUpdateFieldsReturning() {
	repo := s.setupBasicRepository()

	// Insert a user to update
	user := &IntegrationTestUser{
		Name:   "Fields Test User",
		Email:  "fields@example.com",
		Status: "pending",
	}

	var originalID int64
	err := repo.InsertReturning(user, []string{"id"}, &originalID)
	require.NoError(s.T(), err)

	// Test UpdateFieldsReturning with specific fields
	fieldsToUpdate := map[string]any{
		"name":   "Updated Fields Name",
		"status": "active",
	}

	whereConditions := map[string]any{"id": originalID}
	result := &IntegrationTestUser{}
	err = repo.UpdateFieldsReturning(&IntegrationTestUser{}, fieldsToUpdate, whereConditions, []string{"id", "name", "email", "status"}, result)
	require.NoError(s.T(), err)

	assert.Equal(s.T(), originalID, result.ID)
	assert.Equal(s.T(), "Updated Fields Name", result.Name)
	assert.Equal(s.T(), "fields@example.com", result.Email) // Should remain unchanged
	assert.Equal(s.T(), "active", result.Status)
}

// Test transaction functionality
func (s *DBIntegrationTestSuite) TestTransactions() {
	repo := s.setupBasicRepository()

	s.T().Run("TransactionCommit", func(t *testing.T) {
		// Start transaction
		tx, err := repo.NewTransaction(nil)
		require.NoError(t, err)

		// Insert user in transaction
		user1 := &IntegrationTestUser{
			Name:   "TX User 1",
			Email:  "tx1@example.com",
			Status: "pending",
		}

		var id1 int64
		err = tx.InsertReturning(user1, []string{"id"}, &id1)
		require.NoError(t, err)
		assert.NotZero(t, id1)

		// Update user in transaction
		updateFields := map[string]any{"status": "active"}
		whereConditions := map[string]any{"id": id1}

		var updatedStatus string
		err = tx.UpdateFieldsReturning(&IntegrationTestUser{}, updateFields, whereConditions, []string{"status"}, &updatedStatus)
		require.NoError(t, err)
		assert.Equal(t, "active", updatedStatus)

		// Commit transaction
		err = tx.Commit()
		require.NoError(t, err)

		// Verify changes persisted
		var count int64
		err = repo.Db().QueryRow("SELECT COUNT(*) FROM test_users WHERE id = $1 AND status = 'active'", id1).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, int64(1), count)
	})

	s.T().Run("TransactionRollback", func(t *testing.T) {
		// Get initial count
		var initialCount int64
		err := repo.Db().QueryRow("SELECT COUNT(*) FROM test_users").Scan(&initialCount)
		require.NoError(t, err)

		// Start transaction
		tx, err := repo.NewTransaction(nil)
		require.NoError(t, err)

		// Insert user in transaction
		user := &IntegrationTestUser{
			Name:   "Rollback User",
			Email:  "rollback@example.com",
			Status: "pending",
		}

		var id int64
		err = tx.InsertReturning(user, []string{"id"}, &id)
		require.NoError(t, err)
		assert.NotZero(t, id)

		// Rollback transaction
		err = tx.Rollback()
		require.NoError(t, err)

		// Verify no changes persisted
		var finalCount int64
		err = repo.Db().QueryRow("SELECT COUNT(*) FROM test_users").Scan(&finalCount)
		require.NoError(t, err)
		assert.Equal(t, initialCount, finalCount)
	})
}

// Test extensive repository functionality
func (s *DBIntegrationTestSuite) TestExtensiveRepository() {
	helper, userRepo, _ := s.setupExtensiveRepository()

	// Create sample users
	users := helper.createSampleUsers(s.T())
	require.Len(s.T(), users, 4, "Should create 4 sample users")

	// Create sample profiles
	profiles := helper.createSampleProfiles(s.T(), users)
	require.Len(s.T(), profiles, 3, "Should create 3 sample profiles")

	s.T().Run("FetchOperations", func(t *testing.T) {
		// Test FetchOne
		var user ExtensiveTestUser
		err := userRepo.FetchOne(userRepo.SqlSelect().Where(goqu.C("email").Eq("alice@example.com")), &user)
		require.NoError(t, err)
		assert.Equal(t, "Alice Johnson", user.Name)
		assert.Equal(t, "alice@example.com", user.Email)

		// Test Fetch multiple
		var allUsers []ExtensiveTestUser
		err = userRepo.Fetch(userRepo.SqlSelect().Order(goqu.C("id").Asc()), &allUsers)
		require.NoError(t, err)
		assert.Len(t, allUsers, 4)

		// Test FetchRecord
		var activeUser ExtensiveTestUser
		err = userRepo.FetchRecord(map[string]any{"status": "active"}, &activeUser)
		require.NoError(t, err)
		assert.Equal(t, "active", activeUser.Status)

		// Test FetchWhere
		var activeUsers []ExtensiveTestUser
		err = userRepo.FetchWhere(map[string]any{"status": "active"}, &activeUsers)
		require.NoError(t, err)
		assert.Len(t, activeUsers, 2) // Alice and Carol are active
	})

	s.T().Run("CountOperations", func(t *testing.T) {
		// Test Count
		count, err := userRepo.Count()
		require.NoError(t, err)
		assert.Equal(t, int64(4), count)

		// Test CountWhere
		activeCount, err := userRepo.CountWhere(map[string]any{"status": "active"})
		require.NoError(t, err)
		assert.Equal(t, int64(2), activeCount)
	})

	s.T().Run("ExistsOperations", func(t *testing.T) {
		// Test Exists - existing email
		exists, err := userRepo.Exists("email", "bob@example.com")
		require.NoError(t, err)
		assert.True(t, exists)

		// Test Exists - non-existing email
		exists, err = userRepo.Exists("email", "nonexistent@example.com")
		require.NoError(t, err)
		assert.False(t, exists)
	})

	s.T().Run("DeleteOperations", func(t *testing.T) {
		// Test DeleteWhere
		err := userRepo.DeleteWhere(map[string]any{"status": "inactive"})
		require.NoError(t, err)

		// Verify deletion
		count, err := userRepo.CountWhere(map[string]any{"status": "inactive"})
		require.NoError(t, err)
		assert.Equal(t, int64(0), count)
	})
}

// Test error handling
func (s *DBIntegrationTestSuite) TestErrorHandling() {
	repo := s.setupBasicRepository()

	// Test duplicate email error
	user1 := &IntegrationTestUser{
		Name:   "User 1",
		Email:  "duplicate@example.com",
		Status: "active",
	}

	var id1 int64
	err := repo.InsertReturning(user1, []string{"id"}, &id1)
	require.NoError(s.T(), err)

	// Try to insert with same email (should fail due to unique constraint)
	user2 := &IntegrationTestUser{
		Name:   "User 2",
		Email:  "duplicate@example.com", // Same email
		Status: "pending",
	}

	var id2 int64
	err = repo.InsertReturning(user2, []string{"id"}, &id2)
	assert.Error(s.T(), err)
	assert.Zero(s.T(), id2)

	// Test update with non-existent ID
	whereConditions := map[string]any{"id": 99999} // Non-existent ID
	result := &IntegrationTestUser{}
	err = repo.UpdateReturning(&IntegrationTestUser{Name: "Non-existent User"}, whereConditions, []string{"id", "name"}, result)

	// Should return ErrNoRows
	if err == nil {
		// If no error, result should be empty (no rows affected)
		assert.Zero(s.T(), result.ID)
	} else {
		// Or should return a "no rows" type error
		assert.True(s.T(), EmptyResult(err) || err != nil)
	}
}

// Test repository factory
func (s *DBIntegrationTestSuite) TestRepositoryFactory() {
	s.T().Run("NewRepository", func(t *testing.T) {
		// Create repository
		repo := NewRepository(s.ctx, s.client, "test_table")
		require.NotNil(t, repo)

		// Verify repository properties
		assert.Equal(t, "test_table", repo.Name())
		assert.NotNil(t, repo.Db())
	})

	s.T().Run("RepositoryMethods", func(t *testing.T) {
		repo := NewRepository(s.ctx, s.client, "test_table")

		// Test SQL methods
		dialect := repo.Sql()
		require.NotNil(t, dialect)

		// Create custom select query
		query := dialect.Select(goqu.L("COUNT(*)")).From("information_schema.tables")
		sql, _, err := query.ToSQL()
		require.NoError(t, err)
		assert.Contains(t, sql, "SELECT COUNT(*)")
		assert.Contains(t, sql, "FROM")
	})
}

// Test array and null handling
func (s *DBIntegrationTestSuite) TestAdvancedTypes() {
	helper, userRepo, _ := s.setupExtensiveRepository()

	s.T().Run("ArrayHandling", func(t *testing.T) {
		user := ExtensiveTestUser{
			Name:     "Array Test User",
			Email:    "array@example.com",
			Status:   "active",
			Score:    85.0,
			IsActive: true,
			Tags:     pq.StringArray{"one", "two", "three"},
		}

		result := &ExtensiveTestUser{}
		err := userRepo.InsertReturning(&user, []string{"id", "name", "tags"}, result)
		require.NoError(t, err)
		assert.NotZero(t, result.ID)
		assert.Equal(t, "Array Test User", result.Name)
		assert.Equal(t, []string{"one", "two", "three"}, []string(result.Tags))
	})

	s.T().Run("NullHandling", func(t *testing.T) {
		// Count users with null age
		count, err := userRepo.CountWhere(map[string]any{"age": nil})
		require.NoError(t, err)
		assert.GreaterOrEqual(t, count, int64(0))

		// Fetch users with null bio
		var results []ExtensiveTestUser
		err = userRepo.FetchWhere(map[string]any{"bio": nil}, &results)
		require.NoError(t, err)
		// Verify all have null bio
		for _, user := range results {
			assert.Nil(t, user.Bio)
		}
	})

	// Clean up
	helper.cleanupTestData(s.T())
}

// Test complex queries and aggregation
func (s *DBIntegrationTestSuite) TestComplexQueries() {
	helper, userRepo, _ := s.setupExtensiveRepository()

	// Create sample data
	users := helper.createSampleUsers(s.T())
	require.Len(s.T(), users, 4)

	s.T().Run("AggregationQueries", func(t *testing.T) {
		type AggResult struct {
			TotalUsers  int64   `db:"total_users"`
			AvgScore    float64 `db:"avg_score"`
			MaxScore    float64 `db:"max_score"`
			MinScore    float64 `db:"min_score"`
			ActiveCount int64   `db:"active_count"`
		}

		query := userRepo.SqlSelect().Select(
			goqu.COUNT("*").As("total_users"),
			goqu.AVG("score").As("avg_score"),
			goqu.MAX("score").As("max_score"),
			goqu.MIN("score").As("min_score"),
			goqu.SUM(goqu.Case().When(goqu.C("is_active").IsTrue(), 1).Else(0)).As("active_count"),
		)

		var result AggResult
		err := userRepo.FetchOne(query, &result)
		require.NoError(t, err)
		assert.Equal(t, int64(4), result.TotalUsers)
		assert.Greater(t, result.AvgScore, 0.0)
	})

	// Clean up
	helper.cleanupTestData(s.T())
}

// Test dbgrid functionality
func (s *DBIntegrationTestSuite) TestDbGrid() {
	helper, userRepo, _ := s.setupExtensiveRepository()

	// Create sample data
	users := helper.createSampleUsers(s.T())
	require.Len(s.T(), users, 4)

	s.T().Run("GridCreation", func(t *testing.T) {
		var record ExtensiveTestUser
		grid, err := userRepo.Grid(&record)
		require.NoError(t, err)
		require.NotNil(t, grid)
	})

	s.T().Run("SearchFunctionality", func(t *testing.T) {
		// Test search by name
		query, err := NewGridQuery(SearchAny, 0, 0)
		require.NoError(t, err)
		query.SearchText = "Alice"

		var results []ExtensiveTestUser
		err = userRepo.QueryGrid(&ExtensiveTestUser{}, query, &results)
		require.NoError(t, err)
		assert.Len(t, results, 1)
		assert.Equal(t, "Alice Johnson", results[0].Name)
	})

	s.T().Run("FilterFunctionality", func(t *testing.T) {
		// Test filter by status
		query, err := NewGridQuery(SearchNone, 0, 0)
		require.NoError(t, err)
		query.FilterFields = map[string]any{"status": "active"}

		var results []ExtensiveTestUser
		err = userRepo.QueryGrid(&ExtensiveTestUser{}, query, &results)
		require.NoError(t, err)
		assert.Len(t, results, 2) // Alice and Carol are active
		for _, user := range results {
			assert.Equal(t, "active", user.Status)
		}
	})

	s.T().Run("SortFunctionality", func(t *testing.T) {
		// Test sort by score descending
		query, err := NewGridQuery(SearchNone, 0, 0)
		require.NoError(t, err)
		query.SortFields = map[string]string{"score": "desc"}

		var results []ExtensiveTestUser
		err = userRepo.QueryGrid(&ExtensiveTestUser{}, query, &results)
		require.NoError(t, err)
		assert.Len(t, results, 4)
		
		// Should be sorted by score descending
		for i := 0; i < len(results)-1; i++ {
			assert.GreaterOrEqual(t, results[i].Score, results[i+1].Score)
		}
	})

	s.T().Run("PaginationFunctionality", func(t *testing.T) {
		// Test pagination with limit 2
		query, err := NewGridQuery(SearchNone, 2, 0)
		require.NoError(t, err)

		var results []ExtensiveTestUser
		err = userRepo.QueryGrid(&ExtensiveTestUser{}, query, &results)
		require.NoError(t, err)
		assert.Len(t, results, 2)

		// Test pagination with offset
		query2, err := NewGridQuery(SearchNone, 2, 2)
		require.NoError(t, err)

		var results2 []ExtensiveTestUser
		err = userRepo.QueryGrid(&ExtensiveTestUser{}, query2, &results2)
		require.NoError(t, err)
		assert.Len(t, results2, 2)

		// Results should be different
		assert.NotEqual(t, results[0].ID, results2[0].ID)
	})

	// Clean up
	helper.cleanupTestData(s.T())
}

// Run the test suite
func TestDBIntegrationSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	suite.Run(t, new(DBIntegrationTestSuite))
}
