package s3

import (
	"context"
	"io"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/oddbit-project/blueprint/utils"
)

const (
	// Default configuration values
	DefaultTimeout            = time.Minute * 5 // Increased to 5 minutes for large uploads
	DefaultRegion             = "eu-west-1"
	DefaultMultipartThreshold = int64(100 * 1024 * 1024) // 100MB
	DefaultPartSize           = int64(10 * 1024 * 1024)  // Increased to 10MB for better performance
	DefaultMaxUploadParts     = 10000
	DefaultMaxRetries         = 3
	DefaultUploadTimeout      = time.Minute * 30 // 30 minutes for large file uploads

	// Minimum and maximum part sizes for multipart uploads
	MinPartSize = int64(5 * 1024 * 1024)        // 5MB
	MaxPartSize = int64(5 * 1024 * 1024 * 1024) // 5GB

	// Server-Side Encryption types
	SSEAlgorithmAES256  = "AES256"
	SSEAlgorithmKMS     = "aws:kms"
	SSEAlgorithmKMSDSSE = "aws:kms:dsse"

	// Customer-provided encryption algorithm
	SSECAlgorithmAES256 = "AES256"

	// Object Lock retention modes (WORM)
	RetentionGovernance = "GOVERNANCE"
	RetentionCompliance = "COMPLIANCE"

	// Legal hold statuses
	LegalHoldEnabled  = "ON"
	LegalHoldDisabled = "OFF"

	// Bucket object-lock default retention validity units
	ValidityDays  = "DAYS"
	ValidityYears = "YEARS"
)

// Error constants
const (
	ErrNilConfig           = utils.Error("Config is nil")
	ErrMissingEndpoint     = utils.Error("missing endpoint")
	ErrMissingRegion       = utils.Error("missing region")
	ErrInvalidTimeout      = utils.Error("invalid timeout")
	ErrInvalidPartSize     = utils.Error("invalid part size")
	ErrInvalidThreshold    = utils.Error("invalid multipart threshold")
	ErrBucketNotFound      = utils.Error("bucket not found")
	ErrBucketAlreadyExists = utils.Error("bucket already exists")
	ErrObjectNotFound      = utils.Error("object not found")
	ErrInvalidBucketName   = utils.Error("invalid bucket bucketName")
	ErrInvalidObjectKey    = utils.Error("invalid object key")
	ErrClientNotConnected  = utils.Error("client not connected")
)

// BucketInfo represents information about an S3 bucket
type BucketInfo struct {
	Name         string
	CreationDate time.Time
}

// ObjectInfo represents information about an S3 object
type ObjectInfo struct {
	Key          string
	Size         int64
	LastModified time.Time
	ETag         string
	StorageClass string
	ContentType  string
	// Versioning fields (populated when listing with Versions enabled or on
	// version-enabled buckets)
	VersionID      string
	IsLatest       bool
	IsDeleteMarker bool
}

// ListOptions represents options for listing objects
type ListOptions struct {
	Prefix     string
	Delimiter  string
	MaxKeys    int32
	StartAfter string
	Versions   bool // list all object versions (including delete markers)
}

// ObjectOptions represents options for object operations
type ObjectOptions struct {
	ContentType        string
	CacheControl       string
	ContentDisposition string
	ContentEncoding    string
	ContentLanguage    string
	Metadata           map[string]string
	Tags               map[string]string
	StorageClass       string
	// Server-Side Encryption options
	ServerSideEncryption    string            // AES256 or aws:kms (aws:kms:dsse is not supported)
	SSEKMSKeyId             string            // KMS key ID for SSE-KMS
	SSEKMSEncryptionContext map[string]string // KMS encryption context
	SSECustomerAlgorithm    string            // Customer-provided encryption algorithm (AES256)
	SSECustomerKey          string            // Customer-provided encryption key (base64)
	SSECustomerKeyMD5       string            // MD5 digest of customer key
	BucketKeyEnabled        *bool             // Enable S3 Bucket Key for cost optimization
	// SourceSSECustomerKey decrypts an SSE-C encrypted source on CopyObject
	// (base64-encoded key); ignored by other operations.
	SourceSSECustomerKey string
	// Object Lock / WORM options (require an object-lock-enabled bucket)
	LockMode        string    // RetentionGovernance or RetentionCompliance
	RetainUntilDate time.Time // retain-until date for LockMode
	LegalHold       string    // LegalHoldEnabled or LegalHoldDisabled
}

// BucketOptions represents options for bucket operations
type BucketOptions struct {
	Region        string
	ACL           string
	ObjectLocking bool // enable Object Lock (WORM) at bucket creation; cannot be changed later
}

// DeleteOptions represents options for deleting an object
type DeleteOptions struct {
	VersionID        string // delete a specific object version
	GovernanceBypass bool   // bypass GOVERNANCE retention (requires s3:BypassGovernanceRetention)
	ForceDelete      bool   // MinIO extension: force delete of a WORM-locked object
}

// RetentionOptions represents options for setting object retention (WORM)
type RetentionOptions struct {
	Mode             string    // RetentionGovernance or RetentionCompliance
	RetainUntilDate  time.Time // date until which the object is locked
	GovernanceBypass bool      // bypass governance mode to shorten/remove retention
	VersionID        string    // optional object version
}

// ObjectRetention represents the retention state of an object
type ObjectRetention struct {
	Mode            string
	RetainUntilDate time.Time
}

// ObjectLockConfig represents a bucket's Object Lock configuration
type ObjectLockConfig struct {
	Enabled  bool   // Object Lock enabled on the bucket
	Mode     string // default retention mode, empty if no default set
	Validity uint   // default retention validity
	Unit     string // ValidityDays or ValidityYears
}

// DownloadOptions provides options for download operations
type DownloadOptions struct {
	// Range specification
	StartByte *int64
	EndByte   *int64

	// Concurrency control
	Concurrency int

	// Part size for multipart downloads
	PartSize int64
}

// UploadOptions provides options for advanced upload operations
type UploadOptions struct {
	ObjectOptions
	// Additional upload-specific options
	LeavePartsOnError bool // Leave successfully uploaded parts on error for manual recovery
	MaxUploadParts    int  // Override the default maximum number of parts
	Concurrency       int  // Override the default concurrency level
}

type ClientInterface interface {
	// Connection management
	Connect(ctx context.Context) error
	MinioClient() *minio.Client
	Close() error
	IsConnected() bool

	// Bucket operations
	CreateBucket(ctx context.Context, bucket string, opts ...BucketOptions) error
	DeleteBucket(ctx context.Context, bucket string) error
	ListBuckets(ctx context.Context) ([]BucketInfo, error)
	BucketExists(ctx context.Context, bucket string) (bool, error)
}

// BucketInterface is the set of operations provided by *Bucket. All operations
// act on the bucket the *Bucket was created for.
type BucketInterface interface {
	// Object operations
	PutObject(ctx context.Context, name string, reader io.Reader, size int64, opts ...ObjectOptions) error
	GetObject(ctx context.Context, name string, versionID ...string) (io.ReadCloser, error)
	DeleteObject(ctx context.Context, name string, opts ...DeleteOptions) error
	ListObjects(ctx context.Context, opts ...ListOptions) ([]ObjectInfo, error)
	ObjectExists(ctx context.Context, name string) (bool, error)
	HeadObject(ctx context.Context, name string, versionID ...string) (*ObjectInfo, error)

	// Advanced operations
	PutObjectStream(ctx context.Context, name string, reader io.Reader, opts ...ObjectOptions) error
	GetObjectStream(ctx context.Context, name string, writer io.Writer) error
	CopyObject(ctx context.Context, srcName, dstBucket, dstName string, opts ...ObjectOptions) error

	// Advanced download operations
	GetObjectRange(ctx context.Context, name string, start, end int64) (io.ReadCloser, error)
	GetObjectStreamRange(ctx context.Context, name string, writer io.Writer, start, end int64) error
	GetObjectAdvanced(ctx context.Context, name string, writer io.Writer, opts DownloadOptions) error

	// Multipart upload operations
	PutObjectMultipart(ctx context.Context, name string, reader io.Reader, size int64, opts ...ObjectOptions) error
	PutObjectAdvanced(ctx context.Context, name string, reader io.Reader, size int64, opts UploadOptions) error

	// Pre-signed URLs
	PresignGetObject(ctx context.Context, name string, expiry time.Duration) (string, error)
	PresignPutObject(ctx context.Context, name string, expiry time.Duration, opts ...ObjectOptions) (string, error)
	PresignHeadObject(ctx context.Context, name string, expiry time.Duration) (string, error)

	// Object Lock / WORM operations
	SetObjectRetention(ctx context.Context, name string, opts RetentionOptions) error
	GetObjectRetention(ctx context.Context, name string, versionID ...string) (*ObjectRetention, error)
	SetObjectLegalHold(ctx context.Context, name string, enabled bool) error
	GetObjectLegalHold(ctx context.Context, name string, versionID ...string) (bool, error)
	SetObjectLockConfig(ctx context.Context, mode string, validity uint, unit string) error
	GetObjectLockConfig(ctx context.Context) (*ObjectLockConfig, error)
}

// Compile-time assertions that the concrete types satisfy the interfaces.
var (
	_ ClientInterface = (*Client)(nil)
	_ BucketInterface = (*Bucket)(nil)
)
