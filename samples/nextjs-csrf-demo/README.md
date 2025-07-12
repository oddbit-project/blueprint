# Next.js CSRF Demo Application

This is a complete working example of integrating Blueprint's CSRF protection with a Next.js frontend application.

## Project Structure

```
nextjs-csrf-demo/
├── app/
│   ├── layout.tsx          # Root layout with CSRF provider
│   ├── page.tsx            # Main demo page
│   └── globals.css         # Global styles
├── components/
│   ├── user-manager.tsx    # User CRUD component with CSRF
│   └── csrf-status.tsx     # CSRF token status display
├── lib/
│   ├── api-client.ts       # API client with CSRF support
│   └── csrf-context.tsx    # React context for CSRF management
└── .env.local              # Environment configuration
```

## Features Demonstrated

- **CSRF Token Management**: Automatic token fetching and refresh
- **API Integration**: Complete CRUD operations with CSRF protection
- **Error Handling**: Graceful handling of CSRF failures with retry logic
- **Real-time Status**: Visual CSRF protection status indicator
- **Session Management**: Cookie-based session persistence

## Prerequisites

1. **Blueprint API Server**: The backend must be running
   ```bash
   cd ../nextjs-api
   go run main.go
   ```

2. **Node.js**: Version 18 or higher

## Running the Demo

1. **Install dependencies**:
   ```bash
   npm install
   ```

2. **Start the development server**:
   ```bash
   npm run dev
   ```

3. **Open your browser**:
   Navigate to [http://localhost:3000](http://localhost:3000)

## Testing the Integration

### Manual Browser Testing

1. **Load the page** - CSRF token should automatically load
2. **View users** - GET request works without CSRF token
3. **Create user** - POST request requires CSRF token
4. **Delete user** - DELETE request requires CSRF token
5. **Check network tab** - Observe X-CSRF-Token headers

### Automated Testing Results

The integration has been tested and verified:

**CSRF Protection**: Blocks unauthorized requests without tokens  
**CORS Configuration**: Properly allows Next.js origin  
**Token Generation**: Session-based tokens work correctly  
**Protected Endpoints**: POST/PUT/DELETE require valid tokens  
**Public Endpoints**: GET requests work without tokens  
**Token Refresh**: Tokens rotate after successful requests  

### Example Test Output

```bash
1. Testing CSRF protection - should BLOCK request without token:
{"success":false,"error":{"message":"Forbidden"}}

2. Testing complete CSRF flow:
   a) Get CSRF token with session:
      Token: 943e5d05-751d-4aba-ada7-de72b359cd25
   b) Use token for protected request:
      Response: {"message":"User created successfully","success":true,...}
   c) Verify GET requests work without CSRF:
      Users loaded: 3
```

## Key Integration Points

### 1. CSRF Context Provider
- Manages token lifecycle
- Provides token to all components
- Handles automatic refresh

### 2. API Client
- Automatically includes CSRF tokens
- Handles token refresh from response headers
- Implements retry logic for CSRF failures

### 3. Error Handling
- Graceful degradation on CSRF failures
- User-friendly error messages
- Automatic retry with fresh tokens

## Security Features

- **Session-based Security**: Tokens tied to server sessions
- **Automatic Rotation**: Tokens refresh after each successful request
- **Multi-source Support**: Accepts tokens from headers or form fields
- **CORS Protection**: Properly configured origins
- **Method Filtering**: Only protects state-changing methods

## Development Notes

### Environment Variables
- `NEXT_PUBLIC_API_BASE_URL`: Backend API URL (default: http://localhost:8080)

### Network Considerations
- Requires `credentials: 'include'` for session cookies
- CORS must allow the Next.js origin
- Session cookies use SameSite=Lax for cross-origin requests

### Production Deployment
- Update API_BASE_URL for production backend
- Ensure HTTPS for secure cookies
- Configure production CORS origins
- Set appropriate session security flags

## Troubleshooting

### Common Issues

1. **CSRF Token Not Loading**
   - Check that the backend API is running
   - Verify CORS configuration allows your origin
   - Ensure cookies are being sent with requests

2. **403 Forbidden Errors**
   - Normal for requests without valid tokens
   - Check that tokens are being included in headers
   - Verify session persistence across requests

3. **CORS Errors**
   - Ensure backend allows your frontend origin
   - Check that credentials are included in requests
   - Verify preflight OPTIONS handling

### Debug Tips

- Use browser dev tools to inspect network requests
- Check the CSRF Status component for token information  
- Look for X-CSRF-Token headers in response
- Verify session cookies are being set and sent

## Architecture Benefits

This integration provides:
- **Security**: Robust CSRF protection
- **Usability**: Transparent to end users
- **Maintainability**: Clean separation of concerns
- **Scalability**: Stateless token validation
- **Flexibility**: Works with any Blueprint backend

The demo serves as a complete reference implementation for integrating Blueprint's CSRF protection with modern React applications.