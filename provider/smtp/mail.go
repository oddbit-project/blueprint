package smtp

import (
	"crypto/tls"
	"path/filepath"
	"strings"

	"github.com/oddbit-project/blueprint/config"
	"github.com/rs/zerolog/log"
	"github.com/wneessen/go-mail"
)

const (
	ConfigKey = "smtpServer"
)

type Mailer struct {
	Host     string `json:"host,omitempty"`
	Port     int    `json:"port,omitempty"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	TLS      bool   `json:"TLS,omitempty"`
	From     string `json:"from,omitempty"`
	Bcc      string `json:"bcc,omitempty"`
}

// File attachment
type Attachment struct {
	FilePath string
	Name     string
}

// mailer return default config
func mailer() *Mailer {
	return &Mailer{
		Host:     "127.0.0.1",
		Port:     1025,
		Username: "",
		Password: "",
		TLS:      false,
		From:     "no-reply@acme.co",
		Bcc:      "",
	}
}

func NewMailer(config config.ConfigInterface) (*Mailer, error) {
	m := mailer()
	if err := config.GetKey(ConfigKey, m); err != nil {
		return nil, err
	}
	return m, nil
}

// NewHtmlMessage creates a new HTML message with support for attachments
func (m *Mailer) NewHtmlMessage(from string, to string, bcc string, subject string, content string, attachments []Attachment) *mail.Msg {
	message := mail.NewMsg()

	// Set From header
	if err := message.From(m.From); err != nil {
		log.Error().Str("From", m.From).Err(err).Msg("Failed to set From header")
		return nil
	}

	// Set To headers
	for _, t := range strings.Split(to, ",") {
		if err := message.AddTo(strings.TrimSpace(t)); err != nil {
			log.Error().Str("To", t).Err(err).Msg("Failed to set To header")
			return nil
		}
	}

	// Set BCC headers from config
	if len(m.Bcc) > 0 {
		bccList := strings.Split(m.Bcc, ",")
		for i := range bccList {
			if err := message.AddBcc(strings.TrimSpace(bccList[i])); err != nil {
				log.Error().Str("Bcc", bccList[i]).Err(err).Msg("Failed to set Bcc header")
				return nil
			}
		}
	}
	message.Subject(subject)
	message.SetBodyString(mail.TypeTextHTML, content)

	// Add attachments
	if len(attachments) > 0 {
		for _, attachment := range attachments {
			fileName := attachment.Name
			if fileName == "" {
				fileName = filepath.Base(attachment.FilePath)
			}
			message.AttachFile(attachment.FilePath, mail.WithFileName(fileName))

			log.Info().Str("Attachment", fileName).Str("Path", attachment.FilePath).Msg("File attached successfully")
		}
	}
	return message
}

func (m *Mailer) Send(msg ...*mail.Msg) error {
	var d *mail.Client
	var err error
	if m.TLS {
		d, err = mail.NewClient(m.Host,
			mail.WithPort(m.Port),
			mail.WithUsername(m.Username),
			mail.WithPassword(m.Password),
			mail.WithSMTPAuth(mail.SMTPAuthPlain),
			mail.WithTLSPortPolicy(mail.TLSMandatory),
			mail.WithTLSConfig(&tls.Config{InsecureSkipVerify: true}),
		)
	} else {
		d, err = mail.NewClient(m.Host,
			mail.WithPort(m.Port),
			mail.WithUsername(m.Username),
			mail.WithPassword(m.Password),
			mail.WithSMTPAuth(mail.SMTPAuthPlain),
		)
	}

	if err != nil {
		log.Error().Err(err).Msg("Failed to create new mail delivery client")
		return err
	}

	for _, message := range msg {
		if message != nil {
			log.Info().Str("To", strings.Join(message.GetToString(), ",")).Msg("Sending email message...")
			if err := d.DialAndSend(message); err != nil {
				log.Error().Err(err).Msg("Failed to send email message")
				return err
			}
		}
	}
	return nil
}
