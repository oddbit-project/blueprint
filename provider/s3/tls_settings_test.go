package s3

import (
	"testing"

	"github.com/oddbit-project/blueprint/provider/tls"
	"github.com/stretchr/testify/assert"
)

// tlsSettings returns one ClientConfig per TLS setting, each given without TLSEnable
func tlsSettings() map[string]tls.ClientConfig {
	return map[string]tls.ClientConfig{
		"ca":                   {TLSCA: "/ca.pem"},
		"cert":                 {TLSCert: "/c.pem"},
		"key":                  {TLSKey: "/k.pem"},
		"skip verify":          {TLSInsecureSkipVerify: true},
		"key password":         {TlsKeyCredential: tls.TlsKeyCredential{Password: "<dummy>"}},
		"key password env var": {TlsKeyCredential: tls.TlsKeyCredential{PasswordEnvVar: "KEY_PASSWORD"}},
		"key password file":    {TlsKeyCredential: tls.TlsKeyCredential{PasswordFile: "/key-password"}},
	}
}

// tlsAccepted returns the TLS configurations that stay valid
func tlsAccepted() map[string]tls.ClientConfig {
	return map[string]tls.ClientConfig{
		"enabled with settings": {TLSEnable: true, TLSCA: "/ca.pem", TLSCert: "/c.pem", TLSKey: "/k.pem"},
		"enabled with key password": {TLSEnable: true, TLSCert: "/c.pem", TLSKey: "/k.pem",
			TlsKeyCredential: tls.TlsKeyCredential{Password: "<dummy>", PasswordEnvVar: "KEY_PASSWORD", PasswordFile: "/key-password"}},
		"no tls settings": {},
	}
}

// validators returns each config's Validate for the given TLS settings
func validators(c tls.ClientConfig) map[string]func() error {
	cfg := NewConfig()
	cfg.ClientConfig = c
	return map[string]func() error{"client": cfg.Validate}
}

func TestIssue116_TLSSettingsWithoutEnableRejected(t *testing.T) {
	for name, c := range tlsSettings() {
		for kind, validate := range validators(c) {
			t.Run(kind+"/"+name, func(t *testing.T) {
				assert.ErrorIs(t, validate(), tls.ErrTLSNotEnabled)
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

func TestIssue116_TLSEnableRequiresUseSSL(t *testing.T) {
	cfg := NewConfig()
	cfg.Endpoint = "localhost:9000"
	cfg.UseSSL = false
	cfg.ClientConfig = tls.ClientConfig{TLSEnable: true, TLSCA: "/ca.pem"}
	assert.ErrorIs(t, cfg.Validate(), ErrTLSRequiresSSL)

	cfg.UseSSL = true
	assert.NoError(t, cfg.Validate())

	cfg.UseSSL = false
	cfg.ClientConfig = tls.ClientConfig{}
	assert.NoError(t, cfg.Validate(), "plain http without TLS settings stays valid for a custom endpoint")

	cfg.ClientConfig = tls.ClientConfig{TLSCA: "/ca.pem"}
	assert.ErrorIs(t, cfg.Validate(), tls.ErrTLSNotEnabled, "settings without tlsEnable report the missing tlsEnable, not useSSL")
}
