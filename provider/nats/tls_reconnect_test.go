package nats

import (
	"bufio"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// plaintextEvent is what the plaintext recorder saw on one connection: either a CONNECT
// line or the client closing the connection without sending one
type plaintextEvent struct {
	acceptedAt time.Time
	connect    bool
}

// plaintextRecorder is a server without TLS that accepts any number of connections and
// reports, per connection, whether the client sent CONNECT in plaintext or hung up
func plaintextRecorder(t *testing.T) (string, <-chan plaintextEvent) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })
	events := make(chan plaintextEvent, 16)
	report := func(ev plaintextEvent) {
		select {
		case events <- ev:
		default:
		}
	}
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn, acceptedAt time.Time) {
				defer func() { _ = c.Close() }()
				_ = c.SetDeadline(time.Now().Add(5 * time.Second))
				_, _ = c.Write([]byte(`INFO {"server_id":"p","version":"2.10.0","proto":1,"max_payload":1048576,"auth_required":true}` + "\r\n"))
				r := bufio.NewReader(c)
				for {
					line, err := r.ReadString('\n')
					if err != nil {
						report(plaintextEvent{acceptedAt: acceptedAt})
						return
					}
					if strings.HasPrefix(line, "CONNECT") {
						report(plaintextEvent{acceptedAt: acceptedAt, connect: true})
						return
					}
				}
			}(c, time.Now())
		}
	}()
	return "nats://" + ln.Addr().String(), events
}

func TestIssue110_ReconnectRefusesPlaintextServer(t *testing.T) {
	tlsURL, caFile, tlsDone := fakeServer(t, fakeTLSRequired)
	plainURL, events := plaintextRecorder(t)

	// the TLS server closes after the handshake, so the client moves on to the other
	// server; the pool order is random, so the plaintext one may also be tried first
	p, err := NewProducer(tlsProducerConfig(tlsURL+","+plainURL, caFile, true), nil)
	require.NoError(t, err)
	defer p.Conn.Close()
	var closedAt time.Time
	select {
	case res := <-tlsDone:
		require.True(t, res.tlsConnect, "the first connection was not over TLS")
		closedAt = res.closedAt
	case <-time.After(10 * time.Second):
		t.Fatal("the TLS server never completed a handshake")
	}

	// only a connection accepted after the TLS server closed is a reconnect
	timeout := time.After(10 * time.Second)
	for {
		select {
		case ev := <-events:
			if !ev.acceptedAt.After(closedAt) {
				continue
			}
			assert.False(t, ev.connect, "credentials were sent in plaintext after a reconnect")
			return
		case <-timeout:
			t.Fatal("the client never reconnected to the plaintext server")
		}
	}
}
