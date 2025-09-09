# OpenAPI Demo

This sample demonstrates how to integrate OpenAPI 3.0 documentation generation with the Blueprint framework.

## Features

- **Automatic OpenAPI spec generation** from Go code using reflection
- **Multiple documentation formats**: Swagger UI, ReDoc, and raw JSON
- **Struct tag parsing** for enhanced documentation (`doc`, `example`, validation tags)
- **Type-safe schema generation** with support for complex nested structures
- **Zero code changes** required to existing handlers

## Running the Demo

1. Start the server:
```bash
go run main.go
```

2. Visit the documentation:
   - **Documentation Index**: http://localhost:8080/docs
   - **Swagger UI**: http://localhost:8080/swagger
   - **ReDoc**: http://localhost:8080/redoc
   - **Raw OpenAPI Spec**: http://localhost:8080/openapi.json

## API Endpoints

The demo provides a simple User Management API:

- `GET /api/v1/users` - List all users
- `POST /api/v1/users` - Create a new user
- `GET /api/v1/users/:id` - Get user by ID
- `PUT /api/v1/users/:id` - Update user
- `DELETE /api/v1/users/:id` - Delete user
- `GET /health` - Health check endpoint

## Code Highlights

### Enhanced Struct Tags

The demo shows how to use enhanced struct tags for better documentation:

```go
type User struct {
    ID    int    `json:"id" doc:"Unique user identifier" example:"123"`
    Name  string `json:"name" binding:"required" doc:"User's full name" example:"John Doe"`
    Email string `json:"email" binding:"required,email" doc:"Valid email address" example:"john@example.com"`
    Age   int    `json:"age" binding:"min=0,max=120" doc:"User's age" example:"30"`
}
```

### Integration with Existing Blueprint Code

The OpenAPI integration requires minimal changes to existing Blueprint applications:

```go
func (a *Application) Build() {
    // ... existing server setup code ...
    
    // Generate OpenAPI documentation (NEW - just these lines!)
    spec := openapi.ScanServer(a.httpServer)
    spec.SetInfo("User Management API", VERSION, "API description")
    
    // Register documentation endpoints (NEW)
    openapi.RegisterHandlers(a.httpServer.Route(), spec)
}
```

### Automatic Type Analysis

The system automatically analyzes:
- Path parameters from gin routes (`/users/:id` → `{id}`)
- Request/response types from handler signatures
- Validation rules from `binding:` tags
- Documentation from `doc:` tags
- Examples from `example:` tags
- Complex nested structures and arrays

## Architecture

The OpenAPI integration consists of four main components:

1. **`spec.go`** - OpenAPI 3.0 specification structures
2. **`scanner.go`** - Route discovery and analysis using gin's reflection
3. **`analyzer.go`** - Go type analysis and schema generation
4. **`handlers.go`** - HTTP handlers for serving documentation

## Benefits for LLM Integration

This implementation makes the Blueprint framework more LLM-friendly by:

- **Providing machine-readable API contracts** via OpenAPI specs
- **Enabling automatic code generation** from specifications
- **Supporting API discovery** through standardized documentation
- **Maintaining consistency** across API endpoints
- **Facilitating integration testing** with generated schemas

## Customization

The documentation can be customized:

```go
// Custom Swagger UI configuration
config := openapi.DefaultSwaggerUIConfig()
config.Title = "Custom API Docs"
config.TryItOutEnabled = false

// Use custom handler
router.GET("/docs", openapi.CustomSwaggerUIHandler(spec, config))
```

## Production Considerations

- The documentation endpoints can be disabled in production by conditional registration
- OpenAPI spec generation happens once at startup, not per request
- Consider caching the generated JSON specification
- Add authentication to documentation endpoints if needed