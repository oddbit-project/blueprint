# JWT User Token Tracking Example

This example demonstrates the user token tracking functionality in the Blueprint JWT provider.

## Features Demonstrated

- User token tracking and session management
- Configurable concurrent session limits
- Automatic token tracking during generation
- Session count monitoring
- Bulk token revocation for security events
- Error handling for session limits

## Running the Example

```bash
cd examples/user-token-tracking
go run main.go
```

## Expected Output

```
=== JWT User Token Tracking Demo ===

Generating tokens for user 'user123'...
  Token 1: SUCCESS (length: 351)
    Current sessions: 1
  Token 2: SUCCESS (length: 351)
    Current sessions: 2
  Token 3: SUCCESS (length: 351)
    Current sessions: 3
  Token 4: FAILED - maximum concurrent sessions exceeded

Active tokens for user 'user123': 3
  1. QJtiI50aflavEil5hIRf3gwcTTuUBgx2O8SS4kVkSjA
  2. CvXeyYTewWIWqD2CLXffDj2OxDaLeGHdYR3SSP0ZcKY
  3. VPbjvG-W6EPKvqYyskrmx1SUhAO4sTB_5yQvG3VYC44

Revoking one token manually...
Sessions after revocation: 2

Revoking all tokens for user 'user123'...
Sessions after revoking all: 0

Testing token parsing after revocation...
  Token 1: REVOKED (token is already revoked)
  Token 2: REVOKED (token is already revoked)
  Token 3: REVOKED (token is already revoked)

=== Demo Complete ===
```

## Key Configuration

```go
// Enable user token tracking
config.TrackUserTokens = true

// Set maximum concurrent sessions (0 = unlimited)
config.MaxUserSessions = 3

// Revocation manager is required for tracking
revocationMgr := jwtprovider.NewRevocationManager(
    jwtprovider.NewMemoryRevocationBackend(),
)
```

## API Methods Demonstrated

- `provider.GenerateToken()` - Automatic tracking when enabled
- `provider.GetUserSessionCount()` - Count active sessions
- `provider.GetActiveUserTokens()` - List all active tokens
- `provider.RevokeAllUserTokens()` - Bulk revocation
- `provider.ParseToken()` - Validates against revoked tokens

## Use Cases

This functionality is useful for:

- **Session Management**: Limiting concurrent logins per user
- **Security Response**: Revoking all sessions on password change
- **Audit Trails**: Tracking token issuance and usage
- **Device Management**: Managing sessions across multiple devices
- **Compliance**: Meeting security requirements for session control

## Security Benefits

- Prevents credential sharing by limiting concurrent sessions
- Enables quick security response during incidents
- Provides audit trail for token lifecycle
- Automatic cleanup prevents memory leaks
- Thread-safe operations for concurrent usage