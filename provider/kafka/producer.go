package kafka

import (
	"context"
	"encoding/json"
	tlsProvider "github.com/oddbit-project/blueprint/provider/tls"
	"github.com/oddbit-project/blueprint/utils/str"
	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl/plain"
	"github.com/segmentio/kafka-go/sasl/scram"
	"strings"
)

type ProducerConfig struct {
	Brokers  string `json:"brokers"`
	Topic    string `json:"topic"`
	AuthType string `json:"authType"`
	Username string `json:"username"`
	Password string `json:"password"`
	tlsProvider.ClientConfig
}

type KafkaProducer struct {
	ctx     context.Context
	Brokers string
	Topic   string
	Writer  *kafka.Writer
}

func (c ProducerConfig) Validate() error {
	if len(c.Brokers) == 0 {
		return ErrMissingProducerBroker
	}
	if len(c.Topic) == 0 {
		return ErrMissingProducerTopic
	}
	if str.Contains(c.AuthType, validAuthTypes) == -1 {
		return ErrInvalidAuthType
	}
	return nil
}

func NewProducer(ctx context.Context, cfg *ProducerConfig) (*KafkaProducer, error) {
	if cfg == nil {
		return nil, ErrNilConfig
	}
	// check if config has errors
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	transport := &kafka.Transport{
		DialTimeout: DefaultTimeout,
	}

	switch cfg.AuthType {
	case AuthTypePlain:
		transport.SASL = plain.Mechanism{
			Username: cfg.Username,
			Password: cfg.Password,
		}
	case AuthTypeScram256:
		if sasl, err := scram.Mechanism(scram.SHA256, cfg.Username, cfg.Password); err != nil {
			return nil, err
		} else {
			transport.SASL = sasl
		}
	case AuthTypeScram512:
		if sasl, err := scram.Mechanism(scram.SHA512, cfg.Username, cfg.Password); err != nil {
			return nil, err
		} else {
			transport.SASL = sasl
		}
	}

	if tls, err := cfg.TLSConfig(); err != nil {
		return nil, err
	} else {
		transport.TLS = tls
	}

	producer := &kafka.Writer{
		Addr:                   kafka.TCP(strings.Split(cfg.Brokers, ",")...),
		Topic:                  cfg.Topic,
		AllowAutoTopicCreation: true,
		Transport:              transport,
	}

	return &KafkaProducer{
		ctx:     ctx,
		Brokers: cfg.Brokers,
		Topic:   cfg.Topic,
		Writer:  producer,
	}, nil
}

// Disconnect disconnects from the Writer
func (p *KafkaProducer) Disconnect() {
	if p.Writer != nil {
		p.Writer.Close()
		p.Writer = nil
	}
}

// IsConnected returns ture if Writer is connected
func (p *KafkaProducer) IsConnected() bool {
	return p.Writer != nil
}

// Write writes a single message to topic
func (p *KafkaProducer) Write(value []byte, key ...[]byte) error {
	if p.Writer == nil {
		return ErrProducerClosed
	}
	var k []byte = nil
	if len(key) > 0 {
		k = key[0]
	}
	return p.Writer.WriteMessages(p.ctx, kafka.Message{
		Key:   k,
		Value: value,
	})
}

// WriteMulti Write multiple messages to Topic
func (p *KafkaProducer) WriteMulti(values ...[]byte) error {
	if p.Writer == nil {
		return ErrProducerClosed
	}
	ml := make([]kafka.Message, len(values))
	for idx, value := range values {
		ml[idx].Key = nil
		ml[idx].Value = value
	}
	return p.Writer.WriteMessages(p.ctx, ml...)
}

// WriteJson Write a struct to a Topic as a json message
func (p *KafkaProducer) WriteJson(data interface{}, key ...[]byte) error {
	var k []byte = nil
	if len(key) > 0 {
		k = key[0]
	}
	value, err := json.Marshal(data)
	if err != nil {
		return err
	}

	return p.Writer.WriteMessages(p.ctx, kafka.Message{
		Key:   k,
		Value: value,
	})
}

// WriteMultiJson Write a slice of structs to a Topic as a json message
func (p *KafkaProducer) WriteMultiJson(values ...interface{}) error {
	ml := make([]kafka.Message, len(values))
	for i, v := range values {
		value, err := json.Marshal(v)
		if err != nil {
			return err
		}
		ml[i] = kafka.Message{
			Value: value,
		}
	}
	return p.Writer.WriteMessages(p.ctx, ml...)
}
