package smtp_test

import (
	"testing"

	"github.com/oddbit-project/blueprint/crypt/secure"
	"github.com/oddbit-project/blueprint/provider/smtp"
	"github.com/oddbit-project/blueprint/provider/tls"
	"github.com/stretchr/testify/require"
	gomail "github.com/wneessen/go-mail"
)

// Helper function to create a valid test configuration
func testConfig(authType string) *smtp.Config {
	cfg := &smtp.Config{
		Host:     "127.0.0.1",
		Port:     1025,
		AuthType: authType,
		ClientConfig: tls.ClientConfig{
			TLSEnable: false,
		},
		From: "no-reply@example.com",
		Bcc:  "",
	}
	// credentials are only valid with an authentication method
	if authType != "" && authType != smtp.AuthTypeNone {
		cfg.Username = "user"
		cfg.DefaultCredentialConfig = secure.DefaultCredentialConfig{Password: "pass"}
	}
	return cfg
}

// Test creating a Mailer with valid config and plain auth
func TestMailerWithPlainAuth(t *testing.T) {
	cfg := testConfig("plain")

	mailer, err := smtp.NewMailer(cfg)
	require.NoError(t, err)
	require.NotNil(t, mailer)
}

// Test creating a Mailer with valid config and custom auth
func TestNewMailerWithCustomAuth(t *testing.T) {
	cfg := testConfig("custom")
	mailer, err := smtp.NewMailer(cfg, nil)
	require.NoError(t, err)
	require.NotNil(t, mailer)
}

// Test creating a new message
func TestNewMessage(t *testing.T) {
	cfg := testConfig("noauth")

	mailer, err := smtp.NewMailer(cfg)
	require.NoError(t, err)

	// Create a message with subject, body, and recipient
	msg, err := mailer.NewMessage([]string{"receiver@example.com"}, "Hello subject",
		smtp.WithBody("Plain text body", "<b>HTML body</b>"),
	)
	require.NoError(t, err)
	require.NotNil(t, msg)

	subjects := msg.GetGenHeader("Subject")
	require.NotEmpty(t, subjects)
	require.Equal(t, "Hello subject", subjects[0])
}

// Test the default configuration is usable
func TestNewConfigIsValid(t *testing.T) {
	mailer, err := smtp.NewMailer(smtp.NewConfig())
	require.NoError(t, err)
	require.NotNil(t, mailer)
}

// Test the configured sender is applied to every message
func TestNewMessageWithConfiguredFrom(t *testing.T) {
	cfg := testConfig("noauth")
	cfg.From = "sender@example.com"

	mailer, err := smtp.NewMailer(cfg)
	require.NoError(t, err)

	msg, err := mailer.NewMessage([]string{"receiver@example.com"}, "Hello subject")
	require.NoError(t, err)
	require.Equal(t, "<sender@example.com>", msg.GetFromString()[0])
}

// Test WithFrom overrides the configured sender
func TestNewMessageFromOverride(t *testing.T) {
	cfg := testConfig("noauth")
	cfg.From = "sender@example.com"

	mailer, err := smtp.NewMailer(cfg)
	require.NoError(t, err)

	msg, err := mailer.NewMessage([]string{"receiver@example.com"}, "Hello subject",
		smtp.WithFrom("other@example.com"),
	)
	require.NoError(t, err)
	require.Equal(t, "<other@example.com>", msg.GetFromString()[0])
}

// Test validation: certificate settings without TLSEnable should return an error
func TestInvalidConfigTLSNotEnabled(t *testing.T) {
	cases := map[string]func(cfg *smtp.Config){
		"ca":             func(cfg *smtp.Config) { cfg.TLSCA = "/path/to/ca.pem" },
		"cert":           func(cfg *smtp.Config) { cfg.TLSCert = "/path/to/cert.pem" },
		"key":            func(cfg *smtp.Config) { cfg.TLSKey = "/path/to/key.pem" },
		"skipVerify":     func(cfg *smtp.Config) { cfg.TLSInsecureSkipVerify = true },
		"keyPassword":    func(cfg *smtp.Config) { cfg.TlsKeyCredential.Password = "secret" },
		"keyPasswordEnv": func(cfg *smtp.Config) { cfg.TlsKeyCredential.PasswordEnvVar = "TLS_KEY_PWD" },
		"keyPasswordFil": func(cfg *smtp.Config) { cfg.TlsKeyCredential.PasswordFile = "/secrets/key" },
	}

	for name, apply := range cases {
		t.Run(name, func(t *testing.T) {
			cfg := testConfig("noauth")
			apply(cfg)

			_, err := smtp.NewMailer(cfg)
			require.Equal(t, smtp.ErrTLSNotEnabled, err)
		})
	}
}

// Test validation: credentials without an auth type should return an error, as
// go-mail would never send them
func TestInvalidConfigCredentialsWithoutAuth(t *testing.T) {
	for _, authType := range []string{"", "noauth", "none"} {
		t.Run(authType, func(t *testing.T) {
			cfg := testConfig(authType)
			cfg.Username = "user"
			cfg.Password = "pass"

			_, err := smtp.NewMailer(cfg)
			require.Equal(t, smtp.ErrCredentialsUnused, err)
		})
	}
}

// Test validation: an invalid sender should return an error
func TestInvalidConfigFrom(t *testing.T) {
	cfg := testConfig("noauth")
	cfg.From = "not an address"

	_, err := smtp.NewMailer(cfg)
	require.Equal(t, smtp.ErrInvalidFrom, err)
}

// Test configured BCC recipients are added to every message
func TestNewMessageWithConfiguredBcc(t *testing.T) {
	cfg := testConfig("noauth")
	cfg.Bcc = "compliance@example.com, audit@example.com"

	mailer, err := smtp.NewMailer(cfg)
	require.NoError(t, err)

	msg, err := mailer.NewMessage([]string{"receiver@example.com"}, "Hello subject")
	require.NoError(t, err)
	require.Equal(t, []string{"<compliance@example.com>", "<audit@example.com>"}, msg.GetBccString())
}

// Test configured BCC recipients do not replace message specific ones
func TestNewMessageBccKeepsMessageRecipients(t *testing.T) {
	cfg := testConfig("noauth")
	cfg.Bcc = "compliance@example.com"

	mailer, err := smtp.NewMailer(cfg)
	require.NoError(t, err)

	msg, err := mailer.NewMessage([]string{"receiver@example.com"}, "Hello subject",
		func(m *gomail.Msg) {
			require.NoError(t, m.Bcc("archive@example.com"))
		},
	)
	require.NoError(t, err)
	require.Equal(t, []string{"<archive@example.com>", "<compliance@example.com>"}, msg.GetBccString())
}

// Test validation: missing host should return an error
func TestInvalidConfigMissingHost(t *testing.T) {
	cfg := testConfig("noauth")
	cfg.Host = ""

	_, err := smtp.NewMailer(cfg)
	require.Equal(t, smtp.ErrMissingHost, err)
}

// Test no messages passed to Send (should not fail)
func TestSendWithNoMessages(t *testing.T) {
	cfg := testConfig("noauth")

	mailer, err := smtp.NewMailer(cfg)
	require.NoError(t, err)

	// Call Send without passing any messages
	err = mailer.Send()
	require.NoError(t, err)
}
