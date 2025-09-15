# HMAC Provider Changelog

All notable changes to the Blueprint HMAC provider will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [v0.8.0]

### Added
- Initial release of HMAC provider as independent module
- Complete HMAC-SHA256 authentication system
- Replay attack protection with nonce management
- Dual signature methods: basic SHA256Sign/Verify and advanced Sign256/Verify256
- Multiple nonce storage backends (memory, Redis, generic key-value)
- Configurable timestamp validation windows
- Input size limits for DoS protection
- Automatic nonce expiration and cleanup
- Multiple key support for enhanced security
- HTTP authentication middleware integration
- Python client library compatibility
- Configuration management with secure key storage
- Sample applications with cross-language examples
- Comprehensive error handling

### Technical Details
- HMAC-SHA256 implementation with timing attack resistance
- UUID-based nonce generation with TTL
- Constant-time comparison operations
- Key provider interface for flexible key management
- Connection health monitoring for storage backends
- Graceful shutdown handling

### Dependencies
- Compatible with Blueprint core framework v0.8.0+
- Optional Redis integration for distributed nonce storage

### Security Features
- Replay attack protection using UUID-based nonces
- Timing attack resistance through constant-time comparisons
- Input validation and size limits
- Secure credential storage with encrypted keys
- DoS protection with fail-safe error handling

### Migration Notes
- Enhanced key provider interface with keyId support
- Improved integration tests and documentation
- No breaking changes from previous Blueprint versions
- All existing imports continue to work unchanged