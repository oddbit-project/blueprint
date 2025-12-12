# Changelog

All notable changes to the franz Kafka provider will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [v0.8.0]

### Added

#### Producer
- Synchronous message production with `Produce()` returning detailed results
- Asynchronous message production with `ProduceAsync()` and callbacks
- JSON serialization helpers: `ProduceJSON()` and `ProduceJSONAsync()`
- Fluent record builder with `NewRecord()`, `WithKey()`, `WithTopic()`, `WithPartition()`, `WithTimestamp()`, `WithHeader()`, `WithHeaders()`
- Configurable batching: max records, max bytes, and linger time
- Acknowledgment modes: none, leader, all (ISR)
- Compression support: none, gzip, snappy, lz4, zstd
- Idempotent producer support for exactly-once semantics
- `Flush()` method to wait for buffered records

#### Transactions
- Full transaction support with `BeginTransaction()`, `Commit()`, and `Abort()`
- Convenience method `Transact()` for automatic commit/abort with panic recovery
- Batch transaction helper `TransactRecords()` for producing multiple records atomically

#### Consumer
- Poll-based consumption with `Poll()` and `PollRecords()`
- Multiple consumption patterns:
  - Single record processing with `Consume()`
  - Batch processing by partition with `ConsumeBatches()`
  - Full fetch handling with `ConsumeFetches()` for high-throughput scenarios
  - Channel-based consumption with `ConsumeChannel()`
- Consumer group support with configurable session timeout, rebalance timeout, and heartbeat interval
- Automatic offset commit with configurable interval
- Manual offset commit: `CommitOffsets()`, `CommitRecord()`, `CommitBatch()`
- Pause/resume functionality for topics and partitions
- Configurable start offset: earliest or latest
- Configurable isolation levels: read uncommitted or read committed
- Configurable fetch settings: min bytes, max bytes, max wait time

#### Admin Client
- Topic management:
  - `CreateTopics()` with partition count, replication factor, and custom configs
  - `DeleteTopics()` for topic removal
  - `ListTopics()` and `DescribeTopics()` for topic inspection
  - `TopicExists()` for existence checks
- Broker management:
  - `ListBrokers()` for cluster topology inspection
- Consumer group management:
  - `ListGroups()` and `DescribeGroups()` for group inspection
  - `DeleteGroups()` for group removal

#### Authentication
- Plain text authentication (SASL/PLAIN)
- SCRAM-SHA-256 authentication
- SCRAM-SHA-512 authentication
- AWS MSK IAM authentication with explicit credentials or default credential chain
- OAuth/OIDC OAUTHBEARER authentication

#### Security
- TLS/SSL support via embedded `tlsProvider.ClientConfig`
- Secure credential handling via `secure.DefaultCredentialConfig`
- Automatic credential cleanup from memory after use

#### Configuration
- Sensible defaults for all configuration options
- Connection settings: dial timeout, request timeout, retry backoff, max retries
- Validation methods for all configuration types
- JSON-serializable configuration structures

#### Observability
- Structured logging with `log.Logger` integration
- Specialized loggers for producer, consumer, and admin operations
- Error logging with contextual information (topic, partition, offset)

#### General
- Thread-safe client operations with mutex protection
- Context-aware operations throughout
- Graceful connection state checking with `IsConnected()`
- Access to underlying franz-go client for advanced use cases
