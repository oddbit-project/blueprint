package nats

import (
	"bufio"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	tlsProvider "github.com/oddbit-project/blueprint/provider/tls"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeTLSMode int

const (
	fakePlaintext fakeTLSMode = iota
	fakeTLSAvailable
	fakeTLSRequired
)

// fakeServerResult is what the fake server saw from the client
type fakeServerResult struct {
	plainConnect bool      // a CONNECT line arrived before any TLS handshake
	tlsConnect   bool      // a CONNECT line arrived over TLS
	closedAt     time.Time // when the server closed the connection
}

// selfSignedCert returns the server certificate and the path of a PEM file holding it,
// for the client to trust as its CA
func selfSignedCert(t *testing.T) (tls.Certificate, string) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "127.0.0.1"},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)
	caFile := filepath.Join(t.TempDir(), "ca.pem")
	require.NoError(t, os.WriteFile(caFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600))
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}, caFile
}

// fakeServer speaks enough of the NATS protocol to complete one handshake. In the
// TLS modes it advertises TLS in INFO and expects the client to start a TLS handshake
// before CONNECT, as nats-server does. connectURLs (bare host:port) are advertised in
// INFO as other cluster members, for the client to discover.
func fakeServer(t *testing.T, mode fakeTLSMode, connectURLs ...string) (string, string, <-chan fakeServerResult) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })
	cert, caFile := selfSignedCert(t)
	done := make(chan fakeServerResult, 1)

	go func() {
		var res fakeServerResult
		defer func() { done <- res }()
		c, err := ln.Accept()
		if err != nil {
			return
		}
		defer func() {
			res.closedAt = time.Now()
			_ = c.Close()
		}()
		_ = c.SetDeadline(time.Now().Add(5 * time.Second))

		info := `{"server_id":"x","version":"2.10.0","proto":1,"max_payload":1048576,"auth_required":true`
		switch mode {
		case fakeTLSAvailable:
			info += `,"tls_available":true`
		case fakeTLSRequired:
			info += `,"tls_required":true`
		}
		if len(connectURLs) > 0 {
			info += `,"connect_urls":["` + strings.Join(connectURLs, `","`) + `"]`
		}
		if _, err := c.Write([]byte("INFO " + info + "}\r\n")); err != nil {
			return
		}

		conn := c
		if mode != fakePlaintext {
			// a plaintext CONNECT here would fail the handshake; peek for it first
			br := bufio.NewReader(c)
			first, err := br.Peek(7)
			if err == nil && string(first) == "CONNECT" {
				res.plainConnect = true
				return
			}
			tc := tls.Server(&peekedConn{Conn: c, r: br}, &tls.Config{Certificates: []tls.Certificate{cert}})
			if err := tc.Handshake(); err != nil {
				return
			}
			conn = tc
		}

		r := bufio.NewReader(conn)
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				return
			}
			if strings.HasPrefix(line, "CONNECT") {
				if mode == fakePlaintext {
					res.plainConnect = true
				} else {
					res.tlsConnect = true
				}
			}
			if strings.HasPrefix(line, "PING") {
				_, _ = conn.Write([]byte("PONG\r\n"))
				return
			}
		}
	}()
	return "nats://" + ln.Addr().String(), caFile, done
}

// peekedConn reads through the bufio.Reader used to peek, so no bytes are lost
type peekedConn struct {
	net.Conn
	r *bufio.Reader
}

func (p *peekedConn) Read(b []byte) (int, error) { return p.r.Read(b) }

func tlsProducerConfig(url, caFile string, enable bool) *ProducerConfig {
	cfg := &ProducerConfig{
		URL:                     url,
		Subject:                 "x",
		AuthType:                AuthTypeBasic,
		Username:                "u",
		DefaultCredentialConfig: StringPasswordConfig("p"),
	}
	// TLS settings without tlsEnable are rejected (#110)
	if enable {
		cfg.ClientConfig = tlsProvider.ClientConfig{TLSEnable: true, TLSCA: caFile}
	}
	return cfg
}

// serverResult waits for the fake server's result, failing instead of hanging when the
// client never connected
func serverResult(t *testing.T, done <-chan fakeServerResult) fakeServerResult {
	t.Helper()
	select {
	case res := <-done:
		return res
	case <-time.After(10 * time.Second):
		t.Fatal("the client never connected to the fake server")
		return fakeServerResult{}
	}
}

func TestIssue104_TLSEnabledRefusesPlaintextServer(t *testing.T) {
	url, caFile, done := fakeServer(t, fakePlaintext)
	p, err := NewProducer(tlsProducerConfig(url, caFile, true), nil)
	if p != nil {
		p.Disconnect()
	}
	assert.True(t, errors.Is(err, nats.ErrSecureConnWanted), "connected to a server without TLS: err=%v", err)
	res := serverResult(t, done)
	assert.False(t, res.plainConnect, "credentials were sent in plaintext")
}

func TestIssue104_TLSEnabledUpgradesWhenServerOffersTLS(t *testing.T) {
	url, caFile, done := fakeServer(t, fakeTLSAvailable)
	p, err := NewProducer(tlsProducerConfig(url, caFile, true), nil)
	if p != nil {
		p.Disconnect()
	}
	res := serverResult(t, done)
	assert.False(t, res.plainConnect, "credentials were sent in plaintext to a server offering TLS")
	assert.True(t, res.tlsConnect, "no CONNECT over TLS (err=%v)", err)
}

func TestIssue104_TLSDisabledPlaintextServerConnects(t *testing.T) {
	url, caFile, done := fakeServer(t, fakePlaintext)
	p, err := NewProducer(tlsProducerConfig(url, caFile, false), nil)
	require.NoError(t, err)
	p.Disconnect()
	assert.True(t, serverResult(t, done).plainConnect)
}

func TestIssue104_TLSEnabledTLSRequiredServerConnects(t *testing.T) {
	url, caFile, done := fakeServer(t, fakeTLSRequired)
	p, err := NewProducer(tlsProducerConfig(url, caFile, true), nil)
	require.NoError(t, err)
	p.Disconnect()
	res := serverResult(t, done)
	assert.True(t, res.tlsConnect)
	assert.False(t, res.plainConnect)
}
