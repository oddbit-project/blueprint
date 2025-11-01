# ETCD Provider Changelog

All notable changes to the Blueprint ETCD provider will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [v0.8.3]

### Changed
- Changed `Lease()` signature to require explicit context


## [v0.8.2]

### Added
- Helper functions for encryption and decryption

## [v0.8.1]

### Added
- Made configuration timeouts explicitly in seconds; 

## [v0.8.0]

### Added
- Initial release of ETCD provider as independent module
- Distributed coordination and configuration management
- Complete etcd v3 client implementation with TLS support
- Distributed locking mechanism with lease management
- Automatic lease renewal for long-running locks
- Configuration storage and retrieval
- Watch functionality for real-time updates
- Transaction support for atomic operations
- Configuration management with secure connections
- Integration tests with testcontainers
- Sample application demonstrating usage patterns
- Comprehensive error handling

### Technical Details
- etcd v3 client implementation
- Support for clustering and high availability
- Lease-based distributed locking
- Watch streams for configuration changes
- Connection health monitoring
- Graceful shutdown handling

### Dependencies
- Compatible with Blueprint core framework v0.8.0+
- Requires etcd cluster version 3.4+

### Migration Notes
- No breaking changes from previous Blueprint versions
- All existing imports continue to work unchanged