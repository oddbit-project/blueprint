package clickhouse

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/oddbit-project/blueprint/db/migrations"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"
)

const testCluster = "c1"

// clusterNodeConfig is the config.d file for one node of a two-shard cluster;
// Keeper runs embedded in ch1, and ch2 uses it
const clusterNodeConfig = `<clickhouse>
    <interserver_http_host>%[1]s</interserver_http_host>
    <macros>
        <shard>%[2]s</shard>
        <replica>r1</replica>
    </macros>
    <remote_servers>
        <c1>
            <shard><replica><host>ch1</host><port>9000</port></replica></shard>
            <shard><replica><host>ch2</host><port>9000</port></replica></shard>
        </c1>
        <c-1>
            <shard><replica><host>ch1</host><port>9000</port></replica></shard>
            <shard><replica><host>ch2</host><port>9000</port></replica></shard>
        </c-1>
    </remote_servers>
    <zookeeper>
        <node><host>ch1</host><port>9181</port></node>
    </zookeeper>
    <distributed_ddl>
        <path>/clickhouse/task_queue/ddl</path>
    </distributed_ddl>
    %[3]s
</clickhouse>`

const embeddedKeeperConfig = `<keeper_server>
        <tcp_port>9181</tcp_port>
        <server_id>1</server_id>
        <log_storage_path>/var/lib/clickhouse/coordination/log</log_storage_path>
        <snapshot_storage_path>/var/lib/clickhouse/coordination/snapshots</snapshot_storage_path>
        <raft_configuration>
            <server><id>1</id><hostname>ch1</hostname><port>9234</port></server>
        </raft_configuration>
    </keeper_server>`

// ClickhouseClusterMigrationTestSuite runs the migration manager against a
// two-node cluster (two shards of one replica each)
type ClickhouseClusterMigrationTestSuite struct {
	suite.Suite
	ctx        context.Context
	net        *testcontainers.DockerNetwork
	containers []testcontainers.Container
	node1      *Client
	node2      *Client
}

func (s *ClickhouseClusterMigrationTestSuite) startNode(alias, shard, keeper string) *Client {
	ctr, err := testcontainers.Run(s.ctx, "clickhouse/clickhouse-server:latest",
		testcontainers.WithExposedPorts("9000/tcp", "8123/tcp"),
		testcontainers.WithEnv(map[string]string{
			"CLICKHOUSE_USER":                      "default",
			"CLICKHOUSE_PASSWORD":                  "testpassword",
			"CLICKHOUSE_DB":                        "default",
			"CLICKHOUSE_DEFAULT_ACCESS_MANAGEMENT": "1",
		}),
		testcontainers.WithFiles(testcontainers.ContainerFile{
			Reader:            strings.NewReader(fmt.Sprintf(clusterNodeConfig, alias, shard, keeper)),
			ContainerFilePath: "/etc/clickhouse-server/config.d/cluster.xml",
			FileMode:          0o644,
		}),
		network.WithNetwork([]string{alias}, s.net),
		testcontainers.WithWaitStrategy(wait.ForAll(
			wait.ForListeningPort("9000/tcp"),
			wait.ForHTTP("/ping").WithPort("8123/tcp").WithStatusCodeMatcher(func(status int) bool {
				return status == http.StatusOK
			}),
		).WithDeadline(90*time.Second)),
	)
	if ctr != nil {
		s.containers = append(s.containers, ctr)
	}
	require.NoError(s.T(), err, "start %s", alias)

	host, err := ctr.Host(s.ctx)
	require.NoError(s.T(), err)
	port, err := ctr.MappedPort(s.ctx, "9000")
	require.NoError(s.T(), err)

	config := NewClientConfig()
	config.Hosts = []string{host + ":" + port.Port()}
	config.Database = "default"
	config.Username = "default"
	config.Password = "testpassword"
	client, err := NewClient(config)
	require.NoError(s.T(), err, "connect %s", alias)
	return client
}

func (s *ClickhouseClusterMigrationTestSuite) SetupSuite() {
	s.ctx = context.Background()
	var err error
	s.net, err = network.New(s.ctx)
	require.NoError(s.T(), err)

	s.node1 = s.startNode("ch1", "01", embeddedKeeperConfig)
	s.node2 = s.startNode("ch2", "02", "")

	// both nodes must reach Keeper before ON CLUSTER DDL can run
	for _, c := range []*Client{s.node1, s.node2} {
		require.Eventually(s.T(), func() bool {
			var n uint64
			return c.Conn.QueryRow(s.ctx, "SELECT count() FROM system.zookeeper WHERE path = '/'").Scan(&n) == nil
		}, 60*time.Second, time.Second, "keeper not reachable")
	}
}

func (s *ClickhouseClusterMigrationTestSuite) TearDownSuite() {
	for _, c := range []*Client{s.node1, s.node2} {
		if c != nil {
			_ = c.Close()
		}
	}
	for _, ctr := range s.containers {
		_ = ctr.Terminate(s.ctx)
	}
	if s.net != nil {
		_ = s.net.Remove(s.ctx)
	}
}

func (s *ClickhouseClusterMigrationTestSuite) SetupTest() {
	for _, tbl := range []string{MigrationTable, MigrationTable + "_new", "issue98_sample"} {
		require.NoError(s.T(), s.node1.Conn.Exec(s.ctx,
			fmt.Sprintf("DROP TABLE IF EXISTS %s ON CLUSTER %s SYNC", tbl, testCluster)))
	}
}

func (s *ClickhouseClusterMigrationTestSuite) engine(c *Client) string {
	var engine string
	err := c.Conn.QueryRow(s.ctx,
		"SELECT engine FROM system.tables WHERE database = currentDatabase() AND name = ?", MigrationTable).Scan(&engine)
	if err != nil {
		return ""
	}
	return engine
}

func (s *ClickhouseClusterMigrationTestSuite) tableUUID(c *Client) string {
	var id string
	require.NoError(s.T(), c.Conn.QueryRow(s.ctx,
		"SELECT toString(uuid) FROM system.tables WHERE database = currentDatabase() AND name = ?", MigrationTable).Scan(&id))
	return id
}

// syncLog waits until node2 has fetched every part of the replicated log
func (s *ClickhouseClusterMigrationTestSuite) syncLog() {
	require.NoError(s.T(), s.node2.Conn.Exec(s.ctx, "SYSTEM SYNC REPLICA "+MigrationTable))
}

func (s *ClickhouseClusterMigrationTestSuite) record(name, contents string) *migrations.MigrationRecord {
	src := migrations.NewMemorySource()
	src.Add(name, contents)
	r, err := src.Read(name)
	require.NoError(s.T(), err)
	return r
}

func (s *ClickhouseClusterMigrationTestSuite) TestIssue98_InitOnCluster() {
	_, err := NewMigrationManager(s.ctx, s.node1, WithCluster(testCluster))
	require.NoError(s.T(), err)

	for i, c := range []*Client{s.node1, s.node2} {
		exists, err := TableExists(s.ctx, c, "default", MigrationTable)
		require.NoError(s.T(), err)
		assert.True(s.T(), exists, "migration log missing on node %d", i+1)
	}
}

func (s *ClickhouseClusterMigrationTestSuite) TestIssue98_LogSharedAcrossNodes() {
	mgr1, err := NewMigrationManager(s.ctx, s.node1, WithCluster(testCluster))
	require.NoError(s.T(), err)

	src := migrations.NewMemorySource()
	src.Add("issue98_1.sql", "CREATE TABLE issue98_sample ON CLUSTER c1 (id Int32) ENGINE = TinyLog")
	require.NoError(s.T(), mgr1.Run(s.ctx, src, migrations.DefaultProgressFn))
	s.syncLog()

	for i, c := range []*Client{s.node1, s.node2} {
		assert.True(s.T(), strings.HasPrefix(s.engine(c), "Replicated"), "node %d engine %q", i+1, s.engine(c))
	}

	mgr2, err := NewMigrationManager(s.ctx, s.node2, WithCluster(testCluster))
	require.NoError(s.T(), err)
	list, err := mgr2.List(s.ctx)
	require.NoError(s.T(), err)
	require.Len(s.T(), list, 1, "node 2 does not see the migration run through node 1")
	assert.Equal(s.T(), "issue98_1.sql", list[0].Name)

	ran := make([]string, 0)
	require.NoError(s.T(), mgr2.Run(s.ctx, src, func(msgType int, name string, e error) {
		if msgType == migrations.MsgRunMigration {
			ran = append(ran, name)
		}
	}))
	assert.Empty(s.T(), ran, "node 2 re-ran a migration applied through node 1")
}

func (s *ClickhouseClusterMigrationTestSuite) TestIssue98_NoClusterStaysLocal() {
	_, err := NewMigrationManager(s.ctx, s.node1)
	require.NoError(s.T(), err)

	assert.Equal(s.T(), "TinyLog", s.engine(s.node1))
	exists, err := TableExists(s.ctx, s.node2, "default", MigrationTable)
	require.NoError(s.T(), err)
	assert.False(s.T(), exists, "log created on node 2 without a cluster")
}

func (s *ClickhouseClusterMigrationTestSuite) TestIssue98_ReplicatedLogNotRebuilt() {
	_, err := NewMigrationManager(s.ctx, s.node1, WithCluster(testCluster))
	require.NoError(s.T(), err)
	before := s.tableUUID(s.node1)

	_, err = NewMigrationManager(s.ctx, s.node1, WithCluster(testCluster))
	require.NoError(s.T(), err)
	assert.Equal(s.T(), before, s.tableUUID(s.node1), "an already replicated log was rebuilt")
}

func (s *ClickhouseClusterMigrationTestSuite) TestIssue98_NoClusterLogNotRebuilt() {
	_, err := NewMigrationManager(s.ctx, s.node1)
	require.NoError(s.T(), err)
	before := s.tableUUID(s.node1)

	_, err = NewMigrationManager(s.ctx, s.node1)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), before, s.tableUUID(s.node1), "an existing non-cluster log was rebuilt")
}

func (s *ClickhouseClusterMigrationTestSuite) TestIssue98_NodeWithoutLogJoinsExistingLog() {
	mgr1, err := NewMigrationManager(s.ctx, s.node1, WithCluster(testCluster))
	require.NoError(s.T(), err)
	r := s.record("join.sql", "SELECT 1")
	require.NoError(s.T(), mgr1.RegisterMigration(s.ctx, r))

	// node 2 has no copy, as a node added after the log was created
	require.NoError(s.T(), s.node2.Conn.Exec(s.ctx, "DROP TABLE "+MigrationTable+" SYNC"))

	mgr2, err := NewMigrationManager(s.ctx, s.node2, WithCluster(testCluster))
	require.NoError(s.T(), err)
	exists, err := mgr2.MigrationExists(s.ctx, r.Name, r.SHA2)
	require.NoError(s.T(), err)
	assert.True(s.T(), exists, "node 2 started a separate log instead of joining the existing one")
}

func (s *ClickhouseClusterMigrationTestSuite) TestIssue98_ReadsWaitForReplication() {
	mgr1, err := NewMigrationManager(s.ctx, s.node1, WithCluster(testCluster))
	require.NoError(s.T(), err)
	mgr2, err := NewMigrationManager(s.ctx, s.node2, WithCluster(testCluster))
	require.NoError(s.T(), err)

	// hold node 2's copy behind node 1 until after the read has started
	require.NoError(s.T(), s.node2.Conn.Exec(s.ctx, "SYSTEM STOP FETCHES "+MigrationTable))
	defer func() { _ = s.node2.Conn.Exec(s.ctx, "SYSTEM START FETCHES "+MigrationTable) }()

	r := s.record("lag.sql", "SELECT 1")
	require.NoError(s.T(), mgr1.RegisterMigration(s.ctx, r))
	time.AfterFunc(2*time.Second, func() {
		_ = s.node2.Conn.Exec(s.ctx, "SYSTEM START FETCHES "+MigrationTable)
	})

	exists, err := mgr2.MigrationExists(s.ctx, r.Name, r.SHA2)
	require.NoError(s.T(), err)
	assert.True(s.T(), exists, "node 2 read the log before it had replicated")
}

func (s *ClickhouseClusterMigrationTestSuite) TestIssue98_ClusterNameNeedsQuoting() {
	_, err := NewMigrationManager(s.ctx, s.node1, WithCluster("c-1"))
	require.NoError(s.T(), err)

	exists, err := TableExists(s.ctx, s.node2, "default", MigrationTable)
	require.NoError(s.T(), err)
	assert.True(s.T(), exists, "migration log missing on node 2")
}

func TestClickhouseClusterMigrations(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	suite.Run(t, new(ClickhouseClusterMigrationTestSuite))
}
