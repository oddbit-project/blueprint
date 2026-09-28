# Redis Provider

The Redis provider offers a robust Redis client implementation with connection management, secure credential handling,
TLS support, and key-value operations with TTL management.

## Features

- **Connection Management**: Automatic connection handling with health checking
- **Secure Credentials**: Support for password encryption and secure storage
- **TLS Support**: Optional TLS encryption for secure connections
- **Key Prefixing**: Automatic key prefixing for namespace isolation
- **TTL Management**: Configurable time-to-live for keys with custom TTL support
- **Context-Aware**: All operations support context for timeout and cancellation
- **KV Interface**: Compatible with Blueprint's key-value interface

## Installation

```bash
go get github.com/oddbit-project/blueprint/provider/redis
```

## Configuration

### Basic Configuration

```go
package main

import (
	"github.com/oddbit-project/blueprint/provider/redis"
)

func main() {
	// Create default configuration
	config := redis.NewConfig()
	config.Address = "localhost:6379"
	config.DB = 0
	config.Password = "your-redis-password"
	config.TTL = 3600 // 1 hour default TTL
	config.TimeoutSeconds = 10
	config.KeyPrefix = "myapp:"

	// Create client
	client, err := redis.NewClient(config)
	if err != nil {
		panic(err)
	}
	defer client.Close()
}
```

### JSON Configuration

```json
{
  "redis": {
    "address": "localhost:6379",
    "db": 0,
    "keyPrefix": "myapp:",
    "ttl": 3600,
    "timeoutSeconds": 10,
    "password": "your-redis-password"
  }
}
```

### Configuration with Secure Credentials

```go
config := redis.NewConfig()
config.Address = "redis.example.com:6379"
config.DefaultCredentialConfig = secure.DefaultCredentialConfig{
	PasswordEnvVar: "REDIS_PASSWORD",     // Read from environment
	PasswordFile:   "/secrets/redis_pwd", // Or read from file
}
```

### TLS Configuration

The TLS settings are a `tls.ClientConfig` embedded in the Redis `Config`; they are applied to the
connection only when `TLSEnable` is `true`.

```go
config := redis.NewConfig()
config.Address = "redis.example.com:6380"
config.ClientConfig = tls.ClientConfig{
	TLSEnable: true,
	TLSCA:     "/path/to/ca.crt",     // CA used to verify the server certificate
	TLSCert:   "/path/to/client.crt", // optional client certificate (mTLS)
	TLSKey:    "/path/to/client.key", // optional client key (mTLS)
}
```

## Configuration Options

| Field            | Type     | Default             | Description                       |
|------------------|----------|---------------------|-----------------------------------|
| `Address`        | `string` | `"localhost:6379"`  | Redis server address              |
| `DB`             | `int`    | `0`                 | Redis database number             |
| `KeyPrefix`      | `string` | `""`                | Prefix for all keys               |
| `TTL`            | `uint`   | `2592000` (30 days) | Default TTL in seconds            |
| `TimeoutSeconds` | `uint`   | `10`                | Operation timeout in seconds      |
| `Password`       | `string` | `""`                | Redis password                    |
| `PasswordEnvVar` | `string` | `""`                | Environment variable for password |
| `PasswordFile`   | `string` | `""`                | File path containing password     |
| `TLSEnable`      | `bool`   | `false`             | Enable TLS encryption             |
| `TLSCA`          | `string` | `""`                | CA certificate file used to verify the server |
| `TLSCert`        | `string` | `""`                | Client certificate file (mTLS)    |
| `TLSKey`         | `string` | `""`                | Client key file (mTLS)            |
| `TLSInsecureSkipVerify` | `bool` | `false`      | Skip server certificate verification (testing only) |

The JSON keys for the TLS fields are `tlsEnable`, `tlsCa`, `tlsCert`, `tlsKey`, `tlsInsecureSkipVerify`,
and `tlsKeyPassword`/`tlsKeyPasswordEnvVar`/`tlsKeyPasswordFile` for an encrypted client key.

## Usage Examples

### Basic Key-Value Operations

```go
package main

import (
	"fmt"
	"log"
	"github.com/oddbit-project/blueprint/provider/redis"
)

func main() {
	config := redis.NewConfig()
	config.Address = "localhost:6379"

	client, err := redis.NewClient(config)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	// Connect to Redis
	if err := client.Connect(); err != nil {
		log.Fatal("Failed to connect:", err)
	}

	// Set a value
	err = client.Set("user:123", []byte("John Doe"))
	if err != nil {
		log.Fatal("Failed to set:", err)
	}

	// Get a value
	data, err := client.Get("user:123")
	if err != nil {
		log.Fatal("Failed to get:", err)
	}

	if data != nil {
		fmt.Printf("Retrieved: %s\n", string(data))
	} else {
		fmt.Println("Key not found")
	}

	// Delete a key
	err = client.Delete("user:123")
	if err != nil {
		log.Fatal("Failed to delete:", err)
	}
}
```

### Custom TTL Operations

```go
import "time"

// Set with custom TTL (5 minutes)
err := client.SetTTL("session:abc123", []byte("session-data"), 5*time.Minute)
if err != nil {
	log.Fatal(err)
}

// Check default TTL
defaultTTL := client.TTL()
fmt.Printf("Default TTL: %v\n", defaultTTL)
```

### Key Prefixing

```go
config := redis.NewConfig()
config.KeyPrefix = "myapp:"
client, _ := redis.NewClient(config)

// This will store the key as "myapp:user:123"
client.Set("user:123", []byte("data"))

// You can also manually construct keys
key := client.Key("custom:key") // Returns "myapp:custom:key"
```

### Using as KV Backend

The Redis client implements Blueprint's `kv.KV` interface, and `kv.AtomicSetter` (`SetNX`), so it can
be passed to any component that takes a `kv.KV` backend:

```go
import (
	"time"

	"github.com/oddbit-project/blueprint/provider/hmacprovider/store"
	"github.com/oddbit-project/blueprint/provider/kv"
	"github.com/oddbit-project/blueprint/provider/redis"
)

client, err := redis.NewClient(redis.NewConfig())
if err != nil {
	log.Fatal(err)
}

var backend kv.KV = client

// e.g. as the backend of a generic KV nonce store; SetNX makes the nonce check atomic
nonceStore := store.NewKvStore(backend, time.Hour)
```

### Connection Health Checking

```go
client, _ := redis.NewClient(config)

// Test connection
if err := client.Connect(); err != nil {
	log.Printf("Redis connection failed: %v", err)
	// Handle connection failure
} else {
	log.Println("Redis connection successful")
}
```

### Expiration and Prune

Redis expires keys itself, so `Prune()` is a no-op that always returns `nil`; it exists only to satisfy
the `kv.KV` interface (the session store calls it on every cleanup tick). Use `SetTTL` to control
expiry per key; a TTL of `0` stores the key without expiry. `Set` uses the configured default `TTL`.

## Security Considerations

### Credential Management

```go
// Use environment variables
config.PasswordEnvVar = "REDIS_PASSWORD"

// Use secure file storage
config.PasswordFile = "/run/secrets/redis_password"
```

The password is decrypted once in `NewClient` and handed to the go-redis client, which keeps it in its
options to authenticate new pool connections.

### TLS Security

```go
config.ClientConfig = tls.ClientConfig{
	TLSEnable: true,
	TLSCA:     "/path/to/ca.crt", // verify the server against this CA
}
```

The server certificate is verified against `TLSCA` (or the system roots when empty) and the host in
`Address`. Leave `TLSInsecureSkipVerify` off outside of testing. Minimum version and cipher suites use the
Go `crypto/tls` client defaults; they are not configurable on the Redis client.

### Key Security

- Use key prefixes to isolate different applications
- Implement appropriate TTL values to prevent key accumulation
- Consider using Redis AUTH for additional security

## Integration Examples

### With HTTP Server Session Storage

```go
import (
	"github.com/oddbit-project/blueprint/log"
	"github.com/oddbit-project/blueprint/provider/httpserver"
	"github.com/oddbit-project/blueprint/provider/httpserver/session"
	"github.com/oddbit-project/blueprint/provider/redis"
)

logger := log.New("app")

// Use Redis as session backend
redisClient, err := redis.NewClient(redis.NewConfig())
if err != nil {
	logger.Fatal(err, "failed to create redis client")
}

server, err := httpserver.NewServer(httpserver.NewServerConfig(), logger)
if err != nil {
	logger.Fatal(err, "failed to create server")
}

// UseSession registers the session middleware and returns the session manager
sessionManager, err := server.UseSession(session.NewConfig(), redisClient, logger)
if err != nil {
	logger.Fatal(err, "failed to enable sessions")
}
```

The session store calls `Prune()` on every cleanup tick; with Redis this is a no-op, and sessions
expire through the TTL set on each key.

### With HMAC Provider for Nonce Storage

```go
import (
	"time"

	"github.com/oddbit-project/blueprint/provider/hmacprovider"
	"github.com/oddbit-project/blueprint/provider/hmacprovider/store"
	"github.com/oddbit-project/blueprint/provider/redis"
)

redisClient, err := redis.NewClient(redis.NewConfig())
if err != nil {
	log.Fatal(err)
}

// SetNX-based nonce store; keys are "nonce:<nonce>" by default (see store.WithPrefix)
nonceStore := store.NewRedisNonceStore(redisClient, time.Hour)

hmacProvider := hmacprovider.NewHmacProvider(keyProvider, hmacprovider.WithNonceStore(nonceStore))
```

## Error Handling

The Redis provider handles various error conditions:

```go
data, err := client.Get("nonexistent")
if err != nil {
	log.Fatal("Error:", err)
}

if data == nil {
	fmt.Println("Key not found")
} else {
	fmt.Printf("Data: %s\n", data)
}
```

## Best Practices

1. **Connection Management**: Always call `Close()` to properly cleanup connections
2. **Error Handling**: Check for connection errors and implement retry logic
3. **TTL Management**: Set appropriate TTL values to prevent memory bloat
4. **Key Naming**: Use consistent key naming conventions and prefixes
5. **Security**: Use TLS for production deployments and secure credential storage
6. **Monitoring**: Implement connection health checks in production

## Performance Considerations

- **Connection Pooling**: The underlying go-redis library handles connection pooling automatically
- **TTL Setting**: Configure appropriate default TTL to balance data persistence and memory usage
- **Key Prefixing**: Use prefixes to organize keys and enable efficient pattern matching
- **Timeout Configuration**: Set reasonable timeouts to prevent hanging operations

## Troubleshooting

### Connection Issues

```go
if err := client.Connect(); err != nil {
	log.Printf("Redis connection failed: %v", err)
	// Check network connectivity, credentials, and Redis server status
}
```

### Common Errors

- **Missing Address**: Ensure `Address` is properly configured
- **Authentication Failure**: Verify password configuration
- **Network Timeout**: Adjust `TimeoutSeconds` for slow networks
- **TLS Handshake**: Check certificate configuration for TLS connections

## See Also

- [Secure Credentials](../crypt/secure-credentials.md) - For secure password management
- [TLS Configuration](tls.md) - For TLS setup and security
- [HMAC Provider](hmacprovider.md) - For using Redis as nonce storage
- [HTTP Server Sessions](httpserver/session.md) - For session management integration