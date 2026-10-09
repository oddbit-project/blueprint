# Redis Provider Changelog

All notable changes to the Blueprint Redis provider will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [Unreleased]

### Breaking changes

- With `tlsEnable`, a `tlsCert` without `tlsKey`, a `tlsKey` without `tlsCert`, or a key password
  without both now fails `Validate()` with `tls.ErrTLSIncompleteKeyPair`, when built with a core
  release that carries the check (after v0.14.0), whether this provider or the application requires
  it. They used to be ignored silently: the client connected without a client certificate, so a
  server requiring one rejected the handshake and a server only requesting one could accept it
  without the intended identity. Set both `tlsCert` and `tlsKey`, or remove the client certificate
  settings (#115).

## [v0.10.0] - 2026-10-08

Requires Blueprint core v0.14.0.

### Breaking changes

- TLS settings (`tlsCa`, `tlsCert`, `tlsKey`, `tlsInsecureSkipVerify` or a key password) without
  `tlsEnable` now fail `Validate()` with `tls.ErrTLSNotEnabled`. They used to be ignored
  silently: the client connected in plaintext, password included. Set `tlsEnable`, or remove the
  settings (#116).

## [v0.9.0] - 2026-09-28

Requires Blueprint core v0.12.0.

### Breaking changes

- **`Config` embeds `tls.ClientConfig` instead of `tls.ServerConfig`.** The server-only fields
  `TLSAllowedCACerts` (`tlsAllowedCACerts`), `TLSCipherSuites`, `TLSMinVersion`, `TLSMaxVersion`
  and `TLSAllowedDNSNames` are gone; Go code using them no longer compiles, and JSON configs
  using them are silently ignored. Use `TLSCA` (`tlsCa`) for the CA that signs the Redis server
  certificate and `TLSInsecureSkipVerify` (`tlsInsecureSkipVerify`) to skip verification.
  `TLSCert`/`TLSKey` are now the *client* certificate for mutual TLS.
- **`TLSEnable` is now honoured.** It used to be ignored and the client connected in plaintext.
  A config with `TLSEnable: true` pointing at a plaintext Redis will now fail to connect; turn it
  off or enable TLS on the server.
- **`Prune()` is a no-op.** It used to run `FLUSHDB`. Code that called `Prune()` to empty the
  database must call `client.Redis.FlushDB` explicitly.

### Added

- `Client.SetNX`, implementing `kv.AtomicSetter` (added in the next core release).

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