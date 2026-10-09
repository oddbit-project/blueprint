package nats

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/oddbit-project/blueprint/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// quietLogger discards output; the race tests log an error on every call after Disconnect
func quietLogger() *log.Logger {
	return log.New("nats").WithOutput(io.Discard)
}

// Issue #113 guard: reading Conn through a snapshot must not stop the connected path
// from reaching the connection
func TestIssue113_ProducerConnectedPathUsesConnection(t *testing.T) {
	p, err := NewProducer(&ProducerConfig{URL: handshakeServer(t), Subject: "issue113", AuthType: AuthTypeNone}, quietLogger())
	require.NoError(t, err)
	defer p.Disconnect()
	conn := p.Conn

	assert.True(t, p.IsConnected())

	require.NoError(t, p.Publish([]byte("m")))
	assert.Equal(t, uint64(1), conn.Stats().OutMsgs, "Publish")
	require.NoError(t, p.PublishMsg("issue113.msg", []byte("m")))
	assert.Equal(t, uint64(2), conn.Stats().OutMsgs, "PublishMsg")
	require.NoError(t, p.PublishRequest("issue113.preq", "issue113.reply", []byte("m")))
	assert.Equal(t, uint64(3), conn.Stats().OutMsgs, "PublishRequest")

	// the fake server never replies, so a request that reached the connection times out
	_, err = p.Request("issue113.req", []byte("m"), 50*time.Millisecond)
	assert.ErrorIs(t, err, nats.ErrTimeout)
	assert.Equal(t, uint64(4), conn.Stats().OutMsgs, "Request")
}

// Issue #113 guard: once Disconnect has returned every method reports the producer closed
func TestIssue113_ProducerClosedAfterDisconnect(t *testing.T) {
	p, err := NewProducer(&ProducerConfig{URL: handshakeServer(t), Subject: "issue113", AuthType: AuthTypeNone}, quietLogger())
	require.NoError(t, err)
	p.Disconnect()

	assert.False(t, p.IsConnected())
	assert.ErrorIs(t, p.Publish([]byte("m")), ErrProducerClosed)
	assert.ErrorIs(t, p.PublishMsg("issue113.msg", []byte("m")), ErrProducerClosed)
	assert.ErrorIs(t, p.PublishRequest("issue113.preq", "issue113.reply", []byte("m")), ErrProducerClosed)
	_, err = p.Request("issue113.req", []byte("m"), 50*time.Millisecond)
	assert.ErrorIs(t, err, ErrProducerClosed)
}

// Issue #113 guard: Subscribe and SubscribeSync hold subsLock when they check the
// connection, so they must not go through the locking IsConnected
func TestIssue113_ConsumerConnectedPathUsesConnection(t *testing.T) {
	c, err := NewConsumer(&ConsumerConfig{URL: handshakeServer(t), Subject: "issue113", AuthType: AuthTypeNone}, quietLogger())
	require.NoError(t, err)
	conn := c.Conn

	assert.True(t, c.IsConnected())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	type subResult struct {
		sub *nats.Subscription
		err error
	}
	returned := make(chan subResult, 1)
	go func() {
		if err := c.Subscribe(ctx, func(ctx context.Context, msg Message) error { return nil }); err != nil {
			returned <- subResult{err: err}
			return
		}
		sub, err := c.SubscribeSync()
		returned <- subResult{sub: sub, err: err}
	}()
	var res subResult
	select {
	case res = <-returned:
	case <-time.After(5 * time.Second):
		t.Fatal("Subscribe or SubscribeSync did not return")
	}
	require.NoError(t, res.err)
	defer c.Disconnect()

	_, err = c.NextMsg(res.sub, 10*time.Millisecond)
	assert.ErrorIs(t, err, nats.ErrTimeout)
	assert.NoError(t, c.Unsubscribe(res.sub))

	_, err = c.Request("issue113.req", []byte("m"), 50*time.Millisecond)
	assert.ErrorIs(t, err, nats.ErrTimeout)
	assert.Equal(t, uint64(1), conn.Stats().OutMsgs, "Request")
}

// Issue #113 guard: once Disconnect has returned every method reports the consumer closed
func TestIssue113_ConsumerClosedAfterDisconnect(t *testing.T) {
	c, err := NewConsumer(&ConsumerConfig{URL: handshakeServer(t), Subject: "issue113", AuthType: AuthTypeNone}, quietLogger())
	require.NoError(t, err)
	sub, err := c.SubscribeSync()
	require.NoError(t, err)
	c.Disconnect()

	assert.False(t, c.IsConnected())
	_, err = c.NextMsg(sub, 10*time.Millisecond)
	assert.ErrorIs(t, err, ErrConsumerClosed)
	assert.ErrorIs(t, c.Unsubscribe(sub), ErrConsumerClosed)
	_, err = c.Request("issue113.req", []byte("m"), 50*time.Millisecond)
	assert.ErrorIs(t, err, ErrConsumerClosed)
	_, err = c.SubscribeSync()
	assert.ErrorIs(t, err, ErrConsumerClosed)
}
