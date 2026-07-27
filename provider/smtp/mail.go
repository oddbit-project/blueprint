package smtp

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/mail"
	"strings"
	"time"

	"github.com/oddbit-project/blueprint/crypt/secure"
	tlsProvider "github.com/oddbit-project/blueprint/provider/tls"
	"github.com/oddbit-project/blueprint/types/duration"
	"github.com/oddbit-project/blueprint/utils"
	gomail "github.com/wneessen/go-mail"
)

const (
	ErrMissingHost       = utils.Error("SMTP host is required")
	ErrMissingPort       = utils.Error("Port number is required")
	ErrInvalidFrom       = utils.Error("From is not valid")
	ErrInvalidTo         = utils.Error("To address is not valid")
	ErrInvalidBcc        = utils.Error("BCC address is not valid")
	ErrInvalidConfig     = utils.Error("Config is not valid")
	ErrCreatingClient    = utils.Error("Error creating client")
	ErrInvalidPassword   = utils.Error("Invalid Password")
	ErrSMTPServer        = utils.Error("Failed to dial SMTP server")
	ErrMessage           = utils.Error("Failed to send email message")
	ErrInvalidAuthType   = utils.Error("Invalid auth type")
	ErrCredentialsUnused = utils.Error("Username configured without an auth type")
	ErrInvalidTLSPolicy  = utils.Error("Invalid TLS policy")
	ErrInvalidTLSConfig  = utils.Error("Invalid TLS configuration")
	ErrTLSNotEnabled     = utils.Error("TLS settings require tlsEnable")
	ErrInvalidTimeout    = utils.Error("Invalid timeout")
)

// TLS policy values for Config.TLSPolicy
const (
	// TLSPolicyMandatory requires STARTTLS; the connection fails if the server does not support it
	TLSPolicyMandatory = "mandatory"
	// TLSPolicyOpportunistic uses STARTTLS when advertised, and falls back to an unencrypted connection
	TLSPolicyOpportunistic = "opportunistic"
	// TLSPolicyNone disables STARTTLS
	TLSPolicyNone = "none"
)

// AuthTypeNone disables SMTP authentication; it is also assumed when Config.AuthType is empty
const AuthTypeNone = "noauth"

// DefaultTimeout is the deadline used when Config.Timeout is zero
const DefaultTimeout = duration.Seconds(15)

type Config struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	secure.DefaultCredentialConfig
	tlsProvider.ClientConfig
	AuthType string `json:"authType"`
	// TLSPolicy is the STARTTLS policy; one of "mandatory" (default), "opportunistic" or "none"
	TLSPolicy string `json:"tlsPolicy"`
	// SSLOnConnect enables implicit TLS (SMTPS, usually port 465) instead of STARTTLS
	SSLOnConnect bool `json:"sslOnConnect"`
	// Timeout is the deadline for the SMTP conversation, in seconds; it covers the
	// connection attempt, the greeting, STARTTLS and authentication, and is renewed
	// for each message sent. Zero uses DefaultTimeout
	Timeout duration.Seconds `json:"timeout"`
	From    string           `json:"from"`
	Bcc     string           `json:"bcc,omitempty"`
}

// parseAuthType converts a configured auth type to a go-mail SMTPAuthType; an empty
// value means no authentication
func parseAuthType(name string) (gomail.SMTPAuthType, error) {
	authType := gomail.SMTPAuthNoAuth
	if name == "" {
		return authType, nil
	}
	if err := authType.UnmarshalString(name); err != nil {
		return authType, ErrInvalidAuthType
	}
	return authType, nil
}

// parseTLSPolicy converts a configured policy name to a go-mail TLSPolicy
func parseTLSPolicy(policy string) (gomail.TLSPolicy, error) {
	switch strings.ToLower(strings.TrimSpace(policy)) {
	case "", TLSPolicyMandatory:
		return gomail.TLSMandatory, nil
	case TLSPolicyOpportunistic:
		return gomail.TLSOpportunistic, nil
	case TLSPolicyNone:
		return gomail.NoTLS, nil
	}
	return 0, ErrInvalidTLSPolicy
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
	authType, err := parseAuthType(c.AuthType)
	if err != nil {
		return err
	}
	// go-mail skips authentication entirely for SMTPAuthNoAuth, so credentials
	// configured without an auth type would be silently unused
	if c.Username != "" && authType == gomail.SMTPAuthNoAuth {
		return ErrCredentialsUnused
	}
	if _, err = parseTLSPolicy(c.TLSPolicy); err != nil {
		return err
	}
	if c.Timeout < 0 {
		return ErrInvalidTimeout
	}
	// the certificate settings are only read when TLS is enabled; setting them without
	// TLSEnable is a misconfiguration, as they would be silently ignored
	keyCredential := c.ClientConfig.TlsKeyCredential
	if !c.TLSEnable && (c.TLSCA != "" || c.TLSCert != "" || c.TLSKey != "" || c.TLSInsecureSkipVerify ||
		keyCredential.Password != "" || keyCredential.PasswordEnvVar != "" || keyCredential.PasswordFile != "") {
		return ErrTLSNotEnabled
	}
	// From validation (if provided)
	if c.From != "" && !isValidEmail(c.From) {
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
		Host:      "127.0.0.1",
		Port:      1025,
		AuthType:  AuthTypeNone,
		TLSPolicy: TLSPolicyMandatory,
		Timeout:   DefaultTimeout,
		From:      "no-reply@acme.co",
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

	authType, err := parseAuthType(cfg.AuthType)
	if err != nil {
		return nil, err
	}

	if authType == gomail.SMTPAuthCustom {
		if len(customAuth) == 0 {
			return nil, ErrInvalidAuthType
		}
		clientOpts = append(clientOpts, customAuth...)
	} else {
		clientOpts = append(clientOpts, gomail.WithSMTPAuth(authType))
	}

	policy, err := parseTLSPolicy(cfg.TLSPolicy)
	if err != nil {
		return nil, err
	}
	clientOpts = append(clientOpts, gomail.WithTLSPolicy(policy))

	// TLSConfig() returns nil when TLSEnable is false; fall back to go-mail's defaults
	tlsConfig, err := cfg.ClientConfig.TLSConfig()
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidTLSConfig, err)
	}
	if tlsConfig == nil {
		tlsConfig = &tls.Config{}
	}
	if tlsConfig.ServerName == "" {
		tlsConfig.ServerName = cfg.Host
	}
	if tlsConfig.MinVersion == 0 {
		tlsConfig.MinVersion = gomail.DefaultTLSMinVersion
	}
	clientOpts = append(clientOpts, gomail.WithTLSConfig(tlsConfig))

	if cfg.SSLOnConnect {
		clientOpts = append(clientOpts, gomail.WithSSL())
	}

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = DefaultTimeout
	}
	clientOpts = append(clientOpts,
		gomail.WithTimeout(timeout.Std()),
		gomail.WithDialContextFunc(dialContext(cfg.SSLOnConnect, tlsConfig, timeout.Std())),
	)

	client, err := gomail.NewClient(cfg.Host, clientOpts...)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrCreatingClient, err)
	}

	return &Mailer{
		config: cfg,
		client: client,
	}, nil
}

// dialContext returns a dial function that applies timeout to the whole SMTP conversation.
// go-mail's own timeout only covers establishing the connection, leaving the server greeting,
// EHLO, STARTTLS and authentication unbounded; the deadline set here covers those, and is
// extended by go-mail before each message is sent.
func dialContext(useSSL bool, tlsConfig *tls.Config, timeout time.Duration) gomail.DialContextFunc {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		dialer := &net.Dialer{}

		var conn net.Conn
		var err error
		if useSSL {
			conn, err = (&tls.Dialer{NetDialer: dialer, Config: tlsConfig}).DialContext(ctx, network, address)
		} else {
			conn, err = dialer.DialContext(ctx, network, address)
		}
		if err != nil {
			return nil, err
		}

		if err = conn.SetDeadline(time.Now().Add(timeout)); err != nil {
			conn.Close()
			return nil, err
		}
		return conn, nil
	}
}

// Creates a new message with support for attachments
func (m *Mailer) NewMessage(to []string, subject string, opts ...MessageOpts) (*gomail.Msg, error) {
	msg := gomail.NewMsg()

	validTo := make([]string, 0, len(to))

	for _, addr := range to {
		addr = strings.TrimSpace(addr)
		if !isValidEmail(addr) {
			return nil, ErrInvalidTo
		}
		validTo = append(validTo, addr)
	}
	msg.To(validTo...)
	msg.Subject(subject)

	// configured sender, overridable with WithFrom
	if m.config.From != "" {
		if err := msg.From(m.config.From); err != nil {
			return nil, ErrInvalidFrom
		}
	}

	for _, opt := range opts {
		opt(msg)
	}

	// configured BCC recipients are added to every message
	if m.config.Bcc != "" {
		for _, bcc := range strings.Split(m.config.Bcc, ",") {
			if err := msg.AddBcc(strings.TrimSpace(bcc)); err != nil {
				return nil, ErrInvalidBcc
			}
		}
	}

	return msg, nil
}

func (m *Mailer) Send(msg ...*gomail.Msg) error {
	return m.SendWithContext(context.Background(), msg...)
}

// SendWithContext sends the given messages over a single connection; ctx bounds the
// connection attempt, and the configured timeout bounds each message transfer
func (m *Mailer) SendWithContext(ctx context.Context, msg ...*gomail.Msg) error {
	messages := make([]*gomail.Msg, 0, len(msg))
	for _, message := range msg {
		if message != nil {
			messages = append(messages, message)
		}
	}
	if len(messages) == 0 {
		return nil
	}

	client, err := m.client.DialToSMTPClientWithContext(ctx)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrSMTPServer, err)
	}
	defer func() {
		// a failed QUIT leaves the socket open, as go-mail returns before closing it
		if closeErr := m.client.CloseWithSMTPClient(client); closeErr != nil {
			_ = client.Close()
		}
	}()

	// sent one at a time; go-mail extends the connection deadline on each call
	// and records the failure on the message itself
	errs := make([]error, 0, len(messages))
	for _, message := range messages {
		err = m.client.SendWithSMTPClient(client, message)
		if err == nil {
			continue
		}
		// a broken connection fails every remaining message; report it as such
		// instead of as a per-message delivery failure
		var sendErr *gomail.SendError
		if errors.As(err, &sendErr) && sendErr.Reason == gomail.ErrConnCheck {
			return fmt.Errorf("%w: %w", ErrSMTPServer, errors.Join(append(errs, err)...))
		}
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return fmt.Errorf("%w: %w", ErrMessage, errors.Join(errs...))
	}

	return nil
}
