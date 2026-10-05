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
func (b *Bucket) GetObject(ctx context.Context, name string, versionID ...string) (io.ReadCloser, error) {
	if !b.IsConnected() {
		return nil, ErrClientNotConnected
	}

	getOpts := minio.GetObjectOptions{}
	if len(versionID) > 0 {
		getOpts.VersionID = versionID[0]
	}

	// Don't use getContextWithTimeout here - let the caller manage the context
	// This prevents the context from being canceled while the reader is still being used
	obj, err := b.minioClient.GetObject(ctx, b.bucketName, name, getOpts)
	if err != nil {
		return nil, err
	}

	return obj, nil
}

// DeleteObject deletes an object from S3.
// On versioned/object-lock buckets, a delete without a VersionID writes a delete
// marker; pass DeleteOptions to target a specific version or bypass governance retention.
func (b *Bucket) DeleteObject(ctx context.Context, name string, opts ...DeleteOptions) error {
	if !b.IsConnected() {
		return ErrClientNotConnected
	}

	startTime := logOperationStart(b.logger, "delete_object", name, log.KV{
		"bucket_name": b.bucketName,
	})

	ctx, cancel := getContextWithTimeout(b.timeout, ctx)
	defer cancel()

	removeOpts := minio.RemoveObjectOptions{}
	if len(opts) > 0 {
		removeOpts.VersionID = opts[0].VersionID
		removeOpts.GovernanceBypass = opts[0].GovernanceBypass
		removeOpts.ForceDelete = opts[0].ForceDelete
	}

	err := b.minioClient.RemoveObject(ctx, b.bucketName, name, removeOpts)

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
		// When Versions is set, each object version (and delete marker) is a
		// separate entry, so MaxKeys bounds the number of versions, not keys.
		listOpts.WithVersions = opt.Versions
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
	maxKeys := int32(0)
	if len(opts) > 0 && opts[0].MaxKeys > 0 {
		maxKeys = opts[0].MaxKeys
	}

	count := int32(0)
	for objInfo := range objectCh {
		if objInfo.Err != nil {
			return nil, objInfo.Err
		}

		info := ObjectInfo{
			Key:            objInfo.Key,
			Size:           objInfo.Size,
			ETag:           objInfo.ETag,
			LastModified:   objInfo.LastModified,
			StorageClass:   objInfo.StorageClass,
			ContentType:    objInfo.ContentType,
			VersionID:      objInfo.VersionID,
			IsLatest:       objInfo.IsLatest,
			IsDeleteMarker: objInfo.IsDeleteMarker,
		}

		objects = append(objects, info)

		// Check if we've reached the maximum number of keys
		if maxKeys > 0 {
			count++
			if count >= maxKeys {
				break
			}
		}
	}

	return objects, nil
}

// ObjectExists checks if an object exists
func (b *Bucket) ObjectExists(ctx context.Context, name string) (bool, error) {
	if !b.IsConnected() {
		return false, ErrClientNotConnected
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
func (b *Bucket) HeadObject(ctx context.Context, name string, versionID ...string) (*ObjectInfo, error) {
	if !b.IsConnected() {
		return nil, ErrClientNotConnected
	}

	ctx, cancel := getContextWithTimeout(b.timeout, ctx)
	defer cancel()

	statOpts := minio.StatObjectOptions{}
	if len(versionID) > 0 {
		statOpts.VersionID = versionID[0]
	}

	result, err := b.minioClient.StatObject(ctx, b.bucketName, name, statOpts)
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
		VersionID:    result.VersionID,
	}

	return info, nil
}

// CopyObject copies an object within S3
func (b *Bucket) CopyObject(ctx context.Context, srcName, dstBucket, dstName string, opts ...ObjectOptions) error {
	_, err := b.CopyObjectVersion(ctx, CopySource{Name: srcName}, dstBucket, dstName, opts...)
	return err
}

// CopyObjectVersion copies an object (optionally a specific version) and returns the version it created.
// Copies are single-request (5 GiB source limit on AWS).
func (b *Bucket) CopyObjectVersion(ctx context.Context, srcObj CopySource, dstBucket, dstName string, opts ...ObjectOptions) (ObjectVersion, error) {
	if !b.IsConnected() {
		return ObjectVersion{}, ErrClientNotConnected
	}

	startTime := logOperationStart(b.logger, "copy_object", srcObj.Name, log.KV{
		"bucket_name":    b.bucketName,
		"dst_bucket":     dstBucket,
		"dst_name":       dstName,
		"src_version_id": srcObj.VersionID,
	})

	ctx, cancel := getContextWithTimeout(b.timeout, ctx)
	defer cancel()

	// Create copy source
	src := minio.CopySrcOptions{
		Bucket:    b.bucketName,
		Object:    srcObj.Name,
		VersionID: srcObj.VersionID,
	}

	// Create copy destination options
	dst := minio.CopyDestOptions{
		Bucket: dstBucket,
		Object: dstName,
	}

	endKV := log.KV{
		"bucket_name": b.bucketName,
		"dst_bucket":  dstBucket,
		"dst_name":    dstName,
	}

	// Apply options if provided
	if len(opts) > 0 {
		// Decryption key for an SSE-C encrypted source object
		if opts[0].SourceSSECustomerKey != "" {
			srcSSE, err := sseCustomerKey(opts[0].SourceSSECustomerKey)
			if err != nil {
				logOperationEnd(b.logger, "copy_object", srcObj.Name, startTime, err, endKV)
				return ObjectVersion{}, err
			}
			src.Encryption = srcSSE
		}
		if err := b.applyMinIOCopyOptions(&dst, opts[0]); err != nil {
			logOperationEnd(b.logger, "copy_object", srcObj.Name, startTime, err, endKV)
			return ObjectVersion{}, err
		}
	}

	info, err := b.minioClient.CopyObject(ctx, dst, src)
	// S3 may answer 200 OK with an error body; minio-go then yields an empty ETag
	if err == nil && info.ETag == "" {
		err = ErrEmptyCopyResult
	}
	if info.VersionID != "" {
		endKV["version_id"] = info.VersionID
	}
	logOperationEnd(b.logger, "copy_object", srcObj.Name, startTime, err, endKV)
	if err != nil {
		return ObjectVersion{}, err
	}

	return ObjectVersion{
		Bucket:    dstBucket,
		Key:       dstName,
		VersionID: info.VersionID,
		ETag:      info.ETag,
		Size:      -1,
	}, nil
}

// applyMinIOPutOptions applies ObjectOptions to MinIO PutObjectOptions
func (b *Bucket) applyMinIOPutOptions(opts *minio.PutObjectOptions, objectOpts ObjectOptions) error {
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
	if objectOpts.LockMode != "" {
		opts.Mode = minio.RetentionMode(objectOpts.LockMode)
	}
	if !objectOpts.RetainUntilDate.IsZero() {
		opts.RetainUntilDate = objectOpts.RetainUntilDate
	}
	if objectOpts.LegalHold != "" {
		opts.LegalHold = minio.LegalHoldStatus(objectOpts.LegalHold)
	}

	// Server-side encryption (SSE-S3, SSE-KMS, SSE-C)
	sse, err := serverSideEncryption(objectOpts)
	if err != nil {
		return err
	}
	if sse != nil {
		opts.ServerSideEncryption = sse
	}

	return nil
}

// applyMinIOCopyOptions applies ObjectOptions to MinIO CopyDestOptions
func (b *Bucket) applyMinIOCopyOptions(dst *minio.CopyDestOptions, objectOpts ObjectOptions) error {
	// Initialize metadata if needed
	if dst.UserMetadata == nil {
		dst.UserMetadata = make(map[string]string)
	}

	if objectOpts.ContentType != "" {
		dst.UserMetadata["Content-Type"] = objectOpts.ContentType
		dst.ReplaceMetadata = true
	}
	if len(objectOpts.Metadata) > 0 {
		// Merge metadata
		for k, v := range objectOpts.Metadata {
			dst.UserMetadata[k] = v
		}
		dst.ReplaceMetadata = true
	}
	if len(objectOpts.Tags) > 0 {
		dst.UserTags = objectOpts.Tags
		dst.ReplaceTags = true
	}

	if objectOpts.LockMode != "" {
		dst.Mode = minio.RetentionMode(objectOpts.LockMode)
	}
	if !objectOpts.RetainUntilDate.IsZero() {
		dst.RetainUntilDate = objectOpts.RetainUntilDate
	}
	if objectOpts.LegalHold != "" {
		dst.LegalHold = minio.LegalHoldStatus(objectOpts.LegalHold)
	}

	// Server-side encryption for the destination object
	sse, err := serverSideEncryption(objectOpts)
	if err != nil {
		return err
	}
	if sse != nil {
		dst.Encryption = sse
	}

	// Note: MinIO CopyDestOptions doesn't have StorageClass field
	// Storage class would need to be handled differently
	return nil
}
