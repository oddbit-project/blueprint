# NATS Provider Changelog

All notable changes to the Blueprint NATS provider will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [v0.8.0]

### Added
- Initial release of NATS provider as independent module
- Lightweight messaging capabilities
- Publisher and subscriber functionality
- JetStream support for persistent messaging
- Configuration management
- Integration tests with testcontainers
- Comprehensive error handling

### Technical Details
- Full NATS client implementation
- Support for request-reply patterns
- Connection pooling and retry mechanisms
- Graceful shutdown handling

### Dependencies
- Compatible with Blueprint core framework v0.8.0+
- Requires NATS server version 2.0+

### Migration Notes
- No breaking changes from previous Blueprint versions
- All existing imports continue to work unchanged