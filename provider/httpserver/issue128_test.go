package httpserver

import (
	"testing"

	tlsProvider "github.com/oddbit-project/blueprint/provider/tls"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var halfPairs128 = map[string]tlsProvider.ServerConfig{
	"cert without key":  {TLSEnable: true, TLSCert: "server.crt"},
	"key without cert":  {TLSEnable: true, TLSKey: "server.key"},
	"key password only": {TLSEnable: true, TlsKeyCredential: tlsProvider.TlsKeyCredential{Password: "x"}},
}

func TestIssue128_ValidateRejectsHalfPair(t *testing.T) {
	for name, tc := range halfPairs128 {
		t.Run(name, func(t *testing.T) {
			cfg := NewServerConfig()
			cfg.ServerConfig = tc
			assert.ErrorIs(t, cfg.Validate(), tlsProvider.ErrTLSIncompleteKeyPair)
		})
	}
}

func TestIssue128_NewServerRejectsHalfPair(t *testing.T) {
	for name, tc := range halfPairs128 {
		t.Run(name, func(t *testing.T) {
			cfg := NewServerConfig()
			cfg.ServerConfig = tc
			srv, err := NewServer(cfg, nil)
			assert.ErrorIs(t, err, tlsProvider.ErrTLSIncompleteKeyPair)
			assert.Nil(t, srv)
		})
	}
}

// Guard: a full pair passes Validate (the files are only read by NewServer)
func TestIssue128_ValidateAcceptsFullPair(t *testing.T) {
	cfg := NewServerConfig()
	cfg.ServerConfig = tlsProvider.ServerConfig{TLSEnable: true, TLSCert: "server.crt", TLSKey: "server.key"}
	require.NoError(t, cfg.Validate())
}

// Guard: the port range is checked before the key pair
func TestIssue128_ValidatePortCheckedFirst(t *testing.T) {
	cfg := NewServerConfig()
	cfg.Port = 70000
	cfg.ServerConfig = halfPairs128["cert without key"]
	assert.EqualError(t, cfg.Validate(), "port must be between 0 and 65535")
}
