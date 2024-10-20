package kafka

import (
	"context"
	tlsProvider "github.com/oddbit-project/blueprint/provider/tls"
	"github.com/oddbit-project/blueprint/utils/str"
	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl/plain"
	"github.com/segmentio/kafka-go/sasl/scram"
)

type AdminConfig struct {
	Brokers  string `json:"brokers"`
	AuthType string `json:"authType"`
	Username string `json:"username"`
	Password string `json:"password"`
	tlsProvider.ClientConfig
}

type KafkaAdmin struct {
	broker string
	ctx    context.Context
	dialer *kafka.Dialer
	Conn   *kafka.Conn
}

func (c AdminConfig) Validate() error {
	if len(c.Brokers) == 0 {
		return ErrMissingAdminBroker
	}

	if str.Contains(c.AuthType, validAuthTypes) == -1 {
		return ErrInvalidAuthType
	}
	return nil
}
func NewAdmin(ctx context.Context, cfg *AdminConfig) (*KafkaAdmin, error) {
	if cfg == nil {
		return nil, ErrNilConfig
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	dialer := &kafka.Dialer{
		Timeout:   DefaultTimeout,
		DualStack: true,
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

	return &KafkaAdmin{
		broker: cfg.Brokers,
		ctx:    ctx,
		dialer: dialer,
		Conn:   nil,
	}, nil
}

func (c *KafkaAdmin) Connect() error {
	var err error
	if c.Conn, err = c.dialer.DialContext(c.ctx, "tcp", c.broker); err != nil {
		c.Conn = nil
		return err
	}
	return nil
}

func (c *KafkaAdmin) Disconnect() {
	if c.Conn != nil {
		c.Conn.Close()
		c.Conn = nil
	}
}

func (c *KafkaAdmin) IsConnected() bool {
	return c.Conn != nil
}

func (c *KafkaAdmin) GetTopics(topics ...string) ([]kafka.Partition, error) {
	if c.Conn == nil {
		if err := c.Connect(); err != nil {
			return nil, err
		}
		defer c.Disconnect()
	}
	return c.Conn.ReadPartitions(topics...)
}

// ListTopics list existing kafka topics
func (c *KafkaAdmin) ListTopics() ([]string, error) {
	if c.Conn == nil {
		if err := c.Connect(); err != nil {
			return nil, err
		}
		defer c.Disconnect()
	}
	if partitions, err := c.Conn.ReadPartitions(); err != nil {
		return nil, err
	} else {
		topics := make([]string, len(partitions))
		for i, v := range partitions {
			topics[i] = v.Topic
		}
		return topics, nil
	}
}

// TopicExists returns true if Topic exists
func (c *KafkaAdmin) TopicExists(topic string) (bool, error) {
	if topics, err := c.ListTopics(); err != nil {
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
func (c *KafkaAdmin) CreateTopic(topic string, numPartitions int, replicationFactor int) error {
	if c.Conn == nil {
		if err := c.Connect(); err != nil {
			return err
		}
		defer c.Disconnect()
	}
	return c.Conn.CreateTopics(kafka.TopicConfig{
		Topic:             topic,
		NumPartitions:     numPartitions,
		ReplicationFactor: replicationFactor,
	})
}

// DeleteTopic removes a Topic
func (c *KafkaAdmin) DeleteTopic(topic string) error {
	if c.Conn == nil {
		if err := c.Connect(); err != nil {
			return err
		}
		defer c.Disconnect()
	}
	return c.Conn.DeleteTopics(topic)
}
