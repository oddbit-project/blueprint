# SMTP Provider

The SMTP provider offers a comprehensive email client implementation with support for various authentication methods,
TLS encryption and attachments. Built on the powerful `go-mail` library, it provides a secure and feature-rich solution
for sending emails.

## Features

- **Multiple Authentication Methods**: Support for Plain, Login, CRAMMD5, XOAUTH2, SCRAM-SHA-1, SCRAM-SHA-256, and
  custom auth
- **TLS/SSL Support**: Optional TLS encryption with certificate validation
- **Secure Credential Management**: Password encryption and secure storage
- **Rich Message Composition**: HTML and plain text bodies with attachments
- **Email Validation**: Built-in email address validation
- **BCC Support**: Automatic BCC functionality for compliance
- **Message Options**: Flexible message building with functional options
- **Connection Management**: Automatic connection handling and cleanup

## Installation

```bash
go get github.com/oddbit-project/blueprint/provider/smtp
```

## Configuration

### Basic Configuration

```go
package main

import (
	"github.com/oddbit-project/blueprint/provider/smtp"
)

func main() {
	config := smtp.NewConfig()
	config.Host = "smtp.gmail.com"
	config.Port = 587
	config.Username = "your-email@gmail.com"
	config.Password = "your-app-password"  // Embedded field, accessed directly
	config.AuthType = "plain"
	config.From = "your-email@gmail.com"
	config.TLSEnable = true  // Embedded field, accessed directly

	mailer, err := smtp.NewMailer(config)
	if err != nil {
		panic(err)
	}
}
```

### JSON Configuration

```json
{
  "smtp": {
    "host": "smtp.gmail.com",
    "port": 587,
    "username": "your-email@gmail.com",
    "password": "your-app-password",
    "authType": "plain",
    "from": "your-email@gmail.com",
    "bcc": "compliance@company.com",
    "tlsEnable": true,
    "tlsInsecureSkipVerify": false,
    "tlsPolicy": "mandatory",
    "sslOnConnect": false,
    "timeout": 15
  }
}
```

### Configuration with Secure Credentials

```go
config := smtp.NewConfig()
config.Host = "smtp.example.com"
config.Port = 587
config.Username = "mailer@example.com"
config.DefaultCredentialConfig = secure.DefaultCredentialConfig{
PasswordEnvVar: "SMTP_PASSWORD", // Read from environment
PasswordFile:   "/secrets/smtp_pwd", // Or read from file
}
config.AuthType = "plain"
config.From = "noreply@example.com"
```

### Advanced TLS Configuration

```go
config := smtp.NewConfig()
config.Host = "secure-smtp.example.com"
config.Port = 465
config.ClientConfig = tls.ClientConfig{
TLSEnable: true,
TLSCA:     "/path/to/ca-cert.pem",
TLSCert:   "/path/to/client-cert.pem",
TLSKey:    "/path/to/client-key.pem",
TLSInsecureSkipVerify: false,
}
config.SSLOnConnect = true // implicit TLS on port 465
```

`TLSEnable` controls whether the certificate settings above are used; when it is `false`, go-mail's own defaults apply
(server name taken from the host, TLS 1.2 minimum, system trust store), and configuring any of `TLSCA`, `TLSCert`,
`TLSKey` or `TLSInsecureSkipVerify` without it is rejected with `ErrTLSNotEnabled` rather than silently ignored.

Note that `TLSEnable` does not, on its own, decide whether the connection is encrypted — that is the STARTTLS policy
below, which requires TLS by default. The two are configured separately:

```go
// Self-signed or otherwise untrusted server certificate
config.TLSEnable = true
config.TLSInsecureSkipVerify = true

// Private CA instead of skipping verification
config.TLSEnable = true
config.TLSCA = "/path/to/ca-cert.pem"

// Plaintext server, e.g. a local mailhog instance
config.TLSPolicy = smtp.TLSPolicyNone
```

### TLS Policy

| Value             | Behaviour                                                              |
|-------------------|------------------------------------------------------------------------|
| `"mandatory"`     | STARTTLS is required; the connection fails if the server lacks it (default) |
| `"opportunistic"` | STARTTLS is used when advertised, otherwise the connection stays plaintext |
| `"none"`          | STARTTLS is never attempted                                            |

The policy applies to STARTTLS only; with `SSLOnConnect` the connection is encrypted from the start and the policy is
not used.

> **Note**: with `SSLOnConnect` the provider performs the TLS dial itself, in order to apply the timeout to the whole
> conversation. `go-mail` therefore does not see the connection as encrypted when auto-discovering the authentication
> mechanism, and restricts its choice to SCRAM-SHA-256, SCRAM-SHA-1 and CRAM-MD5. Set an explicit `AuthType` instead of
> `"auto"` when using implicit TLS with a server that only offers PLAIN or LOGIN.

## Configuration Options

| Field                   | Type     | Default              | Description                                         |
|-------------------------|----------|----------------------|-----------------------------------------------------|
| `Host`                  | `string` | `"127.0.0.1"`        | SMTP server hostname                                |
| `Port`                  | `int`    | `1025`               | SMTP server port                                    |
| `Username`              | `string` | `""`                 | SMTP authentication username                        |
| `Password`              | `string` | `""`                 | SMTP authentication password (embedded field)       |
| `PasswordEnvVar`        | `string` | `""`                 | Environment variable for password                   |
| `PasswordFile`          | `string` | `""`                 | File path containing password                       |
| `AuthType`              | `string` | `"noauth"`           | Authentication method (plain, login, crammd5, etc.) |
| `From`                  | `string` | `"no-reply@acme.co"` | Sender address, applied to every message            |
| `Bcc`                   | `string` | `""`                 | Comma-separated BCC addresses, added to every message |
| `TLSEnable`             | `bool`   | `false`              | Apply the TLS certificate settings (embedded field) |
| `TLSCA`                 | `string` | `""`                 | CA bundle used to verify the server certificate     |
| `TLSCert`               | `string` | `""`                 | Client certificate                                  |
| `TLSKey`                | `string` | `""`                 | Client certificate key                              |
| `TLSInsecureSkipVerify` | `bool`   | `false`              | Skip TLS certificate verification (self-signed)     |
| `TLSPolicy`             | `string` | `"mandatory"`        | STARTTLS policy: mandatory, opportunistic, none     |
| `SSLOnConnect`          | `bool`   | `false`              | Implicit TLS (SMTPS, usually port 465)              |
| `Timeout`               | `duration.Seconds` | `15`       | Timeout for the SMTP conversation, in seconds       |

`Timeout` uses the `types/duration` package, which serializes as a plain integer number of seconds:

```go
import "github.com/oddbit-project/blueprint/types/duration"

config.Timeout = duration.Seconds(30)
```

## Authentication Types

The SMTP provider supports multiple authentication methods:

- **`"plain"`** - Plain text authentication (most common)
- **`"login"`** - LOGIN authentication
- **`"crammd5"`** - CRAM-MD5 authentication
- **`"xoauth2"`** - XOAUTH2 for OAuth2 authentication
- **`"scram-sha-1"`** - SCRAM-SHA-1 authentication
- **`"scram-sha-256"`** - SCRAM-SHA-256 authentication
- **`"custom"`** - Custom authentication (requires custom auth option)
- **`"noauth"`** - No authentication (for development/testing); also assumed when `AuthType` is empty

## Usage Examples

### Basic Email Sending

```go
package main

import (
	"log"
	"github.com/oddbit-project/blueprint/provider/smtp"
)

func main() {
	// Create configuration
	config := smtp.NewConfig()
	config.Host = "smtp.gmail.com"
	config.Port = 587
	config.Username = "your-email@gmail.com"
	config.Password = "your-app-password"
	config.AuthType = "plain"
	config.From = "your-email@gmail.com"
	config.TLSEnable = true

	// Create mailer
	mailer, err := smtp.NewMailer(config)
	if err != nil {
		log.Fatal(err)
	}

	// Create and send message
	recipients := []string{"recipient@example.com"}
	msg, err := mailer.NewMessage(recipients, "Hello from Blueprint!",
		smtp.WithBody("Hello World!", "<h1>Hello World!</h1>"),
	)
	if err != nil {
		log.Fatal(err)
	}

	if err := mailer.Send(msg); err != nil {
		log.Fatal(err)
	}

	log.Println("Email sent successfully!")
}

```

### HTML Email with Plain Text Alternative

```go
plainText := `
Hello John,

Thank you for signing up for our service!

Best regards,
The Team
`

htmlBody := `
<html>
<head><title>Welcome</title></head>
<body>
    <h1>Hello John,</h1>
    <p>Thank you for signing up for our service!</p>
    <p><strong>Best regards,</strong><br/>The Team</p>
</body>
</html>
`

msg, err := mailer.NewMessage(
	[]string{"john@example.com"},
	"Welcome to our service!",
	smtp.WithBody(plainText, htmlBody),
)
if err != nil {
	log.Fatal(err)
}

err = mailer.Send(msg)
if err != nil {
	log.Fatal(err)
}
```

### Email with Attachments

```go
msg, err := mailer.NewMessage(
	[]string{"recipient@example.com"},
	"Document Attached",
	smtp.WithBody("Please find the attached document.", ""),
	smtp.WithAttachment("/path/to/document.pdf"),
	smtp.WithAttachment("/path/to/image.png"),
)
if err != nil {
	log.Fatal(err)
}

err = mailer.Send(msg)
if err != nil {
	log.Fatal(err)
}
```

### Custom From Address

```go
msg, err := mailer.NewMessage(
	[]string{"customer@example.com"},
	"Support Ticket Update",
	smtp.WithFrom("support@company.com"),
	smtp.WithBody("Your ticket has been updated.", ""),
)
if err != nil {
	log.Fatal(err)
}

err = mailer.Send(msg)
```

### Batch Email Sending

```go
var messages []*gomail.Msg

// Create multiple messages
for _, recipient := range recipients {
	msg, err := mailer.NewMessage(
		[]string{recipient.Email},
		"Newsletter",
		smtp.WithBody("Newsletter content", "<h1>Newsletter</h1>"),
	)
	if err != nil {
		log.Printf("Failed to create message for %s: %v", recipient.Email, err)
		continue
	}
	messages = append(messages, msg)
}

// Send all messages in batch, over a single connection
if err := mailer.Send(messages...); err != nil {
	log.Fatal("Failed to send batch emails:", err)
}

// Or with a caller-supplied context bounding the connection attempt
if err := mailer.SendWithContext(ctx, messages...); err != nil {
	log.Fatal("Failed to send batch emails:", err)
}
```

`Send` opens one connection for the whole batch and closes it when done. Connection failures — dial, TLS handshake,
STARTTLS negotiation and authentication — are reported as `ErrSMTPServer`; failures while transferring a message are
reported as `ErrMessage`. Both wrap the underlying `go-mail` error, so `errors.Is`/`errors.As` reach it.

The `Timeout` setting is applied as a deadline over the whole conversation: it bounds the connection attempt, the
server greeting, STARTTLS and authentication, and is renewed before each message is sent.

### BCC Support

```go
// Configure automatic BCC
config.Bcc = "compliance@company.com,audit@company.com"

// All messages created by the mailer include these BCC recipients
msg, err := mailer.NewMessage(
	[]string{"customer@example.com"},
	"Important Notice",
	smtp.WithBody("This email will be BCC'd to compliance.", ""),
)
```

The configured recipients are added after the message options are applied, so they do not replace any BCC address set
on the message itself.

## Message Options

The SMTP provider uses functional options for flexible message composition:

### `WithFrom(address)`

Override the default sender address:

```go
smtp.WithFrom("noreply@company.com")
```

### `WithBody(plainText, htmlBody)`

Set message body content:

```go
smtp.WithBody("Plain text version", "<h1>HTML version</h1>")
```

### `WithAttachment(path)`

Add file attachments:

```go
smtp.WithAttachment("/path/to/file.pdf")
```

Multiple options can be combined:

```go
msg, err := mailer.NewMessage(
	[]string{"user@example.com"},
	"Multi-part Message",
	smtp.WithFrom("sales@company.com"),
	smtp.WithBody("Plain text", "<h1>HTML content</h1>"),
	smtp.WithAttachment("/path/to/brochure.pdf"),
	smtp.WithAttachment("/path/to/image.jpg"),
)
```

## Security Considerations

### Secure Password Management

```go
// Use environment variables
config.PasswordEnvVar = "SMTP_PASSWORD"

// Use secure file storage
config.PasswordFile = "/run/secrets/smtp_password"

// Passwords are automatically cleared from memory after use
```

### TLS Configuration

```go
// Enable TLS with certificate verification
config.TLSEnable = true
config.TLSInsecureSkipVerify = false

// Use custom CA certificates
config.TLSCA = "/path/to/ca-bundle.crt"

// Client certificate authentication
config.TLSCert = "/path/to/client.crt"
config.TLSKey = "/path/to/client.key"
```

### Email Validation

The provider automatically validates email addresses:

```go
// Invalid emails will return ErrInvalidTo
invalidRecipients := []string{"not-an-email", "invalid@"}
_, err := mailer.NewMessage(invalidRecipients, "Subject", ...)
// err == smtp.ErrInvalidTo
```

## Integration Examples

### With HTTP Server for Contact Forms

```go
import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/oddbit-project/blueprint/provider/httpserver"
	"github.com/oddbit-project/blueprint/provider/smtp"
)

type ContactForm struct {
	Name    string `json:"name" binding:"required"`
	Email   string `json:"email" binding:"required,email"`
	Message string `json:"message" binding:"required"`
}

func setupContactHandler(mailer *smtp.Mailer) gin.HandlerFunc {
	return func(c *gin.Context) {
		var form ContactForm
		if err := c.ShouldBindJSON(&form); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}

		// Send notification email
		msg, err := mailer.NewMessage(
			[]string{"contact@company.com"},
			"New Contact Form Submission",
			smtp.WithBody(
				fmt.Sprintf("Name: %s\nEmail: %s\nMessage: %s",
					form.Name, form.Email, form.Message),
				fmt.Sprintf("<h3>New Contact</h3><p><strong>Name:</strong> %s<br/><strong>Email:</strong> %s<br/><strong>Message:</strong> %s</p>",
					form.Name, form.Email, form.Message),
			),
		)
		if err != nil {
			c.JSON(500, gin.H{"error": "Failed to create email"})
			return
		}

		if err := mailer.Send(msg); err != nil {
			c.JSON(500, gin.H{"error": "Failed to send email"})
			return
		}

		c.JSON(200, gin.H{"message": "Contact form submitted successfully"})
	}
}
```

### With Configuration Provider

```go
import (
	"github.com/oddbit-project/blueprint/config/provider"
	"github.com/oddbit-project/blueprint/provider/smtp"
)

func setupMailerFromConfig(configFile string) (*smtp.Mailer, error) {
	// Load configuration
	cfg, err := provider.NewJsonProvider(configFile)
	if err != nil {
		return nil, err
	}

	// Create SMTP config
	smtpConfig := smtp.NewConfig()
	if err := cfg.GetKey("smtp", smtpConfig); err != nil {
		return nil, err
	}

	// Create mailer
	return smtp.NewMailer(smtpConfig)
}
```

### Email Templates

```go
import "html/template"

type EmailTemplate struct {
Subject string
Plain   string
HTML    string
}

func (t *EmailTemplate) Render(data interface{}) (string, string, error) {
	plainTmpl, err := template.New("plain").Parse(t.Plain)
	if err != nil {
		return "", "", err
	}

	htmlTmpl, err := template.New("html").Parse(t.HTML)
	if err != nil {
		return "", "", err
	}

	var plainBuf, htmlBuf strings.Builder

	if err := plainTmpl.Execute(&plainBuf, data); err != nil {
		return "", "", err
	}

	if err := htmlTmpl.Execute(&htmlBuf, data); err != nil {
		return "", "", err
	}

	return plainBuf.String(), htmlBuf.String(), nil
}

// Usage
welcomeTemplate := &EmailTemplate{
	Subject: "Welcome {{.Name}}!",
	Plain:   "Hello {{.Name}}, welcome to our service!",
	HTML:    "<h1>Hello {{.Name}}</h1><p>Welcome to our service!</p>",
}

userData := map[string]string{"Name": "John"}
plain, html, err := welcomeTemplate.Render(userData)
if err != nil {
	log.Fatal(err)
}

msg, err := mailer.NewMessage(
	[]string{"john@example.com"},
	"Welcome John!",
	smtp.WithBody(plain, html),
)
```

## Error Handling

The SMTP provider defines specific error types for different failure scenarios:

```go
_, err := mailer.NewMessage([]string{"invalid-email"}, "Subject")
if err != nil {
	switch err {
	case smtp.ErrInvalidTo:
		log.Println("Invalid recipient email address")
	case smtp.ErrInvalidFrom:
		log.Println("Invalid sender email address")
	case smtp.ErrInvalidBcc:
		log.Println("Invalid BCC email address")
	default:
		log.Printf("Unexpected error: %v", err)
	}
}
```

Sending errors wrap the underlying `go-mail` error, so they must be matched with `errors.Is` rather than compared
directly:

```go
if err := mailer.Send(msg); err != nil {
	switch {
	case errors.Is(err, smtp.ErrSMTPServer):
		log.Println("Failed to connect to SMTP server:", err)
	case errors.Is(err, smtp.ErrMessage):
		log.Println("Failed to send email message:", err)
	default:
		log.Printf("Send error: %v", err)
	}
}
```

`NewMailer` reports invalid configuration before any connection is attempted:

| Error                 | Cause                                                                       |
|-----------------------|-----------------------------------------------------------------------------|
| `ErrInvalidConfig`    | Nil configuration                                                            |
| `ErrMissingHost`      | Empty `Host`                                                                 |
| `ErrMissingPort`      | `Port` below 1                                                               |
| `ErrInvalidTLSPolicy` | `TLSPolicy` is not mandatory, opportunistic or none                          |
| `ErrInvalidTimeout`   | Negative `Timeout`                                                           |
| `ErrTLSNotEnabled`    | Certificate settings configured while `TLSEnable` is false                   |
| `ErrInvalidFrom`      | `From` is not a valid address                                                |
| `ErrInvalidBcc`       | `Bcc` contains an invalid address                                            |
| `ErrInvalidAuthType`  | `AuthType` is not recognised, or `custom` without a custom auth option        |
| `ErrInvalidTLSConfig` | The CA, certificate or key could not be loaded (wraps the underlying error)  |
| `ErrInvalidPassword`  | The configured password could not be read                                    |
| `ErrCreatingClient`   | `go-mail` rejected the client options (wraps the underlying error)           |

## Best Practices

1. **Credential Security**: Always use environment variables or secure files for passwords
2. **TLS Configuration**: Enable TLS for production deployments
3. **Email Validation**: Validate email addresses before sending
4. **Error Handling**: Implement proper error handling and retry logic
5. **Rate Limiting**: Respect SMTP server rate limits
6. **Message Composition**: Provide both plain text and HTML versions
7. **Testing**: Use development SMTP servers (like MailHog) for testing

## Testing with Development SMTP Server

For development and testing, you can use MailHog or similar tools:

```bash
# Install and run MailHog
go install github.com/mailhog/MailHog@latest
MailHog
```

Configuration for development:

```go
config := smtp.NewConfig()
config.Host = "127.0.0.1"
config.Port = 1025
config.AuthType = "noauth" // No authentication needed for MailHog
config.From = "test@example.com"
config.TLSPolicy = smtp.TLSPolicyNone // MailHog does not offer STARTTLS
```

## Performance Considerations

- **Connection Reuse**: Each `Send` call uses a single connection for all the messages it is given
- **Batch Sending**: Use batch sending for multiple emails
- **Authentication Caching**: Credentials are cached during the session
- **TLS Overhead**: Consider TLS overhead for high-volume sending
- **Message Size**: Be mindful of attachment sizes and message limits

## Common SMTP Server Settings

### Gmail

```go
config.Host = "smtp.gmail.com"
config.Port = 587
config.AuthType = "plain"
config.TLSEnable = true
// Requires App Password, not regular password
```

### Outlook/Hotmail

```go
config.Host = "smtp-mail.outlook.com"
config.Port = 587
config.AuthType = "plain"
config.TLSEnable = true
```

### Amazon SES

```go
config.Host = "email-smtp.us-east-1.amazonaws.com"
config.Port = 587
config.AuthType = "plain"
config.TLSEnable = true
// Requires SES SMTP credentials
```

### SendGrid

```go
config.Host = "smtp.sendgrid.net"
config.Port = 587
config.Username = "apikey"
config.Password = "your-sendgrid-api-key"
config.AuthType = "plain"
config.TLSEnable = true
```

### Implicit TLS (SMTPS)

```go
config.Host = "smtp.example.com"
config.Port = 465
config.AuthType = "plain"
config.TLSEnable = true
config.SSLOnConnect = true
```

## Troubleshooting

### Connection Issues

```go
// Test SMTP connection
config := smtp.NewConfig()
mailer, err := smtp.NewMailer(config)
if err != nil {
	log.Printf("Configuration error: %v", err)
	return
}

// Try sending a test message
msg, _ := mailer.NewMessage([]string{"test@example.com"}, "Test")
if err := mailer.Send(msg); err != nil {
	log.Printf("Send failed: %v", err)
	// Check network, credentials, server settings
}
```

### Common Issues

- **Authentication Failure**: Verify username/password and auth type
- **TLS Handshake**: Check TLS configuration and certificate validation
- **Port Blocked**: Ensure SMTP port is not blocked by firewall
- **Rate Limiting**: Implement delays between batch sends if needed
- **Message Rejected**: Check email format, size limits, and content filters

## See Also

- [Secure Credentials](../crypt/secure-credentials.md) - For secure password management
- [TLS Configuration](tls.md) - For TLS setup and certificates
- [HTTP Server](httpserver/index.md) - For web application integration
- [Configuration Management](../config/config.md) - For application configuration