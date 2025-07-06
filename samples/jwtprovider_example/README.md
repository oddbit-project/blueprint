# JWT Provider Example

This example demonstrates how to implement JWT authentication with the Blueprint framework using the `jwtprovider` package. It showcases a complete JWT authentication system with stateful sessions, token revocation, and proper security practices.

## Features

- **JWT Authentication**: Secure token-based authentication with configurable expiration
- **Token Revocation**: Optional token revocation using memory backend
- **Stateful Sessions**: Session data persistence across requests via JWT claims
- **Visit Tracking**: Demonstrates session state management with visit counters
- **Token Refresh**: Automatic token refresh functionality
- **Security Best Practices**: Proper claim validation, issuer/audience verification

## Quick Start

### 1. Build and Run

```bash
# Build the example
go build -o jwt-example main.go

# Run the server
./jwt-example
```

The server will start at `http://localhost:8090`

### 2. Default Credentials

- **Username**: `user`
- **Password**: `password`

## API Endpoints

### Public Endpoints

#### `POST /login`
Authenticate with username/password and receive a JWT token.

**Request:**
```json
{
  "username": "user",
  "password": "password"
}
```

**Response:**
```json
{
  "success": true,
  "message": "Authentication successful",
  "user_id": 123,
  "username": "user",
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
}
```

**Headers:**
- `Authorization: Bearer <token>` - JWT token for subsequent requests

### Protected Endpoints

All protected endpoints require the `Authorization: Bearer <token>` header.

#### `GET /profile`
Get user profile information from JWT claims.

**Response:**
```json
{
  "user_id": 123,
  "username": "user",
  "visits": 5
}
```

#### `GET /visit`
Increment visit counter and return updated token.

**Response:**
```json
{
  "message": "Visit count: 6",
  "visits": 6
}
```

**Headers:**
- `Authorization: Bearer <new_token>` - Updated token with new visit count

#### `POST /refresh`
Refresh the current JWT token.

**Response:**
```json
{
  "message": "Token refreshed",
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
}
```

**Headers:**
- `Authorization: Bearer <new_token>` - Refreshed token

#### `POST /logout`
Logout and revoke the current token.

**Response:**
```json
{
  "message": "Logged out successfully"
}
```

### Health Check

#### `GET /health`
Health check endpoint (no authentication required).

**Response:**
```json
{
  "status": "healthy",
  "server": "jwt-session-example",
  "logged in": true
}
```

## Usage Examples

### 1. Login and Get Token

```bash
# Login
curl -X POST http://localhost:8090/login \
  -H "Content-Type: application/json" \
  -d '{"username": "user", "password": "password"}'

# Save the token from response
export TOKEN="eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
```

### 2. Access Protected Endpoints

```bash
# Get profile
curl -X GET http://localhost:8090/profile \
  -H "Authorization: Bearer $TOKEN"

# Increment visit counter
curl -X GET http://localhost:8090/visit \
  -H "Authorization: Bearer $TOKEN"

# Refresh token
curl -X POST http://localhost:8090/refresh \
  -H "Authorization: Bearer $TOKEN"
```

### 3. Logout

```bash
# Logout and revoke token
curl -X POST http://localhost:8090/logout \
  -H "Authorization: Bearer $TOKEN"
```

## JWT Configuration

The example uses the following JWT configuration:

```go
// JWT configuration
cfg, err := jwtprovider.NewJWTConfigWithKey([]byte("your-secret-key-should-be-at-least-32-bytes"))
cfg.ExpirationSeconds = 3600 // 1 hour
cfg.Issuer = "jwt-session-example"
cfg.Audience = "api-users"
```

### Security Considerations

1. **Secret Key**: The example uses a hardcoded secret key for demonstration. In production:
   - Use environment variables or secure key management
   - Consider asymmetric algorithms (RS256, ES256, EdDSA)
   - Ensure keys are at least 32 bytes for HMAC algorithms

2. **Token Expiration**: Tokens expire after 1 hour by default
3. **Revocation**: Tokens can be revoked using the logout endpoint
4. **Claim Validation**: Issuer and audience claims are validated automatically

## Key Components

### JWT Provider Setup

```go
// Create revocation manager (optional)
revocationManager := jwtprovider.NewRevocationManager(jwtprovider.NewMemoryRevocationBackend())

// Create JWT provider
provider, err := jwtprovider.NewProvider(cfg, jwtprovider.WithRevocationManager(revocationManager))
```

### Authentication Middleware

```go
// Apply JWT authentication to all routes after this point
server.UseAuth(auth.NewAuthJWT(provider))
```

### Accessing Claims

```go
// Get JWT claims in protected endpoints
claims, ok := auth.GetClaims(c)
if !ok {
    c.JSON(http.StatusUnauthorized, gin.H{"error": "No session found"})
    return
}

// Access user data
userID := claims.Data["userId"]
username := claims.Data["username"]
```

## Session State Management

The example demonstrates stateful JWT sessions by:

1. **Storing Data**: Adding custom data to JWT claims
2. **Updating State**: Modifying claim data (visit counter)
3. **Token Regeneration**: Creating new tokens with updated data
4. **Header Updates**: Returning updated tokens in response headers

## License

This example is part of the Blueprint framework and follows the same license terms.