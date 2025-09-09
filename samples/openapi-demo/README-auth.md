# Handling Authorization in OpenAPI Demo

If you're getting `MissingSecurityHeader` errors, here are solutions:

## Quick Fix - Disable Security
```go
func (a *Application) Build() {
    // ... server setup ...
    
    // Comment out these lines to disable security:
    // a.httpServer.UseDefaultSecurityHeaders()
    // a.httpServer.UseRateLimiting(60)
}
```

## Proper Solution - Add Authorization Header
If your Blueprint app requires authorization:

```bash
# Test with Authorization header
curl -H "Authorization: Bearer your-token-here" http://localhost:8081/api/v1/users
```

## OpenAPI Documentation for Auth
The current implementation already adds JWT bearer auth to the OpenAPI spec:

```go
// This adds bearer auth to the OpenAPI spec
spec.AddBearerAuth()
```

## Enhanced Demo with Auth
To see a version with actual JWT authentication, check the original Blueprint README example which shows:
- JWT token generation on `/login`
- Protected routes with JWT middleware
- Proper OpenAPI security scheme documentation

## Testing Without Auth
Current demo endpoints should work without authorization:
- `GET /health` ✅
- `GET /api/v1/users` ✅  
- `POST /api/v1/users` ✅
- `GET /openapi.json` ✅
- `GET /docs` ✅

If you're still getting auth errors, please share:
1. The exact curl command you're using
2. The complete error response
3. Any additional middleware you've added