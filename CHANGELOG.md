# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## About Module Versioning

Starting with v0.8.0, Blueprint uses independent module versioning. Each provider is a separate Go module with its own
semantic versioning. This changelog tracks:

- **Core Framework Changes**: Changes to the main blueprint module
- **Module Version Updates**: Which provider modules are updated in each release
- **Breaking Changes**: Cross-module compatibility impacts

For detailed changes in specific providers, see the individual CHANGELOG.md files in each provider directory.

## [Unreleased]

## [v0.10.3] - 2026-09-21

### Fixed

- **`threadpool`: `Stop()` no longer discards queued jobs.** It cancelled the
  workers' context and returned, so anything accepted but not yet started was
  dropped with no error and no signal to the caller — who had every reason to
  believe the work was handed over. The documentation already described the
  behaviour this restores ("after they complete their current jobs"). Workers
  now finish the queue before exiting, and run those jobs on a context that is
  still live rather than one already cancelled.

### Added

- **`threadpool.ThreadPool.StopWithContext(ctx)`** — `Stop` with a bound, for a
  shutdown that cannot be open-ended. It drains like `Stop`, and on the
  context's expiry cancels the workers, abandons the queue and returns
  `ctx.Err()` without waiting for a job that ignores its own context.
- **`threadpool.WorkerGroup.Drain()`** — the graceful counterpart to `Stop()`
  at the group level.

### Changed

- **`ThreadPool` guards its worker group with a mutex, and a stop claims it in
  one step.** `Stop` previously cleared the field only after the workers had
  gone, so for the length of a shutdown a dispatch still saw a live pool and was
  accepted into a queue that, on the `StopWithContext` deadline path, nobody
  would read again. `Start`, `Stop`, `StopWithContext` and the accessors now
  agree on one lock. No regression test: with the drain above, a job accepted in
  that window is run anyway, and on the hard-cancel path an accepted job is
  abandoned by contract — so the change is not observable from outside, and a
  test asserting otherwise would be asserting something the contract does not
  promise.

### Documentation

- `docs/threadpool/threadpool.md`: `Stop` semantics, `StopWithContext`, and a
  note that `Dispatch` panics on a stopped pool while `DispatchWithContext`
  reports `ErrPoolNotStarted` — the difference that matters on any path which
  can race a shutdown.

## [v0.10.2] - 2026-09-20

### Security

- **google.golang.org/grpc**: pinned to v1.83.2 instead of v1.84.0. The denial of service via malformed RPC requests
  (CVE-2026-84445) is fixed in 1.82.2 and 1.83.2 but not in the 1.84 line, so v0.10.1 -- which upgraded to v1.84.0 --
  still carried it. Use this release rather than v0.10.1.

### Module Version Updates

Every provider module is patch-released against this core version, so that a consumer pinning a provider alone also
gets the fixed dependencies: `clickhouse` v0.8.4, `etcd` v0.9.1, `franz` v0.8.5, `hmacprovider` v0.8.3, `htpasswd`
v0.8.3, `httpserver` v0.9.4, `jwtprovider` v0.8.3, `kafka` v0.8.4, `metrics` v0.8.2, `mqtt` v0.8.2, `nats` v0.8.3,
`pgsql` v0.8.3, `prometheus` v0.9.3, `redis` v0.8.2, `s3` v0.8.3, `smtp` v0.9.1, `sqlite` v0.8.3.

## [v0.10.1] - 2026-09-20

### Security

- **golang.org/x/crypto**: upgraded from v0.53.0 to v0.57.0, fixing an authentication bypass in
  `golang.org/x/crypto/ssh` where source-address restrictions on an authorized key were not enforced
  (CVE-2026-56854).
- **google.golang.org/grpc**: upgraded from v1.82.1 to v1.84.0, fixing CVE-2026-84304. Pulled in indirectly through
  the etcd client and OpenTelemetry. This does **not** fix CVE-2026-84445, which is not fixed in the 1.84 line; see
  v0.10.2.

Every provider module is upgraded to the same versions; the provider modules are tagged after this release.

## [v0.10.0] - 2026-09-20

### Added

- **`db/migrations`: `Substitute(src, vars)`** wraps any `Source` and replaces `${name}` in each migration as it is read, so a deployment-specific identifier -- an application role, a schema, a tablespace -- can appear in DDL that is otherwise fixed. The recorded `SHA2` stays the template's while `contents` records what actually ran; any unresolved placeholder -- including a name the substitution does not recognise, or one whose value is empty -- is `ErrMissingVar`; `$1` parameters and `$$`-quoted delimiters are untouched. See [Substituted Source](docs/db/migrations.md#substituted-source).

### Fixed

- **Migration managers (`pgsql`, `clickhouse`, `sqlite`)**: `MigrationExists()` failed on every call, making `RunMigration()` and `RegisterMigration()` unusable and `ErrMigrationNameHashMismatch` unreachable; `Run()` ignored the error from listing applied migrations and re-ran every migration when that lookup failed; a migration that ran but could not be registered returned a raw error instead of `ErrRegisterMigration`; and both `pgsql` and `clickhouse` left rows of a pre-module migration table invisible after an upgrade, re-running every historical migration. See the provider changelogs.

### Security

- **Go 1.26.5**: Upgraded from Go 1.26.3, fixing an Encrypted Client Hello privacy leak in `crypto/tls`
  (GO-2026-5856), unescaped input in `net/textproto` errors (GO-2026-5039), quadratic candidate hostname parsing in
  `crypto/x509` (GO-2026-5037), quadratic `WordDecoder.DecodeHeader` in `mime` (GO-2026-5038) and a root escape via
  symlink in `os` (GO-2026-4970). The first three are reachable from provider code paths that perform a TLS handshake
  or verify a certificate.
- **google.golang.org/grpc**: Upgraded from v1.81.1 to v1.82.1, fixing xDS RBAC and HTTP/2 vulnerabilities
  (GHSA-hrxh-6v49-42gf). Pulled in indirectly through the etcd client and OpenTelemetry.
- **golang.org/x/text**: Upgraded from v0.38.0 to v0.40.0, fixing an infinite loop on invalid input (GO-2026-5970).

### Build

- **Provider module dependencies refreshed**: every provider module's `go.mod` had drifted from the graph the
  workspace actually builds; `go mod tidy` brings them in line with core v0.9.0, notably `testcontainers-go` v0.38.0 →
  v0.43.0, `zerolog` v1.34.0 → v1.35.1, `go.step.sm/crypto` v0.73.0 → v0.84.1 and `golang.org/x/crypto` v0.51.0 →
  v0.53.0.
- **`provider/prometheus` builds outside the workspace again**: it required `provider/httpserver` v0.8.5, whose
  `ServerConfig` predates the `ServerName` field the provider reads, so `go build` failed for anyone consuming the
  published module. It now requires core v0.9.0 and `provider/httpserver` v0.9.3.

### Module Version Updates

Provider modules are versioned independently and are tagged after this release, as they depend on it:

- **`provider/pgsql` v0.8.2**, **`provider/sqlite` v0.8.2**, **`provider/clickhouse` v0.8.3**: migration manager fixes
  -- see the fixed entry above and each provider's `CHANGELOG.md`. The ClickHouse release needs a manual repair on an
  installation upgraded by an earlier version; see
  [Repairing a table upgraded before this fix](docs/db/migrations.md#repairing-a-table-upgraded-before-this-fix).
- **`provider/etcd` v0.9.0**: fixes `Lock.TryLock()` reporting a free lock as held; `WithTTL` is no longer needed to
  make it reliable. Requires this release.
- **`provider/prometheus` v0.9.2**: builds outside the workspace again; dependency updates only otherwise.

## [v0.9.0] - 2026-07-27

### Breaking Changes

- **`provider/tls` now honours `TLSInsecureSkipVerify` when no CA, certificate or key is configured.** `TLSConfig()`
  short-circuited to an empty `tls.Config` in that case, so a configuration of `tlsEnable: true` +
  `tlsInsecureSkipVerify: true` with no CA **still verified the server certificate**. That was the bug — self-signed
  servers could not be used without also supplying a CA bundle — but the fix means such a deployment stops verifying
  certificates after upgrading, where it silently verified them before.

  This affects every consumer of `tls.ClientConfig`: `provider/clickhouse`, `provider/etcd`, `provider/franz`,
  `provider/kafka`, `provider/mqtt`, `provider/nats`, `provider/s3` and `provider/smtp`. Audit any deployment that
  sets `tlsInsecureSkipVerify` without a CA; if verification was actually wanted, drop the flag and configure `tlsCa`.

### Fixed

- **Data race in `log`**: `Config.Logger()` wrote the process-wide `zerolog.TimeFieldFormat` and
  `zerolog.CallerSkipFrameCount` as a side effect of building a logger, while every goroutine emitting a log line
  reads them. Building a logger from a `Config` — or calling `Configure()` — while other goroutines were logging was a
  data race, reported by `-race` in the `runner` test suite. Those settings are now applied only by `Configure()`,
  which is documented as startup-only, and the caller skip count uses zerolog's per-logger
  `CallerWithSkipFrameCount` instead of the global. `log.New()` was never affected.

  A logger built from a configuration that is never passed to `Configure()` now formats timestamps with zerolog's
  default layout (RFC3339) rather than the configuration's `TimeFormat`; the caller and level behaviour is unchanged.

- **`caller` field pointed at the logging wrapper**: with `IncludeCaller` enabled, `LogCallerSkipFrames` was one frame
  short, so every entry reported `log/logger.go` instead of the code that called `Info`/`Error`/etc. The default is now
  3. Applications that raised `CallerSkipFrames` to compensate should lower it by one.

### Added

- **`types/duration` package**: a JSON-friendly `duration.Seconds` type (defined `int64` of whole seconds) that serializes as a plain integer, matching the OAuth/OIDC `expires_in` convention. Includes constructors `Minutes`/`Hours`/`Days`/`FromStd`, and `Std()`/`IsPositive()`/`String()` helpers for stdlib interop.

### Build

- **Integration test targets**: `make test-integration`, `test-all`, `test-providers` and `test-db` now pass
  `-tags=integration` (overridable with `INTEGRATION_TAGS`), so the build-tag-gated suites in `db` and `provider/etcd`
  actually run. Adds a `make test-etcd` target.
- **Provider requirements refreshed**: the core module's `require` entries for the provider modules were behind their
  released tags, in one case incompatibly — `provider/prometheus v0.8.0` does not compile against the `httpserver`
  version selected alongside it, so `go build ./...` failed for anyone building the core module outside this
  repository's workspace. Every provider is now required at its latest release, notably `provider/httpserver` v0.8.5 →
  v0.9.3 and `provider/prometheus` v0.8.0 → v0.9.1.
- **Dependencies tidied**: `go.mod`/`go.sum` had drifted from the module's actual imports. Tidying picks up the
  versions the module graph already selects, notably `testcontainers-go` v0.38.0 → v0.43.0 (matching the version the
  workspace unified on), `minio-go/v7` v7.0.95 → v7.2.1, `zerolog` v1.34.0 → v1.35.1, `go.step.sm/crypto` v0.73.0 →
  v0.84.1, `golang.org/x/crypto` v0.51.0 → v0.53.0 and `golang.org/x/net` v0.54.0 → v0.56.0. No dependency moved below
  the versions pinned for the v0.8.7 security fixes.

### Module Version Updates

Provider modules are versioned independently and are tagged after this release, as they depend on it:

- **`provider/smtp`**: TLS configuration support, conversation timeout, and fixes for `From`/`Bcc`, the default
  authentication type, and credentials silently going unused. Requires this release for `types/duration` and for the
  `provider/tls` fix above. Includes breaking changes — see `provider/smtp/CHANGELOG.md`.
- **`provider/kafka`**, **`provider/nats`**: changelog and dependency updates only.

## [v0.8.7]

### Security

- **Go 1.26.3**: Upgraded from Go 1.24.7, fixing 15 stdlib vulnerabilities including XSS in `html/template`, DoS in `net/http` (HTTP/2), panics in `crypto/x509`, and parsing issues in `net/url` and `net/mail`.
- **github.com/jackc/pgx/v5**: Upgraded from v5.7.5/v5.7.6 to v5.9.2.
- **golang.org/x/net**: Upgraded from v0.48.0 to v0.54.0, fixing HTTP/2 infinite loop DoS (GO-2026-4918).
- **github.com/quic-go/quic-go**: Upgraded from v0.54.1 to v0.59.1, fixing HTTP/3 QPACK header expansion DoS (GO-2025-4233).
- **go.opentelemetry.io/otel**: Upgraded from v1.39.0 to v1.43.0, fixing baggage header amplification DoS (CVE-2026-29181).
- **go.opentelemetry.io/otel/sdk**: Upgraded from v1.39.0 to v1.43.0, fixing PATH hijacking on macOS/BSD (CVE-2026-24051, CVE-2026-39883).
- **filippo.io/edwards25519**: Upgraded from v1.1.0 to v1.1.1, fixing incorrect `MultiScalarMult` results (CVE-2026-26958).

### Fixed

- **provider/tls**: Fixed `%q` format verb on `uint16` TLS version values that became a build error under Go 1.26.3 stricter vet checks.

## [v0.8.6]

### Fixed

- **Argon2 config validation in `crypt/hashing`**: invalid Argon2 parameters now return `ErrInvalidConfig` instead of panicking or producing hashes with unsafe zero-value settings.
- **Credential state handling in `crypt/secure`**: updating a credential from empty to non-empty now restores the stored secret correctly instead of leaving the credential permanently marked empty.
- **Credential lifecycle safety in `crypt/secure`**: calling `Update()` after `Clear()` now returns `ErrCredentialCleared` instead of crashing with a nil-pointer panic.
- **Base64 token input validation in `crypt/token`**: negative byte lengths now return `ErrInvalidByteLength` instead of panicking.
- **Shutdown skips destructors on fatal error**: `Shutdown()` now runs registered destructors before calling `log.Fatal()`, ensuring cleanup always executes regardless of shutdown reason.
- **Startup failure bypasses destructor cleanup**: `Container.Run()` now calls `AbortFatal()` instead of `Terminate()` when a `RuntimeFn` returns an error, ensuring destructors registered by earlier startup steps are properly invoked.
- **CSRF protection silent no-op without session**: `CSRFProtection()` middleware now returns 403 instead of passing through when no session is available on unsafe HTTP methods.
- **ServerConfig.Validate() loses default server name**: `Validate()` now writes the default server name back to the config struct instead of a local variable.
- **Integration tests run without Docker**: `db/integration_testcontainers_test.go` now has `//go:build integration` tag to prevent failures during plain `go test ./...`.
- **runner: Stop() goroutine leak on timeout**: `Stop()` no longer resets status before the goroutine exits, preventing concurrent `Start()` from launching a second goroutine while the first is still running.
- **runner: NewUpdater accepts invalid inputs**: `NewUpdater()` now validates interval, function, and logger parameters, returning an error instead of panicking at runtime. Signature changed to `(*PeriodicRunner, error)`.
- **threadpool: worker counter uses mutex on hot path**: `Worker.requestCounter` replaced with `atomic.Uint64`, removing per-job mutex overhead. `WorkerGroup.RequestCount()` simplified to sequential iteration.
- **threadpool: dispatch methods don't check if pool is started**: `Dispatch()` panics, `TryDispatch()`/`DispatchWithTimeout()` return false, and `DispatchWithContext()` returns `ErrPoolNotStarted` when the pool has not been started.
- **Config default validation**: `config/provider/env` and `config/provider/json` now return `config.ErrInvalidDefault` when a `default:` tag cannot be parsed into the target field type instead of silently leaving zero values in place.
- **Nested pointer config loading**: `config/provider/env` now allocates and fills nested `*struct` fields during recursive env loading, including applying nested defaults.
- **Nested pointer JSON defaults**: `config/provider/json` now applies defaults recursively through nested `*struct` fields instead of skipping pointer-backed sub-configs.

### Added

- **Explicit file-or-string helper**: `config.StrOrFileIfExists()` reads any existing regular file path, including plain relative paths, while preserving the original `StrOrFile()` behavior for existing callers.

## [v0.8.5]

### Added
- **runner** (/runner): a simple manager for periodic tasks with configurable intervals and error handling

### Fixed

- **batchwriter**: Fixed flush goroutine race where `Stop()` could return while flush operations were still in progress
- **batchwriter**: Fixed `drainAndStop()` premature exit that could lose buffered records during shutdown
- **types/collections**: Fixed missing `defer` on mutex unlock in `Map` operations that could deadlock on panic
- **runner**: Added panic recovery in `PeriodicRunner` to prevent application crash from user-supplied function panics

## [v0.8.4]

### Changed

- **db.Repository**: Changed ```Insert()``` interface to remove variadic lists; existing code may require migration


### Removed
- **db.Repository**: Removed helper function ```ToAnySlice()```

## [v0.8.3]

### Changed

- **CI/CD**: SBOM generation now triggers on version tags and attaches artifacts to GitHub releases
    - CycloneDX JSON SBOMs for core module and all providers
    - Trivy vulnerability scan report included in release assets

## [v0.8.2]

### Added

- **PIN Generation** (crypt/pin) - Cryptographically secure PIN generation and comparison
    - Numeric PIN generation using `crypto/rand` for secure randomness
    - Alphanumeric PIN generation with uppercase letters and digits
    - Auto-formatting with dashes every 3 characters for readability
    - Constant-time comparison functions to prevent timing attacks
    - Case-insensitive alphanumeric comparison support

## [v0.8.0] - Core Framework + Module Independence

### Module Version Matrix

This release introduces independent module versioning. Initial module versions:

| Module                | Version | Status   | Notes                                   |
|-----------------------|---------|----------|-----------------------------------------|
| provider/kafka        | v0.8.0  | ✅ Stable | Message streaming and processing        |
| provider/nats         | v0.8.0  | ✅ Stable | Lightweight messaging                   |
| provider/mqtt         | v0.8.0  | ✅ Stable | IoT messaging protocol                  |
| provider/redis        | v0.8.0  | ✅ Stable | Caching and key-value storage           |
| provider/s3           | v0.8.0  | ✅ Stable | Object storage with multi-cloud support |
| provider/etcd         | v0.8.1  | ✅ Stable | Distributed coordination                |
| provider/pgsql        | v0.8.0  | ✅ Stable | PostgreSQL database operations          |
| provider/clickhouse   | v0.8.0  | ✅ Stable | Analytics database                      |
| provider/httpserver   | v0.8.1  | ✅ Stable | HTTP server with security features      |
| provider/metrics      | v0.8.0  | ✅ Stable | Application metrics collection          |
| provider/smtp         | v0.8.0  | ✅ Stable | Email functionality                     |
| provider/htpasswd     | v0.8.0  | ✅ Stable | HTTP basic authentication               |
| provider/hmacprovider | v0.8.0  | ✅ Stable | HMAC authentication                     |
| provider/jwtprovider  | v0.8.0  | ✅ Stable | JWT authentication and session tracking |

### Core Framework Changes

### Breaking Changes

- **Modular Architecture**: Blueprint is now organized as a modular framework using Go modules with rewrite rules
    - Each provider is now a separate Go module for independent versioning and reduced dependencies
    - **Backward Compatible**: All existing imports continue to work without changes
    - **New Installation**: Users can now install only the providers they need

### Added

- **Modular Provider System**:
    - 14 independent provider modules: kafka, nats, mqtt, redis, s3, etcd, pgsql, clickhouse, httpserver, metrics, smtp,
      htpasswd, hmacprovider, jwtprovider
    - Go workspace support with `go.work` for development
    - Independent semantic versioning for each provider
    - Selective dependency installation

### Changed

- **Build System**: Updated Makefile with targets for modular builds and testing
- **CI/CD**: Enhanced GitHub Actions workflows for multi-module SBOM generation
- **Documentation**: Updated all documentation to reflect modular architecture and installation methods

### Technical Details

- Uses Go module rewrite rules for seamless backward compatibility
- Zero runtime overhead - redirects happen at build time
- Each provider can be updated independently
- Workspace-based development for contributors

## [v0.7.0]

### Added

- **S3 Provider** (provider/s3) - Complete S3-compatible storage integration
    - Multi-cloud support for AWS S3, MinIO, DigitalOcean Spaces, Backblaze B2
    - Comprehensive bucket and object operations with automatic multipart uploads
    - Advanced features: range downloads, presigned URLs, metadata management
    - Security features: TLS/SSL encryption, secure credential handling
    - Performance optimizations: configurable HTTP connection pooling, concurrent operations, smart timeouts
    - Complete CLI sample application (samples/s3-client) with comprehensive examples
    - Full documentation with integration examples and troubleshooting guide
    - Docker Compose setup for easy MinIO testing and development

- **ThreadPool FuncRunner** (threadpool/FuncRunner) - Function wrapper utility
    - Simple wrapper to convert `func(ctx context.Context)` into Job interface
    - Enables dispatching regular functions to threadpool for concurrent execution
    - Comprehensive test suite with performance benchmarks and integration tests

- **ETCD Client Provider** (provider/etcd) - Distributed coordination and configuration management
    - Complete etcd v3 client implementation with TLS support
    - Distributed locking mechanism with lease management and automatic renewal
    - Configuration storage and retrieval with watch functionality
    - Integration tests with testcontainers for reliable testing
    - Sample application demonstrating etcd usage patterns
    - Comprehensive documentation with usage examples

- **mTLS Support** - Mutual TLS authentication for enhanced security
    - Complete mTLS implementation for HTTP server with client certificate validation
    - Certificate generation utilities and sample applications
    - Comprehensive mTLS documentation with security best practices
    - Sample applications demonstrating client and server mTLS configurations

- **Testcontainers Integration** - Modern test infrastructure
    - Migration from Docker Compose to Testcontainers for all integration tests
    - Enhanced test reliability and isolation across all providers
    - Simplified test setup and teardown procedures
    - Better CI/CD integration with containerized testing

### Enhanced

- **Testing Infrastructure Modernization**
    - Complete migration of integration tests to Testcontainers
    - Improved test isolation and reliability for ClickHouse, Kafka, MQTT, NATS, PostgreSQL
    - Consolidated database integration tests with better organization
    - Enhanced Makefile with streamlined test targets

- **PostgreSQL Migration System** - Improved ALTER TABLE handling
    - Fixed DEFAULT value separation in ALTER TABLE queries for better compatibility
    - Enhanced migration reliability with better SQL generation
    - Improved error handling in schema modification operations

- **Password Hashing Reliability**
    - Improved hasher implementation with better error handling
    - Enhanced test coverage for cryptographic operations
    - More robust password validation and security checks

- **Documentation Improvements**
    - Comprehensive S3 provider documentation with multi-cloud examples
    - Enhanced mTLS security documentation with implementation guides
    - Updated HTTP server security documentation
    - Improved sample application documentation and usage examples

### Removed

- **Docker Infrastructure Cleanup**
    - Removed legacy Dockerfiles and docker-compose.yml
    - Simplified build process with focus on Testcontainers
    - Streamlined development environment setup

### Fixed

- **Hash Provider Stability**
    - Enhanced reliability in cryptographic operations
    - Better error handling in password hashing functions
    - Improved test stability and coverage

- **Integration Test Improvements**
    - Resolved test flakiness through Testcontainers migration
    - Better resource management and cleanup in tests
    - Enhanced test isolation and parallel execution

### Security

- **mTLS Implementation**
    - Mutual TLS authentication for client certificate validation
    - Enhanced security for service-to-service communication
    - Certificate-based authentication with comprehensive validation

## [v0.6.1]

### Added

- **RateLimiter** (provider/ratelimiter) - Generic rate limiter
    - Configurable generic rate-limiter with memory backend
    - Suitable for rate-limiting operations such as login

- **CORS Middleware** (provider/httpserver/security) - Configurable CORS middleware
    - Development mode with dynamic origin

- **Text token generation** (secure/token) - Helper to generate a URL-safe, base64-encoded token with configurable
  entropy

- **Password hasher interface** (secure/hasher/PasswordHasher)
    - Provides a simple, generic interface for password hashing
    - Provides an Argon2 implementation using the existing functions;

### Changed

- **Breaking:** removal of str.Contains() - function does not make sense anymore; use slices.Contains() instead
- **Breaking:** crypt/hashing/Argon2* changes
    - Argon2Config{} is now used as a pointer; relevant methods have been updated
    - Argon2IdCreateHash signature change - cfg now comes first

## [v0.6.0]

### Added

- **SMTP Provider** (provider/smtp) - Complete SMTP email functionality
    - Full SMTP client implementation with TLS support
    - Template-based email composition with HTML and plain text support
    - Attachment handling for email messages
    - Comprehensive test suite with unit tests

- **Session Authentication System** - Complete session-based authentication with multiple providers
    - New session authentication middleware (provider/httpserver/auth/session.go)
    - Session data management with enhanced security features
    - Token list management for session tracking
    - Integration with existing HMAC and JWT providers

- **Enhanced Configuration System**
    - Environment variable provider with struct scanning capabilities
    - Better parsing of configuration keys and default values
    - Improved JSON configuration provider with enhanced validation

- **AES256-GCM Encryption** - New constant-time encryption implementation
    - Secure AES256-GCM encryption with constant timing
    - Comprehensive test suite with benchmark tests
    - Enhanced cryptographic security for sensitive data

- **Sample Applications**
    - HTTP server with session authentication (samples/httpserver-session)
    - Updated HMAC provider examples with improved documentation
    - Enhanced Python HMAC client examples

### Enhanced

- **HMAC Provider Improvements**
    - Multiple key support for enhanced security
    - Renamed userId to keyId for better clarity
    - Enhanced key provider interface
    - Improved integration tests and documentation

- **Migration Manager** - Module-aware migration system
    - Enhanced migration manager with better module support
    - Improved integration test infrastructure across all providers
    - Better error handling and validation in migration processes
    - Expanded test coverage for ClickHouse and PostgreSQL migrations

- **HTTP Server Authentication**
    - Unified authentication middleware supporting JWT, HMAC, and session providers
    - Enhanced token management and validation
    - Improved security response patterns and error handling
    - Better fingerprinting middleware integration

- **Testing Infrastructure**
    - New Docker-based test containers for ClickHouse, Kafka, MQTT, NATS, and PostgreSQL
    - Enhanced Makefile with comprehensive test targets
    - Improved integration test organization and reliability
    - Better test isolation and cleanup procedures

- **Documentation**
    - Comprehensive updates to HMAC provider documentation
    - Enhanced HTTP server authentication and session documentation
    - Updated configuration system documentation
    - Improved sample application documentation

### Fixed

- Integration test stability improvements across all providers
- Enhanced error handling in database repository operations
- Better cleanup procedures in test suites
- Improved migration validation and error reporting
- Various dependency updates for security and compatibility

### Security

- **AES256-GCM Implementation**
    - Constant-time encryption to prevent timing attacks
    - Secure key derivation and nonce generation
    - Enhanced cryptographic security for credential storage

- **Session Security**
    - Enhanced session data protection
    - Improved session token management
    - Better session expiration and cleanup mechanisms

- **Multi-Provider Authentication**
    - Unified security model across HMAC, JWT, and session providers
    - Enhanced validation and error handling
    - Improved DoS protection and input validation

## [v0.5.2]

### Added

- Dummy release to appease goproxy :)

## [v0.5.1]

### Added

- **HMAC Provider** (provider/hmacprovider) - Complete HMAC-SHA256 authentication system with replay attack protection
    - Dual signature methods: basic SHA256Sign/Verify and advanced Sign256/Verify256 with nonce/timestamp
    - Multiple nonce storage backends: memory, Redis, and generic key-value stores
    - Configurable timestamp validation windows and input size limits
    - Automatic nonce expiration and cleanup with configurable eviction policies
    - DoS protection with input size limits and fail-safe error handling
- **HMAC Authentication Middleware** (provider/httpserver/auth/hmac.go) for HTTP request authentication
- **Python HMAC Client Library** (samples/hmac-python-client) with full Blueprint compatibility
    - Complete Python client implementation with context manager support
    - Cross-language HMAC signature compatibility
    - Comprehensive test suite with unit and integration tests
    - Example usage and detailed documentation
- **JWT Provider Enhancements**
    - Token revocation system with pluggable backends
    - User session tracking and session limit enforcement
    - Token rotation with secure refresh functionality
    - Enhanced security: DoS protection, token size limits, parsing timeouts
    - Cryptographically secure JWT IDs with 256 bits of entropy
- **Sample Applications**
    - HTTP server with HMAC authentication (samples/httpserver-hmacprovider)
    - JWT user session tracking example (samples/jwtprovider-user-tracking)
    - Python HMAC client demonstration server
- **Security & DevOps**
    - SBOM (Software Bill of Materials) generation with Trivy security scanning
    - Enhanced logging with proper stack trace reporting
    - Comprehensive documentation for HMAC and JWT providers

### Enhanced

- **JWT Provider Security Features**
    - Mandatory JWT ID for all tokens enabling revocation support
    - Session management with configurable maximum concurrent sessions per user
    - Automatic cleanup of expired revocation entries
    - Enhanced algorithm support verification (HS256/384/512, RS256/384/512, ES256/384/512, EdDSA)
    - Reserved claim protection to prevent header injection attacks
- **Logger Improvements**
    - Fixed stack trace reporting to point to actual relevant code lines
    - Enhanced error context and debugging capabilities
- **HTTP Server Authentication**
    - Unified authentication middleware supporting both JWT and HMAC providers
    - Improved error handling and security response patterns

### Fixed

- JWT provider test failures related to token revocation manager setup
- NATS unit test intermittent failures in integration testing
- Various dependency vulnerabilities through updates
- Logger stack trace accuracy issues
- Memory management and cleanup in JWT revocation system

### Security

- **HMAC Provider Security**
    - Replay attack protection using UUID-based nonces with TTL
    - Timing attack resistance through constant-time comparisons
    - Input validation and size limits to prevent DoS attacks
    - Secure credential storage with encrypted secret keys
- **JWT Provider Security**
    - Token revocation capability to invalidate compromised tokens
    - Session limit enforcement to prevent token accumulation attacks
    - Enhanced validation for issuer and audience claims
    - DoS protection with token size and parsing timeout limits
- **Cross-Language Security**
    - Python client library with same security standards as Go implementation
    - Verified compatibility and security parity between language implementations

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

- **Breaking:** Session management architecture - JWT functionality moved from session package to dedicated jwtprovider
  package
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
