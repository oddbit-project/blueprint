package nats

import (
	"context"
	"errors"
	"github.com/nats-io/nats.go"
	"github.com/oddbit-project/blueprint/crypt/secure"
	"github.com/oddbit-project/blueprint/log"
	tlsProvider "github.com/oddbit-project/blueprint/provider/tls"
	"slices"
	"sync"
	"time"
)

// ConsumerOptions additional consumer options
type ConsumerOptions struct {
	QueueGroup   string `json:"queueGroup"`   // QueueGroup for distributing messages among subscribers
	PingInterval uint   `json:"pingInterval"` // PingInterval value in seconds, defaults to 2 minutes
	MaxPingsOut  uint   `json:"maxPingsOut"`  // MaxPingsOut value, defaults to 2
	Timeout      uint   `json:"timeout"`      // Connection timeout in milliseconds, defaults to 2000
	DrainTimeout uint   `json:"drainTimeout"` // Drain timeout in milliseconds, defaults to 30000; bounds Disconnect
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
	URL       string
	Subject   string
	Queue     string
	Conn      *nats.Conn
	Logger    *log.Logger
	subs      []*nats.Subscription // SubscribeSync subscriptions
	handlers  map[*nats.Subscription]*handlerSub
	handlerWg sync.WaitGroup
	closing   bool          // Disconnect has started; guarded by subsLock
	stopped   chan struct{} // closed when the first Disconnect returns; guarded by subsLock
	subsLock  sync.Mutex
}

// handlerSub is a Subscribe subscription and the channel its handler goroutine reads
type handlerSub struct {
	sub       *nats.Subscription
	ch        chan *nats.Msg
	closeOnce sync.Once
}

// closeChan closes the handler channel; the caller must ensure nats.go can no longer
// deliver to it (the subscription was removed), or the read loop panics
func (h *handlerSub) closeChan() {
	h.closeOnce.Do(func() { close(h.ch) })
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
	if c.DrainTimeout > 0 {
		opts.DrainTimeout = time.Duration(c.DrainTimeout) * time.Millisecond
	}
}

// Validate checks if the consumer configuration is valid
func (c ConsumerConfig) Validate() error {
	if len(serverURLs(c.URL)) == 0 {
		return ErrMissingConsumerURL
	}
	if len(c.Subject) == 0 {
		return ErrMissingConsumerTopic
	}
	if !slices.Contains(validAuthTypes, c.AuthType) {
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

	if cfg.ConsumerName == "" {
		cfg.ConsumerName = "natsConsumer"
	}

	// Create logger if not provided
	if logger == nil {
		logger = NewConsumerLogger(cfg.Subject, cfg.QueueGroup)
	} else {
		logger = ConsumerLogger(logger, cfg.Subject, cfg.QueueGroup)
	}

	// Connect to NATS via shared helper
	conn, err := connect(connectParams{
		URL:          cfg.URL,
		Name:         cfg.ConsumerName,
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

	return &Consumer{
		URL:      cfg.URL,
		Subject:  cfg.Subject,
		Queue:    cfg.QueueGroup,
		Conn:     conn,
		Logger:   logger,
		subs:     make([]*nats.Subscription, 0),
		handlers: make(map[*nats.Subscription]*handlerSub),
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

// Disconnect stops the subscriptions, lets the Subscribe handlers finish the messages
// already delivered while the connection is still open, then drains and closes the
// connection. Handlers get up to drainTimeout; closing the connection normally takes
// up to 5 seconds more. Once Disconnect has started, Subscribe and SubscribeSync return
// ErrConsumerClosed, and a second Disconnect waits for this one to finish.
//
// Keep the context passed to Subscribe live until Disconnect returns: a handler stops
// when it is cancelled and its buffered messages are dropped. Do not call Disconnect
// from inside a handler: it waits for every handler, including the caller, so it sits
// out the whole drainTimeout.
func (c *Consumer) Disconnect() {
	if c == nil {
		return
	}

	c.subsLock.Lock()
	conn := c.Conn
	if conn == nil {
		c.subsLock.Unlock()
		return
	}
	if c.closing {
		// another Disconnect is running; return when it has finished
		stopped := c.stopped
		c.subsLock.Unlock()
		<-stopped
		return
	}
	c.closing = true
	c.stopped = make(chan struct{})
	defer close(c.stopped)
	syncSubs := c.subs
	c.subs = make([]*nats.Subscription, 0)
	handlers := make([]*handlerSub, 0, len(c.handlers))
	for _, h := range c.handlers {
		handlers = append(handlers, h)
	}
	c.subsLock.Unlock()

	if c.Logger != nil {
		c.Logger.Info("Closing consumer connection", log.KV{
			"subject": c.Subject,
			"queue":   c.Queue,
		})
	}

	deadline := time.Now().Add(conn.Opts.DrainTimeout)

	// messages left on a SubscribeSync subscription cannot be read through the
	// provider once Disconnect has started
	for _, sub := range syncSubs {
		if err := sub.Unsubscribe(); err != nil && c.Logger != nil {
			c.Logger.Error(err, "Error unsubscribing from NATS subject", log.KV{
				"subject": sub.Subject,
			})
		}
	}

	// the server stops sending; messages in flight are still delivered
	for _, h := range handlers {
		if err := h.sub.Drain(); err != nil && c.Logger != nil {
			c.Logger.Error(err, "Error draining NATS subscription", log.KV{
				"subject": h.sub.Subject,
			})
		}
	}

	// nats.go removes a drained channel subscription after one flush round trip;
	// only then can nothing more be written to its channel
	for _, h := range handlers {
		for h.sub.IsValid() && time.Now().Before(deadline) {
			time.Sleep(10 * time.Millisecond)
		}
	}

	// the handlers work through their buffer while the connection is open, so
	// replies and handler publishes still go out
	for _, h := range handlers {
		if !h.sub.IsValid() {
			h.closeChan()
		}
	}
	done := make(chan struct{})
	go func() {
		c.handlerWg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Until(deadline)):
	}

	if err := conn.Drain(); err != nil && c.Logger != nil {
		c.Logger.Error(err, "Error during NATS connection drain", nil)
	}
	waitClosed(conn, drainFlushTimeout)

	// the closed connection no longer delivers to any subscription
	for _, h := range handlers {
		h.closeChan()
	}

	c.subsLock.Lock()
	c.Conn = nil
	c.subsLock.Unlock()
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

// Subscribe subscribes to the subject and processes messages with the handler function.
// The handler stops when ctx is cancelled; for a graceful Disconnect keep ctx live until
// Disconnect returns, and do not call Disconnect from inside the handler
func (c *Consumer) Subscribe(ctx context.Context, handler ConsumerFunc) error {
	c.subsLock.Lock()
	defer c.subsLock.Unlock()
	if c.closing || !c.IsConnected() {
		return ErrConsumerClosed
	}
	conn := c.Conn

	// Create a message channel
	msgChan := make(chan *nats.Msg, 100)

	// Subscribe to the subject
	var sub *nats.Subscription
	var err error

	if c.Queue != "" {
		// Queue subscription
		sub, err = conn.QueueSubscribeSyncWithChan(c.Subject, c.Queue, msgChan)
	} else {
		// Regular subscription
		sub, err = conn.ChanSubscribe(c.Subject, msgChan)
	}

	if err != nil {
		close(msgChan)
		c.Logger.Error(err, "Failed to subscribe to NATS subject", log.KV{
			"subject": c.Subject,
			"queue":   c.Queue,
		})
		return err
	}

	h := &handlerSub{sub: sub, ch: msgChan}
	if c.handlers == nil {
		c.handlers = make(map[*nats.Subscription]*handlerSub)
	}
	c.handlers[sub] = h
	c.handlerWg.Add(1)

	// Log subscription
	c.Logger.Info("Subscribed to NATS subject", log.KV{
		"subject": c.Subject,
		"queue":   c.Queue,
	})

	// Process messages in a goroutine
	go func() {
		defer c.handlerWg.Done()
		defer func() {
			// the channel is closed here only once nats.go can no longer deliver to
			// it; otherwise Disconnect closes it
			if !sub.IsValid() {
				h.closeChan()
			} else if err := sub.Unsubscribe(); err != nil {
				// expected while Disconnect drains, or once it has removed the
				// subscription or closed the connection
				if !errors.Is(err, nats.ErrConnectionDraining) &&
					!errors.Is(err, nats.ErrBadSubscription) &&
					!errors.Is(err, nats.ErrConnectionClosed) {
					c.Logger.Error(err, "Failed to unsubscribe from NATS subject", log.KV{
						"subject": c.Subject,
					})
				}
			} else {
				h.closeChan()
			}

			c.subsLock.Lock()
			delete(c.handlers, sub)
			c.subsLock.Unlock()
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
					if err := conn.Publish(msg.Reply, nil); err != nil {
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
	c.subsLock.Lock()
	defer c.subsLock.Unlock()
	if c.closing || !c.IsConnected() {
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
	c.subs = append(c.subs, sub)

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
