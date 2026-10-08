# NATS Provider Changelog

All notable changes to the Blueprint NATS provider will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [Unreleased]

### Fixed

- `Producer.IsConnected`, `Publish`, `PublishMsg`, `PublishRequest` and `Request`, and
  `Consumer.IsConnected`, `NextMsg`, `Unsubscribe` and `Request`, no longer race with
  `Disconnect` on the `Conn` field when called from another goroutine while it runs, for
  example a worker still publishing during shutdown. Under `-race` such a call was
  reported as a data race. In builds that read the field twice, such as race-enabled or
  unoptimised (`-gcflags='-N -l'`, as debuggers use) builds, `IsConnected` could also
  dereference a nil connection if `Disconnect` cleared the field between its check and its
  call. Code that reads the exported `Conn` field itself while `Disconnect` runs still
  races; use the methods instead (#113).

## [v0.10.0] - 2026-10-08

Requires Blueprint core v0.14.0.

### Security

- With TLS enabled (`tlsEnable`), the connection now requires TLS. Before, a `nats://` URL used
  TLS only when the server's INFO demanded it, so a server without TLS, or an attacker on the
  network path, received CONNECT, username and password or token included, in plaintext, on
  the first connect and on every reconnect. With `tlsInsecureSkipVerify` an on-path attacker
  can still present any certificate (#104).
- TLS settings without `tlsEnable` are now rejected instead of silently leaving the
  connection in plaintext; see Breaking changes (#110).

### Breaking changes

- With `tlsEnable` set, three kinds of deployment that connected in plaintext before now
  fail to connect (#104, #110):
  - Against a server that does not offer TLS, the error is `nats: secure connection not
    available`. Enable TLS on the server, or unset `tlsEnable`.
  - Against a server that offers TLS without requiring it, the client now uses TLS, so a
    certificate the client cannot verify (private CA without `tlsCa`, connecting by IP to a
    certificate with only DNS names) fails with an `x509` error. Set `tlsCa` or connect by
    the certificate's name.
  - Against a server that offers TLS without requiring it but requires a client
    certificate, a client without one fails with `tls: certificate required`. Set
    `tlsCert` and `tlsKey` (#110).
- TLS settings (`tlsCa`, `tlsCert`, `tlsKey`, `tlsInsecureSkipVerify` or a key password)
  without `tlsEnable` now fail `Validate()` with the new `ErrTLSNotEnabled`, for the
  producer, consumer and JetStream configs. They used to be ignored silently: the
  connection, and its credentials, stayed in plaintext unless a `tls://` URL or the server
  forced TLS, and even then the CA, client certificate and skip-verify settings were not
  applied. Set `tlsEnable`, or remove the settings (#110).

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
- `Consumer.Disconnect` now shuts down gracefully: it drains the `Subscribe`
  subscriptions, lets the handlers finish the messages already delivered to them while
  the connection is still open, then drains and closes the connection. Previously it
  unsubscribed and closed at once, so buffered messages were handled on a closed
  connection and their reply acknowledgements failed. Unread `SubscribeSync` messages are
  dropped. `Producer.Disconnect` now waits for its drain to finish. Both can block for up
  to `drainTimeout` plus 5 seconds; lower `drainTimeout` if your shutdown has a tighter
  deadline. To get the graceful drain, keep the `Subscribe` context live until
  `Disconnect` returns (a handler still stops, dropping its buffer, once that context is
  cancelled), and do not call `Disconnect` from inside a handler (#105).
- Once `Consumer.Disconnect` has started, `Subscribe` and `SubscribeSync` return
  `ErrConsumerClosed`; a second `Disconnect` on a consumer or producer waits for the first
  to finish (#105).
- The JetStream types' `Disconnect` closes the connection directly instead of starting a
  drain it then cut short (pending writes are still flushed) (#105).
- Connections now start from nats.go's default options, which also turns on reconnect
  jitter (100ms, 1s with TLS) and a 1-minute write timeout (#105).

### Fixed

- `pingInterval` now defaults to 2 minutes as documented, for producers, consumers and
  JetStream connections. Client pings were off when it was unset, so a stale connection
  was only noticed by TCP (#105).
- A configured `drainTimeout` is now applied to the connection (default 30000ms); it was
  ignored (#105).

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
