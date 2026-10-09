package prometheus

import (
	"testing"

	tlsProvider "github.com/oddbit-project/blueprint/provider/tls"
	"github.com/stretchr/testify/assert"
)

func TestIssue128_ConfigValidateRejectsHalfPair(t *testing.T) {
	halfPairs := map[string]tlsProvider.ServerConfig{
		"cert without key":  {TLSEnable: true, TLSCert: "server.crt"},
		"key without cert":  {TLSEnable: true, TLSKey: "server.key"},
		"key password only": {TLSEnable: true, TlsKeyCredential: tlsProvider.TlsKeyCredential{Password: "x"}},
	}
	for name, tc := range halfPairs {
		t.Run(name, func(t *testing.T) {
			cfg := NewConfig()
			cfg.ServerConfig.ServerConfig = tc
			assert.ErrorIs(t, cfg.Validate(), tlsProvider.ErrTLSIncompleteKeyPair)
		})
	}
}
