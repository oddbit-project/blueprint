# SQLite Provider Changelog

All notable changes to the Blueprint SQLite provider will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [v0.8.0]

### Added
- Initial release of SQLite provider as independent module
- Pure Go (non-CGO) SQLite driver via `modernc.org/sqlite`
- Repository pattern integration with the Blueprint `db` package
- Database migration system with schema versioning (module-aware)
- Connection pool configuration (max open/idle connections, lifetimes)
- Integration tests for client and migration flows

### Technical Details
- Driver name: `sqlite`
- Uses file-based DSNs (or `:memory:`) supported by `modernc.org/sqlite`
- Registers goqu + `db/qb` dialects under the `sqlite` driver name

### Dependencies
- Compatible with Blueprint core framework v0.8.0+
- Requires no C toolchain (pure Go driver)
