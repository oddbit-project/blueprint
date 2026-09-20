# PostgreSQL Provider Changelog

All notable changes to the Blueprint PostgreSQL provider will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [v0.8.2] - 2026-09-20

Requires Blueprint core v0.10.0.

### Fixed

- `MigrationExists()` fetched a single record through a multi-row fetch, so it failed on every call with `expected slice but got struct`, which made `RunMigration()` and `RegisterMigration()` unusable. It now fetches a single record, and looks the migration up by name, so an already-applied migration is reported as such and one recorded under the same name with different contents returns `ErrMigrationNameHashMismatch`.
- `Run()` no longer ignores the error from listing applied migrations; previously a failed lookup left the applied set empty and re-ran every migration.
- `updateTable()` used `UPDATE TABLE ... SET module = ?`, which is not valid PostgreSQL and used an unsupported placeholder, and swallowed the resulting error; upgrading a pre-module migration table left `module` unset, so every migration re-ran. The backfill is now correct and runs unconditionally (`WHERE module IS NULL`), so a table left half-upgraded by an earlier version is repaired on the next run.
- The `ALTER TABLE ... ADD COLUMN module` statement was issued through `QueryRowContext` and never scanned, holding the connection open; it now uses `ExecContext`.
- A migration that ran but could not be registered now returns `ErrRegisterMigration` wrapping the cause, as documented; previously the raw repository error was returned and the documented error was never used.
- The migration advisory lock now waits under the caller's context instead of `context.Background()`, so a cancelled or expired context aborts the wait; the lock is released with an uncancellable context, so a cancelled run still frees it.

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