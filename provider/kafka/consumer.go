package kafka

import (
	"context"
	"errors"
	tlsProvider "github.com/oddbit-project/blueprint/provider/tls"
	"github.com/oddbit-project/blueprint/utils/str"
	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl/plain"
	"github.com/segmentio/kafka-go/sasl/scram"
	"strings"
)

type ConsumerConfig struct {
	Brokers  string `json:"brokers"`
	Topic    string `json:"topic"`
	Group    string `json:"group"`
	AuthType string `json:"authType"`
	Username string `json:"username"`
	Password string `json:"password"`
	tlsProvider.ClientConfig
}

// Message is a type alias to avoid using kafka-go in application code
type Message = kafka.Message

// ConsumerFunc Reader handler type
type ConsumerFunc func(ctx context.Context, message Message) error

type KafkaConsumer struct {
	ctx     context.Context
	Brokers string
	Group   string
	Topic   string
	config  *kafka.ReaderConfig
	Reader  *kafka.Reader
}

func (c ConsumerConfig) Validate() error {
	if len(c.Brokers) == 0 {
		return ErrMissingConsumerBroker
	}
	if len(c.Topic) == 0 {
		return ErrMissingConsumerTopic
	}
	if len(c.Group) == 0 {
		return ErrMissingConsumerGroup
	}
	if str.Contains(c.AuthType, validAuthTypes) == -1 {
		return ErrInvalidAuthType
	}
	return nil
}

func NewConsumer(ctx context.Context, cfg *ConsumerConfig) (*KafkaConsumer, error) {
	if cfg == nil {
		return nil, ErrNilConfig
	}
	// check if config has errors
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	dialer := &kafka.Dialer{
		DualStack: true,
		Timeout:   DefaultTimeout,
	}

	switch cfg.AuthType {
	case AuthTypePlain:
		dialer.SASLMechanism = plain.Mechanism{
			Username: cfg.Username,
			Password: cfg.Password,
		}
	case AuthTypeScram256:
		if sasl, err := scram.Mechanism(scram.SHA256, cfg.Username, cfg.Password); err != nil {
			return nil, err
		} else {
			dialer.SASLMechanism = sasl
		}
	case AuthTypeScram512:
		if sasl, err := scram.Mechanism(scram.SHA512, cfg.Username, cfg.Password); err != nil {
			return nil, err
		} else {
			dialer.SASLMechanism = sasl
		}
	}

	if tls, err := cfg.TLSConfig(); err != nil {
		return nil, err
	} else {
		dialer.TLS = tls
	}

	cfgReader := &kafka.ReaderConfig{
		Brokers: strings.Split(cfg.Brokers, ","),
		GroupID: cfg.Group,
		Topic:   cfg.Topic,
		Dialer:  dialer,
	}

	return &KafkaConsumer{
		ctx:     ctx,
		config:  cfgReader,
		Brokers: cfg.Brokers,
		Topic:   cfg.Topic,
		Group:   cfg.Group,
		Reader:  nil,
	}, nil
}

// GetConfig Get initial config object
// Useful to set other options before connect
func (c *KafkaConsumer) GetConfig() *kafka.ReaderConfig {
	return c.config
}

// Rewind Read messages from the beginning
func (c *KafkaConsumer) Rewind() error {
	if c.Reader == nil {
		c.config.StartOffset = kafka.FirstOffset
		return nil
	}
	return ErrConsumerAlreadyConnected
}

// Connect to Kafka broker
func (c *KafkaConsumer) Connect() {
	c.Reader = kafka.NewReader(*c.config)
}

// Disconnect Diconnect from kafka
func (c *KafkaConsumer) Disconnect() {
	if c.Reader != nil {
		c.Reader.Close()
		c.Reader = nil
	}
}

// IsConnected Returns true if Reader was initialized
func (c *KafkaConsumer) IsConnected() bool {
	return c.Reader != nil
}

// Subscribe consumes a message from a topic using a handler
// Note: this function is blocking
func (c *KafkaConsumer) Subscribe(handler ConsumerFunc) error {
	if !c.IsConnected() {
		c.Connect()
	}
	defer c.Reader.Close()
	for {
		msg, err := c.Reader.ReadMessage(c.ctx)
		if err != nil {
			if !errors.Is(err, context.Canceled) {
				return err
			}
			return nil
		}
		if err := handler(c.ctx, msg); err != nil {
			return err
		}
	}
}

// ReadMessage reads a single message from Kafka
// It returns the Kafka message and an error
// If there is no message available, it will block until a message is available
// If an error occurs, it will be returned
func (c *KafkaConsumer) ReadMessage() (Message, error) {
	if !c.IsConnected() {
		c.Connect()
	}
	return c.Reader.ReadMessage(c.ctx)
}

// ChannelSubscribe subscribes to a reader handler by channel
// Note: This function is blocking
func (c *KafkaConsumer) ChannelSubscribe(ch chan Message) error {
	if !c.IsConnected() {
		c.Connect()
	}
	defer c.Reader.Close()

	for {
		msg, err := c.Reader.ReadMessage(c.ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				// clean exit
				return nil
			}
			return err
		}
		ch <- msg
	}
}

// SubscribeWithOffsets manages a reader handler that explicitly commits offsets
// Note: this function is blocking
func (c *KafkaConsumer) SubscribeWithOffsets(handler ConsumerFunc) error {
	if !c.IsConnected() {
		c.Connect()
	}
	defer c.Reader.Close()
	for {
		msg, err := c.Reader.FetchMessage(c.ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				// clean exit
				return nil
			}
			return err
		}
		if err := handler(c.ctx, msg); err != nil {
			return err
		}
	}
}
