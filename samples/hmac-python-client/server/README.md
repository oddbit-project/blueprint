# HMAC Python Client Demo Server

A demonstration Go server built with the Blueprint framework that showcases HMAC authentication for testing the Python HMAC client library.

## Features

- **Blueprint Framework Integration**: Uses Blueprint's HMAC provider for authentication
- **Memory-Based Nonce Store**: In-memory nonce storage for demo purposes
- **Multiple Endpoint Types**: Public and protected endpoints for comprehensive testing
- **CORS Support**: Enabled for cross-origin requests during development
- **Structured Logging**: Uses Blueprint's logging system
- **Error Handling**: Proper error responses and validation

## Quick Start

### Prerequisites

- Go 1.23 or later
- Blueprint framework (included via go.mod replace directive)

### Running the Server

```bash
# Navigate to server directory
cd server

# Install dependencies
go mod tidy

# Run the server
go run main.go
```

The server will start on `http://localhost:8080` and display available endpoints.

## API Endpoints

### Public Endpoints (No Authentication Required)

#### Health Check
- **GET** `/api/public/health`
- Returns server health status and timestamp
- Used by Python client to verify server availability

```json
{
  "status": "healthy",
  "timestamp": "2024-01-15T10:30:00Z",
  "service": "HMAC Python Client Demo Server"
}
```

#### Service Information
- **GET** `/api/public/info`
- Returns service information and available endpoints

```json
{
  "service": "HMAC Python Client Demo Server",
  "version": "1.0.0",
  "description": "Demo server for testing Python HMAC client",
  "endpoints": {
    "public": ["/api/public/health", "/api/public/info"],
    "protected": ["/api/protected/profile", "/api/protected/data"]
  }
}
```

#### HMAC Signing Service
- **POST** `/api/public/sign`
- Demonstrates Go HMAC signature generation for compatibility testing

```bash
curl -X POST http://localhost:8080/api/public/sign \
  -H "Content-Type: application/json" \
  -d '{"data": "test message"}'
```

### Protected Endpoints (HMAC Authentication Required)

All protected endpoints require valid HMAC authentication headers:
- `X-Hmac-Hash`: HMAC-SHA256 signature
- `X-Hmac-Timestamp`: ISO 8601 timestamp
- `X-Hmac-Nonce`: Unique request identifier

#### User Profile
- **GET** `/api/protected/profile`
- Returns mock user profile data

```json
{
  "user_id": "python-client-user",
  "username": "python_tester",
  "email": "python@example.com",
  "message": "Profile accessed successfully via HMAC auth"
}
```

#### Data Operations
- **POST** `/api/protected/data`
- Accepts JSON payload and returns processed data

```bash
# Using Python client
response = client.post("/api/protected/data", json={
    "message": "Hello from Python!",
    "type": "test"
})
```

#### Settings Management
- **PUT** `/api/protected/settings`
- Updates user settings and returns confirmation

#### Resource Management
- **DELETE** `/api/protected/resource/{id}`
- Deletes specified resource and returns confirmation

#### Echo Service
- **POST** `/api/protected/echo`
- Returns request body and headers for debugging

### Test Endpoints (HMAC Authentication Required)

#### Simple Test
- **GET** `/api/test/simple`
- Basic test endpoint for connection verification

#### JSON Test
- **POST** `/api/test/json`
- Tests JSON payload handling and returns received data

#### Large Data Test
- **POST** `/api/test/large`
- Tests handling of larger payloads (up to configured limits)

## Configuration

### HMAC Configuration

```go
const (
    SecretKey = "python-client-demo-secret"  // Shared secret
    MaxNonces = 10000                        // Memory store capacity
    NonceTTL  = 300 * time.Second           // 5 minutes
)
```

### Server Configuration

```go
const (
    ServerPort = ":8080"                     // Listen port
    TimeWindow = 300 * time.Second          // Timestamp tolerance
)
```

## Security Features

### HMAC Authentication
- **Secret Key**: Shared between server and client
- **Message Format**: `{timestamp}:{nonce}:{request_body}`
- **Hash Algorithm**: HMAC-SHA256
- **Encoding**: Lowercase hexadecimal

### Replay Protection
- **Nonce Store**: Memory-based with automatic cleanup
- **Timestamp Validation**: 5-minute window (configurable)
- **Unique Nonces**: Rejects duplicate nonces within TTL period

### Input Validation
- **Size Limits**: Configurable maximum request size
- **Format Validation**: Validates timestamp and nonce formats
- **Error Handling**: Secure error messages without information leakage

## Development

### Project Structure

```
server/
├── main.go              # Server implementation
├── go.mod               # Go module definition
└── README.md            # This file
```

### Blueprint Integration

The server demonstrates proper Blueprint framework usage:

```go
// HMAC Provider Setup
memStore := memory.NewMemoryStore(MaxNonces, NonceTTL)
hmacProvider := hmacprovider.NewContainer(SecretKey, memStore)

// Middleware Configuration
auth := middleware.NewAuth(log.Logger)
protected := router.Group("/api/protected")
protected.Use(auth.AuthMiddleware(auth.HMACAuth(hmacProvider)))
```

### Testing with Python Client

1. **Start the server**:
   ```bash
   go run main.go
   ```

2. **Run Python client tests**:
   ```bash
   cd ..
   pipenv run pytest tests/test_integration.py -v
   ```

3. **Run Python examples**:
   ```bash
   pipenv run python examples/basic_usage.py
   ```

## Logging

The server uses Blueprint's structured logging:

```
INFO[2024-01-15T10:30:00Z] Starting HMAC Python Client Demo Server port=:8080
INFO[2024-01-15T10:30:00Z] HMAC provider configured secret_length=26
INFO[2024-01-15T10:30:00Z] Memory nonce store initialized capacity=10000 ttl=5m0s
INFO[2024-01-15T10:30:05Z] HMAC authentication successful endpoint="/api/protected/profile" nonce="550e8400-e29b-41d4-a716-446655440000"
```

## Error Responses

### Authentication Failures

```json
{
  "error": "Unauthorized",
  "message": "HMAC authentication failed",
  "status": 401
}
```

### Validation Errors

```json
{
  "error": "Bad Request", 
  "message": "Invalid request format",
  "status": 400
}
```

### Server Errors

```json
{
  "error": "Internal Server Error",
  "message": "Server error occurred",
  "status": 500
}
```

## Production Considerations

This is a **demonstration server** for testing purposes. For production use:

1. **Persistent Storage**: Replace memory store with Redis or database
2. **Configuration**: Use environment variables or config files
3. **TLS/HTTPS**: Enable secure transport
4. **Rate Limiting**: Add request rate limiting
5. **Monitoring**: Add metrics and health checks
6. **Secret Management**: Use secure secret storage

## Troubleshooting

### Common Issues

1. **Authentication Failed (401)**
   - Verify secret key matches between client and server
   - Check timestamp is within tolerance window
   - Ensure nonce is unique

2. **Connection Refused**
   - Verify server is running on correct port
   - Check firewall settings
   - Ensure no port conflicts

3. **Invalid Signature**
   - Verify request body encoding (UTF-8)
   - Check message format: `{timestamp}:{nonce}:{body}`
   - Ensure hash is lowercase hexadecimal

### Debug Mode

Enable debug logging for troubleshooting:

```go
// Add debug logging in main.go
log.Logger.Debug().
    Str("method", method).
    Str("path", path).
    Str("hash", hash).
    Msg("HMAC verification details")
```

## License

This demonstration server is part of the Blueprint framework samples and follows the same license terms.