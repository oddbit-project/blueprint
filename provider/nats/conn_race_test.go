//go:build race

package nats

import (
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
)

// callPastClose runs call in a goroutine until conn has closed and then for another
// 200ms. It never synchronises with Disconnect, so calls that land after Disconnect has
// closed the connection are concurrent with its write of the Conn field; an unlocked
// read of Conn in call is reported by the race detector, which fails the test
func callPastClose(conn *nats.Conn, call func()) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		for !conn.IsClosed() {
			call()
		}
		for end := time.Now().Add(200 * time.Millisecond); time.Now().Before(end); {
			call()
		}
	}()
	return done
}

func waitDone(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("caller did not finish")
	}
}

// Issue #113 claim 1: Producer methods called concurrently with Disconnect do not race
// on the Conn field
func TestIssue113_ProducerMethodsDoNotRaceDisconnect(t *testing.T) {
	calls := map[string]func(p *Producer){
		"IsConnected": func(p *Producer) { _ = p.IsConnected() },
		"Publish":     func(p *Producer) { _ = p.Publish([]byte("m")) },
		"PublishMsg":  func(p *Producer) { _ = p.PublishMsg("issue113.msg", []byte("m")) },
		"PublishRequest": func(p *Producer) {
			_ = p.PublishRequest("issue113.preq", "issue113.reply", []byte("m"))
		},
		"Request": func(p *Producer) { _, _ = p.Request("issue113.req", []byte("m"), time.Millisecond) },
	}
	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			p, err := NewProducer(&ProducerConfig{URL: handshakeServer(t), Subject: "issue113", AuthType: AuthTypeNone}, quietLogger())
			require.NoError(t, err)

			done := callPastClose(p.Conn, func() { call(p) })
			p.Disconnect()
			waitDone(t, done)
		})
	}
}

// Issue #113 claim 2: Consumer methods called concurrently with Disconnect do not race
// on the Conn field
func TestIssue113_ConsumerMethodsDoNotRaceDisconnect(t *testing.T) {
	calls := map[string]func(c *Consumer, sub *nats.Subscription){
		"IsConnected": func(c *Consumer, _ *nats.Subscription) { _ = c.IsConnected() },
		"NextMsg": func(c *Consumer, sub *nats.Subscription) {
			_, _ = c.NextMsg(sub, time.Millisecond)
		},
		"Unsubscribe": func(c *Consumer, sub *nats.Subscription) { _ = c.Unsubscribe(sub) },
		"Request": func(c *Consumer, _ *nats.Subscription) {
			_, _ = c.Request("issue113.req", []byte("m"), time.Millisecond)
		},
	}
	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			c, err := NewConsumer(&ConsumerConfig{URL: handshakeServer(t), Subject: "issue113", AuthType: AuthTypeNone}, quietLogger())
			require.NoError(t, err)
			sub, err := c.SubscribeSync()
			require.NoError(t, err)

			done := callPastClose(c.Conn, func() { call(c, sub) })
			c.Disconnect()
			waitDone(t, done)
		})
	}
}
