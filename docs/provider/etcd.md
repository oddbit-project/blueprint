# etcd Provider

The etcd provider offers a comprehensive Go client for interacting with etcd, a distributed key-value store. It provides
enhanced functionality including automatic encryption/decryption, simplified APIs, distributed locking, and robust
connection management.

## Features

- **Full etcd API Coverage**: Complete support for all standard etcd operations
- **Client-Side Encryption**: Optional AES-256-GCM encryption for sensitive data
- **Distributed Locking**: Robust distributed lock implementation using etcd sessions
- **Connection Management**: Built-in connection pooling, keep-alive, and timeout handling
- **TLS Support**: Secure communication with certificate-based authentication
- **Atomic Operations**: Support for transactions and compare-and-swap operations
- **Watch Support**: Real-time monitoring of key changes
- **Lease Management**: TTL-based key expiration and automatic renewal
- **Bulk Operations**: Efficient batch put/delete operations

## Installation

```bash
go get github.com/oddbit-project/blueprint/provider/etcd
```

## Quick Start

```go
package main

import (
	"context"
	"log"
	"github.com/oddbit-project/blueprint/provider/etcd"
)

func main() {
	// Create client with default configuration
	client, err := etcd.NewClient(nil)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	ctx := context.Background()

	// Store a value
	err = client.Put(ctx, "/myapp/config", []byte("value"))

	// Retrieve a value
	value, err := client.Get(ctx, "/myapp/config")

	// Delete a key
	deleted, err := client.Delete(ctx, "/myapp/config")
}
```

## Configuration

### Basic Configuration

```go
config := etcd.DefaultConfig().
WithEndpoints("localhost:2379", "localhost:2380").
WithTimeout(10 * time.Second).
WithAuth("username", "password")

client, err := config.NewClient()
```

### Configuration Options

| Option                 | Type            | Default              | Description                   |
|------------------------|-----------------|----------------------|-------------------------------|
| `Endpoints`            | `[]string`      | `["localhost:2379"]` | etcd server URLs              |
| `Username`             | `string`        | `""`                 | Authentication username       |
| `DialTimeout`          | `time.Duration` | `5s`                 | Connection timeout            |
| `DialKeepAliveTime`    | `time.Duration` | `30s`                | Keep-alive interval           |
| `DialKeepAliveTimeout` | `time.Duration` | `10s`                | Keep-alive timeout            |
| `RequestTimeout`       | `time.Duration` | `5s`                 | Individual request timeout    |
| `EnableEncryption`     | `bool`          | `false`              | Enable client-side encryption |
| `EncryptionKey`        | `[]byte`        | `nil`                | 32-byte encryption key        |
| `MaxCallSendMsgSize`   | `int`           | `2MB`                | Max message send size         |
| `MaxCallRecvMsgSize`   | `int`           | `2MB`                | Max message receive size      |

### TLS Configuration

```go
config := etcd.DefaultConfig().
WithEndpoints("etcd.example.com:2379").
WithTLS(
"/path/to/cert.pem", // Client certificate
"/path/to/key.pem",  // Client key
"/path/to/ca.pem", // CA certificate
false,             // InsecureSkipVerify
)
```

### Client-Side Encryption

```go
// Generate or load a 32-byte encryption key
encryptionKey := []byte("your-32-byte-encryption-key-here")

config := etcd.DefaultConfig().
WithEndpoints("localhost:2379").
WithEncryption(encryptionKey)

client, err := config.NewClient()
// All values will be automatically encrypted/decrypted
```

## Core Operations

### Key-Value Operations

```go
ctx := context.Background()

// Put - Store a key-value pair
err := client.Put(ctx, "/app/config", []byte("value"))

// Get - Retrieve a value
value, err := client.Get(ctx, "/app/config")

// GetMultiple - Retrieve multiple values with prefix
values, err := client.GetMultiple(ctx, "/app/", clientv3.WithPrefix())

// Delete - Remove a key
deleted, err := client.Delete(ctx, "/app/config")

// DeletePrefix - Remove all keys with prefix
deleted, err := client.DeletePrefix(ctx, "/app/")

// Exists - Check if key exists
exists, err := client.Exists(ctx, "/app/config")

// Count - Count keys with prefix
count, err := client.Count(ctx, "/app/")
```

### List Operations

```go
// List all keys with prefix
keys, err := client.List(ctx, "/app/")

// List with values
kvs, err := client.ListWithValues(ctx, "/app/")

// Get keys with limit
keys, err := client.GetKeysWithPrefix(ctx, "/app/", 10)

// Get keys by pattern
keys, err := client.GetKeysByPattern(ctx, "/app/", "config")
```

### Atomic Operations

```go
// Put if not exists
created, err := client.PutIfNotExists(ctx, "/app/lock", []byte("owner1"))

// Compare and swap
swapped, err := client.CompareAndSwap(ctx, "/app/version",
[]byte("v1"), []byte("v2"))

// Move key atomically
err := client.MoveKey(ctx, "/app/old", "/app/new")
```

### Bulk Operations

```go
// Bulk put - atomic batch insert
kvs := map[string][]byte{
"/app/config1": []byte("value1"),
"/app/config2": []byte("value2"),
"/app/config3": []byte("value3"),
}
err := client.BulkPut(ctx, kvs)

// Bulk delete - atomic batch delete
keys := []string{"/app/config1", "/app/config2"}
deleted, err := client.BulkDelete(ctx, keys)
```

## Distributed Locking

The provider includes a robust distributed locking mechanism using etcd sessions:

```go
// Create a distributed lock
lock, err := client.NewLock("/locks/resource1")
if err != nil {
log.Fatal(err)
}
defer lock.Close()

// Acquire lock (blocking)
err = lock.Lock(ctx)
if err != nil {
log.Fatal(err)
}

// Do critical section work
// ...

// Release lock
err = lock.Unlock(ctx)

// Try to acquire lock (non-blocking)
acquired, err := lock.TryLock(ctx, etcd.WithTTL(100*time.Millisecond))
if acquired {
// Got the lock
defer lock.Unlock(ctx)
}
```

### Lock Properties

- **Session-based**: Locks are tied to etcd sessions for automatic cleanup
- **Auto-release**: Locks are automatically released on session expiration
- **Safe**: Multiple unlock calls are safe (idempotent)
- **Non-blocking option**: TryLock for immediate return

## Watch Operations

Monitor key changes in real-time:

```go
// Watch single key
watchChan := client.Watch(ctx, "/app/config")

go func () {
for wresp := range watchChan {
for _, event := range wresp.Events {
fmt.Printf("Event: %s Key: %s Value: %s\n",
event.Type, event.Kv.Key, event.Kv.Value)
}
}
}()

// Watch with prefix
watchChan := client.WatchPrefix(ctx, "/app/")
```

## Lease Management

Implement TTL-based key expiration:

```go
// Create a lease with 60 second TTL
leaseID, err := client.Lease(60)

// Store key with lease
err = client.PutWithLease(ctx, "/temp/session",
[]byte("session-data"), leaseID)

// Keep lease alive
keepAliveChan, err := client.KeepAlive(ctx, leaseID)

go func () {
for ka := range keepAliveChan {
fmt.Printf("Lease %d renewed, TTL: %d\n",
ka.ID, ka.TTL)
}
}()

// Revoke lease (deletes all associated keys)
err = client.RevokeLease(ctx, leaseID)
```

## Transactions

Execute multiple operations atomically:

```go
txn := client.Transaction(ctx)

// If-Then-Else transaction
resp, err := txn.
If(clientv3.Compare(clientv3.Value("/app/version"), "=", "v1")).
Then(
clientv3.OpPut("/app/version", "v2"),
clientv3.OpPut("/app/updated", "true"),
).
Else(
clientv3.OpGet("/app/version"),
).
Commit()

if resp.Succeeded {
fmt.Println("Transaction succeeded")
}
```

## Advanced Features

### Range Queries

```go
// Get all keys in range [start, end)
kvs, err := client.GetRange(ctx, "/app/a", "/app/z")
```

### Historical Revisions

```go
// Get value at specific revision
value, err := client.GetWithRevision(ctx, "/app/config", revision)
```

### Cluster Management

```go
// Get cluster status
status, err := client.Status(ctx)
fmt.Printf("etcd version: %s, DB size: %d\n",
status.Version, status.DbSize)

// List cluster members
members, err := client.MemberList(ctx)
for _, member := range members.Members {
fmt.Printf("Member: %s, Peer URLs: %v\n",
member.Name, member.PeerURLs)
}
```

### Maintenance Operations

```go
// Compact revision history
err := client.CompactRevision(ctx, revision)
```

## Error Handling

The provider returns standard Go errors. Common error scenarios:

```go
value, err := client.Get(ctx, "/nonexistent")
if err != nil {
if err.Error() == "key not found" {
// Handle missing key
}
// Handle other errors
}
```

## Best Practices

### 1. Connection Management

- Use a single client instance per application
- Always defer `client.Close()` to cleanup resources
- Configure appropriate timeouts for your use case

### 2. Key Naming

- Use hierarchical key structure (e.g., `/app/module/key`)
- Avoid special characters in key names
- Use consistent prefixes for related data

### 3. Encryption

- Use encryption for sensitive data
- Store encryption keys securely (not in code)
- Consider key rotation strategies

### 4. Performance

- Use bulk operations for multiple updates
- Leverage prefix operations for related keys
- Set appropriate message size limits

### 5. Distributed Locking

- Always release locks in defer statements
- Use TryLock for non-critical sections
- Consider lock TTLs for fault tolerance

### 6. Watch Operations

- Handle watch events in separate goroutines
- Implement reconnection logic for long-lived watches
- Consider using revisions for resumable watches

## Testing

The provider includes comprehensive integration tests using testcontainers:

```bash
# Run integration tests
go test -tags=integration ./provider/etcd/...
```

## Example Application

A complete example application is available in `samples/etcd/` demonstrating:

- Basic CRUD operations
- Prefix operations
- Watch functionality
- Distributed locking
- Lease management
- Transactions
- Encryption

Run the example:

```bash
cd samples/etcd
go run main.go

# With custom endpoint
ETCD_ENDPOINTS=etcd.example.com:2379 go run main.go
```

## Dependencies

- `go.etcd.io/etcd/client/v3`: Official etcd Go client
- `github.com/oddbit-project/blueprint/crypt/secure`: Encryption utilities
- `github.com/oddbit-project/blueprint/provider/tls`: TLS configuration

## Thread Safety

All client methods are thread-safe and can be called concurrently from multiple goroutines.

## Compatibility

- etcd v3.5.x and later
- Go 1.18 or higher

## Migration from Standard etcd Client

The provider wraps the standard etcd client, providing access to the underlying client when needed:

```go
// Get underlying etcd client for advanced operations
etcdClient := client.GetClient()

// Use standard etcd client methods
resp, err := etcdClient.Get(ctx, "/key")
```
