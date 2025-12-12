package franz

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/oddbit-project/blueprint/log"
	"github.com/twmb/franz-go/pkg/kgo"
)

// ProduceResult represents the result of a produce operation
type ProduceResult struct {
	Record    *Record
	Partition int32
	Offset    int64
	Err       error
}

// ProduceCallback is called when an async produce completes
type ProduceCallback func(result ProduceResult)

// Producer is a Kafka producer with batch and async support
type Producer struct {
	client *kgo.Client
	config *ProducerConfig
	Logger *log.Logger

	mu     sync.RWMutex
	closed bool
}

// NewProducer creates a new producer
func NewProducer(cfg *ProducerConfig, logger *log.Logger) (*Producer, error) {
	if cfg == nil {
		cfg = DefaultProducerConfig()
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	opts, err := cfg.buildOpts()
	if err != nil {
		return nil, err
	}

	client, err := kgo.NewClient(opts...)
	if err != nil {
		return nil, err
	}

	if logger == nil {
		logger = NewProducerLogger(cfg.DefaultTopic)
	} else {
		logger = ProducerLogger(logger, cfg.DefaultTopic)
	}

	return &Producer{
		client: client,
		config: cfg,
		Logger: logger,
	}, nil
}

// Produce sends records synchronously and returns results
func (p *Producer) Produce(ctx context.Context, records ...*Record) ([]ProduceResult, error) {
	if ctx == nil {
		return nil, ErrNilContext
	}

	p.mu.RLock()
	if p.closed {
		p.mu.RUnlock()
		return nil, ErrClientClosed
	}
	client := p.client
	p.mu.RUnlock()

	kgoRecords := make([]*kgo.Record, len(records))
	for i, r := range records {
		kgoRecords[i] = recordToKgo(r, p.config.DefaultTopic)
	}

	results := client.ProduceSync(ctx, kgoRecords...)

	produceResults := make([]ProduceResult, len(results))
	for i, res := range results {
		produceResults[i] = ProduceResult{
			Record:    records[i],
			Partition: res.Record.Partition,
			Offset:    res.Record.Offset,
			Err:       res.Err,
		}
		if res.Err != nil {
			p.Logger.Error(res.Err, "Failed to produce record", log.KV{
				"topic":     res.Record.Topic,
				"partition": res.Record.Partition,
			})
		}
	}

	return produceResults, nil
}

// ProduceAsync sends a record asynchronously with a callback
func (p *Producer) ProduceAsync(ctx context.Context, record *Record, callback ProduceCallback) error {
	if ctx == nil {
		return ErrNilContext
	}

	p.mu.RLock()
	if p.closed {
		p.mu.RUnlock()
		return ErrClientClosed
	}
	client := p.client
	logger := p.Logger
	p.mu.RUnlock()

	kgoRecord := recordToKgo(record, p.config.DefaultTopic)

	client.Produce(ctx, kgoRecord, func(r *kgo.Record, err error) {
		if err != nil {
			logger.Error(err, "Failed to produce record (async)", log.KV{
				"topic":     r.Topic,
				"partition": r.Partition,
			})
		}
		if callback != nil {
			callback(ProduceResult{
				Record:    record,
				Partition: r.Partition,
				Offset:    r.Offset,
				Err:       err,
			})
		}
	})

	return nil
}

// ProduceJSON marshals data to JSON and sends it synchronously
func (p *Producer) ProduceJSON(ctx context.Context, data interface{}, key []byte) (ProduceResult, error) {
	value, err := json.Marshal(data)
	if err != nil {
		p.Logger.Error(err, "Failed to marshal JSON")
		return ProduceResult{Err: err}, err
	}

	record := NewRecord(value)
	if key != nil {
		record.WithKey(key)
	}

	results, err := p.Produce(ctx, record)
	if err != nil {
		return ProduceResult{Err: err}, err
	}

	return results[0], results[0].Err
}

// ProduceJSONAsync marshals data to JSON and sends it asynchronously
func (p *Producer) ProduceJSONAsync(ctx context.Context, data interface{}, key []byte, callback ProduceCallback) error {
	value, err := json.Marshal(data)
	if err != nil {
		p.Logger.Error(err, "Failed to marshal JSON")
		return err
	}

	record := NewRecord(value)
	if key != nil {
		record.WithKey(key)
	}

	return p.ProduceAsync(ctx, record, callback)
}

// Flush waits for all buffered records to be sent
func (p *Producer) Flush(ctx context.Context) error {
	p.mu.RLock()
	if p.closed {
		p.mu.RUnlock()
		return ErrClientClosed
	}
	client := p.client
	p.mu.RUnlock()

	return client.Flush(ctx)
}

// Close closes the producer, flushing any buffered records first
func (p *Producer) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return
	}

	p.closed = true
	p.client.Close()
	p.Logger.Info("Producer closed")
}

// IsConnected returns true if the producer is connected
func (p *Producer) IsConnected() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return !p.closed && p.client != nil
}

// Client returns the underlying kgo.Client for advanced use cases
// Use with caution - prefer the Producer methods for normal operations
func (p *Producer) Client() *kgo.Client {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.client
}
