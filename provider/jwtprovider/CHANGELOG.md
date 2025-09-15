# JWT Provider Changelog

All notable changes to the Blueprint JWT provider will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [v0.8.0]

### Added
- Initial release of JWT provider as independent module
- Complete JWT implementation with multiple signing algorithms
- Support for RS256, ES256, EdDSA, HS256/384/512 algorithms
- Token revocation system with pluggable backends
- User session tracking and session limit enforcement
- Token rotation with secure refresh functionality
- Cryptographically secure JWT IDs with 256 bits of entropy
- In-memory revocation storage backend
- Configuration management with algorithm validation
- Integration with HTTP server authentication middleware
- Sample applications demonstrating JWT usage patterns
- Comprehensive error handling

### Security Features
- Mandatory JWT ID for all tokens enabling revocation support
- Session management with configurable maximum concurrent sessions per user
- Automatic cleanup of expired revocation entries
- Enhanced algorithm support verification
- Reserved claim protection to prevent header injection attacks
- DoS protection with token size limits and parsing timeouts
- Enhanced validation for issuer and audience claims

### Technical Details
- JWT implementation using golang-jwt/jwt/v5
- Support for multiple key types (RSA, ECDSA, EdDSA, HMAC)
- Thread-safe token revocation management
- Configurable token expiration and refresh intervals
- Memory-efficient session tracking
- Graceful shutdown handling

### Dependencies
- Compatible with Blueprint core framework v0.8.0+
- Optional integration with HTTP server provider for authentication

### Migration Notes
- Moved from core session package to dedicated provider module
- Enhanced JWT functionality with revocation and session management
- No breaking changes from previous Blueprint versions
- All existing imports continue to work unchanged