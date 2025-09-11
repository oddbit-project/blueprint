package s3

import (
	"context"

	"github.com/minio/minio-go/v7"
	"github.com/oddbit-project/blueprint/log"
)

type Bucket struct {
	*Client
	bucketName string
}

func NewBucket(client *Client, bucketName string) (*Bucket, error) {

	return &Bucket{
		Client:     client,
		bucketName: bucketName,
	}, nil
}

// Create attempt to create current bucket
func (b *Bucket) Create(ctx context.Context, opts ...BucketOptions) error {
	if !b.IsConnected() {
		return ErrClientNotConnected
	}

	// Log bucket creation attempt
	startTime := logOperationStart(b.logger, "create_bucket", b.bucketName, log.KV{
		"bucket_name": b.bucketName,
	})

	ctx, cancel := getContextWithTimeout(b.timeout, ctx)
	defer cancel()

	// Determine region from options
	region := b.config.Region
	if len(opts) > 0 && opts[0].Region != "" {
		region = opts[0].Region
	}

	// Check if bucket already exists first
	exists, err := b.minioClient.BucketExists(ctx, b.bucketName)
	if err != nil {
		logOperationEnd(b.logger, "create_bucket", b.bucketName, startTime, err, log.KV{
			"bucket_name": b.bucketName,
			"error":       err.Error(),
		})
		return err
	}

	if exists {
		err := ErrBucketAlreadyExists
		logOperationEnd(b.logger, "create_bucket", b.bucketName, startTime, err, log.KV{
			"bucket_name": b.bucketName,
			"exists":      true,
			"error":       err.Error(),
		})
		return err
	}

	// Create bucket with MinIO client
	options := minio.MakeBucketOptions{
		Region: region,
	}

	err = b.minioClient.MakeBucket(ctx, b.bucketName, options)
	if err != nil {
		logOperationEnd(b.logger, "create_bucket", b.bucketName, startTime, err, log.KV{
			"bucket_name": b.bucketName,
			"error":       err.Error(),
		})
		return err
	}

	// Log successful bucket creation
	logOperationEnd(b.logger, "create_bucket", b.bucketName, startTime, nil, log.KV{
		"bucket_name":    b.bucketName,
		"bucket_created": true,
	})

	return nil
}

// Delete attempt to delete current bucket
func (b *Bucket) Delete(ctx context.Context) error {
	if !b.IsConnected() {
		return ErrClientNotConnected
	}

	// Log bucket deletion attempt
	startTime := logOperationStart(b.logger, "delete_bucket", b.bucketName, log.KV{
		"bucket_name": b.bucketName,
	})

	ctx, cancel := getContextWithTimeout(b.timeout, ctx)
	defer cancel()

	err := b.minioClient.RemoveBucket(ctx, b.bucketName)
	if err != nil {
		logOperationEnd(b.logger, "delete_bucket", b.bucketName, startTime, err, log.KV{
			"bucket_name": b.bucketName,
			"error":       err.Error(),
		})
		return err
	}

	// Log successful bucket deletion
	logOperationEnd(b.logger, "delete_bucket", b.bucketName, startTime, nil, log.KV{
		"bucket_name":    b.bucketName,
		"bucket_deleted": true,
	})
	return nil
}

// Exists check if current bucket exists
func (b *Bucket) Exists(ctx context.Context) (bool, error) {
	if !b.IsConnected() {
		return false, ErrClientNotConnected
	}

	ctx, cancel := getContextWithTimeout(b.timeout, ctx)
	defer cancel()

	exists, err := b.minioClient.BucketExists(ctx, b.bucketName)
	if err != nil {
		return false, err
	}

	return exists, nil
}

func (b *Bucket) Name() string {
	return b.bucketName
}
