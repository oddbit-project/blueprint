package s3

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func readObject(t *testing.T, b *Bucket, key string, versionID ...string) string {
	t.Helper()
	r, err := b.GetObject(context.Background(), key, versionID...)
	require.NoError(t, err)
	defer func() { _ = r.Close() }()
	data, err := io.ReadAll(r)
	require.NoError(t, err)
	return string(data)
}

func TestIntegrationObjectVersions(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	container, cleanup := setupMinIOContainer(ctx, t)
	defer cleanup()

	client := createTestClientWithContainer(t, container)
	defer func() { _ = client.Close() }()

	bucketName := generateTestBucketName()
	bucket, err := client.Bucket(bucketName)
	require.NoError(t, err)
	require.NoError(t, bucket.Create(ctx, BucketOptions{ObjectLocking: true}))

	key := generateTestObjectKey()
	v1, err := bucket.PutObjectInfo(ctx, key, bytes.NewReader([]byte("one")), 3)
	require.NoError(t, err)
	v2, err := bucket.PutObjectInfo(ctx, key, bytes.NewReader([]byte("two")), 3)
	require.NoError(t, err)
	require.NotEmpty(t, v1.VersionID)
	require.NotEmpty(t, v2.VersionID)
	require.NotEqual(t, v1.VersionID, v2.VersionID)

	t.Run("put returns the written version", func(t *testing.T) {
		for _, v := range []ObjectVersion{v1, v2} {
			assert.Equal(t, bucketName, v.Bucket)
			assert.Equal(t, key, v.Key)
			assert.Equal(t, int64(3), v.Size)
			head, err := bucket.HeadObject(ctx, key, v.VersionID)
			require.NoError(t, err)
			assert.NotEmpty(t, v.ETag)
			assert.Equal(t, head.ETag, v.ETag)
		}
		assert.Equal(t, "one", readObject(t, bucket, key, v1.VersionID))
		assert.Equal(t, "two", readObject(t, bucket, key, v2.VersionID))
	})

	t.Run("stream put returns the written version", func(t *testing.T) {
		key3 := generateTestObjectKey() + "-stream"
		v, err := bucket.PutObjectInfo(ctx, key3, strings.NewReader("abc"), -1)
		require.NoError(t, err)
		head, err := bucket.HeadObject(ctx, key3)
		require.NoError(t, err)
		assert.Equal(t, int64(3), v.Size)
		assert.NotEmpty(t, v.VersionID)
		assert.Equal(t, head.VersionID, v.VersionID)
		assert.NotEmpty(t, v.ETag)
	})

	t.Run("multipart put returns the written version", func(t *testing.T) {
		key4 := generateTestObjectKey() + "-multipart"
		size := 11 << 20
		v, err := bucket.PutObjectInfo(ctx, key4, bytes.NewReader(generateTestData(size)), int64(size))
		require.NoError(t, err)
		head, err := bucket.HeadObject(ctx, key4)
		require.NoError(t, err)
		assert.Equal(t, int64(size), v.Size)
		assert.Equal(t, key4, v.Key)
		assert.NotEmpty(t, v.VersionID)
		assert.Equal(t, head.VersionID, v.VersionID)
		assert.True(t, strings.Contains(v.ETag, "-"), "multipart ETag expected, got %q", v.ETag)
	})

	t.Run("copy an older version", func(t *testing.T) {
		key2 := generateTestObjectKey() + "-copy"
		c, err := bucket.CopyObjectVersion(ctx, CopySource{Name: key, VersionID: v1.VersionID}, bucketName, key2)
		require.NoError(t, err)
		require.NotEmpty(t, c.VersionID)
		head, err := bucket.HeadObject(ctx, key2)
		require.NoError(t, err)
		assert.Equal(t, bucketName, c.Bucket)
		assert.Equal(t, key2, c.Key)
		assert.Equal(t, int64(-1), c.Size)
		assert.Equal(t, head.VersionID, c.VersionID)
		assert.Equal(t, "one", readObject(t, bucket, key2, c.VersionID))
	})

	t.Run("copy without a version copies latest", func(t *testing.T) {
		key5 := generateTestObjectKey() + "-latest"
		require.NoError(t, bucket.CopyObject(ctx, key, bucketName, key5))
		assert.Equal(t, "two", readObject(t, bucket, key5))
	})

	t.Run("copy applies lock options", func(t *testing.T) {
		key6 := generateTestObjectKey() + "-locked"
		c, err := bucket.CopyObjectVersion(ctx, CopySource{Name: key, VersionID: v1.VersionID}, bucketName, key6, ObjectOptions{
			LegalHold:       LegalHoldEnabled,
			LockMode:        RetentionGovernance,
			RetainUntilDate: time.Now().Add(time.Hour).UTC().Truncate(time.Second),
		})
		require.NoError(t, err)

		held, err := bucket.GetObjectLegalHold(ctx, key6, c.VersionID)
		require.NoError(t, err)
		assert.True(t, held)
		ret, err := bucket.GetObjectRetention(ctx, key6, c.VersionID)
		require.NoError(t, err)
		assert.Equal(t, RetentionGovernance, ret.Mode)

		require.NoError(t, bucket.SetObjectLegalHoldVersion(ctx, key6, c.VersionID, false))
	})

	t.Run("copy to another bucket", func(t *testing.T) {
		otherName := generateTestBucketName() + "-other"
		other, err := client.Bucket(otherName)
		require.NoError(t, err)
		require.NoError(t, other.Create(ctx, BucketOptions{ObjectLocking: true}))

		key7 := generateTestObjectKey() + "-other"
		c, err := bucket.CopyObjectVersion(ctx, CopySource{Name: key, VersionID: v1.VersionID}, otherName, key7)
		require.NoError(t, err)
		assert.Equal(t, otherName, c.Bucket)
		_, err = other.HeadObject(ctx, key7, c.VersionID)
		require.NoError(t, err)
		assert.Equal(t, "one", readObject(t, other, key7, c.VersionID))
	})

	t.Run("legal hold on an older version only", func(t *testing.T) {
		require.NoError(t, bucket.SetObjectLegalHoldVersion(ctx, key, v1.VersionID, true))

		held, err := bucket.GetObjectLegalHold(ctx, key, v1.VersionID)
		require.NoError(t, err)
		assert.True(t, held)
		held, err = bucket.GetObjectLegalHold(ctx, key, v2.VersionID)
		require.NoError(t, err)
		assert.False(t, held)
		held, err = bucket.GetObjectLegalHold(ctx, key)
		require.NoError(t, err)
		assert.False(t, held)

		require.NoError(t, bucket.SetObjectLegalHoldVersion(ctx, key, v1.VersionID, false))
		held, err = bucket.GetObjectLegalHold(ctx, key, v1.VersionID)
		require.NoError(t, err)
		assert.False(t, held)
	})

	t.Run("legal hold without a version targets latest", func(t *testing.T) {
		require.NoError(t, bucket.SetObjectLegalHold(ctx, key, true))

		held, err := bucket.GetObjectLegalHold(ctx, key, v2.VersionID)
		require.NoError(t, err)
		assert.True(t, held)
		held, err = bucket.GetObjectLegalHold(ctx, key, v1.VersionID)
		require.NoError(t, err)
		assert.False(t, held)

		require.NoError(t, bucket.SetObjectLegalHold(ctx, key, false))
	})
}

func TestIntegrationObjectVersionsUnversionedBucket(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	container, cleanup := setupMinIOContainer(ctx, t)
	defer cleanup()

	client := createTestClientWithContainer(t, container)
	defer func() { _ = client.Close() }()

	bucket, err := client.Bucket(generateTestBucketName())
	require.NoError(t, err)
	require.NoError(t, bucket.Create(ctx))

	key := generateTestObjectKey()
	v, err := bucket.PutObjectInfo(ctx, key, bytes.NewReader([]byte("abc")), 3)
	require.NoError(t, err)
	assert.Empty(t, v.VersionID)
	assert.Equal(t, int64(3), v.Size)

	c, err := bucket.CopyObjectVersion(ctx, CopySource{Name: key}, bucket.bucketName, generateTestObjectKey()+"-copy")
	require.NoError(t, err)
	assert.Empty(t, c.VersionID)
	assert.NotEmpty(t, c.ETag)
}
