package smtp_test

import (
	"bufio"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
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

// selfSignedCert generates a self-signed certificate valid for 127.0.0.1 and writes
// the PEM encoded certificate to disk, so it can also be used as a CA bundle
func selfSignedCert(t *testing.T) (tls.Certificate, string) {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	template := x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "127.0.0.1"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
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

	certFile := filepath.Join(t.TempDir(), "cert.pem")
	require.NoError(t, os.WriteFile(certFile, certPEM, 0o600))

	return cert, certFile
}

// smtpStats records what the test SMTP server saw
type smtpStats struct {
	connections atomic.Int32
	delivered   atomic.Int32
}

// startSMTPServer starts a minimal SMTP server; when cert is non-nil the listener
// uses implicit TLS (SMTPS). Recipients containing "reject" are refused
func startSMTPServer(t *testing.T, cert *tls.Certificate) (int, *smtpStats) {
	t.Helper()

	var ln net.Listener
	var err error
	if cert == nil {
		ln, err = net.Listen("tcp", "127.0.0.1:0")
	} else {
		ln, err = tls.Listen("tcp", "127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{*cert}})
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
			go serveSMTP(conn, stats)
		}
	}()

	port, err := strconv.Atoi(strings.Split(ln.Addr().String(), ":")[1])
	require.NoError(t, err)
	return port, stats
}

func serveSMTP(conn net.Conn, stats *smtpStats) {
	defer conn.Close()

	r := bufio.NewReader(conn)
	write := func(s string) bool {
		_, err := conn.Write([]byte(s + "\r\n"))
		return err == nil
	}

	if !write("220 localhost ESMTP ready") {
		return
	}

	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		verb := strings.ToUpper(strings.Fields(line + " ")[0])
		switch verb {
		case "EHLO", "HELO":
			if !write("250-localhost\r\n250 SIZE 10240000") {
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
	cert, _ := selfSignedCert(t)
	port, _ := startSMTPServer(t, &cert)
	cfg := tlsTestConfig(port)
	cfg.TLSInsecureSkipVerify = true

	require.NoError(t, sendTestMessage(t, cfg))
}

// Self-signed certificates are rejected when verification is enabled
func TestTLSVerifyRejectsSelfSigned(t *testing.T) {
	cert, _ := selfSignedCert(t)
	port, _ := startSMTPServer(t, &cert)
	cfg := tlsTestConfig(port)

	err := sendTestMessage(t, cfg)
	require.Error(t, err)
	require.ErrorIs(t, err, smtp.ErrSMTPServer)
}

// Self-signed certificates are accepted when supplied as CA
func TestTLSWithCA(t *testing.T) {
	cert, certFile := selfSignedCert(t)
	port, _ := startSMTPServer(t, &cert)
	cfg := tlsTestConfig(port)
	cfg.TLSCA = certFile

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
	port, _ := startSMTPServer(t, nil)
	cfg := tlsTestConfig(port)
	cfg.SSLOnConnect = false
	cfg.TLSEnable = false
	cfg.TLSPolicy = smtp.TLSPolicyNone

	require.NoError(t, sendTestMessage(t, cfg))
}

// STARTTLS is required by default, and a server without it fails the transaction
func TestTLSPolicyMandatory(t *testing.T) {
	port, _ := startSMTPServer(t, nil)
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
	cert, _ := selfSignedCert(t)
	port, _ := startSMTPServer(t, &cert)
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

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		<-time.After(30 * time.Second)
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
	cert, _ := selfSignedCert(t)
	port, stats := startSMTPServer(t, &cert)
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

// A rejected recipient does not prevent the remaining messages from being sent
func TestSendContinuesAfterFailure(t *testing.T) {
	cert, _ := selfSignedCert(t)
	port, stats := startSMTPServer(t, &cert)
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
