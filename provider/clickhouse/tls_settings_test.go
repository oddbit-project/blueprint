package clickhouse

import (
	"testing"

	tlsProvider "github.com/oddbit-project/blueprint/provider/tls"
	"github.com/stretchr/testify/assert"
)

// tlsSettings returns one ClientConfig per TLS setting, each given without TLSEnable
func tlsSettings() map[string]tlsProvider.ClientConfig {
	return map[string]tlsProvider.ClientConfig{
		"ca":                   {TLSCA: "/ca.pem"},
		"cert":                 {TLSCert: "/c.pem"},
		"key":                  {TLSKey: "/k.pem"},
		"skip verify":          {TLSInsecureSkipVerify: true},
		"key password":         {TlsKeyCredential: tlsProvider.TlsKeyCredential{Password: "<dummy>"}},
		"key password env var": {TlsKeyCredential: tlsProvider.TlsKeyCredential{PasswordEnvVar: "KEY_PASSWORD"}},
		"key password file":    {TlsKeyCredential: tlsProvider.TlsKeyCredential{PasswordFile: "/key-password"}},
	}
}

// tlsAccepted returns the TLS configurations that stay valid
func tlsAccepted() map[string]tlsProvider.ClientConfig {
	return map[string]tlsProvider.ClientConfig{
		"enabled with settings": {TLSEnable: true, TLSCA: "/ca.pem", TLSCert: "/c.pem", TLSKey: "/k.pem"},
		"enabled with key password": {TLSEnable: true, TLSCert: "/c.pem", TLSKey: "/k.pem",
			TlsKeyCredential: tlsProvider.TlsKeyCredential{Password: "<dummy>", PasswordEnvVar: "KEY_PASSWORD", PasswordFile: "/key-password"}},
		"no tls settings": {},
	}
}

// validators returns each config's Validate for the given TLS settings
func validators(c tlsProvider.ClientConfig) map[string]func() error {
	cfg := NewClientConfig()
	cfg.Hosts = []string{"localhost:9000"}
	cfg.ClientConfig = c
	return map[string]func() error{"client": cfg.Validate}
}

func TestIssue116_TLSSettingsWithoutEnableRejected(t *testing.T) {
	for name, c := range tlsSettings() {
		for kind, validate := range validators(c) {
			t.Run(kind+"/"+name, func(t *testing.T) {
				assert.ErrorIs(t, validate(), tlsProvider.ErrTLSNotEnabled)
			})
		}
	}
}

func TestIssue116_TLSSettingsAcceptedWithEnableOrNone(t *testing.T) {
	for name, c := range tlsAccepted() {
		for kind, validate := range validators(c) {
			t.Run(kind+"/"+name, func(t *testing.T) {
				assert.NoError(t, validate())
			})
		}
	}
}

// tlsIncompleteKeyPairs returns one ClientConfig per incomplete client certificate/key pair, each with TLSEnable
func tlsIncompleteKeyPairs() map[string]tlsProvider.ClientConfig {
	return map[string]tlsProvider.ClientConfig{
		"cert without key":          {TLSEnable: true, TLSCert: "/c.pem"},
		"key without cert":          {TLSEnable: true, TLSKey: "/k.pem"},
		"key password without pair": {TLSEnable: true, TlsKeyCredential: tlsProvider.TlsKeyCredential{Password: "<dummy>"}},
	}
}

func TestIssue115_TLSIncompleteKeyPairRejected(t *testing.T) {
	for name, c := range tlsIncompleteKeyPairs() {
		for kind, validate := range validators(c) {
			t.Run(kind+"/"+name, func(t *testing.T) {
				assert.ErrorIs(t, validate(), tlsProvider.ErrTLSIncompleteKeyPair)
			})
		}
	}
}
