package nats

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func (s *NATSIntegrationTestSuite) TestIssue105_DisconnectDeliversReplies() {
	const n = 5
	cfg := &ConsumerConfig{URL: s.natsURL, Subject: "issue105.request", AuthType: AuthTypeNone}
	cfg.DrainTimeout = 10000
	c, err := NewConsumer(cfg, s.logger)
	require.NoError(s.T(), err)

	var started int32
	require.NoError(s.T(), c.Subscribe(context.Background(), func(ctx context.Context, msg Message) error {
		atomic.StoreInt32(&started, 1)
		time.Sleep(100 * time.Millisecond)
		return nil
	}))
	require.NoError(s.T(), c.Conn.Flush())

	p := s.getTestProducer("issue105.request")
	defer p.Disconnect()
	replies, err := p.Conn.SubscribeSync("issue105.reply.*")
	require.NoError(s.T(), err)
	// unread replies would otherwise hold the producer's drain for drainTimeout at teardown
	defer func() { _ = replies.Unsubscribe() }()
	for i := 0; i < n; i++ {
		require.NoError(s.T(), p.PublishRequest("issue105.request", fmt.Sprintf("issue105.reply.%d", i), []byte("m")))
	}
	require.NoError(s.T(), p.Conn.Flush())
	require.Eventually(s.T(), func() bool { return atomic.LoadInt32(&started) == 1 }, 5*time.Second, 5*time.Millisecond)

	c.Disconnect()
	// the consumer's connection flushed before closing, so one round trip here is
	// enough for every reply it sent to have reached this requester
	require.NoError(s.T(), p.Conn.Flush())
	got, _, err := replies.Pending()
	require.NoError(s.T(), err)
	assert.Equal(s.T(), n, got, "replies sent while Disconnect drained did not reach the requester")
}
