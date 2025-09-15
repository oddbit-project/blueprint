# Redis Provider Changelog

All notable changes to the Blueprint Redis provider will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [v0.8.0]

### Added
- Initial release of Redis provider as independent module
- Caching and key-value storage capabilities
- Full Redis command support (strings, hashes, lists, sets, sorted sets)
- Connection pooling and clustering support
- Pub/Sub messaging functionality
- Lua scripting support
- Configuration management with TLS support
- Integration tests with testcontainers
- Comprehensive error handling

### Technical Details
- Redis client implementation with go-redis
- Support for Redis Sentinel and Cluster modes
- Pipeline and transaction support
- Connection health monitoring
- Graceful shutdown handling

### Dependencies
- Compatible with Blueprint core framework v0.8.0+
- Requires Redis server version 6.0+

### Migration Notes
- No breaking changes from previous Blueprint versions
- All existing imports continue to work unchanged