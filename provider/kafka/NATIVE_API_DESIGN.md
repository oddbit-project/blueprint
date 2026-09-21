# Native Franz-go Kafka Provider Design

This document proposes a redesigned Kafka provider that leverages franz-go's native patterns while following blueprint library conventions.

## Design Goals

1. **Expose franz-go's strengths** - Batch processing, unified client, transactions
2. **Follow blueprint patterns** - Config structs, embedded credentials/TLS, validation, logging
3. **Simpler mental model** - Single client for produce/consume when appropriate
4. **Better performance** - Native batch operations, proper async handling
5. **Backward compatibility option** - Can coexist with current API during transition

## Package Structure

```
provider/kafka2/
├── client.go           # Unified Client for simple use cases
├── consumer.go         # Dedicated Consumer with batch support
├── producer.go         # Dedicated Producer with callbacks
├── admin.go            # Admin operations
├── config.go           # All configuration types
├── options.go          # Functional options
├── message.go          # Message and batch types
├── errors.go           # Error constants
├── logger.go           # Logging utilities
├── transaction.go      # Transaction support
├── go.mod
└── *_test.go
```

## Core Types

### errors.go

```go
package kafka2

import "github.com/oddbit-project/blueprint/utils"

const (
    ErrNilConfig          = utils.Error("config is nil")
    ErrMissingBrokers     = utils.Error("brokers address is required")
    ErrMissingTopic       = utils.Error("topic is required")
    ErrMissingGroup       = utils.Error("consumer group is required for group consumption")
    ErrClientClosed       = utils.Error("client is closed")
    ErrInvalidAuthType    = utils.Error("invalid authentication type")
    ErrTransactionAborted = utils.Error("transaction was aborted")
    ErrNilHandler         = utils.Error("handler function is nil")
    ErrNilContext         = utils.Error("context is nil")
)
```

### message.go

```go
package kafka2

import "time"

// Header represents a Kafka record header
type Header struct {
    Key   string
    Value []byte
}

// Record represents a Kafka record for producing
type Record struct {
    Topic     string    // Target topic (optional if default set)
    Key       []byte
    Value     []byte
    Headers   []Header
    Partition int32     // -1 for auto-assignment
    Timestamp time.Time // Zero for broker timestamp
}

// NewRecord creates a record with value only
func NewRecord(value []byte) *Record {
    return &Record{Value: value, Partition: -1}
}

// WithKey adds a key to the record
func (r *Record) WithKey(key []byte) *Record {
    r.Key = key
    return r
}

// WithTopic sets the target topic
func (r *Record) WithTopic(topic string) *Record {
    r.Topic = topic
    return r
}

// WithHeaders adds headers to the record
func (r *Record) WithHeaders(headers ...Header) *Record {
    r.Headers = append(r.Headers, headers...)
    return r
}

// ConsumedRecord represents a received Kafka record with metadata
type ConsumedRecord struct {
    Topic         string
    Partition     int32
    Offset        int64
    Key           []byte
    Value         []byte
    Headers       []Header
    Timestamp     time.Time
    LeaderEpoch   int32
}

// Batch represents a batch of consumed records
type Batch struct {
    Records []ConsumedRecord
    Topic   string
    Partition int32
}

// FetchResult represents the result of a poll operation
type FetchResult struct {
    Batches []Batch
    Errors  []FetchError
}

// FetchError represents an error for a specific topic/partition
type FetchError struct {
    Topic     string
    Partition int32
    Err       error
}

// IsEmpty returns true if no records were fetched
func (f *FetchResult) IsEmpty() bool {
    return len(f.Batches) == 0
}

// Records returns all records flattened
func (f *FetchResult) Records() []ConsumedRecord {
    var records []ConsumedRecord
    for _, b := range f.Batches {
        records = append(records, b.Records...)
    }
    return records
}

// HasErrors returns true if any fetch errors occurred
func (f *FetchResult) HasErrors() bool {
    return len(f.Errors) > 0
}
```

### config.go

```go
package kafka2

import (
    "time"

    "github.com/oddbit-project/blueprint/crypt/secure"
    tlsProvider "github.com/oddbit-project/blueprint/provider/tls"
)

const (
    AuthTypeNone     = "none"
    AuthTypePlain    = "plain"
    AuthTypeScram256 = "scram256"
    AuthTypeScram512 = "scram512"

    AcksNone   = "none"
    AcksLeader = "leader"
    AcksAll    = "all"

    OffsetStart = "start"
    OffsetEnd   = "end"

    IsolationReadUncommitted = "uncommitted"
    IsolationReadCommitted   = "committed"
)

// BaseConfig contains common configuration for all client types
type BaseConfig struct {
    Brokers  string `json:"brokers"`  // Comma-separated broker addresses
    AuthType string `json:"authType"` // none, plain, scram256, scram512
    Username string `json:"username"`
    secure.DefaultCredentialConfig
    tlsProvider.ClientConfig

    // Connection settings
    DialTimeout    time.Duration `json:"dialTimeout"`    // Default: 30s
    RequestTimeout time.Duration `json:"requestTimeout"` // Default: 30s
    RetryBackoff   time.Duration `json:"retryBackoff"`   // Default: 100ms
    MaxRetries     int           `json:"maxRetries"`     // Default: 3
}

// Validate validates base configuration
func (c *BaseConfig) Validate() error {
    if len(c.Brokers) == 0 {
        return ErrMissingBrokers
    }
    if c.AuthType != "" && !isValidAuthType(c.AuthType) {
        return ErrInvalidAuthType
    }
    return nil
}

// DefaultBaseConfig returns base config with sensible defaults
func DefaultBaseConfig() BaseConfig {
    return BaseConfig{
        AuthType:       AuthTypeNone,
        DialTimeout:    30 * time.Second,
        RequestTimeout: 30 * time.Second,
        RetryBackoff:   100 * time.Millisecond,
        MaxRetries:     3,
    }
}

// ProducerConfig configures a producer
type ProducerConfig struct {
    BaseConfig

    DefaultTopic string `json:"defaultTopic"` // Default topic for records without explicit topic

    // Batching
    BatchMaxRecords int           `json:"batchMaxRecords"` // Max records per batch (default: 10000)
    BatchMaxBytes   int           `json:"batchMaxBytes"`   // Max bytes per batch (default: 1MB)
    Linger          time.Duration `json:"linger"`          // Time to wait for batch fill (default: 0)

    // Reliability
    Acks              string `json:"acks"`              // none, leader, all (default: leader)
    Idempotent        bool   `json:"idempotent"`        // Enable idempotent producer
    TransactionalID   string `json:"transactionalId"`   // For transactional producer

    // Compression
    Compression string `json:"compression"` // none, gzip, snappy, lz4, zstd
}

// Validate validates producer configuration
func (c *ProducerConfig) Validate() error {
    return c.BaseConfig.Validate()
}

// DefaultProducerConfig returns producer config with sensible defaults
func DefaultProducerConfig() *ProducerConfig {
    return &ProducerConfig{
        BaseConfig:      DefaultBaseConfig(),
        BatchMaxRecords: 10000,
        BatchMaxBytes:   1048576,
        Linger:          0,
        Acks:            AcksLeader,
    }
}

// ConsumerConfig configures a consumer
type ConsumerConfig struct {
    BaseConfig

    // Topics can be set here or via options
    Topics []string `json:"topics"`
    Group  string   `json:"group"` // Consumer group (required for group consumption)

    // Consumer behavior
    StartOffset    string        `json:"startOffset"`    // start, end (default: end)
    IsolationLevel string        `json:"isolationLevel"` // uncommitted, committed (default: committed)

    // Group settings
    SessionTimeout   time.Duration `json:"sessionTimeout"`   // Default: 45s
    RebalanceTimeout time.Duration `json:"rebalanceTimeout"` // Default: 60s
    HeartbeatInterval time.Duration `json:"heartbeatInterval"` // Default: 3s

    // Fetch settings
    FetchMinBytes int           `json:"fetchMinBytes"` // Default: 1
    FetchMaxBytes int           `json:"fetchMaxBytes"` // Default: 50MB
    FetchMaxWait  time.Duration `json:"fetchMaxWait"`  // Default: 5s

    // Offset management
    AutoCommit         bool          `json:"autoCommit"`         // Default: true
    AutoCommitInterval time.Duration `json:"autoCommitInterval"` // Default: 5s
}

// Validate validates consumer configuration
func (c *ConsumerConfig) Validate() error {
    if err := c.BaseConfig.Validate(); err != nil {
        return err
    }
    if len(c.Topics) == 0 {
        return ErrMissingTopic
    }
    return nil
}

// DefaultConsumerConfig returns consumer config with sensible defaults
func DefaultConsumerConfig() *ConsumerConfig {
    return &ConsumerConfig{
        BaseConfig:         DefaultBaseConfig(),
        StartOffset:        OffsetEnd,
        IsolationLevel:     IsolationReadCommitted,
        SessionTimeout:     45 * time.Second,
        RebalanceTimeout:   60 * time.Second,
        HeartbeatInterval:  3 * time.Second,
        FetchMinBytes:      1,
        FetchMaxBytes:      52428800,
        FetchMaxWait:       5 * time.Second,
        AutoCommit:         true,
        AutoCommitInterval: 5 * time.Second,
    }
}

// AdminConfig configures an admin client
type AdminConfig struct {
    BaseConfig
}

// Validate validates admin configuration
func (c *AdminConfig) Validate() error {
    return c.BaseConfig.Validate()
}

// DefaultAdminConfig returns admin config with sensible defaults
func DefaultAdminConfig() *AdminConfig {
    return &AdminConfig{
        BaseConfig: DefaultBaseConfig(),
    }
}
```

### producer.go

```go
package kafka2

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

    opts, err := buildProducerOpts(cfg)
    if err != nil {
        return nil, err
    }

    client, err := kgo.NewClient(opts...)
    if err != nil {
        return nil, err
    }

    if logger == nil {
        logger = NewProducerLogger(cfg.DefaultTopic)
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
    }

    return produceResults, nil
}

// ProduceAsync sends records asynchronously with per-record callbacks
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
    p.mu.RUnlock()

    kgoRecord := recordToKgo(record, p.config.DefaultTopic)

    client.Produce(ctx, kgoRecord, func(r *kgo.Record, err error) {
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

// ProduceJSON marshals data to JSON and sends it
func (p *Producer) ProduceJSON(ctx context.Context, data interface{}, key []byte) (ProduceResult, error) {
    value, err := json.Marshal(data)
    if err != nil {
        return ProduceResult{Err: err}, err
    }

    record := NewRecord(value).WithKey(key)
    results, err := p.Produce(ctx, record)
    if err != nil {
        return ProduceResult{Err: err}, err
    }

    return results[0], results[0].Err
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

// Close closes the producer
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
```

### consumer.go

```go
package kafka2

import (
    "context"
    "sync"

    "github.com/oddbit-project/blueprint/log"
    "github.com/twmb/franz-go/pkg/kgo"
)

// RecordHandler processes a single record
type RecordHandler func(ctx context.Context, record ConsumedRecord) error

// BatchHandler processes a batch of records
type BatchHandler func(ctx context.Context, batch Batch) error

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

    opts, err := buildConsumerOpts(cfg)
    if err != nil {
        return nil, err
    }

    client, err := kgo.NewClient(opts...)
    if err != nil {
        return nil, err
    }

    if logger == nil {
        logger = NewConsumerLogger(cfg.Topics, cfg.Group)
    }

    return &Consumer{
        client: client,
        config: cfg,
        Logger: logger,
    }, nil
}

// Poll fetches records from Kafka (non-blocking if no records available)
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

// PollRecords fetches up to maxRecords (convenience method)
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
        return nil, result.Errors[0].Err
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
            return err
        }

        if ctx.Err() != nil {
            c.Logger.Info("Context cancelled, stopping consumption")
            return nil
        }

        if result.HasErrors() {
            for _, fetchErr := range result.Errors {
                c.Logger.Error(fetchErr.Err, "Fetch error", log.KV{
                    "topic":     fetchErr.Topic,
                    "partition": fetchErr.Partition,
                })
            }
            return result.Errors[0].Err
        }

        for _, record := range result.Records() {
            if err := handler(ctx, record); err != nil {
                return err
            }
        }
    }
}

// ConsumeBatches processes records in batches (blocking)
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
            return err
        }

        if ctx.Err() != nil {
            c.Logger.Info("Context cancelled, stopping consumption")
            return nil
        }

        if result.HasErrors() {
            return result.Errors[0].Err
        }

        for _, batch := range result.Batches {
            if err := handler(ctx, batch); err != nil {
                return err
            }
        }
    }
}

// CommitOffsets commits the current offsets
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

    offsets := make(map[string]map[int32]kgo.EpochOffset)
    offsets[record.Topic] = map[int32]kgo.EpochOffset{
        record.Partition: {Epoch: record.LeaderEpoch, Offset: record.Offset + 1},
    }

    return client.CommitOffsets(ctx, offsets, nil)
}

// Pause pauses consumption of specific topics/partitions
func (c *Consumer) Pause(topicPartitions map[string][]int32) {
    c.mu.RLock()
    defer c.mu.RUnlock()

    if c.closed {
        return
    }

    c.client.PauseFetchTopics(topicsFromMap(topicPartitions)...)
}

// Resume resumes consumption of specific topics/partitions
func (c *Consumer) Resume(topicPartitions map[string][]int32) {
    c.mu.RLock()
    defer c.mu.RUnlock()

    if c.closed {
        return
    }

    c.client.ResumeFetchTopics(topicsFromMap(topicPartitions)...)
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
```

### transaction.go

```go
package kafka2

import (
    "context"

    "github.com/twmb/franz-go/pkg/kgo"
)

// Transaction represents a Kafka transaction
type Transaction struct {
    producer *Producer
    client   *kgo.Client
    ctx      context.Context
    records  []*kgo.Record
    aborted  bool
}

// TransactionFunc is executed within a transaction context
type TransactionFunc func(tx *Transaction) error

// BeginTransaction starts a new transaction
func (p *Producer) BeginTransaction(ctx context.Context) (*Transaction, error) {
    if p.config.TransactionalID == "" {
        return nil, utils.Error("transactional ID required for transactions")
    }

    p.mu.RLock()
    if p.closed {
        p.mu.RUnlock()
        return nil, ErrClientClosed
    }
    client := p.client
    p.mu.RUnlock()

    if err := client.BeginTransaction(); err != nil {
        return nil, err
    }

    return &Transaction{
        producer: p,
        client:   client,
        ctx:      ctx,
    }, nil
}

// Produce adds a record to the transaction
func (tx *Transaction) Produce(record *Record) {
    if tx.aborted {
        return
    }
    tx.records = append(tx.records, recordToKgo(record, tx.producer.config.DefaultTopic))
}

// Commit commits the transaction
func (tx *Transaction) Commit() error {
    if tx.aborted {
        return ErrTransactionAborted
    }

    // Produce all records
    tx.client.ProduceSync(tx.ctx, tx.records...)

    // End transaction
    return tx.client.EndTransaction(tx.ctx, kgo.TryCommit)
}

// Abort aborts the transaction
func (tx *Transaction) Abort() error {
    tx.aborted = true
    return tx.client.EndTransaction(tx.ctx, kgo.TryAbort)
}

// Transact executes a function within a transaction
// Commits on success, aborts on error or panic
func (p *Producer) Transact(ctx context.Context, fn TransactionFunc) error {
    tx, err := p.BeginTransaction(ctx)
    if err != nil {
        return err
    }

    defer func() {
        if r := recover(); r != nil {
            tx.Abort()
            panic(r)
        }
    }()

    if err := fn(tx); err != nil {
        tx.Abort()
        return err
    }

    return tx.Commit()
}
```

### admin.go

```go
package kafka2

import (
    "context"

    "github.com/oddbit-project/blueprint/log"
    "github.com/twmb/franz-go/pkg/kadm"
    "github.com/twmb/franz-go/pkg/kgo"
)

// TopicConfig represents topic configuration for creation
type TopicConfig struct {
    Name              string
    Partitions        int32
    ReplicationFactor int16
    Configs           map[string]*string // Topic-level configs
}

// TopicInfo represents information about a topic
type TopicInfo struct {
    Name       string
    Partitions []PartitionInfo
    Internal   bool
}

// PartitionInfo represents information about a partition
type PartitionInfo struct {
    ID       int32
    Leader   int32
    Replicas []int32
    ISR      []int32
}

// Admin is a Kafka admin client
type Admin struct {
    client      *kgo.Client
    adminClient *kadm.Client
    config      *AdminConfig
    Logger      *log.Logger
}

// NewAdmin creates a new admin client
func NewAdmin(cfg *AdminConfig, logger *log.Logger) (*Admin, error) {
    if cfg == nil {
        cfg = DefaultAdminConfig()
    }
    if err := cfg.Validate(); err != nil {
        return nil, err
    }

    opts, err := buildAdminOpts(cfg)
    if err != nil {
        return nil, err
    }

    client, err := kgo.NewClient(opts...)
    if err != nil {
        return nil, err
    }

    if logger == nil {
        logger = NewAdminLogger(cfg.Brokers)
    }

    return &Admin{
        client:      client,
        adminClient: kadm.NewClient(client),
        config:      cfg,
        Logger:      logger,
    }, nil
}

// CreateTopics creates one or more topics
func (a *Admin) CreateTopics(ctx context.Context, topics ...TopicConfig) error {
    for _, topic := range topics {
        resp, err := a.adminClient.CreateTopics(ctx, topic.Partitions, topic.ReplicationFactor, topic.Configs, topic.Name)
        if err != nil {
            return err
        }
        for _, t := range resp {
            if t.Err != nil {
                return t.Err
            }
        }
    }
    return nil
}

// DeleteTopics deletes one or more topics
func (a *Admin) DeleteTopics(ctx context.Context, topics ...string) error {
    resp, err := a.adminClient.DeleteTopics(ctx, topics...)
    if err != nil {
        return err
    }
    for _, t := range resp {
        if t.Err != nil {
            return t.Err
        }
    }
    return nil
}

// ListTopics lists all topics
func (a *Admin) ListTopics(ctx context.Context) ([]TopicInfo, error) {
    topics, err := a.adminClient.ListTopics(ctx)
    if err != nil {
        return nil, err
    }

    result := make([]TopicInfo, 0, len(topics))
    for _, t := range topics {
        info := TopicInfo{
            Name:     t.Topic,
            Internal: t.IsInternal,
        }
        for _, p := range t.Partitions {
            info.Partitions = append(info.Partitions, PartitionInfo{
                ID:       p.Partition,
                Leader:   p.Leader,
                Replicas: p.Replicas,
                ISR:      p.ISR,
            })
        }
        result = append(result, info)
    }
    return result, nil
}

// TopicExists checks if a topic exists
func (a *Admin) TopicExists(ctx context.Context, topic string) (bool, error) {
    topics, err := a.ListTopics(ctx)
    if err != nil {
        return false, err
    }
    for _, t := range topics {
        if t.Name == topic {
            return true, nil
        }
    }
    return false, nil
}

// Close closes the admin client
func (a *Admin) Close() {
    a.adminClient = nil
    a.client.Close()
    a.Logger.Info("Admin client closed")
}
```

## Usage Examples

### Basic Producer

```go
cfg := kafka2.DefaultProducerConfig()
cfg.Brokers = "localhost:9092"
cfg.DefaultTopic = "my-topic"
cfg.Acks = kafka2.AcksAll

producer, err := kafka2.NewProducer(cfg, nil)
if err != nil {
    log.Fatal(err)
}
defer producer.Close()

// Sync produce
results, err := producer.Produce(ctx,
    kafka2.NewRecord([]byte("message 1")),
    kafka2.NewRecord([]byte("message 2")).WithKey([]byte("key")),
)

// Async produce with callback
producer.ProduceAsync(ctx, kafka2.NewRecord([]byte("async")), func(r kafka2.ProduceResult) {
    if r.Err != nil {
        log.Printf("Failed: %v", r.Err)
    } else {
        log.Printf("Produced to partition %d offset %d", r.Partition, r.Offset)
    }
})
```

### Batch Consumer

```go
cfg := kafka2.DefaultConsumerConfig()
cfg.Brokers = "localhost:9092"
cfg.Topics = []string{"my-topic"}
cfg.Group = "my-group"
cfg.AutoCommit = false

consumer, err := kafka2.NewConsumer(cfg, nil)
if err != nil {
    log.Fatal(err)
}
defer consumer.Close()

// Process in batches for high throughput
err = consumer.ConsumeBatches(ctx, func(ctx context.Context, batch kafka2.Batch) error {
    // Process entire batch
    for _, record := range batch.Records {
        processRecord(record)
    }

    // Commit after batch
    return consumer.CommitOffsets(ctx)
})
```

### Transactions

```go
cfg := kafka2.DefaultProducerConfig()
cfg.Brokers = "localhost:9092"
cfg.TransactionalID = "my-transactional-producer"
cfg.Idempotent = true

producer, err := kafka2.NewProducer(cfg, nil)
if err != nil {
    log.Fatal(err)
}
defer producer.Close()

err = producer.Transact(ctx, func(tx *kafka2.Transaction) error {
    tx.Produce(kafka2.NewRecord([]byte("msg1")).WithTopic("topic1"))
    tx.Produce(kafka2.NewRecord([]byte("msg2")).WithTopic("topic2"))
    return nil // Commits on success
})
```

## Comparison with Current API

| Feature | Current API | Native API |
|---------|-------------|------------|
| Batch consumption | Hidden (single records) | Native `ConsumeBatches()` |
| Async callbacks | No per-record feedback | `ProduceAsync()` with callback |
| Transactions | Not supported | `Transact()` helper |
| Configuration | Large structs | Smaller focused configs |
| Record building | Manual struct | Fluent builder pattern |
| Offset control | Limited | `CommitRecord()`, `Pause/Resume` |
| Error handling | Per-operation | Batch-aware `FetchResult` |

## Migration Path

1. Create `provider/kafka2` package
2. Users can migrate incrementally
3. Eventually deprecate `provider/kafka`
4. Or maintain both for different use cases
