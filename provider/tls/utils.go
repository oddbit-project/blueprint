/*
  Adapted from https://github.com/influxdata/telegraf/tree/master/plugins/common/tls
  All changes are made available under the original MIT License:

	The MIT License (MIT)

	Copyright (c) 2015-2020 InfluxData Inc.

	Permission is hereby granted, free of charge, to any person obtaining a copy
	of this software and associated documentation files (the "Software"), to deal
	in the Software without restriction, including without limitation the rights
	to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
	copies of the Software, and to permit persons to whom the Software is
	furnished to do so, subject to the following conditions:

	The above copyright notice and this permission notice shall be included in all
	copies or substantial portions of the Software.

	THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
	IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
	FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
	AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
	LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
	OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
	SOFTWARE.
*/

package tls

import (
	"crypto/tls"
	"github.com/oddbit-project/blueprint/utils"
	"github.com/oddbit-project/blueprint/utils/env"
	"github.com/oddbit-project/blueprint/utils/fs"
	"github.com/rs/zerolog/log"
	"strings"
)

const (
	ErrInvalidCipher     = utils.Error("non-supported cipher")
	ErrInvalidTlsVersion = utils.Error("invalid TLS version")
)

type TlsKeyCredential struct {
	Password       string `json:"tlsKeyPassword"`
	PasswordEnvVar string `json:"tlsKeyPasswordEnvVar"`
	PasswordFile   string `json:"tlsKeyPasswordFile"`
}

var tlsVersionMap = map[string]uint16{
	// Removed TLS 1.0 and 1.1 as they are insecure and deprecated
	"TLS12": tls.VersionTLS12,
	"TLS13": tls.VersionTLS13, // TLS 1.3 is recommended
}

// Only include secure cipher suites that are recommended for modern TLS security
// RC4, 3DES, and non-AEAD ciphers are vulnerable
var tlsCipherMap = map[string]uint16{
	// ChaCha20-Poly1305 ciphers (preferred for performance on systems without AES-NI)
	"TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305":   tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
	"TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305": tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,

	// AES-GCM ciphers (AEAD authenticated encryption)
	"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256":   tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
	"TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256": tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
	"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384":   tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
	"TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384": tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,

	// TLS 1.3 cipher suites
	"TLS_AES_128_GCM_SHA256":       tls.TLS_AES_128_GCM_SHA256,
	"TLS_AES_256_GCM_SHA384":       tls.TLS_AES_256_GCM_SHA384,
	"TLS_CHACHA20_POLY1305_SHA256": tls.TLS_CHACHA20_POLY1305_SHA256,
}

// default server cipher suites to be used, if none specified
// Only using secure AEAD ciphers with Perfect Forward Secrecy
var defaultCipherSuites = []uint16{
	// TLS 1.3 cipher suites (preferred)
	tls.TLS_AES_128_GCM_SHA256,
	tls.TLS_AES_256_GCM_SHA384,
	tls.TLS_CHACHA20_POLY1305_SHA256,

	// TLS 1.2 ECDHE cipher suites (for backward compatibility)
	tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
	tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
	tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
	tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
	tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
	tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
}

// ParseCiphers returns a `[]uint16` by received `[]string` key that represents ciphers from crypto/tls.
func ParseCiphers(ciphers []string) ([]uint16, error) {
	var suites []uint16

	for _, cipher := range ciphers {
		v, ok := tlsCipherMap[cipher]
		if !ok {
			log.Error().Msgf("ParseCiphers(): unsupported cipher %v", cipher)
			return nil, ErrInvalidCipher
		}
		suites = append(suites, v)
	}
	return suites, nil
}

// ParseTLSVersion returns a `uint16` by received version string key that represents tls version from crypto/tls,
// or 0 if version is invalid
func ParseTLSVersion(version string) (uint16, error) {
	if v, ok := tlsVersionMap[version]; ok {
		return v, nil
	}
	log.Error().Msgf("ParseTLSVersion(): invalid tls version %q", version)
	return 0, ErrInvalidTlsVersion
}

// IsEmpty returns true if credential source is empty
func (c TlsKeyCredential) IsEmpty() bool {
	return strings.TrimSpace(c.Password) == "" &&
		strings.TrimSpace(c.PasswordEnvVar) == "" &&
		strings.TrimSpace(c.PasswordFile) == ""
}

// Fetch retrieve the contents of the credential
func (c TlsKeyCredential) Fetch() (string, error) {
	plainText := strings.TrimSpace(c.Password)
	if plainText == "" {
		// attempt to read env var, if set
		envVar := strings.TrimSpace(c.PasswordEnvVar)
		if envVar == "" {
			// attempt to read secrets file, if set
			secretsFile := c.PasswordFile
			if secretsFile == "" {
				plainText = ""
			} else {
				// read secrets
				var err error
				if plainText, err = fs.ReadString(secretsFile); err != nil {
					return "", err
				}
			}
		} else {
			// read from env var and clear it
			plainText = env.GetEnvVar(envVar)
			_ = env.SetEnvVar(envVar, "")
		}
	}
	return plainText, nil
}
