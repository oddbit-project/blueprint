package nats

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// drainServer completes the handshake, records the subscription the client makes, and
// holds the PONG for the drain's flush until release is closed; it then delivers one
// more message on that subscription, as a server does for messages already in flight
func drainServer(t *testing.T) (url string, subscribed <-chan struct{}, release chan struct{}) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })
	sub := make(chan struct{})
	release = make(chan struct{})
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		defer func() { _ = c.Close() }()
		_, _ = c.Write([]byte(fakeServerINFO))
		r := bufio.NewReader(c)
		var sid, subject string
		unsubscribed := false
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				return
			}
			f := strings.Fields(line)
			switch {
			case len(f) >= 3 && f[0] == "SUB":
				subject, sid = f[1], f[len(f)-1]
				close(sub)
			case len(f) >= 2 && f[0] == "UNSUB":
				unsubscribed = true
			case len(f) >= 1 && f[0] == "PING":
				if unsubscribed && sid != "" {
					<-release
					_, _ = fmt.Fprintf(c, "MSG %s %s 1\r\nx\r\n", subject, sid)
					sid = ""
				}
				_, _ = c.Write([]byte("PONG\r\n"))
			}
		}
	}()
	return "nats://" + ln.Addr().String(), sub, release
}

func TestIssue105_CancelDuringDrainDoesNotPanic(t *testing.T) {
	url, subscribed, release := drainServer(t)
	cfg := &ConsumerConfig{URL: url, Subject: "issue105", AuthType: AuthTypeNone}
	const drain = 500 * time.Millisecond
	cfg.DrainTimeout = uint(drain / time.Millisecond)
	c, err := NewConsumer(cfg, nil)
	require.NoError(t, err)
	conn := c.Conn

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require.NoError(t, c.Subscribe(ctx, func(ctx context.Context, msg Message) error { return nil }))
	select {
	case <-subscribed:
	case <-time.After(5 * time.Second):
		t.Fatal("server never saw the subscription")
	}

	done := make(chan struct{})
	go func() {
		c.Disconnect()
		close(done)
	}()
	// the server holds the subscription drain's PONG, so the connection drain starts
	// once drainTimeout has passed
	require.Eventually(t, conn.IsDraining, drain+2*time.Second, 5*time.Millisecond)

	// the handler stops while the connection drains, then an in-flight message arrives;
	// a closed handler channel here panics inside nats.go's read loop
	cancel()
	time.Sleep(100 * time.Millisecond)
	close(release)

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("Disconnect did not return")
	}
}
