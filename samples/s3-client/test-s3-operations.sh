#!/bin/bash

# S3 Client CLI Integration Test Script
# This script tests all major operations of the S3 CLI against a MinIO server

set -e  # Exit on any error

# Configuration
CLI_BINARY="./s3-cli"
CLI_ENDPOINT="--endpoint localhost:9010"  # Use docker-compose MinIO port
TEST_BUCKET="test-cli-bucket-$(date +%s)"
TEST_FILE="test-file.txt"
TEST_CONTENT="Hello, S3 CLI! This is a test file created at $(date)"
DOWNLOADED_FILE="downloaded-test.txt"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Test functions
test_command() {
    local cmd="$1"
    local description="$2"
    
    log_info "Testing: $description"
    echo "Command: $cmd"
    
    if eval "$cmd"; then
        log_success "$description - PASSED"
        echo ""
        return 0
    else
        log_error "$description - FAILED"
        return 1
    fi
}

cleanup() {
    log_info "Cleaning up test files and buckets..."
    
    # Remove local test files
    [ -f "$TEST_FILE" ] && rm -f "$TEST_FILE"
    [ -f "$DOWNLOADED_FILE" ] && rm -f "$DOWNLOADED_FILE"
    
    # Try to clean up S3 resources (may fail if already deleted)
    $CLI_BINARY $CLI_ENDPOINT delete-object "$TEST_BUCKET" "$TEST_FILE" 2>/dev/null || true
    $CLI_BINARY $CLI_ENDPOINT delete-bucket "$TEST_BUCKET" 2>/dev/null || true
    
    log_info "Cleanup completed"
}

# Set up cleanup trap
trap cleanup EXIT

# Main test function
run_tests() {
    log_info "Starting S3 CLI Integration Tests"
    log_info "Test bucket: $TEST_BUCKET"
    echo ""
    
    # Check if CLI binary exists
    if [ ! -f "$CLI_BINARY" ]; then
        log_error "CLI binary not found: $CLI_BINARY"
        log_info "Please build the binary first: go build -o s3-cli main.go"
        exit 1
    fi
    
    # Check if MinIO is running
    if ! curl -s http://localhost:9010/minio/health/live >/dev/null 2>&1; then
        log_warning "MinIO server may not be running at localhost:9010"
        log_info "Start MinIO with: docker-compose up -d (or docker run -p 9010:9000 -p 9011:9001 --name minio quay.io/minio/minio server /data --console-address :9001)"
        echo ""
    fi
    
    # Create test file
    echo "$TEST_CONTENT" > "$TEST_FILE"
    log_info "Created test file: $TEST_FILE"
    echo ""
    
    # Test 1: Show help
    test_command "$CLI_BINARY help" "Show help command"
    
    # Test 2: List buckets (initial state)
    test_command "$CLI_BINARY $CLI_ENDPOINT list-buckets" "List buckets (initial)"
    
    # Test 3: Create bucket
    test_command "$CLI_BINARY $CLI_ENDPOINT create-bucket $TEST_BUCKET" "Create bucket"
    
    # Test 4: List buckets (should include new bucket)
    test_command "$CLI_BINARY $CLI_ENDPOINT list-buckets" "List buckets (after creation)"
    
    # Test 5: Upload file
    test_command "$CLI_BINARY $CLI_ENDPOINT upload $TEST_BUCKET $TEST_FILE" "Upload file"
    
    # Test 6: List objects in bucket
    test_command "$CLI_BINARY $CLI_ENDPOINT list-objects $TEST_BUCKET" "List objects in bucket"
    
    # Test 7: Download file
    test_command "$CLI_BINARY $CLI_ENDPOINT download $TEST_BUCKET $TEST_FILE $DOWNLOADED_FILE" "Download file"
    
    # Test 8: Verify downloaded content
    if [ -f "$DOWNLOADED_FILE" ]; then
        DOWNLOADED_CONTENT=$(cat "$DOWNLOADED_FILE")
        if [ "$DOWNLOADED_CONTENT" = "$TEST_CONTENT" ]; then
            log_success "File content verification - PASSED"
        else
            log_error "File content verification - FAILED"
            echo "Expected: $TEST_CONTENT"
            echo "Got: $DOWNLOADED_CONTENT"
        fi
    else
        log_error "Downloaded file not found"
    fi
    echo ""
    
    # Test 9: Generate presigned GET URL
    test_command "$CLI_BINARY $CLI_ENDPOINT presign-get $TEST_BUCKET $TEST_FILE 5" "Generate presigned GET URL"
    
    # Test 10: Generate presigned PUT URL
    test_command "$CLI_BINARY $CLI_ENDPOINT presign-put $TEST_BUCKET new-file.txt 5" "Generate presigned PUT URL"
    
    # Test 11: Upload with custom key
    test_command "$CLI_BINARY $CLI_ENDPOINT upload $TEST_BUCKET $TEST_FILE custom/path/test.txt" "Upload with custom key"
    
    # Test 12: List objects with prefix
    test_command "$CLI_BINARY $CLI_ENDPOINT list-objects $TEST_BUCKET custom/" "List objects with prefix"
    
    # Test 13: Delete specific object
    test_command "$CLI_BINARY $CLI_ENDPOINT delete-object $TEST_BUCKET custom/path/test.txt" "Delete object with custom path"
    
    # Test 14: Delete original object
    test_command "$CLI_BINARY $CLI_ENDPOINT delete-object $TEST_BUCKET $TEST_FILE" "Delete original object"
    
    # Test 15: List objects (should be empty)
    test_command "$CLI_BINARY $CLI_ENDPOINT list-objects $TEST_BUCKET" "List objects (should be empty)"
    
    # Test 16: Delete bucket
    test_command "$CLI_BINARY $CLI_ENDPOINT delete-bucket $TEST_BUCKET" "Delete bucket"
    
    # Test 17: List buckets (final state)
    test_command "$CLI_BINARY $CLI_ENDPOINT list-buckets" "List buckets (final)"
    
    log_success "All tests completed successfully!"
}

# Advanced tests function
run_advanced_tests() {
    log_info "Running advanced tests..."
    
    # Test with verbose mode
    test_command "$CLI_BINARY $CLI_ENDPOINT --verbose create-bucket verbose-test-bucket" "Verbose mode test"
    test_command "$CLI_BINARY $CLI_ENDPOINT delete-bucket verbose-test-bucket" "Cleanup verbose test bucket"
    
    # Test with different file types
    echo '{"test": "json content"}' > test.json
    test_command "$CLI_BINARY $CLI_ENDPOINT create-bucket file-type-test" "Create bucket for file type test"
    test_command "$CLI_BINARY $CLI_ENDPOINT upload file-type-test test.json" "Upload JSON file"
    test_command "$CLI_BINARY $CLI_ENDPOINT list-objects file-type-test" "List JSON file"
    test_command "$CLI_BINARY $CLI_ENDPOINT delete-object file-type-test test.json" "Delete JSON file"
    test_command "$CLI_BINARY $CLI_ENDPOINT delete-bucket file-type-test" "Delete file type test bucket"
    rm -f test.json
    
    log_success "Advanced tests completed!"
}

# Performance test function
run_performance_tests() {
    log_info "Running performance tests..."
    
    # Create a larger test file (1MB)
    dd if=/dev/zero of=large-test.bin bs=1024 count=1024 2>/dev/null
    
    test_command "$CLI_BINARY $CLI_ENDPOINT create-bucket perf-test-bucket" "Create performance test bucket"
    
    # Time the upload
    log_info "Timing large file upload (1MB)..."
    time $CLI_BINARY $CLI_ENDPOINT upload perf-test-bucket large-test.bin
    
    # Time the download
    log_info "Timing large file download (1MB)..."
    time $CLI_BINARY $CLI_ENDPOINT download perf-test-bucket large-test.bin downloaded-large.bin
    
    # Cleanup
    test_command "$CLI_BINARY $CLI_ENDPOINT delete-object perf-test-bucket large-test.bin" "Delete large file"
    test_command "$CLI_BINARY $CLI_ENDPOINT delete-bucket perf-test-bucket" "Delete performance test bucket"
    
    rm -f large-test.bin downloaded-large.bin
    
    log_success "Performance tests completed!"
}

# Error handling test function
run_error_tests() {
    log_info "Running error handling tests..."
    
    # Test invalid bucket name
    if $CLI_BINARY $CLI_ENDPOINT create-bucket "Invalid Bucket Name" 2>/dev/null; then
        log_error "Invalid bucket name test - FAILED (should have failed)"
    else
        log_success "Invalid bucket name test - PASSED (correctly failed)"
    fi
    
    # Test non-existent bucket operations
    if $CLI_BINARY $CLI_ENDPOINT list-objects non-existent-bucket 2>/dev/null; then
        log_error "Non-existent bucket test - FAILED (should have failed)"
    else
        log_success "Non-existent bucket test - PASSED (correctly failed)"
    fi
    
    # Test missing file upload
    if $CLI_BINARY $CLI_ENDPOINT upload some-bucket non-existent-file.txt 2>/dev/null; then
        log_error "Missing file test - FAILED (should have failed)"
    else
        log_success "Missing file test - PASSED (correctly failed)"
    fi
    
    log_success "Error handling tests completed!"
}

# Main execution
echo "================================================="
echo "         S3 CLI Integration Test Suite"
echo "================================================="
echo ""

case "${1:-basic}" in
    "basic")
        run_tests
        ;;
    "advanced") 
        run_tests
        run_advanced_tests
        ;;
    "performance")
        run_tests
        run_performance_tests
        ;;
    "errors")
        run_error_tests
        ;;
    "all")
        run_tests
        run_advanced_tests
        run_performance_tests
        run_error_tests
        ;;
    *)
        echo "Usage: $0 [basic|advanced|performance|errors|all]"
        echo ""
        echo "Test suites:"
        echo "  basic       - Basic CRUD operations (default)"
        echo "  advanced    - Advanced features and edge cases"
        echo "  performance - Upload/download timing tests"
        echo "  errors      - Error handling validation"
        echo "  all         - Run all test suites"
        exit 1
        ;;
esac

echo ""
echo "================================================="
echo "              Test Suite Complete"
echo "================================================="