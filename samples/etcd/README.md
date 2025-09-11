# etcd Provider Sample

This sample demonstrates how to use the Blueprint etcd provider for distributed key-value storage, configuration management, service discovery, and coordination.

## Features Demonstrated

### Basic Operations
- **Put/Get/Delete**: Store, retrieve, and remove key-value pairs
- **Exists**: Check if a key exists without retrieving its value
- **Bulk Operations**: Atomic insertion and deletion of multiple keys

### Prefix Operations
- **List**: Get all keys with a specific prefix
- **ListWithValues**: Get all key-value pairs with a prefix
- **Count**: Count keys matching a prefix
- **DeletePrefix**: Remove all keys with a prefix

### Watch Operations
- **Watch**: Monitor changes to specific keys in real-time
- **WatchPrefix**: Monitor changes to all keys with a prefix
- **Event Handling**: Process PUT, DELETE, and other etcd events

### Lease Management
- **TTL Keys**: Store keys that automatically expire
- **Lease Creation**: Create leases with custom time-to-live
- **Keep-Alive**: Extend lease lifetimes
- **Automatic Cleanup**: Keys are removed when leases expire

### Distributed Locking
- **Mutex**: Distributed mutual exclusion
- **TryLock**: Non-blocking lock acquisition
- **Lock/Unlock**: Blocking lock operations
- **Session Management**: Automatic cleanup on disconnection

### Transactions
- **Atomic Operations**: Multiple operations in a single transaction
- **Compare-and-Swap**: Conditional updates based on current values
- **If-Then-Else**: Complex conditional logic

### Client-Side Encryption
- **Transparent Encryption**: Automatic encrypt/decrypt of values
- **AES-256-GCM**: Industry-standard encryption
- **Raw Data Protection**: Encrypted data in etcd cluster

## Prerequisites

### etcd Server
You need a running etcd server. Choose one of the following options:

#### Option 1: Docker
```bash
# Single node etcd
docker run -d --name etcd \
  -p 2379:2379 \
  -p 2380:2380 \
  quay.io/coreos/etcd:v3.5.0 \
  /usr/local/bin/etcd \
  --name s1 \
  --data-dir /etcd-data \
  --listen-client-urls http://0.0.0.0:2379 \
  --advertise-client-urls http://0.0.0.0:2379 \
  --listen-peer-urls http://0.0.0.0:2380 \
  --initial-advertise-peer-urls http://0.0.0.0:2380 \
  --initial-cluster s1=http://0.0.0.0:2380 \
  --initial-cluster-token tkn \
  --initial-cluster-state new
```

#### Option 2: Docker Compose
Create `docker-compose.yml`:
```yaml
version: '3.8'
services:
  etcd:
    image: quay.io/coreos/etcd:v3.5.0
    container_name: etcd
    ports:
      - "2379:2379"
      - "2380:2380"
    environment:
      ETCD_NAME: s1
      ETCD_DATA_DIR: /etcd-data
      ETCD_LISTEN_CLIENT_URLS: http://0.0.0.0:2379
      ETCD_ADVERTISE_CLIENT_URLS: http://0.0.0.0:2379
      ETCD_LISTEN_PEER_URLS: http://0.0.0.0:2380
      ETCD_INITIAL_ADVERTISE_PEER_URLS: http://0.0.0.0:2380
      ETCD_INITIAL_CLUSTER: s1=http://0.0.0.0:2380
      ETCD_INITIAL_CLUSTER_TOKEN: tkn
      ETCD_INITIAL_CLUSTER_STATE: new
    volumes:
      - etcd-data:/etcd-data

volumes:
  etcd-data:
```

Then run:
```bash
docker-compose up -d
```

### Go Dependencies
The sample uses Go modules and will automatically download dependencies:
```bash
go mod tidy
```

## Running the Sample

### Basic Run
```bash
go run main.go
```

### Help
```bash
go run main.go --help
```

### Expected Output
```
etcd Provider Sample Application
===============================
Connected to etcd at [localhost:2379]

Basic Operations Demo
---------------------
PUT: /sample/basic/message = Hello from Blueprint etcd provider!
GET: /sample/basic/message = Hello from Blueprint etcd provider!
EXISTS: /sample/basic/message = true
DELETE: /sample/basic/message (deleted 1 keys)

Prefix Operations Demo
----------------------
BULK_PUT: inserted 5 key-value pairs
LIST KEYS: found 3 user keys
   - /sample/users/alice
   - /sample/users/bob
   - /sample/users/charlie
LIST WITH VALUES: found 3 users
   - /sample/users/alice: {"name": "Alice", "email": "alice@example.com"}
   - /sample/users/bob: {"name": "Bob", "email": "bob@example.com"}
   - /sample/users/charlie: {"name": "Charlie", "email": "charlie@example.com"}
COUNT: total 5 keys under /sample/
DELETE PREFIX: removed 5 keys

Watch Demo
-----------
Started watching key: /sample/watch/counter
Event 1: PUT /sample/watch/counter = value-1
Event 2: PUT /sample/watch/counter = value-2
Event 3: PUT /sample/watch/counter = value-3
Watch demo completed

Lease Demo
-----------
Created lease with ID: 694d7aa7c9580a50 (TTL: 5s)
Stored key with lease: /sample/lease/temp-data
Key exists: true
Waiting 6 seconds for lease to expire...
Key exists after expiry: false
Lease demo completed

Distributed Lock Demo
---------------------
Created lock: /sample/locks/demo-lock
Testing blocking lock acquisition...
Lock acquired successfully
Performing critical section work...
Lock released
Testing TryLock (non-blocking) with 100ms timeout...
TryLock result: true
TryLock succeeded
TryLock released
Testing concurrent lock access...
TryLock while other lock held: false (should be false)
Lock2 acquired successfully
Lock2 released
Distributed lock demo completed

Transaction Demo
----------------
Initial counter value: 10
CompareAndSwap (10->20): true
Counter value after CAS: 20
CompareAndSwap (15->30): false (should be false)
Final counter value: 20
Transaction demo completed

Encryption Demo
---------------
Created encrypted client (encryption: true)
Stored encrypted data: /sample/encrypted/secret
Retrieved decrypted data: {"password": "super-secret-password", "api_key": "sk-1234567890"}
Raw encrypted data (first 50 chars): [encrypted binary data]...
Encryption demo completed

All samples completed successfully!
```

## Code Structure

### Configuration
```go
// Basic configuration
config := etcd.DefaultConfig().
    WithEndpoints("localhost:2379").
    WithTimeout(5 * time.Second)

// With authentication
config := etcd.DefaultConfig().
    WithEndpoints("localhost:2379").
    WithAuth("username", "password")

// With TLS
config := etcd.DefaultConfig().
    WithEndpoints("localhost:2379").
    WithTLS("/path/to/cert.pem", "/path/to/key.pem", "/path/to/ca.pem", false)

// With encryption
config := etcd.DefaultConfig().
    WithEndpoints("localhost:2379").
    WithEncryption([]byte("32-byte-encryption-key-here!!!"))
```

### Client Creation
```go
client, err := etcd.NewClient(config)
if err != nil {
    return fmt.Errorf("failed to create etcd client: %w", err)
}
defer client.Close()
```

### Basic Operations
```go
// Store data
err := client.Put(ctx, "/config/app/timeout", []byte("30s"))

// Retrieve data
value, err := client.Get(ctx, "/config/app/timeout")

// Check existence
exists, err := client.Exists(ctx, "/config/app/timeout")

// Delete
deleted, err := client.Delete(ctx, "/config/app/timeout")
```

### Prefix Operations
```go
// Bulk insert
kvs := map[string][]byte{
    "/users/alice": []byte(`{"name": "Alice"}`),
    "/users/bob":   []byte(`{"name": "Bob"}`),
}
err := client.BulkPut(ctx, kvs)

// List keys with prefix
keys, err := client.List(ctx, "/users/")

// Get all values with prefix
users, err := client.ListWithValues(ctx, "/users/")

// Count keys with prefix
count, err := client.Count(ctx, "/users/")
```

### Watch Operations
```go
watchChan := client.Watch(ctx, "/config/")

go func() {
    for watchResp := range watchChan {
        for _, event := range watchResp.Events {
            fmt.Printf("Event: %s %s = %s\n", 
                event.Type, string(event.Kv.Key), string(event.Kv.Value))
        }
    }
}()
```

### Distributed Locking
```go
lock, err := client.NewLock("/locks/critical-section")
if err != nil {
    return err
}
defer lock.Close()

// Try to acquire lock (non-blocking)
acquired, err := lock.TryLock(ctx)
if !acquired {
    return errors.New("lock is held by another process")
}

// Perform critical work
doImportantWork()

// Release lock
err = lock.Unlock(ctx)
```

### Leases
```go
// Create lease with 60 second TTL
leaseID, err := client.Lease(60)
if err != nil {
    return err
}

// Store key with lease
err = client.PutWithLease(ctx, "/temp/session", []byte("active"), leaseID)

// Key will automatically expire after 60 seconds
```

## Configuration Options

### Connection Settings
- **Endpoints**: List of etcd server URLs
- **DialTimeout**: Connection timeout
- **RequestTimeout**: Individual request timeout
- **KeepAlive**: Connection keep-alive settings

### Authentication
- **Username/Password**: Basic authentication
- **TLS**: Certificate-based authentication
- **Client Certificates**: Mutual TLS authentication

### Advanced Features
- **Encryption**: Client-side encryption of values
- **Compression**: Automatic compression of large values
- **Load Balancing**: Automatic load balancing across endpoints

## Error Handling

The provider follows Go's standard error handling patterns:

```go
value, err := client.Get(ctx, "/some/key")
if err != nil {
    if errors.Is(err, etcd.ErrKeyNotFound) {
        // Handle missing key
        return nil
    }
    return fmt.Errorf("failed to get key: %w", err)
}
```

## Performance Considerations

### Connection Pooling
The etcd client automatically manages connection pooling. Use a single client instance across your application.

### Batch Operations
Use bulk operations for better performance:
```go
// Better: single transaction
client.BulkPut(ctx, kvs)

// Avoid: multiple individual operations
for k, v := range kvs {
    client.Put(ctx, k, v) // Creates multiple round trips
}
```

### Watch Efficiency
- Use prefix watches instead of multiple individual watches
- Process watch events in batches when possible
- Close watch channels when no longer needed

### Memory Usage
- Close unused locks and sessions
- Use context cancellation to abort long-running operations
- Implement proper resource cleanup with defer statements

## Production Deployment

### High Availability
Configure multiple etcd endpoints:
```go
config := etcd.DefaultConfig().
    WithEndpoints("etcd1:2379", "etcd2:2379", "etcd3:2379")
```

### Security
- Always use TLS in production
- Enable client certificate authentication
- Use client-side encryption for sensitive data
- Rotate encryption keys regularly

### Monitoring
- Monitor etcd cluster health
- Set up alerts for connection failures
- Track key space usage and growth
- Monitor lease expiration rates

### Backup and Recovery
- Implement regular etcd snapshots
- Test backup restoration procedures
- Document recovery procedures
- Consider disaster recovery scenarios

## Further Reading

- [etcd Documentation](https://etcd.io/docs/)
- [Blueprint Provider Documentation](../../docs/providers/etcd.md)
- [etcd Client v3 API](https://pkg.go.dev/go.etcd.io/etcd/client/v3)
- [Distributed Systems Patterns](https://martinfowler.com/articles/patterns-of-distributed-systems/)