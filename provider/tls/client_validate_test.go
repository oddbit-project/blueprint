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

func TestIssue115_ValidateEnabledKeyPair(t *testing.T) {
	rejected := map[string]ClientConfig{
		"cert without key":                {TLSEnable: true, TLSCert: "/c.pem"},
		"cert and ca without key":         {TLSEnable: true, TLSCA: "/ca.pem", TLSCert: "/c.pem"},
		"cert and key password, no key":   {TLSEnable: true, TLSCert: "/c.pem", TlsKeyCredential: TlsKeyCredential{Password: "<dummy>"}},
		"key without cert":                {TLSEnable: true, TLSKey: "/k.pem"},
		"key and key password, no cert":   {TLSEnable: true, TLSKey: "/k.pem", TlsKeyCredential: TlsKeyCredential{PasswordFile: "/key-password"}},
		"key password without pair":       {TLSEnable: true, TlsKeyCredential: TlsKeyCredential{Password: "<dummy>"}},
		"key password env var, no pair":   {TLSEnable: true, TlsKeyCredential: TlsKeyCredential{PasswordEnvVar: "KEY_PASSWORD"}},
		"key password file, no pair":      {TLSEnable: true, TlsKeyCredential: TlsKeyCredential{PasswordFile: "/key-password"}},
		"key password with ca and verify": {TLSEnable: true, TLSCA: "/ca.pem", TLSInsecureSkipVerify: true, TlsKeyCredential: TlsKeyCredential{Password: "<dummy>"}},
	}
	for name, c := range rejected {
		t.Run("rejected/"+name, func(t *testing.T) {
			assert.ErrorIs(t, c.ValidateEnabled(), ErrTLSIncompleteKeyPair)
		})
	}

	accepted := map[string]ClientConfig{
		"pair":                       {TLSEnable: true, TLSCert: "/c.pem", TLSKey: "/k.pem"},
		"pair with key password":     {TLSEnable: true, TLSCert: "/c.pem", TLSKey: "/k.pem", TlsKeyCredential: TlsKeyCredential{Password: "<dummy>"}},
		"pair with key password env": {TLSEnable: true, TLSCert: "/c.pem", TLSKey: "/k.pem", TlsKeyCredential: TlsKeyCredential{PasswordEnvVar: "KEY_PASSWORD"}},
		"pair with key password file": {TLSEnable: true, TLSCert: "/c.pem", TLSKey: "/k.pem",
			TlsKeyCredential: TlsKeyCredential{PasswordFile: "/key-password"}},
		"enabled alone":       {TLSEnable: true},
		"enabled with ca":     {TLSEnable: true, TLSCA: "/ca.pem"},
		"enabled skip verify": {TLSEnable: true, TLSInsecureSkipVerify: true},
		"no tls settings":     {},
	}
	for name, c := range accepted {
		t.Run("accepted/"+name, func(t *testing.T) {
			assert.NoError(t, c.ValidateEnabled())
		})
	}
}
