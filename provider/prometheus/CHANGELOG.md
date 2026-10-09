# Changelog

All notable changes to the prometheus provider will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Breaking changes

- With `tlsEnable`, a `tlsCert` without `tlsKey`, a `tlsKey` without `tlsCert`, or a key password
  without both now fails `Config.Validate()`, and so `NewServer`, with `tls.ErrTLSIncompleteKeyPair`,
  through the httpserver provider's `ServerConfig.Validate()`, when built with an httpserver release
  that carries the check. Set both `tlsCert` and `tlsKey`, or remove the certificate settings (#128).

## [v0.10.0] - 2026-09-28

Requires Blueprint core v0.11.0 and `provider/httpserver` v0.9.4.

### Breaking changes

- **`NewServer`/`Config.NewServer` return `(nil, nil)` when `Enabled` is false.** `Enabled` used
  to be ignored. A `Config` not built with `NewConfig()` (a struct literal, or a zero value
  filled from JSON without `"enabled": true`) has `Enabled=false`, so calling `Start` on the
  result now panics with a nil pointer. Set `Enabled: true` or build the config with
  `NewConfig()`, and nil-check the server. `Register` takes no config and is unaffected.
- **`NewConfig()` defaults `Host` to `localhost`** (new `DefaultHost`); the endpoint used to bind
  all interfaces. Set `Host` (for example `"0.0.0.0"`) if Prometheus scrapes from another host,
  container or pod.
- **A zero `Port` becomes `DefaultPort` (2220)** in `Validate()`; it used to become httpserver's
  default, 5000.
- **The module now requires core v0.11.0**, which is itself a breaking release.

### Fixed

- Added `replace` directives for core and httpserver so the module builds outside `go.work`.

## [v0.9.3] - 2026-09-20

### Security

- **golang.org/x/crypto**: upgraded from v0.53.0 to v0.57.0, fixing an authentication bypass in `golang.org/x/crypto/ssh` where source-address restrictions on an authorized key were not enforced (CVE-2026-56854).
- Requires Blueprint core v0.10.2, which carries the same upgrades.

## [v0.9.2] - 2026-07-27

### Security

- Upgraded Go from 1.26.3 to 1.26.5, fixing stdlib vulnerabilities in `crypto/tls`, `net/textproto`, `crypto/x509`,
  `mime` and `os` (GO-2026-5856, GO-2026-5039, GO-2026-5037, GO-2026-5038, GO-2026-4970).
- Upgraded `golang.org/x/text` from v0.38.0 to v0.40.0, fixing an infinite loop on invalid input (GO-2026-5970).

### Fixed

- **The module did not build outside the workspace**: it required `provider/httpserver` v0.8.5, whose `ServerConfig`
  predates the `ServerName` field `config.go` reads, so `go build` failed with `cfg.ServerName undefined` for anyone
  consuming the published module. It now requires core v0.9.0 and `provider/httpserver` v0.9.3.

## [v0.9.1]

### Security

- Upgraded Go from 1.24.7 to 1.26.3, fixing 15 stdlib vulnerabilities.
- Upgraded `github.com/jackc/pgx/v5` from v5.7.6 to v5.9.2.
- Upgraded `golang.org/x/net` to v0.54.0, fixing HTTP/2 DoS (GO-2026-4918).
- Upgraded `github.com/quic-go/quic-go` from v0.54.1 to v0.59.1, fixing HTTP/3 QPACK header expansion DoS (GO-2025-4233).
- Upgraded `go.opentelemetry.io/otel/sdk` to v1.43.0, fixing PATH hijacking (CVE-2026-24051, CVE-2026-39883).
- Upgraded `go.opentelemetry.io/otel` to v1.43.0, fixing baggage header DoS (CVE-2026-29181).

## [v0.9.0]

### Changed

- Updated `NewConfig()` to use `ServerConfig.ServerName` field instead of removed `Options["serverName"]` map, following httpserver v0.9.0 breaking change.

## [v0.8.0]

### Added

- Initial prometheus metrics server implementation
- Configurable metrics endpoint (default: `/metrics`)
- Configurable host and port (default: `localhost:2220`)
- TLS support via `tlsProvider.ServerConfig`
- Custom collector registration via variadic `prometheus.Collector` arguments
- Graceful shutdown support
