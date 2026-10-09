package tls

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIssue128_ServerValidateKeyPair(t *testing.T) {
	rejected := map[string]ServerConfig{
		"cert without key":              {TLSCert: "server.crt"},
		"cert and key password, no key": {TLSCert: "server.crt", TlsKeyCredential: TlsKeyCredential{Password: "x"}},
		"cert and allowed CAs, no key":  {TLSCert: "server.crt", TLSAllowedCACerts: []string{"ca.crt"}},
		"key without cert":              {TLSKey: "server.key"},
		"key and key password, no cert": {TLSKey: "server.key", TlsKeyCredential: TlsKeyCredential{PasswordFile: "pw"}},
		"key password only":             {TlsKeyCredential: TlsKeyCredential{Password: "x"}},
		"key password env var only":     {TlsKeyCredential: TlsKeyCredential{PasswordEnvVar: "PW"}},
		"key password file only":        {TlsKeyCredential: TlsKeyCredential{PasswordFile: "pw"}},
		"key password and allowed CAs":  {TLSAllowedCACerts: []string{"ca.crt"}, TlsKeyCredential: TlsKeyCredential{Password: "x"}},
	}
	for name, cfg := range rejected {
		t.Run("rejected/"+name, func(t *testing.T) {
			cfg.TLSEnable = true
			assert.ErrorIs(t, cfg.ValidateKeyPair(), ErrTLSIncompleteKeyPair)
		})
	}

	// guards: a full pair, no certificate settings, and anything without TLSEnable are accepted
	accepted := map[string]ServerConfig{
		"full pair":                       {TLSEnable: true, TLSCert: "server.crt", TLSKey: "server.key"},
		"full pair with key password":     {TLSEnable: true, TLSCert: "server.crt", TLSKey: "server.key", TlsKeyCredential: TlsKeyCredential{Password: "x"}},
		"tls enabled, no certificate":     {TLSEnable: true},
		"tls enabled, only allowed CAs":   {TLSEnable: true, TLSAllowedCACerts: []string{"ca.crt"}},
		"tls disabled, cert without key":  {TLSCert: "server.crt"},
		"tls disabled, key password only": {TlsKeyCredential: TlsKeyCredential{Password: "x"}},
	}
	for name, cfg := range accepted {
		t.Run("accepted/"+name, func(t *testing.T) {
			assert.NoError(t, cfg.ValidateKeyPair())
		})
	}
}
