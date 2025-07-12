# blueprint

![go-version](https://img.shields.io/github/go-mod/go-version/oddbit-project/blueprint)
![go-build](https://img.shields.io/github/actions/workflow/status/oddbit-project/blueprint/run-tests.yml)
![go-reportcard](https://goreportcard.com/badge/github.com/oddbit-project/blueprint)


Go application framework for building web applications and microservices with built-in support for:

- Container-based application lifecycle management
- Configuration management
- Structured logging with file rotation
- HTTP server with comprehensive security features:
  - TLS support with configurable cipher suites
  - CSRF protection with session-based tokens
  - Security headers (CSP, HSTS, XSS protection, etc.)
  - Rate limiting with per-IP and per-endpoint controls
  - Device fingerprinting for security monitoring
  - Cookie-based session management with multiple backends
- Authentication providers:
  - JWT authentication with symmetric/asymmetric key support
  - Token revocation system
  - HTPasswd file-based authentication
- Database connectivity (PostgreSQL, ClickHouse)
- Message queue integration (Kafka, MQTT)
- Metrics endpoint for Prometheus monitoring
- Middleware system with request helpers and utilities


## Application example

Applications are executed in a runtime context, instantiated from blueprint.Container. This container holds the main
run cycle, and manages the startup/shutdown cycle of the application.

It is possible to register global destructor functions that will be called back in sequence when the application is
terminated. To achieve this, any ordered application exit must use blueprint.Shutdown():
```go
(...)
if err := doSomething(); err != nil {
	// perform application shutdown
	blueprint.Shutdown(err)
	os.exit(-1)
}
```

Simple API application example using blueprint.Container with JWT authentication and security middleware:

```go
package main

import (
 "flag"
 "fmt"
 "github.com/gin-gonic/gin"
 "github.com/oddbit-project/blueprint"
 "github.com/oddbit-project/blueprint/config/provider"
 "github.com/oddbit-project/blueprint/log"
 "github.com/oddbit-project/blueprint/provider/httpserver"
 "github.com/oddbit-project/blueprint/provider/httpserver/auth"
 "github.com/oddbit-project/blueprint/provider/jwtprovider"
 "os"
)

const (
 VERSION = "9.9.0"
)

// CliArgs Command-line options
type CliArgs struct {
 ConfigFile  *string
 ShowVersion *bool
}

// Application sample application container
type Application struct {
 container   *blueprint.Container      // runnable application container
 args        *CliArgs                  // cli args
 httpServer  *httpserver.Server        // our api server
 logger      *log.Logger
 jwtProvider jwtprovider.JWTProvider   // JWT authentication provider
}

// command-line args
var cliArgs = &CliArgs{
 ConfigFile:  flag.String("c", "config/sample.json", "Config file"),
 ShowVersion: flag.Bool("version", false, "Show version"),
}

// NewApplication Sample application factory
func NewApplication(args *CliArgs, logger *log.Logger) (*Application, error) {
 cfg, err := provider.NewJsonProvider(*args.ConfigFile)
 if err != nil {
  return nil, err
 }
 if logger == nil {
  logger = log.New("application")
 }
 return &Application{
  container:   blueprint.NewContainer(cfg),
  args:        args,
  httpServer:  nil,
  logger:      logger,
  jwtProvider: nil,
 }, nil
}

func (a *Application) Build() {
 // assemble internal dependencies of application
 // if some error occurs, generate fatal error & abort execution

 // initialize http server
 a.logger.Info("Building Sample Application...")

 // initialize JWT provider
 jwtConfig := jwtprovider.NewJWTConfig()
 if err := a.container.Config.GetKey("jwt", jwtConfig); err != nil {
  a.container.AbortFatal(err)
 }
 var err error
 a.jwtProvider, err = jwtprovider.NewProvider(jwtConfig)
 a.container.AbortFatal(err)

 // initialize http server config
 httpConfig := httpserver.NewServerConfig()
 // fill parameters from config provider
 if err := a.container.Config.GetKey("server", httpConfig); err != nil {
  a.container.AbortFatal(err)
 }
 // Create http server from config
 a.httpServer, err = httpConfig.NewServer(a.logger)
 a.container.AbortFatal(err)

 // Apply security middleware
 a.httpServer.UseDefaultSecurityHeaders()
 a.httpServer.UseCSRFProtection()
 a.httpServer.UseRateLimiting(60) // 60 requests per minute

 // Create JWT auth middleware
 jwtAuth := auth.NewAuthJWT(a.jwtProvider)

 // Add protected routes with JWT authentication
 v1 := a.httpServer.Route().Group("/v1")
 v1.Use(auth.AuthMiddleware(jwtAuth))
 
 // endpoint: /v1/hello (protected)
 v1.GET("/hello", func(ctx *gin.Context) {
  claims, _ := auth.GetClaims(ctx)
  ctx.JSON(200, gin.H{
   "message": "Hello World",
   "user":    claims.Subject,
   "data":    claims.Data,
  })
 })

 // Public login endpoint
 a.httpServer.Route().POST("/login", func(ctx *gin.Context) {
  // Basic authentication logic (replace with your auth system)
  var loginData struct {
   Username string `json:"username"`
   Password string `json:"password"`
  }
  
  if err := ctx.ShouldBindJSON(&loginData); err != nil {
   ctx.JSON(400, gin.H{"error": "Invalid request"})
   return
  }
  
  // Validate credentials (implement your validation logic)
  if loginData.Username == "demo" && loginData.Password == "password" {
   token, err := a.jwtProvider.GenerateToken("demo", map[string]interface{}{
    "role": "user",
   })
   if err != nil {
    ctx.JSON(500, gin.H{"error": "Token generation failed"})
    return
   }
   ctx.JSON(200, gin.H{"token": token})
  } else {
   ctx.JSON(401, gin.H{"error": "Invalid credentials"})
  }
 })
}

func (a *Application) Run() {
 // register http destructor callback
 blueprint.RegisterDestructor(func() error {
  return a.httpServer.Shutdown(a.container.GetContext())
 })

 // Start  application - http server
 a.container.Run(func(app interface{}) error {
  go func() {
   a.logger.Infof("Running Sample Application API at https://%s:%d", a.httpServer.Config.Host, a.httpServer.Config.Port)
   a.logger.Infof("Login: POST /login (username: demo, password: password)")
   a.logger.Infof("Protected: GET /v1/hello (requires JWT token)")
   a.container.AbortFatal(a.httpServer.Start())
  }()
  return nil
 })
}

func main() {
 // config logger
 log.Configure(log.NewDefaultConfig())
 logger := log.New("sample-application")

 flag.Parse()

 if *cliArgs.ShowVersion {
  fmt.Printf("Version: %s\n", VERSION)
  os.Exit(0)
 }

 app, err := NewApplication(cliArgs, logger)
 if err != nil {
  logger.Error(err, "Initialization failed")
  os.Exit(-1)
 }

 // build application
 app.Build()
 // execute application
 app.Run()
}
```

## Documentation

For comprehensive guides and API documentation, see:

- **HTTP Server Framework**: [docs/provider/httpserver/index.md](docs/provider/httpserver/index.md)
- **Security & Authentication**: [docs/provider/httpserver/auth.md](docs/provider/httpserver/auth.md)
- **Middleware System**: [docs/provider/httpserver/middleware.md](docs/provider/httpserver/middleware.md)
- **CSRF Protection**: [docs/provider/httpserver/security.md](docs/provider/httpserver/security.md)
- **Session Management**: [docs/provider/httpserver/session.md](docs/provider/httpserver/session.md)
- **Performance Guide**: [docs/provider/httpserver/performance.md](docs/provider/httpserver/performance.md)
- **API Reference**: [docs/provider/httpserver/api-reference.md](docs/provider/httpserver/api-reference.md)
- **Examples**: [docs/provider/httpserver/examples.md](docs/provider/httpserver/examples.md)
- **Troubleshooting**: [docs/provider/httpserver/troubleshooting.md](docs/provider/httpserver/troubleshooting.md)

## License information

All the custom code is licensed under Apache2 license. Some code pieces were copied or imported from different sources,
and have their own licensing info. All the adapted files contain a full copy of their respective license, and all
the adaptations are licensed under the original license of the source.


The repository includes code adapted from the following sources:

[Argon2Id password hashing](https://github.com/alexedwards/argon2id)
(c) Alex Edwards, MIT License


[Telegraf TLS plugin](https://github.com/influxdata/telegraf/tree/master/plugins/common/tls)
(c) InfluxData Inc, MIT License


[Threadsafe package](https://github.com/hayageek/threadsafe)
 (c) Ravishanker Kusuma, MIT License
