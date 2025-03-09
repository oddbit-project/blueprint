package kafka

import (
	"context"
	"encoding/json"
	"github.com/oddbit-project/blueprint/log"
	tlsProvider "github.com/oddbit-project/blueprint/provider/tls"
	"github.com/oddbit-project/blueprint/utils/str"
	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl/plain"
	"github.com/segmentio/kafka-go/sasl/scram"
	"strings"
	"time"
)

// ProducerOptions additional producer options
type ProducerOptions struct {
	MaxAttempts     uint   `json:"maxAttempts"`
	WriteBackoffMin uint   `json:"writeBackoffMin"` // WriteBackoffMin value in milliseconds, defaults to 100
	WriteBackoffMax uint   `json:"writeBackoffMax"` // WriteBackoffMax value in milliseconds, defaults to 1000
	BatchSize       uint   `json:"batchSize"`       // BatchSize, defaults to 100
	BatchBytes      uint64 `json:"batchBytes"`      // BatchBytes, defaults to 1048576
	BatchTimeout    uint   `json:"batchTimeout"`    // BatchTimeout value in milliseconds, defaults to 1000
	ReadTimeout     uint   `json:"readTimeout"`     // ReadTimeout value in milliseconds, defaults to 10.000
	WriteTimeout    uint   `json:"writeTimeout"`    // WriteTimeout value in milliseconds, defaults to 10.000
	RequiredAcks    string `json:"requiredAcks"`    // RequiredAcks one of "none", "one", "all", default "none"
	Async           bool   `json:"async"`           // Async, default false
}

type ProducerConfig struct {
	Brokers  string `json:"brokers"`
	Topic    string `json:"topic"`
	AuthType string `json:"authType"`
	Username string `json:"username"`
	Password string `json:"password"`
	tlsProvider.ClientConfig
	ProducerOptions
}

type Producer struct {
	ctx     context.Context
	Brokers string
	Topic   string
	Writer  *kafka.Writer
}

// ApplyOptions sets additional Writer parameters
func (p ProducerOptions) ApplyOptions(w *kafka.Writer) {
	if p.MaxAttempts > 0 {
		w.MaxAttempts = int(p.MaxAttempts)
	}
	if p.WriteBackoffMin > 0 {
		w.WriteBackoffMin = time.Duration(p.WriteBackoffMin) * time.Millisecond
	}
	if p.WriteBackoffMax > 0 {
		w.WriteBackoffMax = time.Duration(p.WriteBackoffMax) * time.Millisecond
	}
	if p.BatchSize > 0 {
		w.BatchSize = int(p.BatchSize)
	}
	if p.BatchBytes > 0 {
		w.BatchBytes = int64(p.BatchBytes)
	}
	if p.BatchTimeout > 0 {
		w.BatchTimeout = time.Duration(p.BatchTimeout) * time.Millisecond
	}
	if p.ReadTimeout > 0 {
		w.ReadTimeout = time.Duration(p.ReadTimeout) * time.Millisecond
	}
	if p.WriteTimeout > 0 {
		w.WriteTimeout = time.Duration(p.WriteTimeout) * time.Millisecond
	}

	switch p.RequiredAcks {
	case "all":
		w.RequiredAcks = kafka.RequireAll
	case "one":
		w.RequiredAcks = kafka.RequireOne
	default:
		w.RequiredAcks = kafka.RequireNone
	}

	w.Async = p.Async
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

func NewProducer(ctx context.Context, cfg *ProducerConfig) (*Producer, error) {
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

	// apply writer options, if defined
	cfg.ApplyOptions(producer)

	return &Producer{
		ctx:     ctx,
		Brokers: cfg.Brokers,
		Topic:   cfg.Topic,
		Writer:  producer,
	}, nil
}

// Disconnect disconnects from the Writer
func (p *Producer) Disconnect() {
	if p.Writer != nil {
		p.Writer.Close()
		p.Writer = nil
	}
}

// IsConnected returns ture if Writer is connected
func (p *Producer) IsConnected() bool {
	return p.Writer != nil
}

// Write writes a single message to topic
func (p *Producer) Write(value []byte, key ...[]byte) error {
	logger := log.NewKafkaProducerLogger(p.ctx, p.Topic)
	
	if p.Writer == nil {
		logger.Error(ErrProducerClosed, "Failed to write message - producer closed", nil)
		return ErrProducerClosed
	}
	
	var k []byte = nil
	if len(key) > 0 {
		k = key[0]
	}
	
	msg := kafka.Message{
		Topic: p.Topic,
		Key:   k,
		Value: value,
		// Add trace information as headers
		Headers: log.AddKafkaHeadersFromContext(p.ctx, nil),
	}
	
	// Log at debug level before sending
	logger.Debug("Sending message to Kafka", map[string]interface{}{
		"message_size": len(value),
		"has_key":      k != nil,
	})
	
	err := p.Writer.WriteMessages(p.ctx, msg)
	if err != nil {
		logger.Error(err, "Failed to write message to Kafka", map[string]interface{}{
			"message_size": len(value),
		})
		return err
	}
	
	// Log message sent at debug level
	log.LogKafkaMessageSent(p.ctx, msg)
	
	return nil
}

// WriteMulti Write multiple messages to Topic
func (p *Producer) WriteMulti(values ...[]byte) error {
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
func (p *Producer) WriteJson(data interface{}, key ...[]byte) error {
	logger := log.NewKafkaProducerLogger(p.ctx, p.Topic)
	
	if p.Writer == nil {
		logger.Error(ErrProducerClosed, "Failed to write JSON message - producer closed", nil)
		return ErrProducerClosed
	}
	
	var k []byte = nil
	if len(key) > 0 {
		k = key[0]
	}
	
	// Log serialization attempt
	logger.Debug("Serializing object to JSON for Kafka", nil)
	
	value, err := json.Marshal(data)
	if err != nil {
		logger.Error(err, "Failed to serialize object to JSON", nil)
		return err
	}
	
	msg := kafka.Message{
		Topic: p.Topic,
		Key:   k,
		Value: value,
		// Add trace information as headers
		Headers: log.AddKafkaHeadersFromContext(p.ctx, nil),
	}
	
	// Log at debug level before sending
	logger.Debug("Sending JSON message to Kafka", map[string]interface{}{
		"message_size": len(value),
		"has_key":      k != nil,
	})
	
	err = p.Writer.WriteMessages(p.ctx, msg)
	if err != nil {
		logger.Error(err, "Failed to write JSON message to Kafka", map[string]interface{}{
			"message_size": len(value),
		})
		return err
	}
	
	// Log message sent
	log.LogKafkaMessageSent(p.ctx, msg)
	
	return nil
}

// WriteMultiJson Write a slice of structs to a Topic as a json message
func (p *Producer) WriteMultiJson(values ...interface{}) error {
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
