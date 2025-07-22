/*
 TLS Server Provider

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
	"crypto/x509"
	"github.com/oddbit-project/blueprint/utils"
	"github.com/rs/zerolog/log"
	"slices"
	"time"
)

const (
	TLSMinVersionDefault = tls.VersionTLS13 // Use TLS 1.3 by default for better security
	ErrInvalidPeerCert   = utils.Error("invalid peer certificate")
	ErrForbiddenDNS      = utils.Error("peer certificate not allowed in DNS name list")
	ErrExpiredCert       = utils.Error("peer certificate has expired")
)

// ServerConfig represents the standard server TLS config.
type ServerConfig struct {
	TLSCert            string   `json:"tlsCert"`
	TLSKey             string   `json:"tlsKey"`
	TlsKeyCredential            // TLS key password
	TLSAllowedCACerts  []string `json:"tlsAllowedCACerts"`
	TLSCipherSuites    []string `json:"tlsCipherSuites"`
	TLSMinVersion      string   `json:"tlsMinVersion"`
	TLSMaxVersion      string   `json:"tlsMaxVersion"`
	TLSAllowedDNSNames []string `json:"tlsAllowedDNSNames"`
	TLSEnable          bool     `json:"tlsEnable"`
}

// TLSConfig returns a tls.Config, may be nil without error if TLS is not configured.
func (c *ServerConfig) TLSConfig() (*tls.Config, error) {
	if !c.TLSEnable {
		return nil, nil
	}

	tlsConfig := &tls.Config{}
	// empty config
	if c.TLSCert == "" && c.TLSKey == "" && len(c.TLSAllowedCACerts) == 0 {
		return tlsConfig, nil
	}

	if len(c.TLSAllowedCACerts) != 0 {
		pool, err := LoadTLSCertPool(c.TLSAllowedCACerts)
		if err != nil {
			return nil, err
		}
		tlsConfig.ClientCAs = pool
		tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
	}

	if c.TLSCert != "" && c.TLSKey != "" {
		err := LoadTLSCertificate(tlsConfig, c.TLSCert, c.TLSKey, c.TlsKeyCredential)
		if err != nil {
			return nil, err
		}
	}

	if len(c.TLSCipherSuites) != 0 {
		cipherSuites, err := ParseCiphers(c.TLSCipherSuites)
		if err != nil {
			return nil, err
		}
		tlsConfig.CipherSuites = cipherSuites
	} else {
		tlsConfig.CipherSuites = defaultCipherSuites
	}

	if c.TLSMaxVersion != "" {
		version, err := ParseTLSVersion(c.TLSMaxVersion)
		if err != nil {
			return nil, err
		}
		tlsConfig.MaxVersion = version
	}

	// Explicitly and consistently set the minimal accepted version using the
	// defined default. We use this setting for both clients and servers
	// instead of relying on Golang's default that is different for clients
	// and servers and might change over time.
	tlsConfig.MinVersion = TLSMinVersionDefault
	if c.TLSMinVersion != "" {
		version, err := ParseTLSVersion(c.TLSMinVersion)
		if err != nil {
			return nil, err
		}
		tlsConfig.MinVersion = version
	}

	if tlsConfig.MinVersion != 0 && tlsConfig.MaxVersion != 0 && tlsConfig.MinVersion > tlsConfig.MaxVersion {
		log.Error().Msgf("TLSConfig(): tls min version %q can't be greater than tls max version %q", tlsConfig.MinVersion, tlsConfig.MaxVersion)
		return nil, ErrInvalidTlsVersion
	}

	// Since clientAuth is tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
	// there must be certs to validate.
	if len(c.TLSAllowedCACerts) > 0 && len(c.TLSAllowedDNSNames) > 0 {
		tlsConfig.VerifyPeerCertificate = c.verifyPeerCertificate
	}

	return tlsConfig, nil
}

func (c *ServerConfig) verifyPeerCertificate(rawCerts [][]byte, _ [][]*x509.Certificate) error {
	// The certificate chain is client + intermediate + root.
	// Let's review the client certificate.
	cert, err := x509.ParseCertificate(rawCerts[0])
	if err != nil {
		log.Error().Msgf("could not validate peer certificate: %v", err)
		return ErrInvalidPeerCert
	}

	// Check certificate expiration
	now := time.Now()
	if now.Before(cert.NotBefore) || now.After(cert.NotAfter) {
		log.Error().
			Time("notBefore", cert.NotBefore).
			Time("notAfter", cert.NotAfter).
			Time("now", now).
			Msg("peer certificate has expired or is not yet valid")
		return ErrExpiredCert
	}

	// Check DNS names
	if len(c.TLSAllowedDNSNames) > 0 {
		for _, name := range cert.DNSNames {
			if !slices.Contains(c.TLSAllowedDNSNames, name) {
				return nil
			}
		}
		log.Error().Msgf("peer certificate not in allowed DNS Name list: %v", cert.DNSNames)
		return ErrForbiddenDNS
	}

	return nil
}
