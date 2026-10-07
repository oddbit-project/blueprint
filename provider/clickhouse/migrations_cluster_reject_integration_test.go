package clickhouse

import (
	"fmt"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func (s *ClickhouseClusterMigrationTestSuite) TestIssue98_RejectsNodeLocalLog() {
	// a log created before WithCluster was set: TinyLog on node 1 only
	local, err := NewMigrationManager(s.ctx, s.node1, WithModule("issue98"))
	require.NoError(s.T(), err)
	r := s.record("local.sql", "SELECT 1")
	require.NoError(s.T(), local.RegisterMigration(s.ctx, r))

	_, err = NewMigrationManager(s.ctx, s.node1, WithCluster(testCluster), WithModule("issue98"))
	assert.ErrorIs(s.T(), err, ErrMigrationTableNotClusterLog)

	// the node-local log is left as it was
	assert.Equal(s.T(), "TinyLog", s.engine(s.node1))
	exists, err := local.MigrationExists(s.ctx, r.Name, r.SHA2)
	require.NoError(s.T(), err)
	assert.True(s.T(), exists, "node-local rows were changed")
}

func (s *ClickhouseClusterMigrationTestSuite) TestIssue98_RejectsLegacyLog() {
	// a pre-module log on node 1, with the applied migration recorded
	require.NoError(s.T(), s.node1.Conn.Exec(s.ctx, fmt.Sprintf(
		"CREATE TABLE %s (created DateTime, name String, sha2 String, contents String) ENGINE = TinyLog", MigrationTable)))
	r := s.record("legacy.sql", "SELECT 1")
	require.NoError(s.T(), s.node1.Conn.Exec(s.ctx, fmt.Sprintf(
		"INSERT INTO %s (created, name, sha2, contents) VALUES (now(), '%s', '%s', '%s')",
		MigrationTable, r.Name, r.SHA2, r.Contents)))

	_, err := NewMigrationManager(s.ctx, s.node1, WithCluster(testCluster))
	assert.ErrorIs(s.T(), err, ErrMigrationTableNotClusterLog)

	hasModule, err := ColumnExists(s.ctx, s.node1, "default", MigrationTable, "module")
	require.NoError(s.T(), err)
	assert.False(s.T(), hasModule, "the pre-module log was rebuilt")
}

func (s *ClickhouseClusterMigrationTestSuite) TestIssue98_RejectsLogReplicatedElsewhere() {
	// a replicated log made by hand under another Keeper path (one log per shard)
	require.NoError(s.T(), s.node1.Conn.Exec(s.ctx, fmt.Sprintf(
		"CREATE TABLE %s (created DateTime, module String, name String, sha2 String, contents String) "+
			"ENGINE = ReplicatedMergeTree('/clickhouse/tables/{shard}/default/%s', '{replica}') ORDER BY (module, name)",
		MigrationTable, MigrationTable)))

	_, err := NewMigrationManager(s.ctx, s.node1, WithCluster(testCluster))
	assert.ErrorIs(s.T(), err, ErrMigrationTableNotClusterLog)
}
