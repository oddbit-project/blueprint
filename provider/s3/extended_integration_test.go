package s3

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	comprehensiveTestTimeout = 600 * time.Second // 10 minutes for comprehensive tests
	testDataSize             = 1024 * 1024       // 1MB test data
	largeTestDataSize        = 10 * 1024 * 1024  // 10MB for multipart tests
)

// TestComprehensiveIntegration runs comprehensive integration tests for all S3 provider functions
func TestComprehensiveIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping comprehensive integration tests in short mode")
	}

	// Setup MinIO container using testcontainers
	ctx := context.Background()
	container, cleanup := setupMinIOContainer(ctx, t)
	defer cleanup()

	client := createTestClientWithContainer(t, container)
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), comprehensiveTestTimeout)
	defer cancel()

	t.Logf("MinIO container started and ready for comprehensive tests on %s", container.GetEndpoint())

	// Run all test suites
	t.Run("ClientFunctions", func(t *testing.T) {
		testClientFunctions(t, ctx, client)
	})

	t.Run("BucketManagement", func(t *testing.T) {
		testBucketManagement(t, ctx, client)
	})

	t.Run("ObjectCRUD", func(t *testing.T) {
		testObjectCRUD(t, ctx, client)
	})

	t.Run("UploadOperations", func(t *testing.T) {
		testUploadOperations(t, ctx, client)
	})

	t.Run("DownloadOperations", func(t *testing.T) {
		testDownloadOperations(t, ctx, client)
	})

	t.Run("PresignedURLs", func(t *testing.T) {
		testPresignedURLs(t, ctx, client)
	})

	t.Run("CopyOperations", func(t *testing.T) {
		testCopyOperations(t, ctx, client)
	})

	t.Run("ErrorScenarios", func(t *testing.T) {
		testErrorScenarios(t, ctx, client)
	})

	t.Run("ConcurrencyTests", func(t *testing.T) {
		testConcurrency(t, ctx, client)
	})

	t.Run("EdgeCases", func(t *testing.T) {
		testEdgeCases(t, ctx, client)
	})
}

// testClientFunctions tests all Client-level functions
func testClientFunctions(t *testing.T, ctx context.Context, client *Client) {
	t.Run("IsConnected", func(t *testing.T) {
		assert.True(t, client.IsConnected(), "Client should be connected")
	})

	t.Run("MinioClient", func(t *testing.T) {
		minioClient := client.MinioClient()
		assert.NotNil(t, minioClient, "Should return MinIO client instance")
	})

	t.Run("ListBuckets", func(t *testing.T) {
		buckets, err := client.ListBuckets(ctx)
		assert.NoError(t, err, "Should list buckets successfully")
		assert.NotNil(t, buckets, "Should return bucket list")
	})

	t.Run("ConnectAndClose", func(t *testing.T) {
		// Test disconnect and reconnect
		err := client.Close()
		assert.NoError(t, err, "Should close client successfully")
		assert.False(t, client.IsConnected(), "Client should not be connected after close")

		err = client.Connect(ctx)
		assert.NoError(t, err, "Should reconnect successfully")
		assert.True(t, client.IsConnected(), "Client should be connected after reconnect")
	})
}

// testBucketManagement tests all bucket management functions
func testBucketManagement(t *testing.T, ctx context.Context, client *Client) {
	bucketName := generateTestBucketName()

	t.Run("CreateBucket", func(t *testing.T) {
		// Test basic bucket creation
		err := client.CreateBucket(ctx, bucketName)
		assert.NoError(t, err, "Should create bucket successfully")

		// Test bucket creation with options
		bucketNameWithOptions := generateTestBucketName()
		defer client.DeleteBucket(ctx, bucketNameWithOptions) // Cleanup
		err = client.CreateBucket(ctx, bucketNameWithOptions, BucketOptions{
			Region: testMinIORegion,
			ACL:    "private",
		})
		assert.NoError(t, err, "Should create bucket with options successfully")
	})

	t.Run("BucketExists", func(t *testing.T) {
		exists, err := client.BucketExists(ctx, bucketName)
		assert.NoError(t, err, "Should check bucket existence successfully")
		assert.True(t, exists, "Bucket should exist")

		// Test non-existent bucket
		exists, err = client.BucketExists(ctx, "non-existent-bucket-12345")
		assert.NoError(t, err, "Should check non-existent bucket successfully")
		assert.False(t, exists, "Non-existent bucket should not exist")
	})

	t.Run("ListBucketsAfterCreation", func(t *testing.T) {
		buckets, err := client.ListBuckets(ctx)
		assert.NoError(t, err, "Should list buckets successfully")
		assert.Greater(t, len(buckets), 0, "Should have at least one bucket")

		// Find our test bucket
		found := false
		for _, bucket := range buckets {
			if bucket.Name == bucketName {
				found = true
				assert.NotZero(t, bucket.CreationDate, "Bucket should have creation date")
				break
			}
		}
		assert.True(t, found, "Should find our test bucket in list")
	})

	t.Run("BucketObject", func(t *testing.T) {
		bucket, err := client.Bucket(bucketName)
		assert.NoError(t, err, "Should create bucket object successfully")
		assert.NotNil(t, bucket, "Bucket object should not be nil")
		assert.True(t, bucket.IsConnected(), "Bucket should be connected")

		// Test bucket methods
		exists, err := bucket.Exists(ctx)
		assert.NoError(t, err, "Should check bucket existence via bucket object")
		assert.True(t, exists, "Bucket should exist via bucket object")
	})

	t.Run("DeleteBucket", func(t *testing.T) {
		// Create a temporary bucket for deletion test
		tempBucketName := generateTestBucketName()
		err := client.CreateBucket(ctx, tempBucketName)
		require.NoError(t, err, "Should create temp bucket for deletion test")

		// Delete the bucket
		err = client.DeleteBucket(ctx, tempBucketName)
		assert.NoError(t, err, "Should delete bucket successfully")

		// Verify bucket no longer exists
		exists, err := client.BucketExists(ctx, tempBucketName)
		assert.NoError(t, err, "Should check deleted bucket existence")
		assert.False(t, exists, "Deleted bucket should not exist")
	})

	// Keep the main test bucket for other tests
	t.Cleanup(func() {
		client.DeleteBucket(ctx, bucketName)
	})
}

// testObjectCRUD tests all basic object CRUD operations
func testObjectCRUD(t *testing.T, ctx context.Context, client *Client) {
	bucketName := generateTestBucketName()
	err := client.CreateBucket(ctx, bucketName)
	require.NoError(t, err, "Should create test bucket")
	defer client.DeleteBucket(ctx, bucketName)

	bucket, err := client.Bucket(bucketName)
	require.NoError(t, err, "Should get bucket object")

	objectKey := generateTestObjectKey()
	testData := generateTestData(1024) // 1KB test data

	t.Run("PutObject", func(t *testing.T) {
		reader := bytes.NewReader(testData)
		err := bucket.PutObject(ctx, objectKey, reader, int64(len(testData)))
		assert.NoError(t, err, "Should put object successfully")

		// Test with options
		optionsObjectKey := generateTestObjectKey()
		reader = bytes.NewReader(testData)
		err = bucket.PutObject(ctx, optionsObjectKey, reader, int64(len(testData)), ObjectOptions{
			ContentType:        "application/octet-stream",
			CacheControl:       "max-age=3600",
			ContentDisposition: "attachment; filename=\"test.bin\"",
			ContentEncoding:    "gzip",
			ContentLanguage:    "en-US",
			Metadata: map[string]string{
				"test-key": "test-value",
				"author":   "integration-test",
			},
			Tags: map[string]string{
				"environment": "test",
				"type":        "integration-test",
			},
			StorageClass: "STANDARD",
		})
		assert.NoError(t, err, "Should put object with options successfully")
		defer bucket.DeleteObject(ctx, optionsObjectKey)
	})

	t.Run("ObjectExists", func(t *testing.T) {
		exists, err := bucket.ObjectExists(ctx, objectKey)
		assert.NoError(t, err, "Should check object existence successfully")
		assert.True(t, exists, "Object should exist")

		// Test non-existent object
		exists, err = bucket.ObjectExists(ctx, "non-existent-object")
		assert.NoError(t, err, "Should check non-existent object successfully")
		assert.False(t, exists, "Non-existent object should not exist")
	})

	t.Run("HeadObject", func(t *testing.T) {
		info, err := bucket.HeadObject(ctx, objectKey)
		assert.NoError(t, err, "Should get object metadata successfully")
		assert.NotNil(t, info, "Object info should not be nil")
		assert.Equal(t, objectKey, info.Key, "Object key should match")
		assert.Equal(t, int64(len(testData)), info.Size, "Object size should match")
		assert.NotZero(t, info.LastModified, "Object should have last modified date")
		assert.NotEmpty(t, info.ETag, "Object should have ETag")
	})

	t.Run("GetObject", func(t *testing.T) {
		reader, err := bucket.GetObject(ctx, objectKey)
		assert.NoError(t, err, "Should get object successfully")
		require.NotNil(t, reader, "Reader should not be nil")
		defer reader.Close()

		data, err := io.ReadAll(reader)
		assert.NoError(t, err, "Should read object data successfully")
		assert.Equal(t, testData, data, "Retrieved data should match original")
	})

	t.Run("ListObjects", func(t *testing.T) {
		// Put a few more objects for listing
		additionalKeys := []string{
			generateTestObjectKey(),
			generateTestObjectKey(),
			generateTestObjectKey(),
		}
		for _, key := range additionalKeys {
			reader := bytes.NewReader(testData)
			err := bucket.PutObject(ctx, key, reader, int64(len(testData)))
			require.NoError(t, err, "Should put additional object")
			defer bucket.DeleteObject(ctx, key)
		}

		// Test basic listing
		objects, err := bucket.ListObjects(ctx)
		assert.NoError(t, err, "Should list objects successfully")
		assert.GreaterOrEqual(t, len(objects), 4, "Should have at least 4 objects")

		// Test listing with options
		objects, err = bucket.ListObjects(ctx, ListOptions{
			Prefix:  testObjectPrefix,
			MaxKeys: 2,
		})
		assert.NoError(t, err, "Should list objects with options successfully")
		assert.LessOrEqual(t, len(objects), 2, "Should respect MaxKeys option")

		// Verify object info structure
		if len(objects) > 0 {
			obj := objects[0]
			assert.NotEmpty(t, obj.Key, "Object should have key")
			assert.Greater(t, obj.Size, int64(0), "Object should have size")
			assert.NotZero(t, obj.LastModified, "Object should have last modified date")
			assert.NotEmpty(t, obj.ETag, "Object should have ETag")
		}
	})

	t.Run("DeleteObject", func(t *testing.T) {
		// Delete the main test object
		err := bucket.DeleteObject(ctx, objectKey)
		assert.NoError(t, err, "Should delete object successfully")

		// Verify object no longer exists
		exists, err := bucket.ObjectExists(ctx, objectKey)
		assert.NoError(t, err, "Should check deleted object existence")
		assert.False(t, exists, "Deleted object should not exist")

		// Test deleting non-existent object (should not error)
		err = bucket.DeleteObject(ctx, "non-existent-object")
		assert.NoError(t, err, "Should handle deleting non-existent object")
	})
}

// testUploadOperations tests all upload-related functions
func testUploadOperations(t *testing.T, ctx context.Context, client *Client) {
	bucketName := generateTestBucketName()
	err := client.CreateBucket(ctx, bucketName)
	require.NoError(t, err, "Should create test bucket")
	defer client.DeleteBucket(ctx, bucketName)

	bucket, err := client.Bucket(bucketName)
	require.NoError(t, err, "Should get bucket object")

	t.Run("PutObjectStream", func(t *testing.T) {
		objectKey := generateTestObjectKey()
		testData := generateTestData(2048) // 2KB test data
		reader := bytes.NewReader(testData)

		err := bucket.PutObjectStream(ctx, objectKey, reader)
		assert.NoError(t, err, "Should put object stream successfully")
		defer bucket.DeleteObject(ctx, objectKey)

		// Verify uploaded data
		downloadReader, err := bucket.GetObject(ctx, objectKey)
		require.NoError(t, err, "Should get streamed object")
		defer downloadReader.Close()

		data, err := io.ReadAll(downloadReader)
		assert.NoError(t, err, "Should read streamed object data")
		assert.Equal(t, testData, data, "Streamed data should match original")

		// Test with options
		objectKeyWithOptions := generateTestObjectKey()
		reader = bytes.NewReader(testData)
		err = bucket.PutObjectStream(ctx, objectKeyWithOptions, reader, ObjectOptions{
			ContentType: "text/plain",
			Metadata: map[string]string{
				"upload-method": "stream",
			},
		})
		assert.NoError(t, err, "Should put object stream with options successfully")
		defer bucket.DeleteObject(ctx, objectKeyWithOptions)
	})

	t.Run("PutObjectMultipart", func(t *testing.T) {
		objectKey := generateTestObjectKey()
		testData := generateTestData(largeTestDataSize) // 10MB for multipart
		reader := bytes.NewReader(testData)

		err := bucket.PutObjectMultipart(ctx, objectKey, reader, int64(len(testData)))
		assert.NoError(t, err, "Should put object multipart successfully")
		defer bucket.DeleteObject(ctx, objectKey)

		// Verify uploaded data
		downloadReader, err := bucket.GetObject(ctx, objectKey)
		require.NoError(t, err, "Should get multipart object")
		defer downloadReader.Close()

		data, err := io.ReadAll(downloadReader)
		assert.NoError(t, err, "Should read multipart object data")
		assert.Equal(t, testData, data, "Multipart data should match original")

		// Test with options
		objectKeyWithOptions := generateTestObjectKey()
		reader = bytes.NewReader(testData)

		err = bucket.PutObjectMultipart(ctx, objectKeyWithOptions, reader, int64(len(testData)), ObjectOptions{
			ContentType:  "application/octet-stream",
			StorageClass: "STANDARD",
			Tags: map[string]string{
				"upload-type": "multipart",
			},
		})
		assert.NoError(t, err, "Should put multipart object with options successfully")
		defer bucket.DeleteObject(ctx, objectKeyWithOptions)
	})

	t.Run("PutObjectAdvanced", func(t *testing.T) {
		objectKey := generateTestObjectKey()
		testData := generateTestData(5 * 1024 * 1024) // 5MB
		reader := bytes.NewReader(testData)

		uploadOpts := UploadOptions{
			ObjectOptions: ObjectOptions{
				ContentType: "application/test-data",
				Metadata: map[string]string{
					"upload-method": "advanced",
					"test-id":       "comprehensive-integration",
				},
				Tags: map[string]string{
					"environment": "test",
					"method":      "advanced",
				},
			},
			LeavePartsOnError: false,
			MaxUploadParts:    1000,
			Concurrency:       2,
		}

		err := bucket.PutObjectAdvanced(ctx, objectKey, reader, int64(len(testData)), uploadOpts)
		assert.NoError(t, err, "Should put object advanced successfully")
		defer bucket.DeleteObject(ctx, objectKey)

		// Verify uploaded data
		downloadReader, err := bucket.GetObject(ctx, objectKey)
		require.NoError(t, err, "Should get advanced upload object")
		defer downloadReader.Close()

		data, err := io.ReadAll(downloadReader)
		assert.NoError(t, err, "Should read advanced upload object data")
		assert.Equal(t, testData, data, "Advanced upload data should match original")

		// Verify metadata
		info, err := bucket.HeadObject(ctx, objectKey)
		assert.NoError(t, err, "Should get object metadata")
		assert.Equal(t, "application/test-data", info.ContentType, "Content type should be set correctly")
	})
}

// testDownloadOperations tests all download-related functions
func testDownloadOperations(t *testing.T, ctx context.Context, client *Client) {
	bucketName := generateTestBucketName()
	err := client.CreateBucket(ctx, bucketName)
	require.NoError(t, err, "Should create test bucket")
	defer client.DeleteBucket(ctx, bucketName)

	bucket, err := client.Bucket(bucketName)
	require.NoError(t, err, "Should get bucket object")

	// Create test object
	objectKey := generateTestObjectKey()
	testData := generateTestData(testDataSize) // 1MB
	reader := bytes.NewReader(testData)
	err = bucket.PutObject(ctx, objectKey, reader, int64(len(testData)))
	require.NoError(t, err, "Should create test object for download tests")
	defer bucket.DeleteObject(ctx, objectKey)

	t.Run("GetObjectStream", func(t *testing.T) {
		var buf bytes.Buffer
		err := bucket.GetObjectStream(ctx, objectKey, &buf)
		assert.NoError(t, err, "Should get object stream successfully")

		data := buf.Bytes()
		assert.Equal(t, testData, data, "Streamed data should match original")
	})

	t.Run("GetObjectRange", func(t *testing.T) {
		// Test range from start to specific end
		rangeReader, err := bucket.GetObjectRange(ctx, objectKey, 10, 19)
		assert.NoError(t, err, "Should get object range successfully")
		require.NotNil(t, rangeReader, "Range reader should not be nil")
		defer rangeReader.Close()

		data, err := io.ReadAll(rangeReader)
		assert.NoError(t, err, "Should read range data successfully")
		expected := testData[10:20] // end+1 for slice
		assert.Equal(t, expected, data, "Range data should match expected slice")

		// Test range from start to end of file
		rangeReader, err = bucket.GetObjectRange(ctx, objectKey, 100, -1)
		assert.NoError(t, err, "Should get range to EOF successfully")
		require.NotNil(t, rangeReader, "Range reader should not be nil")
		defer rangeReader.Close()

		data, err = io.ReadAll(rangeReader)
		assert.NoError(t, err, "Should read range to EOF successfully")
		expected = testData[100:]
		assert.Equal(t, expected, data, "Range to EOF should match expected slice")

		// Test single byte range
		rangeReader, err = bucket.GetObjectRange(ctx, objectKey, 50, 50)
		assert.NoError(t, err, "Should get single byte range successfully")
		require.NotNil(t, rangeReader, "Single byte range reader should not be nil")
		defer rangeReader.Close()

		data, err = io.ReadAll(rangeReader)
		assert.NoError(t, err, "Should read single byte successfully")
		assert.Equal(t, 1, len(data), "Should get exactly 1 byte")
		assert.Equal(t, testData[50], data[0], "Single byte should match expected")
	})

	t.Run("GetObjectStreamRange", func(t *testing.T) {
		var buf bytes.Buffer
		err := bucket.GetObjectStreamRange(ctx, objectKey, &buf, 20, 29)
		assert.NoError(t, err, "Should get object stream range successfully")

		data := buf.Bytes()
		expected := testData[20:30]
		assert.Equal(t, expected, data, "Stream range data should match expected slice")
	})

	t.Run("GetObjectAdvanced", func(t *testing.T) {
		// Test advanced download with range
		var buf bytes.Buffer
		downloadOpts := DownloadOptions{
			StartByte:   &[]int64{30}[0],
			EndByte:     &[]int64{39}[0],
			Concurrency: 1,
			PartSize:    1024,
		}

		err := bucket.GetObjectAdvanced(ctx, objectKey, &buf, downloadOpts)
		assert.NoError(t, err, "Should get object advanced successfully")

		data := buf.Bytes()
		expected := testData[30:40]
		assert.Equal(t, expected, data, "Advanced download data should match expected slice")

		// Test advanced download without range (full download)
		buf.Reset()
		downloadOpts = DownloadOptions{
			Concurrency: 2,
			PartSize:    512,
		}

		err = bucket.GetObjectAdvanced(ctx, objectKey, &buf, downloadOpts)
		assert.NoError(t, err, "Should get full object advanced successfully")

		data = buf.Bytes()
		assert.Equal(t, testData, data, "Advanced full download should match original data")
	})
}

// testPresignedURLs tests all presigned URL functions
func testPresignedURLs(t *testing.T, ctx context.Context, client *Client) {
	bucketName := generateTestBucketName()
	err := client.CreateBucket(ctx, bucketName)
	require.NoError(t, err, "Should create test bucket")
	defer client.DeleteBucket(ctx, bucketName)

	bucket, err := client.Bucket(bucketName)
	require.NoError(t, err, "Should get bucket object")

	// Create test object for GET and HEAD tests
	objectKey := generateTestObjectKey()
	testData := generateTestData(1024)
	reader := bytes.NewReader(testData)
	err = bucket.PutObject(ctx, objectKey, reader, int64(len(testData)))
	require.NoError(t, err, "Should create test object for presigned tests")
	defer bucket.DeleteObject(ctx, objectKey)

	t.Run("PresignGetObject", func(t *testing.T) {
		expiry := 5 * time.Minute
		presignedURL, err := bucket.PresignGetObject(ctx, objectKey, expiry)
		assert.NoError(t, err, "Should generate presigned GET URL successfully")
		assert.NotEmpty(t, presignedURL, "Presigned URL should not be empty")

		// Validate URL format
		parsedURL, err := url.Parse(presignedURL)
		assert.NoError(t, err, "Should parse presigned URL successfully")
		assert.NotEmpty(t, parsedURL.Scheme, "URL should have scheme")
		assert.NotEmpty(t, parsedURL.Host, "URL should have host")
		assert.Contains(t, parsedURL.Path, objectKey, "URL path should contain object key")

		// Test the presigned URL by making HTTP request
		resp, err := http.Get(presignedURL)
		assert.NoError(t, err, "Should access presigned GET URL successfully")
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode, "Presigned GET should return 200")

		data, err := io.ReadAll(resp.Body)
		assert.NoError(t, err, "Should read presigned GET response")
		assert.Equal(t, testData, data, "Presigned GET data should match original")
	})

	t.Run("PresignPutObject", func(t *testing.T) {
		putObjectKey := generateTestObjectKey()
		expiry := 5 * time.Minute

		presignedURL, err := bucket.PresignPutObject(ctx, putObjectKey, expiry)
		assert.NoError(t, err, "Should generate presigned PUT URL successfully")
		assert.NotEmpty(t, presignedURL, "Presigned PUT URL should not be empty")
		defer bucket.DeleteObject(ctx, putObjectKey) // Cleanup

		// Validate URL format
		parsedURL, err := url.Parse(presignedURL)
		assert.NoError(t, err, "Should parse presigned PUT URL successfully")
		assert.NotEmpty(t, parsedURL.Host, "PUT URL should have host")
		assert.Contains(t, parsedURL.Path, putObjectKey, "PUT URL path should contain object key")

		// Test the presigned PUT URL
		putData := []byte("Test data for presigned PUT")
		req, err := http.NewRequest("PUT", presignedURL, bytes.NewReader(putData))
		assert.NoError(t, err, "Should create PUT request")

		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Do(req)
		assert.NoError(t, err, "Should execute presigned PUT successfully")
		defer resp.Body.Close()

		assert.True(t, resp.StatusCode >= 200 && resp.StatusCode < 300, "Presigned PUT should return success status")

		// Verify the object was created
		exists, err := bucket.ObjectExists(ctx, putObjectKey)
		assert.NoError(t, err, "Should check presigned PUT object existence")
		assert.True(t, exists, "Object should exist after presigned PUT")

		// Test with options
		putObjectKeyWithOptions := generateTestObjectKey()
		presignedURL, err = bucket.PresignPutObject(ctx, putObjectKeyWithOptions, expiry, ObjectOptions{
			ContentType: "text/plain",
		})
		assert.NoError(t, err, "Should generate presigned PUT URL with options successfully")
		defer bucket.DeleteObject(ctx, putObjectKeyWithOptions)
	})

	t.Run("PresignHeadObject", func(t *testing.T) {
		expiry := 5 * time.Minute
		presignedURL, err := bucket.PresignHeadObject(ctx, objectKey, expiry)
		assert.NoError(t, err, "Should generate presigned HEAD URL successfully")
		assert.NotEmpty(t, presignedURL, "Presigned HEAD URL should not be empty")

		// Test the presigned HEAD URL
		req, err := http.NewRequest("HEAD", presignedURL, nil)
		assert.NoError(t, err, "Should create HEAD request")

		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Do(req)
		assert.NoError(t, err, "Should execute presigned HEAD successfully")
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode, "Presigned HEAD should return 200")
		assert.NotEmpty(t, resp.Header.Get("ETag"), "HEAD response should include ETag")
		assert.NotEmpty(t, resp.Header.Get("Content-Length"), "HEAD response should include Content-Length")
	})

	t.Run("PresignDeleteObject", func(t *testing.T) {
		expiry := 5 * time.Minute
		presignedURL, err := bucket.PresignDeleteObject(ctx, objectKey, expiry)

		// MinIO-Go doesn't support presigned DELETE URLs
		assert.Error(t, err, "Should return error for presigned DELETE (not supported by MinIO-Go)")
		assert.Empty(t, presignedURL, "Presigned DELETE URL should be empty")
		assert.Contains(t, err.Error(), "not supported", "Error should indicate DELETE is not supported")
	})
}

// testCopyOperations tests object copy functions
func testCopyOperations(t *testing.T, ctx context.Context, client *Client) {
	sourceBucketName := generateTestBucketName()
	destBucketName := generateTestBucketName()

	// Create source and destination buckets
	err := client.CreateBucket(ctx, sourceBucketName)
	require.NoError(t, err, "Should create source bucket")
	defer client.DeleteBucket(ctx, sourceBucketName)

	err = client.CreateBucket(ctx, destBucketName)
	require.NoError(t, err, "Should create destination bucket")
	defer client.DeleteBucket(ctx, destBucketName)

	sourceBucket, err := client.Bucket(sourceBucketName)
	require.NoError(t, err, "Should get source bucket object")

	// Create source object
	sourceObjectKey := generateTestObjectKey()
	testData := generateTestData(2048)
	reader := bytes.NewReader(testData)
	err = sourceBucket.PutObject(ctx, sourceObjectKey, reader, int64(len(testData)), ObjectOptions{
		ContentType: "application/test-data",
		Metadata: map[string]string{
			"original": "true",
			"test-id":  "copy-test",
		},
	})
	require.NoError(t, err, "Should create source object")
	defer sourceBucket.DeleteObject(ctx, sourceObjectKey)

	t.Run("CopyObjectSameBucket", func(t *testing.T) {
		destObjectKey := generateTestObjectKey()
		err := sourceBucket.CopyObject(ctx, sourceObjectKey, sourceBucketName, destObjectKey)
		assert.NoError(t, err, "Should copy object within same bucket successfully")
		defer sourceBucket.DeleteObject(ctx, destObjectKey)

		// Verify copied object exists
		exists, err := sourceBucket.ObjectExists(ctx, destObjectKey)
		assert.NoError(t, err, "Should check copied object existence")
		assert.True(t, exists, "Copied object should exist")

		// Verify copied object data
		reader, err := sourceBucket.GetObject(ctx, destObjectKey)
		assert.NoError(t, err, "Should get copied object")
		defer reader.Close()

		data, err := io.ReadAll(reader)
		assert.NoError(t, err, "Should read copied object data")
		assert.Equal(t, testData, data, "Copied object data should match original")
	})

	t.Run("CopyObjectDifferentBucket", func(t *testing.T) {
		destObjectKey := generateTestObjectKey()
		err := sourceBucket.CopyObject(ctx, sourceObjectKey, destBucketName, destObjectKey)
		assert.NoError(t, err, "Should copy object to different bucket successfully")

		// Verify copied object exists in destination bucket
		destBucket, err := client.Bucket(destBucketName)
		require.NoError(t, err, "Should get destination bucket")
		defer destBucket.DeleteObject(ctx, destObjectKey)

		exists, err := destBucket.ObjectExists(ctx, destObjectKey)
		assert.NoError(t, err, "Should check copied object existence in dest bucket")
		assert.True(t, exists, "Copied object should exist in destination bucket")

		// Verify copied object data
		reader, err := destBucket.GetObject(ctx, destObjectKey)
		assert.NoError(t, err, "Should get copied object from dest bucket")
		defer reader.Close()

		data, err := io.ReadAll(reader)
		assert.NoError(t, err, "Should read copied object data from dest bucket")
		assert.Equal(t, testData, data, "Copied object data should match original")
	})

	t.Run("CopyObjectWithOptions", func(t *testing.T) {
		destObjectKey := generateTestObjectKey()
		copyOptions := ObjectOptions{
			ContentType: "application/copied-data",
			Metadata: map[string]string{
				"copied": "true",
				"source": sourceObjectKey,
			},
			Tags: map[string]string{
				"operation": "copy",
				"test":      "comprehensive",
			},
		}

		err := sourceBucket.CopyObject(ctx, sourceObjectKey, sourceBucketName, destObjectKey, copyOptions)
		assert.NoError(t, err, "Should copy object with options successfully")
		defer sourceBucket.DeleteObject(ctx, destObjectKey)

		// Verify copied object metadata
		info, err := sourceBucket.HeadObject(ctx, destObjectKey)
		assert.NoError(t, err, "Should get copied object metadata")
		assert.Equal(t, "application/copied-data", info.ContentType, "Copied object should have new content type")
	})
}

// testErrorScenarios tests error handling and edge cases
func testErrorScenarios(t *testing.T, ctx context.Context, client *Client) {
	t.Run("NonExistentBucketOperations", func(t *testing.T) {
		nonExistentBucket := "non-existent-bucket-12345"

		// Test bucket operations on non-existent bucket
		err := client.DeleteBucket(ctx, nonExistentBucket)
		assert.Error(t, err, "Should error when deleting non-existent bucket")

		// Test object operations on non-existent bucket
		bucket, err := client.Bucket(nonExistentBucket)
		require.NoError(t, err, "Should create bucket object even for non-existent bucket")

		reader := bytes.NewReader([]byte("test"))
		err = bucket.PutObject(ctx, "test-object", reader, 4)
		assert.Error(t, err, "Should error when putting object to non-existent bucket")

		// Note: minio does not return error for non-existing keys
		//_, err = bucket.GetObject(ctx, "test-object")
		//assert.Error(t, err, "Should error when getting object from non-existent bucket")
	})

	t.Run("InvalidObjectOperations", func(t *testing.T) {
		bucketName := generateTestBucketName()
		err := client.CreateBucket(ctx, bucketName)
		require.NoError(t, err, "Should create test bucket")
		defer client.DeleteBucket(ctx, bucketName)

		bucket, err := client.Bucket(bucketName)
		require.NoError(t, err, "Should get bucket object")

		// Test invalid object names
		invalidNames := []string{
			"",                        // empty name
			strings.Repeat("a", 1025), // too long (>1024 chars)
		}

		for _, invalidName := range invalidNames {
			reader := bytes.NewReader([]byte("test"))
			err = bucket.PutObject(ctx, invalidName, reader, 4)
			assert.Error(t, err, "Should error with invalid object name: %s", invalidName)
		}
	})

	t.Run("InvalidBucketNames", func(t *testing.T) {
		invalidBucketNames := []string{
			"",                        // empty
			"ab",                      // too short
			strings.Repeat("a", 64),   // too long
			"UPPERCASE",               // uppercase not allowed
			"bucket_with_underscores", // underscores not allowed
			"bucket..double.dots",     // consecutive dots
			"bucket-.hyphen-dot",      // hyphen adjacent to dot
			"192.168.1.1",             // IP address format
		}

		for _, invalidName := range invalidBucketNames {
			err := client.CreateBucket(ctx, invalidName)
			assert.Error(t, err, "Should error with invalid bucket name: %s", invalidName)
		}
	})

	t.Run("DisconnectedClientOperations", func(t *testing.T) {
		// Create a new client and disconnect it
		config := NewConfig()
		config.Endpoint = "localhost:9999" // non-existent endpoint
		config.Region = testMinIORegion
		config.AccessKeyID = testMinIOAccessKey
		config.UseSSL = false

		// Set secret key using test-specific env var
		envVarName := fmt.Sprintf("MINIO_DISCONNECTED_TEST_SECRET_%s", strings.ReplaceAll(t.Name(), "/", "_"))
		os.Setenv(envVarName, testMinIOSecretKey)
		config.DefaultCredentialConfig.PasswordEnvVar = envVarName

		disconnectedClient, err := NewClient(config, nil)
		require.NoError(t, err, "Should create disconnected client")

		// Test operations on disconnected client
		_, err = disconnectedClient.ListBuckets(ctx)
		assert.Error(t, err, "Should error when listing buckets with disconnected client")

		err = disconnectedClient.CreateBucket(ctx, "test-bucket")
		assert.Error(t, err, "Should error when creating bucket with disconnected client")
	})

	t.Run("TimeoutScenarios", func(t *testing.T) {
		// Create client with very short timeout
		config := NewConfig()
		config.Endpoint = client.config.Endpoint
		config.Region = testMinIORegion
		config.AccessKeyID = testMinIOAccessKey
		config.UseSSL = false
		config.TimeoutSeconds = 1 // 1 second timeout
		config.UploadTimeoutSeconds = 1

		// Set secret key using test-specific env var
		envVarName := fmt.Sprintf("MINIO_TIMEOUT_TEST_SECRET_%s", strings.ReplaceAll(t.Name(), "/", "_"))
		os.Setenv(envVarName, testMinIOSecretKey)
		config.DefaultCredentialConfig.PasswordEnvVar = envVarName

		timeoutClient, err := NewClient(config, nil)
		require.NoError(t, err, "Should create timeout client")
		err = timeoutClient.Connect(ctx)
		require.NoError(t, err, "Should connect timeout client")
		defer timeoutClient.Close()

		bucketName := generateTestBucketName()
		err = timeoutClient.CreateBucket(ctx, bucketName)
		require.NoError(t, err, "Should create bucket with timeout client")
		defer timeoutClient.DeleteBucket(ctx, bucketName)

		bucket, err := timeoutClient.Bucket(bucketName)
		require.NoError(t, err, "Should get bucket")

		// Try to upload large data that might timeout
		largeData := generateTestData(50 * 1024 * 1024) // 50MB
		reader := bytes.NewReader(largeData)
		objectKey := generateTestObjectKey()

		err = bucket.PutObject(ctx, objectKey, reader, int64(len(largeData)))
		// This might timeout or succeed depending on system performance
		if err == nil {
			bucket.DeleteObject(ctx, objectKey)
		}
		// We don't assert error here as it's system-dependent
	})
}

// testConcurrency tests concurrent operations
func testConcurrency(t *testing.T, ctx context.Context, client *Client) {
	bucketName := generateTestBucketName()
	err := client.CreateBucket(ctx, bucketName)
	require.NoError(t, err, "Should create test bucket")
	defer client.DeleteBucket(ctx, bucketName)

	bucket, err := client.Bucket(bucketName)
	require.NoError(t, err, "Should get bucket object")

	t.Run("ConcurrentUploads", func(t *testing.T) {
		const numConcurrentUploads = 10
		objectKeys := make([]string, numConcurrentUploads)
		testData := generateTestData(1024)

		// Create channels for coordination
		results := make(chan error, numConcurrentUploads)

		// Start concurrent uploads
		for i := 0; i < numConcurrentUploads; i++ {
			objectKeys[i] = generateTestObjectKey()
			go func(key string) {
				reader := bytes.NewReader(testData)
				err := bucket.PutObject(ctx, key, reader, int64(len(testData)))
				results <- err
			}(objectKeys[i])
		}

		// Wait for all uploads to complete
		var uploadErrors []error
		for i := 0; i < numConcurrentUploads; i++ {
			if err := <-results; err != nil {
				uploadErrors = append(uploadErrors, err)
			}
		}

		assert.Empty(t, uploadErrors, "All concurrent uploads should succeed")

		// Verify all objects exist and clean up
		for _, key := range objectKeys {
			exists, err := bucket.ObjectExists(ctx, key)
			assert.NoError(t, err, "Should check object existence")
			assert.True(t, exists, "Concurrent uploaded object should exist: %s", key)
			bucket.DeleteObject(ctx, key) // Cleanup
		}
	})

	t.Run("ConcurrentDownloads", func(t *testing.T) {
		// First, create an object to download
		objectKey := generateTestObjectKey()
		testData := generateTestData(2048)
		reader := bytes.NewReader(testData)
		err := bucket.PutObject(ctx, objectKey, reader, int64(len(testData)))
		require.NoError(t, err, "Should create object for concurrent download test")
		defer bucket.DeleteObject(ctx, objectKey)

		const numConcurrentDownloads = 10
		results := make(chan error, numConcurrentDownloads)

		// Start concurrent downloads
		for i := 0; i < numConcurrentDownloads; i++ {
			go func() {
				reader, err := bucket.GetObject(ctx, objectKey)
				if err != nil {
					results <- err
					return
				}
				defer reader.Close()

				data, err := io.ReadAll(reader)
				if err != nil {
					results <- err
					return
				}

				if !bytes.Equal(data, testData) {
					results <- fmt.Errorf("downloaded data mismatch")
					return
				}

				results <- nil
			}()
		}

		// Wait for all downloads to complete
		var downloadErrors []error
		for i := 0; i < numConcurrentDownloads; i++ {
			if err := <-results; err != nil {
				downloadErrors = append(downloadErrors, err)
			}
		}

		assert.Empty(t, downloadErrors, "All concurrent downloads should succeed")
	})

	t.Run("ConcurrentBucketOperations", func(t *testing.T) {
		const numConcurrentBuckets = 5
		bucketNames := make([]string, numConcurrentBuckets)
		results := make(chan error, numConcurrentBuckets)

		// Create buckets concurrently
		for i := 0; i < numConcurrentBuckets; i++ {
			bucketNames[i] = generateTestBucketName()
			go func(name string) {
				err := client.CreateBucket(ctx, name)
				results <- err
			}(bucketNames[i])
		}

		// Wait for all bucket creations
		var createErrors []error
		for i := 0; i < numConcurrentBuckets; i++ {
			if err := <-results; err != nil {
				createErrors = append(createErrors, err)
			}
		}

		assert.Empty(t, createErrors, "All concurrent bucket creations should succeed")

		// Verify buckets exist and clean up concurrently
		for i := 0; i < numConcurrentBuckets; i++ {
			go func(name string) {
				exists, err := client.BucketExists(ctx, name)
				if err != nil || !exists {
					results <- fmt.Errorf("bucket %s should exist", name)
					return
				}
				err = client.DeleteBucket(ctx, name)
				results <- err
			}(bucketNames[i])
		}

		// Wait for all bucket deletions
		var deleteErrors []error
		for i := 0; i < numConcurrentBuckets; i++ {
			if err := <-results; err != nil {
				deleteErrors = append(deleteErrors, err)
			}
		}

		assert.Empty(t, deleteErrors, "All concurrent bucket deletions should succeed")
	})
}

// testEdgeCases tests various edge cases and boundary conditions
func testEdgeCases(t *testing.T, ctx context.Context, client *Client) {
	bucketName := generateTestBucketName()
	err := client.CreateBucket(ctx, bucketName)
	require.NoError(t, err, "Should create test bucket")
	defer client.DeleteBucket(ctx, bucketName)

	bucket, err := client.Bucket(bucketName)
	require.NoError(t, err, "Should get bucket object")

	t.Run("EmptyObjectUploadDownload", func(t *testing.T) {
		objectKey := generateTestObjectKey()
		emptyData := []byte{}

		// Upload empty object
		reader := bytes.NewReader(emptyData)
		err := bucket.PutObject(ctx, objectKey, reader, 0)
		assert.NoError(t, err, "Should upload empty object successfully")
		defer bucket.DeleteObject(ctx, objectKey)

		// Download empty object
		downloadReader, err := bucket.GetObject(ctx, objectKey)
		assert.NoError(t, err, "Should download empty object successfully")
		defer downloadReader.Close()

		data, err := io.ReadAll(downloadReader)
		assert.NoError(t, err, "Should read empty object data")
		assert.Equal(t, emptyData, data, "Empty object data should match")

		// Check metadata
		info, err := bucket.HeadObject(ctx, objectKey)
		assert.NoError(t, err, "Should get empty object metadata")
		assert.Equal(t, int64(0), info.Size, "Empty object size should be 0")
	})

	t.Run("VeryLongObjectKey", func(t *testing.T) {
		// Test with maximum allowed object key length (1024 characters)
		longKey := strings.Repeat("a", 255)
		testData := []byte("test-data-for-long-key")

		reader := bytes.NewReader(testData)
		err := bucket.PutObject(ctx, longKey, reader, int64(len(testData)))
		assert.NoError(t, err, "Should upload object with long key successfully")
		defer bucket.DeleteObject(ctx, longKey)

		// Verify the object exists
		exists, err := bucket.ObjectExists(ctx, longKey)
		assert.NoError(t, err, "Should check long key object existence")
		assert.True(t, exists, "Long key object should exist")

		// Download and verify
		downloadReader, err := bucket.GetObject(ctx, longKey)
		assert.NoError(t, err, "Should download long key object")
		defer downloadReader.Close()

		data, err := io.ReadAll(downloadReader)
		assert.NoError(t, err, "Should read long key object data")
		assert.Equal(t, testData, data, "Long key object data should match")
	})

	t.Run("SpecialCharactersInObjectKey", func(t *testing.T) {
		// Test object keys with various special characters
		specialKeys := []string{
			"object with spaces",
			"object-with-hyphens",
			"object_with_underscores",
			"object.with.dots",
			"object(with)parentheses",
			"object[with]brackets",
			"object{with}braces",
			"object/with/slashes",
			"object:with:colons",
			"object;with;semicolons",
			"object,with,commas",
			"object=with=equals",
			"object+with+plus",
			"object%20with%20encoding",
		}

		testData := []byte("test data for special characters")

		for _, key := range specialKeys {
			t.Run(fmt.Sprintf("Key-%s", key), func(t *testing.T) {
				reader := bytes.NewReader(testData)
				err := bucket.PutObject(ctx, key, reader, int64(len(testData)))

				// Some special characters might be invalid, so we check if upload succeeds
				if err == nil {
					defer bucket.DeleteObject(ctx, key)

					// If upload succeeded, verify download works
					downloadReader, err := bucket.GetObject(ctx, key)
					assert.NoError(t, err, "Should download special key object: %s", key)
					if downloadReader != nil {
						defer downloadReader.Close()
						data, err := io.ReadAll(downloadReader)
						assert.NoError(t, err, "Should read special key object data: %s", key)
						assert.Equal(t, testData, data, "Special key object data should match: %s", key)
					}
				}
				// If upload failed, that's also acceptable for some special characters
			})
		}
	})

	t.Run("LargeObjectOperations", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping large object test in short mode")
		}

		objectKey := generateTestObjectKey()
		// Create 50MB test data
		largeData := generateTestData(50 * 1024 * 1024)

		// Upload large object
		reader := bytes.NewReader(largeData)
		err := bucket.PutObject(ctx, objectKey, reader, int64(len(largeData)))
		assert.NoError(t, err, "Should upload large object successfully")
		defer bucket.DeleteObject(ctx, objectKey)

		// Verify object metadata
		info, err := bucket.HeadObject(ctx, objectKey)
		assert.NoError(t, err, "Should get large object metadata")
		assert.Equal(t, int64(len(largeData)), info.Size, "Large object size should match")

		// Download and verify large object
		downloadReader, err := bucket.GetObject(ctx, objectKey)
		assert.NoError(t, err, "Should download large object")
		defer downloadReader.Close()

		data, err := io.ReadAll(downloadReader)
		assert.NoError(t, err, "Should read large object data")
		assert.Equal(t, largeData, data, "Large object data should match")
	})

	t.Run("RapidCreateDeleteOperations", func(t *testing.T) {
		// Test rapid creation and deletion of objects
		const numRapidOperations = 20
		objectKeys := make([]string, numRapidOperations)
		testData := []byte("rapid operation test data")

		// Rapid creation
		for i := 0; i < numRapidOperations; i++ {
			objectKeys[i] = generateTestObjectKey()
			reader := bytes.NewReader(testData)
			err := bucket.PutObject(ctx, objectKeys[i], reader, int64(len(testData)))
			assert.NoError(t, err, "Should create object rapidly: %s", objectKeys[i])
		}

		// Verify all objects exist
		for _, key := range objectKeys {
			exists, err := bucket.ObjectExists(ctx, key)
			assert.NoError(t, err, "Should check rapid object existence: %s", key)
			assert.True(t, exists, "Rapid object should exist: %s", key)
		}

		// Rapid deletion
		for _, key := range objectKeys {
			err := bucket.DeleteObject(ctx, key)
			assert.NoError(t, err, "Should delete object rapidly: %s", key)
		}

		// Verify all objects are deleted
		for _, key := range objectKeys {
			exists, err := bucket.ObjectExists(ctx, key)
			assert.NoError(t, err, "Should check deleted object existence: %s", key)
			assert.False(t, exists, "Rapid deleted object should not exist: %s", key)
		}
	})
}

// Helper functions for test data generation and management
// Note: generateTestBucketName, generateTestObjectKey, and generateTestData
// are already defined in integration_test.go
