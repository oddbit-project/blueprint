# JWT Authentication Demo

This sample demonstrates how to use Blueprint's JWT authentication system with the new `provider/auth/jwt` package.

## Features Demonstrated

- **JWT Token Management**: Secure token generation, validation, and refresh using RSA-256
- **Session Integration**: JWT tokens work seamlessly with Blueprint's session system
- **Token Revocation**: Ability to revoke tokens and maintain a revocation list
- **JWKS Support**: Public key distribution via JSON Web Key Set endpoint (RSA keys)
- **Role-based Access**: Different endpoints require different permissions
- **Security Best Practices**: Asymmetric signing, proper issuer/audience validation
- **Interactive Web Interface**: Easy-to-use web UI for testing all features

## Quick Start

1. **Run the demo:**
   ```bash
   cd samples/jwt-auth
   go run main.go
   ```

2. **Open your browser and visit http://localhost:8092**

3. **Use the interactive web interface to test all features!**

## Web Interface

The demo includes a user-friendly web interface that allows you to:

- **Login/Logout**: Test authentication with the provided credentials
- **View Profile**: See authenticated user information
- **Refresh Tokens**: Test JWT token rotation
- **Admin Functions**: Access admin-only endpoints
- **Token Management**: Revoke tokens and see the effects
- **System Info**: Check server health and JWKS endpoints

### Default Credentials
- **Username**: `admin`
- **Password**: `password`

## Interactive Features

The web interface provides real-time feedback for:
- ✅ Authentication status indicator
- 🔐 JWT token display (with automatic updates)
- 📊 JSON response formatting
- 🎯 Error handling and display
- 🔄 Automatic token refresh capabilities

## API Endpoints

### Authentication
- `POST /login` - Authenticate with username/password
- `POST /logout` - Logout and clear session
- `POST /refresh` - Refresh JWT token
- `POST /revoke` - Revoke current token

### Protected Resources
- `GET /profile` - Get user profile (requires authentication)
- `GET /admin` - Admin dashboard (requires admin role)

### Utility
- `GET /health` - Health check
- `GET /.well-known/jwks.json` - JWKS endpoint for public keys

## Usage Examples

### 1. Login
```bash
curl -X POST http://localhost:8080/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"password"}'
```

Response:
```json
{
  "success": true,
  "message": "Login successful",
  "user_id": "123",
  "username": "admin",
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
}
```

### 2. Access Protected Resource
```bash
curl -X GET http://localhost:8080/profile \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
```

### 3. Refresh Token
```bash
curl -X POST http://localhost:8080/refresh \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
```

### 4. Admin Dashboard
```bash
curl -X GET http://localhost:8080/admin \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
```

### 5. Get JWKS (Public Keys)
```bash
curl -X GET http://localhost:8092/.well-known/jwks.json
```

Response:
```json
{
  "keys": [
    {
      "kty": "RSA",
      "kid": "demo-rsa-key",
      "use": "sig",
      "alg": "RS256",
      "n": "x22fLouRroOKTAJqlqWw...",
      "e": "AQAB"
    }
  ]
}
```

## Key Features

### JWT Configuration
- **Algorithm**: RSA-256 (RS256) with 2048-bit keys for enhanced security
- **Expiration**: 1 hour tokens
- **Claims Validation**: Mandatory issuer and audience validation
- **Token Rotation**: Automatic token refresh with rotation metadata
- **JWKS**: Public key distribution for token verification by third parties

### Security Features
- **Token Revocation**: In-memory revocation backend
- **JWKS Support**: Public key distribution for verification
- **Session Integration**: JWT tokens stored in session for easy access
- **Role-based Access**: Admin endpoints require specific roles

### Session Management
- **Stateless**: JWT tokens carry all session information
- **Automatic Headers**: Tokens automatically added to Authorization headers
- **Session Helpers**: Easy access to session data via Blueprint's session helpers

## Architecture

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   HTTP Client   │───▶│  JWT Middleware  │───▶│   Route Handler │
└─────────────────┘    └──────────────────┘    └─────────────────┘
                                │
                                ▼
                       ┌──────────────────┐
                       │   JWT Manager    │
                       └──────────────────┘
                                │
                    ┌───────────┼───────────┐
                    ▼           ▼           ▼
              ┌──────────┐ ┌─────────┐ ┌──────────┐
              │   JWKS   │ │ Session │ │Revocation│
              │ Manager  │ │ Manager │ │ Manager  │
              └──────────┘ └─────────┘ └──────────┘
```

## Configuration Options

The demo shows basic configuration. For production use, consider:

- **Asymmetric Algorithms**: Use RSA, ECDSA, or EdDSA for better security
- **Database Revocation**: Replace memory backend with persistent storage
- **Enhanced Security**: Enable device fingerprinting and IP validation
- **Environment Variables**: Load configuration from environment

## Next Steps

1. **Enhanced Security Demo**: Check `samples/jwt-auth-enhanced` for advanced security features
2. **Asymmetric Keys Demo**: See `samples/jwt-auth-rsa` for RSA/ECDSA examples
3. **Production Setup**: Review the main JWT documentation for production guidelines

## Credentials

For demo purposes:
- **Username**: `admin`
- **Password**: `password`

**⚠️ Important**: Change these credentials for any real application!