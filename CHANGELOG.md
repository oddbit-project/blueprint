# Changelog

All notable changes between `main` and `develop` branches.

## [Unreleased] - 2025-07-07

### Major Features

#### JWT Provider System
- **NEW**: Complete JWT authentication and authorization system (`provider/jwtprovider/`)
  - Support for symmetric (HMAC) and asymmetric (RSA, ECDSA, EdDSA) algorithms
  - JWT token generation, validation, and parsing
  - Configurable expiration, issuer, and audience validation
  - Advanced key management with file and environment variable support
  - Comprehensive test coverage (1000+ lines of tests)

#### Token Revocation System
- **NEW**: Advanced JWT token revocation capabilities
  - Memory-based revocation backend with automatic cleanup
  - Individual token revocation and bulk user token revocation
  - Expiration-based cleanup with configurable intervals
  - Thread-safe operations with proper synchronization

#### HTTP Server Authentication
- **NEW**: JWT-based authentication middleware (`provider/httpserver/auth/jwt.go`)
  - Seamless integration with Gin framework
  - Claims extraction and context injection
  - Authorization header parsing with Bearer token support

#### Device Fingerprinting
- **NEW**: Browser fingerprinting system (`provider/httpserver/fingerprint/`)
  - Multiple configuration modes (Default, Strict, Privacy-friendly)
  - Device identification using User-Agent, Accept headers, IP, timezone
  - Fingerprint comparison with strict/non-strict matching
  - Change detection for security monitoring
  - IP subnet calculation for privacy protection

#### HTPasswd Authentication
- **NEW**: Complete HTPasswd authentication provider (`provider/htpasswd/`)
  - Support for multiple hash formats (bcrypt, SHA, MD5, plain text)
  - User authentication and password verification
  - Integration with existing authentication systems

### Core Improvements

#### Session Management Overhaul
- **BREAKING**: Complete session system refactoring
  - Separated JWT functionality into dedicated `jwtprovider` package
  - Cookie-based sessions with configurable backends (memory, Redis)
  - Flash message support for one-time notifications
  - Session regeneration for security (prevents fixation attacks)
  - Automatic cleanup with TTL management
  - Type-safe session data accessors

#### Credentials System Enhancement
- **BREAKING**: Major credentials interface refactoring
  - New `CredentialConfig` interface for unified credential management
  - Enhanced TLS credential handling with better validation
  - Improved error handling and validation
  - Extended test coverage for credential scenarios

#### Security Enhancements
- Enhanced CSRF protection with better token generation
- Improved rate limiting with configurable burst limits
- Security headers middleware with comprehensive defaults
- Input validation and sanitization improvements

### Documentation

#### New Documentation
- **NEW**: Comprehensive JWT authentication guide (`docs/httpserver/auth.md`)
- **NEW**: HTTP server security documentation (`docs/httpserver/security.md`)
- **NEW**: HTPasswd provider documentation (`docs/provider/htpasswd.md`)
- **NEW**: Secure credentials guide (`docs/crypt/secure-credentials.md`)

#### Updated Documentation
- **UPDATED**: Session management documentation (removed JWT references, focused on cookies)
- **UPDATED**: TLS provider documentation with enhanced examples
- **UPDATED**: Main documentation index with new features

### 🛠 Development & Samples

#### New Sample Applications
- **NEW**: JWT provider example (`samples/jwtprovider_example/`)
  - Complete JWT authentication demo
  - Token generation, validation, refresh, and logout
  - Visit tracking with stateful sessions
  - Comprehensive README with usage examples

- **NEW**: HTPasswd authentication sample (`samples/htpasswd/`)
  - User authentication with multiple hash formats
  - Integration example with HTTP server

- **NEW**: ClickHouse migrations sample (`samples/ch-migrations/`)
  - Database migration management example

#### Sample Reorganization
- **BREAKING**: Moved all samples from `sample/` to `samples/` directory
- Updated import paths and configuration in all sample applications
- Enhanced sample applications with better error handling

### Bug Fixes

#### Critical Fixes
- **FIXED**: Data race in JWT revocation tests
  - Added thread-safe test helper methods
  - Proper mutex synchronization for concurrent access
  - Eliminated race conditions in memory backend tests

#### General Fixes
- **FIXED**: Kafka consumer EOF handling improvements
- **FIXED**: NATS integration test stability
- **FIXED**: Session middleware cookie handling edge cases
- **FIXED**: TLS certificate validation in test scenarios

### ⚡ Performance & Quality

#### Testing Improvements
- Added comprehensive race condition testing
- Enhanced test coverage across all new components
- Improved integration test reliability
- Added property-based testing for JWT scenarios

#### Code Quality
- Enhanced error handling with specific error types
- Improved logging and debugging capabilities
- Better separation of concerns in session management
- Consistent API patterns across providers

### Breaking Changes

#### Session System
- **BREAKING**: JWT functionality moved from `provider/httpserver/session` to `provider/jwtprovider`
- **BREAKING**: Session middleware API changes for cookie-based sessions
- **BREAKING**: Removed JWT-related methods from session package

#### Credentials System
- **BREAKING**: `CredentialConfig` interface changes require updates to custom implementations
- **BREAKING**: TLS credential loading methods have new signatures

#### Directory Structure
- **BREAKING**: Sample applications moved from `sample/` to `samples/`
- **BREAKING**: Import path changes for sample applications

### Dependencies

#### New Dependencies
- Enhanced JWT library support for additional algorithms
- Improved cryptographic libraries for secure operations

#### Removed Dependencies
- Cleaned up unused dependencies from session refactoring
- Removed deprecated authentication libraries

### Security

#### Enhancements
- Improved token validation with stricter checks
- Enhanced session security with configurable cookie attributes
- Better protection against timing attacks in authentication
- Strengthened random number generation for session IDs

#### Vulnerability Fixes
- Fixed potential timing vulnerabilities in password comparison
- Enhanced protection against session fixation attacks
- Improved CSRF token validation

### Statistics

- **Files Changed**: 88 files
- **Lines Added**: ~10,500
- **Lines Removed**: ~1,800
- **New Tests**: 15+ new test files
- **New Documentation**: 5 new documentation files
- **New Samples**: 3 new sample applications

### Migration Guide

#### For JWT Users
1. Update imports from `provider/httpserver/session` to `provider/jwtprovider`
2. Replace `session.NewJWTConfig()` with `jwtprovider.NewJWTConfig()`
3. Update authentication middleware usage

#### For Session Users
1. Review cookie-based session configuration
2. Update session middleware initialization
3. Migrate JWT sessions to new JWT provider if needed

#### For Sample Users
1. Update import paths from `sample/` to `samples/`
2. Review configuration changes in sample applications

---
