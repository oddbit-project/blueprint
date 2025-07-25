package s3

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// Test configuration for testcontainers
const (
	// MinIO test configuration
	testMinIOImage     = "quay.io/minio/minio:latest"
	testMinIOAccessKey = "minioadmin"
	testMinIOSecretKey = "minioadmin"
	testMinIORegion    = "us-east-1"

	// Test constants
	testBucketPrefix   = "test-bucket-"
	testObjectPrefix   = "test-object-"
	integrationTimeout = 300 * time.Second
)

// MinIOContainer wraps the testcontainers container with MinIO-specific functionality
type MinIOContainer struct {
	testcontainers.Container
	endpoint string
}

// GetEndpoint returns the MinIO endpoint URL
func (c *MinIOContainer) GetEndpoint() string {
	return c.endpoint
}

// setupMinIOContainer starts a MinIO container using testcontainers
func setupMinIOContainer(ctx context.Context, t *testing.T) (*MinIOContainer, func()) {
	t.Helper()

	// Create container request
	req := testcontainers.ContainerRequest{
		Image:        testMinIOImage,
		ExposedPorts: []string{"9000/tcp", "9001/tcp"},
		Env: map[string]string{
			"MINIO_ROOT_USER":              testMinIOAccessKey,
			"MINIO_ROOT_PASSWORD":          testMinIOSecretKey,
			"MINIO_PROMETHEUS_AUTH_TYPE":   "public",
		},
		Cmd: []string{
			"server", "/data",
			"--console-address", ":9001",
			"--address", ":9000",
		},
		WaitingFor: wait.ForAll(
			wait.ForHTTP("/minio/health/live").WithPort("9000/tcp"),
			wait.ForLog("MinIO Object Storage Server").WithOccurrence(1),
		).WithDeadline(60 * time.Second),
	}

	// Start container
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err, "Failed to start MinIO container")

	// Get the endpoint
	host, err := container.Host(ctx)
	require.NoError(t, err, "Failed to get container host")

	port, err := container.MappedPort(ctx, "9000")
	require.NoError(t, err, "Failed to get container port")

	endpoint := fmt.Sprintf("%s:%s", host, port.Port())

	// Verify MinIO is ready with a basic connectivity test
	require.Eventually(t, func() bool {
		return testMinIOConnectivity(t, endpoint)
	}, 30*time.Second, 1*time.Second, "MinIO container should be ready")

	t.Logf("MinIO container started successfully on %s", endpoint)

	minioContainer := &MinIOContainer{
		Container: container,
		endpoint:  endpoint,
	}

	// Return container and cleanup function
	cleanup := func() {
		t.Log("Cleaning up MinIO container")
		if err := container.Terminate(ctx); err != nil {
			t.Logf("Warning: Failed to terminate container: %v", err)
		}
	}

	return minioContainer, cleanup
}

// testMinIOConnectivity tests if MinIO is ready to accept S3 operations
func testMinIOConnectivity(t *testing.T, endpoint string) bool {
	// Create a test client
	config := NewConfig()
	config.Endpoint = endpoint
	config.Region = testMinIORegion
	config.AccessKeyID = testMinIOAccessKey
	config.DefaultCredentialConfig.Password = testMinIOSecretKey
	config.UseSSL = false
	config.ForcePathStyle = true

	client, err := NewClient(config, nil)
	if err != nil {
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = client.Connect(ctx)
	if err != nil {
		return false
	}
	defer client.Close()

	// Test basic operation
	_, err = client.ListBuckets(ctx)
	return err == nil
}

// createTestClientWithContainer creates a configured S3 client for the container
func createTestClientWithContainer(t *testing.T, container *MinIOContainer) *Client {
	config := NewConfig()
	config.Endpoint = container.GetEndpoint()
	config.Region = testMinIORegion
	config.AccessKeyID = testMinIOAccessKey
	config.UseSSL = false
	config.ForcePathStyle = true

	// Set secret key using test-specific env var
	envVarName := fmt.Sprintf("MINIO_TEST_SECRET_%s", strings.ReplaceAll(t.Name(), "/", "_"))
	os.Setenv(envVarName, testMinIOSecretKey)
	config.DefaultCredentialConfig.PasswordEnvVar = envVarName
	
	// Also set the password directly as a fallback
	config.DefaultCredentialConfig.Password = testMinIOSecretKey

	client, err := NewClient(config, nil)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), integrationTimeout)
	defer cancel()

	err = client.Connect(ctx)
	require.NoError(t, err)

	return client
}

// Helper functions (same as before)
func generateTestBucketName() string {
	return fmt.Sprintf("%s%d", testBucketPrefix, time.Now().UnixNano())
}

func generateTestObjectKey() string {
	return fmt.Sprintf("%s%d.txt", testObjectPrefix, time.Now().UnixNano())
}

func generateTestData(size int) []byte {
	data := make([]byte, size)
	rand.Read(data)
	return data
}

// Integration test for bucket operations using testcontainers
func TestIntegrationBucketOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	container, cleanup := setupMinIOContainer(ctx, t)
	defer cleanup()

	client := createTestClientWithContainer(t, container)
	defer client.Close()

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
		nonExistentBucketObj, err := client.Bucket(nonExistentBucket)
		assert.NoError(t, err)
		exists, err = nonExistentBucketObj.Exists(ctx)
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

	// Clean up
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

// Integration test for object operations using testcontainers
func TestIntegrationObjectOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	container, cleanup := setupMinIOContainer(ctx, t)
	defer cleanup()

	client := createTestClientWithContainer(t, container)
	defer client.Close()

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

	testData := []byte("Hello, MinIO testcontainers integration test!")

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

// Integration test for multipart upload using testcontainers
func TestIntegrationMultipartUpload(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	container, cleanup := setupMinIOContainer(ctx, t)
	defer cleanup()

	client := createTestClientWithContainer(t, container)
	defer client.Close()

	bucketName := generateTestBucketName()
	bucket, err := client.Bucket(bucketName)
	assert.NoError(t, err)

	// Setup bucket
	err = bucket.Create(ctx)
	require.NoError(t, err)

	defer bucket.Delete(ctx)

	t.Run("MultipartUploadSmallFile", func(t *testing.T) {
		objectKey := generateTestObjectKey()
		testData := generateTestData(15 * 1024 * 1024) // 15MB

		reader := bytes.NewReader(testData)

		err := bucket.PutObjectMultipart(ctx, objectKey, reader, int64(len(testData)))
		assert.NoError(t, err, "Should upload large object via multipart successfully")

		defer bucket.DeleteObject(ctx, objectKey)

		// Verify uploaded data
		downloadReader, err := bucket.GetObject(ctx, objectKey)
		require.NoError(t, err)
		defer downloadReader.Close()

		downloadedData, err := io.ReadAll(downloadReader)
		assert.NoError(t, err, "Should read multipart uploaded object successfully")
		assert.Equal(t, testData, downloadedData, "Multipart uploaded data should match original data")
	})
}

// Integration test for range downloads using testcontainers
func TestIntegrationRangeDownloads(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	container, cleanup := setupMinIOContainer(ctx, t)
	defer cleanup()

	client := createTestClientWithContainer(t, container)
	defer client.Close()

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

		t.Run("RangeMiddleBytes", func(t *testing.T) {
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
	})

	t.Run("GetObjectAdvanced", func(t *testing.T) {
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
	})
}

// Comprehensive test suite using testcontainers
func TestIntegrationComprehensiveWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping comprehensive integration test in short mode")
	}

	ctx := context.Background()
	container, cleanup := setupMinIOContainer(ctx, t)
	defer cleanup()

	client := createTestClientWithContainer(t, container)
	defer client.Close()

	bucketName := generateTestBucketName()
	bucket, err := client.Bucket(bucketName)
	require.NoError(t, err)

	// Setup bucket
	err = bucket.Create(ctx)
	require.NoError(t, err)
	defer bucket.Delete(ctx)

	t.Run("ComprehensiveWorkflow", func(t *testing.T) {
		// Test a complete workflow: upload, copy, download, range download, delete
		objectKey := generateTestObjectKey()
		copyKey := fmt.Sprintf("copy-%s", objectKey)
		testData := []byte("Comprehensive workflow test data for testcontainers")

		// 1. Upload
		reader := bytes.NewReader(testData)
		err := bucket.PutObject(ctx, objectKey, reader, int64(len(testData)))
		assert.NoError(t, err, "Should upload object successfully")

		// 2. Verify upload
		exists, err := bucket.ObjectExists(ctx, objectKey)
		assert.NoError(t, err)
		assert.True(t, exists, "Object should exist after upload")

		// 3. Copy object
		err = bucket.CopyObject(ctx, objectKey, bucketName, copyKey)
		assert.NoError(t, err, "Should copy object successfully")

		// 4. Download original
		downloadReader, err := bucket.GetObject(ctx, objectKey)
		assert.NoError(t, err, "Should download original object successfully")
		originalData, err := io.ReadAll(downloadReader)
		downloadReader.Close()
		assert.NoError(t, err)
		assert.Equal(t, testData, originalData, "Original downloaded data should match")

		// 5. Download copy
		copyReader, err := bucket.GetObject(ctx, copyKey)
		assert.NoError(t, err, "Should download copied object successfully")
		copyData, err := io.ReadAll(copyReader)
		copyReader.Close()
		assert.NoError(t, err)
		assert.Equal(t, testData, copyData, "Copied data should match original")

		// 6. Range download
		rangeReader, err := bucket.GetObjectRange(ctx, objectKey, 0, 4)
		assert.NoError(t, err, "Should get range successfully")
		rangeData, err := io.ReadAll(rangeReader)
		rangeReader.Close()
		assert.NoError(t, err)
		assert.Equal(t, testData[0:5], rangeData, "Range data should match")

		// 7. Advanced download with options
		var buf bytes.Buffer
		startByte := int64(10)
		endByte := int64(20)
		opts := DownloadOptions{
			StartByte: &startByte,
			EndByte:   &endByte,
		}
		err = bucket.GetObjectAdvanced(ctx, objectKey, &buf, opts)
		assert.NoError(t, err, "Should download with advanced options successfully")
		assert.Equal(t, testData[10:21], buf.Bytes(), "Advanced download should match expected range")

		// 8. List objects
		objects, err := bucket.ListObjects(ctx)
		assert.NoError(t, err, "Should list objects successfully")
		assert.Len(t, objects, 2, "Should have both original and copied objects")

		// 9. Delete objects
		err = bucket.DeleteObject(ctx, objectKey)
		assert.NoError(t, err, "Should delete original object successfully")

		err = bucket.DeleteObject(ctx, copyKey)
		assert.NoError(t, err, "Should delete copied object successfully")

		// 10. Verify deletion
		exists, err = bucket.ObjectExists(ctx, objectKey)
		assert.NoError(t, err)
		assert.False(t, exists, "Original object should not exist after deletion")

		exists, err = bucket.ObjectExists(ctx, copyKey)
		assert.NoError(t, err)
		assert.False(t, exists, "Copied object should not exist after deletion")
	})
}
