# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

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
