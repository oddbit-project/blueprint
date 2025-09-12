# blueprint

![go-version](https://img.shields.io/github/go-mod/go-version/oddbit-project/blueprint)
[![Release](https://img.shields.io/github/v/release/oddbit-project/blueprint)](https://github.com/oddbit-project/blueprint/releases)
[![Build Status](https://github.com/oddbit-project/blueprint/actions/workflows/run-tests.yml/badge.svg?branch=main)](https://github.com/oddbit-project/blueprint/actions/workflows/run-tests.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/oddbit-project/blueprint)](https://goreportcard.com/report/github.com/oddbit-project/blueprint)

---

**Documentation:** [https://oddbit-project.github.io/blueprint/](https://oddbit-project.github.io/blueprint/)

---

Blueprint is a modular Go application framework for building web applications and microservices. Starting from v0.8.0, Blueprint features a **modular architecture** that allows you to import only the components you need, reducing dependencies and improving build times.

## Table of Contents

- [Features](#features)  
- [Application Example](#application-example)
- [Core Framework](#-core-framework)
- [Available Providers](#-available-providers)
- [License](#license-information)


### Installation

```bash
# Install core framework
go get github.com/oddbit-project/blueprint

# Install specific providers as needed
go get github.com/oddbit-project/blueprint/provider/httpserver
go get github.com/oddbit-project/blueprint/provider/kafka
go get github.com/oddbit-project/blueprint/provider/pgsql
```

### Using Providers

```go
// Import specific providers
import "github.com/oddbit-project/blueprint/provider/httpserver"
import "github.com/oddbit-project/blueprint/provider/kafka" 
import "github.com/oddbit-project/blueprint/provider/pgsql"

// Core framework imports work as before
import "github.com/oddbit-project/blueprint"
import "github.com/oddbit-project/blueprint/log"
```

## Features

Blueprint provides:

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
  "github.com/oddbit-project/blueprint/provider/httpserver/response"
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
  *blueprint.Container                    // runnable application container
  args                 *CliArgs           // cli args
  httpServer           *httpserver.Server // our api server
  logger               *log.Logger
  jwtProvider          jwtprovider.JWTProvider // JWT authentication provider
}

// command-line args
var cliArgs = &CliArgs{
  ConfigFile:  flag.String("c", "config/config.json", "Config file"),
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
    Container:   blueprint.NewContainer(cfg),
    args:        args,
    httpServer:  nil,
    logger:      logger,
    jwtProvider: nil,
  }, nil
}

func (a *Application) Build() {
  // assemble internal dependencies of application
  // if some error occurs, generate fatal error & abort execution
  var err error

  // initialize http server
  a.logger.Info("Building Sample Application...")

  // initialize JWT provider from configuration
  jwtConfig := jwtprovider.NewJWTConfig()
  a.AbortFatal(a.Config.GetKey("jwt", jwtConfig))

  // optional - create a revocation manager instance for token revocation
  revocationManager := jwtprovider.NewRevocationManager(jwtprovider.NewMemoryRevocationBackend())

  // create the JWT provider to use with the API server
  a.jwtProvider, err = jwtprovider.NewProvider(jwtConfig, jwtprovider.WithRevocationManager(revocationManager))
  a.AbortFatal(err)

  a.jwtProvider, err = jwtprovider.NewProvider(jwtConfig)
  a.AbortFatal(err)

  // initialize http server config
  httpConfig := httpserver.NewServerConfig()
  // fill parameters from config provider
  a.AbortFatal(a.Config.GetKey("server", httpConfig))

  // Create http server from config
  a.httpServer, err = httpConfig.NewServer(a.logger)
  a.AbortFatal(err)

  // Apply security middleware
  a.httpServer.UseDefaultSecurityHeaders()
  a.httpServer.UseCSRFProtection()
  a.httpServer.UseRateLimiting(60) // 60 requests per minute

  // Create router group
  v1 := a.httpServer.Route().Group("/v1")

  // Public login endpoint
  a.httpServer.Route().POST("/login", func(ctx *gin.Context) {
    // Basic authentication logic (replace with your auth system)
    var loginData struct {
      Username string `json:"username" binding:"required"`
      Password string `json:"password" binding:"required"`
    }

    // bind request params
    if !httpserver.ValidateJSON(ctx, &loginData) {
      // if ValidateJSON() fails, error response was already sent
      return
    }

    // Validate credentials (implement your validation logic)
    if loginData.Username == "demo" && loginData.Password == "password" {
      token, err := a.jwtProvider.GenerateToken("demo", map[string]interface{}{
        "role": "user",
      })
      if err != nil {
        response.Http400(ctx, "Token generation failed")
        return
      }

      response.Success(ctx, gin.H{"token": token})
      return
    }

    response.Http401(ctx)
  })

  // Create JWT auth middleware and enable middleware
  jwtAuth := auth.NewAuthJWT(a.jwtProvider)
  v1.Use(auth.AuthMiddleware(jwtAuth))

  // protected endpoint: /v1/hello
  v1.GET("/hello", func(ctx *gin.Context) {
    token, _ := auth.GetJWTToken(ctx)
    claims, err := a.jwtProvider.ParseToken(token)
    if err != nil {
      response.Http400(ctx, "Token generation failed")
      return
    }

    response.Success(ctx, gin.H{
      "id":   claims.ID,
      "user": claims.Subject,
      "data": claims.Data,
    })
  })

}

func (a *Application) Start() {

  // register http destructor callback
  blueprint.RegisterDestructor(func() error {
    return a.httpServer.Shutdown(a.GetContext())
  })

  // Start  application - http server
  a.Run(func(app interface{}) error {
    go func() {
      a.logger.Infof("Running Sample Application API at http://%s:%d", a.httpServer.Config.Host, a.httpServer.Config.Port)
      a.logger.Infof("Login: POST /login (username: demo, password: password)")
      a.logger.Infof("Protected: GET /v1/hello (requires JWT token)")
      a.AbortFatal(a.httpServer.Start())
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
  app.Start()
}
```

Example config file *sample.json*:
```json
{
  "server": {
    "host": "localhost",
    "port": 8080,
    "readTimeout": 30,
    "writeTimeout": 30,
    "debug": true,
    "tlsEnable": false
  },
  "jwt": {
    "signingKey": {
      "password": "your-jwt-secret-key-change-in-production"
    },
    "signingAlgorithm": "HS256",
    "expirationSeconds": 3600,
    "issuer": "sample-application",
    "audience": "api-users",
    "keyID": "default",
    "requireIssuer": true,
    "requireAudience": true,
    "trackUserTokens": false,
    "maxUserSessions": 0,
    "maxTokenSize": 4096
  }
}
```


## Core Framework

Blueprint's base library provides the following components for application development:

### **Application Lifecycle**
- **[Container](docs/index.md)** - Application lifecycle management with graceful startup/shutdown
- **[Configuration](docs/config/config.md)** - JSON and environment-based configuration with validation
- **[Logging](docs/log/logging.md)** - Structured logging with file rotation and levels

### **Database**
- **[Database Package](docs/db/index.md)** - Database abstraction layer with repository pattern
- **[Repository Pattern](docs/db/repository.md)** - Clean architecture database access
- **[Query Builder](docs/db/query-builder.md)** - SQL query building with type safety
- **[Data Grid](docs/db/dbgrid.md)** - Advanced filtering, sorting, and pagination
- **[Migrations](docs/db/migrations.md)** - Database schema versioning

### **Security & Cryptography**
- **[Password Hashing](docs/crypt/password-hashing.md)** - Argon2id password hashing with timing attack protection
- **[Secure Credentials](docs/crypt/secure-credentials.md)** - In-memory credential encryption and key management
- **[TLS](docs/provider/tls.md)** - TLS/SSL certificate management

### **Utilities**
- **[ThreadPool](docs/threadpool/threadpool.md)** - High-performance worker pool with graceful shutdown
- **[BatchWriter](docs/batchwriter/batchwriter.md)** - Batched operations with time and size-based flushing

```bash
# Core framework includes all these components
go get github.com/oddbit-project/blueprint
```

### Core Usage Example

```go
package main

import (
    "github.com/oddbit-project/blueprint"
    "github.com/oddbit-project/blueprint/config/provider"
    "github.com/oddbit-project/blueprint/log"
    "github.com/oddbit-project/blueprint/db"
)

func main() {
    // Initialize logging
    log.Configure(log.NewDefaultConfig())
    logger := log.New("my-app")
    
    // Load configuration
    cfg, _ := provider.NewJsonProvider("config.json")
    
    // Create application container
    app := blueprint.NewContainer()
    
    // Your application logic here
    logger.Info("Application started")
    
    // Run application with graceful shutdown
    app.Run()
}
```

## Available Providers

Blueprint also includes the following modular providers:

### **Message Queues & Communication**
- **[Kafka](docs/provider/kafka.md)** - Apache Kafka integration with SASL authentication
- **[NATS](docs/provider/nats.md)** - NATS messaging system integration  
- **[MQTT](docs/provider/mqtt.md)** - MQTT broker communication

### **Databases & Storage**
- **[PostgreSQL](docs/provider/pgsql.md)** - PostgreSQL database integration with connection pooling
- **[ClickHouse](docs/provider/clickhouse.md)** - ClickHouse database for analytics workloads
- **[S3](docs/provider/s3.md)** - S3-compatible object storage (AWS S3, MinIO, etc.)
- **[etcd](docs/provider/etcd.md)** - Distributed key-value store integration

### **Web & HTTP**  
- **[HTTP Server](docs/provider/httpserver/index.md)** - Complete HTTP server with middleware, security, and routing
- **[Metrics](docs/provider/metrics.md)** - Prometheus metrics endpoint

### **Authentication & Security**
- **[JWT Provider](docs/provider/jwtprovider.md)** - JWT token authentication and management
- **[HMAC Provider](docs/provider/hmacprovider.md)** - HMAC-based request authentication
- **[htpasswd](docs/provider/htpasswd.md)** - Apache htpasswd file authentication

### **Utilities**
- **[SMTP](docs/provider/smtp.md)** - Email sending functionality

Each provider is independently versioned and can be imported separately. See the [documentation](https://oddbit-project.github.io/blueprint/) for detailed usage examples.

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
