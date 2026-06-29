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

// PutObject uploads an object to S3
func (b *Bucket) PutObject(ctx context.Context, objectName string, reader io.Reader, size int64, opts ...ObjectOptions) error {
	if !b.IsConnected() {
		return ErrClientNotConnected
	}

	startTime := logOperationStart(b.logger, "put_object", objectName, log.KV{
		"bucket_name": b.bucketName,
	})

	// Use upload timeout (MinIO handles multipart automatically)
	ctx, cancel := getContextWithTimeout(b.uploadTimeout, ctx)
	defer cancel()

	// Create MinIO put options
	putOpts := b.basePutOptions()

	// Apply options if provided
	if len(opts) > 0 {
		if err := b.applyMinIOPutOptions(&putOpts, opts[0]); err != nil {
			logOperationEnd(b.logger, "put_object", objectName, startTime, err, log.KV{
				"bucket_name": b.bucketName,
			})
			return err
		}
	}

	// MinIO automatically handles multipart uploads based on size
	_, err := b.minioClient.PutObject(ctx, b.bucketName, objectName, reader, size, putOpts)

	logOperationEnd(b.logger, "put_object", objectName, startTime, err, log.KV{
		"bucket_name": b.bucketName,
	})

	return err
}

// PutObjectStream uploads an object using streaming (no size required)
func (b *Bucket) PutObjectStream(ctx context.Context, objectName string, reader io.Reader, opts ...ObjectOptions) error {
	if !b.IsConnected() {
		return ErrClientNotConnected
	}

	startTime := logOperationStart(b.logger, "put_object_stream", objectName, log.KV{
		"bucket_name": b.bucketName,
	})

	ctx, cancel := getContextWithTimeout(b.uploadTimeout, ctx)
	defer cancel()

	// Create MinIO put options
	putOpts := b.basePutOptions()

	// Apply options if provided
	if len(opts) > 0 {
		if err := b.applyMinIOPutOptions(&putOpts, opts[0]); err != nil {
			logOperationEnd(b.logger, "put_object_stream", objectName, startTime, err, log.KV{
				"bucket_name": b.bucketName,
			})
			return err
		}
	}

	// Use -1 for unknown size streaming uploads
	_, err := b.minioClient.PutObject(ctx, b.bucketName, objectName, reader, -1, putOpts)

	logOperationEnd(b.logger, "put_object_stream", objectName, startTime, err, log.KV{
		"bucket_name": b.bucketName,
	})

	return err
}

// PutObjectMultipart uploads an object using multipart upload with progress tracking
func (b *Bucket) PutObjectMultipart(ctx context.Context, objectName string, reader io.Reader, size int64, opts ...ObjectOptions) error {
	if !b.IsConnected() {
		return ErrClientNotConnected
	}

	startTime := logOperationStart(b.logger, "put_object_multipart", objectName, log.KV{
		"bucket_name": b.bucketName,
	})

	ctx, cancel := getContextWithTimeout(b.uploadTimeout, ctx)
	defer cancel()

	// Create MinIO put options
	putOpts := b.basePutOptions()

	// Apply options if provided
	if len(opts) > 0 {
		if err := b.applyMinIOPutOptions(&putOpts, opts[0]); err != nil {
			logOperationEnd(b.logger, "put_object_multipart", objectName, startTime, err, log.KV{
				"bucket_name": b.bucketName,
			})
			return err
		}
	}

	// Note: MinIO handles multipart uploads automatically
	_, err := b.minioClient.PutObject(ctx, b.bucketName, objectName, reader, size, putOpts)

	logOperationEnd(b.logger, "put_object_multipart", objectName, startTime, err, log.KV{
		"bucket_name": b.bucketName,
	})

	return err
}

// progressReader removed - was causing data corruption issues in multipart uploads

// PutObjectAdvanced provides advanced upload functionality with detailed control
func (b *Bucket) PutObjectAdvanced(ctx context.Context, objectName string, reader io.Reader, size int64, opts UploadOptions) error {
	if !b.IsConnected() {
		return ErrClientNotConnected
	}

	startTime := logOperationStart(b.logger, "put_object_advanced", objectName, log.KV{
		"bucket_name": b.bucketName,
	})

	ctx, cancel := getContextWithTimeout(b.uploadTimeout, ctx)
	defer cancel()

	// Create MinIO put options with advanced settings
	putOpts := b.basePutOptions()

	// Apply basic object options
	if err := b.applyMinIOPutOptions(&putOpts, opts.ObjectOptions); err != nil {
		logOperationEnd(b.logger, "put_object_advanced", objectName, startTime, err, log.KV{
			"bucket_name": b.bucketName,
		})
		return err
	}

	// Override multipart tuning from UploadOptions when provided.
	// Note: MinIO-Go's PutObject does not expose LeavePartsOnError or a per-call
	// MaxUploadParts, so those UploadOptions fields are not applied.
	if opts.Concurrency > 0 {
		putOpts.NumThreads = uint(opts.Concurrency)
	}

	_, err := b.minioClient.PutObject(ctx, b.bucketName, objectName, reader, size, putOpts)

	logOperationEnd(b.logger, "put_object_advanced", objectName, startTime, err, log.KV{
		"bucket_name": b.bucketName,
	})

	return err
}
