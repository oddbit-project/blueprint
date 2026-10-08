# MQTT Provider Changelog

All notable changes to the Blueprint MQTT provider will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [Unreleased]

Requires the Blueprint core release after v0.13.0 (`tls.ClientConfig.ValidateEnabled`, #116).

### Breaking changes

- TLS settings (`tlsCa`, `tlsCert`, `tlsKey`, `tlsInsecureSkipVerify` or a key password) without
  `tlsEnable` now fail `Validate()` with `tls.ErrTLSNotEnabled`. They used to be ignored
  silently: with protocol `tcp` the client connected in plaintext, username and password
  included; with protocol `ssl` it used TLS without the CA, client certificate or skip-verify
  setting. Set `tlsEnable`, or remove the settings (#116).

## [v0.9.0] - 2026-09-27

Requires Blueprint core v0.11.0.

### Changed

- Code-quality fixes from enabling golangci-lint (unchecked errors made explicit, staticcheck
  simplifications, dead unexported code removed); no API changes.

## [v0.8.2] - 2026-09-20

### Security

- **golang.org/x/crypto**: upgraded from v0.53.0 to v0.57.0, fixing an authentication bypass in `golang.org/x/crypto/ssh` where source-address restrictions on an authorized key were not enforced (CVE-2026-56854).
- Requires Blueprint core v0.10.2, which carries the same upgrades.

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