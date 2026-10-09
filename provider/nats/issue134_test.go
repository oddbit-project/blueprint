package nats

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/oddbit-project/blueprint/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// deliverServer completes the handshake, answers every PING, and sends one message,
// with the given reply subject, on the first subscription the client makes
func deliverServer(t *testing.T, reply string) string {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		defer func() { _ = c.Close() }()
		_, _ = c.Write([]byte(fakeServerINFO))
		r := bufio.NewReader(c)
		sent := false
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				return
			}
			f := strings.Fields(line)
			switch {
			case len(f) >= 3 && f[0] == "SUB" && !sent:
				sent = true
				if reply != "" {
					_, _ = fmt.Fprintf(c, "MSG %s %s %s 1\r\nx\r\n", f[1], f[len(f)-1], reply)
				} else {
					_, _ = fmt.Fprintf(c, "MSG %s %s 1\r\nx\r\n", f[1], f[len(f)-1])
				}
			case len(f) >= 1 && f[0] == "PING":
				_, _ = c.Write([]byte("PONG\r\n"))
			}
		}
	}()
	return "nats://" + ln.Addr().String()
}

// literal134 builds a Consumer as a struct literal, without NewConsumer
func literal134(t *testing.T, url, subject string, logger *log.Logger) *Consumer {
	nc, err := nats.Connect(url, nats.DrainTimeout(2*time.Second))
	require.NoError(t, err)
	t.Cleanup(nc.Close)
	return &Consumer{Conn: nc, Subject: subject, Logger: logger}
}

// await134 waits for ch to be closed, failing the test after 5s
func await134(t *testing.T, ch <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		t.Fatal(what)
	}
}

// waitHandlers waits for the Subscribe handler goroutines to return
func waitHandlers(t *testing.T, c *Consumer) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		c.handlerWg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("handler goroutine did not return")
	}
}

func TestIssue134_SubscribeNilLoggerReceivesMessages(t *testing.T) {
	c := literal134(t, deliverServer(t, ""), "x", nil)
	got := make(chan Message, 1)
	var err error
	require.NotPanics(t, func() {
		err = c.Subscribe(context.Background(), func(_ context.Context, m Message) error {
			got <- m
			return nil
		})
	})
	require.NoError(t, err)
	select {
	case m := <-got:
		assert.Equal(t, "x", m.Subject)
	case <-time.After(5 * time.Second):
		t.Fatal("handler never received the message")
	}
	c.Disconnect()
}

func TestIssue134_DisconnectAfterNilLoggerSubscribeIsPrompt(t *testing.T) {
	c := literal134(t, handshakeServer(t), "x", nil)
	// assert, not require: a recovered panic must still reach the Disconnect timing below
	assert.NotPanics(t, func() {
		_ = c.Subscribe(context.Background(), func(context.Context, Message) error { return nil })
	})
	start := time.Now()
	c.Disconnect()
	assert.Less(t, time.Since(start), time.Second, "Disconnect waited for the drain deadline (2s)")
}

func TestIssue134_NextMsgNilLoggerReturnsError(t *testing.T) {
	c := literal134(t, handshakeServer(t), "x", nil)
	sub, err := c.Conn.SubscribeSync("z")
	require.NoError(t, err)
	require.NoError(t, sub.Unsubscribe())
	require.NotPanics(t, func() { _, err = c.NextMsg(sub, 10*time.Millisecond) })
	assert.ErrorIs(t, err, nats.ErrBadSubscription)
}

func TestIssue134_UnsubscribeNilLoggerReturnsError(t *testing.T) {
	c := literal134(t, handshakeServer(t), "x", nil)
	sub, err := c.Conn.SubscribeSync("z")
	require.NoError(t, err)
	require.NoError(t, sub.Unsubscribe())
	require.NotPanics(t, func() { err = c.Unsubscribe(sub) })
	assert.ErrorIs(t, err, nats.ErrBadSubscription)
}

func TestIssue134_SubscribeErrorNilLoggerReturnsError(t *testing.T) {
	c := literal134(t, handshakeServer(t), "", nil)
	var err error
	require.NotPanics(t, func() {
		err = c.Subscribe(context.Background(), func(context.Context, Message) error { return nil })
	})
	assert.ErrorIs(t, err, nats.ErrBadSubject)
	require.NotPanics(t, func() { _, err = c.SubscribeSync() })
	assert.ErrorIs(t, err, nats.ErrBadSubject)
}

// A panic in the handler goroutine cannot be recovered here: it ends the test binary
func TestIssue134_HandlerGoroutineNilLogger(t *testing.T) {
	t.Run("handler error", func(t *testing.T) {
		c := literal134(t, deliverServer(t, ""), "x", nil)
		handled := make(chan struct{})
		require.NotPanics(t, func() {
			_ = c.Subscribe(context.Background(), func(context.Context, Message) error {
				close(handled)
				return errors.New("handler failed")
			})
		})
		await134(t, handled, "handler never ran")
		c.Disconnect()
		waitHandlers(t, c)
	})
	t.Run("ack failure and ctx cancel", func(t *testing.T) {
		c := literal134(t, deliverServer(t, "reply.x"), "x", nil)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		handled := make(chan struct{})
		require.NotPanics(t, func() {
			_ = c.Subscribe(ctx, func(context.Context, Message) error {
				// the ack that follows fails on the closed connection
				c.Conn.Close()
				close(handled)
				return nil
			})
		})
		await134(t, handled, "handler never ran")
		cancel()
		waitHandlers(t, c)
	})
}

// Guard: a Consumer with a Logger still logs each of these errors

func TestIssue134_GuardLoggerStillLogs(t *testing.T) {
	logged := func(t *testing.T, buf *bytes.Buffer, msg string) {
		t.Helper()
		assert.Contains(t, buf.String(), msg)
	}

	t.Run("subscribe, handler, ack and cancel", func(t *testing.T) {
		logger, buf := setupTestLogger(t)
		c := literal134(t, deliverServer(t, "reply.x"), "x", logger)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		handled := make(chan struct{})
		require.NoError(t, c.Subscribe(ctx, func(context.Context, Message) error {
			c.Conn.Close()
			close(handled)
			return errors.New("handler failed")
		}))
		await134(t, handled, "handler never ran")
		cancel()
		waitHandlers(t, c)
		logged(t, buf, "Subscribed to NATS subject")
		logged(t, buf, "Error processing NATS message")
		logged(t, buf, "Failed to acknowledge message")
		logged(t, buf, "Context canceled, stopping NATS subscription")
	})
	t.Run("subscribe errors", func(t *testing.T) {
		logger, buf := setupTestLogger(t)
		c := literal134(t, handshakeServer(t), "", logger)
		assert.Error(t, c.Subscribe(context.Background(), func(context.Context, Message) error { return nil }))
		logged(t, buf, "Failed to subscribe to NATS subject")
		_, err := c.SubscribeSync()
		assert.Error(t, err)
		logged(t, buf, "Failed to subscribe synchronously to NATS subject")
	})
	t.Run("NextMsg and Unsubscribe errors", func(t *testing.T) {
		logger, buf := setupTestLogger(t)
		c := literal134(t, handshakeServer(t), "x", logger)
		sub, err := c.Conn.SubscribeSync("z")
		require.NoError(t, err)
		require.NoError(t, sub.Unsubscribe())
		_, err = c.NextMsg(sub, 10*time.Millisecond)
		assert.Error(t, err)
		logged(t, buf, "Error getting next NATS message")
		assert.Error(t, c.Unsubscribe(sub))
		logged(t, buf, "Failed to unsubscribe from NATS subject")
	})
}
