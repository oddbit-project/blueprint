package s3

import (
	"encoding/base64"
	"testing"

	"github.com/minio/minio-go/v7/pkg/encrypt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServerSideEncryption(t *testing.T) {
	validKey := base64.StdEncoding.EncodeToString(make([]byte, 32)) // 32-byte SSE-C key

	t.Run("none", func(t *testing.T) {
		sse, err := serverSideEncryption(ObjectOptions{})
		require.NoError(t, err)
		assert.Nil(t, sse)
	})

	t.Run("SSE-S3 (AES256)", func(t *testing.T) {
		sse, err := serverSideEncryption(ObjectOptions{ServerSideEncryption: SSEAlgorithmAES256})
		require.NoError(t, err)
		require.NotNil(t, sse)
		assert.Equal(t, encrypt.S3, sse.Type())
	})

	t.Run("SSE-KMS", func(t *testing.T) {
		sse, err := serverSideEncryption(ObjectOptions{
			ServerSideEncryption:    SSEAlgorithmKMS,
			SSEKMSKeyId:             "key-id",
			SSEKMSEncryptionContext: map[string]string{"project": "demo"},
		})
		require.NoError(t, err)
		require.NotNil(t, sse)
		assert.Equal(t, encrypt.KMS, sse.Type())
	})

	t.Run("SSE-C", func(t *testing.T) {
		sse, err := serverSideEncryption(ObjectOptions{SSECustomerKey: validKey})
		require.NoError(t, err)
		require.NotNil(t, sse)
		assert.Equal(t, encrypt.SSEC, sse.Type())
	})

	t.Run("SSE-C takes precedence over ServerSideEncryption", func(t *testing.T) {
		sse, err := serverSideEncryption(ObjectOptions{
			ServerSideEncryption: SSEAlgorithmAES256,
			SSECustomerKey:       validKey,
		})
		require.NoError(t, err)
		assert.Equal(t, encrypt.SSEC, sse.Type())
	})

	t.Run("SSE-C invalid base64", func(t *testing.T) {
		_, err := serverSideEncryption(ObjectOptions{SSECustomerKey: "!!!not-base64!!!"})
		assert.Error(t, err)
	})

	t.Run("SSE-C wrong key length", func(t *testing.T) {
		short := base64.StdEncoding.EncodeToString(make([]byte, 16))
		_, err := serverSideEncryption(ObjectOptions{SSECustomerKey: short})
		assert.Error(t, err)
	})

	t.Run("KMS DSSE is rejected", func(t *testing.T) {
		_, err := serverSideEncryption(ObjectOptions{ServerSideEncryption: SSEAlgorithmKMSDSSE})
		assert.Error(t, err)
	})

	t.Run("unsupported algorithm", func(t *testing.T) {
		_, err := serverSideEncryption(ObjectOptions{ServerSideEncryption: "bogus"})
		assert.Error(t, err)
	})
}

func TestSSECustomerKey(t *testing.T) {
	t.Run("valid 32-byte key", func(t *testing.T) {
		sse, err := sseCustomerKey(base64.StdEncoding.EncodeToString(make([]byte, 32)))
		require.NoError(t, err)
		require.NotNil(t, sse)
		assert.Equal(t, encrypt.SSEC, sse.Type())
	})

	t.Run("invalid base64", func(t *testing.T) {
		_, err := sseCustomerKey("!!!")
		assert.Error(t, err)
	})

	t.Run("wrong length", func(t *testing.T) {
		_, err := sseCustomerKey(base64.StdEncoding.EncodeToString(make([]byte, 10)))
		assert.Error(t, err)
	})
}
