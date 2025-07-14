//go:build integration
// +build integration

package clickhouse

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

// Integration test struct for ClickHouse client
type ClickhouseIntegrationTestSuite struct {
	suite.Suite
	client *Client
	ctx    context.Context
}

// Record type for testing
type ClientTestRecord struct {
	ID        int32     `ch:"id"`
	Name      string    `ch:"name"`
	Value     float64   `ch:"value"`
	Timestamp time.Time `ch:"timestamp"`
	IsActive  uint8     `ch:"is_active"`
}

// SetupSuite prepares the test environment
func (s *ClickhouseIntegrationTestSuite) SetupSuite() {
	// Create context
	s.ctx = context.Background()

	// Create client config
	config := NewClientConfig()
	config.Hosts = []string{"clickhouse:9000"} // Docker-exposed port
	config.Database = "default"
	config.Username = "default"
	config.Password = "somePassword" // From docker-compose.yml

	// Create client
	var err error
	s.client, err = NewClient(config)
	if err != nil {
		s.T().Fatalf("Failed to create ClickHouse client: %v", err)
	}
}

// Teardown the test suite
func (s *ClickhouseIntegrationTestSuite) TearDownSuite() {
	// Drop the test table
	err := s.client.Conn.Exec(s.ctx, "DROP TABLE IF EXISTS test_integration")
	if err != nil {
		s.T().Logf("Failed to drop test table: %v", err)
	}

	// Close the client
	if s.client != nil {
		s.client.Close()
	}
}

// Helper to setup the test table
func (s *ClickhouseIntegrationTestSuite) setupTestTable() {
	// Create test table
	err := s.client.Conn.Exec(s.ctx, `
		CREATE TABLE IF NOT EXISTS test_integration (
			id        Int32,
			name      String,
			value     Float64,
			timestamp DateTime,
			is_active UInt8
		) ENGINE = MergeTree() ORDER BY id
	`)
	if err != nil {
		s.T().Fatalf("Failed to create test table: %v", err)
	}

	// Clear existing data
	err = s.client.Conn.Exec(s.ctx, "TRUNCATE TABLE test_integration")
	if err != nil {
		s.T().Fatalf("Failed to truncate test table: %v", err)
	}
}

// Test client connection
func (s *ClickhouseIntegrationTestSuite) TestConnection() {
	// Ping the server
	err := s.client.Ping(s.ctx)
	assert.NoError(s.T(), err, "Ping should succeed")

	// Check server version
	assert.NotNil(s.T(), s.client.Version, "Server version should be available")
	s.T().Logf("Connected to ClickHouse server version: %s", s.client.Version.String())
}

// Test basic query
func (s *ClickhouseIntegrationTestSuite) TestBasicQuery() {
	// Simple query
	var result string
	row := s.client.Conn.QueryRow(s.ctx, "SELECT 'Hello from ClickHouse'")
	err := row.Scan(&result)
	assert.NoError(s.T(), err, "Basic query should succeed")
	assert.Equal(s.T(), "Hello from ClickHouse", result)
}

// Test repository operations
func (s *ClickhouseIntegrationTestSuite) TestRepositoryOperations() {
	// Setup test table for this test
	s.setupTestTable()

	// Create repository
	repo := s.client.NewRepository(s.ctx, "test_integration")
	assert.NotNil(s.T(), repo, "Repository should be created")
	assert.Equal(s.T(), "test_integration", repo.Name(), "Repository should have correct name")

	// Insert test records
	now := time.Now().Round(time.Second) // Round to seconds as ClickHouse DateTime doesn't store milliseconds
	records := []interface{}{
		&ClientTestRecord{
			ID:        1,
			Name:      "Test Record 1",
			Value:     123.45,
			Timestamp: now,
			IsActive:  1,
		},
		&ClientTestRecord{
			ID:        2,
			Name:      "Test Record 2",
			Value:     678.90,
			Timestamp: now,
			IsActive:  0,
		},
	}

	err := repo.Insert(records...)
	assert.NoError(s.T(), err, "Insert should succeed")

	// Count records
	var count uint64
	row := s.client.Conn.QueryRow(s.ctx, "SELECT COUNT(*) FROM test_integration")
	err = row.Scan(&count)
	assert.NoError(s.T(), err, "Count query should succeed")
	assert.Equal(s.T(), uint64(2), count, "Should have 2 records")

	// Fetch active records
	var activeRecords []ClientTestRecord
	err = repo.FetchWhere(map[string]any{"is_active": uint8(1)}, &activeRecords)
	assert.NoError(s.T(), err, "FetchWhere should succeed")
	assert.Len(s.T(), activeRecords, 1, "Should have 1 active record")
	assert.Equal(s.T(), "Test Record 1", activeRecords[0].Name)

	// Fetch by key
	var record ClientTestRecord
	err = repo.FetchByKey("id", int32(2), &record)
	assert.NoError(s.T(), err, "FetchByKey should succeed")
	assert.Equal(s.T(), "Test Record 2", record.Name)
}

// Test direct SQL execution
func (s *ClickhouseIntegrationTestSuite) TestDirectSQLExecution() {
	// Setup test table for this test
	s.setupTestTable()

	// Create repository
	repo := s.client.NewRepository(s.ctx, "test_integration")

	// Prepare a record to insert manually
	now := time.Now().Round(time.Second)

	// Execute SQL directly
	err := s.client.Conn.Exec(s.ctx, `
		INSERT INTO test_integration (id, name, value, timestamp, is_active)
		VALUES (3, 'Direct SQL Record', 999.99, ?, 1)
	`, now)
	assert.NoError(s.T(), err, "Direct SQL execution should succeed")

	// Verify the record was inserted
	var fetchedRecord ClientTestRecord
	err = repo.FetchByKey("id", int32(3), &fetchedRecord)
	assert.NoError(s.T(), err, "FetchByKey should succeed after direct SQL insert")
	assert.Equal(s.T(), "Direct SQL Record", fetchedRecord.Name)
	assert.Equal(s.T(), 999.99, fetchedRecord.Value)
}

// Run the test suite
func TestClickhouseIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	suite.Run(t, new(ClickhouseIntegrationTestSuite))
	suite.Run(t, new(ClickhouseMigrationTestSuite))
}
