# HMAC Provider

The HMAC Provider offers cryptographically secure message authentication and signature verification using HMAC-SHA256. It provides protection against replay attacks, timing attacks, and memory exhaustion DoS attacks.

## Table of Contents

- [Overview](#overview)
- [Architecture](#architecture)
- [Security Features](#security-features)
- [Quick Start](#quick-start)
- [API Reference](#api-reference)
- [Key Providers](#key-providers)
- [Nonce Stores](#nonce-stores)
- [Configuration Options](#configuration-options)
- [HTTP Authentication](#http-authentication)
- [Best Practices](#best-practices)
- [Examples](#examples)
- [Performance](#performance)
- [Troubleshooting](#troubleshooting)

## Overview

The HMAC Provider implements HMAC-SHA256 signatures with two operation modes:

1. **Simple Mode**: Basic HMAC signatures without replay protection
2. **Secure Mode**: HMAC signatures with nonces and timestamps for replay protection

### Key Features

- **Replay Attack Prevention**: Nonce-based protection with atomic check-and-set
- **Timing Attack Resistance**: Constant-time comparisons for security
- **DoS Protection**: Configurable input size limits (default: 32MB)
- **Pluggable Storage**: Memory, Redis, and generic KV backends
- **Clock Drift Tolerance**: Configurable timestamp validation windows
- **Multi-tenant Support**: Key provider interface for multiple secrets

## Architecture

The HMAC Provider consists of several components working together:

```
┌─────────────────┐     ┌──────────────────┐     ┌─────────────────┐
│  HMAC Provider  │────▶│  Key Provider    │     │  Nonce Store    │
│                 │     │  (Secret Mgmt)   │     │  (Replay Prot)  │
└────────┬────────┘     └──────────────────┘     └─────────────────┘
         │                                                  │
         │              ┌──────────────────┐               │
         └─────────────▶│  Secure          │◀──────────────┘
                        │  Credential      │
                        └──────────────────┘
```

### Core Components

1. **HMACProvider**: Main orchestrator for signature operations
2. **HMACKeyProvider**: Interface for secret key management
3. **NonceStore**: Interface for replay attack prevention
4. **Secure Credential**: Encrypted secret storage with memory protection

## Security Features

### Implemented Protections

- **Replay Protection**: UUID-based nonces with TTL expiration
- **Timing Attack Resistance**: Constant-time HMAC verification
- **Input Size Limits**: Prevents memory exhaustion attacks
- **Timestamp Validation**: Configurable time windows for clock drift
- **Atomic Operations**: Thread-safe nonce consumption
- **Secure Storage**: Integration with encrypted credential system
- **Fail-Safe Defaults**: Secure configuration out of the box

### Security Properties

- **Cryptographic Integrity**: HMAC-SHA256 ensures message authenticity
- **Non-repudiation**: Nonces prevent request replay
- **Forward Secrecy**: Time-limited validity of signatures
- **Defense in Depth**: Multiple layers of validation

## Quick Start

### Basic Usage

```go
package main

import (
    "strings"
    "github.com/oddbit-project/blueprint/crypt/secure"
    "github.com/oddbit-project/blueprint/provider/hmacprovider"
)

func main() {
    // Generate encryption key
    key, err := secure.GenerateKey()
    if err != nil {
        panic(err)
    }
    
    // Create credential from password
    secret, err := secure.NewCredential([]byte("my-secret"), key, false)
    if err != nil {
        panic(err)
    }
    
	// create single key provider
	keyProvider := hmacprovider.NewSingleKeyProvider("mykey", secret)
    // Create HMAC provider
    provider := hmacprovider.NewHmacProvider(keyProvider)
    
    // Sign data with replay protection
    data := "Hello, World!"
    hash, timestamp, nonce, err := provider.Sign256("mykey", strings.NewReader(data))
    if err != nil {
        panic(err)
    }
    
    // Verify signature
    keyId, valid, err := provider.Verify256(strings.NewReader(data), hash, timestamp, nonce)
    if err != nil {
        panic(err)
    }
    
    if valid {
        println("Signature verified! Key ID:", keyId)
    }
}
```

### HTTP Authentication Integration

```go
import (
    "github.com/oddbit-project/blueprint/crypt/secure"	
    "github.com/oddbit-project/blueprint/provider/httpserver/auth"
    "github.com/oddbit-project/blueprint/provider/hmacprovider"
)
// Generate encryption key
key, err := secure.GenerateKey()
if err != nil {
panic(err)
}

// Create credential from password
secret, err := secure.NewCredential([]byte("my-secret"), key, false)
if err != nil {
panic(err)
}

// create single key provider
keyProvider := hmacprovider.NewSingleKeyProvider("mykey", secret)

// Create HMAC provider
provider := hmacprovider.NewHmacProvider(keyProvider)

// Create HMAC auth provider
hmacAuth := auth.NewHMACAuthProvider(provider)

// Apply to routes
router.Use(auth.AuthMiddleware(hmacAuth))

// Access authentication info in handlers
func handler(c *gin.Context) {
    keyId, ok := auth.GetHMACIdentity(c)
    if ok {
        // Use keyId for tenant identification
    }
    
    // Get full HMAC details
    keyId, timestamp, nonce, ok := auth.GetHMACDetails(c)
    if ok {
        // Access all HMAC authentication data
    }
}
```

## API Reference

> Note: keyId **cannot contain dots ('.')** as they are used for keyId+hash concatenation

### Constructor

#### `NewHmacProvider(keyProvider HMACKeyProvider, opts ...HMACProviderOption) *HMACProvider`

Creates a new HMAC provider with the specified key provider and options.

**Parameters:**
- `keyProvider`: Implementation of HMACKeyProvider interface
- `opts`: Optional configuration functions

**Returns:** Configured HMAC provider instance

### Simple HMAC Methods

#### `SHA256Sign(keyId string, data io.Reader) (string, error)`

Generates a simple HMAC-SHA256 signature without replay protection.

**Parameters:**
- `keyId`: Identifier for the key to use
- `data`: Input data to sign

**Returns:** 
- `string`: Hex-encoded HMAC signature
- `error`: Any error that occurred

#### `SHA256Verify(data io.Reader, hash string) (keyId string, valid bool, error)`

Verifies a simple HMAC-SHA256 signature.

**Parameters:**
- `data`: Input data to verify
- `hash`: Hex-encoded HMAC signature to verify

**Returns:**
- `keyId`: Identifier of the key that validated the signature
- `valid`: True if signature is valid
- `error`: Any error that occurred

### Secure HMAC Methods

#### `Sign256(keyId string, data io.Reader) (hash, timestamp, nonce string, err error)`

Generates a secure HMAC-SHA256 signature with replay protection.

**Parameters:**
- `keyId`: Identifier for the key to use
- `data`: Input data to sign

**Returns:**
- `hash`: Hex-encoded HMAC signature
- `timestamp`: RFC3339 timestamp
- `nonce`: UUID v4 nonce
- `err`: Any error that occurred

#### `Verify256(data io.Reader, hash, timestamp, nonce string) (keyId string, valid bool, error)`

Verifies a secure HMAC-SHA256 signature with replay protection.

**Parameters:**
- `data`: Input data to verify
- `hash`: Hex-encoded HMAC signature
- `timestamp`: RFC3339 timestamp from signing
- `nonce`: UUID nonce from signing

**Returns:**
- `keyId`: Identifier of the key that validated the signature
- `valid`: True if signature is valid and not replayed
- `error`: Any error that occurred

## Key Providers

The HMAC system supports multiple key management strategies through the `HMACKeyProvider` interface:

### Interface Definition

```go
type HMACKeyProvider interface {
    GetKey(keyId string) (*secure.Credential, error)
}
```

### Single Key Provider

> Note: the keyId can be an empty string

Simple provider for single-key applications:

```go
// Create single key provider
provider := hmacprovider.NewSingleKeyProvider("myKeyId", credential)

// Always uses the same key regardless of keyId
hmac := hmacprovider.NewHmacProvider(provider)
```

### Multi-Tenant Key Provider

For applications with multiple tenants or key rotation:

```go
type MultiTenantKeyProvider struct {
    keys map[string]*secure.Credential
	m sync.RWMutex
}

func (m *MultiTenantKeyProvider) GetKey(keyId string) (*secure.Credential, error) {
    m.m.RLock()
	defer m.m.RUnlock()
	
	key, exists := m.keys[keyId]
    if !exists {
        return nil, errors.New("unknown key ID")
    }
    return key, nil
}

func (m *MultiTenantKeyProvider) ListKeyIds() []string {
    m.m.RLock()
    defer m.m.RUnlock()
	
    ids := make([]string, 0, len(m.keys))
    for id := range m.keys {
        ids = append(ids, id)
    }
    return ids
}
```

## Nonce Stores

The HMAC provider supports multiple nonce store backends for replay protection:

### Memory Store (Default)

In-memory storage with configurable TTL and eviction policies.

```go
import "github.com/oddbit-project/blueprint/provider/hmacprovider/store"

// Create with custom options
memoryStore := store.NewMemoryNonceStore(
    store.WithTTL(1*time.Hour),
    store.WithMaxSize(1000000),
    store.WithCleanupInterval(15*time.Minute),
    store.WithEvictPolicy(store.EvictHalfLife()),
)

provider := hmacprovider.NewHmacProvider(keyProvider,
    hmacprovider.WithNonceStore(memoryStore),
)
```

**Eviction Policies:**
- `EvictNone()`: No automatic eviction (default)
- `EvictAll()`: Remove all nonces when at capacity
- `EvictHalfLife()`: Remove nonces older than TTL/2

**Best For:** Single-instance applications, development, low-traffic APIs

### Redis Store

Redis-backed storage with atomic operations for distributed systems.

```go
import (
    "github.com/oddbit-project/blueprint/provider/redis"
    "github.com/oddbit-project/blueprint/provider/hmacprovider/store"
)

// Configure Redis client
config := redis.NewConfig()
config.Address = "localhost:6379"
config.Database = 1

redisClient, err := redis.NewClient(config)
if err != nil {
    panic(err)
}

// Create Redis nonce store
redisStore := store.NewRedisStore(
    redisClient, 
    1*time.Hour,     // TTL
    "hmac:nonce:",   // Key prefix
)

provider := hmacprovider.NewHmacProvider(keyProvider,
    hmacprovider.WithNonceStore(redisStore),
)
```

**Features:**
- Atomic SetNX operations
- Configurable key prefix for namespacing
- Automatic TTL management
- Network timeout handling

**Best For:** Multi-instance deployments, high-traffic APIs, production systems

### Generic KV Store

Adapter for any key-value backend implementing the KV interface.

```go
import (
    "github.com/oddbit-project/blueprint/provider/kv"
    "github.com/oddbit-project/blueprint/provider/hmacprovider/store"
)

// Use any KV implementation
var kvBackend kv.KV = getYourKVBackend()

kvStore := store.NewKvStore(kvBackend, 1*time.Hour)

provider := hmacprovider.NewHmacProvider(keyProvider,
    hmacprovider.WithNonceStore(kvStore),
)
```

**Best For:** Custom storage requirements, existing KV infrastructure

## Configuration Options

### `WithNonceStore(store NonceStore)`

Sets the nonce store backend for replay protection.

```go
provider := hmacprovider.NewHmacProvider(keyProvider,
    hmacprovider.WithNonceStore(customStore),
)
```

### `WithKeyInterval(interval time.Duration)`

Sets the allowed timestamp deviation window. Default: 5 minutes.

```go
provider := hmacprovider.NewHmacProvider(keyProvider,
    hmacprovider.WithKeyInterval(10*time.Minute), // ±10 minutes
)
```

### `WithMaxInputSize(maxSize int)`

Sets the maximum input size to prevent DoS attacks. Default: 32MB.

```go
provider := hmacprovider.NewHmacProvider(keyProvider,
    hmacprovider.WithMaxInputSize(1024*1024), // 1MB limit
)
```

## HTTP Authentication

### Required Headers

When using HMAC authentication with HTTP, the following headers are required:

- `X-HMAC-Hash`: The HMAC-SHA256 signature
- `X-HMAC-Timestamp`: RFC3339 formatted timestamp
- `X-HMAC-Nonce`: UUID v4 nonce

### Client Implementation

```go
func makeAuthenticatedRequest(provider *hmacprovider.HMACProvider, url string, body []byte) error {
    // Generate signature
    bodyReader := bytes.NewReader(body)
    hash, timestamp, nonce, err := provider.Sign256("client-key", bodyReader)
    if err != nil {
        return err
    }
    
    // Create request
    req, err := http.NewRequest("POST", url, bytes.NewReader(body))
    if err != nil {
        return err
    }
    
    // Add HMAC headers
    req.Header.Set("X-HMAC-Hash", hash)
    req.Header.Set("X-HMAC-Timestamp", timestamp)
    req.Header.Set("X-HMAC-Nonce", nonce)
    req.Header.Set("Content-Type", "application/json")
    
    // Send request
    client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    
    return nil
}
```

### Server Integration

```go
import (
    "github.com/gin-gonic/gin"
    "github.com/oddbit-project/blueprint/provider/httpserver/auth"
    "github.com/oddbit-project/blueprint/crypt/secure"
    "github.com/oddbit-project/blueprint/provider/hmacprovider"
)

func createHMACProvider() *hmacprovider.HMACProvider {
    // Generate encryption key
    key, err := secure.GenerateKey()
    if err != nil {
        panic(err)
    }

    // Create credential from password
    secret, err := secure.NewCredential([]byte("my-secret"), key, false)
    if err != nil {
        panic(err)
    }

	// create single key provider
    keyProvider := hmacprovider.NewSingleKeyProvider("client-key", secret)
    
	// Create HMAC provider
    return hmacprovider.NewHmacProvider(keyProvider)
}

func setupServer() {
    router := gin.Default()
    
    // Create HMAC provider
    hmacProvider := createHMACProvider()
    
    // Create auth middleware
    hmacAuth := auth.NewHMACAuthProvider(hmacProvider)
    
    // Protected routes
    api := router.Group("/api")
    api.Use(auth.AuthMiddleware(hmacAuth))
    {
        api.POST("/data", handleData)
        api.PUT("/update", handleUpdate)
    }
    
    router.Run(":8080")
}

func handleData(c *gin.Context) {
    // Get authenticated key ID
    keyId, exists := auth.GetHMACIdentity(c)
    if !exists {
        c.JSON(500, gin.H{"error": "Authentication info missing"})
        return
    }
    
    // Get full HMAC details
    keyId, timestamp, nonce, ok := auth.GetHMACDetails(c)
    if !ok {
        c.JSON(500, gin.H{"error": "HMAC details missing"})
        return
    }
    
    c.JSON(200, gin.H{
        "message": "Authenticated request",
        "tenant": keyId,
        "timestamp": timestamp,
        "nonce": nonce,
    })
}
```

## Best Practices

### Security Recommendations

1. **Always Use Secure Mode**: Use `Sign256`/`Verify256` for replay protection
2. **Strong Secrets**: Generate cryptographically secure secrets (32+ bytes)
3. **Key Rotation**: Implement regular key rotation policies
4. **Secure Storage**: Use encrypted credential storage
5. **HTTPS Only**: Always use TLS for transport security
6. **Input Validation**: Set appropriate `MaxInputSize` limits
7. **Clock Sync**: Ensure server clocks are synchronized (NTP)
8. **Monitoring**: Log and monitor authentication failures

### Performance Optimization

1. **Choose Appropriate Backend**:
   - Memory: Single-instance, low-traffic
   - Redis: Multi-instance, high-traffic
   - Custom KV: Specific requirements

2. **Tune Configuration**:
   - Adjust TTL based on security requirements
   - Set cleanup intervals based on traffic
   - Choose eviction policy based on memory

3. **Connection Pooling**:
   - Use connection pools for Redis
   - Configure appropriate timeouts

### Error Handling

```go
func handleHMACError(err error, clientIP string) {
    if err != nil {
        switch {
        case strings.Contains(err.Error(), "invalid request"):
            // Input validation failure
            log.Warn("Invalid HMAC request", "ip", clientIP, "error", err)
        case strings.Contains(err.Error(), "input too large"):
            // Potential DoS attempt
            log.Error("HMAC input too large", "ip", clientIP, "error", err)
        case strings.Contains(err.Error(), "nonce already used"):
            // Replay attack
            log.Error("HMAC replay attack detected", "ip", clientIP, "error", err)
        default:
            // Other errors
            log.Error("HMAC verification failed", "ip", clientIP, "error", err)
        }
    }
}
```

## Examples

### Multi-Tenant API

```go
type TenantKeyProvider struct {
    tenants map[string]*secure.Credential
    mu      sync.RWMutex
}

func (t *TenantKeyProvider) GetKey(tenantId string) (*secure.Credential, error) {
    t.mu.RLock()
    defer t.mu.RUnlock()
    
    cred, exists := t.tenants[tenantId]
    if !exists {
        return nil, fmt.Errorf("unknown tenant: %s", tenantId)
    }
    return cred, nil
}

func (t *TenantKeyProvider) ListKeyIds() []string {
    t.mu.RLock()
    defer t.mu.RUnlock()
    
    ids := make([]string, 0, len(t.tenants))
    for id := range t.tenants {
        ids = append(ids, id)
    }
    return ids
}

// Usage
tenantProvider := &TenantKeyProvider{
    tenants: loadTenantKeys(),
}

hmacProvider := hmacprovider.NewHmacProvider(
    tenantProvider,
    hmacprovider.WithNonceStore(redisStore),
    hmacprovider.WithKeyInterval(10*time.Minute),
)
```

### Webhook Verification

```go
func verifyWebhook(provider *hmacprovider.HMACProvider, r *http.Request) error {
    // Extract headers
    hash := r.Header.Get("X-Webhook-Signature")
    timestamp := r.Header.Get("X-Webhook-Timestamp")
    nonce := r.Header.Get("X-Webhook-Id")
    
    // Read body
    body, err := io.ReadAll(r.Body)
    if err != nil {
        return fmt.Errorf("failed to read body: %w", err)
    }
    r.Body = io.NopCloser(bytes.NewReader(body))
    
    // Verify signature
    keyId, valid, err := provider.Verify256(
        bytes.NewReader(body), 
        hash, 
        timestamp, 
        nonce,
    )
    
    if err != nil {
        return fmt.Errorf("verification error: %w", err)
    }
    
    if !valid {
        return errors.New("invalid webhook signature")
    }
    
    log.Info("Webhook verified", "source", keyId)
    return nil
}
```

## Performance

### Benchmarks

Performance results on Intel Core i5-10400F @ 2.90GHz:

- **SHA256Sign**: ~2.1μs per operation (2,184 B/op, 18 allocs/op)
- **SHA256Verify**: ~2.0μs per operation (2,008 B/op, 15 allocs/op)
- **Sign256** (with nonce): ~3.1μs per operation (2,344 B/op, 25 allocs/op)
- **Verify256** (with nonce): ~2.9μs per operation (2,213 B/op, 19 allocs/op)
- **Full Cycle** (Sign256 + Verify256): ~6.1μs per operation (4,557 B/op, 44 allocs/op)

### Optimization Tips

1. **Reuse Provider Instances**: Create once, use many times
2. **Buffer Pool**: Use sync.Pool for byte buffers
3. **Batch Operations**: Process multiple items in sequence
4. **Connection Pooling**: Configure Redis connection pools
5. **Async Processing**: Use goroutines for independent verifications

## Troubleshooting

### Common Issues

#### "invalid request" Error

**Cause**: Input validation failure
**Solution**: 
- Check all parameters are provided
- Verify timestamp format (RFC3339)
- Ensure nonce is valid UUID

#### "input too large" Error

**Cause**: Input exceeds MaxInputSize
**Solution**:
- Increase limit with `WithMaxInputSize`
- Reduce input size
- Check for erroneous large inputs

#### "nonce already used" Error

**Cause**: Replay attack or duplicate request
**Solution**:
- Ensure unique nonce generation
- Check for request retry logic
- Verify nonce store is working

#### Clock Drift Issues

**Symptoms**: Intermittent verification failures
**Solution**:
- Sync server clocks with NTP
- Increase KeyInterval tolerance
- Monitor timestamp differences

## Constants

- `DefaultKeyInterval`: 5 minutes (300 seconds)
- `DefaultMaxInputSize`: 32MB (33554432 bytes)
- `DefaultTTL`: 4 hours (nonce stores)
- `DefaultMaxSize`: 2,000,000 entries (memory store)
- `DefaultCleanupInterval`: 15 minutes (memory store)