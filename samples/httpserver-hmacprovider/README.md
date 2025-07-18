# HMAC HTTP Server Example

This example demonstrates how to build a secure HTTP server using the Blueprint HMAC provider for authentication.
The server implements HMAC-SHA256 signature verification with nonce-based replay protection.

## Features

- **HMAC-SHA256 Authentication**: Cryptographically secure request authentication
- **Replay Attack Prevention**: Nonce-based protection against replay attacks
- **Timing Attack Resistance**: Constant-time comparisons and early validation
- **DoS Protection**: Configurable input size limits and request timeouts
- **Comprehensive Logging**: Detailed security event logging
- **Multiple Endpoints**: Public, protected, and admin API endpoints
- **Client Examples**: Client implementation with examples
- **Performance Testing**: Built-in benchmarking and performance analysis

## Architecture

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│                 │    │                  │    │                 │
│   HTTP Client   ├────► Middleware Chain ├────►   API Handlers  │
│                 │    │                  │    │                 │
└─────────────────┘    └──────────────────┘    └─────────────────┘
                              │
                              ▼
                       ┌──────────────────┐
                       │                  │
                       │ HMAC Auth Provider│
                       │                  │
                       └──────────────────┘
                              │
                              ▼
                       ┌──────────────────┐
                       │                  │
                       │ HMAC Provider    │
                       │                  │
                       └──────────────────┘
                              │
                              ▼
                       ┌──────────────────┐
                       │                  │
                       │   Nonce Store    │
                       │   (Memory/KV)    │
                       │                  │
                       └──────────────────┘
```

## Security Features

### Implemented Protections

- **HMAC-SHA256 Signatures**: Cryptographically secure message authentication
- **Nonce-based Replay Protection**: Each request requires a unique nonce
- **Timestamp Validation**: Configurable time window for clock drift tolerance
- **Input Size Limits**: Protection against memory exhaustion attacks
- **Constant-time Comparisons**: Mitigation of timing attack vectors
- **Secure Key Storage**: Encrypted credential storage with memory protection
- **Graceful Error Handling**: Panic recovery and comprehensive error logging

### Authentication Flow

1. Client generates HMAC signature for request body using shared secret
2. Client includes signature, timestamp, and nonce in headers
3. Server validates timestamp within allowed window (±5 minutes)
4. Server verifies HMAC signature matches request body
5. Server checks nonce hasn't been used before (replay protection)
6. Server stores nonce to prevent future reuse
7. Request proceeds to handler if all validations pass

## Quick Start

### 1. Start the Server

```bash
cd /home/jpinheiro/oddbit/blueprint/samples/httpserver-hmacprovider
go run main.go middleware.go
```

The server will start on port 8080 with the following endpoints:

**Public Endpoints (No Authentication):**

- `GET /api/public/health` - Health check with server status
- `GET /api/public/info` - Service information and endpoint list
- `POST /api/public/sign` - Sign data for testing purposes

**Protected Endpoints (HMAC Authentication Required):**

- `GET /api/protected/profile` - User profile information
- `POST /api/protected/data` - Submit data with processing
- `PUT /api/protected/settings` - Update user settings
- `DELETE /api/protected/resource/:id` - Delete resource by ID

**Admin Endpoints (HMAC Authentication Required):**

- `GET /api/admin/stats` - Server statistics and HMAC configuration
- `POST /api/admin/maintenance` - Administrative operations
- `GET /api/admin/logs` - Server logs access

### 2. Test with Client Examples

```bash
# Run all client examples (9 comprehensive scenarios)
go run client.go examples

# Run performance test (concurrent requests)
go run client.go performance

# Make single authenticated request
go run client.go request GET /api/protected/profile

# Make POST request with JSON data
go run client.go request POST /api/protected/data '{"message":"test","type":"example"}'

# Make PUT request to update settings
go run client.go request PUT /api/protected/settings '{"theme":"dark","notifications":true}'

# Make DELETE request
go run client.go request DELETE /api/protected/resource/123
```

### 3. Test Public Endpoints

```bash
# Health check (no authentication)
curl http://localhost:8080/api/public/health

# Service info (no authentication)
curl http://localhost:8080/api/public/info

# Sign test data (no authentication)
curl -X POST http://localhost:8080/api/public/sign \
  -H "Content-Type: application/json" \
  -d '{"data":"test message"}'
```

## Configuration

### Server Configuration

The server can be configured by modifying constants in `main.go`:

```go
const (
    ServerPort     = ":8080"                  // Server listen port
    HMACSecret     = "your-hmac-secret-key"   // HMAC secret key
    RequestTimeout = 30 * time.Second         // Request timeout
    KeyId          = "myKey"                  // Key identifier
    
    // HMAC configuration
    HMACKeyInterval = 5 * time.Minute         // ±5 minutes for clock drift
    HMACMaxInput    = 10 * 1024 * 1024        // 10MB max request size
    NonceStoreTTL   = 1 * time.Hour           // Nonce TTL
)
```

### Environment Variables

For production use, set the HMAC secret via environment variable:

```bash
export HMAC_SECRET="your-production-secret-key-32-bytes-minimum"
go run main.go middleware.go
```

### Nonce Store Configuration

The example uses an in-memory nonce store with eviction policy:

```go
// Memory store configuration (current implementation)
nonceStore := store.NewMemoryNonceStore(
    store.WithTTL(NonceStoreTTL),
    store.WithMaxSize(100000),                    // 100k nonces max
    store.WithCleanupInterval(15*time.Minute),
    store.WithEvictPolicy(store.EvictHalfLife()), // Evict old nonces
)
```

For production distributed systems, consider:

```go
// Redis store for distributed systems
config := redis.NewConfig()
config.Address = "localhost:6379"
config.Database = 1

redisClient, _ := redis.NewClient(config)
redisStore := store.NewRedisStore(redisClient, 1*time.Hour, "hmac:")

// KV store for other backends
memKV := kv.NewMemoryKV()
kvStore := store.NewKvStore(memKV, 1*time.Hour)
```

## Client Implementation

### Basic Client Usage

```go
// Create authenticated client
client, err := NewHMACClient("http://localhost:8080", "your-secret-key")
if err != nil {
    log.Fatal(err)
}

// Make authenticated requests
resp, err := client.Get("/api/protected/profile")
if err != nil {
    log.Fatal(err)
}

// POST with JSON data
data := map[string]interface{}{
    "message": "Hello, World!",
    "type":    "example",
}
resp, err := client.Post("/api/protected/data", data)
```

### Manual Request Signing

```go
// Create HMAC provider with key provider
key, _ := secure.GenerateKey()
secret, _ := secure.NewCredential([]byte("your-secret"), key, false)
keyProvider := hmacprovider.NewSingleKeyProvider("myKey", secret)
provider := hmacprovider.NewHmacProvider(keyProvider)

// Sign request body
body := []byte(`{"message":"test"}`)
hash, timestamp, nonce, err := provider.Sign256("myKey", bytes.NewReader(body))
if err != nil {
    log.Fatal(err)
}

// Add headers to HTTP request
req.Header.Set("X-HMAC-Hash", hash)
req.Header.Set("X-HMAC-Timestamp", timestamp)
req.Header.Set("X-HMAC-Nonce", nonce)
```

## Required Headers

All protected endpoints require these headers:

| Header             | Description                 | Example                                |
|--------------------|-----------------------------|----------------------------------------|
| `X-HMAC-Hash`      | HMAC-SHA256 signature (hex) | `a1b2c3d4e5f6...`                      |
| `X-HMAC-Timestamp` | RFC3339 timestamp           | `2024-01-01T12:00:00Z`                 |
| `X-HMAC-Nonce`     | UUID nonce                  | `550e8400-e29b-41d4-a716-446655440000` |

## Client Examples

The client includes 9 comprehensive test scenarios:

1. **Public Endpoints**: Test health check and service info
2. **Protected Profile**: GET request with authentication
3. **Data Submission**: POST request with JSON payload
4. **Settings Update**: PUT request for configuration
5. **Resource Deletion**: DELETE request with path parameter
6. **Admin Access**: Administrative endpoint testing
7. **Data Signing**: Utility endpoint for signature generation
8. **Authentication Failure**: Test with wrong secret key
9. **Missing Headers**: Test server response to incomplete requests

## Security Considerations

### Production Deployment

1. **Secret Management**: Use secure secret storage (not hardcoded)
   ```bash
   # Use strong, randomly generated secrets
   openssl rand -base64 32
   ```

2. **HTTPS Only**: Deploy with TLS/SSL certificates
   ```go
   server := &http.Server{
       Addr:      ":8443",
       TLSConfig: &tls.Config{...},
   }
   ```

3. **Rate Limiting**: Implement rate limiting for API endpoints
   ```go
   router.Use(security.RateLimitMiddleware(rate.Every(time.Second), 10))
   ```

4. **Log Security**: Monitor authentication failures and suspicious patterns
5. **Clock Synchronization**: Ensure server clocks are synchronized (NTP)
6. **Nonce Store Scaling**: Use Redis for distributed deployments

### Monitoring and Alerting

Monitor these security metrics:

- Authentication failure rates
- Replay attack attempts (nonce reuse)
- Timestamp validation failures
- Excessive request rates from single IPs
- Nonce store capacity and performance
- HMAC signature verification latency

## Performance

### Benchmarks

The client includes a performance testing mode:

```bash
go run client.go performance
```

This runs:
- 100 concurrent requests to protected endpoints
- Measures authentication overhead
- Reports success/failure rates
- Analyzes response times

### Optimization Tips

1. **Reuse HMAC Provider**: Create once, use for multiple requests
2. **Connection Pooling**: Configure HTTP client with connection reuse
3. **Batch Operations**: Group multiple operations when possible
4. **Nonce Store Tuning**: Adjust cleanup intervals based on load

## Testing

### Unit Tests

```bash
# Test HMAC provider
go test ./provider/hmacprovider/...

# Test with race detection
go test -race ./provider/hmacprovider/...

# Benchmark HMAC operations
go test -bench=. ./provider/hmacprovider/...
```

### Integration Tests

```bash
# Start server in background
go run main.go middleware.go &
SERVER_PID=$!

# Run client tests
go run client.go examples

# Cleanup
kill $SERVER_PID
```

### Load Testing

```bash
# Using the built-in performance test
go run client.go performance

# Using external tools (requires manual signature generation)
hey -n 1000 -c 10 http://localhost:8080/api/public/health
```

## Error Handling

The implementation includes comprehensive error handling:

### Server-side Errors

- **Authentication Failures**: Logged with client IP and request details
- **Panic Recovery**: Graceful handling with stack traces
- **Validation Errors**: Detailed error messages without security leaks
- **Resource Errors**: Proper HTTP status codes and JSON responses

### Client-side Errors

- **Network Errors**: Retry logic and timeout handling
- **Authentication Errors**: Clear error messages and debugging info
- **JSON Parsing**: Validation and error reporting
- **HTTP Status**: Appropriate handling of different response codes

## Files

- `main.go` - HTTP server implementation with all endpoints (385 lines)
- `middleware.go` - Custom middleware for logging and error handling (65 lines)
- `client.go` - Client implementation with examples and CLI (358 lines)
- `README.md` - This documentation

## Related Documentation

- [HMAC Provider Documentation](../../docs/provider/hmacprovider.md)
- [HTTP Server Framework](../../docs/provider/httpserver/index.md)
- [Authentication & Authorization](../../docs/provider/httpserver/auth.md)
- [Security Best Practices](../../docs/provider/httpserver/security.md)

## License

This example is part of the Blueprint framework and follows the same license terms.