package clickhouse

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/oddbit-project/blueprint/db/migrations"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// Integration test struct for ClickHouse migrations
type ClickhouseMigrationTestSuite struct {
	suite.Suite
	client    *Client
	ctx       context.Context
	container testcontainers.Container
}

// SetupSuite prepares the test environment
func (s *ClickhouseMigrationTestSuite) SetupSuite() {
	// Create context
	s.ctx = context.Background()

	// Setup ClickHouse testcontainer
	req := testcontainers.ContainerRequest{
		Image:        "clickhouse/clickhouse-server:latest",
		ExposedPorts: []string{"9000/tcp", "8123/tcp"},
		Env: map[string]string{
			"CLICKHOUSE_USER":                      "default",
			"CLICKHOUSE_PASSWORD":                  "testpassword",
			"CLICKHOUSE_DB":                        "default",
			"CLICKHOUSE_DEFAULT_ACCESS_MANAGEMENT": "1",
		},
		WaitingFor: wait.ForAll(
			wait.ForListeningPort("9000/tcp"),
			wait.ForHTTP("/ping").WithPort("8123/tcp").WithStatusCodeMatcher(
				func(status int) bool {
					return status == http.StatusOK
				},
			),
		).WithStartupTimeout(60 * time.Second),
	}

	// Start container
	var err error
	s.container, err = testcontainers.GenericContainer(s.ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		s.T().Fatalf("Failed to start ClickHouse container: %v", err)
	}

	// Get container host and port
	host, err := s.container.Host(s.ctx)
	if err != nil {
		s.T().Fatalf("Failed to get container host: %v", err)
	}

	mappedPort, err := s.container.MappedPort(s.ctx, "9000")
	if err != nil {
		s.T().Fatalf("Failed to get mapped port: %v", err)
	}

	// Create client config
	config := NewClientConfig()
	config.Hosts = []string{host + ":" + mappedPort.Port()}
	config.Database = "default"
	config.Username = "default"
	config.Password = "testpassword"

	// Create client
	s.client, err = NewClient(config)
	if err != nil {
		s.T().Fatalf("Failed to create ClickHouse client: %v", err)
	}
}

// Teardown the test suite
func (s *ClickhouseMigrationTestSuite) TearDownSuite() {
	// Drop the test table
	if s.client != nil {
		err := s.client.Conn.Exec(s.ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s", MigrationTable))
		if err != nil {
			s.T().Logf("Failed to drop test table: %v", err)
		}
		// Close the client
		s.client.Close()
	}

	// Stop and remove container
	if s.container != nil {
		err := s.container.Terminate(s.ctx)
		if err != nil {
			s.T().Logf("Failed to terminate container: %v", err)
		}
	}
}

// Test basic query
func (s *ClickhouseMigrationTestSuite) TestMigrationManager() {
	src := migrations.NewMemorySource()

	src.Add("sample1.sql", "create table if not exists sample(id Int32) engine=TinyLog;")
	src.Add("sample2.sql", "insert into sample(id) values(1);")
	src.Add("sample3.sql", "drop table sample;")

	// create migration manager
	mgr, err := NewMigrationManager(context.Background(), s.client)
	assert.Nil(s.T(), err)

	// list existing migrations, should be empty
	list, err := mgr.List(context.Background())
	assert.Zero(s.T(), len(list))
	assert.Nil(s.T(), err)

	err = mgr.Run(context.Background(), src, migrations.DefaultProgressFn)
	assert.Nil(s.T(), err)

	list, err = mgr.List(context.Background())
	assert.Equal(s.T(), 3, len(list))
	assert.Nil(s.T(), err)

	assert.Equal(s.T(), "sample1.sql", list[0].Name)
	assert.Equal(s.T(), "sample2.sql", list[1].Name)
}

func (s *ClickhouseMigrationTestSuite) TestUpdateMigrations() {
	_ = s.client.Conn.Exec(s.ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s", MigrationTable))

	// create old version table
	qry := `CREATE TABLE IF NOT EXISTS  %s %s(created DateTime, name String, sha2 String, contents String) ENGINE = TinyLog`
	qry = fmt.Sprintf(qry, MigrationTable)
	_ = s.client.Conn.Exec(s.ctx, qry)

	src := migrations.NewMemorySource()
	src.Add("sample1.sql", "drop table if exists sample;")
	src.Add("sample2.sql", "create table sample(id Int32)  engine=TinyLog;")
	src.Add("sample3.sql", "drop table if exists sample;")

	// create new migration manager, should update table
	mgr, err := NewMigrationManager(context.Background(), s.client)
	assert.Nil(s.T(), err)

	// run migrations
	err = mgr.Run(context.Background(), src, migrations.DefaultProgressFn)
	assert.Nil(s.T(), err)

	list, err := mgr.List(context.Background())
	assert.Equal(s.T(), 3, len(list))
	assert.Nil(s.T(), err)

	assert.Equal(s.T(), "sample1.sql", list[0].Name)
	assert.Equal(s.T(), "sample2.sql", list[1].Name)
	assert.Equal(s.T(), "sample3.sql", list[2].Name)
}

func (s *ClickhouseMigrationTestSuite) TestModuleMigrations() {
	_ = s.client.Conn.Exec(s.ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s", MigrationTable))
	systemSrc := migrations.NewMemorySource()
	systemSrc.Add("sample1.sql", "drop table if exists sample;")
	systemSrc.Add("sample2.sql", "create table sample(id Int32)  engine=TinyLog;")
	systemSrc.Add("sample3.sql", "drop table if exists sample;")

	moduleSrc := migrations.NewMemorySource()
	moduleSrc.Add("module1.sql", "drop table if exists module;")
	moduleSrc.Add("module2.sql", "create table module(id Int32)  engine=TinyLog;")
	moduleSrc.Add("module3.sql", "drop table if exists module;")

	// migration manager - base module (default)
	sysMgr, err := NewMigrationManager(context.Background(), s.client)
	assert.Nil(s.T(), err)

	list, err := sysMgr.List(context.Background())
	assert.Zero(s.T(), len(list))
	assert.Nil(s.T(), err)

	err = sysMgr.Run(context.Background(), systemSrc, migrations.DefaultProgressFn)
	assert.Nil(s.T(), err)

	list, err = sysMgr.List(context.Background())
	assert.Equal(s.T(), 3, len(list))
	assert.Nil(s.T(), err)

	assert.Equal(s.T(), "sample1.sql", list[0].Name)
	assert.Equal(s.T(), migrations.ModuleBase, list[0].Module)
	assert.Equal(s.T(), "sample2.sql", list[1].Name)
	assert.Equal(s.T(), migrations.ModuleBase, list[1].Module)

	// migration manager - module
	moduleMgr, err := NewMigrationManager(context.Background(), s.client, WithModule("sample-module"))
	assert.Nil(s.T(), err)

	list, err = moduleMgr.List(context.Background())
	assert.Zero(s.T(), len(list))
	assert.Nil(s.T(), err)

	err = moduleMgr.Run(context.Background(), moduleSrc, migrations.DefaultProgressFn)
	assert.Nil(s.T(), err)

	list, err = moduleMgr.List(context.Background())
	assert.Equal(s.T(), 3, len(list))
	assert.Nil(s.T(), err)

	assert.Equal(s.T(), "module1.sql", list[0].Name)
	assert.Equal(s.T(), "sample-module", list[0].Module)
	assert.Equal(s.T(), "module2.sql", list[1].Name)
	assert.Equal(s.T(), "sample-module", list[1].Module)
	assert.Equal(s.T(), "module3.sql", list[2].Name)
	assert.Equal(s.T(), "sample-module", list[2].Module)
}

func (s *ClickhouseMigrationTestSuite) TestSameNameMigrations() {
	_ = s.client.Conn.Exec(s.ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s", MigrationTable))
	systemSrc := migrations.NewMemorySource()
	systemSrc.Add("sample1.sql", "select 1;")

	moduleSrc := migrations.NewMemorySource()
	moduleSrc.Add("sample1.sql", "select 1-2;")

	// migration manager - base module (default)
	sysMgr, err := NewMigrationManager(context.Background(), s.client)
	assert.Nil(s.T(), err)

	// run migrations
	err = sysMgr.Run(context.Background(), systemSrc, migrations.DefaultProgressFn)
	assert.Nil(s.T(), err)

	// list, should have 1
	sysList, err := sysMgr.List(context.Background())
	assert.Equal(s.T(), 1, len(sysList))
	assert.Nil(s.T(), err)

	// run migrations again, should do nothing
	err = sysMgr.Run(context.Background(), systemSrc, migrations.DefaultProgressFn)
	assert.Nil(s.T(), err)

	// list, should still have 1
	sysList, err = sysMgr.List(context.Background())
	assert.Equal(s.T(), 1, len(sysList))

	// migration manager - module
	moduleMgr, err := NewMigrationManager(context.Background(), s.client, WithModule("sample-module"))
	assert.Nil(s.T(), err)

	moduleList, err := moduleMgr.List(context.Background())
	assert.Zero(s.T(), len(moduleList))
	assert.Nil(s.T(), err)

	err = moduleMgr.Run(context.Background(), moduleSrc, migrations.DefaultProgressFn)
	assert.Nil(s.T(), err)

	// module should have 1
	moduleList, err = moduleMgr.List(context.Background())
	assert.Equal(s.T(), 1, len(moduleList))
	assert.Nil(s.T(), err)

	assert.NotEqual(s.T(), sysList[0].SHA2, moduleList[0].SHA2)
	assert.NotEqual(s.T(), sysList[0].Module, moduleList[0].Module)
}

// Rows of a pre-module migration table belong to the base module; an empty
// module would be invisible to List() and every migration would re-run.
func (s *ClickhouseMigrationTestSuite) TestUpdateMigrationsKeepsLegacyRows() {
	_ = s.client.Conn.Exec(s.ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s", MigrationTable))
	_ = s.client.Conn.Exec(s.ctx, "DROP TABLE IF EXISTS legacy_sample")
	defer func() {
		_ = s.client.Conn.Exec(s.ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s", MigrationTable))
		_ = s.client.Conn.Exec(s.ctx, "DROP TABLE IF EXISTS legacy_sample")
	}()

	qry := `CREATE TABLE IF NOT EXISTS %s (created DateTime, name String, sha2 String, contents String) ENGINE = TinyLog`
	require.NoError(s.T(), s.client.Conn.Exec(s.ctx, fmt.Sprintf(qry, MigrationTable)))

	src := migrations.NewMemorySource()
	src.Add("sample1.sql", "create table legacy_sample(id Int32) engine=TinyLog;")
	record, err := src.Read("sample1.sql")
	require.NoError(s.T(), err)

	// the migration as an old version would have recorded it
	require.NoError(s.T(), s.client.Conn.Exec(s.ctx, fmt.Sprintf(
		"INSERT INTO %s (created, name, sha2, contents) VALUES (now(), '%s', '%s', '%s')",
		MigrationTable, record.Name, record.SHA2, record.Contents)))

	mgr, err := NewMigrationManager(context.Background(), s.client)
	require.NoError(s.T(), err)

	list, err := mgr.List(context.Background())
	require.NoError(s.T(), err)
	require.Equal(s.T(), 1, len(list))
	assert.Equal(s.T(), migrations.ModuleBase, list[0].Module)

	// already applied: running again must not re-execute it
	require.NoError(s.T(), mgr.Run(context.Background(), src, migrations.DefaultProgressFn))

	exists, err := TableExists(s.ctx, s.client, "default", "legacy_sample")
	require.NoError(s.T(), err)
	assert.False(s.T(), exists)
}

func (s *ClickhouseMigrationTestSuite) TestMigrationExists() {
	_ = s.client.Conn.Exec(s.ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s", MigrationTable))
	defer func() {
		_ = s.client.Conn.Exec(s.ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s", MigrationTable))
	}()

	src := migrations.NewMemorySource()
	src.Add("sample1.sql", "select 1;")

	mgr, err := NewMigrationManager(context.Background(), s.client)
	require.NoError(s.T(), err)
	require.NoError(s.T(), mgr.Run(context.Background(), src, migrations.DefaultProgressFn))

	applied, err := mgr.List(context.Background())
	require.NoError(s.T(), err)
	require.Equal(s.T(), 1, len(applied))

	exists, err := mgr.MigrationExists(context.Background(), "absent.sql", applied[0].SHA2)
	assert.NoError(s.T(), err)
	assert.False(s.T(), exists)

	exists, err = mgr.MigrationExists(context.Background(), "sample1.sql", applied[0].SHA2)
	assert.NoError(s.T(), err)
	assert.True(s.T(), exists)

	exists, err = mgr.MigrationExists(context.Background(), "sample1.sql", "not-the-recorded-hash")
	assert.ErrorIs(s.T(), err, migrations.ErrMigrationNameHashMismatch)
	assert.True(s.T(), exists)

	record, err := src.Read("sample1.sql")
	require.NoError(s.T(), err)
	assert.ErrorIs(s.T(), mgr.RunMigration(context.Background(), record), migrations.ErrMigrationExists)
}

// A failed lookup of the applied set must abort the run: treating it as "nothing
// applied" re-executes every migration.
func (s *ClickhouseMigrationTestSuite) TestRunFailsWhenAppliedListIsUnreadable() {
	_ = s.client.Conn.Exec(s.ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s", MigrationTable))
	_ = s.client.Conn.Exec(s.ctx, "DROP TABLE IF EXISTS unreadable_list")
	defer func() {
		_ = s.client.Conn.Exec(s.ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s", MigrationTable))
		_ = s.client.Conn.Exec(s.ctx, "DROP TABLE IF EXISTS unreadable_list")
	}()

	src := migrations.NewMemorySource()
	src.Add("sample1.sql", "create table unreadable_list(id Int32) engine=TinyLog;")

	mgr, err := NewMigrationManager(context.Background(), s.client)
	require.NoError(s.T(), err)

	// remove the migration table under the manager
	require.NoError(s.T(), s.client.Conn.Exec(s.ctx, fmt.Sprintf("DROP TABLE %s", MigrationTable)))

	assert.Error(s.T(), mgr.Run(context.Background(), src, migrations.DefaultProgressFn))

	exists, err := TableExists(s.ctx, s.client, "default", "unreadable_list")
	require.NoError(s.T(), err)
	assert.False(s.T(), exists)
}
