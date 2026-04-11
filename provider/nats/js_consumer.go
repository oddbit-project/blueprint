package nats

import (
	"context"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/oddbit-project/blueprint/log"
)

// JSConsumerConfig configures a JetStream pull consumer bound to an existing
// stream. Durable names create a persistent consumer; leave empty for an
// ephemeral one. FilterSubject is optional.
type JSConsumerConfig struct {
	JSConnectionConfig
	StreamName    string `json:"streamName"`
	Durable       string `json:"durable"`
	ConsumerName  string `json:"consumerName"`
	FilterSubject string `json:"filterSubject"`

	// Pull/consumer tuning
	AckPolicy     string        `json:"ackPolicy"` // "explicit" | "all" | "none"
	AckWait       time.Duration `json:"ackWait"`
	MaxDeliver    int           `json:"maxDeliver"`
	MaxAckPending int           `json:"maxAckPending"`
	DeliverPolicy string        `json:"deliverPolicy"` // "all" | "last" | "new"

	// Native overrides the derived jetstream.ConsumerConfig if set.
	Native *jetstream.ConsumerConfig `json:"-"`
}

// Validate verifies the consumer configuration including the embedded
// connection config and the enum-like policy strings. It is called up front by
// NewJSConsumer so that invalid configs fail before any network round-trips.
func (c *JSConsumerConfig) Validate() error {
	if err := c.JSConnectionConfig.Validate(); err != nil {
		return err
	}
	if c.StreamName == "" {
		return ErrMissingStreamName
	}
	if c.Native != nil {
		return nil
	}
	switch c.AckPolicy {
	case "", "explicit", "all", "none":
	default:
		return ErrInvalidAckPolicy
	}
	switch c.DeliverPolicy {
	case "", "all", "last", "new":
	default:
		return ErrInvalidDeliverPolicy
	}
	return nil
}

// JSMessage is a thin wrapper around jetstream.Msg that keeps call sites
// symmetric with the core-NATS Message type while still exposing ack methods.
type JSMessage struct {
	raw jetstream.Msg
}

// Subject returns the message subject.
func (m JSMessage) Subject() string { return m.raw.Subject() }

// Data returns the raw payload.
func (m JSMessage) Data() []byte { return m.raw.Data() }

// Headers returns the message headers.
func (m JSMessage) Headers() nats.Header { return m.raw.Headers() }

// Metadata returns JetStream delivery metadata.
func (m JSMessage) Metadata() (*jetstream.MsgMetadata, error) { return m.raw.Metadata() }

// Ack acknowledges successful processing.
func (m JSMessage) Ack() error { return m.raw.Ack() }

// Nak negatively acknowledges; the server will redeliver.
func (m JSMessage) Nak() error { return m.raw.Nak() }

// InProgress resets the ack-wait timer while still processing.
func (m JSMessage) InProgress() error { return m.raw.InProgress() }

// Term terminates the message; it will not be redelivered.
func (m JSMessage) Term() error { return m.raw.Term() }

// Raw returns the underlying jetstream.Msg for callers needing advanced APIs.
func (m JSMessage) Raw() jetstream.Msg { return m.raw }

// JSConsumerFunc is the handler type used by Consume.
type JSConsumerFunc func(ctx context.Context, msg JSMessage) error

// JSConsumer reads messages from a JetStream consumer.
type JSConsumer struct {
	StreamName string
	Conn       *nats.Conn
	JS         jetstream.JetStream
	Stream     jetstream.Stream
	Consumer   jetstream.Consumer
	Logger     *log.Logger

	mu        sync.Mutex
	consCt    jetstream.ConsumeContext
	stopWatch chan struct{}
	closed    bool
}

// NewJSConsumer creates a pull-based JetStream consumer. The target stream
// must already exist.
func NewJSConsumer(cfg *JSConsumerConfig, logger *log.Logger) (*JSConsumer, error) {
	if cfg == nil {
		return nil, ErrNilConfig
	}
	// Fail fast before opening a connection or doing server round-trips.
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	conn, err := cfg.JSConnectionConfig.dial("natsJSConsumer")
	if err != nil {
		return nil, err
	}

	js, err := jetstream.New(conn)
	if err != nil {
		conn.Close()
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), DefaultJSSetupTimeout)
	stream, err := js.Stream(ctx, cfg.StreamName)
	cancel()
	if err != nil {
		conn.Close()
		return nil, err
	}

	consCfg, err := buildJSConsumerConfig(cfg)
	if err != nil {
		conn.Close()
		return nil, err
	}

	ctx, cancel = context.WithTimeout(context.Background(), DefaultJSSetupTimeout)
	cons, err := stream.CreateOrUpdateConsumer(ctx, consCfg)
	cancel()
	if err != nil {
		conn.Close()
		return nil, err
	}

	if logger == nil {
		logger = NewConsumerLogger(cfg.FilterSubject, "")
	} else {
		logger = ConsumerLogger(logger, cfg.FilterSubject, "")
	}

	return &JSConsumer{
		StreamName: cfg.StreamName,
		Conn:       conn,
		JS:         js,
		Stream:     stream,
		Consumer:   cons,
		Logger:     logger,
	}, nil
}

func buildJSConsumerConfig(cfg *JSConsumerConfig) (jetstream.ConsumerConfig, error) {
	if cfg.Native != nil {
		return *cfg.Native, nil
	}

	out := jetstream.ConsumerConfig{
		Name:          cfg.ConsumerName,
		Durable:       cfg.Durable,
		FilterSubject: cfg.FilterSubject,
		AckWait:       cfg.AckWait,
		MaxDeliver:    cfg.MaxDeliver,
		MaxAckPending: cfg.MaxAckPending,
	}

	switch cfg.AckPolicy {
	case "", "explicit":
		out.AckPolicy = jetstream.AckExplicitPolicy
	case "all":
		out.AckPolicy = jetstream.AckAllPolicy
	case "none":
		out.AckPolicy = jetstream.AckNonePolicy
	default:
		return jetstream.ConsumerConfig{}, ErrInvalidAckPolicy
	}

	switch cfg.DeliverPolicy {
	case "", "all":
		out.DeliverPolicy = jetstream.DeliverAllPolicy
	case "last":
		out.DeliverPolicy = jetstream.DeliverLastPolicy
	case "new":
		out.DeliverPolicy = jetstream.DeliverNewPolicy
	default:
		return jetstream.ConsumerConfig{}, ErrInvalidDeliverPolicy
	}

	return out, nil
}

// IsConnected reports whether the underlying connection is up and the
// consumer has not been disconnected.
func (c *JSConsumer) IsConnected() bool {
	if c == nil || c.Conn == nil {
		return false
	}
	c.mu.Lock()
	closed := c.closed
	c.mu.Unlock()
	if closed {
		return false
	}
	return c.Conn.IsConnected()
}

// Consume starts continuous delivery; the handler is called for each message.
// If the handler returns nil the message is Ack'd automatically; on error it
// is Nak'd for redelivery.
//
// Only one concurrent Consume session is allowed per JSConsumer; calling
// Consume while a session is already active returns ErrAlreadyConsuming. The
// session ends when either the supplied ctx is cancelled or Disconnect() is
// called.
func (c *JSConsumer) Consume(ctx context.Context, handler JSConsumerFunc) error {
	if c == nil || c.Conn == nil {
		return ErrConsumerClosed
	}

	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return ErrConsumerClosed
	}
	if c.Consumer == nil {
		c.mu.Unlock()
		return ErrJSNoConsumer
	}
	if c.consCt != nil {
		c.mu.Unlock()
		return ErrAlreadyConsuming
	}
	c.mu.Unlock()

	if !c.Conn.IsConnected() {
		return ErrConsumerClosed
	}

	cc, err := c.Consumer.Consume(func(m jetstream.Msg) {
		msg := JSMessage{raw: m}
		if herr := handler(ctx, msg); herr != nil {
			if c.Logger != nil {
				c.Logger.Error(herr, "JetStream handler returned error; Nak'ing", log.KV{
					"subject": m.Subject(),
				})
			}
			if nerr := m.Nak(); nerr != nil && c.Logger != nil {
				c.Logger.Error(nerr, "Failed to Nak JetStream message", nil)
			}
			return
		}
		if aerr := m.Ack(); aerr != nil && c.Logger != nil {
			c.Logger.Error(aerr, "Failed to Ack JetStream message", log.KV{
				"subject": m.Subject(),
			})
		}
	})
	if err != nil {
		return err
	}

	stop := make(chan struct{})

	c.mu.Lock()
	if c.closed {
		// Raced with Disconnect(): tear down the consume context we just
		// created and report closed to the caller.
		c.mu.Unlock()
		cc.Stop()
		return ErrConsumerClosed
	}
	c.consCt = cc
	c.stopWatch = stop
	c.mu.Unlock()

	// Watcher goroutine: exits when either the caller's ctx is cancelled or
	// Disconnect() closes stop. Uses cc (captured) rather than c.consCt so a
	// subsequent Consume() after teardown can not confuse identities.
	go func() {
		select {
		case <-ctx.Done():
		case <-stop:
		}
		c.mu.Lock()
		shouldStop := c.consCt == cc
		if shouldStop {
			c.consCt = nil
			c.stopWatch = nil
		}
		c.mu.Unlock()
		// Call Stop outside the lock to avoid holding c.mu across any
		// potentially-blocking upstream behavior.
		if shouldStop {
			cc.Stop()
		}
	}()

	return nil
}

// Fetch pulls up to batch messages, blocking up to timeout for them to arrive.
//
// Fetch may return a non-nil slice AND a non-nil error simultaneously when the
// batch delivery was interrupted mid-stream (e.g. server error after some
// messages were already received). Callers should process the returned slice
// and handle the error separately.
func (c *JSConsumer) Fetch(batch int, timeout time.Duration) ([]JSMessage, error) {
	if c == nil || c.Conn == nil {
		return nil, ErrConsumerClosed
	}

	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil, ErrConsumerClosed
	}
	cons := c.Consumer
	c.mu.Unlock()

	if cons == nil {
		return nil, ErrJSNoConsumer
	}
	if !c.Conn.IsConnected() {
		return nil, ErrConsumerClosed
	}

	mb, err := cons.Fetch(batch, jetstream.FetchMaxWait(timeout))
	if err != nil {
		return nil, err
	}

	var out []JSMessage
	for m := range mb.Messages() {
		out = append(out, JSMessage{raw: m})
	}
	if err := mb.Error(); err != nil {
		return out, err
	}
	return out, nil
}

// Disconnect stops active consumption and drains the connection. It is safe
// to call multiple times; only the first call does work.
func (c *JSConsumer) Disconnect() {
	if c == nil {
		return
	}

	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	c.closed = true
	sw := c.stopWatch
	c.stopWatch = nil
	cc := c.consCt
	c.consCt = nil
	conn := c.Conn
	c.mu.Unlock()

	// Release the watcher goroutine and stop the consume context outside the
	// critical section.
	if sw != nil {
		close(sw)
	}
	if cc != nil {
		cc.Stop()
	}

	if conn == nil || conn.IsDraining() {
		return
	}
	if c.Logger != nil {
		c.Logger.Info("Closing JetStream consumer connection", log.KV{
			"stream": c.StreamName,
		})
	}
	if err := conn.Drain(); err != nil && c.Logger != nil {
		c.Logger.Error(err, "Error during NATS connection drain", nil)
	}
	conn.Close()
	// Intentionally do not set c.Conn = nil: nats.Conn methods are safe to call
	// after Close, IsConnected() returns false, and leaving the field set
	// eliminates read/write races with concurrent callers.
}
