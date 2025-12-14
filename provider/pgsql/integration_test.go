package pgsql

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/doug-martin/goqu/v9"
	"github.com/oddbit-project/blueprint/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	sampleTable    = "sample_table"
	sampleTableDDL = `create table sample_table(id_sample_table serial not null primary key, created_at timestamp with time zone, label text);`
)

type sampleRecord struct {
	Id        int       `db:"id_sample_table,auto" goqu:"skipinsert"`
	CreatedAt time.Time `db:"created_at"`
	Label     string
}

type testRepository interface {
	db.Builder
	db.Reader
	db.Executor
	db.Writer
	db.Deleter
	db.Updater
	db.Counter
}

// PGIntegrationTestSuite manages the PostgreSQL testcontainer and provides comprehensive testing
type PGIntegrationTestSuite struct {
	suite.Suite
	client     *db.SqlClient
	ctx        context.Context
	container  testcontainers.Container
	pgInstance *postgres.PostgresContainer
	dsn        string
}

// getTestClient creates a client using the testcontainer DSN
func (s *PGIntegrationTestSuite) getTestClient() *db.SqlClient {
	cfg := NewClientConfig()
	cfg.DSN = s.dsn
	client, err := NewClient(cfg)
	require.NoError(s.T(), err, "Failed to create test client")
	return client
}

// Legacy function for compatibility with existing standalone tests
func resolveDSN() string {
	// Disable prepared statement cache to avoid "cached plan must not change result type" errors
	// Use default_query_exec_mode=simple_protocol for pgx driver
	return "postgres://blueprint:password@postgres:5432/blueprint?default_query_exec_mode=simple_protocol"
}

// Legacy function for compatibility with existing standalone tests
func dbClient(t *testing.T) *db.SqlClient {
	cfg := NewClientConfig()
	cfg.DSN = resolveDSN()
	client, err := NewClient(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return client
}

// SetupSuite prepares the test environment with testcontainers
func (s *PGIntegrationTestSuite) SetupSuite() {
	s.ctx = context.Background()

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
	config := NewClientConfig()
	config.DSN = s.dsn
	s.client, err = NewClient(config)
	require.NoError(s.T(), err, "Failed to create PostgreSQL client")

	err = s.client.Connect()
	require.NoError(s.T(), err, "Failed to connect to PostgreSQL")
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
	// Drop test artifacts
	if s.client != nil && s.client.Conn != nil {
		_, err := s.client.Conn.ExecContext(s.ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s", MigrationTable))
		if err != nil {
			s.T().Logf("Failed to drop test table: %v", err)
		}
		_, _ = s.client.Conn.ExecContext(s.ctx, "DROP SCHEMA IF EXISTS blueprint CASCADE")
		s.client.Disconnect()
	}

	// Terminate container
	if s.container != nil {
		err := s.container.Terminate(s.ctx)
		if err != nil {
			s.T().Logf("Failed to terminate PostgreSQL container: %v", err)
		}
	}
}

// TestClientConfigValidate tests client configuration validation
func (s *PGIntegrationTestSuite) TestClientConfigValidate() {
	defaultCfg := NewClientConfig()
	defaultCfg.DSN = s.dsn

	testCases := []struct {
		name     string
		cfg      *ClientConfig
		expected error
	}{
		{
			name:     "Empty Config",
			cfg:      &ClientConfig{},
			expected: ErrEmptyDSN,
		},
		{
			name:     "Default Config",
			cfg:      defaultCfg,
			expected: nil,
		},
		{
			name: "Non-empty DSN",
			cfg: &ClientConfig{
				DSN:          defaultCfg.DSN,
				MaxIdleConns: DefaultIdleConns,
				MaxOpenConns: DefaultMaxConns,
				ConnLifetime: DefaultConnLifeTimeSecond,
				ConnIdleTime: DefaultConnIdleTimeSecond,
			},
			expected: nil,
		},
	}

	for _, tc := range testCases {
		s.T().Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if !errors.Is(err, tc.expected) {
				t.Errorf("expected %v, got %v", tc.expected, err)
			}
			if err == nil {
				client, err := NewClient(tc.cfg)
				assert.NotNil(t, client)
				assert.Nil(t, err)
			}
		})
	}
}

// TestLockMultipleConnections tests advisory locks with multiple connections
func (s *PGIntegrationTestSuite) TestLockMultipleConnections() {
	client := s.getTestClient()
	require.NoError(s.T(), client.Connect())
	defer client.Disconnect()

	conn1 := client.Db()
	conn2 := client.Db()

	lockId := 12
	lock1, err := NewAdvisoryLock(s.ctx, conn1, lockId)
	require.NoError(s.T(), err)
	lock2, err := NewAdvisoryLock(s.ctx, conn2, lockId)
	require.NoError(s.T(), err)

	// lock using conn1
	require.NoError(s.T(), lock1.Lock(s.ctx))

	// attempt re-lock with conn2, should fail
	locked, err := lock2.TryLock(s.ctx)
	require.NoError(s.T(), err)
	assert.False(s.T(), locked)

	// unlock using conn1, should work
	require.NoError(s.T(), lock1.Unlock(s.ctx))

	//  try lock with conn2, should work
	locked, err = lock2.TryLock(s.ctx)
	require.NoError(s.T(), err)
	assert.True(s.T(), locked)

	// attempt re-lock with conn1, should fail
	locked, err = lock1.TryLock(s.ctx)
	require.NoError(s.T(), err)
	assert.False(s.T(), locked)

	// unlock using conn2, should work
	require.NoError(s.T(), lock2.Unlock(s.ctx))
}

// TestLockConcurrent tests concurrent advisory lock operations
func (s *PGIntegrationTestSuite) TestLockConcurrent() {
	client := s.getTestClient()
	require.NoError(s.T(), client.Connect())
	defer client.Disconnect()

	conn1 := client.Db()
	conn2 := client.Db()

	lockId := 27
	lock1, err := NewAdvisoryLock(s.ctx, conn1, lockId)
	require.NoError(s.T(), err)
	lock2, err := NewAdvisoryLock(s.ctx, conn2, lockId)
	require.NoError(s.T(), err)

	wg := sync.WaitGroup{}
	wg.Add(1)
	require.NoError(s.T(), lock1.Lock(s.ctx))
	time.AfterFunc(time.Second*1, func() {
		lock1.Unlock(s.ctx)
		wg.Done()
	})

	// should not work
	locked, err := lock2.TryLock(s.ctx)
	require.NoError(s.T(), err)
	assert.False(s.T(), locked)

	wg.Wait()

	// now conn2 can acquire lock
	locked, err = lock2.TryLock(s.ctx)
	require.NoError(s.T(), err)
	assert.True(s.T(), locked)

	// and conn1 cannot
	locked, err = lock1.TryLock(s.ctx)
	require.NoError(s.T(), err)
	assert.False(s.T(), locked)

	// unlock everything
	require.NoError(s.T(), lock2.Unlock(s.ctx))
}

// TestLockUnlock tests basic lock/unlock operations
func (s *PGIntegrationTestSuite) TestLockUnlock() {
	client := s.getTestClient()
	require.NoError(s.T(), client.Connect())
	defer client.Disconnect()

	lockId := 10
	lock, err := NewAdvisoryLock(s.ctx, client.Db(), lockId)
	require.NoError(s.T(), err)

	// lock
	require.NoError(s.T(), lock.Lock(s.ctx))

	// unlock
	require.NoError(s.T(), lock.Unlock(s.ctx))

	// attempt re-lock again
	locked, err := lock.TryLock(s.ctx)
	require.NoError(s.T(), err)
	assert.True(s.T(), locked) // should succeed

	// finally, unlock
	require.NoError(s.T(), lock.Unlock(s.ctx))
}

// TestRepository tests repository operations using testcontainers
func (s *PGIntegrationTestSuite) TestRepository() {
	client := s.getTestClient()
	require.NoError(s.T(), client.Connect())
	defer client.Disconnect()

	s.dbCleanup(client)

	repo := db.NewRepository(s.ctx, client, sampleTable)
	s.testFunctions(repo)
}

// TestTransaction tests transaction operations using testcontainers
func (s *PGIntegrationTestSuite) TestTransaction() {
	client := s.getTestClient()
	require.NoError(s.T(), client.Connect())
	defer client.Disconnect()

	s.dbCleanup(client)

	repo := db.NewRepository(s.ctx, client, sampleTable)

	// test transaction with rollback
	tx, err := repo.NewTransaction(nil)
	require.NoError(s.T(), err)
	s.testFunctions(tx)
	require.NoError(s.T(), tx.Rollback())

	s.dbCleanup(client)

	// test transaction with commit
	tx, err = repo.NewTransaction(nil)
	require.NoError(s.T(), err)
	s.testFunctions(tx)
	require.NoError(s.T(), tx.Commit())
}

// dbCleanup helper method for cleaning up test database
func (s *PGIntegrationTestSuite) dbCleanup(client *db.SqlClient) {
	_, err := client.Db().Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s", sampleTable))
	require.NoError(s.T(), err)
	_, err = client.Db().Exec(sampleTableDDL)
	require.NoError(s.T(), err)
}

// testFunctions tests all repository functionality
func (s *PGIntegrationTestSuite) testFunctions(repo testRepository) {
	// Read non-existing record
	record := &sampleRecord{}
	err := repo.FetchOne(repo.SqlSelect(), record)
	assert.Error(s.T(), err)
	assert.True(s.T(), db.EmptyResult(err))

	newRecord1 := &sampleRecord{
		CreatedAt: time.Now(),
		Label:     "record 1",
	}
	// insert single record
	require.NoError(s.T(), repo.Insert(newRecord1))

	// read single existing record
	err = repo.FetchOne(repo.SqlSelect(), record)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), newRecord1.Label, record.Label)
	assert.Equal(s.T(), newRecord1.CreatedAt.Unix(), record.CreatedAt.Unix())
	assert.True(s.T(), record.Id > 0)

	// add several records
	records := make([]*sampleRecord, 0)
	for i := 2; i < 10; i++ {
		row := &sampleRecord{
			CreatedAt: time.Now(),
			Label:     "record " + strconv.Itoa(i),
		}
		records = append(records, row)
	}
	// insert multiple records
	// we need to convert to []any
	require.NoError(s.T(), repo.Insert(records))

	// read multiple records
	records = make([]*sampleRecord, 0)
	require.NoError(s.T(), repo.Fetch(repo.SqlSelect(), &records))
	assert.Len(s.T(), records, 9)
	// check that labels were read
	for i := 1; i < 9; i++ {
		assert.Equal(s.T(), "record "+strconv.Itoa(i), records[i-1].Label)
	}

	// read record number 3
	require.NoError(s.T(), repo.FetchRecord(map[string]any{"label": "record 3"}, record))
	assert.Equal(s.T(), "record 3", record.Label)

	// read record number 4, but to a slice
	records = make([]*sampleRecord, 0)
	require.NoError(s.T(), repo.FetchWhere(map[string]any{"label": "record 4"}, &records))
	assert.Equal(s.T(), "record 4", records[0].Label)

	// use exists
	exists := false
	// non-existing record
	exists, err = repo.Exists("label", "record 999")
	require.NoError(s.T(), err)
	assert.False(s.T(), exists)
	// existing record
	exists, err = repo.Exists("label", "record 8")
	require.NoError(s.T(), err)
	assert.True(s.T(), exists)

	// existing record with skip value
	exists, err = repo.Exists("label", "record 4", "id_sample_table", records[0].Id)
	require.NoError(s.T(), err)
	assert.False(s.T(), exists)

	// delete record number 4
	require.NoError(s.T(), repo.Delete(repo.SqlDelete().Where(goqu.C("id_sample_table").Eq(4))))
	// try to read deleted item, should fail
	record.Label = ""
	err = repo.FetchRecord(map[string]any{"label": "record 4"}, record)
	assert.Error(s.T(), err)
	assert.True(s.T(), db.EmptyResult(err))

	// delete record number 3
	require.NoError(s.T(), repo.DeleteWhere(map[string]any{"label": "record 3"}))
	// record number 3 deleted, should fail
	err = repo.FetchRecord(map[string]any{"label": "record 3"}, record)
	assert.Error(s.T(), err)
	assert.True(s.T(), db.EmptyResult(err))

	// insert returning
	record = &sampleRecord{
		CreatedAt: time.Now(),
		Label:     "something different",
	}
	require.NoError(s.T(), repo.InsertReturning(record, []string{"id_sample_table"}, &record.Id))
	assert.True(s.T(), record.Id > 0)

	// update last insert using whole record
	record.Label = "foo"
	require.NoError(s.T(), repo.UpdateRecord(record, map[string]any{"label": "something different"}))
	// re-fetch using new label
	require.NoError(s.T(), repo.FetchRecord(map[string]any{"label": "foo"}, record))

	// update last insert using just label
	require.NoError(s.T(), repo.UpdateFields(record, map[string]any{"label": "bar"}, map[string]any{"label": "foo"}))

	// re-fetch using new label
	require.NoError(s.T(), repo.FetchRecord(map[string]any{"label": "bar"}, record))

	// select via exec
	require.NoError(s.T(), repo.Exec(repo.SqlSelect().Where(goqu.C("label").Eq("bar"))))

	// count records
	count, err := repo.Count()
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(8), count)

	// count records with where
	count, err = repo.CountWhere(map[string]any{"label": "bar"})
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(1), count)
}

// Run the test suite
func TestPgIntegrationSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	suite.Run(t, new(PGIntegrationTestSuite))
}
