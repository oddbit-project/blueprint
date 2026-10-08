package nats

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// startSlowConsumer subscribes with a handler that takes `each` per message, publishes
// n messages, and waits until the first one is being handled
func (s *NATSIntegrationTestSuite) startSlowConsumer(subject string, drainMs uint, each time.Duration, n int) (*Consumer, *int32) {
	cfg := &ConsumerConfig{URL: s.natsURL, Subject: subject, AuthType: AuthTypeNone}
	cfg.DrainTimeout = drainMs
	c, err := NewConsumer(cfg, s.logger)
	require.NoError(s.T(), err)

	var handled int32
	var started int32
	require.NoError(s.T(), c.Subscribe(context.Background(), func(ctx context.Context, msg Message) error {
		atomic.StoreInt32(&started, 1)
		time.Sleep(each)
		atomic.AddInt32(&handled, 1)
		return nil
	}))

	p := s.getTestProducer(subject)
	defer p.Disconnect()
	for i := 0; i < n; i++ {
		require.NoError(s.T(), p.Publish([]byte("m")))
	}
	require.NoError(s.T(), p.Conn.Flush())
	require.Eventually(s.T(), func() bool { return atomic.LoadInt32(&started) == 1 }, 5*time.Second, 5*time.Millisecond)
	return c, &handled
}

func (s *NATSIntegrationTestSuite) TestIssue105_DisconnectFinishesDeliveredMessages() {
	c, handled := s.startSlowConsumer("issue105.drain", 10000, 50*time.Millisecond, 10)

	start := time.Now()
	c.Disconnect()
	assert.Equal(s.T(), int32(10), atomic.LoadInt32(handled), "Disconnect returned before the delivered messages were handled")
	// about 10 x 50ms of handling; it must not sit out the 10s drain timeout once done
	assert.Less(s.T(), time.Since(start), 5*time.Second, "Disconnect waited after the handlers had finished")
}

func (s *NATSIntegrationTestSuite) TestIssue105_DisconnectBoundedByDrainTimeout() {
	c, _ := s.startSlowConsumer("issue105.bounded", 500, 2*time.Second, 10)

	start := time.Now()
	c.Disconnect()
	// drainTimeout (500ms) plus margin: against a live server the connection drain
	// finishes in a round trip, far below 10 messages x 2s
	assert.Less(s.T(), time.Since(start), 2*time.Second, "Disconnect waited beyond drainTimeout")
}

func (s *NATSIntegrationTestSuite) TestIssue105_DisconnectWithSyncSubscriptionDoesNotStall() {
	cfg := &ConsumerConfig{URL: s.natsURL, Subject: "issue105.sync", AuthType: AuthTypeNone}
	cfg.DrainTimeout = 3000
	c, err := NewConsumer(cfg, s.logger)
	require.NoError(s.T(), err)
	sub, err := c.SubscribeSync()
	require.NoError(s.T(), err)

	p := s.getTestProducer("issue105.sync")
	defer p.Disconnect()
	require.NoError(s.T(), p.Publish([]byte("unread")))
	require.NoError(s.T(), p.Conn.Flush())
	require.Eventually(s.T(), func() bool {
		n, _, _ := sub.Pending()
		return n == 1
	}, 5*time.Second, 5*time.Millisecond, "the unread message never reached the subscription")

	start := time.Now()
	c.Disconnect()
	// an unread SubscribeSync message cannot be drained through the provider; it must
	// not hold Disconnect for the whole drain timeout
	assert.Less(s.T(), time.Since(start), 2*time.Second, "Disconnect stalled on a SubscribeSync subscription")
}

func (s *NATSIntegrationTestSuite) TestIssue105_DisconnectDeliversReplies() {
	const n = 5
	cfg := &ConsumerConfig{URL: s.natsURL, Subject: "issue105.request", AuthType: AuthTypeNone}
	cfg.DrainTimeout = 10000
	c, err := NewConsumer(cfg, s.logger)
	require.NoError(s.T(), err)
	conn := c.Conn

	var started int32
	require.NoError(s.T(), c.Subscribe(context.Background(), func(ctx context.Context, msg Message) error {
		atomic.StoreInt32(&started, 1)
		time.Sleep(100 * time.Millisecond)
		// a handler's own publish, besides the provider's reply acknowledgement
		return conn.Publish("issue105.out", msg.Data)
	}))
	require.NoError(s.T(), c.Conn.Flush())

	p := s.getTestProducer("issue105.request")
	defer p.Disconnect()
	replies, err := p.Conn.SubscribeSync("issue105.reply.*")
	require.NoError(s.T(), err)
	outs, err := p.Conn.SubscribeSync("issue105.out")
	require.NoError(s.T(), err)
	// unread messages would otherwise hold the producer's drain for drainTimeout at teardown
	defer func() {
		_ = replies.Unsubscribe()
		_ = outs.Unsubscribe()
	}()
	for i := 0; i < n; i++ {
		require.NoError(s.T(), p.PublishRequest("issue105.request", fmt.Sprintf("issue105.reply.%d", i), []byte("m")))
	}
	require.NoError(s.T(), p.Conn.Flush())
	require.Eventually(s.T(), func() bool { return atomic.LoadInt32(&started) == 1 }, 5*time.Second, 5*time.Millisecond)

	c.Disconnect()
	// the consumer's connection flushed before closing, so one round trip here is
	// enough for everything it sent to have reached this requester
	require.NoError(s.T(), p.Conn.Flush())
	gotReplies, _, err := replies.Pending()
	require.NoError(s.T(), err)
	assert.Equal(s.T(), n, gotReplies, "replies sent while Disconnect drained did not reach the requester")
	gotOuts, _, err := outs.Pending()
	require.NoError(s.T(), err)
	assert.Equal(s.T(), n, gotOuts, "handler publishes sent while Disconnect drained did not arrive")
}
