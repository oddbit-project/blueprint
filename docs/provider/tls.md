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
clientConfig.TlsKeyCredential.Password = "keypassword"
// Or use environment variables
clientConfig.TlsKeyCredential.PasswordEnvVar = "KEY_PASSWORD"
// Or use a file
clientConfig.TlsKeyCredential.PasswordFile = "/path/to/keypassword.txt"

// Reject TLS settings given without TLSEnable (call it from your config's Validate)
if err := clientConfig.ValidateEnabled(); err != nil {
    // err is tls.ErrTLSNotEnabled
}

// Generate the TLS configuration
tlsConfig, err := clientConfig.TLSConfig()
if err != nil {
    // handle error
}

// Use tlsConfig with your client implementation
// ...
```

`TLSConfig()` returns `nil, nil` when `TLSEnable` is false and ignores every other setting.
`ValidateEnabled()` returns `ErrTLSNotEnabled` when any of `TLSCA`, `TLSCert`, `TLSKey`,
`TLSInsecureSkipVerify` or a key password is set without `TLSEnable`, so a misconfiguration fails
instead of silently connecting without them (usually in plaintext); the providers that embed
`ClientConfig` call it from the `Validate()` of their configs (#116).

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
    TLSMinVersion: "TLS13",                            // Minimum TLS version ("TLS12" or "TLS13")
    TLSMaxVersion: "TLS13",                            // Maximum TLS version ("TLS12" or "TLS13")
    TLSAllowedDNSNames: []string{"client.example.com"}, // Allowed client cert names
    TLSEnable: true,                                   // Enable TLS
}

// For encrypted keys, set the key password
serverConfig.TlsKeyCredential.Password = "keypassword"
// Or use environment variables
serverConfig.TlsKeyCredential.PasswordEnvVar = "KEY_PASSWORD"
// Or use a file
serverConfig.TlsKeyCredential.PasswordFile = "/path/to/keypassword.txt"

// Generate the TLS configuration
tlsConfig, err := serverConfig.TLSConfig()
if err != nil {
    // handle error
}

// Use tlsConfig with your server implementation
// ...
```

`TLSMinVersion` and `TLSMaxVersion` accept `"TLS12"` or `"TLS13"`; any other value (including `"1.2"` or `"1.3"`)
makes `TLSConfig()` return `ErrInvalidTlsVersion`. `TLSCipherSuites` accepts the names of the supported AEAD suites
(for example `"TLS_AES_128_GCM_SHA256"` or `"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384"`); an unknown name returns
`ErrInvalidCipher`. Versions and cipher suites are validated and applied even when `TLSEnable` is set without any
certificate, key or CA files.

The key password is read from the first non-empty source: `Password`, then `PasswordEnvVar`, then `PasswordFile`.
An environment variable is read from the process environment and is not cleared after reading, so the password can be
fetched again (for example when the configuration is rebuilt).

## Security Features

### Enhanced Certificate Verification

The server configuration includes advanced certificate verification that checks:

- Certificate validity dates
- Allowed DNS names in client certificates
- Certificate integrity

This check is installed only when both `TLSAllowedCACerts` and `TLSAllowedDNSNames` are set. A client certificate is
accepted if **any** of its DNS names (Subject Alternative Names) is in `TLSAllowedDNSNames`; a certificate with no
matching DNS name (including one with no DNS SANs at all) is rejected with `ErrForbiddenDNS`. Expired or not yet valid
certificates are rejected with `ErrExpiredCert`.

### Secure Defaults

- Servers default to a minimum of TLS 1.3 when `TLSMinVersion` is empty; `ClientConfig` has no version fields and
  uses Go's client defaults
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