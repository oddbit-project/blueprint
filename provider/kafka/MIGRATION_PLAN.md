# Kafka Provider Migration Plan: segmentio/kafka-go to twmb/franz-go

## Overview

This document outlines the migration plan for the Kafka provider from `github.com/segmentio/kafka-go` to `github.com/twmb/franz-go`.

### Motivation

Franz-go offers:
- Better performance and lower latency
- More modern API design
- Better support for newer Kafka features
- Active maintenance and community support

## Migration Checklist

### 1. Dependencies (go.mod)

- [x] Remove `github.com/segmentio/kafka-go v0.4.49`
- [x] Add `github.com/twmb/franz-go v1.18.1`
- [x] Add `github.com/twmb/franz-go/pkg/kadm v1.15.0` (for admin operations)
- [x] Run `go mod tidy`

### 2. Core Types (kafka.go)

- [x] Update SASL imports from `kafka-go/sasl/*` to `franz-go/pkg/sasl/*`
- [x] Update `createSASLMechanism()` function:
  - [x] Change return type from `sasl.Mechanism` (kafka-go) to `sasl.Mechanism` (franz-go)
  - [x] Update `plain.Mechanism` to `plain.Auth{}.AsMechanism()`
  - [x] Update `scram.Mechanism()` to `scram.Auth{}.AsSha256Mechanism()` / `AsSha512Mechanism()`
- [x] Create custom `Header` type (franz-go uses `kgo.RecordHeader`)
- [x] Create custom `Message` type to replace `kafka.Message` type alias

### 3. Consumer (consumer.go)

- [x] Replace `kafka.Reader` with `kgo.Client`
- [x] Replace `kafka.ReaderConfig` with internal `consumerConfig` struct
- [x] Update imports from `segmentio/kafka-go` to `twmb/franz-go/pkg/kgo`
- [x] Remove `Message` type alias (now using custom `Message` type)
- [x] Update `Consumer` struct:
  - [x] Replace `Reader *kafka.Reader` with `client *kgo.Client`
  - [x] Replace `config *kafka.ReaderConfig` with `config *consumerConfig`
- [x] Update `ApplyOptions()` to `buildClientOpts()`:
  - [x] Convert kafka-go options to franz-go equivalents
  - [x] Handle consumer group settings differently (franz-go uses different APIs)
- [x] Update `NewConsumer()`:
  - [x] Build `kgo.Opt` slice instead of `kafka.ReaderConfig`
  - [x] Handle SASL via `kgo.SASL()` option
  - [x] Handle TLS via `kgo.DialTLSConfig()` option
- [x] Update `Connect()`:
  - [x] Change from `kafka.NewReader()` to `kgo.NewClient()`
  - [x] Return error (franz-go client creation can fail)
- [x] Update `Disconnect()`:
  - [x] Change from `reader.Close()` to `client.Close()`
- [x] Remove `GetConfig()` method (internal config is no longer exposed)
- [x] Update `Rewind()`:
  - [x] Change from `kafka.FirstOffset` to `startFromOldest` flag
- [x] Create `buildClient()` helper method for lazy client creation
- [x] Create `recordToMessage()` helper to convert `kgo.Record` to `Message`
- [x] Update `Subscribe()`:
  - [x] Replace `reader.ReadMessage()` with `client.PollFetches()`
  - [x] Handle batched message retrieval (franz-go returns batches)
  - [x] Update error handling for franz-go error types
- [x] Update `ReadMessage()`:
  - [x] Replace `reader.ReadMessage()` with `client.PollRecords(ctx, 1)`
- [x] Update `ChannelSubscribe()`:
  - [x] Replace `reader.ReadMessage()` with `client.PollFetches()`
- [x] Update `SubscribeWithOffsets()`:
  - [x] Replace `reader.FetchMessage()` with `client.PollFetches()`
  - [x] Replace `reader.CommitMessages()` with `client.CommitRecords()`

### 4. Producer (producer.go)

- [x] Replace `kafka.Writer` with `kgo.Client`
- [x] Update imports from `segmentio/kafka-go` to `twmb/franz-go/pkg/kgo`
- [x] Update `Producer` struct:
  - [x] Replace `Writer *kafka.Writer` with `client *kgo.Client`
  - [x] Add `async bool` field for async mode
- [x] Update `ApplyOptions()` to `buildProducerOpts()`:
  - [x] Convert `MaxAttempts` to `kgo.RequestRetries()`
  - [x] Convert `BatchSize` to `kgo.MaxBufferedRecords()`
  - [x] Convert `BatchBytes` to `kgo.MaxBufferedBytes()`
  - [x] Convert `BatchTimeout` to `kgo.ProducerLinger()`
  - [x] Convert `WriteTimeout` to `kgo.ProduceRequestTimeout()`
  - [x] Convert `RequiredAcks` to `kgo.RequiredAcks()` with `kgo.AllISRAcks()`, `kgo.LeaderAck()`, `kgo.NoAck()`
- [x] Update `NewProducer()`:
  - [x] Build `kgo.Opt` slice
  - [x] Use `kgo.DefaultProduceTopic()` for topic
  - [x] Create client with `kgo.NewClient()`
- [x] Update `Disconnect()`:
  - [x] Change from `writer.Close()` to `client.Close()`
- [x] Update `WriteWithHeaders()`:
  - [x] Convert `[]kafka.Header` parameter to `[]Header`
  - [x] Create `kgo.Record` instead of `kafka.Message`
  - [x] Use `client.ProduceSync()` for sync mode
  - [x] Use `client.Produce()` with callback for async mode
- [x] Update `WriteMulti()`:
  - [x] Create slice of `*kgo.Record`
  - [x] Use `client.ProduceSync()` for multiple records
- [x] Update `WriteMultiJson()`:
  - [x] Same changes as `WriteMulti()`

### 5. Admin (admin.go)

- [x] Replace `kafka.Conn` with `kgo.Client` + `kadm.Client`
- [x] Update imports to include `twmb/franz-go/pkg/kadm`
- [x] Update `Admin` struct:
  - [x] Replace `dialer *kafka.Dialer` with `config *adminConfig`
  - [x] Replace `Conn *kafka.Conn` with `client *kgo.Client` + `adminClient *kadm.Client`
- [x] Create `adminConfig` struct for internal configuration
- [x] Update `NewAdmin()`:
  - [x] Build `kgo.Opt` slice for client options
  - [x] Store config for deferred connection
- [x] Update `Connect()`:
  - [x] Create `kgo.Client` from stored options
  - [x] Create `kadm.Client` from `kgo.Client`
- [x] Update `Disconnect()`:
  - [x] Close both clients
- [x] Create `Partition` struct to replace `kafka.Partition`
- [x] Create `int32SliceToIntSlice()` helper
- [x] Update `GetTopics()`:
  - [x] Use `adminClient.Metadata()` instead of `conn.ReadPartitions()`
- [x] Update `ListTopics()`:
  - [x] Use `adminClient.ListTopics()` instead of `conn.ReadPartitions()`
- [x] Update `CreateTopic()`:
  - [x] Use `adminClient.CreateTopics()` instead of `conn.CreateTopics()`
  - [x] Handle response errors from franz-go
- [x] Update `DeleteTopic()`:
  - [x] Use `adminClient.DeleteTopics()` instead of `conn.DeleteTopics()`
  - [x] Handle response errors from franz-go

### 6. Logger (logger.go)

- [x] Update `LogMessageReceived()` to use custom `Message` type
- [x] Update `LogMessageSent()` to use custom `Message` type
- [x] Remove `kafka-go` import

### 7. Tests

#### consumer_test.go
- [x] Update imports to use `twmb/franz-go/pkg/kgo`
- [x] Replace `consumer.Reader` references with `consumer.client`
- [x] Remove `GetConfig()` test assertions (method removed)
- [x] Update `TestRewind()` to check `consumer.config.startFromOldest`
- [x] Update connection tests (Connect now returns error)
- [x] Update isolation level test (no longer checking ReaderConfig directly)

#### logger_test.go
- [x] Update to use custom `Message` type instead of `kafka.Message`
- [x] Update to use custom `Header` type instead of `kafka.Header`
- [x] Remove `kafka-go` import

#### Integration tests (existing infrastructure)
- [ ] Verify tests pass with new implementation
- [ ] Update any kafka-go specific test patterns

### 8. API Changes Summary

#### Breaking Changes
- `Consumer.Connect()` now returns `error` (was void)
- `Consumer.GetConfig()` removed (internal config no longer exposed)
- `Producer.Writer` field removed (use `IsConnected()` instead)
- `Admin.Conn` field removed (use `IsConnected()` instead)
- `Admin.dialer` field removed
- `WriteWithHeaders()` signature changed: `[]kafka.Header` -> `[]Header`

#### New Types
- `Header` struct (replaces `kafka.Header`)
- `Message` struct (replaces `kafka.Message` type alias)
- `Partition` struct (replaces `kafka.Partition` in Admin)

#### Preserved API
- All public methods maintain the same signatures (except noted above)
- All configuration structs unchanged
- All error constants unchanged
- Logger functions unchanged (use new Message type)

## Testing Strategy

1. Run unit tests: `go test -v ./...`
2. Run integration tests: `make test-kafka`
3. Verify all existing functionality works
4. Test edge cases:
   - Connection failures
   - Context cancellation
   - Concurrent access
   - Async vs sync producer modes

## Rollback Plan

If issues arise:
1. Revert go.mod changes
2. Restore original implementation files
3. Run `go mod tidy`
