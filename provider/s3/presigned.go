package s3

import (
	"context"
	"net/url"
	"time"

	"github.com/oddbit-project/blueprint/utils"
)

// PresignGetObject generates a pre-signed URL for downloading an object
func (b *Bucket) PresignGetObject(ctx context.Context, objectName string, expiry time.Duration) (string, error) {
	if !b.IsConnected() {
		return "", ErrClientNotConnected
	}

	if err := ValidateObjectName(objectName); err != nil {
		return "", err
	}

	// Generate presigned GET URL using MinIO client
	presignedURL, err := b.minioClient.PresignedGetObject(ctx, b.bucketName, objectName, expiry, url.Values{})
	if err != nil {
		return "", err
	}

	return presignedURL.String(), nil
}

// PresignPutObject generates a pre-signed URL for uploading an object
func (b *Bucket) PresignPutObject(ctx context.Context, objectName string, expiry time.Duration, opts ...ObjectOptions) (string, error) {
	if !b.IsConnected() {
		return "", ErrClientNotConnected
	}

	if err := ValidateObjectName(objectName); err != nil {
		return "", err
	}

	// Generate presigned PUT URL using MinIO client
	// Note: MinIO-Go PresignedPutObject doesn't support the same options as AWS SDK
	// For simplicity, we'll use basic presigned PUT without advanced options
	presignedURL, err := b.minioClient.PresignedPutObject(ctx, b.bucketName, objectName, expiry)
	if err != nil {
		return "", err
	}

	return presignedURL.String(), nil
}

// PresignDeleteObject generates a pre-signed URL for deleting an object
// Note: MinIO-Go does not support presigned DELETE URLs
func (b *Bucket) PresignDeleteObject(ctx context.Context, objectName string, expiry time.Duration) (string, error) {
	if !b.IsConnected() {
		return "", ErrClientNotConnected
	}

	if err := ValidateObjectName(objectName); err != nil {
		return "", err
	}

	// MinIO-Go does not support presigned DELETE URLs
	return "", utils.Error("presigned DELETE URLs are not supported by MinIO-Go client")
}

// PresignHeadObject generates a pre-signed URL for getting object metadata
func (b *Bucket) PresignHeadObject(ctx context.Context, objectName string, expiry time.Duration) (string, error) {
	if !b.IsConnected() {
		return "", ErrClientNotConnected
	}

	if err := ValidateObjectName(objectName); err != nil {
		return "", err
	}

	// Generate presigned HEAD URL using MinIO client
	// MinIO-Go supports presigned HEAD URLs via PresignedHeadObject
	presignedURL, err := b.minioClient.PresignedHeadObject(ctx, b.bucketName, objectName, expiry, url.Values{})
	if err != nil {
		return "", err
	}

	return presignedURL.String(), nil
}

// Note: Advanced presigned URL methods (PresignGetObjectAdvanced, PresignPutObjectAdvanced, PresignPostObject)
// have been removed in the MinIO-Go conversion as they provide AWS SDK-specific details
// that are not available in MinIO-Go. Use the basic presigned methods instead.
