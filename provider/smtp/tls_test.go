package smtp_test

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/oddbit-project/blueprint/provider/smtp"
	tlsProvider "github.com/oddbit-project/blueprint/provider/tls"
	"github.com/oddbit-project/blueprint/types/duration"
	"github.com/stretchr/testify/require"
	gomail "github.com/wneessen/go-mail"
)

// testCert is a self-signed certificate, usable as server certificate, client
// certificate and CA bundle
type testCert struct {
	cert     tls.Certificate
	pool     *x509.CertPool
	certFile string
	keyFile  string
}

// selfSignedCert generates a self-signed certificate valid for 127.0.0.1 and writes
// the PEM encoded certificate and key to disk
func selfSignedCert(t *testing.T) testCert {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	template := x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "127.0.0.1"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
		DNSNames:              []string{"localhost"},
	}

	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	require.NoError(t, err)

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})

	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	require.NoError(t, err)

	pool := x509.NewCertPool()
	require.True(t, pool.AppendCertsFromPEM(certPEM))

	dir := t.TempDir()
	certFile := filepath.Join(dir, "cert.pem")
	keyFile := filepath.Join(dir, "key.pem")
	require.NoError(t, os.WriteFile(certFile, certPEM, 0o600))
	require.NoError(t, os.WriteFile(keyFile, keyPEM, 0o600))

	return testCert{cert: cert, pool: pool, certFile: certFile, keyFile: keyFile}
}

// serverOpts configures the test SMTP server
type serverOpts struct {
	cert      *testCert // server certificate; required for implicit or STARTTLS
	implicit  bool      // the listener itself is TLS (SMTPS)
	starttls  bool      // advertise STARTTLS
	auth      bool      // advertise AUTH PLAIN
	clientCer bool      // require a client certificate
}

// smtpStats records what the test SMTP server saw
type smtpStats struct {
	connections atomic.Int32
	delivered   atomic.Int32
	encrypted   atomic.Int32
	authLine    atomic.Value // last AUTH command received
}

// startSMTPServer starts a minimal SMTP server. Recipients containing "reject" are refused
func startSMTPServer(t *testing.T, opts serverOpts) (int, *smtpStats) {
	t.Helper()

	var ln net.Listener
	var err error
	if opts.implicit {
		require.NotNil(t, opts.cert)
		ln, err = tls.Listen("tcp", "127.0.0.1:0", serverTLSConfig(opts))
	} else {
		ln, err = net.Listen("tcp", "127.0.0.1:0")
	}
	require.NoError(t, err)
	t.Cleanup(func() { ln.Close() })

	stats := &smtpStats{}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			stats.connections.Add(1)
			if opts.implicit {
				stats.encrypted.Add(1)
			}
			go serveSMTP(conn, opts, stats)
		}
	}()

	port, err := strconv.Atoi(strings.Split(ln.Addr().String(), ":")[1])
	require.NoError(t, err)
	return port, stats
}

func serverTLSConfig(opts serverOpts) *tls.Config {
	cfg := &tls.Config{Certificates: []tls.Certificate{opts.cert.cert}}
	if opts.clientCer {
		cfg.ClientAuth = tls.RequireAndVerifyClientCert
		cfg.ClientCAs = opts.cert.pool
	}
	return cfg
}

func serveSMTP(conn net.Conn, opts serverOpts, stats *smtpStats) {
	defer conn.Close()

	r := bufio.NewReader(conn)
	write := func(s string) bool {
		_, err := conn.Write([]byte(s + "\r\n"))
		return err == nil
	}

	if !write("220 localhost ESMTP ready") {
		return
	}

	started := opts.implicit
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		switch strings.ToUpper(fields[0]) {
		case "EHLO", "HELO":
			extensions := []string{"250-localhost"}
			if opts.starttls && !started {
				extensions = append(extensions, "250-STARTTLS")
			}
			if opts.auth {
				extensions = append(extensions, "250-AUTH PLAIN")
			}
			extensions = append(extensions, "250 SIZE 10240000")
			if !write(strings.Join(extensions, "\r\n")) {
				return
			}
		case "STARTTLS":
			if !write("220 ready to start TLS") {
				return
			}
			tlsConn := tls.Server(conn, serverTLSConfig(opts))
			if err = tlsConn.Handshake(); err != nil {
				return
			}
			conn = tlsConn
			r = bufio.NewReader(conn)
			started = true
			stats.encrypted.Add(1)
		case "AUTH":
			stats.authLine.Store(strings.TrimSpace(line))
			if !write("235 2.7.0 authentication succeeded") {
				return
			}
		case "RCPT":
			if strings.Contains(line, "reject") {
				if !write("550 no such user") {
					return
				}
				continue
			}
			if !write("250 OK") {
				return
			}
		case "DATA":
			if !write("354 send message") {
				return
			}
			for {
				body, err := r.ReadString('\n')
				if err != nil {
					return
				}
				if body == ".\r\n" {
					break
				}
			}
			stats.delivered.Add(1)
			if !write("250 OK") {
				return
			}
		case "QUIT":
			write("221 bye")
			return
		default:
			if !write("250 OK") {
				return
			}
		}
	}
}

func tlsTestConfig(port int) *smtp.Config {
	return &smtp.Config{
		Host:         "127.0.0.1",
		Port:         port,
		AuthType:     "noauth",
		SSLOnConnect: true,
		From:         "no-reply@example.com",
		ClientConfig: tlsProvider.ClientConfig{
			TLSEnable: true,
		},
	}
}

func sendTestMessage(t *testing.T, cfg *smtp.Config) error {
	t.Helper()

	mailer, err := smtp.NewMailer(cfg)
	require.NoError(t, err)

	msg, err := mailer.NewMessage([]string{"receiver@example.com"}, "subject",
		smtp.WithFrom(cfg.From),
		smtp.WithBody("body", ""),
	)
	require.NoError(t, err)

	return mailer.Send(msg)
}

// Self-signed certificates are accepted when verification is skipped
func TestTLSInsecureSkipVerify(t *testing.T) {
	cert := selfSignedCert(t)
	port, _ := startSMTPServer(t, serverOpts{cert: &cert, implicit: true})
	cfg := tlsTestConfig(port)
	cfg.TLSInsecureSkipVerify = true

	require.NoError(t, sendTestMessage(t, cfg))
}

// Self-signed certificates are rejected when verification is enabled
func TestTLSVerifyRejectsSelfSigned(t *testing.T) {
	cert := selfSignedCert(t)
	port, _ := startSMTPServer(t, serverOpts{cert: &cert, implicit: true})
	cfg := tlsTestConfig(port)

	err := sendTestMessage(t, cfg)
	require.Error(t, err)
	require.ErrorIs(t, err, smtp.ErrSMTPServer)
}

// Self-signed certificates are accepted when supplied as CA
func TestTLSWithCA(t *testing.T) {
	cert := selfSignedCert(t)
	port, _ := startSMTPServer(t, serverOpts{cert: &cert, implicit: true})
	cfg := tlsTestConfig(port)
	cfg.TLSCA = cert.certFile

	require.NoError(t, sendTestMessage(t, cfg))
}

// A missing CA file is reported as a TLS configuration error
func TestTLSInvalidCA(t *testing.T) {
	cfg := tlsTestConfig(1025)
	cfg.TLSCA = filepath.Join(t.TempDir(), "missing.pem")

	_, err := smtp.NewMailer(cfg)
	require.ErrorIs(t, err, smtp.ErrInvalidTLSConfig)
}

// Plaintext servers are usable with the "none" policy
func TestTLSPolicyNone(t *testing.T) {
	port, _ := startSMTPServer(t, serverOpts{})
	cfg := tlsTestConfig(port)
	cfg.SSLOnConnect = false
	cfg.TLSEnable = false
	cfg.TLSPolicy = smtp.TLSPolicyNone

	require.NoError(t, sendTestMessage(t, cfg))
}

// STARTTLS is required by default, and a server without it fails the transaction
func TestTLSPolicyMandatory(t *testing.T) {
	port, _ := startSMTPServer(t, serverOpts{})
	cfg := tlsTestConfig(port)
	cfg.SSLOnConnect = false
	cfg.TLSEnable = false

	err := sendTestMessage(t, cfg)
	require.Error(t, err)
	require.ErrorIs(t, err, smtp.ErrSMTPServer)
}

func TestInvalidTLSPolicy(t *testing.T) {
	cfg := tlsTestConfig(1025)
	cfg.TLSPolicy = "sometimes"

	_, err := smtp.NewMailer(cfg)
	require.Equal(t, smtp.ErrInvalidTLSPolicy, err)
}

func TestInvalidTimeout(t *testing.T) {
	cfg := tlsTestConfig(1025)
	cfg.Timeout = -1

	_, err := smtp.NewMailer(cfg)
	require.Equal(t, smtp.ErrInvalidTimeout, err)
}

// A configured timeout does not affect a healthy transaction
func TestTimeoutConfigured(t *testing.T) {
	cert := selfSignedCert(t)
	port, _ := startSMTPServer(t, serverOpts{cert: &cert, implicit: true})
	cfg := tlsTestConfig(port)
	cfg.TLSInsecureSkipVerify = true
	cfg.Timeout = duration.Seconds(5)

	require.NoError(t, sendTestMessage(t, cfg))
}

// The configured timeout aborts a connection to a server that accepts and then stalls
func TestTimeoutAbortsSend(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { ln.Close() })

	done := make(chan struct{})
	t.Cleanup(func() { close(done) })
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		<-done // stall without ever answering
	}()

	port, err := strconv.Atoi(strings.Split(ln.Addr().String(), ":")[1])
	require.NoError(t, err)

	cfg := tlsTestConfig(port)
	cfg.SSLOnConnect = false
	cfg.TLSEnable = false
	cfg.TLSPolicy = smtp.TLSPolicyNone
	cfg.Timeout = duration.Seconds(1)

	start := time.Now()
	err = sendTestMessage(t, cfg)
	require.ErrorIs(t, err, smtp.ErrSMTPServer)
	require.Less(t, time.Since(start), 10*time.Second)
}

// All messages are delivered over a single connection
func TestSendMultipleMessages(t *testing.T) {
	cert := selfSignedCert(t)
	port, stats := startSMTPServer(t, serverOpts{cert: &cert, implicit: true})
	cfg := tlsTestConfig(port)
	cfg.TLSInsecureSkipVerify = true

	mailer, err := smtp.NewMailer(cfg)
	require.NoError(t, err)

	messages := make([]*gomail.Msg, 0, 3)
	for i := 0; i < 3; i++ {
		msg, err := mailer.NewMessage([]string{"receiver@example.com"}, "subject",
			smtp.WithFrom(cfg.From),
			smtp.WithBody("body", ""),
		)
		require.NoError(t, err)
		messages = append(messages, msg)
	}

	require.NoError(t, mailer.Send(messages...))
	require.Equal(t, int32(1), stats.connections.Load())
	require.Equal(t, int32(3), stats.delivered.Load())
}

// STARTTLS upgrades the connection and verifies the server certificate against the
// configured CA; this is the path that requires ServerName to be set on the tls.Config
func TestSTARTTLSWithCA(t *testing.T) {
	cert := selfSignedCert(t)
	port, stats := startSMTPServer(t, serverOpts{cert: &cert, starttls: true})
	cfg := tlsTestConfig(port)
	cfg.SSLOnConnect = false
	cfg.TLSCA = cert.certFile

	require.NoError(t, sendTestMessage(t, cfg))
	require.Equal(t, int32(1), stats.encrypted.Load())
	require.Equal(t, int32(1), stats.delivered.Load())
}

// STARTTLS also accepts a self-signed certificate when verification is skipped
func TestSTARTTLSInsecureSkipVerify(t *testing.T) {
	cert := selfSignedCert(t)
	port, stats := startSMTPServer(t, serverOpts{cert: &cert, starttls: true})
	cfg := tlsTestConfig(port)
	cfg.SSLOnConnect = false
	cfg.TLSInsecureSkipVerify = true

	require.NoError(t, sendTestMessage(t, cfg))
	require.Equal(t, int32(1), stats.encrypted.Load())
}

// The opportunistic policy upgrades when STARTTLS is advertised
func TestTLSPolicyOpportunisticUpgrades(t *testing.T) {
	cert := selfSignedCert(t)
	port, stats := startSMTPServer(t, serverOpts{cert: &cert, starttls: true})
	cfg := tlsTestConfig(port)
	cfg.SSLOnConnect = false
	cfg.TLSCA = cert.certFile
	cfg.TLSPolicy = smtp.TLSPolicyOpportunistic

	require.NoError(t, sendTestMessage(t, cfg))
	require.Equal(t, int32(1), stats.encrypted.Load())
}

// The opportunistic policy falls back to plaintext when STARTTLS is not advertised
func TestTLSPolicyOpportunisticFallsBack(t *testing.T) {
	port, stats := startSMTPServer(t, serverOpts{})
	cfg := tlsTestConfig(port)
	cfg.SSLOnConnect = false
	cfg.TLSEnable = false
	cfg.TLSPolicy = smtp.TLSPolicyOpportunistic

	require.NoError(t, sendTestMessage(t, cfg))
	require.Equal(t, int32(0), stats.encrypted.Load())
	require.Equal(t, int32(1), stats.delivered.Load())
}

// The client certificate is presented to a server that requires one
func TestTLSClientCertificate(t *testing.T) {
	cert := selfSignedCert(t)
	port, stats := startSMTPServer(t, serverOpts{cert: &cert, implicit: true, clientCer: true})
	cfg := tlsTestConfig(port)
	cfg.TLSCA = cert.certFile
	cfg.TLSCert = cert.certFile
	cfg.TLSKey = cert.keyFile

	require.NoError(t, sendTestMessage(t, cfg))
	require.Equal(t, int32(1), stats.delivered.Load())
}

// Without a client certificate the same server rejects the connection
func TestTLSClientCertificateMissing(t *testing.T) {
	cert := selfSignedCert(t)
	port, _ := startSMTPServer(t, serverOpts{cert: &cert, implicit: true, clientCer: true})
	cfg := tlsTestConfig(port)
	cfg.TLSCA = cert.certFile

	err := sendTestMessage(t, cfg)
	require.ErrorIs(t, err, smtp.ErrSMTPServer)
}

// Configured credentials are actually used to authenticate
func TestAuthPlain(t *testing.T) {
	cert := selfSignedCert(t)
	port, stats := startSMTPServer(t, serverOpts{cert: &cert, implicit: true, auth: true})
	cfg := tlsTestConfig(port)
	cfg.TLSCA = cert.certFile
	cfg.AuthType = "plain"
	cfg.Username = "user"
	cfg.Password = "secret"

	require.NoError(t, sendTestMessage(t, cfg))

	authLine, _ := stats.authLine.Load().(string)
	require.True(t, strings.HasPrefix(authLine, "AUTH PLAIN "), "got %q", authLine)

	credentials, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(authLine, "AUTH PLAIN "))
	require.NoError(t, err)
	require.Equal(t, "\x00user\x00secret", string(credentials))
}

// A cancelled context aborts the connection attempt
func TestSendWithContextCancelled(t *testing.T) {
	cert := selfSignedCert(t)
	port, stats := startSMTPServer(t, serverOpts{cert: &cert, implicit: true})
	cfg := tlsTestConfig(port)
	cfg.TLSCA = cert.certFile

	mailer, err := smtp.NewMailer(cfg)
	require.NoError(t, err)

	msg, err := mailer.NewMessage([]string{"receiver@example.com"}, "subject",
		smtp.WithBody("body", ""))
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = mailer.SendWithContext(ctx, msg)
	require.ErrorIs(t, err, smtp.ErrSMTPServer)
	require.Equal(t, int32(0), stats.delivered.Load())
}

// Nil messages are skipped
func TestSendSkipsNilMessages(t *testing.T) {
	cert := selfSignedCert(t)
	port, stats := startSMTPServer(t, serverOpts{cert: &cert, implicit: true})
	cfg := tlsTestConfig(port)
	cfg.TLSCA = cert.certFile

	mailer, err := smtp.NewMailer(cfg)
	require.NoError(t, err)

	msg, err := mailer.NewMessage([]string{"receiver@example.com"}, "subject",
		smtp.WithBody("body", ""))
	require.NoError(t, err)

	// only nil messages: no connection is opened
	require.NoError(t, mailer.Send(nil, nil))
	require.Equal(t, int32(0), stats.connections.Load())

	require.NoError(t, mailer.Send(nil, msg, nil))
	require.Equal(t, int32(1), stats.delivered.Load())
}

// Every failure in a batch is reported
func TestSendJoinsFailures(t *testing.T) {
	cert := selfSignedCert(t)
	port, stats := startSMTPServer(t, serverOpts{cert: &cert, implicit: true})
	cfg := tlsTestConfig(port)
	cfg.TLSCA = cert.certFile

	mailer, err := smtp.NewMailer(cfg)
	require.NoError(t, err)

	messages := make([]*gomail.Msg, 0, 3)
	for _, to := range []string{"reject@example.com", "receiver@example.com", "reject2@example.com"} {
		msg, err := mailer.NewMessage([]string{to}, "subject", smtp.WithBody("body", ""))
		require.NoError(t, err)
		messages = append(messages, msg)
	}

	err = mailer.Send(messages...)
	require.ErrorIs(t, err, smtp.ErrMessage)
	require.Equal(t, int32(1), stats.delivered.Load())

	// both rejections are reachable, and each carries the go-mail error detail
	var sendErr *gomail.SendError
	require.ErrorAs(t, err, &sendErr)

	multi, ok := err.(interface{ Unwrap() []error })
	require.True(t, ok)
	joined, ok := multi.Unwrap()[1].(interface{ Unwrap() []error })
	require.True(t, ok)
	require.Len(t, joined.Unwrap(), 2)
	for _, e := range joined.Unwrap() {
		require.ErrorAs(t, e, &sendErr)
	}
}

// A rejected recipient does not prevent the remaining messages from being sent
func TestSendContinuesAfterFailure(t *testing.T) {
	cert := selfSignedCert(t)
	port, stats := startSMTPServer(t, serverOpts{cert: &cert, implicit: true})
	cfg := tlsTestConfig(port)
	cfg.TLSInsecureSkipVerify = true

	mailer, err := smtp.NewMailer(cfg)
	require.NoError(t, err)

	rejected, err := mailer.NewMessage([]string{"reject@example.com"}, "subject",
		smtp.WithBody("body", ""))
	require.NoError(t, err)

	accepted, err := mailer.NewMessage([]string{"receiver@example.com"}, "subject",
		smtp.WithBody("body", ""))
	require.NoError(t, err)

	err = mailer.Send(rejected, accepted)
	require.ErrorIs(t, err, smtp.ErrMessage)
	require.Equal(t, int32(1), stats.delivered.Load())
}
