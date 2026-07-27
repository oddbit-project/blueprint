# SMTP Provider Changelog

All notable changes to the Blueprint SMTP provider will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [Unreleased]

### Added

- **TLS configuration**: the embedded `tls.ClientConfig` is now applied in full — `tlsCa`, `tlsCert`, `tlsKey` and
  `tlsInsecureSkipVerify` are honoured, so private CAs, client certificates and self-signed servers are supported.
  `ServerName` defaults to the configured host, and the minimum TLS version to 1.2.
- **`tlsPolicy`**: STARTTLS policy, one of `mandatory` (default), `opportunistic` or `none`.
- **`sslOnConnect`**: implicit TLS (SMTPS, usually port 465) instead of STARTTLS.
- **`timeout`**: timeout in seconds, defaulting to 15, applied as a deadline over the whole SMTP conversation — the
  connection attempt, the server greeting, STARTTLS and authentication — and renewed before each message is sent.
  `go-mail`'s own timeout only covers establishing the connection, so a server that accepts a connection and then
  stalls would previously hang the caller indefinitely. To set the deadline the provider supplies its own dial
  function, which also performs the TLS handshake when `sslOnConnect` is enabled.
- **`Mailer.SendWithContext(ctx, ...)`**: sends with a caller-supplied context bounding the connection attempt.
  `Send` is unchanged and delegates to it with `context.Background()`.
- **`ErrTLSNotEnabled`**: `Validate()` now rejects a configuration that sets `tlsCa`, `tlsCert`, `tlsKey` or
  `tlsInsecureSkipVerify` while `tlsEnable` is false, as those settings would otherwise be silently ignored. Note that
  `tlsEnable` does not by itself decide whether the connection is encrypted — that is `tlsPolicy`, which requires
  STARTTLS by default.

### Changed

- **`Send` uses a single connection**: messages were previously sent one connection per message; the batch now dials
  once, sends each message in turn and closes the connection at the end.
- **Send error reporting**: connection failures (dial, TLS handshake, STARTTLS, authentication) now return
  `ErrSMTPServer`; failures while transferring a message return `ErrMessage`. Both wrap the underlying `go-mail` error.
- **`Send` no longer stops at the first failing message**: the remaining messages are still attempted, and the failures
  are joined into a single `ErrMessage`. `go-mail` continues to record the failure on the message itself.
- **Removed `ErrClient`**, which was declared but never returned; client creation failures return `ErrCreatingClient`.
- **`NewConfig()`** no longer spells out zero-valued fields; the defaults it sets are `host`, `port`, `authType`,
  `tlsPolicy`, `timeout` and `from`.
- **Auto-discovered authentication with `sslOnConnect`**: because the provider dials TLS itself, `go-mail` does not
  treat the connection as encrypted when auto-discovering the authentication mechanism, and will only select
  SCRAM-SHA-256, SCRAM-SHA-1 or CRAM-MD5. Configure an explicit `authType` when the server offers only PLAIN or LOGIN.

### Fixed

- Previously, enabling TLS built a `tls.Config` with only `InsecureSkipVerify` set, without `ServerName`; connections
  with certificate verification enabled failed with `tls: either ServerName or InsecureSkipVerify must be specified`.
- Client creation and send errors now wrap the underlying error instead of discarding it.
- `Config.Bcc` was validated but never applied; the configured recipients are now added to every message created by
  `NewMessage`, after the message options, so they do not replace a BCC address set on the message itself.
- `Config.From` was never applied; messages built without `WithFrom` had no From header. It is now set on every message
  created by `NewMessage`, before the message options, so `WithFrom` still overrides it, and is validated by
  `Validate()` with the previously unused `ErrInvalidFrom`.
- `NewMailer(NewConfig())` failed with `ErrInvalidAuthType`, as the default configuration left `authType` empty and
  `go-mail` rejects an empty authentication type. An empty `authType` is now treated as `noauth`, and `NewConfig()`
  sets it explicitly.

### Migration Notes

- A configuration that sets `tlsInsecureSkipVerify`, `tlsCa`, `tlsCert` or `tlsKey` without `tlsEnable` used to be
  accepted and the settings ignored; it is now rejected with `ErrTLSNotEnabled`. Set `tlsEnable` to keep the intended
  behaviour, or drop the settings.
- Messages built without `WithFrom` now carry the configured `from` address. Callers that relied on setting the sender
  exclusively through `WithFrom` are unaffected; callers that expected no From header need to clear `Config.From`.
- Send errors are wrapped, so `err == smtp.ErrMessage` no longer matches. Use `errors.Is`.
- Connection failures return `ErrSMTPServer` where they previously surfaced as `ErrMessage`.

## [v0.8.2]

### Security

- Upgraded Go from 1.24.0 to 1.26.3, fixing 15 stdlib vulnerabilities.

## [v0.8.1]

### Changed
- Updated go-mail to version 0.7.1 to mitigate CVE-2025-59937


## [v0.8.0]

### Added
- Initial release of SMTP provider as independent module
- Complete SMTP email functionality
- Full SMTP client implementation with TLS support
- HTML and plain text email support
- Email attachment handling
- SMTP authentication (PLAIN, LOGIN, CRAM-MD5)
- Configuration management with secure credentials
- Bulk email sending capabilities
- Comprehensive test suite with unit tests
- Comprehensive error handling

### Technical Details
- SMTP client implementation with go-mail
- Support for multiple SMTP servers
- Attachment encoding and MIME handling

### Dependencies
- Compatible with Blueprint core framework v0.8.0+
- Requires SMTP server for email delivery

### Migration Notes
- No breaking changes from previous Blueprint versions
- All existing imports continue to work unchanged
