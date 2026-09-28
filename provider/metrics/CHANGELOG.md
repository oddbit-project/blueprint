# Metrics Provider Changelog

All notable changes to the Blueprint Metrics provider will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [Unreleased]

### Fixed

- `Config.Validate()` returns `ErrMissingEndpoint` for an empty `Endpoint` (it used to panic in
  `NewServer`) and `ErrMissingTLSCert` when `TLSEnable` is set without `TLSCert`/`TLSKey`.
- `Config.Validate()` defaults a zero `Port` (to 2201) and non-positive read/write timeouts.
- Corrected the `NewCustomServer` doc comment and the v0.8.0 changelog entry.

## [v0.8.2] - 2026-09-20

### Security

- **golang.org/x/crypto**: upgraded from v0.53.0 to v0.57.0, fixing an authentication bypass in `golang.org/x/crypto/ssh` where source-address restrictions on an authorized key were not enforced (CVE-2026-56854).
- Requires Blueprint core v0.10.2, which carries the same upgrades.

## [v0.8.1]

### Security

- Upgraded Go from 1.23.0 to 1.26.3, fixing 15 stdlib vulnerabilities.

## [v0.8.0]

### Added
- Initial release of Metrics provider as independent module
- Standalone `net/http` server exposing a Prometheus gatherer (`prometheus.DefaultGatherer` by default) on a
  configurable endpoint (`/metrics` by default, port 2201)
- `Config` with host, port, endpoint and read/write timeouts, plus optional TLS through the embedded
  `tls.ServerConfig`
- `NewServer()` / `Config.NewServer()`, and `NewCustomServer()` for a custom gatherer and `promhttp.HandlerOpts`
- `Start()` (blocking; returns nil after `Shutdown()`) and `Shutdown(ctx)`

### Dependencies
- Compatible with Blueprint core framework v0.8.0+
- Prometheus client library (`github.com/prometheus/client_golang`)
