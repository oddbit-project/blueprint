/*
 TLS Certificate Loader

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
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"github.com/oddbit-project/blueprint/utils"
	"github.com/rs/zerolog/log"
	"go.step.sm/crypto/pemutil"
	"os"
)

const (
	ErrCertNotFound    = utils.Error("could not load certificate file")
	ErrInvalidPEM      = utils.Error("could not parse PEM certificate")
	ErrKeyNotFound     = utils.Error("could not load private key file")
	ErrKeyError        = utils.Error("failed to decode private key")
	ErrMissingPassword = utils.Error("missing password for encrypted private key")
	ErrDecryptError    = utils.Error("private key decryption error")
	ErrInvalidCert     = utils.Error("failed to load cert/key pair")
)

// LoadTLSCertPool loads a certificate pool with the certificates from the specified files.
// It takes a slice of certificate file names as input.
//
// Each certificate file is read using os.ReadFile. If there is an error reading the file, an error is returned with ErrCertNotFound.
//
// The content of each certificate file is appended to the certificate pool using pool.AppendCertsFromPEM.
// If parsing the PEM certificate fails, an error is logged and the certificate
func LoadTLSCertPool(certFiles []string) (*x509.CertPool, error) {
	pool := x509.NewCertPool()
	for _, certFile := range certFiles {
		cert, err := os.ReadFile(certFile)
		if err != nil {
			log.Error().Msgf("LoadTLSCertPool(): failed to read certFile '%s'; %v", certFile, err)
			return nil, ErrCertNotFound
		}
		if !pool.AppendCertsFromPEM(cert) {
			log.Error().Msgf("LoadTLSCertPool(): could not parse PEM cerificate from '%s'; %v", certFile, err)
			return nil, ErrInvalidPEM
		}
	}
	return pool, nil
}

// LoadTLSCertificate loads a TLS certificate into the provided tls.Config.
//
// It takes the following parameters:
// - config: Pointer to a tls.Config where the certificate will be loaded.
// - certFile: Path to the certificate file.
// - keyFile: Path to the private key file.
// - password: Password to decrypt the private key file (if encrypted).
//
// The function reads the certificate file and private key file using os.ReadFile.
// If there is an error reading any of the files, an error is returned.
//
// The private key is then decoded using pem.Decode.
// If the private key is encrypted and no password is supplied, an error is returned.
//
// Once the private key is decoded, it is used to load the certificate and private key pair using tls.X509KeyPair.
// If the certificate and private key pair is invalid, an error is returned.
//
// The loaded certificate is then assigned to the config.Certificates field.
//
// Example:
//
// config := &tls.Config{}
// err := LoadTLSCertificate(config, "path/to/cert.pem", "path/to/key.pem", "password")
//
//	if err != nil {
//	    log.Fatal(err)
//	}
//
// // TLS configuration with loaded certificate is ready to use.
func LoadTLSCertificate(config *tls.Config, certFile, keyFile, password string) error {

	certBytes, err := os.ReadFile(certFile)
	if err != nil {
		log.Error().Msgf("LoadTLSCertificates(): failed to read certFile '%s'; %v", certFile, err)
		return ErrCertNotFound
	}

	keyBytes, err := os.ReadFile(keyFile)
	if err != nil {
		log.Error().Msgf("LoadTLSCertificates(): failed to read keyFile '%s'; %v", keyFile, err)
		return ErrKeyNotFound
	}

	keyPEMBlock, _ := pem.Decode(keyBytes)
	if keyPEMBlock == nil {
		log.Error().Msg("LoadTLSCertificates(): failed to decode private key: no PEM data found")
		return ErrKeyError
	}

	var cert tls.Certificate
	if keyPEMBlock.Type == "ENCRYPTED PRIVATE KEY" {
		if password == "" {
			log.Error().Msgf("LoadTLSCertificates(): encrypted private key '%s', but no password supplied", keyFile)
			return ErrMissingPassword
		}
		rawDecryptedKey, err := pemutil.DecryptPKCS8PrivateKey(keyPEMBlock.Bytes, []byte(password))
		if err != nil {
			log.Error().Msgf("LoadTLSCertificates(): failed to decrypt PKCS#8 private key: %v", err)
			return ErrDecryptError
		}
		decryptedKey, err := x509.ParsePKCS8PrivateKey(rawDecryptedKey)
		if err != nil {
			log.Error().Msgf("LoadTLSCertificates(): failed to parse decrypted PKCS#8 private key: %v", err)
			return ErrDecryptError
		}
		privateKey, ok := decryptedKey.(*rsa.PrivateKey)
		if !ok {
			log.Error().Msg("LoadTLSCertificates(): decrypted key is not a RSA private key")
			return ErrDecryptError
		}
		cert, err = tls.X509KeyPair(certBytes, pem.EncodeToMemory(&pem.Block{Type: keyPEMBlock.Type, Bytes: x509.MarshalPKCS1PrivateKey(privateKey)}))
		if err != nil {
			log.Error().Msgf("LoadTLSCertificates(): failed to load cert/key pair: %v", err)
			return ErrInvalidCert
		}
	} else {
		cert, err = tls.X509KeyPair(certBytes, keyBytes)
		if err != nil {
			log.Error().Msgf("LoadTLSCertificates(): failed to load cert/key pair: %v", err)
			return ErrInvalidCert
		}
	}
	config.Certificates = []tls.Certificate{cert}
	return nil
}
