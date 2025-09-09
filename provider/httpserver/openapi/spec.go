package openapi

import (
	"encoding/json"
	"time"
)

// OpenAPI 3.0 specification structures
type OpenAPISpec struct {
	OpenAPI    string                 `json:"openapi"`
	Info       Info                   `json:"info"`
	Servers    []Server               `json:"servers,omitempty"`
	Paths      map[string]PathItem    `json:"paths"`
	Components *Components            `json:"components,omitempty"`
	Security   []SecurityRequirement  `json:"security,omitempty"`
	Tags       []Tag                  `json:"tags,omitempty"`
}

type Info struct {
	Title          string   `json:"title"`
	Description    string   `json:"description,omitempty"`
	Version        string   `json:"version"`
	TermsOfService string   `json:"termsOfService,omitempty"`
	Contact        *Contact `json:"contact,omitempty"`
	License        *License `json:"license,omitempty"`
}

type Contact struct {
	Name  string `json:"name,omitempty"`
	URL   string `json:"url,omitempty"`
	Email string `json:"email,omitempty"`
}

type License struct {
	Name string `json:"name"`
	URL  string `json:"url,omitempty"`
}

type Server struct {
	URL         string                     `json:"url"`
	Description string                     `json:"description,omitempty"`
	Variables   map[string]ServerVariable  `json:"variables,omitempty"`
}

type ServerVariable struct {
	Enum        []string `json:"enum,omitempty"`
	Default     string   `json:"default"`
	Description string   `json:"description,omitempty"`
}

type PathItem struct {
	Summary     string     `json:"summary,omitempty"`
	Description string     `json:"description,omitempty"`
	Get         *Operation `json:"get,omitempty"`
	Put         *Operation `json:"put,omitempty"`
	Post        *Operation `json:"post,omitempty"`
	Delete      *Operation `json:"delete,omitempty"`
	Options     *Operation `json:"options,omitempty"`
	Head        *Operation `json:"head,omitempty"`
	Patch       *Operation `json:"patch,omitempty"`
	Parameters  []Parameter `json:"parameters,omitempty"`
}

type Operation struct {
	Tags         []string              `json:"tags,omitempty"`
	Summary      string                `json:"summary,omitempty"`
	Description  string                `json:"description,omitempty"`
	OperationID  string                `json:"operationId,omitempty"`
	Parameters   []Parameter           `json:"parameters,omitempty"`
	RequestBody  *RequestBody          `json:"requestBody,omitempty"`
	Responses    map[string]Response   `json:"responses"`
	Security     []SecurityRequirement `json:"security,omitempty"`
	Deprecated   bool                  `json:"deprecated,omitempty"`
}

type Parameter struct {
	Name            string  `json:"name"`
	In              string  `json:"in"` // "query", "header", "path", "cookie"
	Description     string  `json:"description,omitempty"`
	Required        bool    `json:"required,omitempty"`
	Deprecated      bool    `json:"deprecated,omitempty"`
	AllowEmptyValue bool    `json:"allowEmptyValue,omitempty"`
	Schema          *Schema `json:"schema,omitempty"`
	Example         any     `json:"example,omitempty"`
}

type RequestBody struct {
	Description string                `json:"description,omitempty"`
	Content     map[string]MediaType  `json:"content"`
	Required    bool                  `json:"required,omitempty"`
}

type Response struct {
	Description string               `json:"description"`
	Headers     map[string]Header    `json:"headers,omitempty"`
	Content     map[string]MediaType `json:"content,omitempty"`
}

type MediaType struct {
	Schema   *Schema `json:"schema,omitempty"`
	Example  any     `json:"example,omitempty"`
	Examples map[string]Example `json:"examples,omitempty"`
}

type Example struct {
	Summary       string `json:"summary,omitempty"`
	Description   string `json:"description,omitempty"`
	Value         any    `json:"value,omitempty"`
	ExternalValue string `json:"externalValue,omitempty"`
}

type Header struct {
	Description     string  `json:"description,omitempty"`
	Required        bool    `json:"required,omitempty"`
	Deprecated      bool    `json:"deprecated,omitempty"`
	AllowEmptyValue bool    `json:"allowEmptyValue,omitempty"`
	Schema          *Schema `json:"schema,omitempty"`
}

type Schema struct {
	Type                 string             `json:"type,omitempty"`
	Format               string             `json:"format,omitempty"`
	Title                string             `json:"title,omitempty"`
	Description          string             `json:"description,omitempty"`
	Default              any                `json:"default,omitempty"`
	Example              any                `json:"example,omitempty"`
	Required             []string           `json:"required,omitempty"`
	Properties           map[string]*Schema `json:"properties,omitempty"`
	Items                *Schema            `json:"items,omitempty"`
	AdditionalProperties any                `json:"additionalProperties,omitempty"`
	Enum                 []any              `json:"enum,omitempty"`
	AllOf                []*Schema          `json:"allOf,omitempty"`
	OneOf                []*Schema          `json:"oneOf,omitempty"`
	AnyOf                []*Schema          `json:"anyOf,omitempty"`
	Not                  *Schema            `json:"not,omitempty"`
	MinLength            *int               `json:"minLength,omitempty"`
	MaxLength            *int               `json:"maxLength,omitempty"`
	Pattern              string             `json:"pattern,omitempty"`
	Minimum              *float64           `json:"minimum,omitempty"`
	Maximum              *float64           `json:"maximum,omitempty"`
	ExclusiveMinimum     *float64           `json:"exclusiveMinimum,omitempty"`
	ExclusiveMaximum     *float64           `json:"exclusiveMaximum,omitempty"`
	MinItems             *int               `json:"minItems,omitempty"`
	MaxItems             *int               `json:"maxItems,omitempty"`
	UniqueItems          bool               `json:"uniqueItems,omitempty"`
}

type Components struct {
	Schemas         map[string]*Schema         `json:"schemas,omitempty"`
	Responses       map[string]Response        `json:"responses,omitempty"`
	Parameters      map[string]Parameter       `json:"parameters,omitempty"`
	Examples        map[string]Example         `json:"examples,omitempty"`
	RequestBodies   map[string]RequestBody     `json:"requestBodies,omitempty"`
	Headers         map[string]Header          `json:"headers,omitempty"`
	SecuritySchemes map[string]SecurityScheme  `json:"securitySchemes,omitempty"`
}

type SecurityScheme struct {
	Type             string `json:"type"`
	Description      string `json:"description,omitempty"`
	Name             string `json:"name,omitempty"`
	In               string `json:"in,omitempty"`
	Scheme           string `json:"scheme,omitempty"`
	BearerFormat     string `json:"bearerFormat,omitempty"`
	OpenIdConnectUrl string `json:"openIdConnectUrl,omitempty"`
}

type SecurityRequirement map[string][]string

type Tag struct {
	Name         string        `json:"name"`
	Description  string        `json:"description,omitempty"`
	ExternalDocs *ExternalDocs `json:"externalDocs,omitempty"`
}

type ExternalDocs struct {
	Description string `json:"description,omitempty"`
	URL         string `json:"url"`
}

// NewSpec creates a new OpenAPI specification with defaults
func NewSpec() *OpenAPISpec {
	return &OpenAPISpec{
		OpenAPI: "3.0.3",
		Info: Info{
			Title:   "API Documentation",
			Version: "1.0.0",
		},
		Paths:      make(map[string]PathItem),
		Components: &Components{
			Schemas:         make(map[string]*Schema),
			Responses:       make(map[string]Response),
			Parameters:      make(map[string]Parameter),
			SecuritySchemes: make(map[string]SecurityScheme),
		},
	}
}

// SetInfo sets the API information
func (s *OpenAPISpec) SetInfo(title, version, description string) *OpenAPISpec {
	s.Info.Title = title
	s.Info.Version = version
	s.Info.Description = description
	return s
}

// AddServer adds a server to the specification
func (s *OpenAPISpec) AddServer(url, description string) *OpenAPISpec {
	s.Servers = append(s.Servers, Server{
		URL:         url,
		Description: description,
	})
	return s
}

// AddSecurityScheme adds a security scheme
func (s *OpenAPISpec) AddSecurityScheme(name string, scheme SecurityScheme) *OpenAPISpec {
	if s.Components == nil {
		s.Components = &Components{
			SecuritySchemes: make(map[string]SecurityScheme),
		}
	}
	if s.Components.SecuritySchemes == nil {
		s.Components.SecuritySchemes = make(map[string]SecurityScheme)
	}
	s.Components.SecuritySchemes[name] = scheme
	return s
}

// AddBearerAuth adds JWT bearer authentication scheme
func (s *OpenAPISpec) AddBearerAuth() *OpenAPISpec {
	return s.AddSecurityScheme("bearerAuth", SecurityScheme{
		Type:         "http",
		Scheme:       "bearer",
		BearerFormat: "JWT",
		Description:  "JWT Bearer token authentication",
	})
}

// AddPath adds or updates a path in the specification
func (s *OpenAPISpec) AddPath(path string, pathItem PathItem) *OpenAPISpec {
	if s.Paths == nil {
		s.Paths = make(map[string]PathItem)
	}
	s.Paths[path] = pathItem
	return s
}

// AddOperation adds an operation to a specific path and method
func (s *OpenAPISpec) AddOperation(path, method string, operation Operation) *OpenAPISpec {
	if s.Paths == nil {
		s.Paths = make(map[string]PathItem)
	}
	
	pathItem, exists := s.Paths[path]
	if !exists {
		pathItem = PathItem{}
	}
	
	switch method {
	case "GET":
		pathItem.Get = &operation
	case "POST":
		pathItem.Post = &operation
	case "PUT":
		pathItem.Put = &operation
	case "DELETE":
		pathItem.Delete = &operation
	case "PATCH":
		pathItem.Patch = &operation
	case "OPTIONS":
		pathItem.Options = &operation
	case "HEAD":
		pathItem.Head = &operation
	}
	
	s.Paths[path] = pathItem
	return s
}

// AddSchema adds a reusable schema component
func (s *OpenAPISpec) AddSchema(name string, schema *Schema) *OpenAPISpec {
	if s.Components == nil {
		s.Components = &Components{
			Schemas: make(map[string]*Schema),
		}
	}
	if s.Components.Schemas == nil {
		s.Components.Schemas = make(map[string]*Schema)
	}
	s.Components.Schemas[name] = schema
	return s
}

// ToJSON converts the specification to JSON
func (s *OpenAPISpec) ToJSON() ([]byte, error) {
	return json.MarshalIndent(s, "", "  ")
}

// ToJSONString converts the specification to JSON string
func (s *OpenAPISpec) ToJSONString() (string, error) {
	data, err := s.ToJSON()
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// GetCreatedAt returns the current timestamp for spec generation
func (s *OpenAPISpec) GetCreatedAt() string {
	return time.Now().Format(time.RFC3339)
}