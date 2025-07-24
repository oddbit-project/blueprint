package s3

import (
	"context"
	"github.com/oddbit-project/blueprint/log"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// PutObject uploads an object to S3
func (b *Bucket) PutObject(ctx context.Context, objectName string, reader io.Reader, size int64, opts ...ObjectOptions) error {
	if !b.IsConnected() {
		return ErrClientNotConnected
	}

	if err := ValidateObjectName(objectName); err != nil {
		return err
	}

	startTime := logOperationStart(b.logger, "put_object", objectName, log.KV{
		"bucket_name": b.bucketName,
	})

	// Use upload timeout for large files, regular timeout for small ones
	var cancel context.CancelFunc
	if size >= b.config.MultipartThreshold {
		ctx, cancel = getContextWithTimeout(b.uploadTimeout, ctx)
		defer cancel()

		err := b.PutObjectMultipart(ctx, objectName, reader, size, nil, opts...)
		logOperationEnd(b.logger, "put_object", objectName, startTime, err, log.KV{
			"bucket_name": b.bucketName,
		})

		return err
	} else {
		ctx, cancel = getContextWithTimeout(b.timeout, ctx)
		defer cancel()
	}

	input := &s3.PutObjectInput{
		Bucket: aws.String(b.Name()),
		Key:    aws.String(objectName),
		Body:   reader,
	}

	if size > 0 {
		input.ContentLength = aws.Int64(size)
	}

	// Apply options if provided
	if len(opts) > 0 {
		b.applyObjectOptions(input, opts[0])
	}

	_, err := b.s3Client.PutObject(ctx, input)

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

	if err := ValidateObjectName(objectName); err != nil {
		return err
	}

	startTime := logOperationStart(b.logger, "put_object_stream", objectName, log.KV{
		"bucket_name": b.bucketName,
	})

	ctx, cancel := getContextWithTimeout(b.uploadTimeout, ctx)
	defer cancel()

	input := &s3.PutObjectInput{
		Bucket: aws.String(b.bucketName),
		Key:    aws.String(objectName),
		Body:   reader,
	}

	// Apply options if provided
	if len(opts) > 0 {
		b.applyObjectOptions(input, opts[0])
	}

	// Use the uploader for streaming uploads
	_, err := b.uploader.Upload(ctx, input)

	logOperationEnd(b.logger, "put_object_stream", objectName, startTime, err, log.KV{
		"bucket_name": b.bucketName,
	})

	return err
}

// PutObjectMultipart uploads an object using multipart upload with progress tracking
func (b *Bucket) PutObjectMultipart(ctx context.Context, objectName string, reader io.Reader, size int64, progress ProgressCallback, opts ...ObjectOptions) error {
	if !b.IsConnected() {
		return ErrClientNotConnected
	}

	if err := ValidateObjectName(objectName); err != nil {
		return err
	}

	startTime := logOperationStart(b.logger, "put_object_multipart", objectName, log.KV{
		"bucket_name": b.bucketName,
	})

	ctx, cancel := getContextWithTimeout(b.uploadTimeout, ctx)
	defer cancel()

	input := &s3.PutObjectInput{
		Bucket: aws.String(b.bucketName),
		Key:    aws.String(objectName),
		Body:   reader,
	}

	if size > 0 {
		input.ContentLength = aws.Int64(size)
	}

	// Apply options if provided
	if len(opts) > 0 {
		b.applyObjectOptions(input, opts[0])
	}

	// Create a progress reader if callback is provided
	if progress != nil {
		totalParts := int(size / b.config.PartSize)
		if size%b.config.PartSize != 0 {
			totalParts++
		}

		// Wrap the reader with progress tracking
		reader = &progressReader{
			reader:     reader,
			totalSize:  size,
			callback:   progress,
			totalParts: totalParts,
		}
		input.Body = reader
	}

	// Use the uploader for multipart uploads
	_, err := b.uploader.Upload(ctx, input)

	logOperationEnd(b.logger, "put_object_multipart", objectName, startTime, err, log.KV{
		"bucket_name": b.bucketName,
	})

	return err
}

// progressReader wraps an io.Reader to track upload progress
type progressReader struct {
	reader       io.Reader
	totalSize    int64
	uploaded     int64
	callback     ProgressCallback
	totalParts   int
	partCount    int
	lastReported int64 // Track last reported progress to avoid duplicates
}

// Read implements io.Reader and tracks progress
func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.reader.Read(p)
	if n > 0 {
		pr.uploaded += int64(n)

		// Estimate parts completed based on bytes uploaded
		if pr.totalParts > 1 {
			// For multipart uploads
			partsCompleted := int(pr.uploaded / (pr.totalSize / int64(pr.totalParts)))
			if partsCompleted > pr.partCount {
				pr.partCount = partsCompleted
			}
		} else {
			// For single-part uploads
			if pr.uploaded >= pr.totalSize {
				pr.partCount = 1
			}
		}

		// Only report progress if we've made significant progress or completed
		// This helps reduce duplicate progress calls and prevents issues with concurrent reads
		progressThreshold := pr.totalSize / 100 // Report every 1% progress
		if progressThreshold < 1024 {
			progressThreshold = 1024 // Minimum threshold of 1KB
		}

		if pr.callback != nil && (pr.uploaded-pr.lastReported >= progressThreshold || pr.uploaded >= pr.totalSize) {
			pr.lastReported = pr.uploaded
			pr.callback(UploadProgress{
				BytesUploaded: pr.uploaded,
				TotalBytes:    pr.totalSize,
				PartsUploaded: pr.partCount,
				TotalParts:    pr.totalParts,
			})
		}
	}
	return n, err
}

// PutObjectAdvanced provides advanced upload functionality with detailed control
func (b *Bucket) PutObjectAdvanced(ctx context.Context, objectName string, reader io.Reader, size int64, progress ProgressCallback, opts UploadOptions) error {
	if !b.IsConnected() {
		return ErrClientNotConnected
	}

	if err := ValidateObjectName(objectName); err != nil {
		return err
	}

	startTime := logOperationStart(b.logger, "put_object_advanced", objectName, log.KV{
		"bucket_name": b.bucketName,
	})

	ctx, cancel := getContextWithTimeout(b.uploadTimeout, ctx)
	defer cancel()

	// Determine if this should be a multipart upload based on PartSize
	// For small files, force single-part upload to avoid concurrent read issues
	useMultipart := size >= b.config.PartSize

	// Create a custom uploader with advanced options
	uploader := manager.NewUploader(b.s3Client, func(u *manager.Uploader) {
		u.PartSize = b.config.PartSize
		u.Concurrency = b.config.Concurrency
		u.LeavePartsOnError = opts.LeavePartsOnError

		if opts.MaxUploadParts > 0 {
			u.MaxUploadParts = int32(opts.MaxUploadParts)
		}

		// Handle concurrency carefully to prevent data duplication issues with AWS SDK
		if useMultipart && opts.Concurrency > 0 {
			// Only use custom concurrency for large files that will be multipart
			u.Concurrency = opts.Concurrency
		} else {
			// Force single concurrency for small files to prevent AWS SDK data duplication bug
			// The AWS SDK v2 uploader has issues with concurrent reads on small files
			u.Concurrency = 1
		}

		// Note: AWS SDK v2 automatically handles multipart threshold based on PartSize
		// Files smaller than PartSize use single PutObject, larger files use multipart
	})

	input := &s3.PutObjectInput{
		Bucket: aws.String(b.bucketName),
		Key:    aws.String(objectName),
		Body:   reader,
	}

	if size > 0 {
		input.ContentLength = aws.Int64(size)
	}

	// Apply object options
	b.applyObjectOptions(input, opts.ObjectOptions)

	// The data reader is used directly since we've handled concurrency issues in the uploader Config
	var originalReader io.Reader = reader

	// Create a progress reader if callback is provided
	if progress != nil {
		var totalParts int

		if useMultipart {
			totalParts = int(size / b.config.PartSize)
			if size%b.config.PartSize != 0 {
				totalParts++
			}
		} else {
			totalParts = 1
		}

		// Always use simple progress wrapper
		input.Body = &progressReader{
			reader:     originalReader,
			totalSize:  size,
			callback:   progress,
			totalParts: totalParts,
		}
	} else {
		input.Body = originalReader
	}

	_, err := uploader.Upload(ctx, input)

	logOperationEnd(b.logger, "put_object_advanced", objectName, startTime, err, log.KV{
		"bucket_name": b.bucketName,
	})

	return err
}
