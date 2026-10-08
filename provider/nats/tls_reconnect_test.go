package nats

import (
	"bufio"
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// plaintextRecorder is a server without TLS that accepts any number of connections and
// counts the CONNECT lines it receives in plaintext
func plaintextRecorder(t *testing.T) (url string, accepted, plainConnects *int32) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })
	accepted, plainConnects = new(int32), new(int32)
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			atomic.AddInt32(accepted, 1)
			go func(c net.Conn) {
				defer func() { _ = c.Close() }()
				_ = c.SetDeadline(time.Now().Add(5 * time.Second))
				_, _ = c.Write([]byte(`INFO {"server_id":"p","version":"2.10.0","proto":1,"max_payload":1048576,"auth_required":true}` + "\r\n"))
				r := bufio.NewReader(c)
				for {
					line, err := r.ReadString('\n')
					if err != nil {
						return
					}
					if strings.HasPrefix(line, "CONNECT") {
						atomic.AddInt32(plainConnects, 1)
					}
					if strings.HasPrefix(line, "PING") {
						_, _ = c.Write([]byte("PONG\r\n"))
					}
				}
			}(c)
		}
	}()
	return "nats://" + ln.Addr().String(), accepted, plainConnects
}

func TestIssue110_ReconnectRefusesPlaintextServer(t *testing.T) {
	tlsURL, caFile, tlsDone := fakeServer(t, fakeTLSRequired)
	plainURL, accepted, plainConnects := plaintextRecorder(t)

	// the TLS server closes after the handshake, so the client moves on to the other
	// server; the pool order is random, so the plaintext one may also be tried first
	p, err := NewProducer(tlsProducerConfig(tlsURL+","+plainURL, caFile, true), nil)
	require.NoError(t, err)
	defer p.Conn.Close()
	select {
	case res := <-tlsDone:
		require.True(t, res.tlsConnect, "the first connection was not over TLS")
	case <-time.After(10 * time.Second):
		t.Fatal("the TLS server never completed a handshake")
	}

	// the initial connect may already have tried the plaintext server (random order);
	// only a connection after the TLS server closed proves a reconnect
	before := atomic.LoadInt32(accepted)
	require.Eventually(t, func() bool { return atomic.LoadInt32(accepted) > before }, 10*time.Second, 10*time.Millisecond,
		"the client never reconnected to the plaintext server")
	time.Sleep(200 * time.Millisecond)
	assert.Zero(t, atomic.LoadInt32(plainConnects), "credentials were sent in plaintext after a reconnect")
}
