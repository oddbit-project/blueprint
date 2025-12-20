# Changelog

All notable changes to the prometheus provider will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [v0.8.0]

### Added

- Initial prometheus metrics server implementation
- Configurable metrics endpoint (default: `/metrics`)
- Configurable host and port (default: `localhost:2220`)
- TLS support via `tlsProvider.ServerConfig`
- Custom collector registration via variadic `prometheus.Collector` arguments
- Graceful shutdown support
