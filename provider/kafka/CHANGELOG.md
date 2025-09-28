# Kafka Provider Changelog

All notable changes to the Blueprint Kafka provider will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [v0.8.1]

### Fixed
- **Critical race condition in Consumer.Disconnect()**: Fixed nil pointer dereference panic that occurred when `Disconnect()` was called while `Subscribe()` was actively consuming messages
- Added proper synchronization using mutex and WaitGroup to ensure thread-safe access to the Kafka reader
- `Disconnect()` now waits for all active subscription goroutines to complete before setting reader to nil
- All subscription methods (`Subscribe()`, `ChannelSubscribe()`, `SubscribeWithOffsets()`) now capture reader reference to prevent access to nil pointer
- Improved error handling for closed reader scenarios with `io.ErrClosedPipe` detection

### Changed
- Consumer struct now includes `subscribeMutex` and `activeReaders` fields for synchronization
- All reader access methods now use mutex protection for thread safety
- `Disconnect()` provides graceful shutdown with proper logging of shutdown phases

## [v0.8.0]

### Added
- Initial release of Kafka provider as independent module
- Message streaming and processing capabilities
- Producer and consumer functionality
- Configuration management
- Integration tests with testcontainers
- Comprehensive error handling

### Technical Details
- Full Kafka client implementation
- Support for message serialization/deserialization
- Connection pooling and retry mechanisms
- Graceful shutdown handling

### Dependencies
- Compatible with Blueprint core framework v0.8.0+
- Requires Kafka broker version 2.0+

### Migration Notes
- No breaking changes from previous Blueprint versions
- All existing imports continue to work unchanged