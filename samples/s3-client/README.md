# S3 Client CLI Sample

This sample application demonstrates how to use the Blueprint S3 provider to interact with S3-compatible storage services like MinIO, AWS S3, or DigitalOcean Spaces.

## Features

The CLI tool provides the following functionality:

### Bucket Operations
- **create-bucket** - Create a new S3 bucket
- **list-buckets** - List all available buckets
- **delete-bucket** - Delete an empty bucket

### Object Operations
- **upload** - Upload files to S3 with automatic content type detection
- **download** - Download files from S3
- **list-objects** - List objects in a bucket with optional prefix filtering
- **delete-object** - Delete objects from S3

### Advanced Features
- **presign-get** - Generate presigned URLs for secure downloading
- **presign-put** - Generate presigned URLs for secure uploading
- **SSL/TLS support** - Secure connections to S3 endpoints
- **Verbose logging** - Detailed operation logging
- **Content type detection** - Automatic MIME type detection for uploads
- **Upload/Download timing** - Displays elapsed time and transfer speed for operations
- **Large file timeout handling** - Configurable timeouts for large file operations

## Quick Start with MinIO

### 1. Start MinIO Server

#### Option A: Using Docker Compose (Recommended)
```bash
# Start MinIO with demo bucket setup
docker-compose up -d

# Check status
docker-compose ps

# View logs
docker-compose logs -f minio
```

#### Option B: Using Docker directly
```bash
docker run -p 9000:9000 -p 9001:9001 --name minio \
  -e MINIO_ROOT_USER=minioadmin \
  -e MINIO_ROOT_PASSWORD=minioadmin \
  quay.io/minio/minio server /data --console-address :9001
```

#### Option C: Using MinIO binary
```bash
minio server /tmp/minio-data --console-address :9001
```

MinIO Console will be available at: http://localhost:9011 (docker-compose) or http://localhost:9001 (direct docker)

### 2. Build the CLI Tool

```bash
cd samples/s3-client
go build -o s3-cli main.go
```

### 3. Basic Usage Examples

```bash
# List available commands
./s3-cli help

# Create a bucket
./s3-cli create-bucket my-test-bucket

# List all buckets
./s3-cli list-buckets

# Upload a file
echo "Hello, S3!" > test.txt
./s3-cli upload my-test-bucket test.txt

# List objects in bucket
./s3-cli list-objects my-test-bucket

# Download the file
./s3-cli download my-test-bucket test.txt downloaded-test.txt

# Generate presigned URL for sharing
./s3-cli presign-get my-test-bucket test.txt 30

# Clean up
./s3-cli delete-object my-test-bucket test.txt
./s3-cli delete-bucket my-test-bucket
```

## Configuration Options

### Command Line Flags

- `--endpoint` - S3 endpoint URL (default: localhost:9000, use localhost:9010 for docker-compose)
- `--region` - AWS region (default: us-east-1)
- `--access-key` - S3 access key (default: minioadmin)
- `--secret-key` - S3 secret key (default: minioadmin)
- `--ssl` - Enable SSL/TLS (default: false for localhost)
- `--verbose` - Enable verbose logging
- `--upload-timeout` - Upload timeout in seconds (default: 1800 for large files)
- `--version` - Show version information

#### Connection Pooling Options
- `--max-idle-conns` - Maximum idle connections in pool (default: 50)
- `--max-idle-conns-per-host` - Maximum idle connections per host (default: 10)
- `--max-conns-per-host` - Maximum connections per host (default: 20)
- `--idle-conn-timeout` - Idle connection timeout in seconds (default: 60)

### Environment Variables

For production use, set credentials via environment variables:

```bash
export S3_SECRET_KEY="your-secret-key"
./s3-cli --access-key "your-access-key" create-bucket production-bucket
```

## Usage Examples

### Working with Different S3 Services

#### AWS S3
```bash
./s3-cli --endpoint s3.amazonaws.com --region us-west-2 --ssl \
  --access-key YOUR_ACCESS_KEY --secret-key YOUR_SECRET_KEY \
  list-buckets
```

#### DigitalOcean Spaces
```bash
./s3-cli --endpoint nyc3.digitaloceanspaces.com --region us-east-1 --ssl \
  --access-key YOUR_SPACES_KEY --secret-key YOUR_SPACES_SECRET \
  list-buckets
```

#### Backblaze B2 (S3-compatible API)
```bash
./s3-cli --endpoint s3.us-west-002.backblazeb2.com --region us-west-002 --ssl \
  --access-key YOUR_KEY_ID --secret-key YOUR_APPLICATION_KEY \
  list-buckets
```

### Advanced Operations

#### Upload with Verbose Logging and Timing
```bash
./s3-cli --verbose upload my-bucket large-file.zip backup/large-file.zip
# Output: File uploaded successfully to 's3://my-bucket/backup/large-file.zip' (elapsed: 2.4s, speed: 41.7 MB/s)
```

#### Upload Large Files with Custom Timeout
```bash
./s3-cli --upload-timeout 3600 upload my-bucket very-large-file.bin
# Sets 1-hour timeout for large file uploads
```

#### List Objects with Prefix Filter
```bash
./s3-cli list-objects my-bucket images/
```

#### Generate Presigned URL for 24 Hours
```bash
./s3-cli presign-get my-bucket important-doc.pdf 1440
```

#### Upload Using Presigned URL
```bash
# First generate the presigned PUT URL
PRESIGN_URL=$(./s3-cli presign-put my-bucket new-file.txt 60)

# Then use curl to upload
curl -X PUT --upload-file local-file.txt "$PRESIGN_URL"
```

## Security Features

This sample demonstrates several security best practices:

1. **Secure Credential Handling** - Uses environment variables for secrets
2. **SSL/TLS Support** - Enables encrypted connections to S3 services  
3. **Input Validation** - Validates bucket names and object keys
4. **Audit Logging** - Logs all operations for security tracking
5. **Memory Clearing** - Securely clears credentials from memory
6. **Presigned URLs** - Enables secure temporary access without exposing credentials

## Error Handling

The CLI provides clear error messages for common issues:

- Invalid bucket names or object keys
- Network connectivity problems
- Authentication failures
- Missing files or objects
- Insufficient permissions

## Performance Features

The underlying S3 provider includes:

- **Multipart Uploads** - Automatic for files > 100MB with 10MB part size
- **Connection Pooling** - Configurable HTTP connection reuse with customizable pool sizes
- **Retry Logic** - Automatic retry with exponential backoff
- **Concurrent Operations** - Parallel part uploads for large files
- **Smart Timeouts** - Configurable timeouts (5 min default, 30 min for large uploads)
- **Transfer Speed Monitoring** - Real-time speed calculation and display

### Connection Pooling Configuration

Optimize performance by tuning connection pooling parameters:

```bash
# High-performance configuration for busy workloads
./s3-cli --max-idle-conns 100 --max-idle-conns-per-host 20 \
  --max-conns-per-host 50 --idle-conn-timeout 300 \
  --verbose upload my-bucket large-file.zip

# Conservative configuration for low-resource environments
./s3-cli --max-idle-conns 10 --max-idle-conns-per-host 2 \
  --max-conns-per-host 5 --idle-conn-timeout 30 \
  upload my-bucket small-file.txt
```

**Connection Pool Guidelines:**
- `max-idle-conns`: Total connections across all hosts (50-200 for busy workloads)
- `max-idle-conns-per-host`: Per-host idle connections (5-20 typical)
- `max-conns-per-host`: Per-host total connections (10-100 typical)
- `idle-conn-timeout`: How long to keep idle connections (30-300 seconds)

## Troubleshooting

### Common Issues

1. **Connection Refused**
   - Ensure MinIO server is running
   - Check endpoint URL and port

2. **Access Denied**
   - Verify access key and secret key
   - Check bucket permissions

3. **SSL Certificate Errors**
   - For local MinIO, use `--ssl=false`
   - For production, ensure valid SSL certificates

4. **Bucket Already Exists**
   - Bucket names must be globally unique
   - Choose a different bucket name

5. **Large File Upload Timeouts**
   - Use `--upload-timeout` flag for large files
   - Default timeout is 30 minutes for large uploads
   - Monitor transfer progress with verbose mode

### Enable Verbose Mode

Use the `--verbose` flag to see detailed operation logs:

```bash
./s3-cli --verbose --endpoint localhost:9000 create-bucket debug-bucket
```

This will show:
- Connection establishment
- Connection pooling configuration
- Request/response details
- Security events
- Performance metrics
- Upload/download timing and transfer speeds

### Docker Compose Management

```bash
# Start MinIO service
docker-compose up -d

# Stop MinIO service
docker-compose down

# Stop and remove volumes (deletes all data)
docker-compose down -v

# View logs
docker-compose logs -f minio

# Restart MinIO
docker-compose restart minio
```

## Development

To extend this sample:

1. Add new commands to the `commands` slice
2. Implement handler functions following the existing pattern
3. Use the Blueprint S3 provider's additional features:
   - Server-side encryption
   - Custom metadata
   - Object tagging
   - Copy operations

## Integration Testing

This sample can be used for integration testing of the S3 provider:

```bash
# Start MinIO using docker-compose
docker-compose up -d

# Wait for MinIO to be ready
sleep 10

# Run comprehensive test
./test-s3-operations.sh

# Clean up
docker-compose down
```

See the test script for automated validation of all operations.

### Performance Testing with Connection Pooling

```bash
# Test with default settings
time ./s3-cli upload demo-bucket large-file.zip

# Test with optimized connection pooling
time ./s3-cli --max-idle-conns 100 --max-conns-per-host 50 \
  upload demo-bucket large-file.zip

# Compare upload speeds and connection reuse efficiency
```