# ClickHouse Provider Changelog

All notable changes to the Blueprint ClickHouse provider will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [Unreleased]

### Added

- `Querier.CheckRecord` (implements `dbx.RecordChecker`) and `ErrRecordMapping`: rejects record
  types whose `db`-derived columns clickhouse-go's `ch`-tag mapper would map to a different field or
  to none, naming each column, field and reason; passing results are cached per type.

### Breaking changes

- **`dbx.NewRepository` over a ClickHouse `Querier` fails with `ErrRecordMapping`** for record
  types whose `ch` tags don't match their `db` columns (they used to scan into the wrong fields or
  fail at query time), and for an embedded non-struct field, on which clickhouse-go's mapper panics.
  `dbx.Query[D]`/`QueryOne[D]` run the same check on `D`.
- **Bound `time.Local` and `time.FixedZone` values are sent as the same instant in UTC.**
  clickhouse-go rendered a `Local` time as a numeric string that ClickHouse misread or rejected for
  almost every date (comparisons with `time.Now()` values failed or matched nothing), and a fixed
  zone's name (e.g. `UTC+5`) is not a zone ClickHouse can load. Times in an IANA zone keep their
  zone, so `Date` comparisons are unchanged for them. A `Local` time compared with a `Date` column
  now uses its UTC calendar day.

### Changed

- Bumped `gohan` to v0.3.0.

### Fixed

- Bound `time.Time` arguments (dbx statements) after 2262 failed with `Decimal math overflow`:
  every time was bound at scale 9. Times are now bound at the smallest exact scale (3, 6 or 9), so
  millisecond/microsecond times bind across `DateTime64(3)`'s 1900-2299; nanosecond times still
  overflow after 2262. clickhouse-go v2.40.3 itself mishandles `DateTime64` values after 2262 in
  columnar inserts and when scanning into `time.Time`; see `docs/db/dbx.md`.

## [v0.9.1] - 2026-09-28

Requires Blueprint core v0.11.0 and `gohan` v0.2.0. gohan v0.2.0 also makes `dbx.Repository.Delete`/`Update`
reject a column-free condition such as `gohan.Raw("1=1")` with `gohan.ErrNoWhere`.

### Fixed

- **`dbx.Querier` truncated time arguments to whole seconds** (#89). Passing a `time.Time`
  through `Exec`/`Get`/`Select`/`QueryInt64` used clickhouse-go's native positional binding, which
  formats it as `toDateTime('YYYY-MM-DD hh:mm:ss')` regardless of the target column's scale, so a
  time-based `Count`/`Exists`/`Get`/`List`/`Delete` condition (and a `Repository.Exec` insert)
  silently dropped sub-second precision. `InsertBatch` was not affected (it writes columns
  directly).

### Changed

- **`Querier.Dialect()` now returns `gohan.ClickHouseNamed()`** instead of `gohan.ClickHouse()`,
  so every statement `dbx.Repository` builds through it uses `@pN` named placeholders. A
  `time.Time` argument (bare, `*time.Time`, or a `driver.Valuer` producing one, such as
  `sql.NullTime`) is now bound with `clickhouse.DateNamed(..., clickhouse.NanoSeconds)` and keeps
  full precision. This is a behaviour change from the previous (broken) whole-second binding:
  comparisons are now **exact-instant** — a `DateTime64(3)` column storing `.123` is not equal to
  a bound value of `.123456789` (it equals that value truncated to milliseconds). Callers who pass
  their own hand-written SQL with positional `?` placeholders are unaffected and keep
  clickhouse-go's whole-second time binding; use `@name` with `clickhouse.DateNamed` directly if
  full precision is needed there.

## [v0.9.0] - 2026-09-27

Requires Blueprint core v0.11.0.

> **Breaking release**: see "Breaking changes" below and in the root
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
  driver doesn't use). Requires Blueprint core v0.11.0, which contains `dbx` and depends on `gohan` v0.1.0.

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
