# Kafka Provider Changelog

All notable changes to the Blueprint Kafka provider will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [v0.8.2]

### Added
- **Enhanced parameter validation**: Added nil checks for context, handler functions, and channels in all Consumer subscription methods
- **New error constants**: Added `ErrNilHandler`, `ErrNilChannel`, and `ErrNilContext` for better error reporting
- **Improved error handling**: Enhanced error logging with structured context information for better debugging
- **New test coverage**: Added `TestIsolationLevelConfiguration` unit test for isolation level validation
- **New test coverage**: Added `TestAdminListTopicsDeduplication` integration test for multi-partition topic handling

### Changed
- **Code refactoring for improved maintainability**: Extracted duplicate authentication logic into shared `createSASLMechanism()` function
- **Simplified credential management**: Created shared `setupCredentials()` function to eliminate duplication across Consumer, Producer, and Admin components
- **Enhanced unit tests**: Updated test suite to work with existing implementation without requiring additional functionality
- **Reduced code duplication**: Consolidated authentication and credential setup patterns, reducing total codebase by ~36 lines
- **Improved reliability**: Fixed `WatchPartitionChanges` configuration handling to properly respect the configured value
- **Better channel handling**: Added graceful context cancellation handling in `ChannelSubscribe` to prevent blocking on channel sends

### Fixed
- **Memory leak in Admin.NewAdmin()**: Fixed missing `credential.Clear()` call that left sensitive password data in memory (admin.go:70)
- **Duplicate topics in Admin.ListTopics()**: Fixed topic deduplication issue where topics with multiple partitions appeared multiple times in the list; now uses map-based deduplication (admin.go:127-134)
- **Incomplete isolation level handling**: Fixed consumer configuration to properly handle "committed" isolation level; previously only "uncommitted" was handled (consumer.go:170-171)
- **Inconsistent error logging in Producer.WriteMulti()**: Added error logging to match the logging behavior of other producer methods (producer.go:213)
- **Offset commit error handling**: Fixed `SubscribeWithOffsets` to properly handle and log commit errors instead of silently ignoring them
- **Thread safety improvements**: Enhanced `ReadMessage` method with proper reader lifecycle management using WaitGroup
- **Graceful shutdown**: Improved error detection for closed readers across all subscription methods

### Technical Improvements
- Authentication logic centralized in `kafka.go` with single source of truth for SASL mechanism creation
- Credential setup and cleanup unified across all Kafka components
- Test coverage maintained while removing dependencies on non-existent methods and fields
- Improved code organization with better separation of concerns
- Enhanced error logging with structured key-value pairs for better observability
- More robust parameter validation across all public methods

### Removed
- Duplicate authentication setup code from individual Consumer, Producer, and Admin constructors
- Redundant credential management patterns across components


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