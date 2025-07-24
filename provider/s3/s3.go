package s3

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"io"
	"time"

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
)

// Error constants
const (
	ErrNilConfig          = utils.Error("Config is nil")
	ErrMissingEndpoint    = utils.Error("missing endpoint")
	ErrMissingRegion      = utils.Error("missing region")
	ErrInvalidTimeout     = utils.Error("invalid timeout")
	ErrInvalidPartSize    = utils.Error("invalid part size")
	ErrInvalidThreshold   = utils.Error("invalid multipart threshold")
	ErrBucketNotFound     = utils.Error("bucket not found")
	ErrObjectNotFound     = utils.Error("object not found")
	ErrInvalidBucketName  = utils.Error("invalid bucket bucketName")
	ErrInvalidObjectKey   = utils.Error("invalid object key")
	ErrClientNotConnected = utils.Error("client not connected")
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
}

// ListOptions represents options for listing objects
type ListOptions struct {
	Prefix     string
	Delimiter  string
	MaxKeys    int32
	StartAfter string
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
	ServerSideEncryption    string            // AES256, aws:kms, aws:kms:dsse
	SSEKMSKeyId             string            // KMS key ID for SSE-KMS
	SSEKMSEncryptionContext map[string]string // KMS encryption context
	SSECustomerAlgorithm    string            // Customer-provided encryption algorithm (AES256)
	SSECustomerKey          string            // Customer-provided encryption key (base64)
	SSECustomerKeyMD5       string            // MD5 digest of customer key
	BucketKeyEnabled        *bool             // Enable S3 Bucket Key for cost optimization
}

// BucketOptions represents options for bucket operations
type BucketOptions struct {
	Region string
	ACL    string
}

// UploadProgress represents progress information for uploads
type UploadProgress struct {
	BytesUploaded int64
	TotalBytes    int64
	PartsUploaded int
	TotalParts    int
}

// ProgressCallback is a function called during upload/download progress
type ProgressCallback func(progress UploadProgress)

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
	S3Client() *s3.Client
	Close() error
	IsConnected() bool

	// Bucket operations
	CreateBucket(ctx context.Context, bucket string, opts ...BucketOptions) error
	DeleteBucket(ctx context.Context, bucket string) error
	ListBuckets(ctx context.Context) ([]BucketInfo, error)
	BucketExists(ctx context.Context, bucket string) (bool, error)
}

type BucketInterface interface {
	// Object operations
	PutObject(ctx context.Context, bucket, key string, reader io.Reader, size int64, opts ...ObjectOptions) error
	GetObject(ctx context.Context, bucket, key string) (io.ReadCloser, error)
	DeleteObject(ctx context.Context, bucket, key string) error
	ListObjects(ctx context.Context, bucket string, opts ...ListOptions) ([]ObjectInfo, error)
	ObjectExists(ctx context.Context, bucket, key string) (bool, error)
	HeadObject(ctx context.Context, bucket, key string) (*ObjectInfo, error)

	// Advanced operations
	PutObjectStream(ctx context.Context, bucket, key string, reader io.Reader, opts ...ObjectOptions) error
	GetObjectStream(ctx context.Context, bucket, key string, writer io.Writer) error
	CopyObject(ctx context.Context, srcBucket, srcKey, dstBucket, dstKey string, opts ...ObjectOptions) error

	// Advanced download operations
	GetObjectRange(ctx context.Context, bucket, key string, start, end int64) (io.ReadCloser, error)
	GetObjectStreamRange(ctx context.Context, bucket, key string, writer io.Writer, start, end int64) error
	GetObjectAdvanced(ctx context.Context, bucket, key string, writer io.Writer, opts DownloadOptions) error

	// Multipart upload operations
	PutObjectMultipart(ctx context.Context, bucket, key string, reader io.Reader, size int64, progress ProgressCallback, opts ...ObjectOptions) error
	PutObjectAdvanced(ctx context.Context, bucket, key string, reader io.Reader, size int64, progress ProgressCallback, opts UploadOptions) error

	// Pre-signed URLs
	PresignGetObject(ctx context.Context, bucket, key string, expiry time.Duration) (string, error)
	PresignPutObject(ctx context.Context, bucket, key string, expiry time.Duration, opts ...ObjectOptions) (string, error)
}
