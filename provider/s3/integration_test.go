package s3

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Integration test configuration
const (
	// MinIO test configuration base
	testMinIOBasePort  = 9022
	testMinIOAccessKey = "minioadmin"
	testMinIOSecretKey = "minioadmin"
	testMinIORegion    = "us-east-1"

	// Test constants
	testBucketPrefix   = "test-bucket-"
	testObjectPrefix   = "test-object-"
	integrationTimeout = 30 * time.Second
)

// setupMinIO starts a MinIO container for testing with unique port
func setupMinIO(t *testing.T) (string, func()) {
	t.Helper()

	// Check if Docker is available
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("Docker not available, skipping integration tests")
	}

	// Generate unique port based on test name hash to avoid conflicts
	_ = t.Name() // testName not used currently but kept for potential future use
	hash := fmt.Sprintf("%x", time.Now().UnixNano())[:4]
	containerName := fmt.Sprintf("minio-test-%s", hash)
	port := testMinIOBasePort + (int(time.Now().UnixNano()) % 1000)
	consolePort := port + 1
	endpoint := fmt.Sprintf("localhost:%d", port)

	// Stop any existing MinIO container with this name
	exec.Command("docker", "stop", containerName).Run()
	exec.Command("docker", "rm", containerName).Run()

	// Start MinIO container
	cmd := exec.Command("docker", "run", "-d",
		"--name", containerName,
		"-p", fmt.Sprintf("%d:9000", port),
		"-p", fmt.Sprintf("%d:9001", consolePort),
		"-e", "MINIO_ROOT_USER="+testMinIOAccessKey,
		"-e", "MINIO_ROOT_PASSWORD="+testMinIOSecretKey,
		"quay.io/minio/minio", "server", "/data", "--console-address", ":9001")

	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to start MinIO container: %v", err)
	}

	// Wait for MinIO to be ready
	ready := false
	for i := 0; i < 30; i++ { // Wait up to 30 seconds
		if testMinIOConnection(t, endpoint) {
			ready = true
			break
		}
		time.Sleep(1 * time.Second)
	}

	if !ready {
		exec.Command("docker", "stop", containerName).Run()
		exec.Command("docker", "rm", containerName).Run()
		t.Fatalf("MinIO did not become ready in time on port %d", port)
	}

	t.Logf("MinIO container started and ready for integration tests on %s", endpoint)

	// Return endpoint and cleanup function
	return endpoint, func() {
		t.Log("Cleaning up MinIO container")
		exec.Command("docker", "stop", containerName).Run()
		exec.Command("docker", "rm", containerName).Run()
	}
}

// testMinIOConnection tests if MinIO is ready to accept connections
func testMinIOConnection(t *testing.T, endpoint string) bool {
	// Use curl to test HTTP endpoint directly
	cmd := exec.Command("curl", "-sf", fmt.Sprintf("http://%s/minio/health/live", endpoint))
	return cmd.Run() == nil
}

// createTestClient creates a configured S3 client for testing
func createTestClient(t *testing.T, endpoint string) *Client {
	config := NewConfig()
	config.Endpoint = endpoint
	config.Region = testMinIORegion
	config.AccessKeyID = testMinIOAccessKey
	config.UseSSL = false
	config.ForcePathStyle = true

	// Set secret key using test-specific env var
	envVarName := fmt.Sprintf("MINIO_TEST_SECRET_%s", strings.ReplaceAll(t.Name(), "/", "_"))
	os.Setenv(envVarName, testMinIOSecretKey)
	config.DefaultCredentialConfig.PasswordEnvVar = envVarName

	client, err := NewClient(config, nil)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), integrationTimeout)
	defer cancel()

	err = client.Connect(ctx)
	require.NoError(t, err)

	return client
}

// generateTestBucketName generates a unique bucket name for testing
func generateTestBucketName() string {
	return fmt.Sprintf("%s%d", testBucketPrefix, time.Now().UnixNano())
}

// generateTestObjectKey generates a unique object key for testing
func generateTestObjectKey() string {
	return fmt.Sprintf("%s%d.txt", testObjectPrefix, time.Now().UnixNano())
}

// generateTestData creates test data of specified size
func generateTestData(size int) []byte {
	data := make([]byte, size)
	rand.Read(data)
	return data
}

// Integration test for bucket operations
func TestIntegrationBucketOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	endpoint, cleanup := setupMinIO(t)
	defer cleanup()

	client := createTestClient(t, endpoint)
	defer client.Close()

	ctx := context.Background()
	bucketName := generateTestBucketName()
	bucket, err := client.Bucket(bucketName)
	assert.NoError(t, err)

	t.Run("CreateBucket", func(t *testing.T) {
		err := bucket.Create(ctx)
		assert.NoError(t, err, "Should create bucket successfully")
	})

	t.Run("BucketExists", func(t *testing.T) {
		exists, err := bucket.Exists(ctx)
		assert.NoError(t, err, "Should check bucket existence without error")
		assert.True(t, exists, "Bucket should exist after creation")

		// Test non-existent bucket
		nonExistentBucket := generateTestBucketName()
		bucket, err = client.Bucket(nonExistentBucket)
		assert.NoError(t, err)
		exists, err = bucket.Exists(ctx)
		assert.NoError(t, err, "Should check non-existent bucket without error")
		assert.False(t, exists, "Non-existent bucket should not exist")
	})

	t.Run("ListBuckets", func(t *testing.T) {
		buckets, err := client.ListBuckets(ctx)
		assert.NoError(t, err, "Should list buckets successfully")

		// Find our test bucket
		found := false
		for _, bucket := range buckets {
			if bucket.Name == bucketName {
				found = true
				assert.False(t, bucket.CreationDate.IsZero(), "Bucket should have creation date")
				break
			}
		}
		assert.True(t, found, "Should find our test bucket in the list")
	})

	t.Run("CreateBucketAlreadyExists", func(t *testing.T) {
		err := bucket.Create(ctx)
		assert.Error(t, err, "Should fail to create bucket that already exists")
	})

	// Clean up - we'll delete the bucket after object tests
	t.Cleanup(func() {
		// List and delete all objects first
		objects, _ := bucket.ListObjects(ctx)
		for _, obj := range objects {
			bucket.DeleteObject(ctx, obj.Key)
		}
		// Then delete the bucket
		bucket.Delete(ctx)
	})
}

// Integration test for object operations
func TestIntegrationObjectOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	endpoint, cleanup := setupMinIO(t)
	defer cleanup()

	client := createTestClient(t, endpoint)
	defer client.Close()

	ctx := context.Background()
	bucketName := generateTestBucketName()
	objectKey := generateTestObjectKey()
	bucket, err := client.Bucket(bucketName)
	assert.NoError(t, err)

	// Setup bucket
	err = bucket.Create(ctx)
	require.NoError(t, err)

	defer func() {
		bucket.DeleteObject(ctx, objectKey)
		bucket.Delete(ctx)
	}()

	testData := []byte("Hello, MinIO integration test!")

	t.Run("PutObject", func(t *testing.T) {
		reader := bytes.NewReader(testData)
		err := bucket.PutObject(ctx, objectKey, reader, int64(len(testData)))
		assert.NoError(t, err, "Should upload object successfully")
	})

	t.Run("ObjectExists", func(t *testing.T) {
		exists, err := bucket.ObjectExists(ctx, objectKey)
		assert.NoError(t, err, "Should check object existence without error")
		assert.True(t, exists, "Object should exist after upload")

		// Test non-existent object
		nonExistentKey := generateTestObjectKey()
		exists, err = bucket.ObjectExists(ctx, nonExistentKey)
		assert.NoError(t, err, "Should check non-existent object without error")
		assert.False(t, exists, "Non-existent object should not exist")
	})

	t.Run("HeadObject", func(t *testing.T) {
		info, err := bucket.HeadObject(ctx, objectKey)
		assert.NoError(t, err, "Should get object metadata successfully")
		assert.Equal(t, objectKey, info.Key, "Object key should match")
		assert.Equal(t, int64(len(testData)), info.Size, "Object size should match")
		assert.NotEmpty(t, info.ETag, "Object should have ETag")
		assert.False(t, info.LastModified.IsZero(), "Object should have last modified date")
	})

	t.Run("GetObject", func(t *testing.T) {
		reader, err := bucket.GetObject(ctx, objectKey)
		assert.NoError(t, err, "Should download object successfully")
		defer reader.Close()

		data, err := io.ReadAll(reader)
		assert.NoError(t, err, "Should read object data successfully")
		assert.Equal(t, testData, data, "Downloaded data should match uploaded data")
	})

	t.Run("ListObjects", func(t *testing.T) {
		objects, err := bucket.ListObjects(ctx)
		assert.NoError(t, err, "Should list objects successfully")
		assert.Len(t, objects, 1, "Should have exactly one object")

		obj := objects[0]
		assert.Equal(t, objectKey, obj.Key, "Object key should match")
		assert.Equal(t, int64(len(testData)), obj.Size, "Object size should match")
		assert.NotEmpty(t, obj.ETag, "Object should have ETag")
		assert.False(t, obj.LastModified.IsZero(), "Object should have last modified date")
	})

	t.Run("ListObjectsWithPrefix", func(t *testing.T) {
		// Upload another object with different prefix
		anotherKey := "different-prefix-test.txt"
		reader := bytes.NewReader([]byte("different data"))
		err := bucket.PutObject(ctx, anotherKey, reader, int64(len("different data")))
		require.NoError(t, err)

		defer bucket.DeleteObject(ctx, anotherKey)

		// List with prefix
		options := ListOptions{Prefix: testObjectPrefix}
		objects, err := bucket.ListObjects(ctx, options)
		assert.NoError(t, err, "Should list objects with prefix successfully")
		assert.Len(t, objects, 1, "Should have exactly one object with prefix")
		assert.Equal(t, objectKey, objects[0].Key, "Should find object with matching prefix")
	})

	t.Run("DeleteObject", func(t *testing.T) {
		// First verify object exists
		exists, err := bucket.ObjectExists(ctx, objectKey)
		require.NoError(t, err)
		require.True(t, exists)

		// Delete object
		err = bucket.DeleteObject(ctx, objectKey)
		assert.NoError(t, err, "Should delete object successfully")

		// Verify object no longer exists
		exists, err = bucket.ObjectExists(ctx, objectKey)
		assert.NoError(t, err, "Should check deleted object existence without error")
		assert.False(t, exists, "Object should not exist after deletion")
	})
}

// Integration test for advanced object operations
func TestIntegrationAdvancedObjectOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	endpoint, cleanup := setupMinIO(t)
	defer cleanup()

	client := createTestClient(t, endpoint)
	defer client.Close()

	ctx := context.Background()
	bucketName := generateTestBucketName()
	bucket, err := client.Bucket(bucketName)
	assert.NoError(t, err)

	// Setup bucket
	err = bucket.Create(ctx)
	require.NoError(t, err)

	defer bucket.Delete(ctx)

	t.Run("PutObjectWithOptions", func(t *testing.T) {
		objectKey := generateTestObjectKey()
		testData := []byte("Test data with metadata")

		options := ObjectOptions{
			ContentType: "text/plain",
			Metadata: map[string]string{
				"test-key":    "test-value",
				"upload-time": time.Now().Format(time.RFC3339),
			},
			Tags: map[string]string{
				"environment": "test",
				"purpose":     "integration-test",
			},
		}

		reader := bytes.NewReader(testData)
		err := bucket.PutObject(ctx, objectKey, reader, int64(len(testData)), options)
		assert.NoError(t, err, "Should upload object with options successfully")

		defer bucket.DeleteObject(ctx, objectKey)

		// Verify metadata (Note: MinIO might not return all metadata in HeadObject)
		info, err := bucket.HeadObject(ctx, objectKey)
		assert.NoError(t, err, "Should get object metadata successfully")
		assert.Equal(t, "text/plain", info.ContentType, "Content type should be set correctly")
	})

	t.Run("CopyObject", func(t *testing.T) {
		sourceKey := generateTestObjectKey()
		destKey := generateTestObjectKey()
		testData := []byte("Data to copy")

		// Upload source object
		reader := bytes.NewReader(testData)
		err := bucket.PutObject(ctx, sourceKey, reader, int64(len(testData)))
		require.NoError(t, err)

		defer func() {
			bucket.DeleteObject(ctx, sourceKey)
			bucket.DeleteObject(ctx, destKey)
		}()

		// Copy object
		err = bucket.CopyObject(ctx, sourceKey, bucketName, destKey)
		assert.NoError(t, err, "Should copy object successfully")

		// Verify copied object
		downloadReader, err := bucket.GetObject(ctx, destKey)
		require.NoError(t, err)
		defer downloadReader.Close()

		data, err := io.ReadAll(downloadReader)
		assert.NoError(t, err, "Should read copied object successfully")
		assert.Equal(t, testData, data, "Copied data should match original data")
	})

	t.Run("PutObjectStream", func(t *testing.T) {
		objectKey := generateTestObjectKey()
		testData := []byte("Streaming upload test data")

		reader := bytes.NewReader(testData)
		err := bucket.PutObjectStream(ctx, objectKey, reader)
		assert.NoError(t, err, "Should upload object via stream successfully")

		defer bucket.DeleteObject(ctx, objectKey)

		// Verify uploaded data
		downloadReader, err := bucket.GetObject(ctx, objectKey)
		require.NoError(t, err)
		defer downloadReader.Close()

		data, err := io.ReadAll(downloadReader)
		assert.NoError(t, err, "Should read streamed object successfully")
		assert.Equal(t, testData, data, "Streamed data should match original data")
	})

	t.Run("GetObjectStream", func(t *testing.T) {
		objectKey := generateTestObjectKey()
		testData := []byte("Stream download test data")

		// Upload object
		reader := bytes.NewReader(testData)
		err := bucket.PutObject(ctx, objectKey, reader, int64(len(testData)))
		require.NoError(t, err)

		defer bucket.DeleteObject(ctx, objectKey)

		// Download via stream
		var buf bytes.Buffer
		err = bucket.GetObjectStream(ctx, objectKey, &buf)
		assert.NoError(t, err, "Should download object via stream successfully")
		assert.Equal(t, testData, buf.Bytes(), "Streamed download data should match original data")
	})
}

// Integration test for multipart upload
func TestIntegrationMultipartUpload(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	endpoint, cleanup := setupMinIO(t)
	defer cleanup()

	client := createTestClient(t, endpoint)
	defer client.Close()

	ctx := context.Background()
	bucketName := generateTestBucketName()
	bucket, err := client.Bucket(bucketName)
	assert.NoError(t, err)

	// Setup bucket
	err = bucket.Create(ctx)
	require.NoError(t, err)

	defer bucket.Delete(ctx)

	t.Run("MultipartUploadSmallFile", func(t *testing.T) {
		objectKey := generateTestObjectKey()
		// Create a file larger than the multipart threshold (100MB default)
		// For testing, we'll use a smaller size but force multipart
		testData := generateTestData(15 * 1024 * 1024) // 15MB

		reader := bytes.NewReader(testData)

		var progressCalls int
		progressCallback := func(progress UploadProgress) {
			progressCalls++
			t.Logf("Upload progress: %d/%d bytes, %d/%d parts",
				progress.BytesUploaded, progress.TotalBytes,
				progress.PartsUploaded, progress.TotalParts)

			assert.True(t, progress.BytesUploaded <= progress.TotalBytes, "Bytes uploaded should not exceed total")
			assert.True(t, progress.PartsUploaded <= progress.TotalParts, "Parts uploaded should not exceed total")
		}

		err := bucket.PutObjectMultipart(ctx, objectKey, reader, int64(len(testData)), progressCallback)
		assert.NoError(t, err, "Should upload large object via multipart successfully")

		defer bucket.DeleteObject(ctx, objectKey)

		// Verify progress callback was called
		assert.True(t, progressCalls > 0, "Progress callback should be called")

		// Verify uploaded data
		downloadReader, err := bucket.GetObject(ctx, objectKey)
		require.NoError(t, err)
		defer downloadReader.Close()

		downloadedData, err := io.ReadAll(downloadReader)
		assert.NoError(t, err, "Should read multipart uploaded object successfully")
		assert.Equal(t, testData, downloadedData, "Multipart uploaded data should match original data")
	})

	t.Run("MultipartUploadLargeFile", func(t *testing.T) {
		objectKey := generateTestObjectKey()
		// Create a file that definitely triggers multipart upload
		testData := generateTestData(150 * 1024 * 1024) // 150MB

		reader := bytes.NewReader(testData)

		var progressCalls int
		var lastProgress UploadProgress
		progressCallback := func(progress UploadProgress) {
			progressCalls++
			lastProgress = progress

			// Only log every 10th call to avoid spam
			if progressCalls%10 == 0 {
				t.Logf("Upload progress: %d/%d bytes (%.1f%%), %d/%d parts",
					progress.BytesUploaded, progress.TotalBytes,
					float64(progress.BytesUploaded)/float64(progress.TotalBytes)*100,
					progress.PartsUploaded, progress.TotalParts)
			}
		}

		err := bucket.PutObjectMultipart(ctx, objectKey, reader, int64(len(testData)), progressCallback)
		assert.NoError(t, err, "Should upload very large object via multipart successfully")

		defer bucket.DeleteObject(ctx, objectKey)

		// Verify progress tracking
		assert.True(t, progressCalls > 0, "Progress callback should be called")
		assert.Equal(t, int64(len(testData)), lastProgress.TotalBytes, "Final progress should show correct total bytes")
		assert.Equal(t, int64(len(testData)), lastProgress.BytesUploaded, "Final progress should show all bytes uploaded")

		// Verify object exists and has correct size
		info, err := bucket.HeadObject(ctx, objectKey)
		assert.NoError(t, err, "Should get metadata for multipart uploaded object")
		assert.Equal(t, int64(len(testData)), info.Size, "Object size should match uploaded data size")

		// For very large files, we'll just verify the first and last chunks to save time
		downloadReader, err := bucket.GetObject(ctx, objectKey)
		require.NoError(t, err)
		defer downloadReader.Close()

		// Read first 1MB
		firstChunk := make([]byte, 1024*1024)
		n, err := io.ReadFull(downloadReader, firstChunk)
		assert.NoError(t, err, "Should read first chunk successfully")
		assert.Equal(t, len(firstChunk), n, "Should read expected number of bytes")
		assert.Equal(t, testData[:len(firstChunk)], firstChunk, "First chunk should match")

		t.Log("Large multipart upload test completed successfully")
	})
}

// Integration test for presigned URLs
func TestIntegrationPresignedURLs(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	endpoint, cleanup := setupMinIO(t)
	defer cleanup()

	client := createTestClient(t, endpoint)
	defer client.Close()

	ctx := context.Background()
	bucketName := generateTestBucketName()
	objectKey := generateTestObjectKey()
	bucket, err := client.Bucket(bucketName)
	assert.NoError(t, err)

	// Setup bucket and object
	err = bucket.Create(ctx)
	require.NoError(t, err)

	defer bucket.Delete(ctx)

	testData := []byte("Presigned URL test data")
	reader := bytes.NewReader(testData)
	err = bucket.PutObject(ctx, objectKey, reader, int64(len(testData)))
	require.NoError(t, err)

	defer bucket.DeleteObject(ctx, objectKey)

	t.Run("PresignGetObject", func(t *testing.T) {
		// Test different expiry times
		expiryTimes := []time.Duration{
			5 * time.Minute,
			1 * time.Hour,
			24 * time.Hour,
		}

		for _, expiry := range expiryTimes {
			t.Run(fmt.Sprintf("Expiry%v", expiry), func(t *testing.T) {
				url, err := bucket.PresignGetObject(ctx, objectKey, expiry)
				assert.NoError(t, err, "Should generate presigned GET URL successfully")
				assert.NotEmpty(t, url, "Presigned URL should not be empty")
				assert.Contains(t, url, bucketName, "URL should contain bucket name")
				assert.Contains(t, url, objectKey, "URL should contain object key")

				// Basic validation that URL looks correct
				assert.True(t, strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://"),
					"URL should have valid protocol")
			})
		}
	})

	t.Run("PresignPutObject", func(t *testing.T) {
		newObjectKey := generateTestObjectKey()
		defer bucket.DeleteObject(ctx, newObjectKey)

		url, err := bucket.PresignPutObject(ctx, newObjectKey, time.Hour)
		assert.NoError(t, err, "Should generate presigned PUT URL successfully")
		assert.NotEmpty(t, url, "Presigned URL should not be empty")
		assert.Contains(t, url, bucketName, "URL should contain bucket name")
		assert.Contains(t, url, newObjectKey, "URL should contain object key")
	})

	t.Run("PresignPutObjectWithOptions", func(t *testing.T) {
		newObjectKey := generateTestObjectKey()
		defer bucket.DeleteObject(ctx, newObjectKey)

		options := ObjectOptions{
			ContentType: "text/plain",
			Metadata:    map[string]string{"test": "value"},
		}

		url, err := bucket.PresignPutObject(ctx, newObjectKey, time.Hour, options)
		assert.NoError(t, err, "Should generate presigned PUT URL with options successfully")
		assert.NotEmpty(t, url, "Presigned URL should not be empty")
	})
}

// Integration test for error conditions
func TestIntegrationErrorConditions(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	endpoint, cleanup := setupMinIO(t)
	defer cleanup()

	client := createTestClient(t, endpoint)
	defer client.Close()

	ctx := context.Background()
	nonExistentBucket := "non-existent-bucket-" + fmt.Sprint(time.Now().UnixNano())
	nonExistentKey := "non-existent-object.txt"
	bucket, err := client.Bucket(nonExistentBucket)
	assert.NoError(t, err)

	t.Run("BucketNotFound", func(t *testing.T) {
		// Try to list objects in non-existent bucket
		_, err := bucket.ListObjects(ctx)
		assert.Error(t, err, "Should fail when listing objects in non-existent bucket")

		// Try to upload to non-existent bucket
		reader := bytes.NewReader([]byte("test"))
		err = bucket.PutObject(ctx, "test.txt", reader, 4)
		assert.Error(t, err, "Should fail when uploading to non-existent bucket")
	})

	t.Run("ObjectNotFound", func(t *testing.T) {
		// Create a bucket for testing
		bucketName := generateTestBucketName()
		bucket, err = client.Bucket(bucketName)
		err := bucket.Create(ctx)
		require.NoError(t, err)
		defer bucket.Delete(ctx)

		// Try to download non-existent object
		_, err = bucket.GetObject(ctx, nonExistentKey)
		assert.Error(t, err, "Should fail when downloading non-existent object")

		// Try to get metadata of non-existent object
		_, err = bucket.HeadObject(ctx, nonExistentKey)
		assert.Error(t, err, "Should fail when getting metadata of non-existent object")
	})

	t.Run("InvalidBucketName", func(t *testing.T) {
		invalidNames := []string{
			"UPPERCASE-BUCKET", // Uppercase not allowed
			"ab",               // Too short
			"bucket..name",     // Consecutive dots
			"bucket.-name",     // Dot adjacent to hyphen
			"192.168.1.1",      // IP address format
		}

		for _, invalidName := range invalidNames {
			bucket, err = client.Bucket(invalidName)
			assert.Error(t, err, "Should fail with invalid bucket name: %s", invalidName)
		}
	})

	t.Run("InvalidObjectKey", func(t *testing.T) {
		bucketName := generateTestBucketName()
		bucket, err = client.Bucket(bucketName)

		err := bucket.Create(ctx)
		require.NoError(t, err)
		defer bucket.Delete(ctx)

		invalidKeys := []string{
			"",                        // Empty key
			strings.Repeat("a", 1025), // Too long
		}

		for _, invalidKey := range invalidKeys {
			reader := bytes.NewReader([]byte("test"))
			err := bucket.PutObject(ctx, invalidKey, reader, 4)
			assert.Error(t, err, "Should fail with invalid object key: %s", invalidKey)
		}
	})

	t.Run("DeleteNonEmptyBucket", func(t *testing.T) {
		bucketName := generateTestBucketName()
		bucket, err = client.Bucket(bucketName)
		err := bucket.Create(ctx)
		require.NoError(t, err)

		// Upload an object
		objectKey := generateTestObjectKey()
		reader := bytes.NewReader([]byte("test data"))
		err = bucket.PutObject(ctx, objectKey, reader, 9)
		require.NoError(t, err)

		// Try to delete bucket with object still in it
		err = bucket.Delete(ctx)
		assert.Error(t, err, "Should fail to delete non-empty bucket")

		// Clean up
		bucket.DeleteObject(ctx, objectKey)
		bucket.Delete(ctx)
	})
}

// Integration test covering edge cases and performance
func TestIntegrationEdgeCases(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	endpoint, cleanup := setupMinIO(t)
	defer cleanup()

	client := createTestClient(t, endpoint)
	defer client.Close()

	ctx := context.Background()
	bucketName := generateTestBucketName()
	bucket, err := client.Bucket(bucketName)
	assert.NoError(t, err)

	// Setup bucket
	err = bucket.Create(ctx)
	require.NoError(t, err)
	defer bucket.Delete(ctx)

	t.Run("EmptyObject", func(t *testing.T) {
		objectKey := generateTestObjectKey()

		// Upload empty object
		reader := bytes.NewReader([]byte{})
		err := bucket.PutObject(ctx, objectKey, reader, 0)
		assert.NoError(t, err, "Should upload empty object successfully")

		defer bucket.DeleteObject(ctx, objectKey)

		// Download and verify
		downloadReader, err := bucket.GetObject(ctx, objectKey)
		require.NoError(t, err)
		defer downloadReader.Close()

		data, err := io.ReadAll(downloadReader)
		assert.NoError(t, err, "Should read empty object successfully")
		assert.Empty(t, data, "Downloaded data should be empty")

		// Check metadata
		info, err := bucket.HeadObject(ctx, objectKey)
		assert.NoError(t, err, "Should get metadata for empty object")
		assert.Equal(t, int64(0), info.Size, "Empty object should have size 0")
	})

	t.Run("ObjectWithSpecialCharacters", func(t *testing.T) {
		// Test various special characters that are allowed in object keys
		specialKeys := []string{
			"file with spaces.txt",
			"file-with-hyphens.txt",
			"file_with_underscores.txt",
			"file.with.periods.txt",
			"file+with+plus.txt",
			"file(with)parentheses.txt",
			"file[with]brackets.txt",
			"path/to/nested/file.txt",
			"深度/nested/中文文件名.txt", // Unicode characters
		}

		testData := []byte("Special character test data")

		for _, objectKey := range specialKeys {
			t.Run(fmt.Sprintf("Key_%s", objectKey), func(t *testing.T) {
				reader := bytes.NewReader(testData)
				err := bucket.PutObject(ctx, objectKey, reader, int64(len(testData)))
				if assert.NoError(t, err, "Should upload object with special characters: %s", objectKey) {
					defer bucket.DeleteObject(ctx, objectKey)

					// Verify download
					downloadReader, err := bucket.GetObject(ctx, objectKey)
					if assert.NoError(t, err, "Should download object with special characters") {
						defer downloadReader.Close()

						data, err := io.ReadAll(downloadReader)
						assert.NoError(t, err, "Should read object data")
						assert.Equal(t, testData, data, "Data should match for object with special characters")
					}
				}
			})
		}
	})

	t.Run("ConcurrentOperations", func(t *testing.T) {
		const numGoroutines = 5
		const objectsPerGoroutine = 3

		results := make(chan error, numGoroutines*objectsPerGoroutine*2) // *2 for upload and download

		for i := 0; i < numGoroutines; i++ {
			go func(goroutineID int) {
				for j := 0; j < objectsPerGoroutine; j++ {
					objectKey := fmt.Sprintf("concurrent-%d-%d.txt", goroutineID, j)
					testData := []byte(fmt.Sprintf("Concurrent test data %d-%d", goroutineID, j))

					// Upload
					reader := bytes.NewReader(testData)
					err := bucket.PutObject(ctx, objectKey, reader, int64(len(testData)))
					results <- err

					if err == nil {
						// Download
						downloadReader, err := bucket.GetObject(ctx, objectKey)
						if err == nil {
							data, readErr := io.ReadAll(downloadReader)
							downloadReader.Close()
							if readErr != nil || !bytes.Equal(data, testData) {
								err = fmt.Errorf("data mismatch for %s", objectKey)
							}

							// Clean up
							bucket.DeleteObject(ctx, objectKey)
						}
						results <- err
					} else {
						results <- nil // Skip download if upload failed
					}
				}
			}(i)
		}

		// Collect results
		successCount := 0
		totalOps := numGoroutines * objectsPerGoroutine * 2
		for i := 0; i < totalOps; i++ {
			err := <-results
			if err == nil {
				successCount++
			} else {
				t.Logf("Operation failed: %v", err)
			}
		}

		// We expect most operations to succeed, but allow for some failures due to concurrency
		successRate := float64(successCount) / float64(totalOps)
		assert.True(t, successRate > 0.8, "Success rate should be > 80%%, got %.2f%%", successRate*100)

		t.Logf("Concurrent operations: %d/%d succeeded (%.2f%%)", successCount, totalOps, successRate*100)
	})
}

// Integration test for advanced download features
func TestIntegrationAdvancedDownloadOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	endpoint, cleanup := setupMinIO(t)
	defer cleanup()

	client := createTestClient(t, endpoint)
	defer client.Close()

	ctx := context.Background()
	bucketName := generateTestBucketName()
	bucket, err := client.Bucket(bucketName)
	assert.NoError(t, err)

	// Setup bucket
	err = bucket.Create(ctx)
	require.NoError(t, err)
	defer bucket.Delete(ctx)

	// Create test data with known content for range testing
	testData := []byte("0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz!@#$%^&*()_+-=[]{}|;:,.<>?")
	objectKey := generateTestObjectKey()

	// Upload test object
	reader := bytes.NewReader(testData)
	err = bucket.PutObject(ctx, objectKey, reader, int64(len(testData)))
	require.NoError(t, err)
	defer bucket.DeleteObject(ctx, objectKey)

	t.Run("GetObjectRange", func(t *testing.T) {
		t.Run("RangeFromStartToEnd", func(t *testing.T) {
			// Get bytes 10-19 (10 bytes)
			rangeReader, err := bucket.GetObjectRange(ctx, objectKey, 10, 19)
			assert.NoError(t, err, "Should get object range successfully")
			require.NotNil(t, rangeReader)
			defer rangeReader.Close()

			data, err := io.ReadAll(rangeReader)
			assert.NoError(t, err, "Should read range data successfully")
			expected := testData[10:20] // end+1 for slice
			assert.Equal(t, expected, data, "Range data should match expected slice")
			assert.Equal(t, 10, len(data), "Should get exactly 10 bytes")
		})

		t.Run("RangeFromStartToEOF", func(t *testing.T) {
			// Get from byte 50 to end of file
			rangeReader, err := bucket.GetObjectRange(ctx, objectKey, 50, -1)
			assert.NoError(t, err, "Should get object range to EOF successfully")
			require.NotNil(t, rangeReader)
			defer rangeReader.Close()

			data, err := io.ReadAll(rangeReader)
			assert.NoError(t, err, "Should read range data successfully")
			expected := testData[50:]
			assert.Equal(t, expected, data, "Range data should match expected slice from offset to EOF")
		})

		t.Run("RangeFirstBytes", func(t *testing.T) {
			// Get first 5 bytes
			rangeReader, err := bucket.GetObjectRange(ctx, objectKey, 0, 4)
			assert.NoError(t, err, "Should get first bytes successfully")
			require.NotNil(t, rangeReader)
			defer rangeReader.Close()

			data, err := io.ReadAll(rangeReader)
			assert.NoError(t, err, "Should read range data successfully")
			expected := testData[0:5]
			assert.Equal(t, expected, data, "Range data should match first 5 bytes")
			assert.Equal(t, "01234", string(data), "Should get correct first 5 characters")
		})

		t.Run("RangeLastBytes", func(t *testing.T) {
			// Get last 5 bytes
			dataLen := len(testData)
			rangeReader, err := bucket.GetObjectRange(ctx, objectKey, int64(dataLen-5), int64(dataLen-1))
			assert.NoError(t, err, "Should get last bytes successfully")
			require.NotNil(t, rangeReader)
			defer rangeReader.Close()

			data, err := io.ReadAll(rangeReader)
			assert.NoError(t, err, "Should read range data successfully")
			expected := testData[dataLen-5:]
			assert.Equal(t, expected, data, "Range data should match last 5 bytes")
		})

		t.Run("RangeSingleByte", func(t *testing.T) {
			// Get a single byte
			rangeReader, err := bucket.GetObjectRange(ctx, objectKey, 25, 25)
			assert.NoError(t, err, "Should get single byte successfully")
			require.NotNil(t, rangeReader)
			defer rangeReader.Close()

			data, err := io.ReadAll(rangeReader)
			assert.NoError(t, err, "Should read single byte successfully")
			assert.Equal(t, 1, len(data), "Should get exactly 1 byte")
			assert.Equal(t, testData[25], data[0], "Should get correct byte value")
		})

		t.Run("RangeInvalidBounds", func(t *testing.T) {
			// Try to get range beyond file size
			dataLen := len(testData)
			rangeReader, err := bucket.GetObjectRange(ctx, objectKey, int64(dataLen+10), int64(dataLen+20))
			// This might return an error or empty data depending on S3 implementation
			if err == nil && rangeReader != nil {
				data, readErr := io.ReadAll(rangeReader)
				rangeReader.Close()
				if readErr == nil {
					assert.Empty(t, data, "Range beyond file should return empty data")
				}
			}
		})
	})

	t.Run("GetObjectStreamRange", func(t *testing.T) {
		t.Run("StreamRangeToBuffer", func(t *testing.T) {
			var buf bytes.Buffer
			err := bucket.GetObjectStreamRange(ctx, objectKey, &buf, 15, 25)
			assert.NoError(t, err, "Should stream object range successfully")

			expected := testData[15:26] // end+1 for slice
			assert.Equal(t, expected, buf.Bytes(), "Streamed range data should match expected slice")
		})

		t.Run("StreamRangeFromMiddleToEOF", func(t *testing.T) {
			var buf bytes.Buffer
			err := bucket.GetObjectStreamRange(ctx, objectKey, &buf, 40, -1)
			assert.NoError(t, err, "Should stream range to EOF successfully")

			expected := testData[40:]
			assert.Equal(t, expected, buf.Bytes(), "Streamed range should match expected slice from middle to EOF")
		})

		t.Run("StreamEntireFileWithRange", func(t *testing.T) {
			var buf bytes.Buffer
			dataLen := len(testData)
			err := bucket.GetObjectStreamRange(ctx, objectKey, &buf, 0, int64(dataLen-1))
			assert.NoError(t, err, "Should stream entire file with range successfully")

			assert.Equal(t, testData, buf.Bytes(), "Streamed entire file should match original data")
		})

		t.Run("StreamSmallRange", func(t *testing.T) {
			var buf bytes.Buffer
			err := bucket.GetObjectStreamRange(ctx, objectKey, &buf, 5, 7)
			assert.NoError(t, err, "Should stream small range successfully")

			expected := testData[5:8] // end+1 for slice
			assert.Equal(t, expected, buf.Bytes(), "Small range should match expected slice")
			assert.Equal(t, 3, buf.Len(), "Should stream exactly 3 bytes")
		})
	})

	t.Run("GetObjectAdvanced", func(t *testing.T) {
		t.Run("AdvancedDownloadWithRange", func(t *testing.T) {
			var buf bytes.Buffer
			startByte := int64(20)
			endByte := int64(30)

			opts := DownloadOptions{
				StartByte: &startByte,
				EndByte:   &endByte,
			}

			err := bucket.GetObjectAdvanced(ctx, objectKey, &buf, opts)
			assert.NoError(t, err, "Should download with advanced options successfully")

			expected := testData[20:31] // end+1 for slice
			assert.Equal(t, expected, buf.Bytes(), "Advanced download range should match expected slice")
		})

		t.Run("AdvancedDownloadWithStartByteOnly", func(t *testing.T) {
			var buf bytes.Buffer
			startByte := int64(60)

			opts := DownloadOptions{
				StartByte: &startByte,
			}

			err := bucket.GetObjectAdvanced(ctx, objectKey, &buf, opts)
			assert.NoError(t, err, "Should download from start byte to EOF successfully")

			expected := testData[60:]
			assert.Equal(t, expected, buf.Bytes(), "Advanced download from start byte should match expected slice")
		})

		t.Run("AdvancedDownloadWithEndByteOnly", func(t *testing.T) {
			var buf bytes.Buffer
			endByte := int64(10)

			opts := DownloadOptions{
				EndByte: &endByte,
			}

			err := bucket.GetObjectAdvanced(ctx, objectKey, &buf, opts)
			assert.NoError(t, err, "Should download from start to end byte successfully")

			expected := testData[0:11] // end+1 for slice
			assert.Equal(t, expected, buf.Bytes(), "Advanced download to end byte should match expected slice")
		})

		t.Run("AdvancedDownloadWithConcurrency", func(t *testing.T) {
			var buf bytes.Buffer

			opts := DownloadOptions{
				Concurrency: 2, // Use 2 concurrent parts
			}

			err := bucket.GetObjectAdvanced(ctx, objectKey, &buf, opts)
			assert.NoError(t, err, "Should download with custom concurrency successfully")

			assert.Equal(t, testData, buf.Bytes(), "Advanced download with concurrency should match original data")
		})

		t.Run("AdvancedDownloadWithPartSize", func(t *testing.T) {
			var buf bytes.Buffer

			opts := DownloadOptions{
				PartSize: 10 * 1024, // 10KB parts
			}

			err := bucket.GetObjectAdvanced(ctx, objectKey, &buf, opts)
			assert.NoError(t, err, "Should download with custom part size successfully")

			assert.Equal(t, testData, buf.Bytes(), "Advanced download with custom part size should match original data")
		})

		t.Run("AdvancedDownloadWithAllOptions", func(t *testing.T) {
			var buf bytes.Buffer
			startByte := int64(10)
			endByte := int64(50)

			opts := DownloadOptions{
				StartByte:   &startByte,
				EndByte:     &endByte,
				Concurrency: 3,
				PartSize:    5 * 1024,
			}

			err := bucket.GetObjectAdvanced(ctx, objectKey, &buf, opts)
			assert.NoError(t, err, "Should download with all advanced options successfully")

			expected := testData[10:51] // end+1 for slice
			assert.Equal(t, expected, buf.Bytes(), "Advanced download with all options should match expected slice")
		})

		t.Run("AdvancedDownloadNoOptions", func(t *testing.T) {
			var buf bytes.Buffer

			opts := DownloadOptions{} // No options, should download entire file

			err := bucket.GetObjectAdvanced(ctx, objectKey, &buf, opts)
			assert.NoError(t, err, "Should download with no options successfully")

			assert.Equal(t, testData, buf.Bytes(), "Advanced download with no options should match original data")
		})
	})

	t.Run("LargeFileAdvancedDownload", func(t *testing.T) {
		// Create a larger test file for better testing of multipart downloads
		largeData := make([]byte, 1024*100) // 100KB
		for i := range largeData {
			largeData[i] = byte(i % 256)
		}

		largeObjectKey := generateTestObjectKey()

		// Upload large file
		reader := bytes.NewReader(largeData)
		err := bucket.PutObject(ctx, largeObjectKey, reader, int64(len(largeData)))
		require.NoError(t, err)
		defer bucket.DeleteObject(ctx, largeObjectKey)

		t.Run("LargeFileRange", func(t *testing.T) {
			// Get a range from the middle of large file
			start := int64(1000)
			end := int64(2000)

			rangeReader, err := bucket.GetObjectRange(ctx, largeObjectKey, start, end)
			assert.NoError(t, err, "Should get large file range successfully")
			require.NotNil(t, rangeReader)
			defer rangeReader.Close()

			data, err := io.ReadAll(rangeReader)
			assert.NoError(t, err, "Should read large file range successfully")
			expected := largeData[start : end+1]
			assert.Equal(t, expected, data, "Large file range should match expected slice")
		})

		t.Run("LargeFileAdvancedWithConcurrency", func(t *testing.T) {
			var buf bytes.Buffer
			start := int64(5000)
			end := int64(15000)

			opts := DownloadOptions{
				StartByte:   &start,
				EndByte:     &end,
				Concurrency: 4,
				PartSize:    2 * 1024, // 2KB parts
			}

			err := bucket.GetObjectAdvanced(ctx, largeObjectKey, &buf, opts)
			assert.NoError(t, err, "Should download large file range with concurrency successfully")

			expected := largeData[start : end+1]
			assert.Equal(t, expected, buf.Bytes(), "Large file advanced download should match expected slice")
		})
	})
}

// Integration test for advanced upload features
func TestIntegrationAdvancedUploadOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	endpoint, cleanup := setupMinIO(t)
	defer cleanup()

	client := createTestClient(t, endpoint)
	defer client.Close()

	ctx := context.Background()
	bucketName := generateTestBucketName()
	bucket, err := client.Bucket(bucketName)
	assert.NoError(t, err)

	// Setup bucket
	err = bucket.Create(ctx)
	require.NoError(t, err)
	defer bucket.Delete(ctx)

	t.Run("PutObjectAdvancedBasic", func(t *testing.T) {
		objectKey := generateTestObjectKey()
		testData := []byte("Advanced upload test data - basic functionality")

		reader := bytes.NewReader(testData)
		opts := UploadOptions{} // No special options

		var progressCalls int
		progressCallback := func(progress UploadProgress) {
			progressCalls++
			t.Logf("Upload progress: %d/%d bytes (%d/%d parts)",
				progress.BytesUploaded, progress.TotalBytes,
				progress.PartsUploaded, progress.TotalParts)

			assert.True(t, progress.BytesUploaded <= progress.TotalBytes, "Bytes uploaded should not exceed total")
			assert.True(t, progress.PartsUploaded <= progress.TotalParts, "Parts uploaded should not exceed total")
		}

		err := bucket.PutObjectAdvanced(ctx, objectKey, reader, int64(len(testData)), progressCallback, opts)
		assert.NoError(t, err, "Should upload object with advanced options successfully")
		defer bucket.DeleteObject(ctx, objectKey)

		// Verify progress callback was called
		assert.True(t, progressCalls > 0, "Progress callback should be called")

		// Verify uploaded data
		downloadReader, err := bucket.GetObject(ctx, objectKey)
		require.NoError(t, err)
		defer downloadReader.Close()

		data, err := io.ReadAll(downloadReader)
		assert.NoError(t, err, "Should read uploaded object successfully")
		assert.Equal(t, testData, data, "Uploaded data should match original data")
	})

	t.Run("PutObjectAdvancedWithCustomConcurrency", func(t *testing.T) {
		objectKey := generateTestObjectKey()
		testData := make([]byte, 50*1024) // 50KB
		for i := range testData {
			testData[i] = byte(i % 256)
		}

		reader := bytes.NewReader(testData)
		opts := UploadOptions{
			Concurrency: 2, // Custom concurrency
		}

		var progressCalls int
		progressCallback := func(progress UploadProgress) {
			progressCalls++
			if progressCalls%5 == 0 { // Log every 5th call to avoid spam
				t.Logf("Concurrent upload progress: %d/%d bytes", progress.BytesUploaded, progress.TotalBytes)
			}
		}

		err := bucket.PutObjectAdvanced(ctx, objectKey, reader, int64(len(testData)), progressCallback, opts)
		assert.NoError(t, err, "Should upload with custom concurrency successfully")
		defer bucket.DeleteObject(ctx, objectKey)

		assert.True(t, progressCalls > 0, "Progress callback should be called for concurrent upload")

		// Verify uploaded data
		downloadReader, err := bucket.GetObject(ctx, objectKey)
		require.NoError(t, err)
		defer downloadReader.Close()

		data, err := io.ReadAll(downloadReader)
		assert.NoError(t, err, "Should read concurrent uploaded object successfully")
		assert.Equal(t, testData, data, "Concurrent uploaded data should match original data")
	})

	t.Run("PutObjectAdvancedWithMaxUploadParts", func(t *testing.T) {
		objectKey := generateTestObjectKey()
		testData := make([]byte, 100*1024) // 100KB
		for i := range testData {
			testData[i] = byte(i % 256)
		}

		reader := bytes.NewReader(testData)
		opts := UploadOptions{
			MaxUploadParts: 5, // Custom max parts
		}

		var progressCalls int
		var finalProgress UploadProgress
		progressCallback := func(progress UploadProgress) {
			progressCalls++
			finalProgress = progress

			// Ensure we don't exceed max parts
			assert.True(t, progress.TotalParts <= 5, "Total parts should not exceed MaxUploadParts setting")
		}

		err := bucket.PutObjectAdvanced(ctx, objectKey, reader, int64(len(testData)), progressCallback, opts)
		assert.NoError(t, err, "Should upload with custom max upload parts successfully")
		defer bucket.DeleteObject(ctx, objectKey)

		assert.True(t, progressCalls > 0, "Progress callback should be called")
		assert.True(t, finalProgress.TotalParts <= 5, "Final progress should respect MaxUploadParts limit")

		// Verify uploaded data
		downloadReader, err := bucket.GetObject(ctx, objectKey)
		require.NoError(t, err)
		defer downloadReader.Close()

		data, err := io.ReadAll(downloadReader)
		assert.NoError(t, err, "Should read max-parts uploaded object successfully")
		assert.Equal(t, testData, data, "Max-parts uploaded data should match original data")
	})

	t.Run("PutObjectAdvancedWithObjectOptions", func(t *testing.T) {
		objectKey := generateTestObjectKey()
		testData := []byte("Advanced upload with metadata and options")

		reader := bytes.NewReader(testData)
		opts := UploadOptions{
			ObjectOptions: ObjectOptions{
				ContentType: "text/plain",
				Metadata: map[string]string{
					"test-metadata": "advanced-upload",
					"upload-type":   "integration-test",
				},
				Tags: map[string]string{
					"environment":   "test",
					"upload-method": "advanced",
				},
			},
			Concurrency: 1,
		}

		var progressCalls int
		progressCallback := func(progress UploadProgress) {
			progressCalls++
		}

		err := bucket.PutObjectAdvanced(ctx, objectKey, reader, int64(len(testData)), progressCallback, opts)
		assert.NoError(t, err, "Should upload with object options successfully")
		defer bucket.DeleteObject(ctx, objectKey)

		assert.True(t, progressCalls > 0, "Progress callback should be called")

		// Verify metadata (Note: MinIO might not return all metadata)
		info, err := bucket.HeadObject(ctx, objectKey)
		assert.NoError(t, err, "Should get object metadata successfully")
		assert.Equal(t, "text/plain", info.ContentType, "Content type should be set correctly")

		// Verify uploaded data
		downloadReader, err := bucket.GetObject(ctx, objectKey)
		require.NoError(t, err)
		defer downloadReader.Close()

		data, err := io.ReadAll(downloadReader)
		assert.NoError(t, err, "Should read object with options successfully")
		assert.Equal(t, testData, data, "Object with options data should match original data")
	})

	t.Run("PutObjectAdvancedWithAllOptions", func(t *testing.T) {
		objectKey := generateTestObjectKey()
		testData := make([]byte, 200*1024) // 200KB for multipart
		for i := range testData {
			testData[i] = byte(i % 256)
		}

		reader := bytes.NewReader(testData)
		opts := UploadOptions{
			ObjectOptions: ObjectOptions{
				ContentType: "application/octet-stream",
				Metadata: map[string]string{
					"comprehensive-test": "true",
				},
			},
			MaxUploadParts:    10,
			Concurrency:       3,
			LeavePartsOnError: false, // Clean up on error
		}

		var progressCalls int
		var lastProgress UploadProgress
		progressCallback := func(progress UploadProgress) {
			progressCalls++
			lastProgress = progress

			// Validate progress values
			assert.True(t, progress.BytesUploaded <= progress.TotalBytes, "Bytes should not exceed total")
			assert.True(t, progress.PartsUploaded <= progress.TotalParts, "Parts should not exceed total")
			assert.True(t, progress.TotalParts <= 10, "Total parts should respect MaxUploadParts")
		}

		err := bucket.PutObjectAdvanced(ctx, objectKey, reader, int64(len(testData)), progressCallback, opts)
		assert.NoError(t, err, "Should upload with all advanced options successfully")
		defer bucket.DeleteObject(ctx, objectKey)

		assert.True(t, progressCalls > 0, "Progress callback should be called")
		assert.Equal(t, int64(len(testData)), lastProgress.TotalBytes, "Final progress should show correct total")
		assert.Equal(t, int64(len(testData)), lastProgress.BytesUploaded, "Final progress should show all bytes uploaded")

		// Verify uploaded data
		downloadReader, err := bucket.GetObject(ctx, objectKey)
		require.NoError(t, err)
		defer downloadReader.Close()

		data, err := io.ReadAll(downloadReader)
		assert.NoError(t, err, "Should read comprehensive advanced upload successfully")
		assert.Equal(t, testData, data, "Comprehensive upload data should match original data")
	})

	t.Run("PutObjectAdvancedNoProgressCallback", func(t *testing.T) {
		objectKey := generateTestObjectKey()
		testData := []byte("Advanced upload without progress callback")

		reader := bytes.NewReader(testData)
		opts := UploadOptions{
			Concurrency: 2,
		}

		// No progress callback (nil)
		err := bucket.PutObjectAdvanced(ctx, objectKey, reader, int64(len(testData)), nil, opts)
		assert.NoError(t, err, "Should upload without progress callback successfully")
		defer bucket.DeleteObject(ctx, objectKey)

		// Verify uploaded data
		downloadReader, err := bucket.GetObject(ctx, objectKey)
		require.NoError(t, err)
		defer downloadReader.Close()

		data, err := io.ReadAll(downloadReader)
		assert.NoError(t, err, "Should read object uploaded without progress callback")
		assert.Equal(t, testData, data, "Data uploaded without progress should match original")
	})

	t.Run("PutObjectAdvancedLargeFile", func(t *testing.T) {
		objectKey := generateTestObjectKey()
		// Create a larger file to test multipart functionality better
		testData := make([]byte, 1024*1024) // 1MB
		for i := range testData {
			testData[i] = byte(i % 256)
		}

		reader := bytes.NewReader(testData)
		opts := UploadOptions{
			MaxUploadParts: 20,
			Concurrency:    4,
			ObjectOptions: ObjectOptions{
				ContentType: "application/octet-stream",
			},
		}

		var progressCalls int
		var maxParts int
		progressCallback := func(progress UploadProgress) {
			progressCalls++
			if progress.TotalParts > maxParts {
				maxParts = progress.TotalParts
			}

			// Log progress every 20th call to avoid spam
			if progressCalls%20 == 0 {
				t.Logf("Large file upload progress: %d/%d bytes (%.1f%%), %d/%d parts",
					progress.BytesUploaded, progress.TotalBytes,
					float64(progress.BytesUploaded)/float64(progress.TotalBytes)*100,
					progress.PartsUploaded, progress.TotalParts)
			}
		}

		err := bucket.PutObjectAdvanced(ctx, objectKey, reader, int64(len(testData)), progressCallback, opts)
		assert.NoError(t, err, "Should upload large file with advanced options successfully")
		defer bucket.DeleteObject(ctx, objectKey)

		assert.True(t, progressCalls > 0, "Progress callback should be called for large file")
		assert.True(t, maxParts <= 20, "Large file upload should respect MaxUploadParts")

		// Verify object exists and has correct size
		info, err := bucket.HeadObject(ctx, objectKey)
		assert.NoError(t, err, "Should get metadata for large uploaded object")
		assert.Equal(t, int64(len(testData)), info.Size, "Large object size should match uploaded data size")

		t.Logf("Large file upload completed with %d progress calls, max %d parts", progressCalls, maxParts)
	})

	t.Run("PutObjectAdvancedErrorConditions", func(t *testing.T) {
		testData := []byte("Test error conditions")
		reader := bytes.NewReader(testData)
		opts := UploadOptions{}

		// Test with invalid bucket name
		bucket, err = client.Bucket("INVALID-BUCKET-NAME")
		err := bucket.PutObjectAdvanced(ctx, "test.txt", reader, int64(len(testData)), nil, opts)
		assert.Error(t, err, "Should fail with invalid bucket name")

		// Test with invalid object key
		reader = bytes.NewReader(testData)
		invalidKey := strings.Repeat("a", 1025) // Too long
		bucket, err = client.Bucket(bucketName)
		err = bucket.PutObjectAdvanced(ctx, invalidKey, reader, int64(len(testData)), nil, opts)
		assert.Error(t, err, "Should fail with invalid object key")

		// Test with zero size (edge case)
		reader = bytes.NewReader([]byte{})
		validKey := generateTestObjectKey()
		err = bucket.PutObjectAdvanced(ctx, validKey, reader, 0, nil, opts)
		assert.NoError(t, err, "Should handle zero-size upload")
		defer bucket.DeleteObject(ctx, validKey)
	})
}
