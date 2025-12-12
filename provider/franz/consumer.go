package franz

import (
	"context"
	"errors"
	"io"
	"net"
	"strings"
	"sync"

	"github.com/oddbit-project/blueprint/log"
	"github.com/twmb/franz-go/pkg/kgo"
)

// RecordHandler processes a single record
type RecordHandler func(ctx context.Context, record ConsumedRecord) error

// BatchHandler processes a batch of records from a single partition
type BatchHandler func(ctx context.Context, batch Batch) error

// FetchHandler processes an entire fetch result
type FetchHandler func(ctx context.Context, result *FetchResult) error

// Consumer is a Kafka consumer with batch support
type Consumer struct {
	client *kgo.Client
	config *ConsumerConfig
	Logger *log.Logger

	mu     sync.RWMutex
	closed bool
}

// NewConsumer creates a new consumer
func NewConsumer(cfg *ConsumerConfig, logger *log.Logger) (*Consumer, error) {
	if cfg == nil {
		return nil, ErrNilConfig
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
		logger = NewConsumerLogger(cfg.Topics, cfg.Group)
	} else {
		logger = ConsumerLogger(logger, cfg.Topics, cfg.Group)
	}

	return &Consumer{
		client: client,
		config: cfg,
		Logger: logger,
	}, nil
}

// Poll fetches records from Kafka
func (c *Consumer) Poll(ctx context.Context) (*FetchResult, error) {
	if ctx == nil {
		return nil, ErrNilContext
	}

	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return nil, ErrClientClosed
	}
	client := c.client
	c.mu.RUnlock()

	fetches := client.PollFetches(ctx)
	return fetchesToResult(fetches), nil
}

// PollRecords fetches up to maxRecords
func (c *Consumer) PollRecords(ctx context.Context, maxRecords int) ([]ConsumedRecord, error) {
	if ctx == nil {
		return nil, ErrNilContext
	}

	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return nil, ErrClientClosed
	}
	client := c.client
	c.mu.RUnlock()

	fetches := client.PollRecords(ctx, maxRecords)
	result := fetchesToResult(fetches)

	if result.HasErrors() {
		return nil, result.FirstError()
	}

	return result.Records(), nil
}

// Consume processes records one at a time (blocking)
func (c *Consumer) Consume(ctx context.Context, handler RecordHandler) error {
	if ctx == nil {
		return ErrNilContext
	}
	if handler == nil {
		return ErrNilHandler
	}

	c.Logger.Info("Starting consumption")

	for {
		result, err := c.Poll(ctx)
		if err != nil {
			if isClosedError(err) {
				c.Logger.Info("Consumer closed, stopping consumption")
				return nil
			}
			return err
		}

		if ctx.Err() != nil {
			c.Logger.Info("Context cancelled, stopping consumption")
			return nil
		}

		if result.HasErrors() {
			for _, fetchErr := range result.Errors {
				if isClosedError(fetchErr.Err) {
					c.Logger.Info("Consumer closed, stopping consumption")
					return nil
				}
				c.Logger.Error(fetchErr.Err, "Fetch error", log.KV{
					"topic":     fetchErr.Topic,
					"partition": fetchErr.Partition,
				})
			}
			return result.FirstError()
		}

		for _, record := range result.Records() {
			if err := handler(ctx, record); err != nil {
				c.Logger.Error(err, "Handler error", log.KV{
					"topic":     record.Topic,
					"partition": record.Partition,
					"offset":    record.Offset,
				})
				return err
			}
		}
	}
}

// ConsumeBatches processes records in batches by partition (blocking)
func (c *Consumer) ConsumeBatches(ctx context.Context, handler BatchHandler) error {
	if ctx == nil {
		return ErrNilContext
	}
	if handler == nil {
		return ErrNilHandler
	}

	c.Logger.Info("Starting batch consumption")

	for {
		result, err := c.Poll(ctx)
		if err != nil {
			if isClosedError(err) {
				c.Logger.Info("Consumer closed, stopping consumption")
				return nil
			}
			return err
		}

		if ctx.Err() != nil {
			c.Logger.Info("Context cancelled, stopping consumption")
			return nil
		}

		if result.HasErrors() {
			for _, fetchErr := range result.Errors {
				if isClosedError(fetchErr.Err) {
					c.Logger.Info("Consumer closed, stopping consumption")
					return nil
				}
			}
			return result.FirstError()
		}

		for _, batch := range result.Batches {
			if err := handler(ctx, batch); err != nil {
				c.Logger.Error(err, "Batch handler error", log.KV{
					"topic":       batch.Topic,
					"partition":   batch.Partition,
					"recordCount": len(batch.Records),
				})
				return err
			}
		}
	}
}

// ConsumeFetches processes entire fetch results (blocking)
// This provides the most control and best performance for high-throughput scenarios
func (c *Consumer) ConsumeFetches(ctx context.Context, handler FetchHandler) error {
	if ctx == nil {
		return ErrNilContext
	}
	if handler == nil {
		return ErrNilHandler
	}

	c.Logger.Info("Starting fetch consumption")

	for {
		result, err := c.Poll(ctx)
		if err != nil {
			if isClosedError(err) {
				c.Logger.Info("Consumer closed, stopping consumption")
				return nil
			}
			return err
		}

		if ctx.Err() != nil {
			c.Logger.Info("Context cancelled, stopping consumption")
			return nil
		}

		if result.HasErrors() {
			for _, fetchErr := range result.Errors {
				if isClosedError(fetchErr.Err) {
					c.Logger.Info("Consumer closed, stopping consumption")
					return nil
				}
			}
			// Pass errors to handler - let it decide how to handle
		}

		if err := handler(ctx, result); err != nil {
			c.Logger.Error(err, "Fetch handler error")
			return err
		}
	}
}

// ConsumeChannel sends records to a channel (blocking)
func (c *Consumer) ConsumeChannel(ctx context.Context, ch chan<- ConsumedRecord) error {
	if ctx == nil {
		return ErrNilContext
	}
	if ch == nil {
		return ErrNilHandler
	}

	c.Logger.Info("Starting channel consumption")

	for {
		result, err := c.Poll(ctx)
		if err != nil {
			if isClosedError(err) {
				c.Logger.Info("Consumer closed, stopping consumption")
				return nil
			}
			return err
		}

		if ctx.Err() != nil {
			c.Logger.Info("Context cancelled, stopping consumption")
			return nil
		}

		if result.HasErrors() {
			for _, fetchErr := range result.Errors {
				if isClosedError(fetchErr.Err) {
					c.Logger.Info("Consumer closed, stopping consumption")
					return nil
				}
			}
			return result.FirstError()
		}

		for _, record := range result.Records() {
			select {
			case ch <- record:
			case <-ctx.Done():
				c.Logger.Info("Context cancelled while sending to channel")
				return nil
			}
		}
	}
}

// CommitOffsets commits the current offsets for all consumed partitions
func (c *Consumer) CommitOffsets(ctx context.Context) error {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return ErrClientClosed
	}
	client := c.client
	c.mu.RUnlock()

	return client.CommitUncommittedOffsets(ctx)
}

// CommitRecord commits the offset for a specific record
func (c *Consumer) CommitRecord(ctx context.Context, record ConsumedRecord) error {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return ErrClientClosed
	}
	client := c.client
	c.mu.RUnlock()

	// Create a kgo.Record to commit
	kgoRecord := &kgo.Record{
		Topic:       record.Topic,
		Partition:   record.Partition,
		Offset:      record.Offset,
		LeaderEpoch: record.LeaderEpoch,
	}

	return client.CommitRecords(ctx, kgoRecord)
}

// CommitBatch commits offsets for all records in a batch
func (c *Consumer) CommitBatch(ctx context.Context, batch Batch) error {
	if len(batch.Records) == 0 {
		return nil
	}

	// Commit the last record's offset + 1
	lastRecord := batch.Records[len(batch.Records)-1]
	return c.CommitRecord(ctx, lastRecord)
}

// Pause pauses consumption of specific topics
func (c *Consumer) Pause(topics ...string) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return
	}

	c.client.PauseFetchTopics(topics...)
	c.Logger.Info("Paused topics", log.KV{"topics": topics})
}

// Resume resumes consumption of specific topics
func (c *Consumer) Resume(topics ...string) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return
	}

	c.client.ResumeFetchTopics(topics...)
	c.Logger.Info("Resumed topics", log.KV{"topics": topics})
}

// PausePartitions pauses specific partitions
func (c *Consumer) PausePartitions(topicPartitions map[string][]int32) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return
	}

	c.client.PauseFetchPartitions(topicPartitionsToKgo(topicPartitions))
}

// ResumePartitions resumes specific partitions
func (c *Consumer) ResumePartitions(topicPartitions map[string][]int32) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return
	}

	c.client.ResumeFetchPartitions(topicPartitionsToKgo(topicPartitions))
}

// Close closes the consumer
func (c *Consumer) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return
	}

	c.closed = true
	c.client.Close()
	c.Logger.Info("Consumer closed")
}

// IsConnected returns true if the consumer is connected
func (c *Consumer) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return !c.closed && c.client != nil
}

// Client returns the underlying kgo.Client for advanced use cases
// Use with caution - prefer the Consumer methods for normal operations
func (c *Consumer) Client() *kgo.Client {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.client
}

// topicPartitionsToKgo converts map[string][]int32 to map[string][]int32
func topicPartitionsToKgo(tp map[string][]int32) map[string][]int32 {
	return tp
}

// isClosedError checks if an error indicates a closed connection
func isClosedError(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrClosedPipe) || errors.Is(err, net.ErrClosed) {
		return true
	}

	if errors.Is(err, context.Canceled) {
		return true
	}

	errStr := err.Error()
	return strings.Contains(errStr, "use of closed network connection") ||
		strings.Contains(errStr, "broken pipe") ||
		strings.Contains(errStr, "connection reset by peer") ||
		strings.Contains(errStr, "client closed")
}
