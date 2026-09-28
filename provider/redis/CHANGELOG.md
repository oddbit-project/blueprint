# Redis Provider Changelog

All notable changes to the Blueprint Redis provider will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [Unreleased]

### Changed

- **Breaking:** `Config` embeds `tls.ClientConfig` instead of `tls.ServerConfig`. Server-only
  fields (`TLSAllowedCACerts`, `TLSCipherSuites`, `TLSMinVersion`, `TLSMaxVersion`,
  `TLSAllowedDNSNames`) are gone; use `TLSCA` and `TLSInsecureSkipVerify`.

### Added

- `Client.SetNX`, implementing `kv.AtomicSetter`.

### Fixed

- **`Prune()` ran `FLUSHDB`**, wiping the whole Redis database, including on every session-store
  cleanup tick when used as the session backend. It is now a no-op; Redis expires keys itself.
- `TLSEnable=true` connected in plaintext; the TLS configuration is now applied.
- Removed a password-zeroing loop that had no effect.

## [v0.8.2] - 2026-09-20

### Security

- **golang.org/x/crypto**: upgraded from v0.53.0 to v0.57.0, fixing an authentication bypass in `golang.org/x/crypto/ssh` where source-address restrictions on an authorized key were not enforced (CVE-2026-56854).
- Requires Blueprint core v0.10.2, which carries the same upgrades.

## [v0.8.1]

### Security

- Upgraded Go from 1.23.0 to 1.26.3, fixing 15 stdlib vulnerabilities.

## [v0.8.0]

### Added
- Initial release of Redis provider as independent module
- Caching and key-value storage capabilities
- Full Redis command support (strings, hashes, lists, sets, sorted sets)
- Connection pooling and clustering support
- Pub/Sub messaging functionality
- Lua scripting support
- Configuration management with TLS support
- Integration tests with testcontainers
- Comprehensive error handling

### Technical Details
- Redis client implementation with go-redis
- Support for Redis Sentinel and Cluster modes
- Pipeline and transaction support
- Connection health monitoring
- Graceful shutdown handling

### Dependencies
- Compatible with Blueprint core framework v0.8.0+
- Requires Redis server version 6.0+

### Migration Notes
- No breaking changes from previous Blueprint versions
- All existing imports continue to work unchanged