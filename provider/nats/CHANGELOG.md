# NATS Provider Changelog

All notable changes to the Blueprint NATS provider will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

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
