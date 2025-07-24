//go:build integration && s3

package s3

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// BenchmarkS3Operations benchmarks various S3 operations to measure transfer speeds
func BenchmarkS3Operations(b *testing.B) {
	config := getTestConfig()

	client, err := NewClient(config, nil)
	require.NoError(b, err)

	ctx := context.Background()
	err = client.Connect(ctx)
	if err != nil {
		b.Skip("Skipping benchmark: unable to connect to S3 service")
	}
	defer client.Close()

	testBucketName := "benchmark-bucket-" + strings.ToLower(time.Now().Format("20060102-150405"))
	bucket, err := client.Bucket(testBucketName)
	require.NoError(b, err)

	// Setup: Create bucket
	err = bucket.Create(ctx)
	require.NoError(b, err)

	// Clean up at the end
	defer func() {
		// Clean up all test objects
		objects, _ := bucket.ListObjects(ctx)
		for _, obj := range objects {
			bucket.DeleteObject(ctx, obj.Key)
		}
		bucket.Delete(ctx)
	}()

	// Test different data sizes
	dataSizes := []struct {
		name string
		size int64
	}{
		{"1KB", 1024},
		{"10KB", 10 * 1024},
		{"100KB", 100 * 1024},
		{"1MB", 1024 * 1024},
		{"10MB", 10 * 1024 * 1024},
		{"50MB", 50 * 1024 * 1024},
		{"100MB", 100 * 1024 * 1024},
	}

	for _, dataSize := range dataSizes {
		b.Run(fmt.Sprintf("Upload_%s", dataSize.name), func(b *testing.B) {
			benchmarkUpload(b, bucket, dataSize.size)
		})

		b.Run(fmt.Sprintf("Download_%s", dataSize.name), func(b *testing.B) {
			benchmarkDownload(b, bucket, dataSize.size)
		})

		b.Run(fmt.Sprintf("RoundTrip_%s", dataSize.name), func(b *testing.B) {
			benchmarkRoundTrip(b, bucket, dataSize.size)
		})
	}
}

// benchmarkUpload measures upload speed
func benchmarkUpload(b *testing.B, bucket *Bucket, dataSize int64) {
	// Generate test data
	data := make([]byte, dataSize)
	_, err := rand.Read(data)
	require.NoError(b, err)

	ctx := context.Background()

	b.ResetTimer()
	b.SetBytes(dataSize)

	for i := 0; i < b.N; i++ {
		objectKey := fmt.Sprintf("upload-bench-%d-%d", dataSize, i)
		reader := bytes.NewReader(data)

		start := time.Now()
		err := bucket.PutObject(ctx, objectKey, reader, dataSize)
		duration := time.Since(start)

		require.NoError(b, err)

		// Calculate and report transfer speed
		speedMBps := float64(dataSize) / duration.Seconds() / (1024 * 1024)
		b.ReportMetric(speedMBps, "MB/s")

		// Clean up
		bucket.DeleteObject(ctx, objectKey)
	}
}

// benchmarkDownload measures download speed
func benchmarkDownload(b *testing.B, bucket *Bucket, dataSize int64) {
	// Generate test data and upload it once
	data := make([]byte, dataSize)
	_, err := rand.Read(data)
	require.NoError(b, err)

	ctx := context.Background()
	objectKey := fmt.Sprintf("download-bench-%d", dataSize)

	// Upload test object
	err = bucket.PutObject(ctx, objectKey, bytes.NewReader(data), dataSize)
	require.NoError(b, err)

	defer bucket.DeleteObject(ctx, objectKey)

	b.ResetTimer()
	b.SetBytes(dataSize)

	for i := 0; i < b.N; i++ {
		start := time.Now()
		reader, err := bucket.GetObject(ctx, objectKey)
		require.NoError(b, err)

		// Read all data to measure actual transfer speed
		buffer := make([]byte, 32*1024) // 32KB buffer
		var totalRead int64
		for {
			n, readErr := reader.Read(buffer)
			totalRead += int64(n)
			if readErr == io.EOF {
				break
			}
			require.NoError(b, readErr)
		}
		reader.Close()
		duration := time.Since(start)

		require.Equal(b, dataSize, totalRead)

		// Calculate and report transfer speed
		speedMBps := float64(dataSize) / duration.Seconds() / (1024 * 1024)
		b.ReportMetric(speedMBps, "MB/s")
	}
}

// benchmarkRoundTrip measures full upload + download cycle
func benchmarkRoundTrip(b *testing.B, bucket *Bucket, dataSize int64) {
	// Generate test data
	data := make([]byte, dataSize)
	_, err := rand.Read(data)
	require.NoError(b, err)

	ctx := context.Background()

	b.ResetTimer()
	b.SetBytes(dataSize * 2) // Upload + Download

	for i := 0; i < b.N; i++ {
		objectKey := fmt.Sprintf("roundtrip-bench-%d-%d", dataSize, i)

		start := time.Now()

		// Upload
		reader := bytes.NewReader(data)
		err := bucket.PutObject(ctx, objectKey, reader, dataSize)
		require.NoError(b, err)

		// Download
		downloadReader, err := bucket.GetObject(ctx, objectKey)
		require.NoError(b, err)

		// Read all data
		buffer := make([]byte, 32*1024)
		var totalRead int64
		for {
			n, readErr := downloadReader.Read(buffer)
			totalRead += int64(n)
			if readErr == io.EOF {
				break
			}
			require.NoError(b, readErr)
		}
		downloadReader.Close()

		duration := time.Since(start)
		require.Equal(b, dataSize, totalRead)

		// Calculate and report transfer speed for round trip
		speedMBps := float64(dataSize*2) / duration.Seconds() / (1024 * 1024)
		b.ReportMetric(speedMBps, "MB/s")

		// Clean up
		bucket.DeleteObject(ctx, objectKey)
	}
}

// BenchmarkMultipartUpload specifically benchmarks multipart upload performance
func BenchmarkMultipartUpload(b *testing.B) {
	config := getTestConfig()

	client, err := NewClient(config, nil)
	require.NoError(b, err)

	ctx := context.Background()
	err = client.Connect(ctx)
	if err != nil {
		b.Skip("Skipping benchmark: unable to connect to S3 service")
	}
	defer client.Close()

	testBucketName := "multipart-benchmark-" + strings.ToLower(time.Now().Format("20060102-150405"))
	bucket, err := client.Bucket(testBucketName)
	require.NoError(b, err)

	// Setup: Create bucket
	err = bucket.Create(ctx)
	require.NoError(b, err)

	// Clean up at the end
	defer func() {
		objects, _ := bucket.ListObjects(ctx)
		for _, obj := range objects {
			bucket.DeleteObject(ctx, obj.Key)
		}
		bucket.Delete(ctx)
	}()

	// Test large files that will trigger multipart upload
	largeSizes := []struct {
		name string
		size int64
	}{
		{"200MB", 200 * 1024 * 1024},
		{"500MB", 500 * 1024 * 1024},
		{"1GB", 1024 * 1024 * 1024},
	}

	for _, dataSize := range largeSizes {
		b.Run(fmt.Sprintf("MultipartUpload_%s", dataSize.name), func(b *testing.B) {
			benchmarkMultipartUpload(b, bucket, dataSize.size)
		})
	}
}

// benchmarkMultipartUpload measures multipart upload performance
func benchmarkMultipartUpload(b *testing.B, bucket *Bucket, dataSize int64) {
	ctx := context.Background()

	b.ResetTimer()
	b.SetBytes(dataSize)

	for i := 0; i < b.N; i++ {
		objectKey := fmt.Sprintf("multipart-bench-%d-%d", dataSize, i)

		// Create a reader that generates data on-the-fly to avoid memory issues
		reader := &randomDataReader{size: dataSize}

		start := time.Now()
		err := bucket.PutObject(ctx, objectKey, reader, dataSize)
		duration := time.Since(start)

		require.NoError(b, err)

		// Calculate and report transfer speed
		speedMBps := float64(dataSize) / duration.Seconds() / (1024 * 1024)
		b.ReportMetric(speedMBps, "MB/s")

		// Verify the object was uploaded correctly
		exists, err := bucket.ObjectExists(ctx, objectKey)
		require.NoError(b, err)
		require.True(b, exists)

		// Clean up
		bucket.DeleteObject(ctx, objectKey)
	}
}

// BenchmarkConcurrentOperations benchmarks concurrent upload/download performance
func BenchmarkConcurrentOperations(b *testing.B) {
	config := getTestConfig()

	client, err := NewClient(config, nil)
	require.NoError(b, err)

	ctx := context.Background()
	err = client.Connect(ctx)
	if err != nil {
		b.Skip("Skipping benchmark: unable to connect to S3 service")
	}
	defer client.Close()

	testBucketName := "concurrent-benchmark-" + strings.ToLower(time.Now().Format("20060102-150405"))
	bucket, err := client.Bucket(testBucketName)
	require.NoError(b, err)

	// Setup: Create bucket
	err = bucket.Create(ctx)
	require.NoError(b, err)

	// Clean up at the end
	defer func() {
		objects, _ := bucket.ListObjects(ctx)
		for _, obj := range objects {
			bucket.DeleteObject(ctx, obj.Key)
		}
		bucket.Delete(ctx)
	}()

	concurrencyLevels := []int{1, 2, 4, 8, 16}
	dataSize := int64(1024 * 1024) // 1MB per operation

	for _, concurrency := range concurrencyLevels {
		b.Run(fmt.Sprintf("ConcurrentUpload_%dx1MB", concurrency), func(b *testing.B) {
			benchmarkConcurrentUpload(b, bucket, dataSize, concurrency)
		})
	}
}

// benchmarkConcurrentUpload measures concurrent upload performance
func benchmarkConcurrentUpload(b *testing.B, bucket *Bucket, dataSize int64, concurrency int) {
	// Generate test data
	data := make([]byte, dataSize)
	_, err := rand.Read(data)
	require.NoError(b, err)

	ctx := context.Background()

	b.ResetTimer()
	b.SetBytes(dataSize * int64(concurrency))

	for i := 0; i < b.N; i++ {
		start := time.Now()

		// Create channels for coordination
		done := make(chan error, concurrency)

		// Launch concurrent uploads
		for j := 0; j < concurrency; j++ {
			go func(index int) {
				objectKey := fmt.Sprintf("concurrent-bench-%d-%d-%d", i, index, time.Now().UnixNano())
				reader := bytes.NewReader(data)
				err := bucket.PutObject(ctx, objectKey, reader, dataSize)
				done <- err
			}(j)
		}

		// Wait for all uploads to complete
		for j := 0; j < concurrency; j++ {
			err := <-done
			require.NoError(b, err)
		}

		duration := time.Since(start)

		// Calculate and report aggregate transfer speed
		totalBytes := float64(dataSize * int64(concurrency))
		speedMBps := totalBytes / duration.Seconds() / (1024 * 1024)
		b.ReportMetric(speedMBps, "MB/s")
	}

	// Clean up (delete all objects created in this benchmark)
	objects, _ := bucket.ListObjects(ctx)
	for _, obj := range objects {
		if strings.Contains(obj.Key, "concurrent-bench-") {
			bucket.DeleteObject(ctx, obj.Key)
		}
	}
}

// randomDataReader generates random data on-the-fly to avoid memory issues with large files
type randomDataReader struct {
	size int64
	read int64
}

func (r *randomDataReader) Read(p []byte) (n int, err error) {
	if r.read >= r.size {
		return 0, io.EOF
	}

	remaining := r.size - r.read
	toRead := int64(len(p))
	if toRead > remaining {
		toRead = remaining
	}

	// Generate random data
	n = int(toRead)
	_, err = rand.Read(p[:n])
	if err != nil {
		return 0, err
	}

	r.read += int64(n)
	return n, nil
}

// BenchmarkPresignedURLOperations benchmarks pre-signed URL generation performance
func BenchmarkPresignedURLOperations(b *testing.B) {
	config := getTestConfig()

	client, err := NewClient(config, nil)
	require.NoError(b, err)

	ctx := context.Background()
	err = client.Connect(ctx)
	if err != nil {
		b.Skip("Skipping benchmark: unable to connect to S3 service")
	}
	defer client.Close()

	testBucketName := "presigned-benchmark-" + strings.ToLower(time.Now().Format("20060102-150405"))
	bucket, err := client.Bucket(testBucketName)
	require.NoError(b, err)

	// Setup: Create bucket
	err = bucket.Create(ctx)
	require.NoError(b, err)

	defer bucket.Delete(ctx)

	b.Run("PresignGetURL", func(b *testing.B) {
		objectKey := "test-object"
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			start := time.Now()
			url, err := bucket.PresignGetObject(ctx, objectKey, time.Hour)
			duration := time.Since(start)

			require.NoError(b, err)
			require.NotEmpty(b, url)

			// Report latency in microseconds
			latencyMicros := float64(duration.Nanoseconds()) / 1000.0
			b.ReportMetric(latencyMicros, "μs/op")
		}
	})

	b.Run("PresignPutURL", func(b *testing.B) {
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			objectKey := fmt.Sprintf("test-object-%d", i)
			start := time.Now()
			url, err := bucket.PresignPutObject(ctx, objectKey, time.Hour)
			duration := time.Since(start)

			require.NoError(b, err)
			require.NotEmpty(b, url)

			// Report latency in microseconds
			latencyMicros := float64(duration.Nanoseconds()) / 1000.0
			b.ReportMetric(latencyMicros, "μs/op")
		}
	})
}
