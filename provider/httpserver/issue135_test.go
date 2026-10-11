package httpserver

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	tlsProvider "github.com/oddbit-project/blueprint/provider/tls"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// selfSigned135 returns a self-signed certificate and the path of its PEM file
func selfSigned135(t *testing.T) (tls.Certificate, string) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	tpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		DNSNames:              []string{"localhost"},
	}
	der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &key.PublicKey, key)
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "ca.pem")
	require.NoError(t, os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600))
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}, path
}

// freeAddr135 returns a loopback address with a free port
func freeAddr135(t *testing.T) string {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := l.Addr().String()
	require.NoError(t, l.Close())
	return addr
}

// newServer135 builds a server for the TLS settings, listening on a free loopback port
func newServer135(t *testing.T, tc tlsProvider.ServerConfig) *Server {
	cfg := NewServerConfig()
	cfg.ServerConfig = tc
	srv, err := NewServer(cfg, nil)
	require.NoError(t, err)
	srv.Server.Addr = freeAddr135(t)
	return srv
}

// startResult135 runs Start and returns its error, or nil while it is still serving after
// check (when given) has run against it; the server is shut down before returning
func startResult135(t *testing.T, srv *Server, check func(addr string) error) (startErr, checkErr error) {
	res := make(chan error, 1)
	go func() { res <- srv.Start() }()
	// wait until Start returns or the listener accepts connections
	deadline := time.Now().Add(5 * time.Second)
	for {
		select {
		case err := <-res:
			return err, nil
		default:
		}
		if c, err := net.DialTimeout("tcp", srv.Server.Addr, 100*time.Millisecond); err == nil {
			_ = c.Close()
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("Start neither returned nor started listening")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if check != nil {
		checkErr = check(srv.Server.Addr)
	}
	require.NoError(t, srv.Shutdown(context.Background()))
	return <-res, checkErr
}

func tlsHandshake135(addr string) error {
	c, err := tls.Dial("tcp", addr, &tls.Config{InsecureSkipVerify: true})
	if err != nil {
		return err
	}
	return c.Close()
}

func TestIssue135_StartTLSEnableNoCertificate(t *testing.T) {
	srv := newServer135(t, tlsProvider.ServerConfig{TLSEnable: true})
	err, _ := startResult135(t, srv, nil)
	assert.ErrorIs(t, err, ErrTLSNoCertificate)
}

func TestIssue135_StartTLSEnableAllowedCAsNoCertificate(t *testing.T) {
	_, caPath := selfSigned135(t)
	srv := newServer135(t, tlsProvider.ServerConfig{TLSEnable: true, TLSAllowedCACerts: []string{caPath}})
	err, _ := startResult135(t, srv, nil)
	assert.ErrorIs(t, err, ErrTLSNoCertificate)
}

// Guards: a certificate supplied on Server.TLSConfig after NewServer still serves TLS
func TestIssue135_StartWithCertificateSetAfterNewServer(t *testing.T) {
	cert, _ := selfSigned135(t)
	set := map[string]func(*tls.Config){
		"GetCertificate": func(c *tls.Config) {
			c.GetCertificate = func(*tls.ClientHelloInfo) (*tls.Certificate, error) { return &cert, nil }
		},
		"Certificates": func(c *tls.Config) { c.Certificates = []tls.Certificate{cert} },
		"GetConfigForClient": func(c *tls.Config) {
			c.GetConfigForClient = func(*tls.ClientHelloInfo) (*tls.Config, error) {
				return &tls.Config{Certificates: []tls.Certificate{cert}}, nil
			}
		},
	}
	for name, apply := range set {
		t.Run(name, func(t *testing.T) {
			srv := newServer135(t, tlsProvider.ServerConfig{TLSEnable: true})
			apply(srv.Server.TLSConfig)
			startErr, handshakeErr := startResult135(t, srv, tlsHandshake135)
			assert.NoError(t, startErr)
			assert.NoError(t, handshakeErr)
		})
	}
}

// Guard: a plaintext server still starts
func TestIssue135_StartPlaintext(t *testing.T) {
	srv := newServer135(t, tlsProvider.ServerConfig{})
	startErr, dialErr := startResult135(t, srv, func(addr string) error {
		c, err := net.Dial("tcp", addr)
		if err != nil {
			return err
		}
		return c.Close()
	})
	assert.NoError(t, startErr)
	assert.NoError(t, dialErr)
}
