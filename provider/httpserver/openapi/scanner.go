package openapi

import (
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/oddbit-project/blueprint/provider/httpserver"
)

// Scanner discovers and analyzes routes from a gin server
type Scanner struct {
	spec   *OpenAPISpec
	server *httpserver.Server
}

// NewScanner creates a new route scanner
func NewScanner(server *httpserver.Server) *Scanner {
	return &Scanner{
		spec:   NewSpec(),
		server: server,
	}
}

// ScanServer analyzes the server and generates OpenAPI specification
func ScanServer(server *httpserver.Server) *OpenAPISpec {
	scanner := NewScanner(server)
	return scanner.Scan()
}

// Scan performs the route discovery and analysis
func (s *Scanner) Scan() *OpenAPISpec {
	if s.server == nil || s.server.Router == nil {
		return s.spec
	}

	// Get all registered routes from gin
	routes := s.server.Router.Routes()
	
	for _, route := range routes {
		s.analyzeRoute(route)
	}
	
	return s.spec
}

// analyzeRoute processes a single gin route
func (s *Scanner) analyzeRoute(route gin.RouteInfo) {
	if s.shouldSkipRoute(route) {
		return
	}

	path := s.convertGinPathToOpenAPI(route.Path)
	method := strings.ToUpper(route.Method)
	
	operation := s.createOperation(route)
	s.spec.AddOperation(path, method, operation)
}

// shouldSkipRoute determines if a route should be excluded from documentation
func (s *Scanner) shouldSkipRoute(route gin.RouteInfo) bool {
	// Skip internal gin routes
	if strings.HasPrefix(route.Path, "/debug/") {
		return true
	}
	
	// Skip common non-API routes
	skipPaths := []string{
		"/favicon.ico",
		"/robots.txt",
		"/health",
		"/metrics",
		"/docs",
		"/swagger",
		"/openapi",
	}
	
	for _, skipPath := range skipPaths {
		if strings.HasPrefix(route.Path, skipPath) {
			return true
		}
	}
	
	return false
}

// convertGinPathToOpenAPI converts gin path format to OpenAPI format
// Example: /users/:id/posts/:postId -> /users/{id}/posts/{postId}
func (s *Scanner) convertGinPathToOpenAPI(ginPath string) string {
	// Convert :param to {param}
	paramRegex := regexp.MustCompile(`:([^/]+)`)
	openAPIPath := paramRegex.ReplaceAllString(ginPath, "{$1}")
	
	// Convert *param to {param} (catch-all parameters)
	catchAllRegex := regexp.MustCompile(`\*([^/]+)`)
	openAPIPath = catchAllRegex.ReplaceAllString(openAPIPath, "{$1}")
	
	return openAPIPath
}

// createOperation creates an OpenAPI operation from a gin route
func (s *Scanner) createOperation(route gin.RouteInfo) Operation {
	operation := Operation{
		Summary:     s.generateSummary(route),
		Description: s.generateDescription(route),
		OperationID: s.generateOperationID(route),
		Parameters:  s.extractPathParameters(route.Path),
		Responses:   s.generateDefaultResponses(route.Method),
	}
	
	// Add request body for POST, PUT, PATCH
	if s.needsRequestBody(route.Method) {
		operation.RequestBody = s.generateRequestBody(route)
	}
	
	// Add tags based on path
	operation.Tags = s.generateTags(route.Path)
	
	return operation
}

// generateSummary creates a human-readable summary for the operation
func (s *Scanner) generateSummary(route gin.RouteInfo) string {
	method := strings.ToLower(route.Method)
	path := route.Path
	
	// Extract resource name from path
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 {
		return strings.Title(method) + " root"
	}
	
	resource := parts[0]
	if len(parts) > 1 && strings.Contains(parts[1], ":") {
		// Path has parameters, likely a specific resource operation
		switch method {
		case "get":
			return "Get " + singularize(resource)
		case "put":
			return "Update " + singularize(resource)
		case "delete":
			return "Delete " + singularize(resource)
		case "patch":
			return "Partially update " + singularize(resource)
		}
	}
	
	// Collection-level operations
	switch method {
	case "get":
		return "List " + resource
	case "post":
		return "Create " + singularize(resource)
	}
	
	return strings.Title(method) + " " + resource
}

// generateDescription creates a description for the operation
func (s *Scanner) generateDescription(route gin.RouteInfo) string {
	return "Auto-generated description for " + route.Method + " " + route.Path
}

// generateOperationID creates a unique operation ID
func (s *Scanner) generateOperationID(route gin.RouteInfo) string {
	method := strings.ToLower(route.Method)
	path := strings.ReplaceAll(route.Path, "/", "_")
	path = strings.ReplaceAll(path, ":", "")
	path = strings.ReplaceAll(path, "*", "")
	path = strings.Trim(path, "_")
	
	if path == "" {
		return method + "_root"
	}
	
	return method + "_" + path
}

// extractPathParameters extracts path parameters from gin route path
func (s *Scanner) extractPathParameters(path string) []Parameter {
	var parameters []Parameter
	
	// Find :param patterns
	paramRegex := regexp.MustCompile(`:([^/]+)`)
	matches := paramRegex.FindAllStringSubmatch(path, -1)
	
	for _, match := range matches {
		if len(match) > 1 {
			paramName := match[1]
			parameters = append(parameters, Parameter{
				Name:        paramName,
				In:          "path",
				Required:    true,
				Description: "Path parameter: " + paramName,
				Schema: &Schema{
					Type: "string",
				},
			})
		}
	}
	
	// Find *param patterns (catch-all)
	catchAllRegex := regexp.MustCompile(`\*([^/]+)`)
	catchAllMatches := catchAllRegex.FindAllStringSubmatch(path, -1)
	
	for _, match := range catchAllMatches {
		if len(match) > 1 {
			paramName := match[1]
			parameters = append(parameters, Parameter{
				Name:        paramName,
				In:          "path",
				Required:    true,
				Description: "Catch-all path parameter: " + paramName,
				Schema: &Schema{
					Type: "string",
				},
			})
		}
	}
	
	return parameters
}

// generateDefaultResponses creates default response definitions
func (s *Scanner) generateDefaultResponses(method string) map[string]Response {
	responses := make(map[string]Response)
	
	switch strings.ToUpper(method) {
	case "GET":
		responses["200"] = Response{
			Description: "Successful response",
			Content: map[string]MediaType{
				"application/json": {
					Schema: &Schema{
						Type: "object",
					},
				},
			},
		}
	case "POST":
		responses["201"] = Response{
			Description: "Resource created successfully",
			Content: map[string]MediaType{
				"application/json": {
					Schema: &Schema{
						Type: "object",
					},
				},
			},
		}
		responses["400"] = Response{
			Description: "Bad request",
		}
	case "PUT", "PATCH":
		responses["200"] = Response{
			Description: "Resource updated successfully",
			Content: map[string]MediaType{
				"application/json": {
					Schema: &Schema{
						Type: "object",
					},
				},
			},
		}
		responses["400"] = Response{
			Description: "Bad request",
		}
		responses["404"] = Response{
			Description: "Resource not found",
		}
	case "DELETE":
		responses["204"] = Response{
			Description: "Resource deleted successfully",
		}
		responses["404"] = Response{
			Description: "Resource not found",
		}
	}
	
	// Common error responses
	responses["500"] = Response{
		Description: "Internal server error",
	}
	
	return responses
}

// needsRequestBody determines if the HTTP method typically needs a request body
func (s *Scanner) needsRequestBody(method string) bool {
	method = strings.ToUpper(method)
	return method == "POST" || method == "PUT" || method == "PATCH"
}

// generateRequestBody creates a generic request body definition
func (s *Scanner) generateRequestBody(route gin.RouteInfo) *RequestBody {
	return &RequestBody{
		Description: "Request body for " + route.Method + " " + route.Path,
		Required:    true,
		Content: map[string]MediaType{
			"application/json": {
				Schema: &Schema{
					Type: "object",
				},
			},
		},
	}
}

// generateTags creates tags based on the route path
func (s *Scanner) generateTags(path string) []string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		return []string{"default"}
	}
	
	// Use the first path segment as the primary tag
	tag := parts[0]
	
	// Remove parameter indicators
	tag = strings.ReplaceAll(tag, ":", "")
	tag = strings.ReplaceAll(tag, "*", "")
	
	if tag == "" {
		return []string{"default"}
	}
	
	return []string{tag}
}

// singularize attempts to convert plural nouns to singular (basic implementation)
func singularize(word string) string {
	if strings.HasSuffix(word, "ies") {
		return strings.TrimSuffix(word, "ies") + "y"
	}
	if strings.HasSuffix(word, "es") {
		return strings.TrimSuffix(word, "es")
	}
	if strings.HasSuffix(word, "s") && len(word) > 1 {
		return strings.TrimSuffix(word, "s")
	}
	return word
}

// SetInfo configures the API information
func (s *Scanner) SetInfo(title, version, description string) *Scanner {
	s.spec.SetInfo(title, version, description)
	return s
}

// AddServer adds a server configuration
func (s *Scanner) AddServer(url, description string) *Scanner {
	s.spec.AddServer(url, description)
	return s
}

// AddBearerAuth adds JWT bearer authentication
func (s *Scanner) AddBearerAuth() *Scanner {
	s.spec.AddBearerAuth()
	return s
}

// GetSpec returns the generated specification
func (s *Scanner) GetSpec() *OpenAPISpec {
	return s.spec
}