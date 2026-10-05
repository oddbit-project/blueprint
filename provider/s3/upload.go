package s3

import (
	"context"
	"io"

	"github.com/minio/minio-go/v7"
	"github.com/oddbit-project/blueprint/log"
)

// basePutOptions returns PutObjectOptions seeded with the client's multipart tuning
func (b *Bucket) basePutOptions() minio.PutObjectOptions {
	opts := minio.PutObjectOptions{}
	if b.config.PartSize > 0 {
		opts.PartSize = uint64(b.config.PartSize)
	}
	if b.config.Concurrency > 0 {
		opts.NumThreads = uint(b.config.Concurrency)
	}
	return opts
}

// put runs one upload with the existing order: connected check, start log,
// upload timeout, options (an options error is logged), upload, end log.
// opts may be nil; threads > 0 overrides NumThreads after options are applied.
func (b *Bucket) put(ctx context.Context, op, name string, reader io.Reader, size int64,
	opts *ObjectOptions, threads int) (minio.UploadInfo, error) {
	if !b.IsConnected() {
		return minio.UploadInfo{}, ErrClientNotConnected
	}

	startTime := logOperationStart(b.logger, op, name, log.KV{
		"bucket_name": b.bucketName,
	})

	// Use upload timeout (MinIO handles multipart automatically)
	ctx, cancel := getContextWithTimeout(b.uploadTimeout, ctx)
	defer cancel()

	putOpts := b.basePutOptions()

	if opts != nil {
		if err := b.applyMinIOPutOptions(&putOpts, *opts); err != nil {
			logOperationEnd(b.logger, op, name, startTime, err, log.KV{
				"bucket_name": b.bucketName,
			})
			return minio.UploadInfo{}, err
		}
	}

	if threads > 0 {
		putOpts.NumThreads = uint(threads)
	}

	info, err := b.minioClient.PutObject(ctx, b.bucketName, name, reader, size, putOpts)

	kv := log.KV{"bucket_name": b.bucketName}
	if info.VersionID != "" {
		kv["version_id"] = info.VersionID
	}
	logOperationEnd(b.logger, op, name, startTime, err, kv)

	return info, err
}

func firstObjectOptions(opts []ObjectOptions) *ObjectOptions {
	if len(opts) > 0 {
		return &opts[0]
	}
	return nil
}

// PutObject uploads an object to S3
func (b *Bucket) PutObject(ctx context.Context, objectName string, reader io.Reader, size int64, opts ...ObjectOptions) error {
	_, err := b.put(ctx, "put_object", objectName, reader, size, firstObjectOptions(opts), 0)
	return err
}

// PutObjectInfo uploads an object and returns the version it created
func (b *Bucket) PutObjectInfo(ctx context.Context, objectName string, reader io.Reader, size int64, opts ...ObjectOptions) (ObjectVersion, error) {
	info, err := b.put(ctx, "put_object_info", objectName, reader, size, firstObjectOptions(opts), 0)
	if err != nil {
		return ObjectVersion{}, err
	}
	return ObjectVersion{
		Bucket:    b.bucketName,
		Key:       objectName,
		VersionID: info.VersionID,
		ETag:      info.ETag,
		Size:      info.Size,
	}, nil
}

// PutObjectStream uploads an object using streaming (no size required)
func (b *Bucket) PutObjectStream(ctx context.Context, objectName string, reader io.Reader, opts ...ObjectOptions) error {
	// Use -1 for unknown size streaming uploads
	_, err := b.put(ctx, "put_object_stream", objectName, reader, -1, firstObjectOptions(opts), 0)
	return err
}

// PutObjectMultipart uploads an object using multipart upload with progress tracking
func (b *Bucket) PutObjectMultipart(ctx context.Context, objectName string, reader io.Reader, size int64, opts ...ObjectOptions) error {
	// MinIO handles multipart uploads automatically
	_, err := b.put(ctx, "put_object_multipart", objectName, reader, size, firstObjectOptions(opts), 0)
	return err
}

// PutObjectAdvanced provides advanced upload functionality with detailed control
func (b *Bucket) PutObjectAdvanced(ctx context.Context, objectName string, reader io.Reader, size int64, opts UploadOptions) error {
	// Note: MinIO-Go's PutObject does not expose LeavePartsOnError or a per-call
	// MaxUploadParts, so those UploadOptions fields are not applied.
	_, err := b.put(ctx, "put_object_advanced", objectName, reader, size, &opts.ObjectOptions, opts.Concurrency)
	return err
}
