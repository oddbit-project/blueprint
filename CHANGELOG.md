# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [v0.5.3]

### Added

- **SMTP Provider** (provider/smtp) - Complete SMTP email functionality
  - Full SMTP client implementation with TLS support
  - Template-based email composition with HTML and plain text support
  - Attachment handling for email messages
  - Comprehensive test suite with unit tests

- **Session Authentication System** - Complete session-based authentication with multiple providers
  - New session authentication middleware (provider/httpserver/auth/session.go)
  - Session data management with enhanced security features
  - Token list management for session tracking
  - Integration with existing HMAC and JWT providers

- **Enhanced Configuration System**
  - Environment variable provider with struct scanning capabilities
  - Better parsing of configuration keys and default values
  - Improved JSON configuration provider with enhanced validation

- **AES256-GCM Encryption** - New constant-time encryption implementation
  - Secure AES256-GCM encryption with constant timing
  - Comprehensive test suite with benchmark tests
  - Enhanced cryptographic security for sensitive data

- **Sample Applications**
  - HTTP server with session authentication (samples/httpserver-session)
  - Updated HMAC provider examples with improved documentation
  - Enhanced Python HMAC client examples

### Enhanced

- **HMAC Provider Improvements**
  - Multiple key support for enhanced security
  - Renamed userId to keyId for better clarity
  - Enhanced key provider interface
  - Improved integration tests and documentation

- **Migration Manager** - Module-aware migration system
  - Enhanced migration manager with better module support
  - Improved integration test infrastructure across all providers
  - Better error handling and validation in migration processes
  - Expanded test coverage for ClickHouse and PostgreSQL migrations

- **HTTP Server Authentication**
  - Unified authentication middleware supporting JWT, HMAC, and session providers
  - Enhanced token management and validation
  - Improved security response patterns and error handling
  - Better fingerprinting middleware integration

- **Testing Infrastructure**
  - New Docker-based test containers for ClickHouse, Kafka, MQTT, NATS, and PostgreSQL
  - Enhanced Makefile with comprehensive test targets
  - Improved integration test organization and reliability
  - Better test isolation and cleanup procedures

- **Documentation**
  - Comprehensive updates to HMAC provider documentation
  - Enhanced HTTP server authentication and session documentation
  - Updated configuration system documentation
  - Improved sample application documentation

### Fixed

- Integration test stability improvements across all providers
- Enhanced error handling in database repository operations
- Better cleanup procedures in test suites
- Improved migration validation and error reporting
- Various dependency updates for security and compatibility

### Security

- **AES256-GCM Implementation**
  - Constant-time encryption to prevent timing attacks
  - Secure key derivation and nonce generation
  - Enhanced cryptographic security for credential storage

- **Session Security**
  - Enhanced session data protection
  - Improved session token management
  - Better session expiration and cleanup mechanisms

- **Multi-Provider Authentication**
  - Unified security model across HMAC, JWT, and session providers
  - Enhanced validation and error handling
  - Improved DoS protection and input validation

## [v0.5.2]

### Added
 - Dummy release to appease goproxy :)

## [v0.5.1]

### Added

- **HMAC Provider** (provider/hmacprovider) - Complete HMAC-SHA256 authentication system with replay attack protection
  - Dual signature methods: basic SHA256Sign/Verify and advanced Sign256/Verify256 with nonce/timestamp
  - Multiple nonce storage backends: memory, Redis, and generic key-value stores
  - Configurable timestamp validation windows and input size limits
  - Automatic nonce expiration and cleanup with configurable eviction policies
  - DoS protection with input size limits and fail-safe error handling
- **HMAC Authentication Middleware** (provider/httpserver/auth/hmac.go) for HTTP request authentication
- **Python HMAC Client Library** (samples/hmac-python-client) with full Blueprint compatibility
  - Complete Python client implementation with context manager support
  - Cross-language HMAC signature compatibility
  - Comprehensive test suite with unit and integration tests
  - Example usage and detailed documentation
- **JWT Provider Enhancements**
  - Token revocation system with pluggable backends
  - User session tracking and session limit enforcement
  - Token rotation with secure refresh functionality
  - Enhanced security: DoS protection, token size limits, parsing timeouts
  - Cryptographically secure JWT IDs with 256 bits of entropy
- **Sample Applications**
  - HTTP server with HMAC authentication (samples/httpserver-hmacprovider)
  - JWT user session tracking example (samples/jwtprovider-user-tracking)
  - Python HMAC client demonstration server
- **Security & DevOps**
  - SBOM (Software Bill of Materials) generation with Trivy security scanning
  - Enhanced logging with proper stack trace reporting
  - Comprehensive documentation for HMAC and JWT providers

### Enhanced

- **JWT Provider Security Features**
  - Mandatory JWT ID for all tokens enabling revocation support
  - Session management with configurable maximum concurrent sessions per user
  - Automatic cleanup of expired revocation entries
  - Enhanced algorithm support verification (HS256/384/512, RS256/384/512, ES256/384/512, EdDSA)
  - Reserved claim protection to prevent header injection attacks
- **Logger Improvements**
  - Fixed stack trace reporting to point to actual relevant code lines
  - Enhanced error context and debugging capabilities
- **HTTP Server Authentication**
  - Unified authentication middleware supporting both JWT and HMAC providers
  - Improved error handling and security response patterns

### Fixed

- JWT provider test failures related to token revocation manager setup
- NATS unit test intermittent failures in integration testing
- Various dependency vulnerabilities through updates
- Logger stack trace accuracy issues
- Memory management and cleanup in JWT revocation system

### Security

- **HMAC Provider Security**
  - Replay attack protection using UUID-based nonces with TTL
  - Timing attack resistance through constant-time comparisons
  - Input validation and size limits to prevent DoS attacks
  - Secure credential storage with encrypted secret keys
- **JWT Provider Security**
  - Token revocation capability to invalidate compromised tokens
  - Session limit enforcement to prevent token accumulation attacks
  - Enhanced validation for issuer and audience claims
  - DoS protection with token size and parsing timeout limits
- **Cross-Language Security**
  - Python client library with same security standards as Go implementation
  - Verified compatibility and security parity between language implementations

## [v0.5.0]

### Added

- New SQL Query Builder (db/qb package) with support for INSERT and UPDATE with complex WHERE conditions
- JWT Provider (provider/jwtprovider) with multiple signing algorithms (RS256, ES256, EdDSA)
- Token revocation system with in-memory storage for enhanced security
- Field metadata mapping system for database operations with struct tag support
- HTPasswd authentication provider for basic HTTP authentication
- Browser fingerprinting middleware for enhanced session security
- Enhanced CSRF protection with improved token handling
- Rate limiting enhancements for HTTP server security
- Generic Map implementation in types/collections package
- Comprehensive documentation for database operations
- HTTP server security and authentication documentation
- Provider-specific documentation (HTPasswd, TLS, secure credentials)

### Changed

- **Breaking:** Session management architecture - JWT functionality moved from session package to dedicated jwtprovider package
- **Breaking:** Credentials system interface changes affecting CredentialConfig implementations
- **Breaking:** Session store architecture updated with new middleware interfaces
- Database Repository pattern enhanced with improved error handling and integration with new SQL query builder
- Project structure reorganized - samples moved from `sample/` to `samples/` directory
- Session middleware interfaces updated for better modularity
- Database operation interfaces improved with batch processing capabilities

### Deprecated

- Old JWT implementation in session package (replaced by dedicated jwtprovider package)
- Legacy session JWT integration methods (use new jwtprovider with session middleware)

### Fixed

- Data race conditions in JWT provider tests
- EdDSA key handling issues in JWT provider
- Various database operation edge cases and error handling
- Kafka EOF handling improvements
- Session management stability issues
- Database connection handling in repository pattern

### Security

- Token revocation system implementation for JWT security
- Browser fingerprinting for enhanced session validation
- Secure credential storage improvements with better encryption handling
- CSRF token generation and validation enhancements
- Enhanced authentication flows with improved security headers
- Rate limiting improvements to prevent abuse
