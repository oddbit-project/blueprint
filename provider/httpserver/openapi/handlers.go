package openapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// SpecHandler returns the OpenAPI specification as JSON
func SpecHandler(spec *OpenAPISpec) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Content-Type", "application/json")
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type")
		
		jsonData, err := spec.ToJSON()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to generate OpenAPI specification",
			})
			return
		}
		
		c.Data(http.StatusOK, "application/json", jsonData)
	}
}

// SwaggerUIHandler serves the Swagger UI interface
func SwaggerUIHandler(spec *OpenAPISpec) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get the base URL for the API spec
		scheme := "http"
		if c.Request.TLS != nil {
			scheme = "https"
		}
		
		specURL := scheme + "://" + c.Request.Host + "/openapi.json"
		
		html := generateSwaggerHTML(specURL)
		
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusOK, html)
	}
}

// RedocHandler serves the Redoc documentation interface
func RedocHandler(spec *OpenAPISpec) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get the base URL for the API spec
		scheme := "http"
		if c.Request.TLS != nil {
			scheme = "https"
		}
		
		specURL := scheme + "://" + c.Request.Host + "/openapi.json"
		
		html := generateRedocHTML(specURL)
		
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusOK, html)
	}
}

// generateSwaggerHTML generates the Swagger UI HTML page
func generateSwaggerHTML(specURL string) string {
	return `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <meta name="description" content="API Documentation" />
    <title>API Documentation - Swagger UI</title>
    <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5.17.14/swagger-ui.css" />
    <style>
        html {
            box-sizing: border-box;
            overflow: -moz-scrollbars-vertical;
            overflow-y: scroll;
        }
        *, *:before, *:after {
            box-sizing: inherit;
        }
        body {
            margin:0;
            background: #fafafa;
        }
        .swagger-ui .topbar {
            background-color: #1b1b1b;
            border-bottom: 1px solid #262626;
        }
        .swagger-ui .topbar .topbar-wrapper .link {
            color: #8cc8ff;
        }
        .swagger-ui .topbar .topbar-wrapper .link:hover {
            color: #ffffff;
        }
        .auth-info {
            background: #e3f2fd;
            padding: 15px;
            margin: 10px;
            border-radius: 8px;
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
        }
        .auth-info h4 {
            margin: 0 0 10px 0;
            color: #1565c0;
        }
        .auth-info code {
            background: #f5f5f5;
            padding: 2px 6px;
            border-radius: 4px;
        }
    </style>
</head>
<body>
    <div class="auth-info">
        <h4>🔐 Authentication Setup</h4>
        <p>To test protected endpoints:</p>
        <ol>
            <li>Click the <strong>"Authorize"</strong> button below</li>
            <li>Enter your JWT token in the format: <code>Bearer YOUR_TOKEN_HERE</code></li>
            <li>Or use this demo token: <code>Bearer demo-jwt-token-12345</code></li>
        </ol>
        <p><strong>Note:</strong> Some endpoints are public (health, docs) and don't require authentication.</p>
    </div>
    <div id="swagger-ui"></div>
    <script src="https://unpkg.com/swagger-ui-dist@5.17.14/swagger-ui-bundle.js" crossorigin></script>
    <script>
        window.onload = () => {
            window.ui = SwaggerUIBundle({
                url: '` + specURL + `',
                dom_id: '#swagger-ui',
                deepLinking: true,
                presets: [
                    SwaggerUIBundle.presets.apis,
                    SwaggerUIBundle.presets.standalone
                ],
                plugins: [
                    SwaggerUIBundle.plugins.DownloadUrl
                ],
                tryItOutEnabled: true,
                persistAuthorization: true,
                initOAuth: {
                    clientId: "swagger-ui",
                    realm: "swagger-ui-realm",
                    appName: "Swagger UI"
                },
                requestInterceptor: function(req) {
                    // Add any custom request headers here
                    return req;
                },
                responseInterceptor: function(res) {
                    // Handle responses here
                    return res;
                }
            });
        };
    </script>
</body>
</html>`
}

// generateRedocHTML generates the Redoc documentation HTML page
func generateRedocHTML(specURL string) string {
	return `<!DOCTYPE html>
<html>
<head>
    <title>API Documentation - ReDoc</title>
    <meta charset="utf-8"/>
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <link href="https://fonts.googleapis.com/css?family=Montserrat:300,400,700|Roboto:300,400,700" rel="stylesheet">
    <style>
        body {
            margin: 0;
            padding: 0;
        }
    </style>
</head>
<body>
    <redoc spec-url='` + specURL + `'></redoc>
    <script src="https://cdn.jsdelivr.net/npm/redoc@2.1.5/bundles/redoc.standalone.js"></script>
</body>
</html>`
}

// DocsIndexHandler provides a simple index page with links to different documentation formats
func DocsIndexHandler(spec *OpenAPISpec) gin.HandlerFunc {
	return func(c *gin.Context) {
		scheme := "http"
		if c.Request.TLS != nil {
			scheme = "https"
		}
		
		baseURL := scheme + "://" + c.Request.Host
		
		html := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>API Documentation</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            max-width: 800px;
            margin: 0 auto;
            padding: 2rem;
            background-color: #f9f9f9;
        }
        .container {
            background: white;
            border-radius: 8px;
            padding: 2rem;
            box-shadow: 0 2px 10px rgba(0,0,0,0.1);
        }
        h1 {
            color: #2c3e50;
            margin-bottom: 2rem;
        }
        .links {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
            gap: 1rem;
            margin: 2rem 0;
        }
        .link-card {
            background: #f8f9fa;
            padding: 1.5rem;
            border-radius: 6px;
            text-decoration: none;
            color: #495057;
            border: 2px solid transparent;
            transition: all 0.2s ease;
        }
        .link-card:hover {
            background: #e9ecef;
            border-color: #007bff;
            transform: translateY(-2px);
        }
        .link-title {
            font-weight: 600;
            margin-bottom: 0.5rem;
            color: #007bff;
        }
        .link-desc {
            font-size: 0.9rem;
            color: #6c757d;
        }
        .info {
            background: #e3f2fd;
            padding: 1rem;
            border-radius: 4px;
            margin: 1rem 0;
        }
        .spec-info {
            margin: 1rem 0;
            padding: 1rem;
            background: #f1f3f4;
            border-radius: 4px;
        }
        .spec-info h3 {
            margin: 0 0 0.5rem 0;
            color: #1a73e8;
        }
        .spec-info p {
            margin: 0.25rem 0;
            color: #5f6368;
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>🚀 API Documentation</h1>
        
        <div class="spec-info">
            <h3>` + spec.Info.Title + `</h3>
            <p><strong>Version:</strong> ` + spec.Info.Version + `</p>` +
			func() string {
				if spec.Info.Description != "" {
					return `<p><strong>Description:</strong> ` + spec.Info.Description + `</p>`
				}
				return ""
			}() + `
        </div>
        
        <div class="info">
            <strong>📖 Choose your preferred documentation format:</strong>
        </div>
        
        <div class="links">
            <a href="` + baseURL + `/swagger" class="link-card">
                <div class="link-title">Swagger UI</div>
                <div class="link-desc">Interactive API explorer with try-it-out functionality</div>
            </a>
            
            <a href="` + baseURL + `/redoc" class="link-card">
                <div class="link-title">ReDoc</div>
                <div class="link-desc">Clean, responsive API documentation</div>
            </a>
            
            <a href="` + baseURL + `/openapi.json" class="link-card">
                <div class="link-title">OpenAPI Spec</div>
                <div class="link-desc">Raw OpenAPI 3.0 JSON specification</div>
            </a>
        </div>
        
        <div class="info">
            <strong>💡 Tip:</strong> This documentation is automatically generated from your code using the Blueprint OpenAPI integration.
        </div>
    </div>
</body>
</html>`
		
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusOK, html)
	}
}

// RegisterHandlers is a convenience function to register all documentation handlers
func RegisterHandlers(router gin.IRouter, spec *OpenAPISpec) {
	// Main documentation index
	router.GET("/docs", DocsIndexHandler(spec))
	
	// Swagger UI
	router.GET("/swagger", SwaggerUIHandler(spec))
	
	// ReDoc
	router.GET("/redoc", RedocHandler(spec))
	
	// Raw OpenAPI specification
	router.GET("/openapi.json", SpecHandler(spec))
	
	// Alternative paths for compatibility
	router.GET("/docs/swagger", SwaggerUIHandler(spec))
	router.GET("/docs/redoc", RedocHandler(spec))
	router.GET("/docs/openapi.json", SpecHandler(spec))
}

// CustomSwaggerUIHandler allows customization of Swagger UI
func CustomSwaggerUIHandler(spec *OpenAPISpec, config SwaggerUIConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		scheme := "http"
		if c.Request.TLS != nil {
			scheme = "https"
		}
		
		specURL := scheme + "://" + c.Request.Host + "/openapi.json"
		html := generateCustomSwaggerHTML(specURL, config)
		
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusOK, html)
	}
}

// SwaggerUIConfig allows customization of Swagger UI
type SwaggerUIConfig struct {
	Title            string
	DeepLinking      bool
	TryItOutEnabled  bool
	RequestSnippets  bool
	DisplayRequestDuration bool
	SupportedSubmitMethods []string
}

// DefaultSwaggerUIConfig returns default Swagger UI configuration
func DefaultSwaggerUIConfig() SwaggerUIConfig {
	return SwaggerUIConfig{
		Title:            "API Documentation",
		DeepLinking:      true,
		TryItOutEnabled:  true,
		RequestSnippets:  true,
		DisplayRequestDuration: true,
		SupportedSubmitMethods: []string{"get", "post", "put", "delete", "patch"},
	}
}

// generateCustomSwaggerHTML generates Swagger UI HTML with custom configuration
func generateCustomSwaggerHTML(specURL string, config SwaggerUIConfig) string {
	return `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <title>` + config.Title + `</title>
    <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5.17.14/swagger-ui.css" />
</head>
<body>
    <div id="swagger-ui"></div>
    <script src="https://unpkg.com/swagger-ui-dist@5.17.14/swagger-ui-bundle.js"></script>
    <script>
        SwaggerUIBundle({
            url: '` + specURL + `',
            dom_id: '#swagger-ui',
            deepLinking: ` + boolToJS(config.DeepLinking) + `,
            tryItOutEnabled: ` + boolToJS(config.TryItOutEnabled) + `,
            requestSnippetsEnabled: ` + boolToJS(config.RequestSnippets) + `,
            displayRequestDuration: ` + boolToJS(config.DisplayRequestDuration) + `,
            supportedSubmitMethods: ` + sliceToJS(config.SupportedSubmitMethods) + `,
            presets: [
                SwaggerUIBundle.presets.apis,
                SwaggerUIBundle.presets.standalone
            ]
        });
    </script>
</body>
</html>`
}

// Helper functions for JavaScript generation
func boolToJS(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func sliceToJS(slice []string) string {
	if len(slice) == 0 {
		return "[]"
	}
	
	result := "["
	for i, item := range slice {
		if i > 0 {
			result += ", "
		}
		result += `"` + item + `"`
	}
	result += "]"
	return result
}