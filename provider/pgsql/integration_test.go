package pgsql

import (
	"context"
	"fmt"
	"github.com/oddbit-project/blueprint/db"
	"github.com/stretchr/testify/suite"
	"os"
	"testing"
)

// Integration test struct for ClickHouse client
type PGIntegrationTestSuite struct {
	suite.Suite
	client *db.SqlClient
	ctx    context.Context
}

func dbClient(t *testing.T) *db.SqlClient {

	cfg := NewClientConfig()
	cfg.DSN = resolveDSN()
	client, err := NewClient(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func resolveDSN() string {
	user := os.Getenv("POSTGRES_USER")
	pwd := os.Getenv("POSTGRES_PASSWORD")
	database := os.Getenv("POSTGRES_DB")
	port := os.Getenv("POSTGRES_PORT")
	host := os.Getenv("POSTGRES_HOST")
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s", user, pwd, host, port, database)
}

// SetupSuite prepares the test environment
func (s *PGIntegrationTestSuite) SetupSuite() {
	// Create context
	s.ctx = context.Background()

	// Create client config
	config := NewClientConfig()
	config.DSN = resolveDSN()
	var err error
	s.client, err = NewClient(config)
	if err != nil {
		s.T().Fatalf("Failed to create PostgreSQL client: %v", err)
	}
	if err = s.client.Connect(); err != nil {
		s.T().Fatal("Failed to open PostgreSQL connection")
	}
}

// Teardown the test suite
func (s *PGIntegrationTestSuite) TearDownSuite() {
	// Drop the test table
	_, err := s.client.Conn.ExecContext(s.ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s", MigrationTable))
	if err != nil {
		s.T().Logf("Failed to drop test table: %v", err)
	}
	// Close the client
	if s.client != nil {
		s.client.Disconnect()
	}
}

// Run the test suite
func TestPgIntegrationSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	suite.Run(t, new(PGIntegrationTestSuite))
}
