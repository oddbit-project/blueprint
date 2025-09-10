# Authentication Guide for OpenAPI Demo

## 🔐 Interactive Authentication Setup

The OpenAPI demo now includes full authentication support in both Swagger UI and ReDoc interfaces.

### **Quick Start**

1. **Start the server:**
   ```bash
   go run main.go
   ```

2. **Visit the Swagger UI:** http://localhost:8081/swagger

3. **Get a demo token:**
   - Use the `/login` endpoint in Swagger UI
   - Username: `demo`
   - Password: `password` 
   - Copy the returned token

4. **Authorize in Swagger UI:**
   - Click the "Authorize" button (🔒 icon)
   - Paste token in format: `Bearer demo-jwt-token-12345-demo`
   - Click "Authorize"

5. **Test protected endpoints:**
   - Try the `/api/v1/users` endpoints
   - They now include the Authorization header automatically

## 🚀 Features Added

### **1. Authentication Information in Spec**
- API endpoints marked as requiring `bearerAuth`
- Public endpoints (login, health, docs) have no auth requirements
- Security schemes properly documented

### **2. Enhanced Swagger UI**
- Visual instructions for authentication setup
- Persistent authorization (stays logged in)
- Demo token provided for easy testing
- Professional styling with auth info banner

### **3. Demo Login Endpoint**
- `POST /login` for getting demo tokens
- Proper request/response documentation
- Error handling with helpful messages

### **4. Smart Route Classification**
- Automatically detects which endpoints need auth
- Public routes: `/health`, `/docs`, `/login`, etc.
- Protected routes: `/api/*` endpoints

## 📋 OpenAPI Specification Changes

### **Security Requirements**
Protected endpoints now include:
```json
{
  "security": [
    {
      "bearerAuth": []
    }
  ]
}
```

### **Security Schemes**
```json
{
  "components": {
    "securitySchemes": {
      "bearerAuth": {
        "type": "http",
        "scheme": "bearer", 
        "bearerFormat": "JWT",
        "description": "JWT Bearer token authentication"
      }
    }
  }
}
```

## 🧪 Testing Authentication

### **Using curl**
1. Get a token:
   ```bash
   curl -X POST http://localhost:8081/login \
     -H "Content-Type: application/json" \
     -d '{"username":"demo","password":"password"}'
   ```

2. Use the token:
   ```bash
   curl -H "Authorization: Bearer demo-jwt-token-12345-demo" \
     http://localhost:8081/api/v1/users
   ```

### **Using Swagger UI**
1. Open http://localhost:8081/swagger
2. Follow the authentication instructions banner
3. Use the login endpoint to get a token
4. Click "Authorize" and enter the token
5. Test any protected endpoint

### **Using ReDoc**
- Open http://localhost:8081/redoc
- View the authentication requirements for each endpoint
- See the security schemes documentation

## 🔧 Configuration Options

### **Customizing Authentication Logic**
Modify the `requiresAuthentication()` function in `scanner.go`:
```go
func (s *Scanner) requiresAuthentication(path string) bool {
    // Add your custom logic here
    publicPaths := []string{"/health", "/docs", "/login"}
    // ... rest of logic
}
```

### **Adding OAuth2 Support**
Extend the security schemes:
```go
spec.AddSecurityScheme("oauth2", SecurityScheme{
    Type: "oauth2",
    Flows: map[string]OAuthFlow{
        "authorizationCode": {
            AuthorizationUrl: "https://auth.example.com/oauth/authorize",
            TokenUrl: "https://auth.example.com/oauth/token",
        },
    },
})
```

### **Custom Swagger UI Configuration**
Use the `CustomSwaggerUIHandler`:
```go
config := openapi.DefaultSwaggerUIConfig()
config.PersistAuthorization = true
config.TryItOutEnabled = true

router.GET("/swagger", openapi.CustomSwaggerUIHandler(spec, config))
```

## 🎯 Production Considerations

1. **Real JWT Implementation:** Replace demo tokens with proper JWT generation
2. **Secure Token Storage:** Use secure cookie or localStorage strategies  
3. **Token Validation:** Add proper JWT validation middleware
4. **Rate Limiting:** Implement rate limiting on auth endpoints
5. **HTTPS Only:** Use HTTPS in production for token security

## 💡 Benefits for LLM Integration

- **Machine-readable auth requirements** in OpenAPI spec
- **Interactive testing** without manual header setup
- **Standardized security documentation** for AI tools
- **Automatic client generation** with proper auth handling
- **Clear separation** between public and protected endpoints

The authentication setup makes the API much more LLM-friendly by providing clear, standardized documentation of security requirements that AI tools can understand and work with! 🤖