# S3 Provider Changelog

All notable changes to the Blueprint S3 provider will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [Unreleased]

### Breaking changes

- With `tlsEnable`, a `tlsCert` without `tlsKey`, a `tlsKey` without `tlsCert`, or a key password
  without both now fails `Validate()` with `tls.ErrTLSIncompleteKeyPair`, when built with a core
  release that carries the check (after v0.14.0), whether this provider or the application requires
  it. They used to be ignored silently: the client connected without a client certificate, so a
  server requiring one rejected the handshake and a server only requesting one could accept it
  without the intended identity. Set both `tlsCert` and `tlsKey`, or remove the client certificate
  settings (#115).

## [v0.11.0] - 2026-10-08

Requires Blueprint core v0.14.0.

### Breaking changes

- TLS settings (`tlsCa`, `tlsCert`, `tlsKey`, `tlsInsecureSkipVerify` or a key password) without
  `tlsEnable` now fail `Validate()` with `tls.ErrTLSNotEnabled`. They used to be ignored
  silently: with `useSSL` the client used TLS on the system roots, without the CA, client
  certificate or skip-verify setting, so a private-CA endpoint failed the handshake; without
  `useSSL` the requests went over plain http. Set `tlsEnable`, or remove the settings (#116).
- `tlsEnable` with `useSSL` false now fails `Validate()` with the new `ErrTLSRequiresSSL`. The
  TLS settings were dropped and requests went over plain http. Set `useSSL`, or unset
  `tlsEnable` (#116).

## [v0.10.0] - 2026-10-05

Requires Blueprint core v0.11.0. `BucketInterface` gains methods, which breaks custom implementations.

### Added

- `Bucket.PutObjectInfo` and `Bucket.CopyObjectVersion` return the written `ObjectVersion`
  (bucket, key, version id, ETag, size); `CopySource` copies a specific source version (#94).
- `Bucket.SetObjectLegalHoldVersion` sets or clears a legal hold on a specific version (#94).
- `ErrEmptyCopyResult`: a copy whose response carries no ETag (S3 "200 OK" with an error body) now fails.

### Changed

- **Breaking:** `BucketInterface` gains `PutObjectInfo`, `CopyObjectVersion` and
  `SetObjectLegalHoldVersion`; custom implementations must add them.
- `CopyObject` now logs `copy_object` start/end events; put end events log `version_id`.
- `CopyObject` with `LockMode`/`RetainUntilDate` or `LegalHold` into a bucket without Object Lock
  now fails (`Bucket is missing ObjectLockConfiguration`); the options used to be dropped.

### Fixed

- `CopyObject` ignored `ObjectOptions.LockMode`, `RetainUntilDate` and `LegalHold`, producing an
  unlocked copy without error; they are now applied to the destination. `LockMode` and
  `RetainUntilDate` are sent only together: a copy with just one of them is still written unlocked.
- Docs: `GetObjectLegalHold` returns `false`, not an error, when no hold was ever set.

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

### Added
- Object Lock (WORM) support:
  - `BucketOptions.ObjectLocking` to enable Object Lock at bucket creation
  - `ObjectOptions.LockMode`, `RetainUntilDate`, `LegalHold` to set retention/legal hold at upload time
  - `Bucket.SetObjectRetention` / `GetObjectRetention` for object retention (Governance/Compliance)
  - `Bucket.SetObjectLegalHold` / `GetObjectLegalHold` for legal holds
  - `Bucket.SetObjectLockConfig` / `GetObjectLockConfig` for bucket-level default retention
  - `DeleteObject` now accepts `DeleteOptions` (`VersionID`, `GovernanceBypass`, `ForceDelete`) to delete specific versions and remove WORM-locked objects
- Object versioning visibility:
  - `ObjectInfo` now includes `VersionID`, `IsLatest`, and `IsDeleteMarker`, populated by `HeadObject` and `ListObjects`
  - `ListOptions.Versions` lists all object versions (including delete markers) on versioned buckets
- Version targeting on reads: optional version id on `GetObject`, `HeadObject`, `GetObjectRetention`, `GetObjectLegalHold`

### Fixed
- Server-side encryption options (`ServerSideEncryption`, `SSEKMSKeyId`, `SSEKMSEncryptionContext`, `SSECustomerKey`) are now applied to uploads and copies; previously they were silently ignored
- Multipart tuning (`Config.PartSize`, `Config.Concurrency`, and `UploadOptions.Concurrency`) is now passed to the MinIO client; previously these settings had no effect
- `GetObjectRange` with an open-ended range starting at offset 0 now returns the whole object instead of a single byte
- `GetObjectLegalHold` returns `false` (not an error) for objects that never had a legal hold set
- `GetObjectLockConfig` returns `Enabled: false` (not an error) for buckets without Object Lock
- `Connect` no longer panics when no logger is configured and credential fetch fails
- Removed a no-op secret-key "zeroing" loop in `Connect` that provided false security
- `BucketInterface` signatures now match `*Bucket` and are enforced with compile-time assertions
- `CopyObject` can decrypt an SSE-C encrypted source via `ObjectOptions.SourceSSECustomerKey`
- `aws:kms:dsse` server-side encryption now returns a clear error instead of silently downgrading to SSE-KMS

## [v0.8.1]

### Security

- Upgraded Go from 1.23.0 to 1.26.3, fixing 15 stdlib vulnerabilities.
- Upgraded `go.opentelemetry.io/otel/sdk` to v1.43.0, fixing PATH hijacking (CVE-2026-24051, CVE-2026-39883).
- Upgraded `go.opentelemetry.io/otel` to v1.43.0, fixing baggage header DoS (CVE-2026-29181).

## [v0.8.0]

### Added
- Initial release of S3 provider as independent module
- Multi-cloud S3-compatible storage support (AWS S3, MinIO, DigitalOcean Spaces, Backblaze B2)
- Comprehensive bucket and object operations
- Automatic multipart uploads for large files
- Range downloads and presigned URLs
- Metadata management and tagging
- Security features with TLS/SSL encryption
- Performance optimizations with configurable HTTP connection pooling
- Concurrent operations and smart timeouts
- Complete CLI sample application (samples/s3-client)
- Docker Compose setup for MinIO testing
- Integration tests with testcontainers
- Comprehensive error handling

### Technical Details
- AWS SDK v2 implementation
- Support for custom S3-compatible endpoints
- Credential chain support (IAM, environment, config files)
- Multipart upload threshold configuration
- Connection pooling and retry mechanisms
- Graceful shutdown handling

### Dependencies
- Compatible with Blueprint core framework v0.8.0+
- Requires S3-compatible storage service

### Migration Notes
- No breaking changes from previous Blueprint versions
- All existing imports continue to work unchanged