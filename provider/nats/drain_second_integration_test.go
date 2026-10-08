package nats

import (
	"sync/atomic"
	"time"

	"github.com/stretchr/testify/assert"
)

func (s *NATSIntegrationTestSuite) TestIssue105_SecondDisconnectWaitsForFirst() {
	c, handled := s.startSlowConsumer("issue105.second", 10000, 50*time.Millisecond, 10)

	go c.Disconnect()
	// let the first call start; a shutdown path that calls Disconnect again (for example
	// a deferred call after a signal handler's) must not return before it has finished
	time.Sleep(20 * time.Millisecond)
	c.Disconnect()
	assert.Equal(s.T(), int32(10), atomic.LoadInt32(handled), "the second Disconnect returned before the first had finished")
}
