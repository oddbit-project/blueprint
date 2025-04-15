package nats

import (
	"encoding/json"
	"github.com/nats-io/nats.go"
	"github.com/oddbit-project/blueprint/crypt/secure"
	"github.com/oddbit-project/blueprint/log"
	tlsProvider "github.com/oddbit-project/blueprint/provider/tls"
	"github.com/oddbit-project/blueprint/utils/str"
	"time"
)

// ProducerOptions additional producer options
type ProducerOptions struct {
	PingInterval uint `json:"pingInterval"` // PingInterval value in seconds, defaults to 2 minutes
	MaxPingsOut  uint `json:"maxPingsOut"`  // MaxPingsOut value, defaults to 2
	Timeout      uint `json:"timeout"`      // Connection timeout in milliseconds, defaults to 2000
	DrainTimeout uint `json:"drainTimeout"` // Drain timeout in milliseconds, defaults to 30000
}

type ProducerConfig struct {
	URL      string `json:"url"`
	Subject  string `json:"subject"`
	AuthType string `json:"authType"`
	Username string `json:"username"`
	secure.DefaultCredentialConfig
	Token        string `json:"token"`
	ProducerName string `json:"ProducerName"`
	tlsProvider.ClientConfig
	ProducerOptions
}

type Producer struct {
	URL     string
	Subject string
	Conn    *nats.Conn
	Logger  *log.Logger
}

// ApplyOptions sets additional connection parameters
func (p ProducerOptions) ApplyOptions(opts *nats.Options) {
	if p.PingInterval > 0 {
		opts.PingInterval = time.Duration(p.PingInterval) * time.Second
	}
	if p.MaxPingsOut > 0 {
		opts.MaxPingsOut = int(p.MaxPingsOut)
	}
	if p.Timeout > 0 {
		opts.Timeout = time.Duration(p.Timeout) * time.Millisecond
	}
}

func (c ProducerConfig) Validate() error {
	if len(c.URL) == 0 {
		return ErrMissingProducerURL
	}
	if len(c.Subject) == 0 {
		return ErrMissingProducerTopic
	}
	if str.Contains(c.AuthType, validAuthTypes) == -1 {
		return ErrInvalidAuthType
	}

	return nil
}

func NewProducer(cfg *ProducerConfig, logger *log.Logger) (*Producer, error) {
	if cfg == nil {
		return nil, ErrNilConfig
	}
	// check if config has errors
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	var key []byte
	var credential *secure.Credential
	var password string
	var err error

	if cfg.AuthType == AuthTypeBasic {
		key, err = secure.GenerateKey()
		if err != nil {
			return nil, err
		}
		if credential, err = secure.CredentialFromConfig(cfg.DefaultCredentialConfig, key, true); err != nil {
			return nil, err
		}
		password, err = credential.Get()
		if err != nil {
			return nil, err
		}
	}

	// Configure connection options
	if len(cfg.ProducerName) == 0 {
		cfg.ProducerName = "natsProducer"
	}
	opts := nats.Options{
		Url:            cfg.URL,
		AllowReconnect: true,
		MaxReconnect:   DefaultConnectRetry,
		ReconnectWait:  DefaultTimeout,
		Name:           cfg.ProducerName,
	}

	// Apply authentication
	switch cfg.AuthType {
	case AuthTypeBasic:
		opts.User = cfg.Username
		opts.Password = password
	case AuthTypeToken:
		opts.Token = cfg.Token
	}

	// Apply TLS settings
	if tls, err := cfg.TLSConfig(); err != nil {
		return nil, err
	} else if tls != nil {
		opts.TLSConfig = tls
	}

	// Apply additional options
	cfg.ProducerOptions.ApplyOptions(&opts)

	// Create logger if not provided
	if logger == nil {
		logger = NewProducerLogger(cfg.Subject)
	} else {
		ProducerLogger(logger, cfg.Subject)
	}

	// Connect to NATS
	conn, err := opts.Connect()
	if err != nil {
		logger.Error(err, "Failed to connect to NATS", log.KV{
			"url":     cfg.URL,
			"subject": cfg.Subject,
		})
		return nil, err
	}

	// Clean up credentials from memory if used
	if credential != nil {
		credential.Clear()
	}

	return &Producer{
		URL:     cfg.URL,
		Subject: cfg.Subject,
		Conn:    conn,
		Logger:  logger,
	}, nil
}

// Disconnect closes the connection to NATS
func (p *Producer) Disconnect() {
	if p.Conn != nil {
		if p.Conn.IsDraining() {
			return
		}
		p.Logger.Info("Closing producer connection", log.KV{
			"subject": p.Subject,
		})
		// Use Drain for graceful shutdown
		if err := p.Conn.Drain(); err != nil {
			p.Logger.Error(err, "Error during NATS connection drain", nil)
		}
		p.Conn.Close()
		p.Conn = nil
	}
}

// IsConnected returns true if the NATS connection is connected
func (p *Producer) IsConnected() bool {
	return p.Conn != nil && p.Conn.IsConnected()
}

// Publish publishes a message to the configured subject
func (p *Producer) Publish(data []byte) error {
	if !p.IsConnected() {
		p.Logger.Error(ErrProducerClosed, "Failed to publish message - producer closed", nil)
		return ErrProducerClosed
	}

	err := p.Conn.Publish(p.Subject, data)
	if err != nil {
		p.Logger.Error(err, "Failed to publish message to NATS", log.KV{
			"subject":      p.Subject,
			"message_size": len(data),
		})
		return err
	}

	return nil
}

// PublishMsg publishes a message with a specific subject
func (p *Producer) PublishMsg(subject string, data []byte) error {
	if !p.IsConnected() {
		p.Logger.Error(ErrProducerClosed, "Failed to publish message - producer closed", nil)
		return ErrProducerClosed
	}

	err := p.Conn.Publish(subject, data)
	if err != nil {
		p.Logger.Error(err, "Failed to publish message to NATS", log.KV{
			"subject":      subject,
			"message_size": len(data),
		})
		return err
	}

	return nil
}

// PublishRequest publishes a request message and waits for a response
func (p *Producer) PublishRequest(subject string, reply string, data []byte) error {
	if !p.IsConnected() {
		p.Logger.Error(ErrProducerClosed, "Failed to publish request - producer closed", nil)
		return ErrProducerClosed
	}

	err := p.Conn.PublishRequest(subject, reply, data)
	if err != nil {
		p.Logger.Error(err, "Failed to publish request to NATS", log.KV{
			"subject":      subject,
			"reply":        reply,
			"message_size": len(data),
		})
		return err
	}

	return nil
}

// Request publishes a request message and waits for a response with a timeout
func (p *Producer) Request(subject string, data []byte, timeout time.Duration) (*nats.Msg, error) {
	if !p.IsConnected() {
		p.Logger.Error(ErrProducerClosed, "Failed to make request - producer closed", nil)
		return nil, ErrProducerClosed
	}

	msg, err := p.Conn.Request(subject, data, timeout)
	if err != nil {
		p.Logger.Error(err, "Failed to make request to NATS", log.KV{
			"subject":      subject,
			"message_size": len(data),
			"timeout":      timeout,
		})
		return nil, err
	}

	return msg, nil
}

// PublishJSON publishes a struct as JSON to the configured subject
func (p *Producer) PublishJSON(data interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		p.Logger.Error(err, "Failed to marshal JSON for NATS publication", nil)
		return err
	}

	return p.Publish(jsonData)
}

// PublishJSONMsg publishes a struct as JSON to a specific subject
func (p *Producer) PublishJSONMsg(subject string, data interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		p.Logger.Error(err, "Failed to marshal JSON for NATS publication", nil)
		return err
	}

	return p.PublishMsg(subject, jsonData)
}

// RequestJSON publishes a JSON request and waits for a response with a timeout
func (p *Producer) RequestJSON(subject string, data interface{}, timeout time.Duration) (*nats.Msg, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		p.Logger.Error(err, "Failed to marshal JSON for NATS request", nil)
		return nil, err
	}

	return p.Request(subject, jsonData, timeout)
}
