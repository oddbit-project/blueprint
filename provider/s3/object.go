package s3

import (
	"context"
	"io"

	"github.com/minio/minio-go/v7"
	"github.com/oddbit-project/blueprint/log"
)

// GetObject downloads an object from S3
// Note: no object is actually transfered; it returns a ReadCloser that will perform the
// read;
// if name does not exist, no error is returned; use ObjectExists()/StatObject() instead to check for existence
func (b *Bucket) GetObject(ctx context.Context, name string) (io.ReadCloser, error) {
	if !b.IsConnected() {
		return nil, ErrClientNotConnected
	}

	if err := ValidateObjectName(name); err != nil {
		return nil, err
	}

	// Don't use getContextWithTimeout here - let the caller manage the context
	// This prevents the context from being canceled while the reader is still being used
	obj, err := b.minioClient.GetObject(ctx, b.bucketName, name, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}

	return obj, nil
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

	err := b.minioClient.RemoveObject(ctx, b.bucketName, name, minio.RemoveObjectOptions{})

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

	// Create MinIO list options
	listOpts := minio.ListObjectsOptions{
		Recursive: true,
	}

	// Apply options if provided
	if len(opts) > 0 {
		opt := opts[0]
		if opt.Prefix != "" {
			listOpts.Prefix = opt.Prefix
		}
		if opt.MaxKeys > 0 {
			listOpts.MaxKeys = int(opt.MaxKeys)
		}
		if opt.StartAfter != "" {
			listOpts.StartAfter = opt.StartAfter
		}
		// Note: MinIO-Go doesn't have direct delimiter support in the same way
		if opt.Delimiter != "" {
			listOpts.Recursive = false // Use non-recursive for delimiter-like behavior
		}
	}

	// List objects using MinIO client
	objectCh := b.minioClient.ListObjects(ctx, b.bucketName, listOpts)

	var objects []ObjectInfo
	for objInfo := range objectCh {
		if objInfo.Err != nil {
			return nil, objInfo.Err
		}

		info := ObjectInfo{
			Key:          objInfo.Key,
			Size:         objInfo.Size,
			ETag:         objInfo.ETag,
			LastModified: objInfo.LastModified,
			StorageClass: objInfo.StorageClass,
			ContentType:  objInfo.ContentType,
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

	_, err := b.minioClient.StatObject(ctx, b.bucketName, name, minio.StatObjectOptions{})
	if err != nil {
		// MinIO returns specific error for object not found
		if minio.ToErrorResponse(err).Code == "NoSuchKey" {
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

	result, err := b.minioClient.StatObject(ctx, b.bucketName, name, minio.StatObjectOptions{})
	if err != nil {
		return nil, err
	}

	info := &ObjectInfo{
		Key:          name,
		Size:         result.Size,
		ETag:         result.ETag,
		ContentType:  result.ContentType,
		LastModified: result.LastModified,
		StorageClass: result.StorageClass,
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

	// Create copy source
	src := minio.CopySrcOptions{
		Bucket: b.bucketName,
		Object: srcName,
	}

	// Create copy destination options
	dst := minio.CopyDestOptions{
		Bucket: dstBucket,
		Object: dstName,
	}

	// Apply options if provided
	if len(opts) > 0 {
		b.applyMinIOCopyOptions(&dst, opts[0])
	}

	_, err := b.minioClient.CopyObject(ctx, dst, src)
	return err
}

// applyMinIOPutOptions applies ObjectOptions to MinIO PutObjectOptions
func (b *Bucket) applyMinIOPutOptions(opts *minio.PutObjectOptions, objectOpts ObjectOptions) {
	if objectOpts.ContentType != "" {
		opts.ContentType = objectOpts.ContentType
	}
	if objectOpts.CacheControl != "" {
		opts.CacheControl = objectOpts.CacheControl
	}
	if objectOpts.ContentDisposition != "" {
		opts.ContentDisposition = objectOpts.ContentDisposition
	}
	if objectOpts.ContentEncoding != "" {
		opts.ContentEncoding = objectOpts.ContentEncoding
	}
	if objectOpts.ContentLanguage != "" {
		opts.ContentLanguage = objectOpts.ContentLanguage
	}
	if objectOpts.StorageClass != "" {
		opts.StorageClass = objectOpts.StorageClass
	}
	if len(objectOpts.Metadata) > 0 {
		opts.UserMetadata = objectOpts.Metadata
	}
	if len(objectOpts.Tags) > 0 {
		opts.UserTags = objectOpts.Tags
	}

	// Note: MinIO encryption support would require using encrypt package
	// For now, we'll skip encryption options as they require more complex setup
}

// applyMinIOCopyOptions applies ObjectOptions to MinIO CopyDestOptions
func (b *Bucket) applyMinIOCopyOptions(dst *minio.CopyDestOptions, objectOpts ObjectOptions) {
	if objectOpts.ContentType != "" {
		// Create metadata map if needed
		if dst.UserMetadata == nil {
			dst.UserMetadata = make(map[string]string)
		}
		dst.UserMetadata["Content-Type"] = objectOpts.ContentType
		dst.ReplaceMetadata = true
	}
	if len(objectOpts.Metadata) > 0 {
		dst.UserMetadata = objectOpts.Metadata
		dst.ReplaceMetadata = true
	}
	if len(objectOpts.Tags) > 0 {
		dst.UserTags = objectOpts.Tags
		dst.ReplaceTags = true
	}
	// Note: MinIO CopyDestOptions doesn't have StorageClass field
	// Storage class would need to be handled differently
}
