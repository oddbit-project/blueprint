package kafka

import (
	"context"
	"errors"
	"github.com/oddbit-project/blueprint/log"
	tlsProvider "github.com/oddbit-project/blueprint/provider/tls"
	"github.com/oddbit-project/blueprint/utils/str"
	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl/plain"
	"github.com/segmentio/kafka-go/sasl/scram"
	"strings"
	"time"
)

// ConsumerOptions additional consumer options
type ConsumerOptions struct {
	GroupTopics            []string `json:"groupTopics"`            // GroupTopics if specified, topics to consume as a group instead of Topic
	Partition              uint     `json:"partition"`              // Partition id, used if no Group specified, defaults to 0
	QueueCapacity          uint     `json:"queueCapacity"`          // QueueCapacity, defaults to 100
	MinBytes               uint     `json:"minBytes"`               // MinBytes, defaults to 0
	MaxBytes               uint     `json:"maxBytes"`               // MaxBytes, defaults to 1048576
	MaxWait                uint     `json:"maxWait"`                // MaxWait in milliseconds, default 10.000 (10s)
	ReadBatchTimeout       uint     `json:"readBatchTimeout"`       // ReadBatchTimeout in milliseconds, default 10.000 (10s)
	HeartbeatInterval      uint     `json:"heartbeatInterval"`      // HeartbeatInterval in milliseconds, default 3000 (3s)
	CommitInterval         uint     `json:"commitInterval"`         // CommitInterval in milliseconds, default 0
	PartitionWatchInterval uint     `json:"partitionWatchInterval"` // PartitionWatchInterval in milliseconds, default 5000 (5s)
	WatchPartitionChanges  bool     `json:"watchPartitionChanges"`  // WatchPartitionChanges, default true
	SessionTimeout         uint     `json:"sessionTimeout"`         // SessionTimeout in milliseconds, default 30.000 (30s)
	RebalanceTimeout       uint     `json:"rebalanceTimeout"`       // RebalanceTimeout in milliseconds, default 30.000 (30s)
	JoinGroupBackoff       uint     `json:"joinGroupBackoff"`       // JoinGroupBackoff in milliseconds, default 5000 (5s)
	RetentionTime          uint     `json:"retentionTime"`          // RetentionTime, in milliseconds, default 86.400.000ms (24h)
	StartOffset            string   `json:"startOffset"`            // StartOffset either "first", "last", default "last"
	ReadBackoffMin         uint     `json:"readBackoffMin"`         // ReadBackoffMin in milliseconds, default 100
	ReadBackoffMax         uint     `json:"readBackoffMax"`         // ReadBackoffMax in milliseconds, default 1000 (1s)
	MaxAttempts            uint     `json:"maxAttempts"`            // MaxAttempts default 3
	IsolationLevel         string   `json:"isolationLevel"`         // IsolationLevel "uncommitted" or "committed", default "committed"
}

type ConsumerConfig struct {
	Brokers  string `json:"brokers"`
	Topic    string `json:"topic"`    // Topic to consume from, if not specified will use GroupTopics
	Group    string `json:"group"`    // Group consumer group, if not specified will use specified partition
	AuthType string `json:"authType"` // AuthType to use, one of "none", "plain", "scram256", "scram512"
	Username string `json:"username"` // Username optional username
	Password string `json:"password"` // Password optional password
	tlsProvider.ClientConfig
	ConsumerOptions
}

// Message is a type alias to avoid using kafka-go in application code
type Message = kafka.Message

// ConsumerFunc Reader handler type
type ConsumerFunc func(ctx context.Context, message Message) error

type Consumer struct {
	ctx     context.Context
	Brokers string
	Group   string
	Topic   string
	config  *kafka.ReaderConfig
	Reader  *kafka.Reader
}

// ApplyOptions set ReaderConfig additional parameters
func (c ConsumerConfig) ApplyOptions(r *kafka.ReaderConfig) {
	if c.Group != "" {
		if c.SessionTimeout > 0 {
			r.SessionTimeout = time.Duration(c.SessionTimeout) * time.Millisecond
		}
		if c.RebalanceTimeout > 0 {
			r.RebalanceTimeout = time.Duration(c.RebalanceTimeout) * time.Millisecond
		}
		if c.RetentionTime > 0 {
			r.RetentionTime = time.Duration(c.RetentionTime) * time.Millisecond
		}

		if len(c.StartOffset) > 0 {
			switch c.StartOffset {
			case "first":
				r.StartOffset = kafka.FirstOffset
			default:
				r.StartOffset = kafka.LastOffset
			}
		}
	} else {
		r.Partition = int(c.Partition)
	}

	if c.GroupTopics != nil && len(c.GroupTopics) > 0 {
		r.GroupTopics = c.GroupTopics
	}

	if c.QueueCapacity > 0 {
		r.QueueCapacity = int(c.QueueCapacity)
	}

	if c.MinBytes != 0 {
		r.MinBytes = int(c.MinBytes)
	}

	if c.MaxBytes > 0 {
		r.MaxBytes = int(c.MaxBytes)
	}

	if c.MaxWait > 0 {
		r.MaxWait = time.Duration(c.MaxWait) * time.Millisecond
	}

	if c.ReadBatchTimeout > 0 {
		r.ReadBatchTimeout = time.Duration(c.ReadBatchTimeout) * time.Millisecond
	}

	if c.HeartbeatInterval > 0 {
		r.HeartbeatInterval = time.Duration(c.HeartbeatInterval) * time.Millisecond
	}

	if c.CommitInterval > 0 {
		r.CommitInterval = time.Duration(c.CommitInterval) * time.Millisecond
	}

	if c.PartitionWatchInterval > 0 {
		r.PartitionWatchInterval = time.Duration(c.PartitionWatchInterval) * time.Millisecond
	}

	if !c.WatchPartitionChanges {
		r.WatchPartitionChanges = false
	}

	if c.JoinGroupBackoff > 0 {
		r.JoinGroupBackoff = time.Duration(c.JoinGroupBackoff) * time.Millisecond
	}

	if c.ReadBackoffMin > 0 {
		r.ReadBackoffMin = time.Duration(c.ReadBackoffMin) * time.Millisecond
	}

	if c.ReadBackoffMax > 0 {
		r.ReadBackoffMax = time.Duration(c.ReadBackoffMax) * time.Millisecond
	}

	if c.MaxAttempts > 0 {
		r.MaxAttempts = int(c.MaxAttempts)
	}

	if len(c.IsolationLevel) > 0 {
		switch c.IsolationLevel {
		case "uncommitted":
			r.IsolationLevel = kafka.ReadUncommitted
		}
	}
}

// Validate validates ConsumerConfig fields
func (c ConsumerConfig) Validate() error {
	if len(c.Brokers) == 0 {
		return ErrMissingConsumerBroker
	}
	if len(c.Topic) == 0 {
		if len(c.GroupTopics) == 0 {
			return ErrMissingConsumerTopic
		}
	}
	if str.Contains(c.AuthType, validAuthTypes) == -1 {
		return ErrInvalidAuthType
	}

	if len(c.Topic) > 0 {
		if len(c.StartOffset) > 0 {
			if str.Contains(c.StartOffset, []string{"first", "last"}) < 0 {
				return ErrInvalidStartOffset
			}
		}
	}

	if len(c.IsolationLevel) > 0 {
		if str.Contains(c.StartOffset, []string{"uncommitted", "committed"}) < 0 {
			return ErrInvalidIsolationLevel
		}
	}

	return nil
}

func NewConsumer(ctx context.Context, cfg *ConsumerConfig) (*Consumer, error) {
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

	// apply extra config options
	cfg.ApplyOptions(cfgReader)

	return &Consumer{
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
func (c *Consumer) GetConfig() *kafka.ReaderConfig {
	return c.config
}

// Rewind Read messages from the beginning
func (c *Consumer) Rewind() error {
	if c.Reader == nil {
		c.config.StartOffset = kafka.FirstOffset
		return nil
	}
	return ErrConsumerAlreadyConnected
}

// Connect to Kafka broker
func (c *Consumer) Connect() {
	c.Reader = kafka.NewReader(*c.config)
}

// Disconnect Diconnect from kafka
func (c *Consumer) Disconnect() {
	if c.Reader != nil {
		c.Reader.Close()
		c.Reader = nil
	}
}

// IsConnected Returns true if Reader was initialized
func (c *Consumer) IsConnected() bool {
	return c.Reader != nil
}

// Subscribe consumes a message from a topic using a handler
// Note: this function is blocking
func (c *Consumer) Subscribe(handler ConsumerFunc) error {
	logger := log.NewKafkaConsumerLogger(c.ctx, c.Topic, c.Group)
	
	if !c.IsConnected() {
		logger.Info("Connecting to Kafka consumer before subscription", nil)
		if err := c.Connect(); err != nil {
			logger.Error(err, "Failed to connect before subscription", nil)
			return err
		}
	}
	
	logger.Info("Starting Kafka message subscription", nil)
	defer c.Reader.Close()
	
	for {
		msg, err := c.Reader.ReadMessage(c.ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				logger.Info("Kafka subscription context canceled, shutting down gracefully", nil)
				return nil
			}
			
			logger.Error(err, "Error reading Kafka message", nil)
			return err
		}
		
		// Log received message
		log.LogKafkaMessageReceived(c.ctx, msg, c.Group)
		
		// Process message with handler
		if err := handler(c.ctx, msg); err != nil {
			logger.Error(err, "Handler error processing Kafka message", map[string]interface{}{
				"topic":     msg.Topic,
				"partition": msg.Partition,
				"offset":    msg.Offset,
			})
			return err
		}
	}
}

// ReadMessage reads a single message from Kafka
// It returns the Kafka message and an error
// If there is no message available, it will block until a message is available
// If an error occurs, it will be returned
func (c *Consumer) ReadMessage() (Message, error) {
	if !c.IsConnected() {
		c.Connect()
	}
	return c.Reader.ReadMessage(c.ctx)
}

// ChannelSubscribe subscribes to a reader handler by channel
// Note: This function is blocking
func (c *Consumer) ChannelSubscribe(ch chan Message) error {
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
func (c *Consumer) SubscribeWithOffsets(handler ConsumerFunc) error {
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
