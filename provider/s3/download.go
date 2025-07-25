package s3

import (
	"context"
	"io"

	"github.com/minio/minio-go/v7"
)

// GetObjectStream downloads an object and writes it directly to a writer
func (b *Bucket) GetObjectStream(ctx context.Context, objectName string, writer io.Writer) error {
	if !b.IsConnected() {
		return ErrClientNotConnected
	}

	// Use upload timeout for large downloads to match upload behavior
	ctx, cancel := getContextWithTimeout(b.uploadTimeout, ctx)
	defer cancel()

	// Get object using MinIO client
	obj, err := b.minioClient.GetObject(ctx, b.bucketName, objectName, minio.GetObjectOptions{})
	if err != nil {
		return err
	}
	defer obj.Close()

	// Copy object data to writer
	_, err = io.Copy(writer, obj)
	return err
}

// GetObjectRange downloads a specific range of bytes from an object
func (b *Bucket) GetObjectRange(ctx context.Context, objectName string, start, end int64) (io.ReadCloser, error) {
	if !b.IsConnected() {
		return nil, ErrClientNotConnected
	}

	// Don't use getContextWithTimeout here - let the caller manage the context
	// This prevents the context from being canceled while the reader is still being used

	// Create MinIO get options with range
	opts := minio.GetObjectOptions{}
	if start >= 0 && end >= start {
		// Range from start to end (inclusive) - MinIO SetRange follows HTTP range spec
		opts.SetRange(start, end)
	} else if start >= 0 && end < 0 {
		// Range from start to end of file
		opts.SetRange(start, 0)
	} else if start < 0 && end >= 0 {
		// Range from beginning of file to end byte (inclusive)
		opts.SetRange(0, end)
	}

	obj, err := b.minioClient.GetObject(ctx, b.bucketName, objectName, opts)
	if err != nil {
		return nil, err
	}

	return obj, nil
}

// GetObjectStreamRange downloads a range of bytes from an object to a writer
func (b *Bucket) GetObjectStreamRange(ctx context.Context, objectName string, writer io.Writer, start, end int64) error {
	if !b.IsConnected() {
		return ErrClientNotConnected
	}

	// Use upload timeout for large downloads to match upload behavior
	ctx, cancel := getContextWithTimeout(b.uploadTimeout, ctx)
	defer cancel()

	body, err := b.GetObjectRange(ctx, objectName, start, end)
	if err != nil {
		return err
	}
	defer body.Close()

	_, err = io.Copy(writer, body)
	return err
}

// GetObjectAdvanced provides advanced download functionality with simplified control
// Note: Complex download manager removed - uses simple range downloads instead
func (b *Bucket) GetObjectAdvanced(ctx context.Context, objectName string, writer io.Writer, opts DownloadOptions) error {
	if !b.IsConnected() {
		return ErrClientNotConnected
	}

	// Use upload timeout for large downloads to match upload behavior
	ctx, cancel := getContextWithTimeout(b.uploadTimeout, ctx)
	defer cancel()

	// Handle range downloads if specified
	if opts.StartByte != nil || opts.EndByte != nil {
		var start, end int64 = -1, -1
		if opts.StartByte != nil {
			start = *opts.StartByte
		}
		if opts.EndByte != nil {
			end = *opts.EndByte
		}
		return b.GetObjectStreamRange(ctx, objectName, writer, start, end)
	}

	// For full downloads, use the regular stream method
	return b.GetObjectStream(ctx, objectName, writer)
}
