package s3

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClientBasics(t *testing.T) {
	t.Run("Client creation and connection state", func(t *testing.T) {
		// Test with nil Config
		client, err := NewClient(nil, nil)
		assert.NoError(t, err)
		assert.NotNil(t, client)
		assert.False(t, client.IsConnected())

		// Test with valid Config
		config := NewConfig()
		client, err = NewClient(config, nil)
		assert.NoError(t, err)
		assert.NotNil(t, client)
		assert.False(t, client.IsConnected())

		// Test with invalid Config
		config.TimeoutSeconds = -1
		client, err = NewClient(config, nil)
		assert.Error(t, err)
		assert.Nil(t, client)
	})

	t.Run("Client close", func(t *testing.T) {
		config := NewConfig()
		client, err := NewClient(config, nil)
		require.NoError(t, err)

		err = client.Close()
		assert.NoError(t, err)
		assert.False(t, client.IsConnected())
	})
}

func TestConstants(t *testing.T) {
	// Test that all constants are properly defined
	assert.Equal(t, "eu-west-1", DefaultRegion)
	assert.Equal(t, int64(100*1024*1024), DefaultMultipartThreshold)
	assert.Equal(t, int64(10*1024*1024), DefaultPartSize)
	assert.Equal(t, int64(5*1024*1024), MinPartSize)
	assert.Equal(t, int64(5*1024*1024*1024), MaxPartSize)
	assert.Equal(t, 10000, DefaultMaxUploadParts)
	assert.Equal(t, 3, DefaultMaxRetries)

	// Encryption constants
	assert.Equal(t, "AES256", SSEAlgorithmAES256)
	assert.Equal(t, "aws:kms", SSEAlgorithmKMS)
	assert.Equal(t, "aws:kms:dsse", SSEAlgorithmKMSDSSE)
	assert.Equal(t, "AES256", SSECAlgorithmAES256)
}

func TestOperationsWithoutConnection(t *testing.T) {
	// Create a client that's not connected
	config := NewConfig()
	client, err := NewClient(config, nil)
	require.NoError(t, err)

	assert.False(t, client.connected) // Should not be connected initially

	ctx := context.Background()

	t.Run("Bucket operations fail when not connected", func(t *testing.T) {
		bucket, err := client.Bucket("test-bucket")
		err = bucket.Create(ctx)
		assert.Error(t, err)
		assert.Equal(t, ErrClientNotConnected, err)

		bucket, err = client.Bucket("test-bucket")
		err = bucket.Delete(ctx)
		assert.Error(t, err)
		assert.Equal(t, ErrClientNotConnected, err)

		_, err = client.ListBuckets(ctx)
		assert.Error(t, err)
		assert.Equal(t, ErrClientNotConnected, err)

		bucket, err = client.Bucket("test-bucket")
		_, err = bucket.Exists(ctx)
		assert.Error(t, err)
		assert.Equal(t, ErrClientNotConnected, err)
	})

	t.Run("Object operations fail when not connected", func(t *testing.T) {
		reader := strings.NewReader("test")

		bucket, err := client.Bucket("bucket")
		assert.NoError(t, err)
		err = bucket.PutObject(ctx, "key", reader, 4)
		assert.Error(t, err)
		assert.Equal(t, ErrClientNotConnected, err)

		_, err = bucket.GetObject(ctx, "key")
		assert.Error(t, err)
		assert.Equal(t, ErrClientNotConnected, err)

		err = bucket.DeleteObject(ctx, "key")
		assert.Error(t, err)
		assert.Equal(t, ErrClientNotConnected, err)

		_, err = bucket.ListObjects(ctx)
		assert.Error(t, err)
		assert.Equal(t, ErrClientNotConnected, err)

		_, err = bucket.ObjectExists(ctx, "key")
		assert.Error(t, err)
		assert.Equal(t, ErrClientNotConnected, err)

		_, err = bucket.HeadObject(ctx, "key")
		assert.Error(t, err)
		assert.Equal(t, ErrClientNotConnected, err)
	})
}
