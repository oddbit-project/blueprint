# Secure Credentials

The secure credentials system in Blueprint provides a way to handle sensitive information like passwords securely in memory. 
It encrypts credentials using AES-256-GCM and provides methods to safely store, retrieve, and clear sensitive data with thread-safe operations.

## Features

- **In-memory encryption** of sensitive data using AES-256-GCM
- **Multiple loading sources**: environment variables, files, or configuration objects
- **Secure memory clearing** with explicit zeroing of sensitive data
- **Thread-safe operations** with RWMutex for concurrent access
- **Flexible configuration** with priority-based credential resolution
- **Key management utilities** for generation, encoding, and decoding

## Complete API Reference

### Core Types

#### Credential

The main structure for storing encrypted credentials in memory.

```go
type Credential struct {
    // Internal fields (not directly accessible)
}
```

**Methods:**

##### NewCredential
```go
func NewCredential(data []byte, encryptionKey []byte, allowEmpty bool) (*Credential, error)
```
Creates a new secure credential container.

**Parameters:**
- `data`: The sensitive data to encrypt (password, secret, etc.)
- `encryptionKey`: 32-byte encryption key for AES-256
- `allowEmpty`: Whether to allow empty credentials

**Returns:**
- `*Credential`: New credential instance
- `error`: ErrEmptyCredential if data is empty and allowEmpty is false, ErrInvalidKey if key is not 32 bytes

##### Get
```go
func (sc *Credential) Get() (string, error)
```
Decrypts and returns the credential as a string.

**Returns:**
- `string`: Decrypted credential value
- `error`: ErrEmptyCredential if credential is empty, ErrDecryption if decryption fails

##### GetBytes
```go
func (sc *Credential) GetBytes() ([]byte, error)
```
Decrypts and returns the raw credential bytes. Use this method sparingly to minimize exposure of sensitive data in memory.

**Returns:**
- `[]byte`: Decrypted credential bytes
- `error`: ErrEmptyCredential if credential is empty, ErrDecryption if decryption fails

##### Update
```go
func (sc *Credential) Update(plaintext string) error
```
Updates the credential with a new plaintext value.

**Parameters:**
- `plaintext`: New credential value

**Returns:**
- `error`: ErrEncryption if encryption fails

##### UpdateBytes
```go
func (sc *Credential) UpdateBytes(data []byte) error
```
Updates the credential with new byte data.

**Parameters:**
- `data`: New credential data

**Returns:**
- `error`: ErrEncryption if encryption fails

##### Clear
```go
func (sc *Credential) Clear()
```
Zeroes out all sensitive data from memory. Call this when the credential is no longer needed.

##### IsEmpty
```go
func (sc *Credential) IsEmpty() bool
```
Returns true if the credential is empty.

**Returns:**
- `bool`: True if credential contains no data

### Configuration Types

#### DefaultCredentialConfig
```go
type DefaultCredentialConfig struct {
    Password       string `json:"password"`       // Direct password (highest priority)
    PasswordEnvVar string `json:"passwordEnvVar"` // Environment variable name (second priority)
    PasswordFile   string `json:"passwordFile"`   // File path (lowest priority)
}
```

Standard configuration structure with priority-based credential resolution.

**Methods:**
- `Fetch() (string, error)`: Retrieves credential from configured source
- `IsEmpty() bool`: Returns true if all fields are empty

#### KeyConfig
```go
type KeyConfig struct {
    Key       string `json:"key"`       // Direct key value (highest priority)
    KeyEnvVar string `json:"keyEnvVar"` // Environment variable name (second priority)
    KeyFile   string `json:"keyFile"`   // File path (lowest priority)
}
```

Similar to DefaultCredentialConfig but for key management scenarios.

**Methods:**
- `Fetch() (string, error)`: Retrieves key from configured source
- `IsEmpty() bool`: Returns true if all fields are empty

#### CredentialConfig Interface
```go
type CredentialConfig interface {
    Fetch() (string, error)
    IsEmpty() bool
}
```

Interface for implementing custom credential configuration sources.

### Factory Functions

#### CredentialFromEnv
```go
func CredentialFromEnv(envName string, encryptionKey []byte, allowEmpty bool) (*Credential, error)
```
Creates a credential from an environment variable.

**Parameters:**
- `envName`: Environment variable name
- `encryptionKey`: 32-byte encryption key
- `allowEmpty`: Whether to allow empty values

**Returns:**
- `*Credential`: New credential instance
- `error`: ErrEmptyCredential if variable is empty and allowEmpty is false

#### CredentialFromFile
```go
func CredentialFromFile(filename string, encryptionKey []byte, allowEmpty bool) (*Credential, error)
```
Creates a credential from a file.

**Parameters:**
- `filename`: Path to secrets file
- `encryptionKey`: 32-byte encryption key
- `allowEmpty`: Whether to allow empty files

**Returns:**
- `*Credential`: New credential instance
- `error`: ErrSecretsFileNotFound if file doesn't exist, ErrEmptyCredential if file is empty and allowEmpty is false

#### CredentialFromConfig
```go
func CredentialFromConfig(cfg CredentialConfig, encryptionKey []byte, allowEmpty bool) (*Credential, error)
```
Creates a credential from a configuration object.

**Parameters:**
- `cfg`: Configuration implementing CredentialConfig interface
- `encryptionKey`: 32-byte encryption key
- `allowEmpty`: Whether to allow empty credentials

**Returns:**
- `*Credential`: New credential instance
- `error`: Configuration-specific errors or ErrEmptyCredential

### Utility Functions

#### GenerateKey
```go
func GenerateKey() ([]byte, error)
```
Generates a cryptographically secure 32-byte key for AES-256.

**Returns:**
- `[]byte`: 32-byte random key
- `error`: Error if random number generation fails

#### EncodeKey
```go
func EncodeKey(key []byte) string
```
Encodes a key as a base64 string for storage.

**Parameters:**
- `key`: Raw key bytes

**Returns:**
- `string`: Base64-encoded key

#### DecodeKey
```go
func DecodeKey(encodedKey string) ([]byte, error)
```
Decodes a base64-encoded key.

**Parameters:**
- `encodedKey`: Base64-encoded key string

**Returns:**
- `[]byte`: Decoded key bytes
- `error`: Error if decoding fails

#### RandomKey32
```go
func RandomKey32() []byte
```
Generates a random 32-byte key. Panics on error.

**Returns:**
- `[]byte`: 32-byte random key

#### RandomCredential
```go
func RandomCredential(l int) (*Credential, error)
```
Creates a credential with random data of specified length.

**Parameters:**
- `l`: Length of random data

**Returns:**
- `*Credential`: Credential with random data
- `error`: Error if random generation fails

### Low-Level Encryption API

For advanced use cases, you can use the AES256GCM encryption provider directly:

#### AES256GCM Interface
```go
type AES256GCM interface {
    Encrypt(data []byte) ([]byte, error)
    Decrypt(data []byte) ([]byte, error)
    Clear()
}
```

#### NewAES256GCM
```go
func NewAES256GCM(key []byte) (AES256GCM, error)
```
Creates a new AES-256-GCM encryption provider.

**Parameters:**
- `key`: 32-byte encryption key

**Returns:**
- `AES256GCM`: Encryption provider instance
- `error`: `ErrInvalidKeyLength` if key is not 32 bytes

**Example:**
```go
key := secure.RandomKey32()
cipher, err := secure.NewAES256GCM(key)
if err != nil {
    log.Fatal(err)
}
defer cipher.Clear()

// Encrypt data
plaintext := []byte("sensitive data")
ciphertext, err := cipher.Encrypt(plaintext)
if err != nil {
    log.Fatal(err)
}

// Decrypt data
decrypted, err := cipher.Decrypt(ciphertext)
if err != nil {
    log.Fatal(err)
}
```

### Error Constants

```go
var (
    ErrEncryption          = errors.New("encryption error")
    ErrDecryption          = errors.New("decryption error")
    ErrInvalidKey          = errors.New("invalid encryption key")
    ErrEmptyCredential     = errors.New("empty credential")
    ErrSecretsFileNotFound = errors.New("secrets file not found")
)
```

#### AES256GCM-Specific Errors

```go
var (
    ErrInvalidKeyLength    = errors.New("key length must be 32 bytes")
    ErrDataTooShort        = errors.New("data too short")
    ErrNonceExhausted      = errors.New("nonce counter exhausted, key rotation required")
    ErrAuthenticationFailed = errors.New("authentication failed")
)
```

## Enhanced Usage Examples

### Basic Credential Creation and Usage

```go
import (
    "github.com/oddbit-project/blueprint/crypt/secure"
    "log"
)

func basicCredentialExample() {
    // Generate a secure encryption key
    key, err := secure.GenerateKey()
    if err != nil {
        log.Fatalf("Failed to generate key: %v", err)
    }

    // Create a credential with sensitive data
    credential, err := secure.NewCredential([]byte("my-secret-password"), key, false)
    if err != nil {
        log.Fatalf("Failed to create credential: %v", err)
    }

    // Use the credential when needed
    password, err := credential.Get()
    if err != nil {
        log.Fatalf("Failed to get credential: %v", err)
    }

    // Use the password for authentication
    authenticateUser(password)

    // Clear the credential when done
    credential.Clear()
}
```

### Configuration-Based Credential Loading

```go
func configBasedExample() {
    // Generate or load encryption key
    key, _ := secure.GenerateKey()

    // Configuration with priority: direct > env var > file
    config := &secure.DefaultCredentialConfig{
        Password:       "",                    // Not set - check env var
        PasswordEnvVar: "DATABASE_PASSWORD",   // Check this env var
        PasswordFile:   "/etc/secrets/db.txt", // Fallback to file
    }

    // Create credential from configuration
    credential, err := secure.CredentialFromConfig(config, key, false)
    if err != nil {
        log.Fatalf("Failed to load credential: %v", err)
    }

    // Use credential
    dbPassword, err := credential.Get()
    if err != nil {
        log.Fatalf("Failed to get password: %v", err)
    }

    // Connect to database
    connectToDatabase("user", dbPassword, "localhost")

    // Clear when done
    credential.Clear()
}
```

### Proper Error Handling

```go
func errorHandlingExample() {
    key, err := secure.GenerateKey()
    if err != nil {
        log.Fatalf("Key generation failed: %v", err)
    }

    // Try to create credential with potential errors
    credential, err := secure.NewCredential([]byte(""), key, false)
    if err != nil {
        switch err {
        case secure.ErrEmptyCredential:
            log.Println("Credential is empty")
        case secure.ErrInvalidKey:
            log.Println("Invalid encryption key")
        default:
            log.Printf("Unexpected error: %v", err)
        }
        return
    }

    // Try to get credential value
    value, err := credential.Get()
    if err != nil {
        switch err {
        case secure.ErrDecryption:
            log.Println("Failed to decrypt credential")
        case secure.ErrEmptyCredential:
            log.Println("Credential is empty")
        default:
            log.Printf("Unexpected error: %v", err)
        }
        return
    }

    log.Printf("Successfully retrieved credential: %s", value)
}
```

### Memory Security Practices

```go
func memorySecurityExample() {
    key, _ := secure.GenerateKey()
    
    // Create credential
    credential, err := secure.NewCredential([]byte("sensitive-data"), key, false)
    if err != nil {
        log.Fatalf("Failed to create credential: %v", err)
    }

    // Minimize exposure time
    func() {
        // Get credential only when needed
        secret, err := credential.Get()
        if err != nil {
            return
        }

        // Use immediately
        result := performSecureOperation(secret)
        
        // Clear local variable (good practice)
        secret = ""
        
        processResult(result)
    }()

    // Always clear credential when done
    credential.Clear()
    
    // Clear key from memory
    for i := range key {
        key[i] = 0
    }
}
```

## Configuration Guide

### Priority Resolution

All configuration structures follow the same priority order:

1. **Direct value** (highest priority) - `Password` or `Key` field
2. **Environment variable** (second priority) - `PasswordEnvVar` or `KeyEnvVar` field
3. **File** (lowest priority) - `PasswordFile` or `KeyFile` field

### Environment Variable Handling

When using environment variables:
- Variables are read once and then cleared for security
- Empty variables are treated as not set
- The `env.SetEnvVar(envVar, "")` call clears the variable after reading

### File-Based Credentials

When using file-based credentials:
- Files must be readable by the application
- File contents are read as plaintext
- Leading/trailing whitespace is trimmed
- Empty files result in empty credentials

### Custom Configuration

Implement the `CredentialConfig` interface for custom sources:

```go
type DatabaseCredentialConfig struct {
    ConnectionString string
    QueryTimeout     time.Duration
}

func (c *DatabaseCredentialConfig) Fetch() (string, error) {
    // Custom logic to fetch credential from database
    return fetchFromDatabase(c.ConnectionString), nil
}

func (c *DatabaseCredentialConfig) IsEmpty() bool {
    return c.ConnectionString == ""
}

// Use with CredentialFromConfig
credential, err := secure.CredentialFromConfig(config, key, false)
```

## Troubleshooting

### Common Issues and Solutions

#### ErrInvalidKey - Invalid Encryption Key

**Problem:** Encryption key is not exactly 32 bytes.

**Symptoms:**
```go
credential, err := secure.NewCredential(data, key, false)
// err == secure.ErrInvalidKey
```

**Solutions:**
```go
// Generate proper 32-byte key
key, err := secure.GenerateKey()
if err != nil {
    // Handle generation error
}

// Or create from existing data
key := make([]byte, 32)
copy(key, []byte("your-key-data")) // Ensure exactly 32 bytes

// Verify key length before use
if len(key) != 32 {
    log.Fatal("Key must be exactly 32 bytes")
}
```

#### ErrSecretsFileNotFound - File Access Issues

**Problem:** Secrets file doesn't exist or isn't readable.

**Common causes:**
- File path is incorrect
- File permissions prevent reading
- File doesn't exist

**Debugging:**
```go
import "os"

func debugFileAccess(filename string) {
    // Check if file exists
    if _, err := os.Stat(filename); os.IsNotExist(err) {
        log.Printf("File does not exist: %s", filename)
        return
    }

    // Check if file is readable
    file, err := os.Open(filename)
    if err != nil {
        log.Printf("Cannot read file: %v", err)
        return
    }
    file.Close()

    log.Printf("File is accessible: %s", filename)
}
```

**Solutions:**
- Verify file path is absolute or relative to working directory
- Check file permissions: `chmod 644 /path/to/secrets.txt`
- Ensure file exists before application starts
- Use proper error handling for missing files

#### ErrEmptyCredential - Empty Credential Issues

**Problem:** Credential is empty when `allowEmpty=false`.

**Debugging:**
```go
func debugEmptyCredential(config *secure.DefaultCredentialConfig) {
    if config.IsEmpty() {
        log.Println("All configuration fields are empty")
        return
    }

    // Check each source
    if config.Password != "" {
        log.Println("Using direct password")
    } else if config.PasswordEnvVar != "" {
        envValue := os.Getenv(config.PasswordEnvVar)
        if envValue == "" {
            log.Printf("Environment variable %s is empty", config.PasswordEnvVar)
        } else {
            log.Printf("Environment variable %s has value", config.PasswordEnvVar)
        }
    } else if config.PasswordFile != "" {
        content, err := os.ReadFile(config.PasswordFile)
        if err != nil {
            log.Printf("Cannot read file %s: %v", config.PasswordFile, err)
        } else if len(content) == 0 {
            log.Printf("File %s is empty", config.PasswordFile)
        } else {
            log.Printf("File %s has content", config.PasswordFile)
        }
    }
}
```

#### Memory Management Issues

**Problem:** Sensitive data remains in memory longer than necessary.

**Best practices:**
```go
func properMemoryManagement() {
    key, _ := secure.GenerateKey()
    credential, _ := secure.NewCredential([]byte("secret"), key, false)

    // Minimize scope of sensitive data
    {
        secret, err := credential.Get()
        if err != nil {
            return
        }
        
        // Use secret immediately
        useSecret(secret)
        
        // Clear local variable
        secret = ""
    }

    // Clear credential when done
    credential.Clear()

    // Clear key
    for i := range key {
        key[i] = 0
    }
}
```

#### Concurrent Access Issues

**Problem:** Race conditions when accessing credentials from multiple goroutines.

**Solution:** The `Credential` type is thread-safe, but ensure proper usage:

```go
func concurrentAccess() {
    key, _ := secure.GenerateKey()
    credential, _ := secure.NewCredential([]byte("shared-secret"), key, false)

    var wg sync.WaitGroup
    
    // Multiple goroutines can safely read
    for i := 0; i < 10; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            secret, err := credential.Get()
            if err != nil {
                return
            }
            // Use secret
            processSecret(secret)
        }()
    }

    wg.Wait()
    
    // Clear once when all operations complete
    credential.Clear()
}
```

### Debugging Techniques

#### Enable Debug Logging

```go
import "log"

func debugCredentialCreation() {
    key, err := secure.GenerateKey()
    if err != nil {
        log.Printf("Key generation failed: %v", err)
        return
    }
    log.Printf("Generated key length: %d", len(key))

    data := []byte("test-password")
    log.Printf("Data length: %d", len(data))

    credential, err := secure.NewCredential(data, key, false)
    if err != nil {
        log.Printf("Credential creation failed: %v", err)
        return
    }
    log.Println("Credential created successfully")

    if credential.IsEmpty() {
        log.Println("Credential is empty")
    } else {
        log.Println("Credential contains data")
    }
}
```

#### Validate Configuration

```go
func validateConfiguration(config *secure.DefaultCredentialConfig) error {
    if config.IsEmpty() {
        return fmt.Errorf("configuration is completely empty")
    }

    // Check environment variable if specified
    if config.PasswordEnvVar != "" {
        if os.Getenv(config.PasswordEnvVar) == "" {
            return fmt.Errorf("environment variable %s is not set", config.PasswordEnvVar)
        }
    }

    // Check file if specified
    if config.PasswordFile != "" {
        if _, err := os.Stat(config.PasswordFile); err != nil {
            return fmt.Errorf("cannot access file %s: %v", config.PasswordFile, err)
        }
    }

    return nil
}
```

## Best Practices

### Development Environment

#### Simplified Configuration
```go
func developmentSetup() *secure.Credential {
    // Use simple, fixed key for development
    key := make([]byte, 32)
    copy(key, []byte("development-key-not-secure"))

    // Allow empty credentials for optional services
    config := &secure.DefaultCredentialConfig{
        Password: "dev-password", // Direct password for simplicity
    }

    credential, err := secure.CredentialFromConfig(config, key, true)
    if err != nil {
        log.Fatalf("Development credential setup failed: %v", err)
    }

    return credential
}
```

#### Development Best Practices
- Use fixed, non-random keys for consistent testing
- Allow empty credentials for optional services
- Store development secrets in easily accessible files
- Log credential operations for debugging
- Don't worry about memory clearing in development

### Staging Environment

#### Realistic Security Testing
```go
func stagingSetup() *secure.Credential {
    // Generate random key but store it for test repeatability
    keyFile := "/etc/staging/encryption.key"
    
    var key []byte
    if content, err := os.ReadFile(keyFile); err == nil {
        key, _ = secure.DecodeKey(string(content))
    } else {
        key, _ = secure.GenerateKey()
        encoded := secure.EncodeKey(key)
        os.WriteFile(keyFile, []byte(encoded), 0600)
    }

    // Use environment variables like production
    config := &secure.DefaultCredentialConfig{
        PasswordEnvVar: "STAGING_DB_PASSWORD",
    }

    credential, err := secure.CredentialFromConfig(config, key, false)
    if err != nil {
        log.Fatalf("Staging credential setup failed: %v", err)
    }

    return credential
}
```

#### Staging Best Practices
- Use realistic key generation and storage
- Test environment variable handling
- Validate all credential sources work correctly
- Test file permission scenarios
- Simulate production-like security constraints

### Production Environment

#### Maximum Security Configuration
```go
func productionSetup() *secure.Credential {
    // Load key from secure key management service or hardware token
    key := loadProductionKey()

    // Strict configuration - no direct passwords
    config := &secure.DefaultCredentialConfig{
        PasswordEnvVar: "PROD_SERVICE_PASSWORD",
        PasswordFile:   "/run/secrets/service_password", // Docker secrets or similar
    }

    credential, err := secure.CredentialFromConfig(config, key, false)
    if err != nil {
        log.Fatalf("Production credential setup failed: %v", err)
    }

    return credential
}

func loadProductionKey() []byte {
    // Example: Load from hardware security module
    // or cloud key management service
    keyData := os.Getenv("ENCRYPTION_KEY_B64")
    if keyData == "" {
        log.Fatal("ENCRYPTION_KEY_B64 environment variable required")
    }

    key, err := secure.DecodeKey(keyData)
    if err != nil {
        log.Fatalf("Invalid encryption key: %v", err)
    }

    return key
}
```

#### Production Best Practices
- Never use direct password fields in configuration
- Use secure key management services for encryption keys
- Implement key rotation procedures
- Clear credentials immediately after use
- Monitor for credential access failures
- Use file-based secrets for container orchestration
- Implement proper logging without exposing secrets

### Performance Optimization

#### Minimize Decryption Operations
```go
type ServiceWithCredentials struct {
    credential *secure.Credential
    cachedAuth string
    authExpiry time.Time
    mutex      sync.RWMutex
}

func (s *ServiceWithCredentials) getAuthToken() (string, error) {
    s.mutex.RLock()
    if time.Now().Before(s.authExpiry) && s.cachedAuth != "" {
        defer s.mutex.RUnlock()
        return s.cachedAuth, nil
    }
    s.mutex.RUnlock()

    // Need to refresh - acquire write lock
    s.mutex.Lock()
    defer s.mutex.Unlock()

    // Double-check after acquiring write lock
    if time.Now().Before(s.authExpiry) && s.cachedAuth != "" {
        return s.cachedAuth, nil
    }

    // Get fresh credential
    password, err := s.credential.Get()
    if err != nil {
        return "", err
    }

    // Authenticate and cache result
    token := authenticateAndGetToken(password)
    s.cachedAuth = token
    s.authExpiry = time.Now().Add(5 * time.Minute)

    return token, nil
}
```

### Security Guidelines

#### Key Management
- Generate unique keys per application instance
- Store keys in secure key management systems
- Never log or expose encryption keys
- Use hardware security modules when available

#### Memory Security
- Call `Clear()` on credentials when done
- Minimize lifetime of decrypted data
- Avoid storing credentials in variables longer than necessary
- Clear temporary variables containing sensitive data

#### Access Control
- Limit which code can access credentials
- Use dependency injection to control credential access
- Implement audit logging for credential operations
- Monitor for unauthorized access attempts

#### Error Handling
- Don't expose sensitive information in error messages
- Log errors appropriately without revealing secrets
- Implement proper fallback mechanisms
- Validate all inputs before creating credentials
