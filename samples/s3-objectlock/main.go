// Command s3-objectlock demonstrates the S3 provider's Object Lock (WORM) features:
// bucket-level default retention, per-object retention (Governance/Compliance),
// governance bypass, and legal holds.
//
// Run against a local MinIO (Object Lock requires an object-lock-enabled bucket):
//
//	docker run -p 9000:9000 -e MINIO_ROOT_USER=minioadmin -e MINIO_ROOT_PASSWORD=minioadmin \
//	    quay.io/minio/minio:latest server /data
//	go run ./samples/s3-objectlock
package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/oddbit-project/blueprint/log"
	"github.com/oddbit-project/blueprint/provider/s3"
)

func main() {
	var (
		endpoint  = flag.String("endpoint", "localhost:9000", "S3 endpoint")
		region    = flag.String("region", "us-east-1", "S3 region")
		accessKey = flag.String("access-key", "minioadmin", "S3 access key")
		secretKey = flag.String("secret-key", "minioadmin", "S3 secret key")
		useSSL    = flag.Bool("ssl", false, "Use SSL/TLS")
	)
	flag.Parse()

	if err := run(*endpoint, *region, *accessKey, *secretKey, *useSSL); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(endpoint, region, accessKey, secretKey string, useSSL bool) error {
	logger := log.New("s3-objectlock")

	config := s3.NewConfig()
	config.Endpoint = endpoint
	config.Region = region
	config.AccessKeyID = accessKey
	config.UseSSL = useSSL
	config.ForcePathStyle = true // required for MinIO

	// Provide the secret via an environment variable (never hardcode in production).
	os.Setenv("S3_OBJECTLOCK_SECRET", secretKey)
	config.DefaultCredentialConfig.PasswordEnvVar = "S3_OBJECTLOCK_SECRET"

	client, err := s3.NewClient(config, logger)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		return err
	}
	defer client.Close()

	bucketName := fmt.Sprintf("worm-demo-%d", time.Now().Unix())
	bucket, err := client.Bucket(bucketName)
	if err != nil {
		return err
	}

	// Object Lock can ONLY be enabled at bucket creation; it cannot be turned on later.
	fmt.Printf("Creating object-lock-enabled bucket %q\n", bucketName)
	if err := bucket.Create(ctx, s3.BucketOptions{ObjectLocking: true}); err != nil {
		return err
	}

	// 1. Inspect the bucket Object Lock configuration.
	cfg, err := bucket.GetObjectLockConfig(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("Object Lock enabled: %t\n", cfg.Enabled)

	// 2. Set a bucket-level default retention applied to every new upload.
	fmt.Println("Setting bucket default retention: GOVERNANCE / 1 day")
	if err := bucket.SetObjectLockConfig(ctx, s3.RetentionGovernance, 1, s3.ValidityDays); err != nil {
		return err
	}

	// 3. Upload an object with an explicit retention date (overrides the bucket default).
	key := "report.txt"
	data := []byte("immutable financial report")
	retainUntil := time.Now().Add(2 * time.Hour).UTC().Truncate(time.Second)
	fmt.Printf("Uploading %q locked until %s\n", key, retainUntil.Format(time.RFC3339))
	if err := bucket.PutObject(ctx, key, bytes.NewReader(data), int64(len(data)), s3.ObjectOptions{
		LockMode:        s3.RetentionGovernance,
		RetainUntilDate: retainUntil,
	}); err != nil {
		return err
	}

	ret, err := bucket.GetObjectRetention(ctx, key)
	if err != nil {
		return err
	}
	fmt.Printf("Retention -> mode=%s until=%s\n", ret.Mode, ret.RetainUntilDate.Format(time.RFC3339))

	// While the retention is in force the object version is immutable. Note that
	// enabling Object Lock also enables versioning, so a plain DeleteObject (no
	// version id) only writes a delete marker and leaves the locked version intact.

	// 4. Shorten the retention using a governance bypass (needs s3:BypassGovernanceRetention).
	shorter := time.Now().Add(15 * time.Minute).UTC().Truncate(time.Second)
	fmt.Printf("Shortening retention to %s with governance bypass\n", shorter.Format(time.RFC3339))
	if err := bucket.SetObjectRetention(ctx, key, s3.RetentionOptions{
		Mode:             s3.RetentionGovernance,
		RetainUntilDate:  shorter,
		GovernanceBypass: true,
	}); err != nil {
		return err
	}

	// 5. Legal hold: protects an object independently of any retention period.
	fmt.Println("Placing legal hold")
	if err := bucket.SetObjectLegalHold(ctx, key, true); err != nil {
		return err
	}
	held, err := bucket.GetObjectLegalHold(ctx, key)
	if err != nil {
		return err
	}
	fmt.Printf("Legal hold: %t\n", held)

	fmt.Println("Releasing legal hold")
	if err := bucket.SetObjectLegalHold(ctx, key, false); err != nil {
		return err
	}

	fmt.Println("Done.")
	return nil
}
