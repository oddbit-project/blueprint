package nats

import (
	"bufio"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// handshakeServer accepts connections and answers every PING, enough for a client
// to connect; the effective options are then read from the client's Conn
func handshakeServer(t *testing.T) string {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer func() { _ = c.Close() }()
				_, _ = c.Write([]byte(`INFO {"server_id":"x","version":"2.10.0","proto":1,"max_payload":1048576}` + "\r\n"))
				r := bufio.NewReader(c)
				for {
					line, err := r.ReadString('\n')
					if err != nil {
						return
					}
					if strings.HasPrefix(line, "PING") {
						_, _ = c.Write([]byte("PONG\r\n"))
					}
				}
			}(c)
		}
	}()
	return "nats://" + ln.Addr().String()
}

func TestIssue105_DefaultPingAndFlusher(t *testing.T) {
	url := handshakeServer(t)

	p, err := NewProducer(&ProducerConfig{URL: url, Subject: "x", AuthType: AuthTypeNone}, nil)
	require.NoError(t, err)
	defer p.Conn.Close()
	assert.Equal(t, nats.DefaultPingInterval, p.Conn.Opts.PingInterval, "producer ping interval")
	assert.Equal(t, nats.DefaultFlusherTimeout, p.Conn.Opts.FlusherTimeout, "producer flusher timeout")

	c, err := NewConsumer(&ConsumerConfig{URL: url, Subject: "x", AuthType: AuthTypeNone}, nil)
	require.NoError(t, err)
	defer c.Conn.Close()
	assert.Equal(t, nats.DefaultPingInterval, c.Conn.Opts.PingInterval, "consumer ping interval")
}

func TestIssue105_ConfiguredPingInterval(t *testing.T) {
	cfg := &ProducerConfig{URL: handshakeServer(t), Subject: "x", AuthType: AuthTypeNone}
	cfg.PingInterval = 7
	p, err := NewProducer(cfg, nil)
	require.NoError(t, err)
	defer p.Conn.Close()
	assert.Equal(t, 7*time.Second, p.Conn.Opts.PingInterval)
}

func TestIssue105_DrainTimeout(t *testing.T) {
	url := handshakeServer(t)

	cfg := &ProducerConfig{URL: url, Subject: "x", AuthType: AuthTypeNone}
	cfg.DrainTimeout = 5000
	p, err := NewProducer(cfg, nil)
	require.NoError(t, err)
	defer p.Conn.Close()
	assert.Equal(t, 5*time.Second, p.Conn.Opts.DrainTimeout, "configured producer drain timeout")

	ccfg := &ConsumerConfig{URL: url, Subject: "x", AuthType: AuthTypeNone}
	ccfg.DrainTimeout = 4000
	c, err := NewConsumer(ccfg, nil)
	require.NoError(t, err)
	defer c.Conn.Close()
	assert.Equal(t, 4*time.Second, c.Conn.Opts.DrainTimeout, "configured consumer drain timeout")

	d, err := NewProducer(&ProducerConfig{URL: url, Subject: "x", AuthType: AuthTypeNone}, nil)
	require.NoError(t, err)
	defer d.Conn.Close()
	assert.Equal(t, nats.DefaultDrainTimeout, d.Conn.Opts.DrainTimeout, "default drain timeout")
}

func TestIssue105_JetStreamDefaultPing(t *testing.T) {
	conn, err := (&JSConnectionConfig{URL: handshakeServer(t), AuthType: AuthTypeNone}).dial("x")
	require.NoError(t, err)
	defer conn.Close()
	assert.Equal(t, nats.DefaultPingInterval, conn.Opts.PingInterval)
}
