package s3

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLockConstants(t *testing.T) {
	assert.Equal(t, "GOVERNANCE", RetentionGovernance)
	assert.Equal(t, "COMPLIANCE", RetentionCompliance)
	assert.Equal(t, "ON", LegalHoldEnabled)
	assert.Equal(t, "OFF", LegalHoldDisabled)
	assert.Equal(t, "DAYS", ValidityDays)
	assert.Equal(t, "YEARS", ValidityYears)
}

func TestLockOperationsWithoutConnection(t *testing.T) {
	config := NewConfig()
	client, err := NewClient(config, nil)
	require.NoError(t, err)

	ctx := context.Background()
	bucket, err := client.Bucket("bucket")
	require.NoError(t, err)

	err = bucket.SetObjectRetention(ctx, "key", RetentionOptions{
		Mode:            RetentionGovernance,
		RetainUntilDate: time.Now().Add(time.Hour),
	})
	assert.Equal(t, ErrClientNotConnected, err)

	_, err = bucket.GetObjectRetention(ctx, "key")
	assert.Equal(t, ErrClientNotConnected, err)

	err = bucket.SetObjectLegalHold(ctx, "key", true)
	assert.Equal(t, ErrClientNotConnected, err)

	_, err = bucket.GetObjectLegalHold(ctx, "key")
	assert.Equal(t, ErrClientNotConnected, err)

	err = bucket.SetObjectLockConfig(ctx, RetentionGovernance, 1, ValidityDays)
	assert.Equal(t, ErrClientNotConnected, err)

	_, err = bucket.GetObjectLockConfig(ctx)
	assert.Equal(t, ErrClientNotConnected, err)
}

// Integration test for Object Lock / WORM operations using testcontainers
func TestIntegrationObjectLock(t *testing.T) {
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
	require.NoError(t, err)

	// Object Lock can only be enabled at bucket creation time
	err = bucket.Create(ctx, BucketOptions{ObjectLocking: true})
	require.NoError(t, err, "Should create object-lock-enabled bucket")

	t.Run("GetObjectLockConfig reports enabled", func(t *testing.T) {
		cfg, err := bucket.GetObjectLockConfig(ctx)
		require.NoError(t, err)
		assert.True(t, cfg.Enabled, "Object Lock should be enabled on the bucket")
	})

	t.Run("PutObject with retention", func(t *testing.T) {
		key := generateTestObjectKey()
		retainUntil := time.Now().Add(2 * time.Hour).UTC().Truncate(time.Second)
		data := []byte("worm data")

		err := bucket.PutObject(ctx, key, bytes.NewReader(data), int64(len(data)), ObjectOptions{
			LockMode:        RetentionGovernance,
			RetainUntilDate: retainUntil,
		})
		require.NoError(t, err)

		ret, err := bucket.GetObjectRetention(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, RetentionGovernance, ret.Mode)
		assert.WithinDuration(t, retainUntil, ret.RetainUntilDate, time.Second)
	})

	t.Run("SetAndGetObjectRetention with governance bypass", func(t *testing.T) {
		key := generateTestObjectKey()
		data := []byte("data")
		require.NoError(t, bucket.PutObject(ctx, key, bytes.NewReader(data), int64(len(data))))

		retainUntil := time.Now().Add(time.Hour).UTC().Truncate(time.Second)
		err := bucket.SetObjectRetention(ctx, key, RetentionOptions{
			Mode:            RetentionGovernance,
			RetainUntilDate: retainUntil,
		})
		require.NoError(t, err)

		ret, err := bucket.GetObjectRetention(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, RetentionGovernance, ret.Mode)
		assert.WithinDuration(t, retainUntil, ret.RetainUntilDate, time.Second)

		// Shortening a GOVERNANCE retention requires the bypass flag
		shorter := time.Now().Add(30 * time.Minute).UTC().Truncate(time.Second)
		err = bucket.SetObjectRetention(ctx, key, RetentionOptions{
			Mode:             RetentionGovernance,
			RetainUntilDate:  shorter,
			GovernanceBypass: true,
		})
		require.NoError(t, err)

		ret, err = bucket.GetObjectRetention(ctx, key)
		require.NoError(t, err)
		assert.WithinDuration(t, shorter, ret.RetainUntilDate, time.Second)
	})

	t.Run("SetAndGetObjectLegalHold", func(t *testing.T) {
		key := generateTestObjectKey()
		data := []byte("data")
		require.NoError(t, bucket.PutObject(ctx, key, bytes.NewReader(data), int64(len(data))))

		// An object that never had a legal hold reports false, not an error.
		held, err := bucket.GetObjectLegalHold(ctx, key)
		require.NoError(t, err)
		assert.False(t, held, "Legal hold should be off by default")

		require.NoError(t, bucket.SetObjectLegalHold(ctx, key, true))
		held, err = bucket.GetObjectLegalHold(ctx, key)
		require.NoError(t, err)
		assert.True(t, held, "Legal hold should be enabled")

		require.NoError(t, bucket.SetObjectLegalHold(ctx, key, false))
		held, err = bucket.GetObjectLegalHold(ctx, key)
		require.NoError(t, err)
		assert.False(t, held, "Legal hold should be disabled")
	})

	// Run last: a bucket default retention is inherited by every subsequent
	// PutObject, which would WORM-protect objects created by other subtests.
	t.Run("SetAndGetObjectLockConfig default retention", func(t *testing.T) {
		err := bucket.SetObjectLockConfig(ctx, RetentionGovernance, 1, ValidityDays)
		require.NoError(t, err)

		cfg, err := bucket.GetObjectLockConfig(ctx)
		require.NoError(t, err)
		assert.True(t, cfg.Enabled)
		assert.Equal(t, RetentionGovernance, cfg.Mode)
		assert.Equal(t, uint(1), cfg.Validity)
		assert.Equal(t, ValidityDays, cfg.Unit)

		// Clearing the default retention leaves Object Lock enabled.
		require.NoError(t, bucket.SetObjectLockConfig(ctx, "", 0, ""))
		cfg, err = bucket.GetObjectLockConfig(ctx)
		require.NoError(t, err)
		assert.True(t, cfg.Enabled)
		assert.Empty(t, cfg.Mode)
	})
}

// GetObjectLockConfig on a bucket without Object Lock reports disabled, not an error.
func TestIntegrationObjectLockConfigDisabledBucket(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	container, cleanup := setupMinIOContainer(ctx, t)
	defer cleanup()

	client := createTestClientWithContainer(t, container)
	defer client.Close()

	bucket, err := client.Bucket(generateTestBucketName())
	require.NoError(t, err)
	require.NoError(t, bucket.Create(ctx)) // no ObjectLocking

	cfg, err := bucket.GetObjectLockConfig(ctx)
	require.NoError(t, err)
	assert.False(t, cfg.Enabled)
}

// Integration test for deleting a specific WORM-locked object version
func TestIntegrationDeleteLockedVersion(t *testing.T) {
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
	require.NoError(t, err)

	require.NoError(t, bucket.Create(ctx, BucketOptions{ObjectLocking: true}))

	key := generateTestObjectKey()
	data := []byte("locked version")
	require.NoError(t, bucket.PutObject(ctx, key, bytes.NewReader(data), int64(len(data)), ObjectOptions{
		LockMode:        RetentionGovernance,
		RetainUntilDate: time.Now().Add(time.Hour).UTC().Truncate(time.Second),
	}))

	// Object Lock enables versioning; HeadObject surfaces the version id.
	head, err := bucket.HeadObject(ctx, key)
	require.NoError(t, err)
	require.NotEmpty(t, head.VersionID)

	// Deleting the locked version without bypass must fail.
	err = bucket.DeleteObject(ctx, key, DeleteOptions{VersionID: head.VersionID})
	assert.Error(t, err, "deleting a governance-locked version without bypass should fail")

	// With governance bypass the locked version can be removed.
	err = bucket.DeleteObject(ctx, key, DeleteOptions{
		VersionID:        head.VersionID,
		GovernanceBypass: true,
	})
	require.NoError(t, err)

	// A version-aware listing should no longer contain that version.
	versions, err := bucket.ListObjects(ctx, ListOptions{Prefix: key, Versions: true})
	require.NoError(t, err)
	for _, v := range versions {
		assert.NotEqual(t, head.VersionID, v.VersionID, "deleted version should not be listed")
	}
}
