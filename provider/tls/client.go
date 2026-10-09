/*
 TLS Redis Provider

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
)

// ErrTLSNotEnabled is returned by ClientConfig.ValidateEnabled for TLS settings given
// without TLSEnable
const ErrTLSNotEnabled = utils.Error("TLS settings require tlsEnable")

// ErrTLSIncompleteKeyPair is returned by ClientConfig.ValidateEnabled and
// ServerConfig.ValidateKeyPair for a certificate without its key, a key without its
// certificate, or a key password without both
const ErrTLSIncompleteKeyPair = utils.Error("TLS certificate requires both tlsCert and tlsKey")

// ClientConfig represents the configuration for a tls client configuration
type ClientConfig struct {
	TLSCA                 string `json:"tlsCa"`
	TLSCert               string `json:"tlsCert"`
	TLSKey                string `json:"tlsKey"`
	TlsKeyCredential             // tls key password
	TLSEnable             bool   `json:"tlsEnable"`
	TLSInsecureSkipVerify bool   `json:"tlsInsecureSkipVerify"`
}

// ValidateEnabled rejects TLS settings that TLSConfig() would silently ignore:
//   - any TLS setting given without TLSEnable returns ErrTLSNotEnabled, as the connection
//     would go without the configured CA, client certificate or verification setting,
//     usually in plaintext;
//   - with TLSEnable, a TLSCert without TLSKey, a TLSKey without TLSCert, or a key password
//     without both returns ErrTLSIncompleteKeyPair, as the connection would go without the
//     client certificate
func (c *ClientConfig) ValidateEnabled() error {
	key := c.TlsKeyCredential
	keyPassword := key.Password != "" || key.PasswordEnvVar != "" || key.PasswordFile != ""
	if !c.TLSEnable && (c.TLSCA != "" || c.TLSCert != "" || c.TLSKey != "" || c.TLSInsecureSkipVerify || keyPassword) {
		return ErrTLSNotEnabled
	}
	// past the check above, any certificate, key or key password is set with TLSEnable
	if (c.TLSCert == "") != (c.TLSKey == "") || (keyPassword && c.TLSCert == "") {
		return ErrTLSIncompleteKeyPair
	}
	return nil
}

// TLSConfig returns a tls.Config{} struct from the ClientConfig
func (c *ClientConfig) TLSConfig() (*tls.Config, error) {
	if !c.TLSEnable {
		return nil, nil
	}

	tlsConfig := &tls.Config{
		InsecureSkipVerify: c.TLSInsecureSkipVerify,
	}

	var err error
	if c.TLSCA != "" {
		tlsConfig.RootCAs, err = LoadTLSCertPool([]string{c.TLSCA})
		if err != nil {
			return nil, err
		}
	}

	if c.TLSCert != "" && c.TLSKey != "" {
		err = LoadTLSCertificate(tlsConfig, c.TLSCert, c.TLSKey, c.TlsKeyCredential)
		if err != nil {
			return nil, err
		}
	}

	return tlsConfig, nil
}
