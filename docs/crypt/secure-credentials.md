# Secure Credentials

The secure credentials system in Blueprint provides a way to handle sensitive information like passwords securely in memory. It encrypts credentials using AES-GCM and provides methods to safely store, retrieve, and clear sensitive data.

## Features

- In-memory encryption of sensitive data
- Support for loading credentials from environment variables, files, or configuration objects
- Secure credential clearing (zeroing memory)
- Thread-safe operations

## Usage

### Creating a New Credential

```go
import (
    "github.com/oddbit-project/blueprint/crypt/secure"
)

// Generate a new encryption key
key, err := secure.GenerateKey()
if err != nil {
    // handle error
}

// Create a new credential with a plaintext password
credential, err := secure.NewCredential("my-secure-password", key, false)
if err != nil {
    // handle error
}

// Get the plaintext (only when needed)
plaintext, err := credential.Get()
if err != nil {
    // handle error
}

// Use the plaintext and then clear it from memory
// ...

// When done with the credential, clear it
credential.Clear()
```

### Loading Credentials from Different Sources

```go
// From environment variable
envCredential, err := secure.CredentialFromEnv("APP_PASSWORD", key, false)

// From file
fileCredential, err := secure.CredentialFromFile("/path/to/secrets.txt", key, false)

// Using configuration object
config := &secure.DefaultCredentialConfig{
    Password: "direct-password",  // Highest priority
    PasswordEnvVar: "APP_PASSWORD", // Second priority
    PasswordFile: "/path/to/secrets.txt", // Lowest priority
}

configCredential, err := secure.CredentialFromConfig(config, key, false)
```

## Configuration

The `DefaultCredentialConfig` struct provides a standard way to configure credential sources with priority handling:

```go
type DefaultCredentialConfig struct {
    Password       string `json:"password"`       // Highest priority
    PasswordEnvVar string `json:"passwordEnvVar"` // Second priority 
    PasswordFile   string `json:"passwordFile"`   // Lowest priority
}
```

You can also implement your own configuration by implementing the `CredentialConfig` interface:

```go
type CredentialConfig interface {
    GetPassword() string
    GetEnvVar() string
    GetFileName() string
}
```

## Security Best Practices

1. Generate unique encryption keys for each application instance
2. Only decrypt credentials when absolutely necessary
3. Clear plaintext credentials from memory as soon as possible
4. Consider using hardware tokens or secure key management services for encryption keys
5. For environment variables, they are read once and then overwritten for added security