package s3

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const tlsKMSKeyName = "minio-test-key"

// 32-byte key reused as the KMS master key and as an SSE-C customer key.
var sseTestKey = []byte("0123456789abcdef0123456789abcdef")

// generateSelfSignedCert returns a self-signed cert/key valid for localhost.
func generateSelfSignedCert(t *testing.T) (certPEM, keyPEM []byte) {
	t.Helper()

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	tmpl := x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "minio-test"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
		DNSNames:              []string{"localhost"},
		IPAddresses:           []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
	}

	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &priv.PublicKey, priv)
	require.NoError(t, err)
	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})

	keyDER, err := x509.MarshalPKCS8PrivateKey(priv)
	require.NoError(t, err)
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})

	return certPEM, keyPEM
}

// setupTLSMinIOContainer starts MinIO with TLS and a single-key KMS, returning
// the container, the path to the CA cert on disk, and a cleanup function.
func setupTLSMinIOContainer(ctx context.Context, t *testing.T) (*MinIOContainer, string, func()) {
	t.Helper()

	certPEM, keyPEM := generateSelfSignedCert(t)
	caPath := filepath.Join(t.TempDir(), "ca.crt")
	require.NoError(t, os.WriteFile(caPath, certPEM, 0o600))

	kmsKey := tlsKMSKeyName + ":" + base64.StdEncoding.EncodeToString(sseTestKey)

	req := testcontainers.ContainerRequest{
		Image:        testMinIOImage,
		ExposedPorts: []string{"9000/tcp", "9001/tcp"},
		Env: map[string]string{
			"MINIO_ROOT_USER":      testMinIOAccessKey,
			"MINIO_ROOT_PASSWORD":  testMinIOSecretKey,
			"MINIO_KMS_SECRET_KEY": kmsKey,
		},
		Files: []testcontainers.ContainerFile{
			{Reader: bytes.NewReader(certPEM), ContainerFilePath: "/root/.minio/certs/public.crt", FileMode: 0o644},
			{Reader: bytes.NewReader(keyPEM), ContainerFilePath: "/root/.minio/certs/private.key", FileMode: 0o600},
		},
		Cmd: []string{
			"server", "/data",
			"--console-address", ":9001",
			"--address", ":9000",
		},
		WaitingFor: wait.ForLog("MinIO Object Storage Server").WithOccurrence(1).WithStartupTimeout(60 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err, "Failed to start TLS MinIO container")

	host, err := container.Host(ctx)
	require.NoError(t, err)
	port, err := container.MappedPort(ctx, "9000")
	require.NoError(t, err)

	minioContainer := &MinIOContainer{
		Container: container,
		endpoint:  fmt.Sprintf("%s:%s", host, port.Port()),
	}

	cleanup := func() {
		if err := container.Terminate(ctx); err != nil {
			t.Logf("Warning: failed to terminate container: %v", err)
		}
	}

	return minioContainer, caPath, cleanup
}

// createTLSTestClient builds a connected client trusting the container's CA over TLS.
func createTLSTestClient(t *testing.T, container *MinIOContainer, caPath string) *Client {
	t.Helper()

	config := NewConfig()
	config.Endpoint = container.GetEndpoint()
	config.Region = testMinIORegion
	config.AccessKeyID = testMinIOAccessKey
	config.UseSSL = true
	config.ForcePathStyle = true
	config.TLSEnable = true
	config.TLSCA = caPath
	config.TLSInsecureSkipVerify = true

	envVar := fmt.Sprintf("MINIO_TLS_SECRET_%s", strings.ReplaceAll(t.Name(), "/", "_"))
	os.Setenv(envVar, testMinIOSecretKey)
	config.DefaultCredentialConfig.PasswordEnvVar = envVar
	config.DefaultCredentialConfig.Password = testMinIOSecretKey

	client, err := NewClient(config, nil)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), integrationTimeout)
	defer cancel()
	require.NoError(t, client.Connect(ctx))

	return client
}

// TestIntegrationSSE exercises SSE-S3, SSE-KMS, and SSE-C against a real
// TLS+KMS-enabled MinIO server (SSE requires a secure connection).
func TestIntegrationSSE(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	container, caPath, cleanup := setupTLSMinIOContainer(ctx, t)
	defer cleanup()

	client := createTLSTestClient(t, container, caPath)
	defer client.Close()

	bucketName := generateTestBucketName()
	bucket, err := client.Bucket(bucketName)
	require.NoError(t, err)
	require.NoError(t, bucket.Create(ctx))

	data := generateTestData(4096)

	t.Run("SSE-S3", func(t *testing.T) {
		key := generateTestObjectKey()
		require.NoError(t, bucket.PutObject(ctx, key, bytes.NewReader(data), int64(len(data)), ObjectOptions{
			ServerSideEncryption: SSEAlgorithmAES256,
		}))

		// Content round-trips (server-side decryption is transparent).
		got := mustGet(t, ctx, bucket, key)
		assert.Equal(t, data, got)

		// The object is actually encrypted server-side.
		info, err := bucket.MinioClient().StatObject(ctx, bucketName, key, minio.StatObjectOptions{})
		require.NoError(t, err)
		assert.Equal(t, "AES256", info.Metadata.Get("X-Amz-Server-Side-Encryption"))
	})

	t.Run("SSE-KMS", func(t *testing.T) {
		key := generateTestObjectKey()
		require.NoError(t, bucket.PutObject(ctx, key, bytes.NewReader(data), int64(len(data)), ObjectOptions{
			ServerSideEncryption: SSEAlgorithmKMS,
			SSEKMSKeyId:          tlsKMSKeyName,
		}))

		got := mustGet(t, ctx, bucket, key)
		assert.Equal(t, data, got)

		info, err := bucket.MinioClient().StatObject(ctx, bucketName, key, minio.StatObjectOptions{})
		require.NoError(t, err)
		assert.Equal(t, "aws:kms", info.Metadata.Get("X-Amz-Server-Side-Encryption"))
	})

	t.Run("SSE-C with copy-source decryption", func(t *testing.T) {
		keyB64 := base64.StdEncoding.EncodeToString(sseTestKey)
		srcKey := generateTestObjectKey()
		require.NoError(t, bucket.PutObject(ctx, srcKey, bytes.NewReader(data), int64(len(data)), ObjectOptions{
			SSECustomerKey: keyB64,
		}))

		// Copy the SSE-C source to a plaintext destination using the source key,
		// then read the (unencrypted) destination back to verify the content.
		dstKey := generateTestObjectKey()
		require.NoError(t, bucket.CopyObject(ctx, srcKey, bucketName, dstKey, ObjectOptions{
			SourceSSECustomerKey: keyB64,
		}))
		got := mustGet(t, ctx, bucket, dstKey)
		assert.Equal(t, data, got)

		// Copying without the source key must fail.
		err := bucket.CopyObject(ctx, srcKey, bucketName, generateTestObjectKey())
		assert.Error(t, err, "copying an SSE-C source without the key should fail")
	})
}

func mustGet(t *testing.T, ctx context.Context, bucket *Bucket, key string) []byte {
	t.Helper()
	rc, err := bucket.GetObject(ctx, key)
	require.NoError(t, err)
	defer rc.Close()
	data, err := io.ReadAll(rc)
	require.NoError(t, err)
	return data
}
