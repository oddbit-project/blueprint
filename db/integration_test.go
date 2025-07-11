//go:build integration
// +build integration

package db

import (
	"context"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"github.com/oddbit-project/blueprint/db/qb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
	"time"
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
	ID        int64          `db:"id,auto" json:"id"`
	Name      string         `db:"name" json:"name"`
	Email     string         `db:"email" json:"email"`
	Age       *int           `db:"age,omitnil" json:"age,omitempty"`
	Status    string         `db:"status" json:"status"`
	Score     float64        `db:"score" json:"score"`
	IsActive  bool           `db:"is_active" json:"is_active"`
	Bio       *string        `db:"bio,omitnil" json:"bio,omitempty"`
	Tags      pq.StringArray `db:"tags" json:"tags"`
	CreatedAt time.Time      `db:"created_at,auto" json:"created_at"`
	UpdatedAt time.Time      `db:"updated_at,auto" json:"updated_at"`
	DeletedAt *time.Time     `db:"deleted_at,omitnil" json:"deleted_at,omitempty"`
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

type connOptions struct {
}

func (c connOptions) Apply(db *sqlx.DB) error {
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	return nil
}

func pgClientFromUrl(url string) *SqlClient {
	return NewSqlClient(url, "pgx", connOptions{})
}

// setupIntegrationTest sets up a test database and repository
func setupIntegrationTest(t *testing.T) (Repository, func()) {
	// Get database URL from environment
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL not set, skipping integration tests")
	}

	client := pgClientFromUrl(dbURL)

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
	_, err := client.Db().Exec(createTableSQL)
	require.NoError(t, err)

	// Clean up any existing test data
	_, err = client.Db().Exec("DELETE FROM test_users")
	require.NoError(t, err)

	// Create repository
	repo := NewRepository(context.Background(), client, "test_users")

	// Return cleanup function
	cleanup := func() {
		// Clean up test data
		client.Db().Exec("DELETE FROM test_users")
		client.Db().Close()
	}

	return repo, cleanup
}

// setupExtensiveIntegrationTest creates a comprehensive test environment
func setupExtensiveIntegrationTest(t *testing.T) *ExtensiveTestHelper {
	// Get database URL from environment
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL not set, skipping extensive integration tests")
	}

	client := pgClientFromUrl(dbURL)

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
	_, err := client.Db().Exec(createTablesSQL)
	require.NoError(t, err)

	// Create repositories
	userRepo := NewRepository(context.Background(), client, "extensive_test_users")

	profileRepo := NewRepository(context.Background(), client, "test_profiles")

	// Return cleanup function
	cleanup := func() {
		// Clean up test data and tables
		client.Db().Exec("DROP TABLE IF EXISTS test_profiles CASCADE")
		client.Db().Exec("DROP TABLE IF EXISTS extensive_test_users CASCADE")
		client.Db().Close()
	}

	return &ExtensiveTestHelper{
		userRepo:    userRepo,
		profileRepo: profileRepo,
		cleanup:     cleanup,
		db:          client.Db(),
	}
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

// assertUserEqual compares two users for equality (ignoring auto fields)
func assertUserEqual(t *testing.T, expected, actual *ExtensiveTestUser, msg string) {
	assert.Equal(t, expected.Name, actual.Name, "%s: name mismatch", msg)
	assert.Equal(t, expected.Email, actual.Email, "%s: email mismatch", msg)
	assert.Equal(t, expected.Status, actual.Status, "%s: status mismatch", msg)
	assert.Equal(t, expected.Score, actual.Score, "%s: score mismatch", msg)
	assert.Equal(t, expected.IsActive, actual.IsActive, "%s: is_active mismatch", msg)
	assert.Equal(t, []string(expected.Tags), []string(actual.Tags), "%s: tags mismatch", msg)

	// Handle nullable fields
	if expected.Age != nil && actual.Age != nil {
		assert.Equal(t, *expected.Age, *actual.Age, "%s: age mismatch", msg)
	} else {
		assert.Equal(t, expected.Age == nil, actual.Age == nil, "%s: age null mismatch", msg)
	}

	if expected.Bio != nil && actual.Bio != nil {
		assert.Equal(t, *expected.Bio, *actual.Bio, "%s: bio mismatch", msg)
	} else {
		assert.Equal(t, expected.Bio == nil, actual.Bio == nil, "%s: bio null mismatch", msg)
	}
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

func init() {
	RegisterDialect("pgx", qb.PostgreSQLDialect())
}
