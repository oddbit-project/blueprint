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
		"ca":           {TLSCA: "/ca.pem"},
		"cert and key": {TLSCert: "/c.pem", TLSKey: "/k.pem"},
		"skip verify":  {TLSInsecureSkipVerify: true},
		"key password": {TlsKeyCredential: tlsProvider.TlsKeyCredential{Password: "<dummy>"}},
	}
	for name, tls := range cases {
		for kind, validate := range validators(tls) {
			assert.ErrorIs(t, validate(), ErrTLSNotEnabled, "%s: %s", kind, name)
		}
	}
}

func TestIssue110_TLSSettingsAcceptedWithEnableOrNone(t *testing.T) {
	for name, tls := range map[string]tlsProvider.ClientConfig{
		"enabled with ca": {TLSEnable: true, TLSCA: "/ca.pem"},
		"no tls settings": {},
	} {
		for kind, validate := range validators(tls) {
			assert.NoError(t, validate(), "%s: %s", kind, name)
		}
	}
}
