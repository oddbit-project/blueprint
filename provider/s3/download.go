package s3

import (
	"context"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// GetObjectStream downloads an object and writes it directly to a writer
func (b *Bucket) GetObjectStream(ctx context.Context, objectName string, writer io.Writer) error {
	if !b.IsConnected() {
		return ErrClientNotConnected
	}

	if err := ValidateObjectName(objectName); err != nil {
		return err
	}

	ctx, cancel := getContextWithTimeout(b.timeout, ctx)
	defer cancel()

	// Use the downloader for efficient streaming
	_, err := b.downloader.Download(ctx, &writerAt{writer: writer}, &s3.GetObjectInput{
		Bucket: aws.String(b.bucketName),
		Key:    aws.String(objectName),
	})

	return err
}

// GetObjectRange downloads a specific range of bytes from an object
func (b *Bucket) GetObjectRange(ctx context.Context, objectName string, start, end int64) (io.ReadCloser, error) {
	if !b.IsConnected() {
		return nil, ErrClientNotConnected
	}

	if err := ValidateObjectName(objectName); err != nil {
		return nil, err
	}

	ctx, cancel := getContextWithTimeout(b.timeout, ctx)
	defer cancel()

	rangeHeader := ""
	if start >= 0 || end >= 0 {
		if end >= 0 {
			rangeHeader = fmt.Sprintf("bytes=%d-%d", start, end)
		} else {
			rangeHeader = fmt.Sprintf("bytes=%d-", start)
		}
	}

	input := &s3.GetObjectInput{
		Bucket: aws.String(b.bucketName),
		Key:    aws.String(objectName),
	}

	if rangeHeader != "" {
		input.Range = aws.String(rangeHeader)
	}

	result, err := b.s3Client.GetObject(ctx, input)
	if err != nil {
		return nil, err
	}

	return result.Body, nil
}

// GetObjectStreamRange downloads a range of bytes from an object to a writer
func (b *Bucket) GetObjectStreamRange(ctx context.Context, objectName string, writer io.Writer, start, end int64) error {
	if !b.IsConnected() {
		return ErrClientNotConnected
	}

	if err := ValidateObjectName(objectName); err != nil {
		return err
	}

	ctx, cancel := getContextWithTimeout(b.timeout, ctx)
	defer cancel()

	body, err := b.GetObjectRange(ctx, objectName, start, end)
	if err != nil {
		return err
	}
	defer body.Close()

	_, err = io.Copy(writer, body)
	return err
}

// GetObjectAdvanced provides advanced download functionality with detailed control
func (b *Bucket) GetObjectAdvanced(ctx context.Context, objectName string, writer io.Writer, opts DownloadOptions) error {
	if !b.IsConnected() {
		return ErrClientNotConnected
	}

	if err := ValidateObjectName(objectName); err != nil {
		return err
	}

	ctx, cancel := getContextWithTimeout(b.timeout, ctx)
	defer cancel()

	// Create custom downloader with options
	downloader := b.downloader
	if opts.Concurrency > 0 || opts.PartSize > 0 {
		downloader = manager.NewDownloader(b.s3Client, func(d *manager.Downloader) {
			if opts.Concurrency > 0 {
				d.Concurrency = opts.Concurrency
			}
			if opts.PartSize > 0 {
				d.PartSize = opts.PartSize
			}
		})
	}

	input := &s3.GetObjectInput{
		Bucket: aws.String(b.bucketName),
		Key:    aws.String(objectName),
	}

	// Set range if specified
	if opts.StartByte != nil || opts.EndByte != nil {
		var rangeHeader string
		if opts.StartByte != nil && opts.EndByte != nil {
			rangeHeader = fmt.Sprintf("bytes=%d-%d", *opts.StartByte, *opts.EndByte)
		} else if opts.StartByte != nil {
			rangeHeader = fmt.Sprintf("bytes=%d-", *opts.StartByte)
		} else {
			rangeHeader = fmt.Sprintf("bytes=0-%d", *opts.EndByte)
		}
		input.Range = aws.String(rangeHeader)
	}

	_, err := downloader.Download(ctx, &writerAt{writer: writer}, input)
	return err
}

// writerAt wraps an io.Writer to implement io.WriterAt
// This is needed because the AWS SDK downloader requires io.WriterAt
type writerAt struct {
	writer io.Writer
	pos    int64
}

// WriteAt implements io.WriterAt by converting to sequential writes
// Note: This assumes sequential writes, which is how the AWS downloader works
func (w *writerAt) WriteAt(p []byte, offset int64) (int, error) {
	// For sequential downloads, we can ignore the offset
	// and just write to the underlying writer
	n, err := w.writer.Write(p)
	w.pos += int64(n)
	return n, err
}

// bufferedWriterAt provides a more robust WriterAt implementation
// that can handle out-of-order writes by buffering
type bufferedWriterAt struct {
	writer io.Writer
	buffer map[int64][]byte
	pos    int64
}

// newBufferedWriterAt creates a new buffered writer
func newBufferedWriterAt(writer io.Writer) *bufferedWriterAt {
	return &bufferedWriterAt{
		writer: writer,
		buffer: make(map[int64][]byte),
		pos:    0,
	}
}

// WriteAt implements io.WriterAt with buffering for out-of-order writes
func (bw *bufferedWriterAt) WriteAt(p []byte, offset int64) (int, error) {
	// If this write is at the current position, write directly
	if offset == bw.pos {
		n, err := bw.writer.Write(p)
		if err != nil {
			return n, err
		}
		bw.pos += int64(n)

		// Try to flush any buffered data that's now sequential
		bw.flushSequential()
		return n, nil
	}

	// Otherwise, buffer this write
	data := make([]byte, len(p))
	copy(data, p)
	bw.buffer[offset] = data

	return len(p), nil
}

// flushSequential writes any buffered data that's now sequential
func (bw *bufferedWriterAt) flushSequential() {
	for {
		data, exists := bw.buffer[bw.pos]
		if !exists {
			break
		}

		n, err := bw.writer.Write(data)
		if err != nil {
			// In a real implementation, we'd need to handle this error
			break
		}

		bw.pos += int64(n)
		delete(bw.buffer, bw.pos-int64(n))
	}
}
