# Changelog

All notable changes to the prometheus provider will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Changed

- Refactored to use `provider/httpserver` instead of standalone HTTP server
- `Config` now embeds `httpserver.ServerConfig` for full httpserver feature support
- `Server` now wraps `*httpserver.Server` with Gin router integration
- Uses custom `prometheus.Registry` instead of global registry to avoid duplicate registration panics
- Replaced deprecated `prometheus.NewProcessCollector()` and `prometheus.NewGoCollector()` with `collectors` package equivalents

### Added

- `Register()` function to add prometheus metrics endpoint to an existing httpserver
- `Registry()` method to access the prometheus registry for registering additional collectors
- TLS support inherited from httpserver
- Read/write timeout configuration inherited from httpserver
- Trusted proxies configuration inherited from httpserver

### Removed

- Standalone `http.ServeMux` router (now uses Gin via httpserver)
- Direct TLS configuration (now handled by embedded `httpserver.ServerConfig`)

## [v0.8.0]

### Added

- Initial prometheus metrics server implementation
- Configurable metrics endpoint (default: `/metrics`)
- Configurable host and port (default: `localhost:2220`)
- TLS support via `tlsProvider.ServerConfig`
- Custom collector registration via variadic `prometheus.Collector` arguments
- Graceful shutdown support
