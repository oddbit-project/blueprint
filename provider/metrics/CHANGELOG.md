# Metrics Provider Changelog

All notable changes to the Blueprint Metrics provider will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [v0.8.0]

### Added
- Initial release of Metrics provider as independent module
- Application metrics collection and reporting
- Prometheus-compatible metrics export
- Counter, gauge, histogram, and summary metric types
- Custom metric labels and tags support
- HTTP metrics endpoint (/metrics)
- Runtime metrics collection (memory, goroutines, GC)
- Request duration and response size tracking
- Error rate and status code monitoring
- Configuration management for metrics collection
- Integration with HTTP server provider
- Comprehensive error handling

### Technical Details
- Prometheus client library integration
- Automatic metrics registration and cleanup
- Configurable metric collection intervals
- Thread-safe metric operations
- Memory-efficient metric storage
- Graceful shutdown handling

### Dependencies
- Compatible with Blueprint core framework v0.8.0+
- Optional integration with HTTP server provider for metrics endpoint

### Migration Notes
- No breaking changes from previous Blueprint versions
- All existing imports continue to work unchanged