package tls

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIssue116_ValidateEnabled(t *testing.T) {
	rejected := map[string]ClientConfig{
		"ca":                   {TLSCA: "/ca.pem"},
		"cert":                 {TLSCert: "/c.pem"},
		"key":                  {TLSKey: "/k.pem"},
		"skip verify":          {TLSInsecureSkipVerify: true},
		"key password":         {TlsKeyCredential: TlsKeyCredential{Password: "<dummy>"}},
		"key password env var": {TlsKeyCredential: TlsKeyCredential{PasswordEnvVar: "KEY_PASSWORD"}},
		"key password file":    {TlsKeyCredential: TlsKeyCredential{PasswordFile: "/key-password"}},
	}
	for name, c := range rejected {
		t.Run("rejected/"+name, func(t *testing.T) {
			assert.ErrorIs(t, c.ValidateEnabled(), ErrTLSNotEnabled)
		})
	}

	accepted := map[string]ClientConfig{
		"enabled with settings": {TLSEnable: true, TLSCA: "/ca.pem", TLSCert: "/c.pem", TLSKey: "/k.pem", TLSInsecureSkipVerify: true},
		"enabled alone":         {TLSEnable: true},
		"enabled with key password": {TLSEnable: true, TLSCert: "/c.pem", TLSKey: "/k.pem",
			TlsKeyCredential: TlsKeyCredential{Password: "<dummy>", PasswordEnvVar: "KEY_PASSWORD", PasswordFile: "/key-password"}},
		"no tls settings": {},
	}
	for name, c := range accepted {
		t.Run("accepted/"+name, func(t *testing.T) {
			assert.NoError(t, c.ValidateEnabled())
		})
	}
}
