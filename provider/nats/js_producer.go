package nats

import (
	"context"
	"encoding/json"
	"errors"
	"sync"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/oddbit-project/blueprint/log"
)

// JSProducerConfig configures a JetStream producer. Subject is the default
// subject used by Publish/PublishJSON. Stream describes the stream to target;
// when AutoCreateStream is true the stream is created or updated on startup.
type JSProducerConfig struct {
	JSConnectionConfig
	Subject          string       `json:"subject"`
	Stream           StreamConfig `json:"stream"`
	AutoCreateStream bool         `json:"autoCreateStream"`
}

// Validate verifies the producer configuration including the embedded
// connection config. It is called up front by NewJSProducer so invalid configs
// fail before any network round-trips.
func (c *JSProducerConfig) Validate() error {
	if err := c.JSConnectionConfig.Validate(); err != nil {
		return err
	}
	if c.Subject == "" {
		return ErrMissingProducerTopic
	}
	if c.AutoCreateStream {
		if _, err := c.Stream.toNative(); err != nil {
			return err
		}
	}
	return nil
}

// JSProducer publishes messages to a JetStream stream with server-side acks.
//
// Stream is the handle resolved (or created) during construction. It is
// exposed as a public field for callers that want to inspect or manage stream
// state; it is not used internally by Publish operations.
type JSProducer struct {
	Subject string
	Conn    *nats.Conn
	JS      jetstream.JetStream
	Stream  jetstream.Stream
	Logger  *log.Logger

	mu     sync.Mutex
	closed bool
}

// NewJSProducer creates a JetStream producer.
func NewJSProducer(cfg *JSProducerConfig, logger *log.Logger) (*JSProducer, error) {
	if cfg == nil {
		return nil, ErrNilConfig
	}
	// Fail fast before opening a connection.
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	conn, err := cfg.JSConnectionConfig.dial("natsJSProducer")
	if err != nil {
		return nil, err
	}

	js, err := jetstream.New(conn)
	if err != nil {
		conn.Close()
		return nil, err
	}

	if logger == nil {
		logger = NewProducerLogger(cfg.Subject)
	} else {
		logger = ProducerLogger(logger, cfg.Subject)
	}

	var stream jetstream.Stream
	if cfg.AutoCreateStream {
		ctx, cancel := context.WithTimeout(context.Background(), DefaultJSSetupTimeout)
		stream, err = EnsureStream(ctx, js, cfg.Stream)
		cancel()
		if err != nil {
			logger.Error(err, "Failed to ensure JetStream stream", log.KV{
				"stream": cfg.Stream.Name,
			})
			conn.Close()
			return nil, err
		}
	} else if cfg.Stream.Name != "" {
		ctx, cancel := context.WithTimeout(context.Background(), DefaultJSSetupTimeout)
		stream, err = js.Stream(ctx, cfg.Stream.Name)
		cancel()
		if err != nil {
			logger.Error(err, "Failed to look up JetStream stream", log.KV{
				"stream": cfg.Stream.Name,
			})
			conn.Close()
			return nil, err
		}
	}

	return &JSProducer{
		Subject: cfg.Subject,
		Conn:    conn,
		JS:      js,
		Stream:  stream,
		Logger:  logger,
	}, nil
}

// IsConnected reports whether the underlying connection is up and the
// producer has not been disconnected.
func (p *JSProducer) IsConnected() bool {
	if p == nil || p.Conn == nil {
		return false
	}
	p.mu.Lock()
	closed := p.closed
	p.mu.Unlock()
	if closed {
		return false
	}
	return p.Conn.IsConnected()
}

// Disconnect drains and closes the underlying connection. Safe to call
// multiple times.
func (p *JSProducer) Disconnect() {
	if p == nil {
		return
	}

	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return
	}
	p.closed = true
	conn := p.Conn
	p.mu.Unlock()

	if conn == nil || conn.IsDraining() {
		return
	}
	if p.Logger != nil {
		p.Logger.Info("Closing JetStream producer connection", log.KV{
			"subject": p.Subject,
		})
	}
	if err := conn.Drain(); err != nil && p.Logger != nil {
		p.Logger.Error(err, "Error during NATS connection drain", nil)
	}
	conn.Close()
	// Intentionally do not set p.Conn = nil; see note in JSConsumer.Disconnect.
}

// Publish publishes data to the configured subject and waits for the server
// ack.
func (p *JSProducer) Publish(ctx context.Context, data []byte) (*jetstream.PubAck, error) {
	return p.PublishMsg(ctx, p.Subject, data)
}

// PublishMsg publishes data to an explicit subject.
func (p *JSProducer) PublishMsg(ctx context.Context, subject string, data []byte) (*jetstream.PubAck, error) {
	if p == nil {
		return nil, errors.New("publisher is nil")
	}
	if !p.IsConnected() {
		return nil, ErrProducerClosed
	}
	ack, err := p.JS.Publish(ctx, subject, data)
	if err != nil {
		if p.Logger != nil {
			p.Logger.Error(err, "Failed to publish JetStream message", log.KV{
				"subject":      subject,
				"message_size": len(data),
			})
		}
		return nil, err
	}
	return ack, nil
}

// PublishJSON marshals data as JSON and publishes to the configured subject.
func (p *JSProducer) PublishJSON(ctx context.Context, data interface{}) (*jetstream.PubAck, error) {
	payload, err := json.Marshal(data)
	if err != nil {
		if p.Logger != nil {
			p.Logger.Error(err, "Failed to marshal JSON for JetStream publish", nil)
		}
		return nil, err
	}
	return p.Publish(ctx, payload)
}

// PublishAsync publishes without waiting for ack in-line; the returned
// PubAckFuture exposes a channel for the ack. The caller-supplied context is
// checked up front (the underlying jetstream async publish has no ctx
// parameter), so callers should also select on the future's channel for
// cancellation after dispatch.
func (p *JSProducer) PublishAsync(ctx context.Context, subject string, data []byte) (jetstream.PubAckFuture, error) {
	if p == nil {
		return nil, errors.New("publisher is nil")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !p.IsConnected() {
		return nil, ErrProducerClosed
	}
	return p.JS.PublishAsync(subject, data)
}
