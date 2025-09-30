package kafka

import (
	"context"
	"github.com/oddbit-project/blueprint/crypt/secure"
	"github.com/oddbit-project/blueprint/log"
	tlsProvider "github.com/oddbit-project/blueprint/provider/tls"
	"github.com/segmentio/kafka-go"
	"slices"
)

type AdminConfig struct {
	Brokers  string `json:"brokers"`
	AuthType string `json:"authType"`
	Username string `json:"username"`
	secure.DefaultCredentialConfig
	tlsProvider.ClientConfig
}

type Admin struct {
	broker string
	dialer *kafka.Dialer
	Conn   *kafka.Conn
	Logger *log.Logger
}

func (c AdminConfig) Validate() error {
	if len(c.Brokers) == 0 {
		return ErrMissingAdminBroker
	}

	if !slices.Contains(validAuthTypes, c.AuthType) {
		return ErrInvalidAuthType
	}
	return nil
}
func NewAdmin(cfg *AdminConfig, logger *log.Logger) (*Admin, error) {
	if cfg == nil {
		return nil, ErrNilConfig
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	password, credential, err := setupCredentials(cfg.DefaultCredentialConfig)
	if err != nil {
		return nil, err
	}

	dialer := &kafka.Dialer{
		DualStack: true,
		Timeout:   DefaultTimeout,
	}

	saslMechanism, err := createSASLMechanism(cfg.AuthType, cfg.Username, password)
	if err != nil {
		return nil, err
	}
	if saslMechanism != nil {
		dialer.SASLMechanism = saslMechanism
	}
	if tls, err := cfg.TLSConfig(); err != nil {
		return nil, err
	} else {
		dialer.TLS = tls
	}

	// remove credential from memory
	// it still exists in the dialer configuration
	credential.Clear()

	if logger == nil {
		logger = NewAdminLogger(cfg.Brokers)
	} else {
		// add kafka context
		AdminLogger(logger, cfg.Brokers)
	}

	return &Admin{
		broker: cfg.Brokers,
		dialer: dialer,
		Conn:   nil,
		Logger: logger,
	}, nil
}

func (c *Admin) Connect(ctx context.Context) error {
	var err error
	c.Logger.Info("connecting to kafka...")
	if c.Conn, err = c.dialer.DialContext(ctx, "tcp", c.broker); err != nil {
		c.Conn = nil
		c.Logger.Error(err, "failed to connect to kafka")
		return err
	}
	return nil
}

func (c *Admin) Disconnect() {
	if c.Conn != nil {
		c.Logger.Info("disconnecting from kafka...")
		c.Conn.Close()
		c.Conn = nil
	}
}

func (c *Admin) IsConnected() bool {
	return c.Conn != nil
}

func (c *Admin) GetTopics(ctx context.Context, topics ...string) ([]kafka.Partition, error) {
	if c.Conn == nil {
		return nil, ErrAdminNotConnected
	}
	return c.Conn.ReadPartitions(topics...)
}

// ListTopics list existing kafka topics
func (c *Admin) ListTopics(ctx context.Context) ([]string, error) {
	if c.Conn == nil {
		return nil, ErrAdminNotConnected
	}
	if partitions, err := c.Conn.ReadPartitions(); err != nil {
		c.Logger.Error(err, "failed to read partitions")
		return nil, err
	} else {
		// Use map to deduplicate topics (multiple partitions per topic)
		topicMap := make(map[string]struct{})
		for _, v := range partitions {
			topicMap[v.Topic] = struct{}{}
		}
		topics := make([]string, 0, len(topicMap))
		for topic := range topicMap {
			topics = append(topics, topic)
		}
		return topics, nil
	}
}

// TopicExists returns true if Topic exists
func (c *Admin) TopicExists(ctx context.Context, topic string) (bool, error) {
	if topics, err := c.ListTopics(ctx); err != nil {
		return false, err
	} else {
		for _, t := range topics {
			if t == topic {
				return true, nil
			}
		}
	}
	return false, nil
}

// CreateTopic create a new Topic
func (c *Admin) CreateTopic(ctx context.Context, topic string, numPartitions int, replicationFactor int) error {
	if c.Conn == nil {
		return ErrAdminNotConnected
	}
	c.Logger.
		WithField("topicName", topic).
		WithField("numPartitions", numPartitions).
		WithField("replicationFactor", replicationFactor).
		Info("attempting to create a topic...")
	return c.Conn.CreateTopics(kafka.TopicConfig{
		Topic:             topic,
		NumPartitions:     numPartitions,
		ReplicationFactor: replicationFactor,
	})
}

// DeleteTopic removes a Topic
func (c *Admin) DeleteTopic(ctx context.Context, topic string) error {
	if c.Conn == nil {
		return ErrAdminNotConnected
	}
	c.Logger.
		WithField("topicName", topic).
		Info("attempting to delete a topic...")
	return c.Conn.DeleteTopics(topic)
}
