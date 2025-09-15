# HTPasswd Provider Changelog

All notable changes to the Blueprint HTPasswd provider will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [v0.8.0]

### Added
- Initial release of HTPasswd provider as independent module
- HTTP basic authentication support
- HTPasswd file format compatibility
- Multiple password hashing algorithms (bcrypt, MD5, SHA1)
- User management operations (add, update, delete)
- File-based credential storage
- Integration with HTTP server authentication middleware
- Configuration management for auth files
- Password validation and verification
- Secure credential handling
- Sample applications demonstrating usage
- Comprehensive error handling

### Technical Details
- HTPasswd file parser and generator
- Support for Apache-compatible password formats
- Thread-safe user management operations
- File monitoring for credential updates
- Memory caching for performance
- Graceful shutdown handling

### Dependencies
- Compatible with Blueprint core framework v0.8.0+
- Optional integration with HTTP server provider for authentication

### Migration Notes
- No breaking changes from previous Blueprint versions
- All existing imports continue to work unchanged