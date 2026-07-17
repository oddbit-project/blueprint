package franz

import (
	"context"
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kgo"
)

// TestCommitMarks covers the pure offset-map construction used by
// MarkCommitOffsets: highest offset per partition, plus one, epoch preserved.
func TestCommitMarks(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		assert.Empty(t, commitMarks(nil))
	})

	t.Run("single record marks offset+1 with epoch", func(t *testing.T) {
		m := commitMarks([]ConsumedRecord{{Topic: "t", Partition: 0, Offset: 5, LeaderEpoch: 2}})
		assert.Equal(t, kgo.EpochOffset{Epoch: 2, Offset: 6}, m["t"][0])
	})

	t.Run("highest offset per partition wins (out of order)", func(t *testing.T) {
		m := commitMarks([]ConsumedRecord{
			{Topic: "t", Partition: 0, Offset: 10},
			{Topic: "t", Partition: 0, Offset: 3},
			{Topic: "t", Partition: 0, Offset: 7},
		})
		assert.Equal(t, int64(11), m["t"][0].Offset)
	})

	t.Run("separate topics and partitions", func(t *testing.T) {
		m := commitMarks([]ConsumedRecord{
			{Topic: "a", Partition: 0, Offset: 1},
			{Topic: "a", Partition: 1, Offset: 4},
			{Topic: "b", Partition: 0, Offset: 2},
		})
		assert.Equal(t, int64(2), m["a"][0].Offset)
		assert.Equal(t, int64(5), m["a"][1].Offset)
		assert.Equal(t, int64(3), m["b"][0].Offset)
	})
}

// TestMarkCommitOffsets_Integration verifies the end-to-end mark-then-autocommit
// contract against a real broker: after marking, the background committer advances
// the group offset so a fresh member of the same group sees no redelivery. Skips
// if no broker is reachable (set KAFKA_BROKER, default localhost:9092).
func TestMarkCommitOffsets_Integration(t *testing.T) {
	broker := os.Getenv("KAFKA_BROKER")
	if broker == "" {
		broker = "localhost:9092"
	}
	conn, err := net.DialTimeout("tcp", broker, 2*time.Second)
	if err != nil {
		t.Skipf("no kafka broker at %s: %v", broker, err)
	}
	conn.Close()

	topic := fmt.Sprintf("marks-it-%d", time.Now().UnixNano())
	group := fmt.Sprintf("marks-it-group-%d", time.Now().UnixNano())

	// Produce 5 records (topic auto-created).
	prod, err := NewProducer(&ProducerConfig{
		BaseConfig: BaseConfig{Brokers: broker, AuthType: AuthTypeNone},
		Acks:       AcksLeader,
	}, nil)
	require.NoError(t, err)
	for i := 0; i < 5; i++ {
		_, err := prod.Produce(context.Background(), NewRecord([]byte(fmt.Sprintf("m-%d", i))).WithTopic(topic))
		require.NoError(t, err)
	}
	require.NoError(t, prod.Flush(context.Background()))
	prod.Close()

	consCfg := &ConsumerConfig{
		BaseConfig:         BaseConfig{Brokers: broker, AuthType: AuthTypeNone},
		Topics:             []string{topic},
		Group:              group,
		StartOffset:        OffsetStart,
		AutoCommit:         true,
		AutoCommitMarks:    true,
		AutoCommitInterval: 200 * time.Millisecond,
	}

	c, err := NewConsumer(consCfg, nil)
	require.NoError(t, err)

	drained := 0
	deadline := time.Now().Add(25 * time.Second)
	for drained < 5 && time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		res, err := c.Poll(ctx)
		cancel()
		if err != nil {
			continue
		}
		recs := res.Records()
		if len(recs) == 0 {
			continue
		}
		c.MarkCommitOffsets(recs) // mark only after (would-be) durable processing
		drained += len(recs)
	}
	require.Equal(t, 5, drained)

	time.Sleep(400 * time.Millisecond) // let the background committer commit the marks
	c.Close()                          // and a graceful close commits marks too

	// Fresh member of the same group must resume past the committed offsets.
	c2, err := NewConsumer(consCfg, nil)
	require.NoError(t, err)
	defer c2.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	res, _ := c2.Poll(ctx)
	cancel()
	got := 0
	if res != nil {
		got = len(res.Records())
	}
	assert.Equal(t, 0, got, "marked+committed offsets must prevent redelivery")
}
