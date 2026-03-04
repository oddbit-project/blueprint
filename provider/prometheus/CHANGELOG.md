# Changelog

All notable changes to the prometheus provider will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

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
