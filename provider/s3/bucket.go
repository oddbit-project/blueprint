package s3

import (
	"context"
	"errors"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	"github.com/oddbit-project/blueprint/log"
)

type Bucket struct {
	*Client
	bucketName string
}

func NewBucket(client *Client, bucketName string, logger *log.Logger) (*Bucket, error) {
	if err := ValidateBucketName(bucketName); err != nil {
		return nil, err
	}

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

	input := &s3.CreateBucketInput{
		Bucket: aws.String(b.bucketName),
	}

	// Apply options if provided
	if len(opts) > 0 {
		opt := opts[0]

		// Set region-specific configuration
		if opt.Region != "" && opt.Region != "us-east-1" {
			input.CreateBucketConfiguration = &types.CreateBucketConfiguration{
				LocationConstraint: types.BucketLocationConstraint(opt.Region),
			}
		}

		// Set ACL if provided
		if opt.ACL != "" {
			input.ACL = types.BucketCannedACL(opt.ACL)
		}
	}

	_, err := b.s3Client.CreateBucket(ctx, input)
	if err != nil {
		logOperationEnd(b.logger, "create_bucket", b.bucketName, startTime, err, log.KV{
			"bucket_name":    b.bucketName,
			"error_type":     "bucket_already_exists",
			"aws_error_code": err.Error(),
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

	if err := ValidateBucketName(b.bucketName); err != nil {
		return err
	}

	// Log bucket creation attempt
	startTime := logOperationStart(b.logger, "delete_bucket", b.bucketName, log.KV{
		"bucket_name": b.bucketName,
	})

	ctx, cancel := getContextWithTimeout(b.timeout, ctx)
	defer cancel()

	_, err := b.s3Client.DeleteBucket(ctx, &s3.DeleteBucketInput{
		Bucket: aws.String(b.bucketName),
	})

	if err != nil {
		var apiErr smithy.APIError
		logOperationEnd(b.logger, "delete_bucket", b.bucketName, startTime, err, log.KV{
			"bucket_name": b.bucketName,
			"error":       err.Error(),
		})
		if errors.As(err, &apiErr) && apiErr.ErrorCode() == "NoSuchBucket" {
			return ErrBucketNotFound
		}
		return err
	}

	// Log successful bucket creation
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

	if err := ValidateBucketName(b.bucketName); err != nil {
		return false, err
	}

	ctx, cancel := getContextWithTimeout(b.timeout, ctx)
	defer cancel()

	_, err := b.s3Client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(b.bucketName),
	})

	if err != nil {
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) && apiErr.ErrorCode() == "NotFound" {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

func (b *Bucket) Name() string {
	return b.bucketName
}
