# SMTP Provider Changelog

All notable changes to the Blueprint SMTP provider will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [v0.8.1]

### Changed
- Updated go-mail to version 0.7.1 to mitigate CVE-2025-59937


## [v0.8.0]

### Added
- Initial release of SMTP provider as independent module
- Complete SMTP email functionality
- Full SMTP client implementation with TLS support
- Template-based email composition
- HTML and plain text email support
- Email attachment handling
- SMTP authentication (PLAIN, LOGIN, CRAM-MD5)
- Configuration management with secure credentials
- Bulk email sending capabilities
- Email queue management
- Comprehensive test suite with unit tests
- Sample applications for email sending
- Comprehensive error handling

### Technical Details
- SMTP client implementation with go-mail
- Support for multiple SMTP servers
- Connection pooling for bulk operations
- Template engine integration
- Attachment encoding and MIME handling
- Graceful shutdown handling

### Dependencies
- Compatible with Blueprint core framework v0.8.0+
- Requires SMTP server for email delivery

### Migration Notes
- No breaking changes from previous Blueprint versions
- All existing imports continue to work unchanged
