# TLS Provider

The TLS provider offers enhanced security for client and server connections by providing robust TLS configuration options. It supports certificate verification, custom cipher suites, and secure defaults.

## Client Configuration

The `ClientConfig` struct provides options for configuring TLS on clients:

```go
import (
    "github.com/oddbit-project/blueprint/provider/tls"
)

// Create a new client configuration
clientConfig := &tls.ClientConfig{
    TLSCA: "/path/to/ca.crt",      // Root CA certificate for verifying server
    TLSCert: "/path/to/client.crt", // Client certificate for mutual TLS
    TLSKey: "/path/to/client.key",  // Client private key
    TLSEnable: true,                // Enable TLS
    TLSInsecureSkipVerify: false,   // Verify server certificate (recommended)
}

// For encrypted keys, set the key password
clientConfig.TlsKeyCredential.Key = "keypassword"
// Or use environment variables
clientConfig.TlsKeyCredential.KeyEnvVar = "KEY_PASSWORD"
// Or use a file
clientConfig.TlsKeyCredential.KeyFile = "/path/to/keypassword.txt"

// Generate the TLS configuration
tlsConfig, err := clientConfig.TLSConfig()
if err != nil {
    // handle error
}

// Use tlsConfig with your client implementation
// ...
```

## Server Configuration

The `ServerConfig` struct provides options for configuring TLS on servers with enhanced security features:

```go
import (
    "github.com/oddbit-project/blueprint/provider/tls"
)

// Create a new server configuration
serverConfig := &tls.ServerConfig{
    TLSCert: "/path/to/server.crt",                    // Server certificate
    TLSKey: "/path/to/server.key",                     // Server private key
    TLSAllowedCACerts: []string{"/path/to/ca.crt"},    // CA certs for client verification
    TLSCipherSuites: []string{"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384"}, // Custom cipher suites
    TLSMinVersion: "1.3",                              // Minimum TLS version
    TLSMaxVersion: "1.3",                              // Maximum TLS version
    TLSAllowedDNSNames: []string{"client.example.com"}, // Allowed client cert names
    TLSEnable: true,                                   // Enable TLS
}

// For encrypted keys, set the key password
serverConfig.TlsKeyCredential.Key = "keypassword"
// Or use environment variables
serverConfig.TlsKeyCredential.KeyEnvVar = "KEY_PASSWORD"
// Or use a file
serverConfig.TlsKeyCredential.KeyFile = "/path/to/keypassword.txt"

// Generate the TLS configuration
tlsConfig, err := serverConfig.TLSConfig()
if err != nil {
    // handle error
}

// Use tlsConfig with your server implementation
// ...
```

## Security Features

### Enhanced Certificate Verification

The server configuration includes advanced certificate verification that checks:

- Certificate validity dates
- Allowed DNS names in client certificates
- Certificate integrity

### Secure Defaults

- TLS 1.3 is used by default for both clients and servers
- Strong cipher suites are preferred
- Client authentication is properly enforced when enabled

### Mutual TLS Support

Both client and server configurations support mutual TLS authentication, where:

- Servers verify client certificates
- Clients verify server certificates

## Best Practices

1. Always use TLS 1.3 when possible
2. Avoid using `TLSInsecureSkipVerify: true` in production
3. Regularly rotate certificates
4. Protect private keys with strong passwords
5. Use mutual TLS for sensitive services