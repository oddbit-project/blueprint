package nats

import (
	"testing"

	tlsProvider "github.com/oddbit-project/blueprint/provider/tls"
	"github.com/stretchr/testify/assert"
)

// validators returns the three connection configs' Validate for the given TLS settings
func validators(tls tlsProvider.ClientConfig) map[string]func() error {
	return map[string]func() error{
		"producer":  ProducerConfig{URL: "nats://h:4222", Subject: "s", AuthType: AuthTypeNone, ClientConfig: tls}.Validate,
		"consumer":  ConsumerConfig{URL: "nats://h:4222", Subject: "s", AuthType: AuthTypeNone, ClientConfig: tls}.Validate,
		"jetstream": JSConnectionConfig{URL: "nats://h:4222", AuthType: AuthTypeNone, ClientConfig: tls}.Validate,
	}
}

func TestIssue110_TLSSettingsWithoutEnableRejected(t *testing.T) {
	cases := map[string]tlsProvider.ClientConfig{
		"ca":                   {TLSCA: "/ca.pem"},
		"cert":                 {TLSCert: "/c.pem"},
		"key":                  {TLSKey: "/k.pem"},
		"skip verify":          {TLSInsecureSkipVerify: true},
		"key password":         {TlsKeyCredential: tlsProvider.TlsKeyCredential{Password: "<dummy>"}},
		"key password env var": {TlsKeyCredential: tlsProvider.TlsKeyCredential{PasswordEnvVar: "KEY_PASSWORD"}},
		"key password file":    {TlsKeyCredential: tlsProvider.TlsKeyCredential{PasswordFile: "/key-password"}},
	}
	for name, tls := range cases {
		for kind, validate := range validators(tls) {
			t.Run(kind+"/"+name, func(t *testing.T) {
				assert.ErrorIs(t, validate(), ErrTLSNotEnabled)
			})
		}
	}
}

func TestIssue110_TLSSettingsAcceptedWithEnableOrNone(t *testing.T) {
	for name, tls := range map[string]tlsProvider.ClientConfig{
		"enabled with ca": {TLSEnable: true, TLSCA: "/ca.pem"},
		"no tls settings": {},
	} {
		for kind, validate := range validators(tls) {
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
