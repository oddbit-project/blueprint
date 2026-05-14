# PostgreSQL Provider Changelog

All notable changes to the Blueprint PostgreSQL provider will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [v0.8.1]

### Security

- Upgraded Go from 1.23.0 to 1.26.3, fixing 15 stdlib vulnerabilities.
- Upgraded `github.com/jackc/pgx/v5` from v5.7.5 to v5.9.2.
- Upgraded `go.opentelemetry.io/otel/sdk` to v1.43.0, fixing PATH hijacking (CVE-2026-24051, CVE-2026-39883).
- Upgraded `go.opentelemetry.io/otel` to v1.43.0, fixing baggage header DoS (CVE-2026-29181).

## [v0.8.0]

### Added
- Initial release of PostgreSQL provider as independent module
- Complete PostgreSQL database operations support
- Repository pattern implementation with generic types
- Database migration system with schema versioning
- Connection pooling and transaction management
- Query builder integration for complex queries
- Field metadata mapping with struct tag support
- Batch processing capabilities
- Configuration management with SSL support
- Integration tests with testcontainers
- Comprehensive error handling

### Technical Details
- PostgreSQL driver implementation (pgx/pq)
- Support for prepared statements and stored procedures
- Connection pool management with health checks
- Transaction isolation level control
- Migration system with rollback support
- Graceful shutdown handling

### Dependencies
- Compatible with Blueprint core framework v0.8.0+
- Requires PostgreSQL server version 12+

### Migration Notes
- Enhanced ALTER TABLE handling for better compatibility
- Improved DEFAULT value separation in migrations
- No breaking changes from previous Blueprint versions
- All existing imports continue to work unchanged