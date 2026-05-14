# Changelog

All notable changes to the prometheus provider will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

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
