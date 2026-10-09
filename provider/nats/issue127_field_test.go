package nats

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIssue127_GoDrainTimeoutSetsConnOption(t *testing.T) {
	url, _ := ackServer(t, nil)
	cfg := &JSProducerConfig{
		JSConnectionConfig: JSConnectionConfig{URL: url, AuthType: "none"},
		Subject:            "issue114",
		DrainTimeout:       2500,
	}
	p, err := NewJSProducer(cfg, nil)
	require.NoError(t, err)
	t.Cleanup(func() { p.Conn.Close() })
	assert.Equal(t, 2500*time.Millisecond, p.Conn.Opts.DrainTimeout)
}
