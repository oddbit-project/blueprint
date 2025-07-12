# HTTP CSRF Protection Demo

This sample demonstrates how to use CSRF (Cross-Site Request Forgery) protection with the Blueprint HTTP server.

## Features

- Session-based CSRF token management
- Protection for POST, PUT, DELETE requests
- HTML form demo with embedded CSRF tokens
- API endpoint examples with header-based tokens
- Comprehensive security middleware integration

## Running the Demo

```bash
cd samples/http-csrf
go run main.go
```

The server will start at `http://localhost:8089`

## Available Endpoints

### Public Endpoints (No CSRF Required)
- `GET /` - Get CSRF token and API instructions
- `GET /form` - Interactive HTML form demo
- `GET /public` - Public endpoint example

### Protected Endpoints (CSRF Required)
- `POST /submit` - Form submission endpoint
- `POST /api/data` - JSON API endpoint
- `PUT /api/update` - Update endpoint
- `DELETE /api/delete` - Delete endpoint

## Usage Examples

### 1. Get CSRF Token
```bash
# Get token and save session
curl -c cookies.txt http://localhost:8089/
```

### 2. Form Submission
```bash
# With CSRF token in form data
curl -b cookies.txt -X POST http://localhost:8089/submit \
  -d "_csrf=YOUR_TOKEN&name=Alice&message=Hello"

# With CSRF token in header
curl -b cookies.txt -H "X-CSRF-Token: YOUR_TOKEN" \
  -X POST http://localhost:8089/submit \
  -d "name=Bob&message=World"
```

### 3. JSON API Calls
```bash
# API call with CSRF token
curl -b cookies.txt -H "X-CSRF-Token: YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -X POST http://localhost:8089/api/data \
  -d '{"test": "data"}'
```

### 4. Interactive Demo
Visit `http://localhost:8089/form` in your browser for an interactive demo showing:
- Forms with valid CSRF tokens (should work)
- Forms without CSRF tokens (should fail)
- JavaScript API examples

## Security Features

- **Session Management**: Uses secure session cookies
- **Token Validation**: Validates CSRF tokens against session storage
- **Auto-Refresh**: Tokens are refreshed after successful requests
- **Multiple Sources**: Accepts tokens from `X-CSRF-Token` header or `_csrf` form field
- **Method Protection**: Only protects state-changing methods (POST, PUT, DELETE)

## Implementation Details

The demo uses:
- `provider/httpserver/security.CSRFProtection()` middleware
- `provider/httpserver/session` for session management
- Memory-based session storage (configurable)
- UUID-based CSRF tokens

## Testing

The application includes comprehensive testing scenarios:
- Valid token acceptance
- Invalid token rejection
- Missing token rejection
- Session-based token validation
- Multiple request methods

Run the server and use the provided curl examples or visit the interactive form demo to test CSRF protection.