# HMAC Python Client Library

A Python client library for making HMAC-authenticated HTTP requests compatible with the Blueprint framework's HMAC provider.

## Features

- **HMAC-SHA256 Authentication**: Secure request signing using HMAC-SHA256
- **Replay Attack Protection**: Automatic nonce generation and timestamp validation
- **Cross-Language Compatibility**: Full compatibility with Blueprint Go HMAC provider
- **Simple API**: Easy-to-use methods for common HTTP operations
- **Comprehensive Error Handling**: Detailed error types for different failure scenarios
- **Input Validation**: Size limits and format validation for security
- **Context Manager Support**: Clean resource management with `with` statements

## Installation

```bash
# Install dependencies using pipenv
pipenv install

# Or install directly with pip
pip install requests
```

## Quick Start

```python
from hmac_client import HMACClient

# Create authenticated client
client = HMACClient("http://localhost:8080", "your-key-id", "your-secret-key")

# Make authenticated requests
response = client.get("/api/protected/data")
response = client.post("/api/protected/data", json={"message": "hello"})

# Clean up
client.close()
```

### Context Manager Usage (Recommended)

```python
from hmac_client import HMACClient

with HMACClient("http://localhost:8080", "your-key-id", "your-secret-key") as client:
    response = client.get("/api/protected/data")
    data = response.json()
    print(data)
# Client automatically closed
```

## API Reference

### HMACClient

#### Constructor

```python
HMACClient(
    base_url: str,
    key_id: str,
    secret_key: str,
    key_interval: int = 300,      # Timestamp tolerance in seconds
    max_input_size: int = 33554432,  # Max input size (32MB)
    timeout: int = 30             # HTTP timeout in seconds
)
```

#### HTTP Methods

```python
# Standard HTTP methods with automatic HMAC authentication
client.get(path, **kwargs)
client.post(path, data=None, json=None, **kwargs)
client.put(path, data=None, json=None, **kwargs)
client.delete(path, **kwargs)
```

#### HMAC Operations

```python
# Simple HMAC signing (no timestamp/nonce)
signature = client.sha256_sign(data: bytes) -> str
is_valid = client.sha256_verify(data: bytes, signature: str) -> bool

# Secure HMAC signing (with timestamp/nonce)
hash_value, timestamp, nonce = client.sign256(data: bytes) -> Tuple[str, str, str]
is_valid = client.verify256(data: bytes, hash_value: str, timestamp: str, nonce: str) -> bool
```

## Configuration

### Custom Configuration

```python
client = HMACClient(
    "http://localhost:8080",
    "your-key-id",
    "your-secret-key",
    key_interval=600,        # 10 minutes tolerance
    max_input_size=1048576,  # 1MB limit
    timeout=60               # 60 second timeout
)
```

## Error Handling

The library provides specific exception types:

```python
from hmac_client import (
    HMACClientError,         # Base exception
    ConfigurationError,      # Invalid configuration
    InputTooLargeError,      # Input exceeds size limit
    InvalidSignatureError,   # Signature verification failed
    HTTPError               # HTTP request failed
)

try:
    response = client.get("/api/protected/data")
except ConfigurationError as e:
    print(f"Configuration error: {e}")
except InputTooLargeError as e:
    print(f"Input too large: {e}")
except InvalidSignatureError as e:
    print(f"Invalid signature: {e}")
except HTTPError as e:
    print(f"HTTP error: {e}")
```

## Security Features

### Input Validation
- **Size Limits**: Configurable maximum input size to prevent memory exhaustion
- **Format Validation**: Validates timestamp and signature formats
- **Safe Defaults**: Conservative default settings for production use

### Cryptographic Security
- **HMAC-SHA256**: Industry-standard message authentication
- **Constant-Time Comparison**: Uses `hmac.compare_digest()` to prevent timing attacks
- **Secure Random**: Uses `uuid.uuid4()` for cryptographically secure nonce generation

### Replay Protection
- **Unique Nonces**: Every request gets a unique UUID v4 nonce
- **Timestamp Validation**: Configurable time window prevents replay of old requests
- **Server-Side Validation**: Compatible with Blueprint's nonce store implementations

## Examples

See the `examples/` directory for comprehensive usage examples:

- `basic_usage.py`: Complete demonstration of all features
- Run with: `python examples/basic_usage.py`

## Testing

### Unit Tests

```bash
# Run unit tests
pipenv run pytest tests/test_client.py -v

# Run with coverage
pipenv run pytest tests/test_client.py --cov=hmac_client
```

### Integration Tests

```bash
# Start the Go server first
cd server && go run main.go

# In another terminal, run integration tests
pipenv run pytest tests/test_integration.py -v
```

## Blueprint Go Server Integration

This client is designed to work with Blueprint framework servers using the HMAC provider:

```go
// Go server setup
hmacProvider := hmacprovider.NewContainer(keyProvider, store)
router.Use(auth.AuthMiddleware(auth.NewHMACAuthProvider(hmacProvider)))
```

The Python client generates signatures in the exact format expected by the Go server:

1. **Message Format**: `{timestamp}:{nonce}:{request_body}`
2. **Headers**: `X-Hmac-Hash`, `X-Hmac-Timestamp`, `X-Hmac-Nonce`
3. **Timestamp Format**: ISO 8601 with timezone
4. **Hash Format**: Lowercase hexadecimal

## Development

### Project Structure

```
hmac-python-client/
├── hmac_client/          # Client library source
│   ├── __init__.py       # Public API exports
│   ├── client.py         # Main client implementation
│   ├── constants.py      # Configuration constants
│   └── exceptions.py     # Custom exception types
├── tests/               # Test suite
│   ├── test_client.py   # Unit tests
│   └── test_integration.py  # Integration tests
├── examples/            # Usage examples
│   └── basic_usage.py   # Comprehensive examples
├── server/              # Go demonstration server
│   ├── main.go          # Server implementation
├── Pipfile              # Python dependencies
└── README.md            # This file
```

### Running the Demo

1. **Start the Go server**:
   ```bash
   cd server
   go run main.go
   ```

2. **Run the Python client examples**:
   ```bash
   pipenv install
   pipenv run python examples/basic_usage.py
   ```

3. **Run the test suite**:
   ```bash
   pipenv run pytest tests/ -v
   ```

## License

This project is part of the Blueprint framework samples and follows the same license terms.