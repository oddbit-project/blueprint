package smtp

import (
	"crypto/tls"
	"net/mail"
	"strings"

	"github.com/oddbit-project/blueprint/crypt/secure"
	tlsProvider "github.com/oddbit-project/blueprint/provider/tls"
	"github.com/oddbit-project/blueprint/utils"
	gomail "github.com/wneessen/go-mail"
)

const (
	ErrMissingHost     = utils.Error("SMTP host is required")
	ErrMissingPort     = utils.Error("Port number is required")
	ErrInvalidFrom     = utils.Error("From is not valid")
	ErrInvalidTo       = utils.Error("To address is not valid")
	ErrInvalidBcc      = utils.Error("BCC address is not valid")
	ErrInvalidConfig   = utils.Error("Config is not valid")
	ErrCreatingClient  = utils.Error("Error creating client")
	ErrInvalidPassword = utils.Error("Invalid Password")
	ErrClient          = utils.Error("Failed creating client")
	ErrSMTPServer      = utils.Error("Failed to dial SMTP server")
	ErrMessage         = utils.Error("Failed to send email message")
	ErrInvalidAuthType = utils.Error("Invalid auth type")
)

type Config struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	secure.DefaultCredentialConfig
	tlsProvider.ClientConfig
	AuthType string `json:"auth_type"`
	From     string `json:"from"`
	Bcc      string `json:"bcc,omitempty"`
}

type Mailer struct {
	config *Config
	client *gomail.Client
}

type MessageOpts func(*gomail.Msg)

func isValidEmail(email string) bool {
	email = strings.TrimSpace(email)
	if email == "" {
		return false
	}

	_, err := mail.ParseAddress(email)

	return err == nil
}

func (c *Config) Validate() error {
	if c.Host == "" {
		return ErrMissingHost
	}
	if c.Port < 1 {
		return ErrMissingPort
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

func WithFrom(from string) MessageOpts {
	return func(msg *gomail.Msg) {
		msg.From(from)
	}
}

func WithBody(plainText, htmlBody string) MessageOpts {
	return func(msg *gomail.Msg) {
		if plainText != "" && htmlBody != "" {
			msg.SetBodyString(gomail.TypeTextPlain, plainText)
			msg.AddAlternativeString(gomail.TypeTextHTML, htmlBody)
		} else if htmlBody != "" {
			msg.SetBodyString(gomail.TypeTextHTML, htmlBody)
		} else if plainText != "" {
			msg.SetBodyString(gomail.TypeTextPlain, plainText)
		}
	}
}

func WithAttachment(path string) MessageOpts {
	return func(msg *gomail.Msg) {
		msg.AttachFile(path)
	}
}

// New smtp configuration with default values
func NewConfig() *Config {
	return &Config{
		Host:     "127.0.0.1",
		Port:     1025,
		Username: "",
		DefaultCredentialConfig: secure.DefaultCredentialConfig{
			Password:       "",
			PasswordEnvVar: "",
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

func NewMailer(cfg *Config, customAuth ...gomail.Option) (*Mailer, error) {
	if cfg == nil {
		return nil, ErrInvalidConfig
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	key, err := secure.GenerateKey()
	if err != nil {
		return nil, err
	}

	credential, err := secure.CredentialFromConfig(cfg.DefaultCredentialConfig, key, true)
	if err != nil {
		return nil, err
	}

	password, err := credential.Get()
	if err != nil {
		return nil, ErrInvalidPassword
	}

	clientOpts := []gomail.Option{
		gomail.WithPort(cfg.Port),
	}

	if cfg.Username != "" {
		clientOpts = append(clientOpts,
			gomail.WithUsername(cfg.Username),
			gomail.WithPassword(password),
		)
	}

	var authType gomail.SMTPAuthType

	if err := authType.UnmarshalString(cfg.AuthType); err != nil {
		return nil, ErrInvalidAuthType
	}

	if authType == gomail.SMTPAuthCustom {
		if len(customAuth) == 0 {
			return nil, ErrInvalidAuthType
		}
		clientOpts = append(clientOpts, customAuth...)
	} else {
		clientOpts = append(clientOpts, gomail.WithSMTPAuth(authType))
	}

	if cfg.TLSEnable {
		tlsConfig := &tls.Config{
			InsecureSkipVerify: cfg.TLSInsecureSkipVerify,
		}
		clientOpts = append(clientOpts,
			gomail.WithTLSConfig(tlsConfig),
		)
	}

	client, err := gomail.NewClient(cfg.Host, clientOpts...)
	if err != nil {
		return nil, ErrCreatingClient
	}

	return &Mailer{
		config: cfg,
		client: client,
	}, nil
}

// Creates a new message with support for attachments
func (m *Mailer) NewMessage(to string, subject string, opts ...MessageOpts) (*gomail.Msg, error) {
	msg := gomail.NewMsg()

	if !isValidEmail(to) {
		return nil, ErrInvalidTo
	}

	if m.config.From != "" {
		if err := msg.From(m.config.From); err != nil {
			return nil, ErrInvalidFrom
		}
	}

	msg.To(to)
	msg.Subject(subject)

	for _, opt := range opts {
		opt(msg)
	}

	return msg, nil
}

func (m *Mailer) Send(msg ...*gomail.Msg) error {

	if len(msg) == 0 {
		return nil
	}

	for _, message := range msg {
		if message != nil {
			if err := m.client.DialAndSend(message); err != nil {
				return ErrMessage
			}
		}
	}

	return nil
}
