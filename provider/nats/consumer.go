package nats

import (
	"context"
	"errors"
	"github.com/nats-io/nats.go"
	"github.com/oddbit-project/blueprint/crypt/secure"
	"github.com/oddbit-project/blueprint/log"
	tlsProvider "github.com/oddbit-project/blueprint/provider/tls"
	"github.com/oddbit-project/blueprint/utils/str"
	"sync"
	"time"
)

// ConsumerOptions additional consumer options
type ConsumerOptions struct {
	QueueGroup   string `json:"queueGroup"`   // QueueGroup for distributing messages among subscribers
	PingInterval uint   `json:"pingInterval"` // PingInterval value in seconds, defaults to 2 minutes
	MaxPingsOut  uint   `json:"maxPingsOut"`  // MaxPingsOut value, defaults to 2
	Timeout      uint   `json:"timeout"`      // Connection timeout in milliseconds, defaults to 2000
	DrainTimeout uint   `json:"drainTimeout"` // Drain timeout in milliseconds, defaults to 30000
}

type ConsumerConfig struct {
	URL      string `json:"url"`
	Subject  string `json:"subject"`  // Subject pattern to subscribe to
	AuthType string `json:"authType"` // Authentication type
	Username string `json:"username"` // Username for basic auth
	secure.DefaultCredentialConfig
	ConsumerName string `json:"consumerName"` // Optional consumer name
	tlsProvider.ClientConfig
	ConsumerOptions
}

// Message is a wrapper around nats.Msg to avoid exposing NATS types
type Message struct {
	Subject string
	Reply   string
	Data    []byte
	Sub     *nats.Subscription
	Headers map[string][]string
}

// ConsumerFunc is the handler type for message processing
type ConsumerFunc func(ctx context.Context, msg Message) error

type Consumer struct {
	URL      string
	Subject  string
	Queue    string
	Conn     *nats.Conn
	Logger   *log.Logger
	subs     []*nats.Subscription
	subsLock sync.Mutex
}

// ApplyOptions sets additional connection parameters
func (c ConsumerOptions) ApplyOptions(opts *nats.Options) {
	if c.PingInterval > 0 {
		opts.PingInterval = time.Duration(c.PingInterval) * time.Second
	}
	if c.MaxPingsOut > 0 {
		opts.MaxPingsOut = int(c.MaxPingsOut)
	}
	if c.Timeout > 0 {
		opts.Timeout = time.Duration(c.Timeout) * time.Millisecond
	}
}

// Validate checks if the consumer configuration is valid
func (c ConsumerConfig) Validate() error {
	if len(c.URL) == 0 {
		return ErrMissingConsumerURL
	}
	if len(c.Subject) == 0 {
		return ErrMissingConsumerTopic
	}
	if str.Contains(c.AuthType, validAuthTypes) == -1 {
		return ErrInvalidAuthType
	}

	return nil
}

// NewConsumer creates a new NATS consumer
func NewConsumer(cfg *ConsumerConfig, logger *log.Logger) (*Consumer, error) {
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

	if cfg.ConsumerName == "" {
		cfg.ConsumerName = "natsConsumer"
	}

	// Configure connection options
	opts := nats.Options{
		Url:            cfg.URL,
		AllowReconnect: true,
		MaxReconnect:   DefaultConnectRetry,
		ReconnectWait:  DefaultTimeout,
		Name:           cfg.ConsumerName,
	}

	// Apply authentication
	switch cfg.AuthType {
	case AuthTypeBasic:
		opts.User = cfg.Username
		opts.Password = password
	case AuthTypeToken:
		opts.Token = password
	}

	// Apply TLS settings
	if tls, err := cfg.TLSConfig(); err != nil {
		return nil, err
	} else if tls != nil {
		opts.TLSConfig = tls
	}

	// Apply additional options
	cfg.ConsumerOptions.ApplyOptions(&opts)

	// Create logger if not provided
	if logger == nil {
		logger = NewConsumerLogger(cfg.Subject, cfg.QueueGroup)
	} else {
		ConsumerLogger(logger, cfg.Subject, cfg.QueueGroup)
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

	return &Consumer{
		URL:     cfg.URL,
		Subject: cfg.Subject,
		Queue:   cfg.QueueGroup,
		Conn:    conn,
		Logger:  logger,
		subs:    make([]*nats.Subscription, 0),
	}, nil
}

// IsConnected returns true if the consumer is connected
func (c *Consumer) IsConnected() bool {
	// Check if consumer or connection is nil
	if c == nil || c.Conn == nil {
		return false
	}
	return c.Conn.IsConnected()
}

// Disconnect disconnects from NATS server
func (c *Consumer) Disconnect() {
	// Check if consumer is nil or already disconnected
	if c == nil || c.Conn == nil {
		return
	}

	// Check if already draining
	if c.Conn.IsDraining() {
		return
	}

	// Log disconnect if logger is available
	if c.Logger != nil {
		c.Logger.Info("Closing consumer connection", log.KV{
			"subject": c.Subject,
			"queue":   c.Queue,
		})
	}

	// Unsubscribe from all subscriptions
	c.subsLock.Lock()
	for _, sub := range c.subs {
		if err := sub.Unsubscribe(); err != nil && c.Logger != nil {
			c.Logger.Error(err, "Error unsubscribing from NATS subject", log.KV{
				"subject": sub.Subject,
			})
		}
	}
	c.subs = make([]*nats.Subscription, 0)
	c.subsLock.Unlock()

	// Use Drain for graceful shutdown
	if err := c.Conn.Drain(); err != nil && c.Logger != nil {
		c.Logger.Error(err, "Error during NATS connection drain", nil)
	}

	// Close and clean up
	c.Conn.Close()
	c.Conn = nil
}

// Convert nats.Msg to Message
func convertMessage(msg *nats.Msg) Message {
	return Message{
		Subject: msg.Subject,
		Reply:   msg.Reply,
		Data:    msg.Data,
		Sub:     msg.Sub,
		Headers: msg.Header,
	}
}

// Subscribe subscribes to the subject and processes messages with the handler function
func (c *Consumer) Subscribe(ctx context.Context, handler ConsumerFunc) error {
	if !c.IsConnected() {
		return ErrConsumerClosed
	}

	// Create a message channel
	msgChan := make(chan *nats.Msg, 100)

	// Subscribe to the subject
	var sub *nats.Subscription
	var err error

	if c.Queue != "" {
		// Queue subscription
		sub, err = c.Conn.QueueSubscribeSyncWithChan(c.Subject, c.Queue, msgChan)
	} else {
		// Regular subscription
		sub, err = c.Conn.ChanSubscribe(c.Subject, msgChan)
	}

	if err != nil {
		close(msgChan)
		c.Logger.Error(err, "Failed to subscribe to NATS subject", log.KV{
			"subject": c.Subject,
			"queue":   c.Queue,
		})
		return err
	}

	// Add subscription to the list
	c.subsLock.Lock()
	c.subs = append(c.subs, sub)
	c.subsLock.Unlock()

	// Log subscription
	c.Logger.Info("Subscribed to NATS subject", log.KV{
		"subject": c.Subject,
		"queue":   c.Queue,
	})

	// Process messages in a goroutine
	go func() {
		defer func() {
			// Unsubscribe and remove from list when done
			if err := sub.Unsubscribe(); err != nil {
				c.Logger.Error(err, "Failed to unsubscribe from NATS subject", log.KV{
					"subject": c.Subject,
				})
			}

			c.subsLock.Lock()
			for i, s := range c.subs {
				if s == sub {
					c.subs = append(c.subs[:i], c.subs[i+1:]...)
					break
				}
			}
			c.subsLock.Unlock()

			close(msgChan)
		}()

		for {
			select {
			case msg, ok := <-msgChan:
				if !ok {
					// Channel closed
					return
				}

				// Convert nats.Msg to our Message type
				message := convertMessage(msg)

				// Process message with handler
				if err := handler(ctx, message); err != nil {
					c.Logger.Error(err, "Error processing NATS message", log.KV{
						"subject": msg.Subject,
					})
				}

				// If there's a reply subject, send an empty acknowledgment (optional)
				if msg.Reply != "" {
					if err := c.Conn.Publish(msg.Reply, nil); err != nil {
						c.Logger.Error(err, "Failed to acknowledge message", log.KV{
							"reply": msg.Reply,
						})
					}
				}

			case <-ctx.Done():
				c.Logger.Info("Context canceled, stopping NATS subscription", log.KV{
					"subject": c.Subject,
				})
				return
			}
		}
	}()

	return nil
}

// SubscribeSync subscribes synchronously and returns a subscription that can be used to fetch messages
func (c *Consumer) SubscribeSync() (*nats.Subscription, error) {
	if !c.IsConnected() {
		return nil, ErrConsumerClosed
	}

	var sub *nats.Subscription
	var err error

	if c.Queue != "" {
		// Queue subscription
		sub, err = c.Conn.QueueSubscribeSync(c.Subject, c.Queue)
	} else {
		// Regular subscription
		sub, err = c.Conn.SubscribeSync(c.Subject)
	}

	if err != nil {
		c.Logger.Error(err, "Failed to subscribe synchronously to NATS subject", log.KV{
			"subject": c.Subject,
			"queue":   c.Queue,
		})
		return nil, err
	}

	// Add subscription to the list
	c.subsLock.Lock()
	c.subs = append(c.subs, sub)
	c.subsLock.Unlock()

	return sub, nil
}

// NextMsg waits for the next message on a subscription
func (c *Consumer) NextMsg(sub *nats.Subscription, timeout time.Duration) (*Message, error) {
	if !c.IsConnected() {
		return nil, ErrConsumerClosed
	}

	msg, err := sub.NextMsg(timeout)
	if err != nil {
		if errors.Is(err, nats.ErrTimeout) {
			// Timeout is a normal condition, not an error to log
			return nil, err
		}

		c.Logger.Error(err, "Error getting next NATS message", log.KV{
			"subject": sub.Subject,
		})
		return nil, err
	}

	// Convert to our Message type
	message := convertMessage(msg)
	return &message, nil
}

// Unsubscribe removes a subscription
func (c *Consumer) Unsubscribe(sub *nats.Subscription) error {
	if !c.IsConnected() {
		return ErrConsumerClosed
	}

	err := sub.Unsubscribe()
	if err != nil {
		c.Logger.Error(err, "Failed to unsubscribe from NATS subject", log.KV{
			"subject": sub.Subject,
		})
		return err
	}

	// Remove from the list
	c.subsLock.Lock()
	defer c.subsLock.Unlock()
	for i, s := range c.subs {
		if s == sub {
			c.subs = append(c.subs[:i], c.subs[i+1:]...)
			break
		}
	}

	return nil
}

// Request sends a request and waits for a response
func (c *Consumer) Request(subject string, data []byte, timeout time.Duration) (*Message, error) {
	// Check if consumer is connected
	if !c.IsConnected() {
		return nil, ErrConsumerClosed
	}

	// Make the request
	msg, err := c.Conn.Request(subject, data, timeout)
	if err != nil {
		// Only log the error if we have a logger
		if c.Logger != nil {
			c.Logger.Error(err, "Failed to send request to NATS", log.KV{
				"subject": subject,
			})
		}
		return nil, err
	}

	// Check if response is nil (shouldn't happen, but being defensive)
	if msg == nil {
		return nil, errors.New("received nil response from NATS request")
	}

	// Convert to our Message type
	message := convertMessage(msg)
	return &message, nil
}
