package s3

import (
	"context"
	"net/http"

	"github.com/minio/minio-go/v7"
	"github.com/oddbit-project/blueprint/log"
)

// SetObjectRetention applies a WORM retention period to an object
func (b *Bucket) SetObjectRetention(ctx context.Context, name string, opts RetentionOptions) error {
	if !b.IsConnected() {
		return ErrClientNotConnected
	}

	startTime := logOperationStart(b.logger, "set_object_retention", name, log.KV{
		"bucket_name": b.bucketName,
	})

	ctx, cancel := getContextWithTimeout(b.timeout, ctx)
	defer cancel()

	putOpts := minio.PutObjectRetentionOptions{
		GovernanceBypass: opts.GovernanceBypass,
		VersionID:        opts.VersionID,
	}
	if opts.Mode != "" {
		mode := minio.RetentionMode(opts.Mode)
		putOpts.Mode = &mode
	}
	if !opts.RetainUntilDate.IsZero() {
		retainUntil := opts.RetainUntilDate
		putOpts.RetainUntilDate = &retainUntil
	}

	err := b.minioClient.PutObjectRetention(ctx, b.bucketName, name, putOpts)

	logOperationEnd(b.logger, "set_object_retention", name, startTime, err, log.KV{
		"bucket_name": b.bucketName,
	})

	return err
}

// GetObjectRetention returns the WORM retention state of an object.
// Pass an optional version id to target a specific version.
func (b *Bucket) GetObjectRetention(ctx context.Context, name string, versionID ...string) (*ObjectRetention, error) {
	if !b.IsConnected() {
		return nil, ErrClientNotConnected
	}

	ctx, cancel := getContextWithTimeout(b.timeout, ctx)
	defer cancel()

	version := ""
	if len(versionID) > 0 {
		version = versionID[0]
	}

	mode, retainUntilDate, err := b.minioClient.GetObjectRetention(ctx, b.bucketName, name, version)
	if err != nil {
		return nil, err
	}

	info := &ObjectRetention{}
	if mode != nil {
		info.Mode = string(*mode)
	}
	if retainUntilDate != nil {
		info.RetainUntilDate = *retainUntilDate
	}

	return info, nil
}

// SetObjectLegalHold enables or disables legal hold on an object
func (b *Bucket) SetObjectLegalHold(ctx context.Context, name string, enabled bool) error {
	if !b.IsConnected() {
		return ErrClientNotConnected
	}

	startTime := logOperationStart(b.logger, "set_object_legal_hold", name, log.KV{
		"bucket_name": b.bucketName,
		"enabled":     enabled,
	})

	ctx, cancel := getContextWithTimeout(b.timeout, ctx)
	defer cancel()

	status := minio.LegalHoldDisabled
	if enabled {
		status = minio.LegalHoldEnabled
	}

	err := b.minioClient.PutObjectLegalHold(ctx, b.bucketName, name, minio.PutObjectLegalHoldOptions{
		Status: &status,
	})

	logOperationEnd(b.logger, "set_object_legal_hold", name, startTime, err, log.KV{
		"bucket_name": b.bucketName,
		"enabled":     enabled,
	})

	return err
}

// GetObjectLegalHold returns true if legal hold is enabled on an object.
// Pass an optional version id to target a specific version. Objects that never
// had a legal hold set report false (not an error).
func (b *Bucket) GetObjectLegalHold(ctx context.Context, name string, versionID ...string) (bool, error) {
	if !b.IsConnected() {
		return false, ErrClientNotConnected
	}

	ctx, cancel := getContextWithTimeout(b.timeout, ctx)
	defer cancel()

	getOpts := minio.GetObjectLegalHoldOptions{}
	if len(versionID) > 0 {
		getOpts.VersionID = versionID[0]
	}

	status, err := b.minioClient.GetObjectLegalHold(ctx, b.bucketName, name, getOpts)
	if err != nil {
		// MinIO returns an error when no legal hold was ever set; treat as "off".
		if resp := minio.ToErrorResponse(err); resp.StatusCode == http.StatusNotFound ||
			resp.Code == "NoSuchObjectLockConfiguration" {
			return false, nil
		}
		return false, err
	}

	return status != nil && *status == minio.LegalHoldEnabled, nil
}

// SetObjectLockConfig sets the bucket-level default Object Lock retention.
// Pass an empty mode with zero validity to clear the default retention.
func (b *Bucket) SetObjectLockConfig(ctx context.Context, mode string, validity uint, unit string) error {
	if !b.IsConnected() {
		return ErrClientNotConnected
	}

	startTime := logOperationStart(b.logger, "set_object_lock_config", b.bucketName, log.KV{
		"bucket_name": b.bucketName,
	})

	ctx, cancel := getContextWithTimeout(b.timeout, ctx)
	defer cancel()

	var (
		modePtr     *minio.RetentionMode
		validityPtr *uint
		unitPtr     *minio.ValidityUnit
	)
	if mode != "" {
		m := minio.RetentionMode(mode)
		modePtr = &m
		v := validity
		validityPtr = &v
		u := minio.ValidityUnit(unit)
		unitPtr = &u
	}

	err := b.minioClient.SetObjectLockConfig(ctx, b.bucketName, modePtr, validityPtr, unitPtr)

	logOperationEnd(b.logger, "set_object_lock_config", b.bucketName, startTime, err, log.KV{
		"bucket_name": b.bucketName,
	})

	return err
}

// GetObjectLockConfig returns the bucket's Object Lock configuration
func (b *Bucket) GetObjectLockConfig(ctx context.Context) (*ObjectLockConfig, error) {
	if !b.IsConnected() {
		return nil, ErrClientNotConnected
	}

	ctx, cancel := getContextWithTimeout(b.timeout, ctx)
	defer cancel()

	objectLock, mode, validity, unit, err := b.minioClient.GetObjectLockConfig(ctx, b.bucketName)
	if err != nil {
		// A bucket without Object Lock reports disabled rather than erroring.
		if resp := minio.ToErrorResponse(err); resp.StatusCode == http.StatusNotFound ||
			resp.Code == "ObjectLockConfigurationNotFoundError" {
			return &ObjectLockConfig{Enabled: false}, nil
		}
		return nil, err
	}

	cfg := &ObjectLockConfig{
		Enabled: objectLock == "Enabled",
	}
	if mode != nil {
		cfg.Mode = string(*mode)
	}
	if validity != nil {
		cfg.Validity = *validity
	}
	if unit != nil {
		cfg.Unit = string(*unit)
	}

	return cfg, nil
}
