//go:build integration && pgsql
// +build integration,pgsql

package pgsql

import (
	"context"
	"fmt"
	"github.com/oddbit-project/blueprint/db"
	"github.com/stretchr/testify/suite"
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
	// Disable prepared statement cache to avoid "cached plan must not change result type" errors
	// Use default_query_exec_mode=simple_protocol for pgx driver
	return "postgres://blueprint:password@postgres:5432/blueprint?default_query_exec_mode=simple_protocol"
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
// SetupTest runs before each test to clean up any existing state
func (s *PGIntegrationTestSuite) SetupTest() {
	// Clean up migration table to ensure clean state between tests
	if s.client != nil && s.client.Conn != nil {
		_, _ = s.client.Conn.ExecContext(s.ctx, "DROP TABLE IF EXISTS blueprint.db_migration")
		_, _ = s.client.Conn.ExecContext(s.ctx, "DROP SCHEMA IF EXISTS blueprint CASCADE")
	}
}

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
