# Migrating from provider/kafka to provider/franz

This guide helps you migrate from the `provider/kafka` package (based on segmentio/kafka-go) to the `provider/franz`
package (based on twmb/franz-go).

## Why Migrate?

The `provider/franz` package offers significant advantages:

| Feature             | provider/kafka     | provider/franz                |
|---------------------|--------------------|-------------------------------|
| Underlying library  | segmentio/kafka-go | twmb/franz-go                 |
| Batch consumption   | Limited            | Native support                |
| Async production    | Basic              | Per-record callbacks          |
| Transactions        | Not supported      | Full support                  |
| AWS MSK IAM         | Not supported      | Supported                     |
| OAuth/OIDC          | Not supported      | Supported                     |
| Compression options | Limited            | All (gzip, snappy, lz4, zstd) |
| Performance         | Good               | Excellent                     |

## Breaking Changes Summary

1. **Import path changed** from `provider/kafka` to `provider/franz`
2. **No `Connect()` method** - clients connect automatically on creation
3. **`Close()` replaces `Disconnect()`** - method renamed
4. **No `GetConfig()` method** - configure before creation instead
5. **No public `Reader`/`Writer`/`Conn` fields** - use `Client()` method for advanced access
6. **Message types changed** - `Message` becomes `Record`/`ConsumedRecord`
7. **Handler signatures changed** - use `franz.ConsumedRecord` instead of `kafka.Message`
8. **Configuration structure simplified** - options embedded directly in config

## Import Changes

```go
// Before
import "github.com/oddbit-project/blueprint/provider/kafka"

// After
import "github.com/oddbit-project/blueprint/provider/franz"
```

## Producer Migration

### Configuration

```go
// Before (kafka)
cfg := &kafka.ProducerConfig{
    Brokers:  "localhost:9092",
    Topic:    "my-topic",
    AuthType: kafka.AuthTypePlain,
    Username: "user",
    DefaultCredentialConfig: secure.DefaultCredentialConfig{
        Password: "password",
    },
    ClientConfig: tlsProvider.ClientConfig{
        TLSEnable: true,
        TLSCA:     "/path/to/ca.crt",
    },
    ProducerOptions: kafka.ProducerOptions{
        BatchSize:    100,
        BatchTimeout: 1000,
        RequiredAcks: "all",
        Async:        true,
    },
}

// After (franz)
cfg := &franz.ProducerConfig{
    BaseConfig: franz.BaseConfig{
        Brokers:  "localhost:9092",
        AuthType: franz.AuthTypePlain,
        Username: "user",
        DefaultCredentialConfig: secure.DefaultCredentialConfig{
            Password: "password",
        },
        ClientConfig: tlsProvider.ClientConfig{
            TLSEnable: true,
            TLSCA:     "/path/to/ca.crt",
        },
    },
    DefaultTopic:    "my-topic",
    BatchMaxRecords: 100,
    Linger:          time.Second,
    Acks:            franz.AcksAll,
    // Note: Async is handled differently - use ProduceAsync() method
}
```

### Creating Producer

```go
// Before (kafka)
producer, err := kafka.NewProducer(cfg, logger)
if err != nil {
    // handle error
}
defer producer.Disconnect()

// After (franz)
producer, err := franz.NewProducer(cfg, logger)
if err != nil {
    // handle error
}
defer producer.Close()
```

### Writing Messages

```go
// Before (kafka) - Simple write
err := producer.Write(ctx, []byte("Hello"))

// After (franz) - Using Record builder
record := franz.NewRecord([]byte("Hello"))
results, err := producer.Produce(ctx, record)


// Before (kafka) - Write with key
err := producer.WriteWithKey(ctx, []byte("Hello"), []byte("key-1"))

// After (franz)
record := franz.NewRecord([]byte("Hello")).WithKey([]byte("key-1"))
results, err := producer.Produce(ctx, record)


// Before (kafka) - Write with headers
headers := []kafka.Header{
    {Key: "trace-id", Value: []byte("abc123")},
}
err := producer.WriteWithHeaders(ctx, []byte("Hello"), []byte("key"), headers)

// After (franz)
record := franz.NewRecord([]byte("Hello")).
    WithKey([]byte("key")).
    WithHeader("trace-id", []byte("abc123"))
results, err := producer.Produce(ctx, record)


// Before (kafka) - Write multiple messages
err := producer.WriteMulti(ctx, []byte("msg1"), []byte("msg2"), []byte("msg3"))

// After (franz)
records := []*franz.Record{
    franz.NewRecord([]byte("msg1")),
    franz.NewRecord([]byte("msg2")),
    franz.NewRecord([]byte("msg3")),
}
results, err := producer.Produce(ctx, records...)


// Before (kafka) - Write JSON
err := producer.WriteJson(ctx, myStruct, []byte("optional-key"))

// After (franz)
result, err := producer.ProduceJSON(ctx, myStruct, []byte("optional-key"))
```

### Async Production

```go
// Before (kafka) - Async was a config option
cfg := &kafka.ProducerConfig{
    // ...
    ProducerOptions: kafka.ProducerOptions{
        Async: true,
    },
}
producer, _ := kafka.NewProducer(cfg, logger)
producer.Write(ctx, []byte("Hello")) // Non-blocking due to Async: true

// After (franz) - Explicit async method with callback
producer, _ := franz.NewProducer(cfg, logger)
err := producer.ProduceAsync(ctx, franz.NewRecord([]byte("Hello")), func(result franz.ProduceResult) {
    if result.Err != nil {
        log.Error(result.Err, "Failed to produce")
    } else {
        log.Info("Produced", log.KV{
            "partition": result.Partition,
            "offset":    result.Offset,
        })
    }
})

// Wait for all async records to complete
producer.Flush(ctx)
```

### Checking Connection Status

```go
// Before (kafka)
if producer.IsConnected() {
    // ...
}

// After (franz) - Same method
if producer.IsConnected() {
    // ...
}
```

## Consumer Migration

### Configuration

```go
// Before (kafka)
cfg := &kafka.ConsumerConfig{
    Brokers:  "localhost:9092",
    Topic:    "my-topic",
    Group:    "my-group",
    AuthType: kafka.AuthTypeScram256,
    Username: "user",
    DefaultCredentialConfig: secure.DefaultCredentialConfig{
        Password: "password",
    },
    ConsumerOptions: kafka.ConsumerOptions{
        StartOffset:       "first",
        IsolationLevel:    "committed",
        SessionTimeout:    30000, // ms
        HeartbeatInterval: 3000,  // ms
        CommitInterval:    5000,  // ms
    },
}

// After (franz)
cfg := &franz.ConsumerConfig{
    BaseConfig: franz.BaseConfig{
        Brokers:  "localhost:9092",
        AuthType: franz.AuthTypeScram256,
        Username: "user",
        DefaultCredentialConfig: secure.DefaultCredentialConfig{
            Password: "password",
        },
    },
    Topics:             []string{"my-topic"},
    Group:              "my-group",
    StartOffset:        franz.OffsetStart, // "start" instead of "first"
    IsolationLevel:     franz.IsolationReadCommitted,
    SessionTimeout:     30 * time.Second,
    HeartbeatInterval:  3 * time.Second,
    AutoCommitInterval: 5 * time.Second,
    AutoCommit:         true,
}
```

### Creating Consumer

```go
// Before (kafka) - Required Connect() call
consumer, err := kafka.NewConsumer(cfg, logger)
if err != nil {
    // handle error
}
consumer.Connect() // Explicit connect required
defer consumer.Disconnect()

// After (franz) - Connects automatically
consumer, err := franz.NewConsumer(cfg, logger)
if err != nil {
    // handle error
}
defer consumer.Close()
```

### Subscribing to Messages

```go
// Before (kafka)
err := consumer.Subscribe(ctx, func(ctx context.Context, msg kafka.Message) error {
    fmt.Printf("Received: %s\n", string(msg.Value))
    fmt.Printf("Key: %s\n", string(msg.Key))
    fmt.Printf("Topic: %s, Partition: %d, Offset: %d\n",
        msg.Topic, msg.Partition, msg.Offset)
    return nil
})

// After (franz)
err := consumer.Consume(ctx, func(ctx context.Context, record franz.ConsumedRecord) error {
    fmt.Printf("Received: %s\n", string(record.Value))
    fmt.Printf("Key: %s\n", string(record.Key))
    fmt.Printf("Topic: %s, Partition: %d, Offset: %d\n",
        record.Topic, record.Partition, record.Offset)
    return nil
})
```

### Channel-Based Consumption

```go
// Before (kafka)
ch := make(chan kafka.Message, 100)
go consumer.ChannelSubscribe(ctx, ch)

for msg := range ch {
    processMessage(msg)
}

// After (franz)
ch := make(chan franz.ConsumedRecord, 100)
go consumer.ConsumeChannel(ctx, ch)

for record := range ch {
    processRecord(record)
}
```

### Manual Offset Commits

```go
// Before (kafka) - SubscribeWithOffsets handled this
err := consumer.SubscribeWithOffsets(ctx, func(ctx context.Context, msg kafka.Message) error {
    // Process message
    // Commit happens automatically after handler returns
    return nil
})

// After (franz) - More control over commits
cfg := &franz.ConsumerConfig{
    // ...
    AutoCommit: false, // Disable auto-commit
}

consumer, _ := franz.NewConsumer(cfg, logger)

err := consumer.Consume(ctx, func(ctx context.Context, record franz.ConsumedRecord) error {
    // Process record
    if err := processRecord(record); err != nil {
        return err
    }
    // Explicitly commit this record
    return consumer.CommitRecord(ctx, record)
})

// Or commit all at once
consumer.CommitOffsets(ctx)
```

### Reading Single Message

```go
// Before (kafka)
msg, err := consumer.ReadMessage(ctx)

// After (franz)
records, err := consumer.PollRecords(ctx, 1)
if err != nil {
    // handle error
}
if len(records) > 0 {
    record := records[0]
    // process record
}
```

### Rewinding Consumer

```go
// Before (kafka)
err := consumer.Rewind() // Must call before Connect()

// After (franz) - Configure in ConsumerConfig
cfg := &franz.ConsumerConfig{
    // ...
    StartOffset: franz.OffsetStart, // Start from beginning
}
```

## Admin Client Migration

### Configuration

```go
// Before (kafka)
cfg := &kafka.AdminConfig{
    Brokers:  "localhost:9092",
    AuthType: kafka.AuthTypePlain,
    Username: "admin",
    DefaultCredentialConfig: secure.DefaultCredentialConfig{
        Password: "password",
    },
}

// After (franz) - Same structure
cfg := &franz.AdminConfig{
    BaseConfig: franz.BaseConfig{
        Brokers:  "localhost:9092",
        AuthType: franz.AuthTypePlain,
        Username: "admin",
        DefaultCredentialConfig: secure.DefaultCredentialConfig{
            Password: "password",
        },
    },
}
```

### Creating Admin Client

```go
// Before (kafka) - Required Connect() call
admin, err := kafka.NewAdmin(cfg, logger)
if err != nil {
    // handle error
}
if err := admin.Connect(ctx); err != nil {
    // handle error
}
defer admin.Disconnect()

// After (franz) - Connects automatically
admin, err := franz.NewAdmin(cfg, logger)
if err != nil {
    // handle error
}
defer admin.Close()
```

### Topic Operations

```go
// Before (kafka)
topics, err := admin.ListTopics(ctx) // Returns []string
exists, err := admin.TopicExists(ctx, "my-topic")
err := admin.CreateTopic(ctx, "new-topic", 3, 1)
err := admin.DeleteTopic(ctx, "old-topic")

// After (franz)
topics, err := admin.ListTopics(ctx) // Returns []franz.TopicInfo
exists, err := admin.TopicExists(ctx, "my-topic")
err := admin.CreateTopics(ctx, franz.NewTopicConfig("new-topic", 3, 1))
err := admin.DeleteTopics(ctx, "old-topic")
```

### Topic Configuration

```go
// Before (kafka) - Basic topic creation
err := admin.CreateTopic(ctx, "my-topic", 6, 3)

// After (franz) - With configuration options
topicCfg := franz.NewTopicConfig("my-topic", 6, 3).
    WithConfig("retention.ms", "86400000").
    WithConfig("cleanup.policy", "compact")
err := admin.CreateTopics(ctx, topicCfg)
```

### Getting Topic Details

```go
// Before (kafka)
partitions, err := admin.GetTopics(ctx, "my-topic") // Returns []kafka.Partition

// After (franz)
topics, err := admin.DescribeTopics(ctx, "my-topic") // Returns []franz.TopicInfo
for _, topic := range topics {
    fmt.Printf("Topic: %s, Partitions: %d\n", topic.Name, len(topic.Partitions))
    for _, p := range topic.Partitions {
        fmt.Printf("  Partition %d: Leader=%d, Replicas=%v, ISR=%v\n",
            p.ID, p.Leader, p.Replicas, p.ISR)
    }
}
```

## New Features in franz

### Batch Consumption

Process records in batches for better throughput:

```go
err := consumer.ConsumeBatches(ctx, func(ctx context.Context, batch franz.Batch) error {
    fmt.Printf("Received batch: topic=%s, partition=%d, records=%d\n",
        batch.Topic, batch.Partition, len(batch.Records))

    for _, record := range batch.Records {
        processRecord(record)
    }
    return nil
})
```

### Fetch-Level Consumption

Maximum control for high-throughput scenarios:

```go
err := consumer.ConsumeFetches(ctx, func(ctx context.Context, result *franz.FetchResult) error {
    if result.HasErrors() {
        for _, fetchErr := range result.Errors {
            logger.Error(fetchErr.Err, "Fetch error")
        }
    }

    for _, batch := range result.Batches {
        for _, record := range batch.Records {
            processRecord(record)
        }
    }
    return nil
})
```

### Transactions

Atomic writes across multiple records:

```go
cfg := &franz.ProducerConfig{
    BaseConfig: franz.BaseConfig{
        Brokers: "localhost:9092",
    },
    TransactionalID: "my-transactional-producer",
    Acks:            franz.AcksAll,
}

producer, _ := franz.NewProducer(cfg, logger)

err := producer.Transact(ctx, func(tx *franz.Transaction) error {
    tx.Produce(franz.NewRecord([]byte("record 1")).WithTopic("topic-a"))
    tx.Produce(franz.NewRecord([]byte("record 2")).WithTopic("topic-b"))
    return nil // Commits on success, aborts on error
})
```

### Pause/Resume Consumption

```go
// Pause specific topics
consumer.Pause("topic-a", "topic-b")

// Resume topics
consumer.Resume("topic-a", "topic-b")

// Pause specific partitions
consumer.PausePartitions(map[string][]int32{
    "topic-a": {0, 1},
})
```

### AWS MSK IAM Authentication

```go
cfg := &franz.BaseConfig{
    Brokers:   "b-1.msk.region.amazonaws.com:9098",
    AuthType:  franz.AuthTypeAWSMSKIAM,
    AWSRegion: "us-east-1",
    ClientConfig: tlsProvider.ClientConfig{
        TLSEnable: true,
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
    OAuthCredentialConfig: franz.OAuthCredentialConfig{
        OAuthSecret: secure.DefaultCredentialConfig{
            Password: "client-secret",
        },
    },
}
```

### Consumer Group Management

```go
// List all consumer groups
groups, err := admin.ListGroups(ctx)

// Describe groups
details, err := admin.DescribeGroups(ctx, "group-1", "group-2")
for _, group := range details {
    fmt.Printf("Group: %s, State: %s, Members: %d\n",
        group.Name, group.State, len(group.Members))
}

// Delete groups
err := admin.DeleteGroups(ctx, "old-group")
```

### Broker Information

```go
brokers, err := admin.ListBrokers(ctx)
for _, broker := range brokers {
    fmt.Printf("Broker %d: %s:%d\n", broker.ID, broker.Host, broker.Port)
}
```

## Configuration Mapping Reference

### Authentication Types

| kafka                    | franz                     |
|--------------------------|---------------------------|
| `kafka.AuthTypeNone`     | `franz.AuthTypeNone`      |
| `kafka.AuthTypePlain`    | `franz.AuthTypePlain`     |
| `kafka.AuthTypeScram256` | `franz.AuthTypeScram256`  |
| `kafka.AuthTypeScram512` | `franz.AuthTypeScram512`  |
| -                        | `franz.AuthTypeAWSMSKIAM` |
| -                        | `franz.AuthTypeOAuth`     |

### Acks Configuration

| kafka (ProducerOptions) | franz                    |
|-------------------------|--------------------------|
| `RequiredAcks: "none"`  | `Acks: franz.AcksNone`   |
| `RequiredAcks: "one"`   | `Acks: franz.AcksLeader` |
| `RequiredAcks: "all"`   | `Acks: franz.AcksAll`    |

### Start Offset

| kafka (ConsumerOptions) | franz                            |
|-------------------------|----------------------------------|
| `StartOffset: "first"`  | `StartOffset: franz.OffsetStart` |
| `StartOffset: "last"`   | `StartOffset: franz.OffsetEnd`   |

### Isolation Level

| kafka (ConsumerOptions)         | franz                                            |
|---------------------------------|--------------------------------------------------|
| `IsolationLevel: "uncommitted"` | `IsolationLevel: franz.IsolationReadUncommitted` |
| `IsolationLevel: "committed"`   | `IsolationLevel: franz.IsolationReadCommitted`   |

### Time Units

| kafka                   | franz                              |
|-------------------------|------------------------------------|
| Milliseconds (uint)     | `time.Duration`                    |
| `BatchTimeout: 1000`    | `Linger: time.Second`              |
| `SessionTimeout: 30000` | `SessionTimeout: 30 * time.Second` |

## Error Mapping

| kafka                      | franz                |
|----------------------------|----------------------|
| `ErrMissingProducerBroker` | `ErrMissingBrokers`  |
| `ErrMissingProducerTopic`  | `ErrMissingTopic`    |
| `ErrProducerClosed`        | `ErrClientClosed`    |
| `ErrMissingConsumerBroker` | `ErrMissingBrokers`  |
| `ErrMissingConsumerTopic`  | `ErrMissingTopic`    |
| `ErrInvalidAuthType`       | `ErrInvalidAuthType` |
| `ErrNilConfig`             | `ErrNilConfig`       |
| `ErrNilHandler`            | `ErrNilHandler`      |
| `ErrNilContext`            | `ErrNilContext`      |
| `ErrMissingAdminBroker`    | `ErrMissingBrokers`  |
| `ErrAdminNotConnected`     | `ErrClientClosed`    |

## Checklist

Use this checklist when migrating:

- [ ] Update import statements to `provider/franz`
- [ ] Replace `ProducerConfig` structure (move options to `BaseConfig`)
- [ ] Replace `ConsumerConfig` structure (move options to `BaseConfig`)
- [ ] Replace `AdminConfig` structure (wrap in `BaseConfig`)
- [ ] Replace `Disconnect()` calls with `Close()`
- [ ] Remove `Connect()` calls (automatic in franz)
- [ ] Update message handling to use `franz.ConsumedRecord`
- [ ] Replace `Write*` methods with `Produce*` methods
- [ ] Update `Subscribe` to `Consume` with new handler signature
- [ ] Update `ChannelSubscribe` to `ConsumeChannel`
- [ ] Replace time values from milliseconds (uint) to `time.Duration`
- [ ] Update start offset values (`"first"` -> `OffsetStart`, `"last"` -> `OffsetEnd`)
- [ ] Update acks values (`"one"` -> `AcksLeader`)
- [ ] Update admin topic operations (single -> variadic)
- [ ] Remove `GetConfig()` calls
- [ ] Update error handling for new error types
- [ ] Consider adding batch consumption for better performance
- [ ] Consider adding transactions where appropriate
