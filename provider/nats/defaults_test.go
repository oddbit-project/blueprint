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

// fakeServerINFO is the INFO line the fake servers send: no TLS, no auth
const fakeServerINFO = `INFO {"server_id":"x","version":"2.10.0","proto":1,"max_payload":1048576}` + "\r\n"

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
				_, _ = c.Write([]byte(fakeServerINFO))
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
	assert.Equal(t, nats.DefaultFlusherTimeout, c.Conn.Opts.FlusherTimeout, "consumer flusher timeout")
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
	cases := []struct {
		name    string
		connect func(drainMs uint) (*nats.Conn, error)
		drainMs uint
		want    time.Duration
	}{
		{"configured producer", producerConn(url), 5000, 5 * time.Second},
		{"configured consumer", consumerConn(url), 4000, 4 * time.Second},
		{"default producer", producerConn(url), 0, nats.DefaultDrainTimeout},
		{"default consumer", consumerConn(url), 0, nats.DefaultDrainTimeout},
	}
	for _, tc := range cases {
		conn, err := tc.connect(tc.drainMs)
		require.NoError(t, err, tc.name)
		assert.Equal(t, tc.want, conn.Opts.DrainTimeout, tc.name)
		conn.Close()
	}
}

func producerConn(url string) func(uint) (*nats.Conn, error) {
	return func(drainMs uint) (*nats.Conn, error) {
		cfg := &ProducerConfig{URL: url, Subject: "x", AuthType: AuthTypeNone}
		cfg.DrainTimeout = drainMs
		p, err := NewProducer(cfg, nil)
		if err != nil {
			return nil, err
		}
		return p.Conn, nil
	}
}

func consumerConn(url string) func(uint) (*nats.Conn, error) {
	return func(drainMs uint) (*nats.Conn, error) {
		cfg := &ConsumerConfig{URL: url, Subject: "x", AuthType: AuthTypeNone}
		cfg.DrainTimeout = drainMs
		c, err := NewConsumer(cfg, nil)
		if err != nil {
			return nil, err
		}
		return c.Conn, nil
	}
}

func TestIssue105_JetStreamDefaultPing(t *testing.T) {
	conn, err := (&JSConnectionConfig{URL: handshakeServer(t), AuthType: AuthTypeNone}).dial("x", 0)
	require.NoError(t, err)
	defer conn.Close()
	assert.Equal(t, nats.DefaultPingInterval, conn.Opts.PingInterval)
}

func TestIssue105_ApplyOptionsDrainTimeout(t *testing.T) {
	var popts, copts nats.Options
	ProducerOptions{DrainTimeout: 5000}.ApplyOptions(&popts)
	ConsumerOptions{DrainTimeout: 4000}.ApplyOptions(&copts)
	assert.Equal(t, 5*time.Second, popts.DrainTimeout, "ProducerOptions")
	assert.Equal(t, 4*time.Second, copts.DrainTimeout, "ConsumerOptions")
}
