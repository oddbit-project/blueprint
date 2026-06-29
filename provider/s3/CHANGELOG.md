# S3 Provider Changelog

All notable changes to the Blueprint S3 provider will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [Unreleased]

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