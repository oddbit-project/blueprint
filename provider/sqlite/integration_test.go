package sqlite

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/doug-martin/goqu/v9"
	"github.com/oddbit-project/blueprint/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

const (
	sampleTable    = "sample_table"
	sampleTableDDL = `create table sample_table(id_sample_table INTEGER PRIMARY KEY AUTOINCREMENT, created_at DATETIME, label TEXT);`
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

// SQLiteIntegrationTestSuite provides comprehensive testing against a local SQLite database file
type SQLiteIntegrationTestSuite struct {
	suite.Suite
	client *db.SqlClient
	ctx    context.Context
	dsn    string
	dir    string
}

// getTestClient creates a client using the suite DSN
func (s *SQLiteIntegrationTestSuite) getTestClient() *db.SqlClient {
	cfg := NewClientConfig()
	cfg.DSN = s.dsn
	client, err := NewClient(cfg)
	require.NoError(s.T(), err, "Failed to create test client")
	return client
}

// SetupSuite prepares a temporary sqlite database shared across tests in the suite
func (s *SQLiteIntegrationTestSuite) SetupSuite() {
	s.ctx = context.Background()
	s.dir = s.T().TempDir()
	s.dsn = filepath.Join(s.dir, "test.db")

	s.T().Logf("SQLite database file: %s", s.dsn)

	config := NewClientConfig()
	config.DSN = s.dsn
	var err error
	s.client, err = NewClient(config)
	require.NoError(s.T(), err, "Failed to create SQLite client")

	err = s.client.Connect()
	require.NoError(s.T(), err, "Failed to connect to SQLite")
}

// SetupTest runs before each test to clean up any existing state
func (s *SQLiteIntegrationTestSuite) SetupTest() {
	if s.client != nil && s.client.Conn != nil {
		_, _ = s.client.Conn.ExecContext(s.ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s", MigrationTable))
	}
}

// TearDownSuite cleans up
func (s *SQLiteIntegrationTestSuite) TearDownSuite() {
	if s.client != nil && s.client.Conn != nil {
		_, _ = s.client.Conn.ExecContext(s.ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s", MigrationTable))
		s.client.Disconnect()
	}
}

// TestClientConfigValidate tests client configuration validation
func (s *SQLiteIntegrationTestSuite) TestClientConfigValidate() {
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

// TestRepository tests repository operations
func (s *SQLiteIntegrationTestSuite) TestRepository() {
	client := s.getTestClient()
	require.NoError(s.T(), client.Connect())
	defer client.Disconnect()

	s.dbCleanup(client)

	repo := db.NewRepository(s.ctx, client, sampleTable)
	s.testFunctions(repo)
}

// TestTransaction tests transaction operations
func (s *SQLiteIntegrationTestSuite) TestTransaction() {
	client := s.getTestClient()
	require.NoError(s.T(), client.Connect())
	defer client.Disconnect()

	s.dbCleanup(client)

	repo := db.NewRepository(s.ctx, client, sampleTable)

	tx, err := repo.NewTransaction(nil)
	require.NoError(s.T(), err)
	s.testFunctions(tx)
	require.NoError(s.T(), tx.Rollback())

	s.dbCleanup(client)

	tx, err = repo.NewTransaction(nil)
	require.NoError(s.T(), err)
	s.testFunctions(tx)
	require.NoError(s.T(), tx.Commit())
}

// dbCleanup helper method for cleaning up test database
func (s *SQLiteIntegrationTestSuite) dbCleanup(client *db.SqlClient) {
	_, err := client.Db().Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s", sampleTable))
	require.NoError(s.T(), err)
	_, err = client.Db().Exec(sampleTableDDL)
	require.NoError(s.T(), err)
}

// testFunctions tests all repository functionality
func (s *SQLiteIntegrationTestSuite) testFunctions(repo testRepository) {
	record := &sampleRecord{}
	err := repo.FetchOne(repo.SqlSelect(), record)
	assert.Error(s.T(), err)
	assert.True(s.T(), db.EmptyResult(err))

	newRecord1 := &sampleRecord{
		CreatedAt: time.Now(),
		Label:     "record 1",
	}
	require.NoError(s.T(), repo.Insert(newRecord1))

	err = repo.FetchOne(repo.SqlSelect(), record)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), newRecord1.Label, record.Label)
	assert.Equal(s.T(), newRecord1.CreatedAt.Unix(), record.CreatedAt.Unix())
	assert.True(s.T(), record.Id > 0)

	records := make([]*sampleRecord, 0)
	for i := 2; i < 10; i++ {
		row := &sampleRecord{
			CreatedAt: time.Now(),
			Label:     "record " + strconv.Itoa(i),
		}
		records = append(records, row)
	}
	require.NoError(s.T(), repo.Insert(records))

	records = make([]*sampleRecord, 0)
	require.NoError(s.T(), repo.Fetch(repo.SqlSelect(), &records))
	assert.Len(s.T(), records, 9)
	for i := 1; i < 9; i++ {
		assert.Equal(s.T(), "record "+strconv.Itoa(i), records[i-1].Label)
	}

	require.NoError(s.T(), repo.FetchRecord(map[string]any{"label": "record 3"}, record))
	assert.Equal(s.T(), "record 3", record.Label)

	records = make([]*sampleRecord, 0)
	require.NoError(s.T(), repo.FetchWhere(map[string]any{"label": "record 4"}, &records))
	assert.Equal(s.T(), "record 4", records[0].Label)

	exists := false
	exists, err = repo.Exists("label", "record 999")
	require.NoError(s.T(), err)
	assert.False(s.T(), exists)
	exists, err = repo.Exists("label", "record 8")
	require.NoError(s.T(), err)
	assert.True(s.T(), exists)

	exists, err = repo.Exists("label", "record 4", "id_sample_table", records[0].Id)
	require.NoError(s.T(), err)
	assert.False(s.T(), exists)

	require.NoError(s.T(), repo.Delete(repo.SqlDelete().Where(goqu.C("id_sample_table").Eq(4))))
	record.Label = ""
	err = repo.FetchRecord(map[string]any{"label": "record 4"}, record)
	assert.Error(s.T(), err)
	assert.True(s.T(), db.EmptyResult(err))

	require.NoError(s.T(), repo.DeleteWhere(map[string]any{"label": "record 3"}))
	err = repo.FetchRecord(map[string]any{"label": "record 3"}, record)
	assert.Error(s.T(), err)
	assert.True(s.T(), db.EmptyResult(err))

	record = &sampleRecord{
		CreatedAt: time.Now(),
		Label:     "something different",
	}
	require.NoError(s.T(), repo.InsertReturning(record, []string{"id_sample_table"}, &record.Id))
	assert.True(s.T(), record.Id > 0)

	record.Label = "foo"
	require.NoError(s.T(), repo.UpdateRecord(record, map[string]any{"label": "something different"}))
	require.NoError(s.T(), repo.FetchRecord(map[string]any{"label": "foo"}, record))

	require.NoError(s.T(), repo.UpdateFields(record, map[string]any{"label": "bar"}, map[string]any{"label": "foo"}))

	require.NoError(s.T(), repo.FetchRecord(map[string]any{"label": "bar"}, record))

	require.NoError(s.T(), repo.Exec(repo.SqlSelect().Where(goqu.C("label").Eq("bar"))))

	count, err := repo.Count()
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(8), count)

	count, err = repo.CountWhere(map[string]any{"label": "bar"})
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(1), count)
}

// TestSqliteIntegrationSuite runs the suite
func TestSqliteIntegrationSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	suite.Run(t, new(SQLiteIntegrationTestSuite))
}
