# blueprint.provider.franz

Blueprint Kafka client built on [franz-go](https://github.com/twmb/franz-go), providing high-performance Kafka
operations with native batch processing, transactions, and async production.

## Features

- **High-performance**: Built on franz-go for optimal throughput and latency
- **Batch processing**: Native batch consumption with `ConsumeBatches` and `ConsumeFetches`
- **Async production**: Non-blocking production with per-record callbacks
- **Transactions**: Full support for Kafka transactions with automatic commit/abort
- **Multiple authentication methods**: PLAIN, SCRAM-SHA-256, SCRAM-SHA-512, AWS MSK IAM, OAuth/OIDC
- **Secure credentials**: In-memory encryption for passwords and secrets
- **TLS support**: Full TLS configuration for secure connections
- **Fluent API**: Builder pattern for record creation
- **Admin operations**: Topic and consumer group management

## Authentication Types

| Type        | Constant            | Description                  |
|-------------|---------------------|------------------------------|
| None        | `AuthTypeNone`      | No authentication            |
| PLAIN       | `AuthTypePlain`     | SASL PLAIN authentication    |
| SCRAM-256   | `AuthTypeScram256`  | SCRAM-SHA-256 authentication |
| SCRAM-512   | `AuthTypeScram512`  | SCRAM-SHA-512 authentication |
| AWS MSK IAM | `AuthTypeAWSMSKIAM` | AWS MSK IAM authentication   |
| OAuth       | `AuthTypeOAuth`     | OAuth/OIDC OAUTHBEARER       |

## Producer

### Configuration

```go
type ProducerConfig struct {
    BaseConfig

    DefaultTopic string // Default topic for records without explicit topic

    // Batching
    BatchMaxRecords int           // Max records per batch (default: 10000)
    BatchMaxBytes   int           // Max bytes per batch (default: 1MB)
    Linger          time.Duration // Time to wait for batch fill (default: 0)

    // Reliability
    Acks            string // none, leader, all (default: leader)
    Idempotent      bool   // Enable idempotent producer
    TransactionalID string // For transactional producer

    // Compression
    Compression string // none, gzip, snappy, lz4, zstd
}
```

### Creating a Producer

```go
package main

import (
    "context"
    "github.com/oddbit-project/blueprint/crypt/secure"
    "github.com/oddbit-project/blueprint/log"
    "github.com/oddbit-project/blueprint/provider/franz"
    tlsProvider "github.com/oddbit-project/blueprint/provider/tls"
)

func main() {
    logger := log.New("kafka-producer")

    cfg := &franz.ProducerConfig{
        BaseConfig: franz.BaseConfig{
            Brokers:  "localhost:9092,localhost:9093",
            AuthType: franz.AuthTypeScram256,
            Username: "producer-user",
            DefaultCredentialConfig: secure.DefaultCredentialConfig{
                Password: "producer-password",
                // Or use environment variables:
                // PasswordEnvVar: "KAFKA_PASSWORD",
                // Or use files:
                // PasswordFile: "/run/secrets/kafka-password",
            },
            ClientConfig: tlsProvider.ClientConfig{
                TLSEnable: true,
                TLSCA:     "/path/to/ca.crt",
            },
            DialTimeout:    30 * time.Second,
            RequestTimeout: 30 * time.Second,
        },
        DefaultTopic:    "my-topic",
        BatchMaxRecords: 10000,
        BatchMaxBytes:   1048576,
        Acks:            franz.AcksAll,
        Compression:     franz.CompressionSnappy,
    }

    producer, err := franz.NewProducer(cfg, logger)
    if err != nil {
        logger.Fatal(err, "Failed to create producer")
    }
    defer producer.Close()

    // Producer is ready to use
}
```

### Synchronous Production

```go
ctx := context.Background()

// Create a record using the fluent builder
record := franz.NewRecord([]byte("Hello, Kafka!")).
    WithKey([]byte("user-123")).
    WithHeader("trace-id", []byte("abc123"))

// Produce synchronously
results, err := producer.Produce(ctx, record)
if err != nil {
    logger.Error(err, "Failed to produce")
    return
}

for _, result := range results {
    if result.Err != nil {
        logger.Error(result.Err, "Record failed", log.KV{
            "partition": result.Partition,
        })
    } else {
        logger.Info("Record sent", log.KV{
            "partition": result.Partition,
            "offset":    result.Offset,
        })
    }
}
```

### Asynchronous Production

```go
// Produce asynchronously with callback
err := producer.ProduceAsync(ctx, record, func(result franz.ProduceResult) {
    if result.Err != nil {
        logger.Error(result.Err, "Async produce failed")
    } else {
        logger.Info("Async produce succeeded", log.KV{
            "partition": result.Partition,
            "offset":    result.Offset,
        })
    }
})
if err != nil {
    logger.Error(err, "Failed to queue record")
}

// Wait for all buffered records to be sent
if err := producer.Flush(ctx); err != nil {
    logger.Error(err, "Flush failed")
}
```

### JSON Production

```go
type Event struct {
    ID        string    `json:"id"`
    Type      string    `json:"type"`
    Timestamp time.Time `json:"timestamp"`
}

event := Event{
    ID:        "evt-123",
    Type:      "user.created",
    Timestamp: time.Now(),
}

// Synchronous JSON production
result, err := producer.ProduceJSON(ctx, event, []byte("user-123"))
if err != nil {
    logger.Error(err, "Failed to produce JSON")
}

// Asynchronous JSON production
err = producer.ProduceJSONAsync(ctx, event, []byte("user-123"), func(result franz.ProduceResult) {
    if result.Err != nil {
        logger.Error(result.Err, "Async JSON produce failed")
    }
})
```

### Producing to Multiple Topics

```go
// Records can specify their own topic
records := []*franz.Record{
    franz.NewRecord([]byte("to topic A")).WithTopic("topic-a"),
    franz.NewRecord([]byte("to topic B")).WithTopic("topic-b"),
    franz.NewRecord([]byte("to default topic")), // Uses DefaultTopic
}

results, err := producer.Produce(ctx, records...)
```

## Consumer

### Configuration

```go
type ConsumerConfig struct {
    BaseConfig

    Topics []string // Topics to consume
    Group  string   // Consumer group (required for group consumption)

    // Consumer behavior
    StartOffset    string // start, end (default: end)
    IsolationLevel string // uncommitted, committed (default: committed)

    // Group settings
    SessionTimeout    time.Duration // Default: 45s
    RebalanceTimeout  time.Duration // Default: 60s
    HeartbeatInterval time.Duration // Default: 3s

    // Fetch settings
    FetchMinBytes int           // Default: 1
    FetchMaxBytes int           // Default: 50MB
    FetchMaxWait  time.Duration // Default: 5s

    // Offset management
    AutoCommit         bool          // Default: true
    AutoCommitInterval time.Duration // Default: 5s
}
```

### Creating a Consumer

```go
package main

import (
    "context"
    "github.com/oddbit-project/blueprint/crypt/secure"
    "github.com/oddbit-project/blueprint/log"
    "github.com/oddbit-project/blueprint/provider/franz"
)

func main() {
    logger := log.New("kafka-consumer")

    cfg := &franz.ConsumerConfig{
        BaseConfig: franz.BaseConfig{
            Brokers:  "localhost:9092",
            AuthType: franz.AuthTypeScram256,
            Username: "consumer-user",
            DefaultCredentialConfig: secure.DefaultCredentialConfig{
                Password: "consumer-password",
            },
        },
        Topics:             []string{"my-topic", "another-topic"},
        Group:              "my-consumer-group",
        StartOffset:        franz.OffsetEnd,
        IsolationLevel:     franz.IsolationReadCommitted,
        SessionTimeout:     45 * time.Second,
        AutoCommit:         true,
        AutoCommitInterval: 5 * time.Second,
    }

    consumer, err := franz.NewConsumer(cfg, logger)
    if err != nil {
        logger.Fatal(err, "Failed to create consumer")
    }
    defer consumer.Close()

    // Consumer is ready to use
}
```

### Record-by-Record Consumption

```go
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

// Process records one at a time
err := consumer.Consume(ctx, func(ctx context.Context, record franz.ConsumedRecord) error {
    logger.Info("Received record", log.KV{
        "topic":     record.Topic,
        "partition": record.Partition,
        "offset":    record.Offset,
        "key":       string(record.Key),
    })

    // Process the record
    if err := processRecord(record); err != nil {
        return err // Stops consumption on error
    }

    return nil
})

if err != nil {
    logger.Error(err, "Consumption error")
}
```

### Batch Consumption

Batch consumption groups records by topic/partition for more efficient processing:

```go
err := consumer.ConsumeBatches(ctx, func(ctx context.Context, batch franz.Batch) error {
    logger.Info("Received batch", log.KV{
        "topic":       batch.Topic,
        "partition":   batch.Partition,
        "recordCount": len(batch.Records),
    })

    // Process all records in the batch
    for _, record := range batch.Records {
        if err := processRecord(record); err != nil {
            return err
        }
    }

    // Optionally commit the batch manually (if AutoCommit is disabled)
    // return consumer.CommitBatch(ctx, batch)

    return nil
})
```

### Fetch-Level Consumption

For maximum control and throughput, process entire fetch results:

```go
err := consumer.ConsumeFetches(ctx, func(ctx context.Context, result *franz.FetchResult) error {
    if result.HasErrors() {
        for _, fetchErr := range result.Errors {
            logger.Error(fetchErr.Err, "Fetch error", log.KV{
                "topic":     fetchErr.Topic,
                "partition": fetchErr.Partition,
            })
        }
        // Decide how to handle errors
    }

    logger.Info("Processing fetch", log.KV{
        "batchCount":  len(result.Batches),
        "recordCount": result.RecordCount(),
    })

    for _, batch := range result.Batches {
        for _, record := range batch.Records {
            processRecord(record)
        }
    }

    return nil
})
```

### Channel-Based Consumption

Send records to a channel for processing in separate goroutines:

```go
records := make(chan franz.ConsumedRecord, 100)

// Start consumer in goroutine
go func() {
    err := consumer.ConsumeChannel(ctx, records)
    if err != nil {
        logger.Error(err, "Channel consumption error")
    }
    close(records)
}()

// Process records from channel
for record := range records {
    processRecord(record)
}
```

### Manual Offset Commits

When `AutoCommit` is disabled, commit offsets manually:

```go
cfg := &franz.ConsumerConfig{
    // ...
    AutoCommit: false,
}

consumer, _ := franz.NewConsumer(cfg, logger)

err := consumer.Consume(ctx, func(ctx context.Context, record franz.ConsumedRecord) error {
    if err := processRecord(record); err != nil {
        return err
    }

    // Commit this specific record's offset
    return consumer.CommitRecord(ctx, record)
})

// Or commit all consumed offsets at once
if err := consumer.CommitOffsets(ctx); err != nil {
    logger.Error(err, "Failed to commit offsets")
}
```

### Pause and Resume

Temporarily pause consumption of specific topics or partitions:

```go
// Pause specific topics
consumer.Pause("topic-a", "topic-b")

// Resume topics
consumer.Resume("topic-a", "topic-b")

// Pause specific partitions
consumer.PausePartitions(map[string][]int32{
    "topic-a": {0, 1},
    "topic-b": {2},
})

// Resume specific partitions
consumer.ResumePartitions(map[string][]int32{
    "topic-a": {0, 1},
})
```

### Polling Records

For more control, poll records directly:

```go
// Poll up to 100 records
records, err := consumer.PollRecords(ctx, 100)
if err != nil {
    logger.Error(err, "Poll failed")
    return
}

for _, record := range records {
    processRecord(record)
}

// Or poll entire fetch result
result, err := consumer.Poll(ctx)
if err != nil {
    logger.Error(err, "Poll failed")
    return
}

if !result.IsEmpty() {
    for _, record := range result.Records() {
        processRecord(record)
    }
}
```

## Transactions

Transactions ensure atomic production of multiple records.

### Configuration

```go
cfg := &franz.ProducerConfig{
    BaseConfig: franz.BaseConfig{
        Brokers: "localhost:9092",
    },
    TransactionalID: "my-transactional-producer",
    Acks:            franz.AcksAll, // Required for transactions
}
```

### Using the Transact Helper

The `Transact` method handles begin, commit, and abort automatically:

```go
err := producer.Transact(ctx, func(tx *franz.Transaction) error {
    // Add records to the transaction
    tx.Produce(franz.NewRecord([]byte("record 1")).WithTopic("topic-a"))
    tx.Produce(franz.NewRecord([]byte("record 2")).WithTopic("topic-b"))

    // If this function returns an error, the transaction is aborted
    // If it returns nil, the transaction is committed
    return nil
})

if err != nil {
    logger.Error(err, "Transaction failed")
}
```

### Convenience Method for Multiple Records

```go
records := []*franz.Record{
    franz.NewRecord([]byte("record 1")),
    franz.NewRecord([]byte("record 2")),
    franz.NewRecord([]byte("record 3")),
}

err := producer.TransactRecords(ctx, records...)
```

### Manual Transaction Control

For more control over the transaction lifecycle:

```go
tx, err := producer.BeginTransaction(ctx)
if err != nil {
    logger.Error(err, "Failed to begin transaction")
    return
}

// Add records
tx.Produce(franz.NewRecord([]byte("record 1")))
tx.ProduceMany(
    franz.NewRecord([]byte("record 2")),
    franz.NewRecord([]byte("record 3")),
)

// Check state
logger.Info("Transaction state", log.KV{
    "recordCount": tx.RecordCount(),
    "isAborted":   tx.IsAborted(),
})

// Commit or abort based on some condition
if shouldCommit {
    if err := tx.Commit(); err != nil {
        logger.Error(err, "Failed to commit transaction")
    }
} else {
    if err := tx.Abort(); err != nil {
        logger.Error(err, "Failed to abort transaction")
    }
}
```

## Admin Client

### Creating an Admin Client

```go
cfg := &franz.AdminConfig{
    BaseConfig: franz.BaseConfig{
        Brokers:  "localhost:9092",
        AuthType: franz.AuthTypeNone,
    },
}

admin, err := franz.NewAdmin(cfg, logger)
if err != nil {
    logger.Fatal(err, "Failed to create admin client")
}
defer admin.Close()
```

### Topic Management

```go
ctx := context.Background()

// Create a topic
topicCfg := franz.NewTopicConfig("my-new-topic", 6, 3).
    WithConfig("retention.ms", "86400000").      // 1 day
    WithConfig("cleanup.policy", "delete")

if err := admin.CreateTopics(ctx, topicCfg); err != nil {
    logger.Error(err, "Failed to create topic")
}

// Create multiple topics
err := admin.CreateTopics(ctx,
    franz.NewTopicConfig("topic-1", 3, 1),
    franz.NewTopicConfig("topic-2", 6, 2),
)

// List all topics
topics, err := admin.ListTopics(ctx)
if err != nil {
    logger.Error(err, "Failed to list topics")
}

for _, topic := range topics {
    logger.Info("Topic", log.KV{
        "name":       topic.Name,
        "partitions": len(topic.Partitions),
        "internal":   topic.Internal,
    })
}

// Check if topic exists
exists, err := admin.TopicExists(ctx, "my-topic")

// Describe specific topics
details, err := admin.DescribeTopics(ctx, "topic-1", "topic-2")

// Delete topics
if err := admin.DeleteTopics(ctx, "old-topic-1", "old-topic-2"); err != nil {
    logger.Error(err, "Failed to delete topics")
}
```

### Broker Information

```go
brokers, err := admin.ListBrokers(ctx)
if err != nil {
    logger.Error(err, "Failed to list brokers")
}

for _, broker := range brokers {
    logger.Info("Broker", log.KV{
        "id":   broker.ID,
        "host": broker.Host,
        "port": broker.Port,
        "rack": broker.Rack,
    })
}
```

### Consumer Group Management

```go
// List all consumer groups
groups, err := admin.ListGroups(ctx)
if err != nil {
    logger.Error(err, "Failed to list groups")
}

for _, group := range groups {
    logger.Info("Consumer group", log.KV{"name": group})
}

// Describe specific groups
details, err := admin.DescribeGroups(ctx, "group-1", "group-2")
for _, group := range details {
    logger.Info("Group details", log.KV{
        "name":        group.Name,
        "state":       group.State,
        "protocol":    group.Protocol,
        "memberCount": len(group.Members),
    })
}

// Delete consumer groups (groups must be empty)
if err := admin.DeleteGroups(ctx, "old-group"); err != nil {
    logger.Error(err, "Failed to delete group")
}
```

## Authentication Examples

### SCRAM Authentication

```go
cfg := &franz.BaseConfig{
    Brokers:  "localhost:9092",
    AuthType: franz.AuthTypeScram512, // or AuthTypeScram256
    Username: "kafka-user",
    DefaultCredentialConfig: secure.DefaultCredentialConfig{
        // Direct password
        Password: "secret-password",

        // Or from environment variable
        // PasswordEnvVar: "KAFKA_PASSWORD",

        // Or from file
        // PasswordFile: "/run/secrets/kafka-password",
    },
}
```

### AWS MSK IAM Authentication

```go
cfg := &franz.BaseConfig{
    Brokers:  "b-1.msk-cluster.region.amazonaws.com:9098",
    AuthType: franz.AuthTypeAWSMSKIAM,
    AWSRegion: "us-east-1",

    // Option 1: Use IAM role (recommended for EKS, EC2, Lambda)
    // Leave AWSAccessKey empty to use default credential chain

    // Option 2: Explicit credentials
    AWSAccessKey: "AKIAIOSFODNN7EXAMPLE",
    AwsCredentialConfig: franz.AwsCredentialConfig{
        AwsSecret: secure.DefaultCredentialConfig{
            Password: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
            // Or use environment variable:
            // PasswordEnvVar: "AWS_SECRET_ACCESS_KEY",
        },
    },

    ClientConfig: tlsProvider.ClientConfig{
        TLSEnable: true, // Required for MSK IAM
    },
}
```

### OAuth/OIDC Authentication

```go
cfg := &franz.BaseConfig{
    Brokers:       "kafka.example.com:9092",
    AuthType:      franz.AuthTypeOAuth,
    OAuthTokenURL: "https://auth.example.com/oauth/token",
    OAuthClientID: "kafka-client",
    OAuthScope:    "kafka:read kafka:write", // Optional
    OAuthCredentialConfig: franz.OAuthCredentialConfig{
        OAuthSecret: secure.DefaultCredentialConfig{
            Password: "client-secret",
            // Or from environment:
            // PasswordEnvVar: "OAUTH_CLIENT_SECRET",
        },
    },
}
```

## Record Builder

The fluent builder pattern makes record creation clean and readable:

```go
// Simple record
record := franz.NewRecord([]byte("Hello"))

// With key
record := franz.NewRecord([]byte("Hello")).
    WithKey([]byte("user-123"))

// With topic override
record := franz.NewRecord([]byte("Hello")).
    WithTopic("specific-topic")

// With partition
record := franz.NewRecord([]byte("Hello")).
    WithPartition(0)

// With timestamp
record := franz.NewRecord([]byte("Hello")).
    WithTimestamp(time.Now())

// With headers
record := franz.NewRecord([]byte("Hello")).
    WithHeader("trace-id", []byte("abc123")).
    WithHeader("source", []byte("api"))

// With multiple headers at once
record := franz.NewRecord([]byte("Hello")).
    WithHeaders(
        franz.Header{Key: "trace-id", Value: []byte("abc123")},
        franz.Header{Key: "source", Value: []byte("api")},
    )

// Full example
record := franz.NewRecord([]byte(`{"event":"user.created"}`)).
    WithKey([]byte("user-123")).
    WithTopic("events").
    WithHeader("content-type", []byte("application/json")).
    WithHeader("trace-id", []byte("abc123"))
```

## Compression

Available compression types:

| Constant            | Description           |
|---------------------|-----------------------|
| `CompressionNone`   | No compression        |
| `CompressionGzip`   | Gzip compression      |
| `CompressionSnappy` | Snappy compression    |
| `CompressionLz4`    | LZ4 compression       |
| `CompressionZstd`   | Zstandard compression |

```go
cfg := &franz.ProducerConfig{
    // ...
    Compression: franz.CompressionZstd, // Best compression ratio
    // or
    Compression: franz.CompressionSnappy, // Fast, good for real-time
}
```

## Acks Configuration

| Constant     | Description                         |
|--------------|-------------------------------------|
| `AcksNone`   | No acknowledgment (fire and forget) |
| `AcksLeader` | Wait for leader acknowledgment      |
| `AcksAll`    | Wait for all in-sync replicas       |

```go
cfg := &franz.ProducerConfig{
    // ...
    Acks: franz.AcksAll, // Maximum durability
}
```

## Offset Configuration

| Constant      | Description                                      |
|---------------|--------------------------------------------------|
| `OffsetStart` | Start consuming from the earliest offset         |
| `OffsetEnd`   | Start consuming from the latest offset (default) |

## Isolation Levels

| Constant                   | Description                             |
|----------------------------|-----------------------------------------|
| `IsolationReadUncommitted` | Read all messages including uncommitted |
| `IsolationReadCommitted`   | Only read committed messages (default)  |

## Accessing Underlying Clients

For advanced use cases, access the underlying franz-go clients:

```go
// Producer
kgoClient := producer.Client() // *kgo.Client

// Consumer
kgoClient := consumer.Client() // *kgo.Client

// Admin
kgoClient := admin.Client()      // *kgo.Client
kadmClient := admin.AdminClient() // *kadm.Client
```

## Error Constants

```go
ErrNilConfig            // config is nil
ErrMissingBrokers       // brokers address is required
ErrMissingTopic         // topic is required
ErrMissingGroup         // consumer group is required for group consumption
ErrClientClosed         // client is closed
ErrInvalidAuthType      // invalid authentication type
ErrInvalidAcks          // invalid acks value
ErrInvalidCompression   // invalid compression type
ErrInvalidOffset        // invalid start offset value
ErrInvalidIsolation     // invalid isolation level
ErrTransactionAborted   // transaction was aborted
ErrNoTransactionalID    // transactional ID required for transactions
ErrNilHandler           // handler function is nil
ErrNilContext           // context is nil
ErrMissingAWSRegion     // AWS region is required for MSK IAM authentication
ErrMissingOAuthTokenURL // OAuth token URL is required for OAuth authentication
```

## Best Practices

1. **Use batch consumption** for high-throughput scenarios to reduce per-record overhead
2. **Enable compression** (snappy or zstd) to reduce network bandwidth
3. **Set appropriate acks** - use `AcksAll` for critical data, `AcksLeader` for lower latency
4. **Use transactions** when you need atomic writes across multiple records or topics
5. **Tune fetch settings** - increase `FetchMaxBytes` and `FetchMaxWait` for throughput
6. **Handle errors gracefully** - check `FetchResult.HasErrors()` when using fetch-level consumption
7. **Use secure credentials** - prefer environment variables or files over hardcoded passwords
8. **Enable TLS** in production environments
9. **Use consumer groups** for scalable consumption across multiple instances
10. **Set appropriate timeouts** - balance between responsiveness and stability
