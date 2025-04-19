//go:build integration
// +build integration

package clickhouse

import (
	"context"
	"fmt"
	"github.com/oddbit-project/blueprint/db/migrations"
	"github.com/oddbit-project/blueprint/provider/clickhouse"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

// Integration test struct for ClickHouse client
type ClickhouseMigrationTestSuite struct {
	suite.Suite
	client *clickhouse.Client
	ctx    context.Context
}

// SetupSuite prepares the test environment
func (s *ClickhouseMigrationTestSuite) SetupSuite() {
	// Create context
	s.ctx = context.Background()

	// Create client config
	config := clickhouse.NewClientConfig()
	config.Hosts = []string{"clickhouse:9000"} // Docker-exposed port
	config.Database = "default"
	config.Username = "default"
	config.Password = "somePassword" // From docker-compose.yml

	// Create client
	var err error
	s.client, err = clickhouse.NewClient(config)
	if err != nil {
		s.T().Fatalf("Failed to create ClickHouse client: %v", err)
	}
}

// Teardown the test suite
func (s *ClickhouseMigrationTestSuite) TearDownSuite() {
	// Drop the test table
	err := s.client.Conn.Exec(s.ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s", clickhouse.MigrationTable))
	if err != nil {
		s.T().Logf("Failed to drop test table: %v", err)
	}

	// Close the client
	if s.client != nil {
		s.client.Close()
	}
}

// Test basic query
func (s *ClickhouseMigrationTestSuite) TestMigrationManager() {
	src := migrations.NewMemorySource()

	src.Add("sample1.sql", "create table if not exists sample(id Int32) engine=TinyLog;")
	src.Add("sample2.sql", "insert into sample(id) values(1);")

	// create migration manager
	mgr, err := clickhouse.NewMigrationManager(context.Background(), s.client, "")
	assert.Nil(s.T(), err)

	// list existing migrations, should be empty
	list, err := mgr.List(context.Background())
	assert.Zero(s.T(), len(list))
	assert.Nil(s.T(), err)

	err = mgr.Run(context.Background(), src, migrations.DefaultProgressFn)
	assert.Nil(s.T(), err)

	list, err = mgr.List(context.Background())
	assert.Equal(s.T(), 2, len(list))
	assert.Nil(s.T(), err)

	assert.Equal(s.T(), "sample1.sql", list[0].Name)
	assert.Equal(s.T(), "sample2.sql", list[1].Name)
}
