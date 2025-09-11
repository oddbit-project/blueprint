# Blueprint HTTP Server mTLS Demo

This sample demonstrates how to implement mutual TLS (mTLS) authentication using Blueprint's HTTP server provider. mTLS provides cryptographic authentication for both client and server, ensuring secure API-to-API communication.

## Overview

The demo includes:
- **Certificate Authority (CA)** for signing certificates
- **mTLS Server** requiring client certificate authentication
- **mTLS Client** with certificate-based authentication
- **Multiple endpoints** with different authorization levels

## Architecture

```
┌─────────────────┐     mTLS Connection     ┌─────────────────┐
│                 │ ◄──────────────────────► │                 │
│   mTLS Client   │   • Client Certificate   │   mTLS Server   │
│                 │   • Server Certificate   │                 │
│   client.crt    │   • Mutual Validation    │   server.crt    │
│   client.key    │                          │   server.key    │
└─────────────────┘                          └─────────────────┘
         │                                            │
         └─────────────── CA Certificate ─────────────┘
                            ca.crt
```

## Quick Start

### 1. Generate Certificates

```bash
# Generate CA, server, and client certificates
./generate-certs.sh
```

This creates:
- `certs/ca.crt` - Certificate Authority certificate
- `certs/ca.key` - Certificate Authority private key  
- `certs/server.crt` - Server certificate (valid for localhost, 127.0.0.1)
- `certs/server.key` - Server private key
- `certs/client.crt` - Client certificate (demo-client.example.com)
- `certs/client.key` - Client private key

### 2. Start the mTLS Server

```bash
cd server
go mod tidy
go run main.go
```

The server starts on `https://localhost:8444` with mTLS enabled.

### 3. Run the mTLS Client

In a new terminal:

```bash
cd client  
go mod tidy
go run main.go
```

The client will connect to the server using mTLS and test various endpoints.

## Endpoints

### Public Endpoints
- `GET /health` - Health check (no client certificate required)

### Protected Endpoints (require client certificate)
- `GET /secure` - Basic mTLS endpoint with client info
- `GET /api/v1/user/profile` - User profile data
- `POST /api/v1/data` - Data submission endpoint
- `GET /api/v1/admin/stats` - Admin statistics (requires admin privileges)

## mTLS Configuration

### Server Configuration

```go
serverConfig := &httpserver.ServerConfig{
    Host: "localhost",
    Port: 8443,
    ServerConfig: tlsProvider.ServerConfig{
        TLSEnable: true,
        TLSCert:   "../certs/server.crt",    // Server certificate
        TLSKey:    "../certs/server.key",    // Server private key
        TLSAllowedCACerts: []string{         // CA for client validation
            "../certs/ca.crt",
        },
        TLSAllowedDNSNames: []string{        // Allowed client DNS names
            "demo-client.example.com",
            "client.blueprint.demo",
        },
        TLSMinVersion: "1.3",                // Use TLS 1.3
        TLSMaxVersion: "1.3",
        TLSCipherSuites: []string{           // Strong cipher suites
            "TLS_AES_256_GCM_SHA384",
            "TLS_CHACHA20_POLY1305_SHA256",
            "TLS_AES_128_GCM_SHA256",
        },
    },
}
```

### Client Configuration

```go
// Load client certificate
clientCert, err := tls.LoadX509KeyPair("client.crt", "client.key")

// Load CA certificate for server validation
caCert, err := os.ReadFile("ca.crt")
caCertPool := x509.NewCertPool()
caCertPool.AppendCertsFromPEM(caCert)

// Configure TLS
tlsConfig := &tls.Config{
    Certificates: []tls.Certificate{clientCert},
    RootCAs:      caCertPool,
    MinVersion:   tls.VersionTLS13,
}

// Create HTTP client
client := &http.Client{
    Transport: &http.Transport{
        TLSClientConfig: tlsConfig,
    },
}
```

## Security Features

### Automatic Certificate Validation
- **Certificate expiration checking** - Rejects expired certificates
- **Certificate chain verification** - Validates CA signature  
- **DNS name validation** - Restricts allowed client DNS names
- **Custom authorization** - Organization-based access control

### Authorization Levels

1. **No Authentication** - Health endpoint
2. **Basic mTLS** - Client certificate required
3. **Organization-based** - Must be from "Blueprint Demo" organization
4. **Role-based** - Admin endpoints require specific OU

### Security Logging

The server logs detailed mTLS information:

```json
{
  "level": "info",
  "timestamp": "2024-01-15T10:30:45Z",
  "message": "mTLS request",
  "client_dn": "CN=demo-client.example.com,OU=Client,O=Blueprint Demo",
  "client_serial": "123456789",
  "path": "/api/v1/user/profile",
  "method": "GET",
  "status": 200,
  "client_ip": "127.0.0.1",
  "duration_ms": 15,
  "tls_version": "TLS 1.3",
  "cipher_suite": "TLS_AES_256_GCM_SHA384"
}
```

## Testing with curl

You can also test with curl:

```bash
# Health check (no client cert)
curl -k https://localhost:8443/health

# Secure endpoint with mTLS
curl -k \
  --cert certs/client.crt \
  --key certs/client.key \
  --cacert certs/ca.crt \
  https://localhost:8443/secure

# API endpoint with JSON data
curl -k \
  --cert certs/client.crt \
  --key certs/client.key \
  --cacert certs/ca.crt \
  -H "Content-Type: application/json" \
  -d '{"message": "Hello from curl!"}' \
  https://localhost:8443/api/v1/data
```

## Demo Output

Expected client output:

```
mTLS Client Demo
==================

1. Testing health endpoint (no client cert required)...
   Status: 200
   Response: {"data":{"server":"mTLS Demo Server","status":"healthy","timestamp":"2024-01-15T10:30:45Z"},"success":true}
   Health check passed

2. Testing secure endpoint (client cert required)...
   Status: 200
   Response: {"data":{"client_info":{...},"message":"Access granted to secure endpoint","timestamp":"2024-01-15T10:30:45Z"},"success":true}
   mTLS authentication successful

3. Testing user profile API...
   Status: 200
   Response: {"data":{"client_dn":"CN=demo-client.example.com,OU=Client,O=Blueprint Demo","email":"demo@example.com","privileges":["read","write"],"user_id":"demo_user_123","username":"demo_user"},"success":true}
   User profile retrieved successfully

4. Testing data submission API...
   Status: 200
   Response: {"data":{"client_dn":"CN=demo-client.example.com,OU=Client,O=Blueprint Demo","data_id":"data_1642248645","message":"Data processed successfully","received":{...}},"success":true}
   Data submitted successfully

5. Testing admin stats API...
   Status: 200
   Response: {"data":{"active_connections":42,"admin_client":"CN=demo-client.example.com,OU=Client,O=Blueprint Demo","memory_usage_mb":128,"uptime_seconds":3600},"success":true}
   Admin stats retrieved successfully

mTLS Demo completed successfully!
```

## Certificate Details

### Generated Certificates

- **CA Certificate**: 10-year validity, Blueprint Demo CA
- **Server Certificate**: 1-year validity, localhost + IP SANs
- **Client Certificate**: 1-year validity, demo client DNS names

### Certificate Validation

The server performs comprehensive certificate validation:

```go
func mTLSAuthorizationMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        // Check client certificate presence
        if c.Request.TLS == nil || len(c.Request.TLS.PeerCertificates) == 0 {
            response.Error(c, 401, "Client certificate required")
            return
        }

        clientCert := c.Request.TLS.PeerCertificates[0]

        // Validate expiration
        now := time.Now()
        if now.Before(clientCert.NotBefore) || now.After(clientCert.NotAfter) {
            response.Error(c, 401, "Client certificate expired")
            return
        }

        // Custom authorization
        if !isAuthorizedClient(clientCert) {
            response.Error(c, 403, "Client not authorized")
            return
        }

        c.Next()
    }
}
```

## Common Issues

### Certificate Errors

1. **"certificate signed by unknown authority"**
   - Ensure the CA certificate is properly loaded
   - Check the certificate chain

2. **"remote error: tls: bad certificate"**
   - Verify client certificate is valid
   - Check DNS names match configuration

3. **"connection refused"**
   - Ensure server is running on correct port
   - Check firewall settings

### Debugging

Enable detailed TLS logging:

```go
// Add to client
import "crypto/tls"

tlsConfig.ClientSessionCache = tls.NewLRUClientSessionCache(0)
// Disable session resumption for debugging
```

## Production Considerations

### Certificate Management
- Use proper certificate rotation
- Monitor certificate expiration
- Implement certificate revocation lists (CRL)
- Use hardware security modules (HSM) for CA keys

### Security Best Practices
- Use strong key sizes (4096-bit RSA or P-384 ECDSA)
- Implement proper certificate validation
- Log all authentication events
- Use role-based access control
- Regular security audits

### Performance
- Enable session resumption for better performance
- Use connection pooling
- Implement proper timeouts
- Monitor connection metrics

## Related Documentation

- [Blueprint HTTP Server Documentation](../../docs/provider/httpserver/)
- [Blueprint TLS Provider](../../docs/provider/tls.md)
- [Security Best Practices](../../docs/provider/httpserver/security.md)

## License

This sample is part of the Blueprint framework and is provided under the same license terms.