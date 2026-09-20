# ETCD Provider Changelog

All notable changes to the Blueprint ETCD provider will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [v0.9.1] - 2026-09-20

### Security

- **golang.org/x/crypto**: upgraded from v0.53.0 to v0.57.0, fixing an authentication bypass in `golang.org/x/crypto/ssh` where source-address restrictions on an authorized key were not enforced (CVE-2026-56854).
- **google.golang.org/grpc**: upgraded from v1.82.1 to v1.83.2, fixing a denial of service via malformed RPC requests (CVE-2026-84445) and CVE-2026-84304.
- Requires Blueprint core v0.10.2, which carries the same upgrades.

## [v0.9.0] - 2026-07-27

Requires Blueprint core v0.9.0.

### Security

- Upgraded Go from 1.26.3 to 1.26.5, fixing stdlib vulnerabilities reachable from the client's TLS handshake and
  certificate verification paths (GO-2026-5856, GO-2026-5039, GO-2026-5037).
- Upgraded `google.golang.org/grpc` from v1.79.3 to v1.82.1, fixing xDS RBAC and HTTP/2 vulnerabilities
  (GHSA-hrxh-6v49-42gf).
- Upgraded `golang.org/x/text` from v0.38.0 to v0.40.0, fixing an infinite loop on invalid input (GO-2026-5970).

### Fixed

- **`Lock.TryLock()` reported a free lock as held**: it emulated a non-blocking attempt by calling the blocking
  `Lock()` under a 1ms timeout, so any round-trip slower than that — routine on a loaded or containerized host —
  returned `false, nil` as if another session held the lock. It now uses etcd's own `Mutex.TryLock()`, which detects
  contention server-side, and `WithTTL` is no longer required to make it reliable: it now only bounds how long the
  attempt may take, and defaults to the context's own deadline.

## [v0.8.4]

### Security

- Upgraded Go from 1.24.0 to 1.26.3, fixing 15 stdlib vulnerabilities.
- Upgraded `go.opentelemetry.io/otel` to v1.43.0, fixing baggage header DoS (CVE-2026-29181).
- Upgraded `filippo.io/edwards25519` from v1.1.0 to v1.1.1, fixing incorrect `MultiScalarMult` results (CVE-2026-26958).

## [v0.8.3]

### Changed
- Changed `Lease()` signature to require explicit context


## [v0.8.2]

### Added
- Helper functions for encryption and decryption

## [v0.8.1]

### Added
- Made configuration timeouts explicitly in seconds; 

## [v0.8.0]

### Added
- Initial release of ETCD provider as independent module
- Distributed coordination and configuration management
- Complete etcd v3 client implementation with TLS support
- Distributed locking mechanism with lease management
- Automatic lease renewal for long-running locks
- Configuration storage and retrieval
- Watch functionality for real-time updates
- Transaction support for atomic operations
- Configuration management with secure connections
- Integration tests with testcontainers
- Sample application demonstrating usage patterns
- Comprehensive error handling

### Technical Details
- etcd v3 client implementation
- Support for clustering and high availability
- Lease-based distributed locking
- Watch streams for configuration changes
- Connection health monitoring
- Graceful shutdown handling

### Dependencies
- Compatible with Blueprint core framework v0.8.0+
- Requires etcd cluster version 3.4+

### Migration Notes
- No breaking changes from previous Blueprint versions
- All existing imports continue to work unchanged