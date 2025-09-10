package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/oddbit-project/blueprint"
	"github.com/oddbit-project/blueprint/config/provider"
	"github.com/oddbit-project/blueprint/log"
	"github.com/oddbit-project/blueprint/provider/httpserver"
	"github.com/oddbit-project/blueprint/provider/httpserver/openapi"
	"github.com/oddbit-project/blueprint/provider/httpserver/response"
)

const VERSION = "1.0.0"

// CliArgs Command-line options
type CliArgs struct {
	ConfigFile  *string
	ShowVersion *bool
}

// Application demonstrates OpenAPI integration
type Application struct {
	*blueprint.Container
	args       *CliArgs
	httpServer *httpserver.Server
	logger     *log.Logger
}

// User represents a user in the system
type User struct {
	ID        int       `json:"id" doc:"Unique user identifier" example:"123"`
	Name      string    `json:"name" binding:"required" doc:"User's full name" example:"John Doe"`
	Email     string    `json:"email" binding:"required,email" doc:"Valid email address" example:"john@example.com"`
	Age       int       `json:"age" binding:"min=0,max=120" doc:"User's age" example:"30"`
	Active    bool      `json:"active" doc:"Whether the user is active" example:"true"`
	CreatedAt time.Time `json:"created_at" doc:"User creation timestamp"`
	Profile   *Profile  `json:"profile,omitempty" doc:"User profile information"`
}

// Profile represents additional user information
type Profile struct {
	Bio     string   `json:"bio" doc:"User biography" example:"Software engineer with 5 years of experience"`
	Skills  []string `json:"skills" doc:"List of user skills" example:"Go,JavaScript,Docker"`
	Website string   `json:"website" binding:"url" doc:"User's personal website" example:"https://johndoe.com"`
}

// CreateUserRequest represents the request for creating a user
type CreateUserRequest struct {
	Name    string   `json:"name" binding:"required" doc:"User's full name" example:"Jane Smith"`
	Email   string   `json:"email" binding:"required,email" doc:"Valid email address" example:"jane@example.com"`
	Age     int      `json:"age" binding:"min=0,max=120" doc:"User's age" example:"25"`
	Profile *Profile `json:"profile,omitempty" doc:"Optional user profile"`
}

// UpdateUserRequest represents the request for updating a user
type UpdateUserRequest struct {
	Name    *string  `json:"name,omitempty" doc:"User's full name" example:"Jane Updated"`
	Email   *string  `json:"email,omitempty" binding:"email" doc:"Valid email address" example:"jane.updated@example.com"`
	Age     *int     `json:"age,omitempty" binding:"min=0,max=120" doc:"User's age" example:"26"`
	Active  *bool    `json:"active,omitempty" doc:"Whether the user is active" example:"false"`
	Profile *Profile `json:"profile,omitempty" doc:"User profile information"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error   string `json:"error" doc:"Error message" example:"User not found"`
	Code    string `json:"code" doc:"Error code" example:"USER_NOT_FOUND"`
	Details string `json:"details,omitempty" doc:"Additional error details"`
}

// ListUsersResponse represents the response for listing users
type ListUsersResponse struct {
	Users []User `json:"users" doc:"List of users"`
	Total int    `json:"total" doc:"Total number of users" example:"100"`
	Page  int    `json:"page" doc:"Current page number" example:"1"`
	Limit int    `json:"limit" doc:"Number of users per page" example:"10"`
}

// command-line args
var cliArgs = &CliArgs{
	ConfigFile:  flag.String("c", "config.json", "Config file"),
	ShowVersion: flag.Bool("version", false, "Show version"),
}

// NewApplication creates a new application instance
func NewApplication(args *CliArgs, logger *log.Logger) (*Application, error) {
	cfg, err := provider.NewJsonProvider(*args.ConfigFile)
	if err != nil {
		// Fallback to default config using JSON bytes
		defaultConfig := `{
			"server": {
				"host": "localhost",
				"port": 8080,
				"readTimeout": 30,
				"writeTimeout": 30,
				"debug": true
			}
		}`
		cfg, err = provider.NewJsonProvider([]byte(defaultConfig))
		if err != nil {
			return nil, err
		}
	}
	
	if logger == nil {
		logger = log.New("openapi-demo")
	}
	
	return &Application{
		Container:  blueprint.NewContainer(cfg),
		args:       args,
		httpServer: nil,
		logger:     logger,
	}, nil
}

func (a *Application) Build() {
	var err error
	
	a.logger.Info("Building OpenAPI Demo Application...")
	
	// Initialize HTTP server config
	httpConfig := httpserver.NewServerConfig()
	a.AbortFatal(a.Config.GetKey("server", httpConfig))
	
	// Create HTTP server
	a.httpServer, err = httpConfig.NewServer(a.logger)
	a.AbortFatal(err)
	
	// Apply security middleware
	//a.httpServer.UseDefaultSecurityHeaders()
	//a.httpServer.UseRateLimiting(60) // 60 requests per minute
	
	// Setup routes
	a.setupRoutes()
	
	// Generate OpenAPI documentation
	a.setupOpenAPIDocumentation()
}

func (a *Application) setupRoutes() {
	// Public login endpoint for getting demo tokens
	a.httpServer.Route().POST("/login", a.login)
	
	// API routes
	api := a.httpServer.Route().Group("/api/v1")
	
	// Users endpoints
	users := api.Group("/users")
	{
		users.GET("", a.listUsers)
		users.POST("", a.createUser)
		users.GET("/:id", a.getUser)
		users.PUT("/:id", a.updateUser)
		users.DELETE("/:id", a.deleteUser)
	}
	
	// Health check endpoint
	a.httpServer.Route().GET("/health", a.healthCheck)
}

func (a *Application) setupOpenAPIDocumentation() {
	// Scan the server to generate OpenAPI specification
	spec := openapi.ScanServer(a.httpServer)
	
	// Configure API information
	spec.SetInfo(
		"User Management API",
		VERSION,
		"A sample API demonstrating OpenAPI integration with Blueprint framework. "+
			"This API provides basic CRUD operations for user management with comprehensive "+
			"OpenAPI 3.0 documentation generated automatically from Go code.",
	)
	
	// Add server information
	spec.AddServer("http://localhost:8080", "Development server")
	
	// Add authentication schemes (even though we're not using auth in this example)
	spec.AddBearerAuth()
	
	// Register all documentation handlers
	openapi.RegisterHandlers(a.httpServer.Route(), spec)
	
	port := a.httpServer.Config.Port
	a.logger.Info("OpenAPI documentation available at:")
	a.logger.Infof("  Documentation Index: http://localhost:%d/docs", port)
	a.logger.Infof("  Swagger UI: http://localhost:%d/swagger", port)
	a.logger.Infof("  ReDoc: http://localhost:%d/redoc", port)
	a.logger.Infof("  OpenAPI Spec: http://localhost:%d/openapi.json", port)
}

// LoginRequest represents the login request
type LoginRequest struct {
	Username string `json:"username" binding:"required" doc:"Username for login" example:"demo"`
	Password string `json:"password" binding:"required" doc:"Password for login" example:"password"`
}

// LoginResponse represents the login response
type LoginResponse struct {
	Token   string `json:"token" doc:"JWT access token" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."`
	Type    string `json:"type" doc:"Token type" example:"Bearer"`
	ExpiresIn int  `json:"expires_in" doc:"Token expiration time in seconds" example:"3600"`
}

// Route handlers

func (a *Application) login(ctx *gin.Context) {
	var req LoginRequest
	
	if !httpserver.ValidateJSON(ctx, &req) {
		return
	}
	
	// Simple demo authentication - in production, use proper password hashing
	if req.Username == "demo" && req.Password == "password" {
		// Generate a demo JWT token (in production, use proper JWT generation)
		token := "demo-jwt-token-12345-" + req.Username
		
		response.Success(ctx, LoginResponse{
			Token:     token,
			Type:      "Bearer",
			ExpiresIn: 3600,
		})
		return
	}
	
	ctx.JSON(401, ErrorResponse{
		Error: "Invalid credentials",
		Code:  "INVALID_CREDENTIALS",
		Details: "Use username: 'demo' and password: 'password' for testing",
	})
}

func (a *Application) listUsers(ctx *gin.Context) {
	// In a real application, you would fetch from database
	users := []User{
		{
			ID:        1,
			Name:      "John Doe",
			Email:     "john@example.com",
			Age:       30,
			Active:    true,
			CreatedAt: time.Now().AddDate(0, -1, 0),
			Profile: &Profile{
				Bio:     "Software engineer with 5 years of experience",
				Skills:  []string{"Go", "JavaScript", "Docker"},
				Website: "https://johndoe.com",
			},
		},
		{
			ID:        2,
			Name:      "Jane Smith",
			Email:     "jane@example.com",
			Age:       25,
			Active:    true,
			CreatedAt: time.Now().AddDate(0, -2, 0),
			Profile: &Profile{
				Bio:     "Frontend developer passionate about UX",
				Skills:  []string{"React", "TypeScript", "CSS"},
				Website: "https://janesmith.dev",
			},
		},
	}
	
	result := ListUsersResponse{
		Users: users,
		Total: len(users),
		Page:  1,
		Limit: 10,
	}
	
	response.Success(ctx, result)
}

func (a *Application) createUser(ctx *gin.Context) {
	var req CreateUserRequest
	
	if !httpserver.ValidateJSON(ctx, &req) {
		return
	}
	
	// In a real application, you would save to database
	user := User{
		ID:        3, // Would be auto-generated
		Name:      req.Name,
		Email:     req.Email,
		Age:       req.Age,
		Active:    true,
		CreatedAt: time.Now(),
		Profile:   req.Profile,
	}
	
	ctx.JSON(201, user)
}

func (a *Application) getUser(ctx *gin.Context) {
	id := ctx.Param("id")
	
	// In a real application, you would fetch from database
	if id == "1" {
		user := User{
			ID:        1,
			Name:      "John Doe",
			Email:     "john@example.com",
			Age:       30,
			Active:    true,
			CreatedAt: time.Now().AddDate(0, -1, 0),
			Profile: &Profile{
				Bio:     "Software engineer with 5 years of experience",
				Skills:  []string{"Go", "JavaScript", "Docker"},
				Website: "https://johndoe.com",
			},
		}
		response.Success(ctx, user)
		return
	}
	
	ctx.JSON(404, ErrorResponse{
		Error: "User not found",
		Code:  "USER_NOT_FOUND",
	})
}

func (a *Application) updateUser(ctx *gin.Context) {
	id := ctx.Param("id")
	
	var req UpdateUserRequest
	if !httpserver.ValidateJSON(ctx, &req) {
		return
	}
	
	// In a real application, you would update in database
	if id == "1" {
		user := User{
			ID:        1,
			Name:      getStringValue(req.Name, "John Doe Updated"),
			Email:     getStringValue(req.Email, "john.updated@example.com"),
			Age:       getIntValue(req.Age, 31),
			Active:    getBoolValue(req.Active, true),
			CreatedAt: time.Now().AddDate(0, -1, 0),
		}
		response.Success(ctx, user)
		return
	}
	
	ctx.JSON(404, ErrorResponse{
		Error: "User not found",
		Code:  "USER_NOT_FOUND",
	})
}

func (a *Application) deleteUser(ctx *gin.Context) {
	id := ctx.Param("id")
	
	// In a real application, you would delete from database
	if id == "1" {
		ctx.Status(204)
		return
	}
	
	ctx.JSON(404, ErrorResponse{
		Error: "User not found",
		Code:  "USER_NOT_FOUND",
	})
}

func (a *Application) healthCheck(ctx *gin.Context) {
	response.Success(ctx, gin.H{
		"status":    "healthy",
		"timestamp": time.Now().UTC(),
		"version":   VERSION,
	})
}

func (a *Application) Start() {
	// Register destructor callback
	blueprint.RegisterDestructor(func() error {
		return a.httpServer.Shutdown(a.GetContext())
	})
	
	// Start application
	a.Run(func(app interface{}) error {
		go func() {
			a.logger.Infof("🚀 OpenAPI Demo API Server running at http://%s:%d", 
				a.httpServer.Config.Host, a.httpServer.Config.Port)
			a.logger.Info("📚 Visit http://localhost:8080/docs for API documentation")
			a.AbortFatal(a.httpServer.Start())
		}()
		return nil
	})
}

func main() {
	// Configure logger
	log.Configure(log.NewDefaultConfig())
	logger := log.New("openapi-demo")
	
	flag.Parse()
	
	if *cliArgs.ShowVersion {
		fmt.Printf("OpenAPI Demo Version: %s\n", VERSION)
		os.Exit(0)
	}
	
	app, err := NewApplication(cliArgs, logger)
	if err != nil {
		logger.Error(err, "Application initialization failed")
		os.Exit(-1)
	}
	
	// Build and start application
	app.Build()
	app.Start()
}

// Helper functions for pointer values
func getStringValue(ptr *string, defaultValue string) string {
	if ptr != nil {
		return *ptr
	}
	return defaultValue
}

func getIntValue(ptr *int, defaultValue int) int {
	if ptr != nil {
		return *ptr
	}
	return defaultValue
}

func getBoolValue(ptr *bool, defaultValue bool) bool {
	if ptr != nil {
		return *ptr
	}
	return defaultValue
}
