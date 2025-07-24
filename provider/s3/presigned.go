package s3

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// PresignGetObject generates a pre-signed URL for downloading an object
func (b *Bucket) PresignGetObject(ctx context.Context, objectName string, expiry time.Duration) (string, error) {
	if !b.IsConnected() {
		return "", ErrClientNotConnected
	}

	if err := ValidateObjectName(objectName); err != nil {
		return "", err
	}

	// Create a presign client
	presignClient := s3.NewPresignClient(b.s3Client)

	// Create the request
	request, err := presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(b.bucketName),
		Key:    aws.String(objectName),
	}, func(opts *s3.PresignOptions) {
		opts.Expires = expiry
	})

	if err != nil {
		return "", err
	}

	return request.URL, nil
}

// PresignPutObject generates a pre-signed URL for uploading an object
func (b *Bucket) PresignPutObject(ctx context.Context, objectName string, expiry time.Duration, opts ...ObjectOptions) (string, error) {
	if !b.IsConnected() {
		return "", ErrClientNotConnected
	}

	if err := ValidateObjectName(objectName); err != nil {
		return "", err
	}

	// Create a presign client
	presignClient := s3.NewPresignClient(b.s3Client)

	input := &s3.PutObjectInput{
		Bucket: aws.String(b.bucketName),
		Key:    aws.String(objectName),
	}

	// Apply options if provided
	if len(opts) > 0 {
		b.applyPresignObjectOptions(input, opts[0])
	}

	// Create the request
	request, err := presignClient.PresignPutObject(ctx, input, func(options *s3.PresignOptions) {
		options.Expires = expiry
	})

	if err != nil {
		return "", err
	}

	return request.URL, nil
}

// PresignDeleteObject generates a pre-signed URL for deleting an object
func (b *Bucket) PresignDeleteObject(ctx context.Context, objectName string, expiry time.Duration) (string, error) {
	if !b.IsConnected() {
		return "", ErrClientNotConnected
	}

	if err := ValidateObjectName(objectName); err != nil {
		return "", err
	}

	// Create a presign client
	presignClient := s3.NewPresignClient(b.s3Client)

	// Create the request
	request, err := presignClient.PresignDeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(b.bucketName),
		Key:    aws.String(objectName),
	}, func(opts *s3.PresignOptions) {
		opts.Expires = expiry
	})

	if err != nil {
		return "", err
	}

	return request.URL, nil
}

// PresignHeadObject generates a pre-signed URL for getting object metadata
func (b *Bucket) PresignHeadObject(ctx context.Context, objectName string, expiry time.Duration) (string, error) {
	if !b.IsConnected() {
		return "", ErrClientNotConnected
	}

	if err := ValidateObjectName(objectName); err != nil {
		return "", err
	}

	// Create a presign client
	presignClient := s3.NewPresignClient(b.s3Client)

	// Create the request
	request, err := presignClient.PresignHeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(b.bucketName),
		Key:    aws.String(objectName),
	}, func(opts *s3.PresignOptions) {
		opts.Expires = expiry
	})

	if err != nil {
		return "", err
	}

	return request.URL, nil
}

// PresignedURLInfo contains information about a pre-signed URL
type PresignedURLInfo struct {
	URL           string            // The pre-signed URL
	Method        string            // HTTP method (GET, PUT, DELETE, HEAD)
	Headers       map[string]string // Required headers to include with the request
	ExpiresAt     time.Time         // When the URL expires
	SignedHeaders []string          // List of headers that are part of the signature
}

// PresignGetObjectAdvanced generates a pre-signed URL with additional information
func (b *Bucket) PresignGetObjectAdvanced(ctx context.Context, objectName string, expiry time.Duration) (*PresignedURLInfo, error) {
	if !b.IsConnected() {
		return nil, ErrClientNotConnected
	}

	if err := ValidateObjectName(objectName); err != nil {
		return nil, err
	}

	// Create a presign client
	presignClient := s3.NewPresignClient(b.s3Client)

	// Create the request
	request, err := presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(b.bucketName),
		Key:    aws.String(objectName),
	}, func(opts *s3.PresignOptions) {
		opts.Expires = expiry
	})

	if err != nil {
		return nil, err
	}

	info := &PresignedURLInfo{
		URL:           request.URL,
		Method:        request.Method,
		Headers:       make(map[string]string),
		ExpiresAt:     time.Now().Add(expiry),
		SignedHeaders: make([]string, 0),
	}

	// Copy headers
	for k, v := range request.SignedHeader {
		if len(v) > 0 {
			info.Headers[k] = v[0]
			info.SignedHeaders = append(info.SignedHeaders, k)
		}
	}

	return info, nil
}

// PresignPutObjectAdvanced generates a pre-signed PUT URL with additional information
func (b *Bucket) PresignPutObjectAdvanced(ctx context.Context, objectName string, expiry time.Duration, opts ...ObjectOptions) (*PresignedURLInfo, error) {
	if !b.IsConnected() {
		return nil, ErrClientNotConnected
	}

	if err := ValidateObjectName(objectName); err != nil {
		return nil, err
	}

	// Create a presign client
	presignClient := s3.NewPresignClient(b.s3Client)

	input := &s3.PutObjectInput{
		Bucket: aws.String(b.bucketName),
		Key:    aws.String(objectName),
	}

	// Apply options if provided
	if len(opts) > 0 {
		b.applyPresignObjectOptions(input, opts[0])
	}

	// Create the request
	request, err := presignClient.PresignPutObject(ctx, input, func(options *s3.PresignOptions) {
		options.Expires = expiry
	})

	if err != nil {
		return nil, err
	}

	info := &PresignedURLInfo{
		URL:           request.URL,
		Method:        request.Method,
		Headers:       make(map[string]string),
		ExpiresAt:     time.Now().Add(expiry),
		SignedHeaders: make([]string, 0),
	}

	// Copy headers
	for k, v := range request.SignedHeader {
		if len(v) > 0 {
			info.Headers[k] = v[0]
			info.SignedHeaders = append(info.SignedHeaders, k)
		}
	}

	return info, nil
}

// applyPresignObjectOptions applies ObjectOptions to PutObjectInput for pre-signed URLs
func (b *Bucket) applyPresignObjectOptions(input *s3.PutObjectInput, opts ObjectOptions) {
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
	// Note: Tags are not supported in pre-signed URLs for security reasons
}

// PresignPostObject generates form fields for browser-based uploads
func (b *Bucket) PresignPostObject(ctx context.Context, objectName string, expiry time.Duration, opts ...ObjectOptions) (*s3.PresignedPostRequest, error) {
	if !b.IsConnected() {
		return nil, ErrClientNotConnected
	}

	if err := ValidateObjectName(objectName); err != nil {
		return nil, err
	}

	// Create a presign client
	presignClient := s3.NewPresignClient(b.s3Client)

	input := &s3.PutObjectInput{
		Bucket: aws.String(b.bucketName),
		Key:    aws.String(objectName),
	}

	// Apply options if provided
	if len(opts) > 0 {
		b.applyPresignObjectOptions(input, opts[0])
	}

	// Create the POST presigned request
	request, err := presignClient.PresignPostObject(ctx, input, func(opts *s3.PresignPostOptions) {
		opts.Expires = expiry
	})

	if err != nil {
		return nil, err
	}

	return request, nil
}
