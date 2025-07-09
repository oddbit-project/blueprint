package smtp

import (
	"crypto/tls"
	"strings"

	"github.com/oddbit-project/blueprint/crypt/secure"
	tlsProvider "github.com/oddbit-project/blueprint/provider/tls"
	"github.com/oddbit-project/blueprint/utils"
	"github.com/wneessen/go-mail"
)

const (
	ErrMissingHost     = utils.Error("SMTP host is required")
	ErrMissingPort     = utils.Error("Port number is required")
	ErrMissingFrom     = utils.Error("From address is required")
	ErrInvalidFrom     = utils.Error("From address is not valid")
	ErrInvalidBcc      = utils.Error("BCC address is not valid")
	ErrInvalidConfig   = utils.Error("Config is not valid")
	ErrCreatingClient  = utils.Error("Error creating client")
	ErrInvalidPassword = utils.Error("Invalid Password")
	ErrClient          = utils.Error("Failed creating client")
	ErrSMTPServer      = utils.Error("Failed to dial SMTP server")
	ErrMessage         = utils.Error("Failed to send email message")
)

type Config struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	secure.DefaultCredentialConfig
	tlsProvider.ClientConfig
	From string `json:"from"`
	Bcc  string `json:"bcc,omitempty"`
}

type Mailer struct {
	config *Config
}

type MessageOpts func(*mail.Msg)

func isValidEmail(email string) bool {
	email = strings.TrimSpace(email)
	if email == "" {
		return false
	}
	if strings.Count(email, "@") != 1 {
		return false
	}

	if !strings.Contains(email, ".") {
		return false
	}

	return true
}

func (c *Config) Validate() error {
	if c.Host == "" {
		return ErrMissingHost
	}
	if c.Port < 1 {
		return ErrMissingPort
	}
	if c.From == "" {
		return ErrMissingFrom
	}
	// Email validation
	if !isValidEmail(c.From) {
		return ErrInvalidFrom
	}
	// BCC validation (if provided)
	if c.Bcc != "" {
		bccList := strings.Split(c.Bcc, ",")
		for _, bcc := range bccList {
			if !isValidEmail(strings.TrimSpace(bcc)) {
				return ErrInvalidBcc
			}
		}
	}
	return nil
}

// New smtp configuration with default values
func NewConfig() *Config {
	return &Config{
		Host:     "127.0.0.1",
		Port:     1025,
		Username: "",
		DefaultCredentialConfig: secure.DefaultCredentialConfig{
			Password:       "",
			PasswordEnvVar: "SMTP_PASSWORD",
			PasswordFile:   "",
		},
		ClientConfig: tlsProvider.ClientConfig{
			TLSCA:   "",
			TLSCert: "",
			TLSKey:  "",
			TlsKeyCredential: tlsProvider.TlsKeyCredential{
				Password:       "",
				PasswordEnvVar: "",
				PasswordFile:   "",
			},
			TLSEnable:             false,
			TLSInsecureSkipVerify: false,
		},
		From: "no-reply@acme.co",
		Bcc:  "",
	}
}
func NewMailer(cfg *Config) (*Mailer, error) {
	if cfg == nil {
		return nil, ErrInvalidConfig
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &Mailer{config: cfg}, nil
}

func WithTo(to string) MessageOpts {
	return func(msg *mail.Msg) {
		msg.To(to)
	}
}

func WithSubject(subject string) MessageOpts {
	return func(msg *mail.Msg) {
		msg.Subject(subject)
	}
}

func WithHTML(body string) MessageOpts {
	return func(msg *mail.Msg) {
		msg.SetBodyString(mail.TypeTextHTML, body)
	}
}

func WithPlainText(content string) MessageOpts {
	return func(msg *mail.Msg) {
		msg.SetBodyString(mail.TypeTextPlain, content)
	}
}

func WithAttachment(path string) MessageOpts {
	return func(msg *mail.Msg) {
		msg.AttachFile(path)
	}
}

// Creates a new message with support for attachments
func (m *Mailer) NewMessage(opts ...MessageOpts) (*mail.Msg, error) {
	msg := mail.NewMsg()

	// Validate From field
	if err := msg.From(m.config.From); err != nil {
		return nil, ErrInvalidFrom
	}

	for _, opt := range opts {
		opt(msg)
	}

	return msg, nil
}

func (m *Mailer) CreateClient() (*mail.Client, error) {
	password := m.config.GetPassword()

	if password == "" {
		return nil, ErrInvalidPassword
	}

	clientOpts := []mail.Option{
		mail.WithPort(m.config.Port),
	}

	// Add authentication
	if m.config.Username != "" {
		clientOpts = append(clientOpts,
			mail.WithUsername(m.config.Username),
			mail.WithPassword(password),
			mail.WithSMTPAuth(mail.SMTPAuthPlain),
		)
	}

	if m.config.TLSEnable {
		tlsConfig := &tls.Config{
			InsecureSkipVerify: m.config.TLSInsecureSkipVerify,
		}
		clientOpts = append(clientOpts,
			mail.WithTLSConfig(tlsConfig),
		)
	}

	client, err := mail.NewClient(m.config.Host, clientOpts...)
	if err != nil {
		return nil, ErrCreatingClient
	}

	return client, nil
}

func (m *Mailer) Send(msg ...*mail.Msg) error {

	if len(msg) == 0 {
		return nil
	}

	client, err := m.CreateClient()

	if err != nil {
		return ErrClient
	}

	for _, message := range msg {
		if message != nil {
			if err := client.DialAndSend(message); err != nil {
				return ErrMessage
			}
		}
	}

	return nil
}
