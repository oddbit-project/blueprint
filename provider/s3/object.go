package s3

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/oddbit-project/blueprint/log"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
)

// GetObject downloads an object from S3
func (b *Bucket) GetObject(ctx context.Context, name string) (io.ReadCloser, error) {
	if !b.IsConnected() {
		return nil, ErrClientNotConnected
	}

	if err := ValidateObjectName(name); err != nil {
		return nil, err
	}

	ctx, cancel := getContextWithTimeout(b.uploadTimeout, ctx)
	defer cancel()

	result, err := b.s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(b.bucketName),
		Key:    aws.String(name),
	})

	if err != nil {
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) && apiErr.ErrorCode() == "NoSuchKey" {
			return nil, ErrObjectNotFound
		}
		return nil, err
	}

	return result.Body, nil
}

// DeleteObject deletes an object from S3
func (b *Bucket) DeleteObject(ctx context.Context, name string) error {
	if !b.IsConnected() {
		return ErrClientNotConnected
	}

	if err := ValidateObjectName(name); err != nil {
		return err
	}

	startTime := logOperationStart(b.logger, "delete_object", name, log.KV{
		"bucket_name": b.bucketName,
	})

	ctx, cancel := getContextWithTimeout(b.timeout, ctx)
	defer cancel()

	_, err := b.s3Client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(b.bucketName),
		Key:    aws.String(name),
	})

	// Log successful bucket creation
	logOperationEnd(b.logger, "delete_object", name, startTime, err, log.KV{
		"bucket_name": b.bucketName,
	})

	return err
}

// ListObjects lists objects in a bucket
func (b *Bucket) ListObjects(ctx context.Context, opts ...ListOptions) ([]ObjectInfo, error) {
	if !b.IsConnected() {
		return nil, ErrClientNotConnected
	}

	ctx, cancel := getContextWithTimeout(b.timeout, ctx)
	defer cancel()

	input := &s3.ListObjectsV2Input{
		Bucket: aws.String(b.bucketName),
	}

	// Apply options if provided
	if len(opts) > 0 {
		opt := opts[0]
		if opt.Prefix != "" {
			input.Prefix = aws.String(opt.Prefix)
		}
		if opt.Delimiter != "" {
			input.Delimiter = aws.String(opt.Delimiter)
		}
		if opt.MaxKeys > 0 {
			input.MaxKeys = aws.Int32(opt.MaxKeys)
		}
		if opt.StartAfter != "" {
			input.StartAfter = aws.String(opt.StartAfter)
		}
	}

	result, err := b.s3Client.ListObjectsV2(ctx, input)
	if err != nil {
		return nil, err
	}

	objects := make([]ObjectInfo, 0, len(result.Contents))
	for _, obj := range result.Contents {
		info := ObjectInfo{
			Key:  aws.ToString(obj.Key),
			Size: aws.ToInt64(obj.Size),
			ETag: aws.ToString(obj.ETag),
		}

		if obj.LastModified != nil {
			info.LastModified = *obj.LastModified
		}

		if obj.StorageClass != "" {
			info.StorageClass = string(obj.StorageClass)
		}

		objects = append(objects, info)
	}

	return objects, nil
}

// ObjectExists checks if an object exists
func (b *Bucket) ObjectExists(ctx context.Context, name string) (bool, error) {
	if !b.IsConnected() {
		return false, ErrClientNotConnected
	}

	if err := ValidateObjectName(name); err != nil {
		return false, err
	}

	ctx, cancel := getContextWithTimeout(b.timeout, ctx)
	defer cancel()

	_, err := b.s3Client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(b.bucketName),
		Key:    aws.String(name),
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

// HeadObject gets object metadata
func (b *Bucket) HeadObject(ctx context.Context, name string) (*ObjectInfo, error) {
	if !b.IsConnected() {
		return nil, ErrClientNotConnected
	}

	if err := ValidateObjectName(name); err != nil {
		return nil, err
	}

	ctx, cancel := getContextWithTimeout(b.timeout, ctx)
	defer cancel()

	result, err := b.s3Client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(b.bucketName),
		Key:    aws.String(name),
	})

	if err != nil {
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) && apiErr.ErrorCode() == "NotFound" {
			return nil, ErrObjectNotFound
		}
		return nil, err
	}

	info := &ObjectInfo{
		Key:         name,
		Size:        aws.ToInt64(result.ContentLength),
		ETag:        aws.ToString(result.ETag),
		ContentType: aws.ToString(result.ContentType),
	}

	if result.LastModified != nil {
		info.LastModified = *result.LastModified
	}

	if result.StorageClass != "" {
		info.StorageClass = string(result.StorageClass)
	}

	return info, nil
}

// CopyObject copies an object within S3
func (b *Bucket) CopyObject(ctx context.Context, srcName, dstBucket, dstName string, opts ...ObjectOptions) error {
	if !b.IsConnected() {
		return ErrClientNotConnected
	}

	if err := ValidateBucketName(dstBucket); err != nil {
		return err
	}
	if err := ValidateObjectName(srcName); err != nil {
		return err
	}
	if err := ValidateObjectName(dstName); err != nil {
		return err
	}

	ctx, cancel := getContextWithTimeout(b.timeout, ctx)
	defer cancel()

	input := &s3.CopyObjectInput{
		Bucket:     aws.String(dstBucket),
		Key:        aws.String(dstName),
		CopySource: aws.String(b.bucketName + "/" + srcName),
	}

	// Apply options if provided
	if len(opts) > 0 {
		b.applyCopyObjectOptions(input, opts[0])
	}

	_, err := b.s3Client.CopyObject(ctx, input)
	return err
}

// applyObjectOptions applies ObjectOptions to PutObjectInput
func (b *Bucket) applyObjectOptions(input *s3.PutObjectInput, opts ObjectOptions) {
	if opts.ContentType != "" {
		input.ContentType = aws.String(opts.ContentType)
	}
	if opts.CacheControl != "" {
		input.CacheControl = aws.String(opts.CacheControl)
	}
	if opts.ContentDisposition != "" {
		input.ContentDisposition = aws.String(opts.ContentDisposition)
	}
	if opts.ContentEncoding != "" {
		input.ContentEncoding = aws.String(opts.ContentEncoding)
	}
	if opts.ContentLanguage != "" {
		input.ContentLanguage = aws.String(opts.ContentLanguage)
	}
	if opts.StorageClass != "" {
		input.StorageClass = types.StorageClass(opts.StorageClass)
	}
	if len(opts.Metadata) > 0 {
		input.Metadata = opts.Metadata
	}
	if len(opts.Tags) > 0 {
		// Convert tags to URL query string format
		var tagSet []string
		for k, v := range opts.Tags {
			tagSet = append(tagSet, k+"="+v)
		}
		input.Tagging = aws.String(joinTags(tagSet))
	}

	// Apply server-side encryption options after validation
	if opts.ServerSideEncryption != "" || opts.SSECustomerAlgorithm != "" {
		// Validate encryption options before applying
		if err := ValidateEncryptionOptions(opts.ServerSideEncryption, opts.SSEKMSKeyId, opts.SSECustomerKey, opts.SSECustomerAlgorithm); err != nil {
			b.logger.Error(err, "error applying server-side encryption options to object")
			return
		}
	}

	if opts.ServerSideEncryption != "" {
		input.ServerSideEncryption = types.ServerSideEncryption(opts.ServerSideEncryption)
	}
	if opts.SSEKMSKeyId != "" {
		input.SSEKMSKeyId = aws.String(opts.SSEKMSKeyId)
	}
	if len(opts.SSEKMSEncryptionContext) > 0 {
		if contextJSON, err := json.Marshal(opts.SSEKMSEncryptionContext); err == nil {
			input.SSEKMSEncryptionContext = aws.String(string(contextJSON))
		}
	}
	if opts.SSECustomerAlgorithm != "" {
		input.SSECustomerAlgorithm = aws.String(opts.SSECustomerAlgorithm)
	}
	if opts.SSECustomerKey != "" {
		input.SSECustomerKey = aws.String(opts.SSECustomerKey)
	}
	if opts.SSECustomerKeyMD5 != "" {
		input.SSECustomerKeyMD5 = aws.String(opts.SSECustomerKeyMD5)
	}
	if opts.BucketKeyEnabled != nil {
		input.BucketKeyEnabled = opts.BucketKeyEnabled
	}
}

// applyCopyObjectOptions applies ObjectOptions to CopyObjectInput
func (b *Bucket) applyCopyObjectOptions(input *s3.CopyObjectInput, opts ObjectOptions) {
	if opts.ContentType != "" {
		input.ContentType = aws.String(opts.ContentType)
		input.MetadataDirective = types.MetadataDirectiveReplace
	}
	if opts.CacheControl != "" {
		input.CacheControl = aws.String(opts.CacheControl)
		input.MetadataDirective = types.MetadataDirectiveReplace
	}
	if opts.ContentDisposition != "" {
		input.ContentDisposition = aws.String(opts.ContentDisposition)
		input.MetadataDirective = types.MetadataDirectiveReplace
	}
	if opts.ContentEncoding != "" {
		input.ContentEncoding = aws.String(opts.ContentEncoding)
		input.MetadataDirective = types.MetadataDirectiveReplace
	}
	if opts.ContentLanguage != "" {
		input.ContentLanguage = aws.String(opts.ContentLanguage)
		input.MetadataDirective = types.MetadataDirectiveReplace
	}
	if opts.StorageClass != "" {
		input.StorageClass = types.StorageClass(opts.StorageClass)
	}
	if len(opts.Metadata) > 0 {
		input.Metadata = opts.Metadata
		input.MetadataDirective = types.MetadataDirectiveReplace
	}
	if len(opts.Tags) > 0 {
		var tagSet []string
		for k, v := range opts.Tags {
			tagSet = append(tagSet, k+"="+v)
		}
		input.Tagging = aws.String(joinTags(tagSet))
		input.TaggingDirective = types.TaggingDirectiveReplace
	}

	// Apply server-side encryption options for destination object
	if opts.ServerSideEncryption != "" {
		input.ServerSideEncryption = types.ServerSideEncryption(opts.ServerSideEncryption)
	}
	if opts.SSEKMSKeyId != "" {
		input.SSEKMSKeyId = aws.String(opts.SSEKMSKeyId)
	}
	if len(opts.SSEKMSEncryptionContext) > 0 {
		if contextJSON, err := json.Marshal(opts.SSEKMSEncryptionContext); err == nil {
			input.SSEKMSEncryptionContext = aws.String(string(contextJSON))
		}
	}
	if opts.SSECustomerAlgorithm != "" {
		input.SSECustomerAlgorithm = aws.String(opts.SSECustomerAlgorithm)
	}
	if opts.SSECustomerKey != "" {
		input.SSECustomerKey = aws.String(opts.SSECustomerKey)
	}
	if opts.SSECustomerKeyMD5 != "" {
		input.SSECustomerKeyMD5 = aws.String(opts.SSECustomerKeyMD5)
	}
	if opts.BucketKeyEnabled != nil {
		input.BucketKeyEnabled = opts.BucketKeyEnabled
	}
}

// joinTags joins tag key-value pairs with "&"
func joinTags(tags []string) string {
	result := ""
	for i, tag := range tags {
		if i > 0 {
			result += "&"
		}
		result += tag
	}
	return result
}
