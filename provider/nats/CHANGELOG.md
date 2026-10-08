# NATS Provider Changelog

All notable changes to the Blueprint NATS provider will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [Unreleased]

### Security

- With TLS enabled (`tlsEnable`), the connection now requires TLS. Before, a `nats://` URL used
  TLS only when the server's INFO demanded it, so a server without TLS, or an attacker on the
  network path, received CONNECT, username and password or token included, in plaintext, on
  the first connect and on every reconnect. With `tlsInsecureSkipVerify` an on-path attacker
  can still present any certificate (#104).

### Breaking changes

- With `tlsEnable` set, two kinds of deployment that connected in plaintext before now fail
  to connect (#104):
  - Against a server that does not offer TLS, the error is `nats: secure connection not
    available`. Enable TLS on the server, or unset `tlsEnable`.
  - Against a server that offers TLS without requiring it, the client now uses TLS, so a
    certificate the client cannot verify (private CA without `tlsCa`, connecting by IP to a
    certificate with only DNS names) fails with an `x509` error. Set `tlsCa` or connect by
    the certificate's name.

### Added

- `url` accepts a comma-separated server list (e.g. a cluster seed list), split as
  `nats.Connect` does, for `ProducerConfig`, `ConsumerConfig` and `JSConnectionConfig`.
  Previously a list failed to connect with "too many colons in address". `Validate()` now
  rejects a `url` with no non-empty entry (such as `" , "`) with the existing missing-URL
  error. The servers are tried in random order (#99).

### Changed

- A comma in `url` now always separates servers. A URL whose credentials contain a comma
  (`nats://user:p,ss@host`) connected before and now fails; percent-encode the comma as
  `%2C` (#99).

## [v0.9.0] - 2026-09-27

Requires Blueprint core v0.11.0.

### Changed

- Code-quality fixes from enabling golangci-lint (unchecked errors made explicit, staticcheck
  simplifications, dead unexported code removed); no API changes.

## [v0.8.3] - 2026-09-20

### Security

- **golang.org/x/crypto**: upgraded from v0.53.0 to v0.57.0, fixing an authentication bypass in `golang.org/x/crypto/ssh` where source-address restrictions on an authorized key were not enforced (CVE-2026-56854).
- Requires Blueprint core v0.10.2, which carries the same upgrades.

## [v0.8.2]

### Security

- Upgraded Go from 1.23.0 to 1.26.3, fixing 15 stdlib vulnerabilities.
- Upgraded `go.opentelemetry.io/otel/sdk` to v1.43.0, fixing PATH hijacking (CVE-2026-24051, CVE-2026-39883).
- Upgraded `go.opentelemetry.io/otel` to v1.43.0, fixing baggage header DoS (CVE-2026-29181).

## [v0.8.1]

### Added
- JetStream support via new `JSProducer` and `JSConsumer` types
  - `JSProducerConfig` with optional `AutoCreateStream` (defaults to false) and
    up-front `Validate()`
  - `StreamConfig` wrapper with friendly JSON tags and a `Native` escape hatch
    for advanced `jetstream.StreamConfig` overrides
  - `JSConsumerConfig` with pull-based `Consume` (auto Ack/Nak on handler
    result) and one-off `Fetch` APIs, plus up-front `Validate()`
  - `JSMessage` wrapper exposing `Ack/Nak/InProgress/Term/Metadata`, with a
    `Native` escape hatch on consumer config for full `jetstream.ConsumerConfig`
    access
  - `EnsureStream` helper for create-or-update outside of producer startup
  - New error constants: `ErrMissingJSURL`, `ErrMissingStreamName`,
    `ErrJSNoConsumer`, `ErrAlreadyConsuming`, `ErrInvalidAckPolicy`,
    `ErrInvalidDeliverPolicy`, `ErrInvalidRetention`, `ErrInvalidStorage`
- Integration tests covering publish/consume, fetch, redelivery, explicit
  stream lookup, double-Consume rejection, and Disconnect-while-consuming
  (testcontainer now runs with `-js`)
- Unit tests for `JSConsumerConfig.Validate` and `JSProducerConfig.Validate`

### Changed
- Extracted shared NATS connection logic into an internal `connect()` helper;
  `NewConsumer` and `NewProducer` now delegate to it. No API changes to the
  existing public types.
- **Core `NewConsumer` token-auth fix (behavior change):** previously
  `ConsumerConfig` with `AuthType: "token"` silently sent an empty token
  because the credential was only loaded for the `basic` auth type. The
  shared connect helper now loads credentials for both `basic` and `token`
  on the consumer side, matching the long-standing producer behavior. If you
  were inadvertently relying on the broken empty-token path (e.g. by also
  embedding credentials in the URL), review your `ConsumerConfig` auth
  setup.
- `JSProducer.PublishAsync` now takes a `context.Context` as its first
  parameter so callers can fail fast on an already-cancelled context
  (consistent with `Publish`/`PublishMsg`). The underlying jetstream async
  publish API takes no context; callers still need to select on the
  `PubAckFuture` channel for post-dispatch cancellation.
- Renamed internal constant `DefaultJSPublishTimeout` →
  `DefaultJSSetupTimeout` (it bounds stream/consumer setup round-trips, not
  publish operations).

### Fixed
- `JSConsumer.Consume` used to silently overwrite its internal consume-context
  reference when called twice, leaking the first session and causing the
  first context's cancellation to stop the wrong consume context. It now
  returns `ErrAlreadyConsuming` on re-entry and correctly associates each
  watcher goroutine with the consume context it created.
- `JSConsumer.Consume` used to leak its watcher goroutine when the caller
  passed a non-cancellable context and then called `Disconnect()`. Disconnect
  now signals an internal stop channel so the goroutine always exits.
- `JSConsumer.Disconnect` is now idempotent; repeated calls are no-ops
  instead of double-draining.
- `buildJSConsumerConfig` no longer returns `ErrInvalidAuthType` for an
  invalid `AckPolicy` value (now returns `ErrInvalidAckPolicy`), and
  `DeliverPolicy` now rejects unknown values with `ErrInvalidDeliverPolicy`
  instead of silently accepting them.

## [v0.8.0]

### Added
- Initial release of NATS provider as independent module
- Lightweight messaging capabilities
- Publisher and subscriber functionality
- Configuration management
- Integration tests with testcontainers
- Comprehensive error handling

## [v0.8.0]

### Added
- Initial release of NATS provider as independent module
- Lightweight messaging capabilities
- Publisher and subscriber functionality
- Configuration management
- Integration tests with testcontainers
- Comprehensive error handling

### Technical Details
- Full NATS client implementation
- Support for request-reply patterns
- Connection pooling and retry mechanisms
- Graceful shutdown handling

### Dependencies
- Compatible with Blueprint core framework v0.8.0+
- Requires NATS server version 2.0+

### Migration Notes
- No breaking changes from previous Blueprint versions
- All existing imports continue to work unchanged
