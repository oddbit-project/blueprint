package nats

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// jsProducer127 builds a JSProducer from a JSON config against the fake ackServer
func jsProducer127(t *testing.T, url, extra string) *JSProducer {
	var cfg JSProducerConfig
	require.NoError(t, json.Unmarshal([]byte(`{"url":"`+url+`","authType":"none","subject":"issue114"`+extra+`}`), &cfg))
	p, err := NewJSProducer(&cfg, nil)
	require.NoError(t, err)
	t.Cleanup(func() { p.Conn.Close() })
	return p
}

func TestIssue127_JSONDrainTimeoutSetsConnOption(t *testing.T) {
	url, _ := ackServer(t, nil)
	p := jsProducer127(t, url, `,"drainTimeout":1500`)
	assert.Equal(t, 1500*time.Millisecond, p.Conn.Opts.DrainTimeout)
}

func TestIssue127_DisconnectBoundedByConfiguredDrainTimeout(t *testing.T) {
	url, published := ackServer(t, nil)
	p := jsProducer127(t, url, `,"drainTimeout":300`)
	paf := publishAsync(t, p, published)

	start := time.Now()
	select {
	case <-disconnect(p):
	case <-time.After(5 * time.Second):
		t.Fatal("Disconnect did not return within 5s with drainTimeout 300ms")
	}
	assert.GreaterOrEqual(t, time.Since(start), 300*time.Millisecond, "Disconnect returned before the drain timeout")
	select {
	case <-paf.Err():
	case <-time.After(time.Second):
		t.Fatal("pending future not failed")
	}
}

// Guard: no drainTimeout keeps nats.go's 30s default
func TestIssue127_NoDrainTimeoutKeepsDefault(t *testing.T) {
	url, _ := ackServer(t, nil)
	p := jsProducer127(t, url, ``)
	assert.Equal(t, 30*time.Second, p.Conn.Opts.DrainTimeout)
}
