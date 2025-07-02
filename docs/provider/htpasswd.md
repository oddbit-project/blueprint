# blueprint.provider.htpasswd

Blueprint htpasswd provider for managing Apache-style password files.

## Overview

The htpasswd provider offers a comprehensive solution for managing user authentication files compatible with Apache's htpasswd format. It supports multiple hash algorithms, thread-safe operations, and provides both programmatic API and command-line tools.

Key features:
- Multiple hash algorithms (bcrypt, Apache MD5, SHA1/256/512, crypt, plaintext)
- Thread-safe container operations with mutex protection
- Apache htpasswd file format compatibility
- Comprehensive input validation (UTF-8, byte limits, forbidden characters)
- In-memory and file-based operations
- Command-line utility compatible with Apache htpasswd

## Supported Hash Algorithms

| Algorithm | Prefix | Security | Recommended |
|-----------|--------|----------|-------------|
| **bcrypt** | `$2a$`, `$2y$` | High | ✅ **Yes** (default) |
| **Apache MD5** | `$apr1$` | Medium | ⚠️ Legacy compatibility |
| **SHA256** | `{SHA256}` | Medium | ⚠️ No salt |
| **SHA512** | `{SHA512}` | Medium | ⚠️ No salt |
| **SHA1** | `{SHA}` | Low | ❌ Deprecated |
| **Crypt** | None (13 chars) | Low | ❌ Deprecated |
| **Plain** | None | None | ❌ Development only |

## Container API

### Basic Usage

```go
package main

import (
    "fmt"
    "os"
    "github.com/oddbit-project/blueprint/provider/htpasswd"
)

func main() {
    // Create new container
    container := htpasswd.NewContainer()
    
    // Add user with bcrypt (recommended)
    err := container.AddUserPassword("alice", "secret123")
    if err != nil {
        panic(err)
    }
    
    // Verify user password
    valid, err := container.VerifyUser("alice", "secret123")
    if err != nil {
        panic(err)
    }
    fmt.Printf("Password valid: %v\n", valid)
    
    // Save to file
    file, err := os.Create("users.htpasswd")
    if err != nil {
        panic(err)
    }
    defer file.Close()
    
    err = container.Write(file)
    if err != nil {
        panic(err)
    }
}
```

### Loading from File

```go
// Load existing htpasswd file
container, err := htpasswd.NewFromFile("/etc/apache2/.htpasswd")
if err != nil {
    panic(err)
}

// Or from any io.Reader
file, err := os.Open("users.htpasswd")
if err != nil {
    panic(err)
}
defer file.Close()

container, err = htpasswd.NewFromReader(file)
if err != nil {
    panic(err)
}
```

### User Management

```go
// Check if user exists
if container.UserExists("alice") {
    fmt.Println("User alice exists")
}

// Get user entry
entry, err := container.GetUser("alice")
if err != nil {
    panic(err)
}
fmt.Printf("Username: %s, Hash: %s\n", entry.Username, entry.Hash)

// List all users
users := container.ListUsers()
fmt.Printf("Total users: %d\n", container.Count())

// Delete user
err = container.DeleteUser("alice")
if err != nil {
    panic(err)
}
```

### Hash Algorithm Selection

```go
// Use specific hash algorithm
err := container.AddUserWithHash("bob", "password", htpasswd.HashTypeBcrypt)
err = container.AddUserWithHash("charlie", "password", htpasswd.HashTypeApacheMD5)
err = container.AddUserWithHash("dave", "password", htpasswd.HashTypeSHA256)
```

## Hash Functions

### Direct Hash Operations

```go
// Generate hash
hash, err := htpasswd.HashPassword("mypassword", htpasswd.HashTypeBcrypt)
if err != nil {
    panic(err)
}
fmt.Printf("Generated hash: %s\n", hash)

// Verify password against hash
valid := htpasswd.VerifyPassword("mypassword", hash)
fmt.Printf("Password valid: %v\n", valid)

// Detect hash type
hashType := htpasswd.DetectHashType(hash)
fmt.Printf("Hash type: %v\n", hashType)
```

### Algorithm-Specific Functions

```go
// Bcrypt (recommended)
hash, err := htpasswd.HashBcrypt("password")
valid := htpasswd.VerifyBcrypt("password", hash)

// Apache MD5
hash, err = htpasswd.HashApacheMD5("password")
valid = htpasswd.VerifyApacheMD5("password", hash)

// SHA variants
hash, err = htpasswd.HashSHA256("password")
valid = htpasswd.VerifySHA256("password", hash)
```

## Input Validation

The provider includes comprehensive validation:

```go
// Username validation
err := htpasswd.ValidateUsername("alice")      // ✅ Valid
err = htpasswd.ValidateUsername("")            // ❌ Empty
err = htpasswd.ValidateUsername("user:name")   // ❌ Contains colon
err = htpasswd.ValidateUsername(string(make([]byte, 256))) // ❌ > 255 bytes

// Password validation  
err = htpasswd.ValidatePassword("secret123")   // ✅ Valid
err = htpasswd.ValidatePassword("")            // ❌ Empty
err = htpasswd.ValidatePassword("\xFF\xFE")    // ❌ Invalid UTF-8
```

### Validation Rules

**Username constraints:**
- Must not be empty (after trimming whitespace)
- Cannot contain colon (`:`) character
- Maximum 255 bytes length
- Must be valid UTF-8

**Password constraints:**
- Must not be empty
- Must be valid UTF-8

## Command-Line Tool

The included `htpasswd` command-line tool provides Apache compatibility:

### Installation

```bash
cd sample/htpasswd
go build -o htpasswd main.go
```

### Usage Examples

```bash
# Create new file with user
./htpasswd -c users.htpasswd alice

# Add user to existing file
./htpasswd users.htpasswd bob

# Batch mode (scripting)
./htpasswd -b users.htpasswd charlie password123

# Use specific algorithm
./htpasswd -b -B sha256 users.htpasswd dave secret

# Delete user
./htpasswd -D users.htpasswd alice

# Verify password
./htpasswd -v users.htpasswd bob
```

### Command-Line Options

| Option | Description |
|--------|-------------|
| `-c` | Create a new file |
| `-D` | Delete the specified user |
| `-v` | Verify password for the specified user |
| `-b` | Use batch mode (password on command line) |
| `-B algorithm` | Force hash algorithm (bcrypt, apr1, sha, sha256, sha512, crypt, plain) |
| `-h` | Show help message |
| `-version` | Show version information |

## Security Considerations

### Best Practices

1. **Use bcrypt** - Default and recommended for new implementations
2. **Validate inputs** - Always validate usernames and passwords
3. **Secure file permissions** - Set appropriate file permissions (600 or 644)
4. **Regular updates** - Update weak hashes to stronger algorithms

### Security Features

- **Timing attack resistance** - Uses `crypto/subtle.ConstantTimeCompare`
- **Thread safety** - All operations are mutex-protected
- **Input validation** - Comprehensive UTF-8 and constraint checking
- **Secure defaults** - bcrypt with default cost factor

### Migration Example

```go
// Migrate from SHA1 to bcrypt
container, err := htpasswd.NewFromFile("legacy.htpasswd")
if err != nil {
    panic(err)
}

for _, username := range container.ListUsers() {
    entry, _ := container.GetUser(username)
    
    // Check if using weak hash
    if htpasswd.DetectHashType(entry.Hash) == htpasswd.HashTypeSHA1 {
        fmt.Printf("User %s uses weak hash, manual password reset required\n", username)
        // Note: Cannot migrate without knowing original password
    }
}
```

## File Format

Standard Apache htpasswd format:
```
username1:$2y$10$abcdefghijklmnopqrstuvwxyz0123456789
username2:{SHA}5baa61e4c9b93f3f0682250b6cf8331b7ee68fd8
username3:$apr1$salt$hash
```

## Error Handling

```go
// Common error scenarios
container := htpasswd.NewContainer()

// Handle validation errors
err := container.AddUser("user:invalid", "password")
if err != nil {
    fmt.Printf("Validation error: %v\n", err)
}

// Handle file errors
_, err = htpasswd.NewFromFile("nonexistent.htpasswd")
if err != nil {
    fmt.Printf("File error: %v\n", err)
}

// Handle user not found
_, err = container.GetUser("missing")
if err != nil {
    fmt.Printf("User not found: %v\n", err)
}
```

## Performance Notes

- **Thread-safe operations** incur minimal overhead
- **bcrypt** is computationally expensive by design (security feature)
- **File operations** are I/O bound
- **In-memory operations** are very fast for user lookups

## Testing

The provider includes comprehensive test coverage:

```bash
# Run all tests
go test -v ./provider/htpasswd/...

# Check coverage
go test -cover ./provider/htpasswd/...

# Run with race detection
go test -race ./provider/htpasswd/...
```

## Integration Examples

### Web Authentication

```go
func authenticateUser(username, password string) bool {
    container, err := htpasswd.NewFromFile("/etc/webapp/.htpasswd")
    if err != nil {
        log.Printf("Failed to load htpasswd: %v", err)
        return false
    }
    
    valid, err := container.VerifyUser(username, password)
    if err != nil {
        log.Printf("Authentication error: %v", err)
        return false
    }
    
    return valid
}
```

### User Registration

```go
func registerUser(username, password string) error {
    container, err := htpasswd.NewFromFile("/etc/webapp/.htpasswd")
    if err != nil {
        // Create new file if doesn't exist
        container = htpasswd.NewContainer()
    }
    
    if container.UserExists(username) {
        return fmt.Errorf("user already exists")
    }
    
    err = container.AddUserPassword(username, password)
    if err != nil {
        return fmt.Errorf("failed to add user: %w", err)
    }
    
    // Save back to file
    file, err := os.OpenFile("/etc/webapp/.htpasswd", 
        os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
    if err != nil {
        return fmt.Errorf("failed to open file: %w", err)
    }
    defer file.Close()
    
    return container.Write(file)
}
```