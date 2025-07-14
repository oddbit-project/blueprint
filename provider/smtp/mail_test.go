package smtp_test

import (
	"testing"

	"github.com/oddbit-project/blueprint/crypt/secure"
	"github.com/oddbit-project/blueprint/provider/smtp"
	"github.com/oddbit-project/blueprint/provider/tls"
	"github.com/stretchr/testify/require"
)

// Helper function to create a valid test configuration
func testConfig(authType string) *smtp.Config {
	return &smtp.Config{
		Host:     "127.0.0.1",
		Port:     1025,
		Username: "user",
		AuthType: authType,
		DefaultCredentialConfig: secure.DefaultCredentialConfig{
			Password: "pass",
		},
		ClientConfig: tls.ClientConfig{
			TLSEnable:             false,
			TLSInsecureSkipVerify: true,
		},
		From: "no-reply@example.com",
		Bcc:  "",
	}
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
	msg, err := mailer.NewMessage("receiver@example.com", "Hello subject",
		smtp.WithBody("Plain text body", "<b>HTML body</b>"),
	)
	require.NoError(t, err)
	require.NotNil(t, msg)

	subjects := msg.GetGenHeader("Subject")
	require.NotEmpty(t, subjects)
	require.Equal(t, "Hello subject", subjects[0])
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
