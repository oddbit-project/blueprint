package nats

import (
	"encoding/json"
	"errors"
	"github.com/nats-io/nats.go"
	"github.com/oddbit-project/blueprint/crypt/secure"
	"github.com/oddbit-project/blueprint/log"
	tlsProvider "github.com/oddbit-project/blueprint/provider/tls"
	"slices"
	"sync"
	"time"
)

// ProducerOptions additional producer options
type ProducerOptions struct {
	PingInterval uint `json:"pingInterval"` // PingInterval value in seconds, defaults to 2 minutes
	MaxPingsOut  uint `json:"maxPingsOut"`  // MaxPingsOut value, defaults to 2
	Timeout      uint `json:"timeout"`      // Connection timeout in milliseconds, defaults to 2000
	DrainTimeout uint `json:"drainTimeout"` // Drain timeout in milliseconds, defaults to 30000; bounds Disconnect
}

type ProducerConfig struct {
	URL      string `json:"url"`
	Subject  string `json:"subject"`
	AuthType string `json:"authType"`
	Username string `json:"username"`
	secure.DefaultCredentialConfig
	ProducerName string `json:"ProducerName"`
	tlsProvider.ClientConfig
	ProducerOptions
}

type Producer struct {
	URL     string
	Subject string
	Conn    *nats.Conn
	Logger  *log.Logger

	mu      sync.Mutex
	closing bool          // Disconnect has started; guarded by mu
	stopped chan struct{} // closed when the first Disconnect returns; guarded by mu
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
	if p.DrainTimeout > 0 {
		opts.DrainTimeout = time.Duration(p.DrainTimeout) * time.Millisecond
	}
}

func (c ProducerConfig) Validate() error {
	if len(serverURLs(c.URL)) == 0 {
		return ErrMissingProducerURL
	}
	if len(c.Subject) == 0 {
		return ErrMissingProducerTopic
	}
	if !slices.Contains(validAuthTypes, c.AuthType) {
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

	if len(cfg.ProducerName) == 0 {
		cfg.ProducerName = "natsProducer"
	}

	// Create logger if not provided
	if logger == nil {
		logger = NewProducerLogger(cfg.Subject)
	} else {
		logger = ProducerLogger(logger, cfg.Subject)
	}

	// Connect to NATS via shared helper
	conn, err := connect(connectParams{
		URL:          cfg.URL,
		Name:         cfg.ProducerName,
		AuthType:     cfg.AuthType,
		Username:     cfg.Username,
		Cred:         cfg.DefaultCredentialConfig,
		TLS:          cfg.ClientConfig,
		PingInterval: cfg.PingInterval,
		MaxPingsOut:  cfg.MaxPingsOut,
		Timeout:      cfg.Timeout,
		DrainTimeout: cfg.DrainTimeout,
	})
	if err != nil {
		logger.Error(err, "Failed to connect to NATS", log.KV{
			"url":     cfg.URL,
			"subject": cfg.Subject,
		})
		return nil, err
	}

	return &Producer{
		URL:     cfg.URL,
		Subject: cfg.Subject,
		Conn:    conn,
		Logger:  logger,
	}, nil
}

// Disconnect drains the connection to NATS and closes it; it can block up to
// drainTimeout plus 5 seconds
func (p *Producer) Disconnect() {
	if p == nil {
		return
	}

	p.mu.Lock()
	conn := p.Conn
	if conn == nil {
		p.mu.Unlock()
		return
	}
	if p.closing {
		// another Disconnect is flushing; return when it has finished
		stopped := p.stopped
		p.mu.Unlock()
		<-stopped
		return
	}
	p.closing = true
	p.stopped = make(chan struct{})
	defer close(p.stopped)
	p.mu.Unlock()

	// Log disconnect if logger is available
	if p.Logger != nil {
		p.Logger.Info("Closing producer connection", log.KV{
			"subject": p.Subject,
		})
	}

	// Use Drain for graceful shutdown: it flushes pending publishes before closing
	if err := conn.Drain(); err != nil && p.Logger != nil {
		p.Logger.Error(err, "Error during NATS connection drain", nil)
	}
	waitClosed(conn, conn.Opts.DrainTimeout+drainFlushTimeout)

	p.mu.Lock()
	p.Conn = nil
	p.mu.Unlock()
}

// IsConnected returns true if the NATS connection is connected
func (p *Producer) IsConnected() bool {
	// Check if producer or connection is nil
	if p == nil || p.Conn == nil {
		return false
	}
	return p.Conn.IsConnected()
}

// Publish publishes a message to the configured subject
func (p *Producer) Publish(data []byte) error {
	// Check for nil producer or disconnected state
	if p == nil {
		return errors.New("publisher is nil")
	}

	if !p.IsConnected() {
		if p.Logger != nil {
			p.Logger.Error(ErrProducerClosed, "Failed to publish message - producer closed", nil)
		}
		return ErrProducerClosed
	}

	// Sanity check for nil connection that might have escaped IsConnected
	if p.Conn == nil {
		return ErrProducerClosed
	}

	err := p.Conn.Publish(p.Subject, data)
	if err != nil {
		if p.Logger != nil {
			p.Logger.Error(err, "Failed to publish message to NATS", log.KV{
				"subject":      p.Subject,
				"message_size": len(data),
			})
		}
		return err
	}

	return nil
}

// PublishMsg publishes a message with a specific subject
func (p *Producer) PublishMsg(subject string, data []byte) error {
	// Check for nil producer
	if p == nil {
		return errors.New("publisher is nil")
	}

	if !p.IsConnected() {
		if p.Logger != nil {
			p.Logger.Error(ErrProducerClosed, "Failed to publish message - producer closed", nil)
		}
		return ErrProducerClosed
	}

	// Sanity check for nil connection
	if p.Conn == nil {
		return ErrProducerClosed
	}

	err := p.Conn.Publish(subject, data)
	if err != nil {
		if p.Logger != nil {
			p.Logger.Error(err, "Failed to publish message to NATS", log.KV{
				"subject":      subject,
				"message_size": len(data),
			})
		}
		return err
	}

	return nil
}

// PublishRequest publishes a request message and waits for a response
func (p *Producer) PublishRequest(subject string, reply string, data []byte) error {
	// Check for nil producer
	if p == nil {
		return errors.New("publisher is nil")
	}

	if !p.IsConnected() {
		if p.Logger != nil {
			p.Logger.Error(ErrProducerClosed, "Failed to publish request - producer closed", nil)
		}
		return ErrProducerClosed
	}

	// Sanity check for nil connection
	if p.Conn == nil {
		return ErrProducerClosed
	}

	err := p.Conn.PublishRequest(subject, reply, data)
	if err != nil {
		if p.Logger != nil {
			p.Logger.Error(err, "Failed to publish request to NATS", log.KV{
				"subject":      subject,
				"reply":        reply,
				"message_size": len(data),
			})
		}
		return err
	}

	return nil
}

// Request publishes a request message and waits for a response with a timeout
func (p *Producer) Request(subject string, data []byte, timeout time.Duration) (*nats.Msg, error) {
	// Check if producer is connected
	if !p.IsConnected() {
		if p.Logger != nil {
			p.Logger.Error(ErrProducerClosed, "Failed to make request - producer closed", nil)
		}
		return nil, ErrProducerClosed
	}

	// Make the request
	msg, err := p.Conn.Request(subject, data, timeout)
	if err != nil {
		// Only log the error if we have a logger
		if p.Logger != nil {
			p.Logger.Error(err, "Failed to make request to NATS", log.KV{
				"subject":      subject,
				"message_size": len(data),
				"timeout":      timeout,
			})
		}
		return nil, err
	}

	// Check if response is nil (shouldn't happen, but being defensive)
	if msg == nil {
		return nil, errors.New("received nil response from NATS request")
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
