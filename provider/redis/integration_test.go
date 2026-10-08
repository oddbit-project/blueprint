package redis

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/oddbit-project/blueprint/provider/kv"
)

var _ kv.KV = (*Client)(nil)
var _ kv.AtomicSetter = (*Client)(nil)

const testRedisImage = "redis:7-alpine"

// RedisIntegrationTestSuite runs the client against a Redis testcontainer
type RedisIntegrationTestSuite struct {
	suite.Suite
	ctx       context.Context
	container testcontainers.Container
	address   string
	client    *Client
}

func (s *RedisIntegrationTestSuite) SetupSuite() {
	s.ctx = context.Background()

	req := testcontainers.ContainerRequest{
		Image:        testRedisImage,
		ExposedPorts: []string{"6379/tcp"},
		WaitingFor:   wait.ForLog("Ready to accept connections").WithStartupTimeout(60 * time.Second),
	}

	var err error
	s.container, err = testcontainers.GenericContainer(s.ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(s.T(), err, "Failed to start Redis container")

	host, err := s.container.Host(s.ctx)
	require.NoError(s.T(), err)
	port, err := s.container.MappedPort(s.ctx, "6379/tcp")
	require.NoError(s.T(), err)
	s.address = fmt.Sprintf("%s:%s", host, port.Port())
}

func (s *RedisIntegrationTestSuite) TearDownSuite() {
	if s.container != nil {
		if err := s.container.Terminate(s.ctx); err != nil {
			s.T().Logf("Warning: failed to terminate container: %v", err)
		}
	}
}

func (s *RedisIntegrationTestSuite) SetupTest() {
	cfg := NewConfig()
	cfg.Address = s.address
	cfg.KeyPrefix = "test:"

	var err error
	s.client, err = NewClient(cfg)
	s.Require().NoError(err)
	s.Require().NoError(s.client.Connect())
	s.Require().NoError(s.client.Redis.FlushDB(s.ctx).Err())
}

func (s *RedisIntegrationTestSuite) TearDownTest() {
	if s.client != nil {
		_ = s.client.Close()
	}
}

func (s *RedisIntegrationTestSuite) TestSetGetDelete() {
	s.Require().NoError(s.client.Set("key", []byte("value")))

	v, err := s.client.Get("key")
	s.Require().NoError(err)
	s.Equal([]byte("value"), v)

	s.Require().NoError(s.client.Delete("key"))
	v, err = s.client.Get("key")
	s.Require().NoError(err)
	s.Nil(v)
}

func (s *RedisIntegrationTestSuite) TestPruneKeepsKeys() {
	s.Require().NoError(s.client.SetTTL("session", []byte("data"), time.Hour))
	s.Require().NoError(s.client.Redis.Set(s.ctx, "unrelated", "other", 0).Err())

	s.Require().NoError(s.client.Prune())

	v, err := s.client.Get("session")
	s.Require().NoError(err)
	s.Equal([]byte("data"), v, "Prune must not delete live keys")

	unrelated, err := s.client.Redis.Get(s.ctx, "unrelated").Result()
	s.Require().NoError(err, "Prune must not delete keys outside the client prefix")
	s.Equal("other", unrelated)
}

func (s *RedisIntegrationTestSuite) TestSetNX() {
	tests := []struct {
		name string
		key  string
		ttl  time.Duration
		want bool
	}{
		{"new key", "nonce", time.Hour, true},
		{"existing key", "nonce", time.Hour, false},
		{"new key without expiry", "other", 0, true},
	}
	for _, tt := range tests {
		s.Run(tt.name, func() {
			ok, err := s.client.SetNX(tt.key, []byte("1"), tt.ttl)
			s.Require().NoError(err)
			s.Equal(tt.want, ok)
		})
	}

	ttl, err := s.client.Redis.TTL(s.ctx, "test:nonce").Result()
	s.Require().NoError(err)
	s.Greater(ttl, time.Duration(0), "SetNX must apply the ttl and the key prefix")
}

func TestRedisIntegrationSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	suite.Run(t, new(RedisIntegrationTestSuite))
}

// generateTestCert returns a self-signed cert/key valid for localhost
func generateTestCert(t *testing.T) (certPEM, keyPEM []byte) {
	t.Helper()

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	tmpl := x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "redis-test"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
		DNSNames:              []string{"localhost"},
		IPAddresses:           []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
	}

	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &priv.PublicKey, priv)
	require.NoError(t, err)
	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})

	keyDER, err := x509.MarshalPKCS8PrivateKey(priv)
	require.NoError(t, err)
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	return certPEM, keyPEM
}

func TestRedisTLSIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	ctx := context.Background()

	certPEM, keyPEM := generateTestCert(t)
	caPath := filepath.Join(t.TempDir(), "ca.crt")
	require.NoError(t, os.WriteFile(caPath, certPEM, 0o600))

	req := testcontainers.ContainerRequest{
		Image:        testRedisImage,
		ExposedPorts: []string{"6379/tcp"},
		Files: []testcontainers.ContainerFile{
			{Reader: bytes.NewReader(certPEM), ContainerFilePath: "/certs/server.crt", FileMode: 0o644},
			{Reader: bytes.NewReader(keyPEM), ContainerFilePath: "/certs/server.key", FileMode: 0o644},
		},
		Cmd: []string{
			"redis-server",
			"--port", "0",
			"--tls-port", "6379",
			"--tls-cert-file", "/certs/server.crt",
			"--tls-key-file", "/certs/server.key",
			"--tls-auth-clients", "no",
		},
		WaitingFor: wait.ForLog("Ready to accept connections").WithStartupTimeout(60 * time.Second),
	}
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err, "Failed to start TLS Redis container")
	defer func() { _ = container.Terminate(ctx) }()

	host, err := container.Host(ctx)
	require.NoError(t, err)
	port, err := container.MappedPort(ctx, "6379/tcp")
	require.NoError(t, err)
	address := fmt.Sprintf("%s:%s", host, port.Port())

	tests := []struct {
		name      string
		tlsEnable bool
		wantErr   bool
	}{
		{"TLS enabled", true, false},
		{"TLS disabled", false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := NewConfig()
			cfg.Address = address
			cfg.TimeoutSeconds = 5
			cfg.TLSEnable = tt.tlsEnable
			// TLS settings without tlsEnable are rejected by Validate (#116)
			if tt.tlsEnable {
				cfg.TLSCA = caPath
			}

			client, err := NewClient(cfg)
			require.NoError(t, err)
			defer func() { _ = client.Close() }()

			err = client.Connect()
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.NoError(t, client.Set("key", []byte("value")))
			v, err := client.Get("key")
			require.NoError(t, err)
			assert.Equal(t, []byte("value"), v)
		})
	}
}
