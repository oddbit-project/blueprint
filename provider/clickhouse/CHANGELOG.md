# ClickHouse Provider Changelog

All notable changes to the Blueprint ClickHouse provider will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [Unreleased]

> **Potentially breaking release**: see "Breaking changes" below and in the root
> [CHANGELOG](../../CHANGELOG.md).

### Breaking changes

- **Backslashes in values are escaped.** ClickHouse treats `\` as an escape character in string
  literals. The goqu dialect now sends it literally, so a value such as `a\nb` is stored as those
  four characters instead of `a`, a newline, and `b`. Rows written by earlier versions keep what
  ClickHouse stored at the time.
- **Column names are validated**: `FetchRecord`, `FetchByKey`, `FetchWhere`, `Exists`,
  `CountWhere`, `DeleteWhere` and `DeleteByKey` return `db.ErrInvalidIdentifier` for column names
  containing `"`, `\` or NUL.
- **`DeleteWhere` refuses an empty map** with `ErrInvalidParameters`.
- `Exists` returns `ErrInvalidParameters` (instead of panicking) when the skip column is not a
  string.

### Fixed

- **`Count`, `CountWhere` and `Exists` failed on every call**: they scanned ClickHouse's `UInt64`
  `COUNT(*)` into a signed integer, which clickhouse-go rejects. They now scan into `uint64`.

### Added

- **`Client.Querier()` / `NewQuerier(conn)`** — returns a `*clickhouse.Querier` implementing
  `dbx.Querier` and `dbx.BatchInserter` directly over the client's native ClickHouse connection,
  so `dbx.Repository[T]` can run against ClickHouse without `database/sql` (which ClickHouse's
  driver doesn't use). **Requires the core Blueprint release that contains `dbx`/`gohan`** — do
  not tag this provider release before `gohan` is tagged, core is tagged, and this module's
  `go.mod` is bumped to that core version; see
  [docs/release-process.md](../../docs/release-process.md#gohandbx-release-order).

## [v0.8.4] - 2026-09-20

### Security

- **golang.org/x/crypto**: upgraded from v0.53.0 to v0.57.0, fixing an authentication bypass in `golang.org/x/crypto/ssh` where source-address restrictions on an authorized key were not enforced (CVE-2026-56854).
- Requires Blueprint core v0.10.2, which carries the same upgrades.

## [v0.8.3] - 2026-09-20

Requires Blueprint core v0.10.0.

### Fixed

- `MigrationExists()` fetched a single record through a multi-row fetch, so it failed on every call with `expected slice but got struct`, which made `RunMigration()` and `RegisterMigration()` unusable. It now fetches a single record, and looks the migration up by name, so an already-applied migration is reported as such and one recorded under the same name with different contents returns `ErrMigrationNameHashMismatch`.
- `Run()` no longer ignores the error from listing applied migrations; previously a failed lookup left the applied set empty and re-ran every migration.
- `updateTable()` copied rows from a pre-module migration table with an empty `module`, which `List()` filters out, so every historical migration re-ran after the upgrade; the rows are now copied as the `base` module. This defect has been present since module support shipped (v0.8.0), so an installation already upgraded by an earlier version is still affected and has to be repaired by hand -- `TinyLog` supports no updates, so the table is rewritten; see [Repairing a table upgraded before this fix](../../docs/db/migrations.md#repairing-a-table-upgraded-before-this-fix).
- A migration that ran but could not be registered now returns `ErrRegisterMigration` wrapping the cause, as documented; previously the raw repository error was returned and the documented error was never used.

## [v0.8.2]

### Security

- Upgraded Go from 1.24.0 to 1.26.3, fixing 15 stdlib vulnerabilities.
- Upgraded `github.com/jackc/pgx/v5` from v5.7.6 to v5.9.2.
- Upgraded `golang.org/x/net` to v0.54.0, fixing HTTP/2 DoS (GO-2026-4918).
- Upgraded `go.opentelemetry.io/otel/sdk` to v1.43.0, fixing PATH hijacking (CVE-2026-24051, CVE-2026-39883).
- Upgraded `go.opentelemetry.io/otel` to v1.43.0, fixing baggage header DoS (CVE-2026-29181).
- Upgraded `filippo.io/edwards25519` from v1.1.0 to v1.1.1, fixing incorrect `MultiScalarMult` results (CVE-2026-26958).

## [v0.8.1]

### Added
- Added InsertAsync() to clickhouse.Repository interface


## [v0.8.0]

### Added
- Initial release of ClickHouse provider as independent module
- Analytics database operations support
- High-performance columnar data processing
- Repository pattern implementation optimized for analytics
- Database migration system for ClickHouse schemas
- Bulk insert operations for large datasets
- Query optimization for analytical workloads
- Configuration management with cluster support
- Integration tests with testcontainers
- Comprehensive error handling

### Technical Details
- ClickHouse native client implementation
- Support for distributed tables and clusters
- Optimized batch insert operations
- Materialized views and aggregation support
- Connection pool management
- Graceful shutdown handling

### Dependencies
- Compatible with Blueprint core framework v0.8.0+
- Requires ClickHouse server version 21.0+

### Migration Notes
- Enhanced migration reliability for analytical schemas
- Improved error handling in schema operations
- No breaking changes from previous Blueprint versions
- All existing imports continue to work unchanged
