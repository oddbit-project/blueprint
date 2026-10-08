package nats

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func consumerClosing(c *Consumer) bool {
	c.subsLock.Lock()
	defer c.subsLock.Unlock()
	return c.closing
}

func producerClosing(p *Producer) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.closing
}

func (s *NATSIntegrationTestSuite) TestIssue105_SecondDisconnectWaitsForFirst() {
	c, handled := s.startSlowConsumer("issue105.second", 10000, 50*time.Millisecond, 10)

	go c.Disconnect()
	require.Eventually(s.T(), func() bool { return consumerClosing(c) }, 5*time.Second, time.Millisecond)
	// a shutdown path that calls Disconnect again (for example a deferred call after a
	// signal handler's) must not return before the first has finished
	c.Disconnect()
	assert.Equal(s.T(), int32(10), atomic.LoadInt32(handled), "the second Disconnect returned before the first had finished")
}

func (s *NATSIntegrationTestSuite) TestIssue105_SubscribeRejectedOnceDisconnectStarted() {
	c, _ := s.startSlowConsumer("issue105.closing", 10000, 50*time.Millisecond, 10)
	conn := c.Conn

	done := make(chan struct{})
	go func() {
		c.Disconnect()
		close(done)
	}()
	require.Eventually(s.T(), func() bool { return consumerClosing(c) }, 5*time.Second, time.Millisecond)
	// the connection is still open while the handlers drain; only the closing flag
	// keeps new subscriptions out
	require.True(s.T(), conn.IsConnected())

	err := c.Subscribe(s.ctx, func(ctx context.Context, msg Message) error { return nil })
	assert.ErrorIs(s.T(), err, ErrConsumerClosed, "Subscribe")
	_, err = c.SubscribeSync()
	assert.ErrorIs(s.T(), err, ErrConsumerClosed, "SubscribeSync")
	<-done
}

func (s *NATSIntegrationTestSuite) TestIssue105_SecondProducerDisconnectWaitsForFirst() {
	cfg := &ProducerConfig{URL: s.natsURL, Subject: "issue105.pdrain", AuthType: AuthTypeNone}
	cfg.DrainTimeout = 1000
	p, err := NewProducer(cfg, s.logger)
	require.NoError(s.T(), err)
	// an unread message on a raw subscription keeps the producer's drain busy until
	// drainTimeout, so the first Disconnect is still running when the second starts
	sub, err := p.Conn.SubscribeSync("issue105.pdrain")
	require.NoError(s.T(), err)
	require.NoError(s.T(), p.Publish([]byte("unread")))
	require.Eventually(s.T(), func() bool {
		n, _, _ := sub.Pending()
		return n == 1
	}, 5*time.Second, 5*time.Millisecond)

	go p.Disconnect()
	require.Eventually(s.T(), func() bool { return producerClosing(p) }, 5*time.Second, time.Millisecond)
	p.Disconnect()
	p.mu.Lock()
	finished := p.Conn == nil
	p.mu.Unlock()
	assert.True(s.T(), finished, "the second Disconnect returned before the first had finished")
}
