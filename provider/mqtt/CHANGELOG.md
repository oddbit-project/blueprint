# MQTT Provider Changelog

All notable changes to the Blueprint MQTT provider will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [v0.8.1]

### Security

- Upgraded Go from 1.23.0 to 1.26.3, fixing 15 stdlib vulnerabilities.
- Upgraded `go.opentelemetry.io/otel/sdk` to v1.43.0, fixing PATH hijacking (CVE-2026-24051, CVE-2026-39883).
- Upgraded `go.opentelemetry.io/otel` to v1.43.0, fixing baggage header DoS (CVE-2026-29181).

## [v0.8.0]

### Added
- Initial release of MQTT provider as independent module
- IoT messaging protocol support (MQTT 3.1.1 and 5.0)
- Publisher and subscriber functionality
- QoS levels support (0, 1, 2)
- Last Will and Testament (LWT) support
- Configuration management with TLS/SSL
- Integration tests with testcontainers
- Comprehensive error handling

### Technical Details
- Full MQTT client implementation
- Support for retained messages
- Connection keep-alive and auto-reconnect
- Topic filtering and wildcards
- Graceful shutdown handling

### Dependencies
- Compatible with Blueprint core framework v0.8.0+
- Requires MQTT broker (Mosquitto, EMQX, etc.)

### Migration Notes
- No breaking changes from previous Blueprint versions
- All existing imports continue to work unchanged