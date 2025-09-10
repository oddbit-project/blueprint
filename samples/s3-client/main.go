package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/oddbit-project/blueprint/log"
	"github.com/oddbit-project/blueprint/provider/s3"
)

const (
	// Default configuration for MinIO
	defaultEndpoint  = "localhost:9000"
	defaultRegion    = "us-east-1"
	defaultAccessKey = "minioadmin"
	defaultSecretKey = "minioadmin"
	defaultBucket    = "test-bucket"
)

// CLI represents the command-line interface
type CLI struct {
	s3Client      *s3.Client
	logger        *log.Logger
	uploadTimeout time.Duration
}

// Command represents a CLI command
type Command struct {
	Name        string
	Description string
	Usage       string
	Handler     func(*CLI, []string) error
}

var commands []Command

func init() {
	commands = []Command{
		{
			Name:        "create-bucket",
			Description: "Create a new S3 bucket",
			Usage:       "create-bucket <bucket-name>",
			Handler:     (*CLI).createBucket,
		},
		{
			Name:        "list-buckets",
			Description: "List all S3 buckets",
			Usage:       "list-buckets",
			Handler:     (*CLI).listBuckets,
		},
		{
			Name:        "delete-bucket",
			Description: "Delete an S3 bucket",
			Usage:       "delete-bucket <bucket-name>",
			Handler:     (*CLI).deleteBucket,
		},
		{
			Name:        "upload",
			Description: "Upload a file to S3",
			Usage:       "upload <bucket-name> <local-file> [remote-key]",
			Handler:     (*CLI).uploadFile,
		},
		{
			Name:        "download",
			Description: "Download a file from S3",
			Usage:       "download <bucket-name> <remote-key> [local-file]",
			Handler:     (*CLI).downloadFile,
		},
		{
			Name:        "list-objects",
			Description: "List objects in a bucket",
			Usage:       "list-objects <bucket-name> [prefix]",
			Handler:     (*CLI).listObjects,
		},
		{
			Name:        "delete-object",
			Description: "Delete an object from S3",
			Usage:       "delete-object <bucket-name> <object-key>",
			Handler:     (*CLI).deleteObject,
		},
		{
			Name:        "presign-get",
			Description: "Generate presigned URL for downloading",
			Usage:       "presign-get <bucket-name> <object-key> [expiry-minutes]",
			Handler:     (*CLI).presignGet,
		},
		{
			Name:        "presign-put",
			Description: "Generate presigned URL for uploading",
			Usage:       "presign-put <bucket-name> <object-key> [expiry-minutes]",
			Handler:     (*CLI).presignPut,
		},
		{
			Name:        "help",
			Description: "Show help information",
			Usage:       "help [command]",
			Handler:     (*CLI).showHelp,
		},
	}
}

func main() {
	// Define command line flags
	var (
		endpoint      = flag.String("endpoint", defaultEndpoint, "S3 endpoint URL")
		region        = flag.String("region", defaultRegion, "S3 region")
		accessKey     = flag.String("access-key", defaultAccessKey, "S3 access key")
		secretKey     = flag.String("secret-key", defaultSecretKey, "S3 secret key")
		useSSL        = flag.Bool("ssl", false, "Use SSL/TLS connection")
		verbose       = flag.Bool("verbose", false, "Enable verbose logging")
		showVersion   = flag.Bool("version", false, "Show version information")
		uploadTimeout = flag.Int("upload-timeout", 1800, "Upload timeout in seconds (default: 30 minutes)")
		// Connection pooling options
		maxIdleConns        = flag.Int("max-idle-conns", 50, "Maximum idle connections in pool")
		maxIdleConnsPerHost = flag.Int("max-idle-conns-per-host", 10, "Maximum idle connections per host")
		maxConnsPerHost     = flag.Int("max-conns-per-host", 20, "Maximum connections per host")
		idleConnTimeout     = flag.Int("idle-conn-timeout", 60, "Idle connection timeout in seconds")
	)

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "S3 Client CLI - A demonstration of the Blueprint S3 provider\n\n")
		fmt.Fprintf(os.Stderr, "Usage: %s [OPTIONS] <command> [args...]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nConnection Pooling:\n")
		fmt.Fprintf(os.Stderr, "  Connection pooling improves performance by reusing HTTP connections.\n")
		fmt.Fprintf(os.Stderr, "  Adjust these settings based on your workload and server capacity.\n")
		fmt.Fprintf(os.Stderr, "\nCommands:\n")
		for _, cmd := range commands {
			fmt.Fprintf(os.Stderr, "  %-15s %s\n", cmd.Name, cmd.Description)
		}
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  %s create-bucket my-bucket\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s upload my-bucket ./file.txt\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s download my-bucket file.txt ./downloaded-file.txt\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nFor MinIO server, run: docker run -p 9000:9000 -p 9001:9001 --name minio quay.io/minio/minio server /data --console-address :9001\n")
	}

	flag.Parse()

	if *showVersion {
		fmt.Println("S3 Client CLI v1.0.0")
		fmt.Println("Built with Blueprint S3 Provider")
		return
	}

	args := flag.Args()
	if len(args) == 0 {
		flag.Usage()
		os.Exit(1)
	}

	// Initialize logger
	logger := log.New("s3-cli")
	if *verbose {
		logger.Info("Starting S3 CLI", map[string]interface{}{
			"endpoint":                *endpoint,
			"region":                  *region,
			"ssl":                     *useSSL,
			"max_idle_conns":          *maxIdleConns,
			"max_idle_conns_per_host": *maxIdleConnsPerHost,
			"max_conns_per_host":      *maxConnsPerHost,
			"idle_conn_timeout":       fmt.Sprintf("%ds", *idleConnTimeout),
		})
	}

	// Create S3 client configuration
	config := s3.NewConfig()
	config.Endpoint = *endpoint
	config.Region = *region
	config.AccessKeyID = *accessKey
	config.UseSSL = *useSSL
	config.ForcePathStyle = true // Required for MinIO
	config.UploadTimeoutSeconds = *uploadTimeout

	// Configure connection pooling
	config.MaxIdleConns = *maxIdleConns
	config.MaxIdleConnsPerHost = *maxIdleConnsPerHost
	config.MaxConnsPerHost = *maxConnsPerHost
	config.IdleConnTimeout = time.Duration(*idleConnTimeout) * time.Second

	// Set secret key using environment variable or flag
	if envSecret := os.Getenv("S3_SECRET_KEY"); envSecret != "" {
		config.DefaultCredentialConfig.PasswordEnvVar = "S3_SECRET_KEY"
	} else {
		// For demo purposes, we'll use the flag value directly
		// In production, always use environment variables or secure credential files
		os.Setenv("S3_SECRET_KEY_DEMO", *secretKey)
		config.DefaultCredentialConfig.PasswordEnvVar = "S3_SECRET_KEY_DEMO"
	}

	if *verbose {
		logger.Info("Credential configuration", map[string]interface{}{
			"access_key": *accessKey,
			"env_var":    config.DefaultCredentialConfig.PasswordEnvVar,
			"has_secret": os.Getenv(config.DefaultCredentialConfig.PasswordEnvVar) != "",
		})
	}

	// Create S3 client
	client, err := s3.NewClient(config, logger)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating S3 client: %v\n", err)
		os.Exit(1)
	}

	// Connect to S3 with longer timeout for large file operations
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Error connecting to S3: %v\n", err)
		os.Exit(1)
	}
	defer client.Close()

	if *verbose {
		logger.Info("Successfully connected to S3")
	}

	// Create CLI instance
	cli := &CLI{
		s3Client:      client,
		logger:        logger,
		uploadTimeout: time.Duration(*uploadTimeout) * time.Second,
	}

	// Find and execute command
	commandName := args[0]
	commandArgs := args[1:]

	for _, cmd := range commands {
		if cmd.Name == commandName {
			if err := cmd.Handler(cli, commandArgs); err != nil {
				fmt.Fprintf(os.Stderr, "Error executing command '%s': %v\n", commandName, err)
				os.Exit(1)
			}
			return
		}
	}

	fmt.Fprintf(os.Stderr, "Unknown command: %s\n", commandName)
	fmt.Fprintf(os.Stderr, "Run '%s help' for usage information\n", os.Args[0])
	os.Exit(1)
}

// createBucket creates a new S3 bucket
func (cli *CLI) createBucket(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: create-bucket <bucket-name>")
	}

	bucketName := args[0]
	ctx := context.Background()

	fmt.Printf("Creating bucket '%s'...\n", bucketName)

	bucket, err := cli.s3Client.Bucket(bucketName)
	if err != nil {
		return fmt.Errorf("failed to intialize bucket helper: %w", err)
	}

	err = bucket.Create(ctx)
	if err != nil {
		return fmt.Errorf("failed to create bucket: %w", err)
	}

	fmt.Printf("Bucket '%s' created successfully\n", bucketName)
	return nil
}

// listBuckets lists all S3 buckets
func (cli *CLI) listBuckets(args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("usage: list-buckets")
	}

	ctx := context.Background()

	fmt.Println("Listing buckets...")

	buckets, err := cli.s3Client.ListBuckets(ctx)
	if err != nil {
		return fmt.Errorf("failed to list buckets: %w", err)
	}

	if len(buckets) == 0 {
		fmt.Println("No buckets found")
		return nil
	}

	fmt.Printf("Found %d bucket(s):\n", len(buckets))
	for _, bucket := range buckets {
		fmt.Printf("  %-30s %s\n", bucket.Name, bucket.CreationDate.Format("2006-01-02 15:04:05"))
	}

	return nil
}

// deleteBucket deletes an S3 bucket
func (cli *CLI) deleteBucket(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: delete-bucket <bucket-name>")
	}

	bucketName := args[0]
	ctx := context.Background()

	fmt.Printf("Deleting bucket '%s'...\n", bucketName)

	bucket, err := cli.s3Client.Bucket(bucketName)
	if err != nil {
		return fmt.Errorf("failed to intialize bucket helper: %w", err)
	}

	err = bucket.Delete(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete bucket: %w", err)
	}

	fmt.Printf("Bucket '%s' deleted successfully\n", bucketName)
	return nil
}

// uploadFile uploads a file to S3
func (cli *CLI) uploadFile(args []string) error {
	if len(args) < 2 || len(args) > 3 {
		return fmt.Errorf("usage: upload <bucket-name> <local-file> [remote-key]")
	}

	bucketName := args[0]
	localFile := args[1]

	// Determine remote key
	remoteKey := filepath.Base(localFile)
	if len(args) == 3 {
		remoteKey = args[2]
	}

	// Use background context and rely on S3 provider's timeout management
	ctx := context.Background()

	// Open local file
	file, err := os.Open(localFile)
	if err != nil {
		return fmt.Errorf("failed to open local file: %w", err)
	}
	defer file.Close()

	// Get file info
	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}

	fmt.Printf("Uploading '%s' to 's3://%s/%s' (%s)...\n",
		localFile, bucketName, remoteKey, formatBytes(info.Size()))

	// Start timing the upload
	startTime := time.Now()

	// Upload with content type detection
	opts := s3.ObjectOptions{
		ContentType: detectContentType(localFile),
	}

	bucket, err := cli.s3Client.Bucket(bucketName)
	if err != nil {
		return fmt.Errorf("failed to intialize bucket helper: %w", err)
	}

	err = bucket.PutObject(ctx, remoteKey, file, info.Size(), opts)
	if err != nil {
		return fmt.Errorf("failed to upload file: %w", err)
	}

	// Calculate elapsed time and transfer speed
	elapsed := time.Since(startTime)
	speed := calculateTransferSpeed(info.Size(), elapsed)

	fmt.Printf("File uploaded successfully to 's3://%s/%s' (elapsed: %s, speed: %s/s)\n",
		bucketName, remoteKey, elapsed.Round(time.Millisecond), formatBytes(int64(speed)))
	return nil
}

// downloadFile downloads a file from S3
func (cli *CLI) downloadFile(args []string) error {
	if len(args) < 2 || len(args) > 3 {
		return fmt.Errorf("usage: download <bucket-name> <remote-key> [local-file]")
	}

	bucketName := args[0]
	remoteKey := args[1]

	// Determine local file path
	localFile := filepath.Base(remoteKey)
	if len(args) == 3 {
		localFile = args[2]
	}

	// Use background context and rely on S3 provider's timeout management
	ctx := context.Background()

	fmt.Printf("Downloading 's3://%s/%s' to '%s'...\n", bucketName, remoteKey, localFile)

	bucket, err := cli.s3Client.Bucket(bucketName)
	if err != nil {
		return fmt.Errorf("failed to intialize bucket helper: %w", err)
	}

	// Start timing the download
	startTime := time.Now()

	// Get object from S3
	reader, err := bucket.GetObject(ctx, remoteKey)
	if err != nil {
		return fmt.Errorf("failed to get object: %w", err)
	}
	defer reader.Close()

	// Create local file
	file, err := os.Create(localFile)
	if err != nil {
		return fmt.Errorf("failed to create local file: %w", err)
	}
	defer file.Close()

	// Copy data
	written, err := io.Copy(file, reader)
	if err != nil {
		return fmt.Errorf("failed to download file: %w", err)
	}

	// Calculate elapsed time and transfer speed
	elapsed := time.Since(startTime)
	speed := calculateTransferSpeed(written, elapsed)

	fmt.Printf("File downloaded successfully (%s, elapsed: %s, speed: %s/s)\n",
		formatBytes(written), elapsed.Round(time.Millisecond), formatBytes(int64(speed)))
	return nil
}

// listObjects lists objects in a bucket
func (cli *CLI) listObjects(args []string) error {
	if len(args) < 1 || len(args) > 2 {
		return fmt.Errorf("usage: list-objects <bucket-name> [prefix]")
	}

	bucketName := args[0]
	ctx := context.Background()

	var opts s3.ListOptions
	if len(args) == 2 {
		opts.Prefix = args[1]
		fmt.Printf("Listing objects in bucket '%s' with prefix '%s'...\n", bucketName, opts.Prefix)
	} else {
		fmt.Printf("Listing objects in bucket '%s'...\n", bucketName)
	}

	bucket, err := cli.s3Client.Bucket(bucketName)
	if err != nil {
		return fmt.Errorf("failed to intialize bucket helper: %w", err)
	}

	objects, err := bucket.ListObjects(ctx, opts)
	if err != nil {
		return fmt.Errorf("failed to list objects: %w", err)
	}

	if len(objects) == 0 {
		fmt.Println("No objects found")
		return nil
	}

	fmt.Printf("Found %d object(s):\n", len(objects))
	fmt.Printf("%-40s %10s %20s %s\n", "Key", "Size", "Modified", "ETag")
	fmt.Println(strings.Repeat("-", 90))

	for _, obj := range objects {
		fmt.Printf("%-40s %10d %20s %s\n",
			truncateString(obj.Key, 40),
			obj.Size,
			obj.LastModified.Format("2006-01-02 15:04:05"),
			strings.Trim(obj.ETag, "\""))
	}

	return nil
}

// deleteObject deletes an object from S3
func (cli *CLI) deleteObject(args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("usage: delete-object <bucket-name> <object-key>")
	}

	bucketName := args[0]
	objectKey := args[1]
	ctx := context.Background()

	fmt.Printf("Deleting object 's3://%s/%s'...\n", bucketName, objectKey)

	bucket, err := cli.s3Client.Bucket(bucketName)
	if err != nil {
		return fmt.Errorf("failed to intialize bucket helper: %w", err)
	}

	err = bucket.DeleteObject(ctx, objectKey)
	if err != nil {
		return fmt.Errorf("failed to delete object: %w", err)
	}

	fmt.Printf("Object 's3://%s/%s' deleted successfully\n", bucketName, objectKey)
	return nil
}

// presignGet generates a presigned URL for downloading
func (cli *CLI) presignGet(args []string) error {
	if len(args) < 2 || len(args) > 3 {
		return fmt.Errorf("usage: presign-get <bucket-name> <object-key> [expiry-minutes]")
	}

	bucketName := args[0]
	objectKey := args[1]

	expiry := 60 * time.Minute // Default 1 hour
	if len(args) == 3 {
		minutes := 0
		if _, err := fmt.Sscanf(args[2], "%d", &minutes); err != nil {
			return fmt.Errorf("invalid expiry minutes: %v", err)
		}
		expiry = time.Duration(minutes) * time.Minute
	}

	ctx := context.Background()

	fmt.Printf("Generating presigned GET URL for 's3://%s/%s' (expires in %v)...\n",
		bucketName, objectKey, expiry)

	bucket, err := cli.s3Client.Bucket(bucketName)
	if err != nil {
		return fmt.Errorf("failed to intialize bucket helper: %w", err)
	}

	url, err := bucket.PresignGetObject(ctx, objectKey, expiry)
	if err != nil {
		return fmt.Errorf("failed to generate presigned URL: %w", err)
	}

	fmt.Printf("Presigned URL (valid for %v):\n%s\n", expiry, url)
	return nil
}

// presignPut generates a presigned URL for uploading
func (cli *CLI) presignPut(args []string) error {
	if len(args) < 2 || len(args) > 3 {
		return fmt.Errorf("usage: presign-put <bucket-name> <object-key> [expiry-minutes]")
	}

	bucketName := args[0]
	objectKey := args[1]

	expiry := 60 * time.Minute // Default 1 hour
	if len(args) == 3 {
		minutes := 0
		if _, err := fmt.Sscanf(args[2], "%d", &minutes); err != nil {
			return fmt.Errorf("invalid expiry minutes: %v", err)
		}
		expiry = time.Duration(minutes) * time.Minute
	}

	ctx := context.Background()

	fmt.Printf("Generating presigned PUT URL for 's3://%s/%s' (expires in %v)...\n",
		bucketName, objectKey, expiry)

	bucket, err := cli.s3Client.Bucket(bucketName)
	if err != nil {
		return fmt.Errorf("failed to intialize bucket helper: %w", err)
	}

	url, err := bucket.PresignPutObject(ctx, objectKey, expiry)
	if err != nil {
		return fmt.Errorf("failed to generate presigned URL: %w", err)
	}

	fmt.Printf("Presigned URL (valid for %v):\n%s\n", expiry, url)
	fmt.Println("\nExample usage:")
	fmt.Printf("curl -X PUT --upload-file <local-file> \"%s\"\n", url)
	return nil
}

// showHelp displays help information
func (cli *CLI) showHelp(args []string) error {
	if len(args) == 0 {
		// General help
		fmt.Println("S3 Client CLI - Available Commands:")
		fmt.Println()
		for _, cmd := range commands {
			fmt.Printf("  %-15s %s\n", cmd.Name, cmd.Description)
		}
		fmt.Println()
		fmt.Println("Use 'help <command>' for detailed usage of a specific command")
		return nil
	}

	// Specific command help
	commandName := args[0]
	for _, cmd := range commands {
		if cmd.Name == commandName {
			fmt.Printf("Command: %s\n", cmd.Name)
			fmt.Printf("Description: %s\n", cmd.Description)
			fmt.Printf("Usage: %s\n", cmd.Usage)
			return nil
		}
	}

	return fmt.Errorf("unknown command: %s", commandName)
}

// Helper functions

// detectContentType attempts to detect content type from file extension
func detectContentType(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".txt":
		return "text/plain"
	case ".json":
		return "application/json"
	case ".xml":
		return "application/xml"
	case ".html", ".htm":
		return "text/html"
	case ".css":
		return "text/css"
	case ".js":
		return "application/javascript"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".pdf":
		return "application/pdf"
	case ".zip":
		return "application/zip"
	case ".tar":
		return "application/x-tar"
	case ".gz":
		return "application/gzip"
	default:
		return "application/octet-stream"
	}
}

// truncateString truncates a string to maxLen characters
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// formatBytes formats bytes in a human-readable format
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// calculateTransferSpeed calculates transfer speed in bytes per second
func calculateTransferSpeed(bytes int64, elapsed time.Duration) float64 {
	if elapsed == 0 {
		return 0
	}
	return float64(bytes) / elapsed.Seconds()
}
