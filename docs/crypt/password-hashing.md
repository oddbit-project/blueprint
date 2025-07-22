# Password Hashing

The password hashing system in Blueprint provides secure password storage using the Argon2id algorithm.
It offers automatic parameter upgrades, timing attack protection, and a clean interface for integrating 
secure password handling into applications.

## Features

- **Argon2id algorithm** - State-of-the-art memory-hard hashing function
- **Automatic rehashing** - Seamlessly upgrade hashes when security parameters change  
- **Timing attack protection** - Constant-time comparison for secure verification
- **Flexible configuration** - Customize memory, iterations, and parallelism
- **Thread-safe operations** - Safe for concurrent use without locks
- **Clean interface design** - Simple API with proper error handling

## Complete API Reference

### Core Interface

#### PasswordHasher

The main interface for password hashing operations.

```go
type PasswordHasher interface {
    Generate(password string) (string, error)
    Verify(password, hash string) (bool, RehashFn, error)
}
```

**Methods:**

##### Generate
```go
Generate(password string) (string, error)
```
Creates a secure hash from the given password.

**Parameters:**
- `password`: The plaintext password to hash

**Returns:**
- `string`: The complete hash including algorithm identifier, parameters, salt, and hash
- `error`: Error if hashing fails

##### Verify
```go
Verify(password, hash string) (bool, RehashFn, error)
```
Checks if a password matches the given hash.

**Parameters:**
- `password`: The plaintext password to verify
- `hash`: The hash string to compare against

**Returns:**
- `bool`: True if the password matches
- `RehashFn`: Function to generate new hash if parameters need updating (nil if not needed)
- `error`: Error if verification fails

#### RehashFn

Function type for rehashing passwords with updated parameters.

```go
type RehashFn = func() (string, error)
```

Returned by Verify when the hash was created with outdated parameters and should be updated.

### Configuration

#### Argon2Config

Configuration structure for Argon2id hashing parameters.

```go
type Argon2Config struct {
    Memory      uint32 `json:"memory"`      // Memory in KiB (e.g., 64*1024 = 64MB)
    Iterations  uint32 `json:"iterations"`  // Number of iterations (time cost)
    Parallelism uint8  `json:"parallelism"` // Number of parallel threads
    SaltLength  uint32 `json:"saltLength"`  // Salt length in bytes
    KeyLength   uint32 `json:"keyLength"`   // Output key length in bytes
}
```

### Factory Functions

#### NewArgon2Hasher
```go
func NewArgon2Hasher(cfg *Argon2Config) (PasswordHasher, error)
```
Creates a new password hasher using Argon2id.

**Parameters:**
- `cfg`: Configuration for Argon2 parameters (nil uses defaults)

**Returns:**
- `PasswordHasher`: New hasher instance
- `error`: Currently always returns nil

#### NewArgon2IdConfig
```go
func NewArgon2IdConfig() *Argon2Config
```
Returns the default Argon2id configuration.

**Default values:**
- Memory: 64MB (65536 KiB)
- Iterations: 4
- Parallelism: Number of CPU cores
- Salt length: 16 bytes
- Key length: 32 bytes

### Utility Functions

#### Argon2IdNeedsRehash
```go
func Argon2IdNeedsRehash(c *Argon2Config) bool
```
Checks if a hash needs to be regenerated with updated parameters.

**Parameters:**
- `c`: Configuration extracted from existing hash

**Returns:**
- `bool`: True if any parameter differs from current defaults

#### Argon2IdCreateHash
```go
func Argon2IdCreateHash(c *Argon2Config, password string) (string, error)
```
Low-level function to create an Argon2id hash.

**Parameters:**
- `c`: Argon2 configuration
- `password`: Password to hash

**Returns:**
- `string`: Complete hash string
- `error`: Error if hashing fails

#### Argon2IdComparePassword
```go
func Argon2IdComparePassword(password, hash string) (bool, *Argon2Config, error)
```
Low-level function to verify a password against a hash.

**Parameters:**
- `password`: Password to verify
- `hash`: Hash to compare against

**Returns:**
- `bool`: True if password matches
- `*Argon2Config`: Configuration used to create the hash
- `error`: Error if comparison fails

### Error Constants

```go
var (
    ErrInvalidHash         = utils.Error("argon2id: hash is not in the correct format")
    ErrIncompatibleVersion = utils.Error("argon2id: incompatible version of argon2")
)
```

## Usage Examples

### Basic Password Hashing

```go
import (
    "github.com/oddbit-project/blueprint/crypt/hashing"
    "log"
)

func basicExample() {
    // Create hasher with default configuration
    hasher, err := hashing.NewArgon2Hasher(nil)
    if err != nil {
        log.Fatal(err)
    }

    // Hash a password
    password := "mySecurePassword123!"
    hash, err := hasher.Generate(password)
    if err != nil {
        log.Fatal(err)
    }

    log.Printf("Generated hash: %s", hash)
    // Output: $argon2id$v=19$m=65536,t=4,p=8$[salt]$[hash]

    // Verify password
    valid, rehashFn, err := hasher.Verify(password, hash)
    if err != nil {
        log.Fatal(err)
    }

    if valid {
        log.Println("Password is correct!")
        if rehashFn != nil {
            log.Println("Hash needs updating")
        }
    }
}
```

### Custom Configuration

```go
func customConfigExample() {
    // Create custom configuration
    config := &hashing.Argon2Config{
        Memory:      32 * 1024, // 32MB
        Iterations:  3,
        Parallelism: 4,
        SaltLength:  16,
        KeyLength:   32,
    }

    hasher, err := hashing.NewArgon2Hasher(config)
    if err != nil {
        log.Fatal(err)
    }

    // Use as normal
    hash, err := hasher.Generate("password123")
    if err != nil {
        log.Fatal(err)
    }

    log.Printf("Custom hash: %s", hash)
}
```

### Handling Password Rehashing

```go
func rehashingExample() {
    hasher, _ := hashing.NewArgon2Hasher(nil)
    
    // Simulate old hash with different parameters
    oldHash := "$argon2id$v=19$m=32768,t=2,p=4$abcdefghijklmnop$..."
    password := "userPassword"

    // Verify password
    valid, rehashFn, err := hasher.Verify(password, oldHash)
    if err != nil {
        log.Fatal(err)
    }

    if !valid {
        log.Println("Invalid password")
        return
    }

    // Check if rehashing is needed
    if rehashFn != nil {
        // Generate new hash with updated parameters
        newHash, err := rehashFn()
        if err != nil {
            log.Printf("Rehashing failed: %v", err)
            return
        }

        // Update stored hash in database
        updateUserPassword(newHash)
        log.Println("Password hash updated successfully")
    }
}
```

### User Registration Flow

```go
type User struct {
    Username string
    Password string // This will store the hash
}

func registerUser(username, password string) error {
    // Validate password strength
    if len(password) < 8 {
        return fmt.Errorf("password too short")
    }

    // Create hasher
    hasher, err := hashing.NewArgon2Hasher(nil)
    if err != nil {
        return err
    }

    // Generate password hash
    hash, err := hasher.Generate(password)
    if err != nil {
        return fmt.Errorf("failed to hash password: %w", err)
    }

    // Store user with hashed password
    user := &User{
        Username: username,
        Password: hash,
    }

    // Save to database
    return saveUser(user)
}
```

### User Login Flow

```go
func loginUser(username, password string) (*User, error) {
    // Fetch user from database
    user, err := getUserByUsername(username)
    if err != nil {
        return nil, fmt.Errorf("user not found")
    }

    // Create hasher
    hasher, err := hashing.NewArgon2Hasher(nil)
    if err != nil {
        return nil, err
    }

    // Verify password
    valid, rehashFn, err := hasher.Verify(password, user.Password)
    if err != nil {
        return nil, fmt.Errorf("password verification failed: %w", err)
    }

    if !valid {
        return nil, fmt.Errorf("invalid password")
    }

    // Handle rehashing if needed
    if rehashFn != nil {
        newHash, err := rehashFn()
        if err == nil {
            user.Password = newHash
            updateUser(user) // Update hash in database
        }
    }

    return user, nil
}
```

### Error Handling

```go
func errorHandlingExample() {
    hasher, _ := hashing.NewArgon2Hasher(nil)
    
    // Handle invalid hash format
    _, _, err := hasher.Verify("password", "invalid-hash-format")
    if err != nil {
        switch err {
        case hashing.ErrInvalidHash:
            log.Println("Hash format is invalid")
        case hashing.ErrIncompatibleVersion:
            log.Println("Hash was created with incompatible Argon2 version")
        default:
            log.Printf("Unexpected error: %v", err)
        }
    }
    
    // Handle empty password
    _, err = hasher.Generate("")
    if err != nil {
        log.Printf("Empty password handling: %v", err)
    }
}
```

## Configuration Guide

### Security Parameters

The default configuration provides strong security suitable for most applications:

| Parameter | Default | Description | Security Impact |
|-----------|---------|-------------|-----------------|
| Memory | 64 MB | Memory usage per hash | Higher = more secure against GPU attacks |
| Iterations | 4 | Number of passes | Higher = slower brute force |
| Parallelism | CPU cores | Threads to use | Higher = faster hashing |
| Salt Length | 16 bytes | Random salt size | 16 bytes provides 128 bits of entropy |
| Key Length | 32 bytes | Output hash size | 32 bytes = 256 bits of security |

### Choosing Parameters

#### For Standard Web Applications
```go
// Use defaults - balanced security and performance
hasher, _ := hashing.NewArgon2Hasher(nil)
```

#### For High-Security Applications
```go
config := &hashing.Argon2Config{
    Memory:      128 * 1024, // 128MB
    Iterations:  6,
    Parallelism: 4,
    SaltLength:  16,
    KeyLength:   32,
}
hasher, _ := hashing.NewArgon2Hasher(config)
```

#### For Resource-Constrained Environments
```go
config := &hashing.Argon2Config{
    Memory:      32 * 1024, // 32MB
    Iterations:  3,
    Parallelism: 2,
    SaltLength:  16,
    KeyLength:   32,
}
hasher, _ := hashing.NewArgon2Hasher(config)
```

### Parameter Guidelines

1. **Memory**: 
   - Minimum: 19 MB (OWASP recommendation)
   - Default: 64 MB (good balance)
   - High security: 128 MB or more

2. **Iterations**:
   - Minimum: 2
   - Default: 4
   - High security: 5-10

3. **Parallelism**:
   - Set based on available CPU cores
   - Usually 2-8 threads

4. **Salt Length**:
   - Never less than 16 bytes
   - 16 bytes = 128 bits of randomness

5. **Key Length**:
   - Minimum: 32 bytes (256 bits)
   - No benefit to going higher

## Best Practices

### Development Environment

```go
func developmentSetup() PasswordHasher {
    // Use lower parameters for faster tests
    config := &hashing.Argon2Config{
        Memory:      16 * 1024, // 16MB for speed
        Iterations:  2,
        Parallelism: 2,
        SaltLength:  16,
        KeyLength:   32,
    }
    
    hasher, _ := hashing.NewArgon2Hasher(config)
    return hasher
}
```

### Production Environment

```go
func productionSetup() PasswordHasher {
    // Use strong defaults or higher
    hasher, _ := hashing.NewArgon2Hasher(nil)
    
    // Consider monitoring hash generation time
    start := time.Now()
    _, err := hasher.Generate("test")
    if err == nil {
        log.Printf("Hash generation took: %v", time.Since(start))
    }
    
    return hasher
}
```

### Security Considerations

1. **Never store plaintext passwords**
   ```go
   // WRONG
   user.Password = request.Password
   
   // CORRECT
   hash, _ := hasher.Generate(request.Password)
   user.Password = hash
   ```

2. **Always handle rehashing**
   ```go
   valid, rehashFn, _ := hasher.Verify(password, hash)
   if valid && rehashFn != nil {
       newHash, _ := rehashFn()
       updateUserPassword(userID, newHash)
   }
   ```

3. **Rate limit authentication attempts**
   ```go
   // Implement rate limiting to prevent brute force
   if rateLimiter.TooManyAttempts(username) {
       return errors.New("too many login attempts")
   }
   ```

4. **Clear sensitive data**
   ```go
   password := getPasswordFromRequest()
   defer func() {
       // Clear password from memory
       for i := range password {
           password = strings.Replace(password, string(password[i]), "", 1)
       }
   }()
   ```

## Migration Guide

### From bcrypt

```go
func migrateFromBcrypt(bcryptHash string, plainPassword string) (string, error) {
    // Verify with bcrypt
    err := bcrypt.CompareHashAndPassword([]byte(bcryptHash), []byte(plainPassword))
    if err != nil {
        return "", err
    }
    
    // Generate new Argon2 hash
    hasher, _ := hashing.NewArgon2Hasher(nil)
    return hasher.Generate(plainPassword)
}
```

### From PBKDF2

```go
func migrateFromPBKDF2(pbkdf2Hash string, plainPassword string) (string, error) {
    // Verify PBKDF2 (implementation specific)
    if !verifyPBKDF2(pbkdf2Hash, plainPassword) {
        return "", errors.New("invalid password")
    }
    
    // Generate new Argon2 hash
    hasher, _ := hashing.NewArgon2Hasher(nil)
    return hasher.Generate(plainPassword)
}
```

### Gradual Migration Strategy

```go
type HashType string

const (
    HashTypeBcrypt  HashType = "bcrypt"
    HashTypeArgon2  HashType = "argon2"
)

func verifyAndMigrate(password, storedHash string, hashType HashType) (bool, string, error) {
    switch hashType {
    case HashTypeBcrypt:
        // Verify old hash
        err := bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(password))
        if err != nil {
            return false, "", err
        }
        
        // Generate new hash
        hasher, _ := hashing.NewArgon2Hasher(nil)
        newHash, err := hasher.Generate(password)
        return true, newHash, err
        
    case HashTypeArgon2:
        // Use normal verification
        hasher, _ := hashing.NewArgon2Hasher(nil)
        valid, rehashFn, err := hasher.Verify(password, storedHash)
        if err != nil || !valid {
            return false, "", err
        }
        
        if rehashFn != nil {
            newHash, err := rehashFn()
            return true, newHash, err
        }
        
        return true, storedHash, nil
        
    default:
        return false, "", errors.New("unknown hash type")
    }
}
```

## Performance Optimization

### Benchmarking

```go
func benchmarkHashingParams() {
    configs := []struct {
        name   string
        config *hashing.Argon2Config
    }{
        {"16MB", &hashing.Argon2Config{Memory: 16384, Iterations: 3, Parallelism: 4}},
        {"32MB", &hashing.Argon2Config{Memory: 32768, Iterations: 3, Parallelism: 4}},
        {"64MB", &hashing.Argon2Config{Memory: 65536, Iterations: 4, Parallelism: 4}},
    }
    
    password := "benchmarkPassword123"
    
    for _, tc := range configs {
        hasher, _ := hashing.NewArgon2Hasher(tc.config)
        
        start := time.Now()
        _, err := hasher.Generate(password)
        duration := time.Since(start)
        
        if err != nil {
            log.Printf("%s: error: %v", tc.name, err)
        } else {
            log.Printf("%s: %v", tc.name, duration)
        }
    }
}
```

### Caching Considerations

```go
// DON'T cache password hashes in memory
// Each verification should read from persistent storage

// DO implement rate limiting
type LoginAttemptCache struct {
    attempts map[string][]time.Time
    mu       sync.RWMutex
}

func (c *LoginAttemptCache) recordAttempt(username string) bool {
    c.mu.Lock()
    defer c.mu.Unlock()
    
    now := time.Now()
    attempts := c.attempts[username]
    
    // Remove attempts older than 15 minutes
    validAttempts := []time.Time{}
    for _, t := range attempts {
        if now.Sub(t) < 15*time.Minute {
            validAttempts = append(validAttempts, t)
        }
    }
    
    validAttempts = append(validAttempts, now)
    c.attempts[username] = validAttempts
    
    // Allow max 5 attempts per 15 minutes
    return len(validAttempts) <= 5
}
```

## Troubleshooting

### Common Issues

#### High Memory Usage

**Problem:** Application uses too much memory during authentication.

**Solution:** Reduce memory parameter or limit concurrent operations:
```go
// Use semaphore to limit concurrent hashing
sem := make(chan struct{}, 5) // Max 5 concurrent operations

func hashWithLimit(hasher PasswordHasher, password string) (string, error) {
    sem <- struct{}{}        // Acquire
    defer func() { <-sem }() // Release
    
    return hasher.Generate(password)
}
```

#### Slow Hash Generation

**Problem:** Hash generation takes too long.

**Solution:** Adjust parameters based on your security requirements:
```go
// Measure current performance
start := time.Now()
hash, _ := hasher.Generate("test")
duration := time.Since(start)

if duration > 500*time.Millisecond {
    // Consider reducing parameters
    log.Printf("Hash generation too slow: %v", duration)
}
```

#### Hash Format Errors

**Problem:** Getting "invalid hash format" errors.

**Solution:** Verify hash format and encoding:
```go
// Valid Argon2id hash format:
// $argon2id$v=19$m=65536,t=4,p=8$[base64-salt]$[base64-hash]

func validateHashFormat(hash string) error {
    parts := strings.Split(hash, "$")
    if len(parts) != 6 {
        return fmt.Errorf("expected 6 parts, got %d", len(parts))
    }
    
    if parts[1] != "argon2id" {
        return fmt.Errorf("not an argon2id hash")
    }
    
    return nil
}
```

## Security Checklist

- [ ] Never store plaintext passwords
- [ ] Use default parameters or stronger
- [ ] Implement rate limiting for login attempts
- [ ] Handle rehashing when parameters change
- [ ] Use HTTPS for password transmission
- [ ] Implement proper session management after login
- [ ] Log authentication failures (without passwords)
- [ ] Regular security audits of authentication flow
- [ ] Monitor hash generation performance
- [ ] Plan for parameter upgrades as hardware improves