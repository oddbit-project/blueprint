# S3 Object Lock (WORM) Example

Demonstrates the S3 provider's Object Lock features:

- Creating an object-lock-enabled bucket (`BucketOptions.ObjectLocking`)
- Bucket-level default retention (`SetObjectLockConfig` / `GetObjectLockConfig`)
- Per-object retention at upload time (`ObjectOptions.LockMode` / `RetainUntilDate`)
- Reading and shortening retention with a governance bypass (`SetObjectRetention` / `GetObjectRetention`)
- Legal holds (`SetObjectLegalHold` / `GetObjectLegalHold`)

## Run

Object Lock requires an object-lock-enabled bucket, so the example creates one. Start a local MinIO:

```bash
docker run -p 9000:9000 \
    -e MINIO_ROOT_USER=minioadmin -e MINIO_ROOT_PASSWORD=minioadmin \
    quay.io/minio/minio:latest server /data
```

Then run:

```bash
go run ./samples/s3-objectlock
```

Flags: `-endpoint`, `-region`, `-access-key`, `-secret-key`, `-ssl` (defaults target the MinIO above).

## Notes

- Object Lock can only be enabled at bucket creation; it cannot be turned on for an existing bucket.
- Enabling Object Lock also enables versioning. A plain `DeleteObject` (no version id) writes a delete
  marker and leaves the locked version intact, rather than failing.
- `COMPLIANCE` retention cannot be shortened or removed by anyone until it expires. `GOVERNANCE` retention
  can be shortened/removed with `GovernanceBypass: true` given the `s3:BypassGovernanceRetention` permission.
