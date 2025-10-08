# HTTP Server Provider Changelog

All notable changes to the Blueprint HTTP Server provider will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [v0.8.3]

### Added
- `Validator` interface for custom validation logic on request structs
- `NewFieldError` helper function for creating field-specific validation errors
- Two-stage validation system: binding validation followed by recursive custom validation
- Support for nested struct validation with full field path reporting (e.g., `address.zip_code`)
- Support for validating slice, array, and map elements
- Comprehensive validation test suite with 1000+ lines covering all validation scenarios

### Changed
- Refactored `ValidateJSON` to support recursive validation of nested structures
- Refactored `ValidateQuery` to support two-stage validation (binding + custom) with recursive nested validation
- Improved validation error handling with better field path context
- Moved from global validator instance to Gin's binding validator
- Enhanced error messages for nested validation failures


## [v0.8.2]

### Added
- TrustedProxies config parameter
- ServerConfig GetUrl() helper

### Changed 
- Fingerprint ip detection changed from custom function to gin-tonic function

## [v0.8.1]

### Added
- Marshaller interface for httpserver.session
- Updated documentation

## [v0.8.0]

### Added
- Initial release of HTTP Server provider as independent module
- Complete HTTP server implementation with security features
- Unified authentication middleware (JWT, HMAC, session)
- mTLS support with client certificate validation
- CORS middleware with configurable policies
- CSRF protection with enhanced token handling
- Rate limiting for DoS protection
- Browser fingerprinting middleware
- Security headers middleware
- Request logging and metrics integration
- Configuration management with TLS/SSL
- Sample applications with authentication examples
- Comprehensive error handling

### Technical Details
- HTTP server implementation with graceful shutdown
- Middleware chain architecture
- Session management with multiple backends
- Certificate-based authentication
- Connection pooling and keep-alive support
- Request timeout and size limit handling

### Dependencies
- Compatible with Blueprint core framework v0.8.0+
- Optional integration with JWT, HMAC, and session providers

### Migration Notes
- Enhanced security response patterns
- Improved middleware integration
- No breaking changes from previous Blueprint versions
- All existing imports continue to work unchanged