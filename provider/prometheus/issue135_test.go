package prometheus

import (
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/oddbit-project/blueprint/provider/httpserver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIssue135_StartTLSEnableNoCertificate(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := l.Addr().(*net.TCPAddr).Port
	require.NoError(t, l.Close())

	cfg := NewConfig()
	cfg.Host = "127.0.0.1"
	cfg.Port = port
	cfg.TLSEnable = true
	srv, err := NewServer(cfg, nil)
	require.NoError(t, err)
	require.NotNil(t, srv)

	res := make(chan error, 1)
	go func() { res <- srv.Start() }()
	select {
	case err = <-res:
	case <-time.After(2 * time.Second):
		_ = srv.Server.Server.Close()
		t.Fatal("Start kept serving on " + strconv.Itoa(port))
	}
	assert.ErrorIs(t, err, httpserver.ErrTLSNoCertificate)
}
