package s3

import (
	"context"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSimpleIntegration tests basic connection to MinIO
func TestSimpleIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Check if Docker is available
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("Docker not available, skipping integration tests")
	}

	// Start MinIO container
	t.Log("Starting MinIO container...")

	// Stop and remove any existing container
	exec.Command("docker", "stop", "simple-minio-test").Run()
	exec.Command("docker", "rm", "simple-minio-test").Run()

	// Start fresh container
	cmd := exec.Command("docker", "run", "-d",
		"--name", "simple-minio-test",
		"-p", "9020:9000",
		"-p", "9021:9001",
		"-e", "MINIO_ROOT_USER=minioadmin",
		"-e", "MINIO_ROOT_PASSWORD=minioadmin",
		"quay.io/minio/minio", "server", "/data", "--console-address", ":9001")

	err := cmd.Run()
	require.NoError(t, err, "Failed to start MinIO container")

	// Cleanup function
	defer func() {
		t.Log("Cleaning up MinIO container...")
		exec.Command("docker", "stop", "simple-minio-test").Run()
		exec.Command("docker", "rm", "simple-minio-test").Run()
	}()

	// Wait for MinIO to be ready
	t.Log("Waiting for MinIO to be ready...")
	for i := 0; i < 30; i++ {
		if testConnectionToMinIO() {
			t.Log("MinIO is ready!")
			break
		}
		if i == 29 {
			// Show container logs for debugging
			logCmd := exec.Command("docker", "logs", "simple-minio-test")
			logs, _ := logCmd.Output()
			t.Logf("MinIO logs:\n%s", string(logs))
			t.Fatal("MinIO did not become ready in time")
		}
		time.Sleep(2 * time.Second)
	}

	// Now test our S3 client
	t.Log("Testing S3 client connection...")

	config := NewConfig()
	config.Endpoint = "localhost:9020"
	config.Region = "us-east-1"
	config.AccessKeyID = "minioadmin"
	config.UseSSL = false
	config.ForcePathStyle = true

	// Set secret key via environment
	os.Setenv("SIMPLE_TEST_SECRET", "minioadmin")
	config.DefaultCredentialConfig.PasswordEnvVar = "SIMPLE_TEST_SECRET"

	client, err := NewClient(config, nil)
	require.NoError(t, err, "Should create client successfully")
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = client.Connect(ctx)
	require.NoError(t, err, "Should connect to MinIO successfully")

	// Test basic operations
	t.Run("ListBuckets", func(t *testing.T) {
		buckets, err := client.ListBuckets(ctx)
		assert.NoError(t, err, "Should list buckets successfully")
		t.Logf("Found %d buckets", len(buckets))
		for _, bucket := range buckets {
			t.Logf("  Bucket: %s, Created: %v", bucket.Name, bucket.CreationDate)
		}
	})

	t.Run("CreateAndListBucket", func(t *testing.T) {
		bucketName := "simple-test-bucket"
		bucket, err := client.Bucket(bucketName)
		assert.NoError(t, err)

		// Create bucket
		err = bucket.Create(ctx)
		assert.NoError(t, err, "Should create bucket successfully")

		// List buckets to verify
		buckets, err := client.ListBuckets(ctx)
		assert.NoError(t, err, "Should list buckets after creation")

		found := false
		for _, bucket := range buckets {
			if bucket.Name == bucketName {
				found = true
				break
			}
		}
		assert.True(t, found, "Should find created bucket in list")

		// Clean up
		bucket.Delete(ctx)
	})
}

// testConnectionToMinIO attempts a simple connection test to MinIO
func testConnectionToMinIO() bool {
	// Try to connect with curl to health endpoint
	cmd := exec.Command("curl", "-sf", "http://localhost:9020/minio/health/live")
	return cmd.Run() == nil
}
