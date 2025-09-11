package s3

import (
	"context"
	"io"

	"github.com/minio/minio-go/v7"
	"github.com/oddbit-project/blueprint/log"
)

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
	putOpts := minio.PutObjectOptions{}

	// Apply options if provided
	if len(opts) > 0 {
		b.applyMinIOPutOptions(&putOpts, opts[0])
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
	putOpts := minio.PutObjectOptions{}

	// Apply options if provided
	if len(opts) > 0 {
		b.applyMinIOPutOptions(&putOpts, opts[0])
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
	putOpts := minio.PutObjectOptions{}

	// Apply options if provided
	if len(opts) > 0 {
		b.applyMinIOPutOptions(&putOpts, opts[0])
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
	putOpts := minio.PutObjectOptions{}

	// Apply basic object options
	b.applyMinIOPutOptions(&putOpts, opts.ObjectOptions)

	// Note: MinIO-Go handles multipart uploads automatically
	// Advanced options like LeavePartsOnError, MaxUploadParts, custom Concurrency
	// are not directly supported in MinIO-Go's PutObject method

	_, err := b.minioClient.PutObject(ctx, b.bucketName, objectName, reader, size, putOpts)

	logOperationEnd(b.logger, "put_object_advanced", objectName, startTime, err, log.KV{
		"bucket_name": b.bucketName,
	})

	return err
}
