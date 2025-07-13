# HMAC Provider

The HMAC Provider offers cryptographically secure message authentication and signature verification using HMAC-SHA256. 
It provides protection against replay attacks, timing attacks, and memory exhaustion DoS attacks.

## Table of Contents

- [Overview](#overview)
- [Security Features](#security-features)
- [Quick Start](#quick-start)
- [API Reference](#api-reference)
- [Nonce Stores](#nonce-stores)
- [Configuration Options](#configuration-options)
- [Best Practices](#best-practices)
- [Examples](#examples)
- [Troubleshooting](#troubleshooting)

## Overview

The HMAC Provider implements HMAC-SHA256 signatures with two operation modes:

1. **Simple Mode**: Basic HMAC signatures without nonces or timestamps
2. **Secure Mode**: HMAC signatures with nonces and timestamps for replay protection

### Key Features

- **Replay Attack Prevention**: Nonce-based protection against message replay
- **Timing Attack Resistance**: Constant-time comparisons and early hex validation
- **DoS Protection**: Configurable input size limits and nonce store capacity
- **Pluggable Storage**: Memory, KV, and Redis nonce store backends
- **Clock Drift Tolerance**: Configurable timestamp validation windows

## Security Features

### Implemented Protections
   - Atomic nonce check-and-set operations
   - Proper operation ordering in verification
   - Configurable input size limits (default: 32MB)
   - Nonce store capacity limits with eviction policies
   - Constant-time HMAC verification
   - Fail-safe error handling
   - UUID-based nonces with TTL expiration
   - Multiple nonce store backends
   - Atomic nonce consumption (where available)

## Quick Start

```go
package main

import (
    "strings"
    "time"
    
    "github.com/oddbit-project/blueprint/crypt/secure"
    "github.com/oddbit-project/blueprint/provider/hmacprovider"
)

func main() {
    // Generate encryption key
    key, err := secure.GenerateKey()
    if err != nil {
        panic(err)
    }
    
    // Create credential
    credential, err := secure.NewCredential([]byte("my-secret"), key, false)
    if err != nil {
        panic(err)
    }
    
    // Create HMAC provider
    provider := hmacprovider.NewHmacProvider(credential)
    
    // Sign data with nonce and timestamp
    data := "Hello, World!"
    hash, timestamp, nonce, err := provider.Sign256(strings.NewReader(data))
    if err != nil {
        panic(err)
    }
    
    // Verify signature
    valid, err := provider.Verify256(strings.NewReader(data), hash, timestamp, nonce)
    if err != nil {
        panic(err)
    }
    
    if valid {
        println("Signature verified!")
    }
}
```

## API Reference

### Constructor

#### `NewHmacProvider(credential *secure.Credential, opts ...HMACProviderOption) *HMACProvider`

Creates a new HMAC provider with the specified credential and options.

**Parameters:**
- `credential`: Secure credential containing the HMAC secret
- `opts`: Optional configuration functions

**Returns:** Configured HMAC provider instance

### Simple HMAC Methods

#### `SHA256Sign(data io.Reader) (string, error)`

Generates a simple HMAC-SHA256 signature without nonce or timestamp.

**Parameters:**
- `data`: Input data to sign

**Returns:** 
- `string`: Hex-encoded HMAC signature
- `error`: Any error that occurred

#### `SHA256Verify(data io.Reader, hash string) (bool, error)`

Verifies a simple HMAC-SHA256 signature.

**Parameters:**
- `data`: Input data to verify
- `hash`: Hex-encoded HMAC signature to verify

**Returns:**
- `bool`: True if signature is valid
- `error`: Any error that occurred

### Secure HMAC Methods

#### `Sign256(data io.Reader) (hash, timestamp, nonce string, err error)`

Generates a secure HMAC-SHA256 signature with nonce and timestamp.

**Parameters:**
- `data`: Input data to sign

**Returns:**
- `hash`: Hex-encoded HMAC signature
- `timestamp`: RFC3339 timestamp
- `nonce`: UUID nonce
- `err`: Any error that occurred

#### `Verify256(data io.Reader, hash, timestamp, nonce string) (bool, error)`

Verifies a secure HMAC-SHA256 signature with replay protection.

**Parameters:**
- `data`: Input data to verify
- `hash`: Hex-encoded HMAC signature
- `timestamp`: RFC3339 timestamp from signing
- `nonce`: UUID nonce from signing

**Returns:**
- `bool`: True if signature is valid and not replayed
- `error`: Any error that occurred

## Nonce Stores

The HMAC provider supports multiple nonce store backends for replay protection:

### Memory Store (Default)

In-memory nonce storage with TTL and eviction policies.

```go
import "github.com/oddbit-project/blueprint/provider/hmacprovider/store"

memoryStore := store.NewMemoryNonceStore(
    store.WithTTL(1*time.Hour),
    store.WithMaxSize(1000000),
    store.WithCleanupInterval(15*time.Minute),
    store.WithEvictPolicy(store.EvictHalfLife()),
)

provider := hmacprovider.NewHmacProvider(credential,
    hmacprovider.WithNonceStore(memoryStore),
)
```

**Eviction Policies:**
- `EvictNone()`: No automatic eviction (default)
- `EvictAll()`: Remove all nonces when at capacity
- `EvictHalfLife()`: Remove nonces older than TTL/2

### KV Store

Generic key-value store adapter supporting any KV backend.

```go
import (
    "github.com/oddbit-project/blueprint/provider/kv"
    "github.com/oddbit-project/blueprint/provider/hmacprovider/store"
)

// Using memory KV (for testing)
memKV := kv.NewMemoryKV()
kvStore := store.NewKvStore(memKV, 1*time.Hour)

provider := hmacprovider.NewHmacProvider(credential,
    hmacprovider.WithNonceStore(kvStore),
)
```

### Redis Store

Redis-backed nonce storage with atomic operations.

```go
import (
    "github.com/oddbit-project/blueprint/provider/redis"
    "github.com/oddbit-project/blueprint/provider/hmacprovider/store"
)

// Configure Redis client
redisClient, err := redis.NewRedisProvider(&redis.RedisConfig{
    Host: "localhost:6379",
    DB:   0,
})
if err != nil {
    panic(err)
}

redisStore := store.NewRedisStore(redisClient, 1*time.Hour, "hmac:")

provider := hmacprovider.NewHmacProvider(credential,
    hmacprovider.WithNonceStore(redisStore),
)
```

## Configuration Options

### `WithNonceStore(store NonceStore)`

Sets the nonce store backend for replay protection.

### `WithKeyInterval(interval time.Duration)`

Sets the allowed timestamp deviation window. Default: 5 minutes.

```go
provider := hmacprovider.NewHmacProvider(credential,
    hmacprovider.WithKeyInterval(10*time.Minute), // ±10 minutes
)
```

### `WithMaxInputSize(maxSize int)`

Sets the maximum input size to prevent DoS attacks. Default: 32MB.

```go
provider := hmacprovider.NewHmacProvider(credential,
    hmacprovider.WithMaxInputSize(1024*1024), // 1MB limit
)
```

## Best Practices

### Security Considerations

1. **Use Secure Mode**: Always use `Sign256`/`Verify256` for replay protection
2. **Proper Secret Management**: Use secure credential storage
3. **Input Validation**: Set appropriate `MaxInputSize` for your use case
4. **Clock Synchronization**: Ensure server clocks are synchronized
5. **Nonce Store Scaling**: Use Redis for high-traffic applications

### Performance Optimization

1. **Memory Store**: Best for low-traffic applications
2. **Redis Store**: Recommended for distributed systems
3. **Cleanup Intervals**: Tune based on traffic patterns
4. **Eviction Policies**: Choose based on memory constraints

### Error Handling

```go
valid, err := provider.Verify256(data, hash, timestamp, nonce)
if err != nil {
    // Log security event - potential attack
    log.Warn("HMAC verification failed", "error", err)
    return false
}
if !valid {
    // Signature mismatch or replay attack
    log.Info("Invalid HMAC signature")
    return false
}
```

## Examples

### Web API Authentication

```go
func authenticateRequest(provider *hmacprovider.HMACProvider, r *http.Request) bool {
    // Extract signature components from headers
    hash := r.Header.Get("X-Signature")
    timestamp := r.Header.Get("X-Timestamp")
    nonce := r.Header.Get("X-Nonce")
    
    // Read request body
    body, err := io.ReadAll(r.Body)
    if err != nil {
        return false
    }
    r.Body = io.NopCloser(bytes.NewReader(body)) // Restore body
    
    // Verify signature
    valid, err := provider.Verify256(bytes.NewReader(body), hash, timestamp, nonce)
    return err == nil && valid
}
```

### Message Queue Verification

```go
func verifyMessage(provider *hmacprovider.HMACProvider, msg *Message) bool {
    payload := strings.NewReader(msg.Data)
    valid, err := provider.Verify256(payload, msg.Hash, msg.Timestamp, msg.Nonce)
    return err == nil && valid
}
```

### Batch Processing

```go
func processBatch(provider *hmacprovider.HMACProvider, items []Item) {
    for _, item := range items {
        data := strings.NewReader(item.Data)
        hash, timestamp, nonce, err := provider.Sign256(data)
        if err != nil {
            log.Error("Failed to sign item", "error", err)
            continue
        }
        
        // Store signature with item
        item.Signature = Signature{
            Hash:      hash,
            Timestamp: timestamp,
            Nonce:     nonce,
        }
    }
}
```

## Troubleshooting

### Common Issues

#### "invalid request" Error

**Cause**: Input validation failure (empty parameters, invalid timestamp, etc.)
**Solution**: Check all required parameters and timestamp format

#### "input too large" Error

**Cause**: Input exceeds configured `MaxInputSize`
**Solution**: Increase limit or reduce input size

#### Nonce Store Capacity Issues

**Cause**: Memory store at capacity, eviction policy insufficient
**Solution**: 
- Increase `MaxSize`
- Use more aggressive eviction policy
- Switch to Redis store for high traffic

#### Clock Drift Issues

**Cause**: Server clocks out of sync beyond `KeyInterval`
**Solution**:
- Synchronize server clocks (NTP)
- Increase `KeyInterval` if needed

### Performance Monitoring

```go
// Monitor nonce store performance
func monitorNonceStore(store store.NonceStore) {
    start := time.Now()
    success := store.AddIfNotExists("test-nonce")
    duration := time.Since(start)
    
    log.Info("Nonce store performance",
        "success", success,
        "duration", duration,
    )
}
```

### Security Auditing

```go
// Log security events
func auditHMACVerification(result bool, err error, clientIP string) {
    if err != nil {
        log.Warn("HMAC verification error",
            "client_ip", clientIP,
            "error", err,
            "time", time.Now(),
        )
    } else if !result {
        log.Info("HMAC verification failed",
            "client_ip", clientIP,
            "time", time.Now(),
        )
    }
}
```

## Constants

- `DefaultKeyInterval`: 5 minutes
- `MaxInputSize`: 32MB
- `DefaultTTL`: 1 hour (nonce stores)
- `DefaultMaxSize`: 2,000,000 nonces (memory store)
- `DefaultCleanupInterval`: 15 minutes (memory store)