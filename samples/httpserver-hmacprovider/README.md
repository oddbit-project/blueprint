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
- **Client Examples**: Complete client implementation with examples

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
                       │  HMAC Provider   │
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
- **Secure Headers**: Security headers added to all responses

### Authentication Flow

1. Client generates HMAC signature for request body
2. Client includes signature, timestamp, and nonce in headers
3. Server validates timestamp within allowed window
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

- `GET /api/public/health` - Health check
- `GET /api/public/info` - Service information
- `POST /api/public/sign` - Sign data for testing

**Protected Endpoints (HMAC Authentication Required):**

- `GET /api/protected/profile` - User profile
- `POST /api/protected/data` - Submit data
- `PUT /api/protected/settings` - Update settings
- `DELETE /api/protected/resource/:id` - Delete resource

**Admin Endpoints (HMAC Authentication Required):**

- `GET /api/admin/stats` - Server statistics
- `POST /api/admin/maintenance` - Maintenance operations
- `GET /api/admin/logs` - Server logs

### 2. Test with Client Examples

```bash
# Run all client examples
go run client.go examples

# Run performance test
go run client.go performance

# Make single authenticated request
go run client.go request GET /api/protected/profile

# Make POST request with data
go run client.go request POST /api/protected/data '{"message":"test","type":"example"}'
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
ServerPort = ":8080"           // Server listen port
HMACSecret = "your-secret-key" // HMAC secret key
RequestTimeout = 30 * time.Second // Request timeout
HMACKeyInterval = 5 * time.Minute // ±5 minutes for clock drift
HMACMaxInput = 10 * 1024 * 1024 // 10MB max request size
NonceStoreTTL = 1 * time.Hour // Nonce TTL
)
```

### Environment Variables

For production use, set the HMAC secret via environment variable:

```bash
export HMAC_SECRET="your-production-secret-key"
go run main.go middleware.go
```

### Nonce Store Configuration

The example uses an in-memory nonce store. For production, consider:

```go
// Redis store for distributed systems
redisClient, _ := redis.NewRedisProvider(&redis.RedisConfig{
Host: "localhost:6379",
})
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
resp, err := client.Post("/api/protected/data", map[string]string{
"message": "Hello, World!",
})
```

### Manual Request Signing

```go
// Create HMAC provider
provider := hmacprovider.NewHmacProvider(credential)

// Sign request body
body := []byte(`{"message":"test"}`)
hash, timestamp, nonce, err := provider.Sign256(bytes.NewReader(body))

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

## Security Considerations

### Production Deployment

1. **Secret Management**: Use secure secret storage (not hardcoded)
2. **HTTPS Only**: Deploy with TLS/SSL certificates
3. **Rate Limiting**: Implement rate limiting for API endpoints
4. **Log Security**: Monitor authentication failures and suspicious patterns
5. **Clock Synchronization**: Ensure server clocks are synchronized (NTP)
6. **Nonce Store Scaling**: Use Redis for distributed deployments

### Monitoring and Alerting

Monitor these security metrics:

- Authentication failure rates
- Replay attack attempts
- Timestamp validation failures
- Excessive request rates from single IPs
- Nonce store capacity and performance

## Testing

### Unit Tests

```bash
# Test HMAC provider
go test ./provider/hmacprovider/...

# Test middleware
go test -v .
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

# Using external tools
hey -n 1000 -c 10 -H "X-HMAC-Hash: signed_hash" \
    -H "X-HMAC-Timestamp: timestamp" \
    -H "X-HMAC-Nonce: unique_nonce" \
    http://localhost:8080/api/protected/profile
```

## Files

- `main.go` - HTTP server implementation with all endpoints
- `middleware.go` - HMAC authentication middleware and security headers
- `client.go` - Client implementation with examples and CLI
- `README.md` - This documentation

## Related Documentation

- [HMAC Provider Documentation](../../docs/provider/hmacprovider.md)
- [HTTP Server Framework](../../docs/provider/httpserver/index.md)
- [Security Best Practices](../../docs/provider/httpserver/security.md)

## License

This example is part of the Blueprint framework and follows the same license terms.