# blueprint

![go-version](https://img.shields.io/github/go-mod/go-version/oddbit-project/blueprint)
![go-build](https://img.shields.io/github/actions/workflow/status/oddbit-project/blueprint/run-tests.yml)
![go-reportcard](https://goreportcard.com/badge/github.com/oddbit-project/blueprint)


Go application framework for building web applications and microservices with built-in support for:

- Container-based application lifecycle management
- Configuration management
- Structured logging with file rotation
- HTTP server with security features (TLS, CSRF protection, security headers, session management)
- Database connectivity (PostgreSQL, ClickHouse)
- Message queue integration (Kafka, MQTT)
- Metrics endpoint for Prometheus monitoring


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

Simple API application example using blueprint.Container:

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
 container  *blueprint.Container // runnable application container
 args       *CliArgs             // cli args
 httpServer *httpserver.Server   // our api server
 logger     *log.Logger
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
  container:  blueprint.NewContainer(cfg),
  args:       args,
  httpServer: nil,
  logger:     logger,
 }, nil
}

func (a *Application) Build() {
 // assemble internal dependencies of application
 // if some error occurs, generate fatal error & abort execution

 // initialize http server
 a.logger.Info("Building Sample Application...")

 // initialize http server config
 httpConfig := httpserver.NewServerConfig()
 // fill parameters from config provider
 if err := a.container.Config.GetKey("server", httpConfig); err != nil {
  a.container.AbortFatal(err)
 }
 // Create http server from config
 var err error
 a.httpServer, err = httpConfig.NewServer(a.logger)
 a.container.AbortFatal(err)

 // add http handler
 // endpoint: /v1/hello
 a.httpServer.Route().Group("/v1").GET(
  "/hello",
  func(ctx *gin.Context) {
   ctx.JSON(200, "Hello World")
  },
 )
}

func (a *Application) Run() {
 // register http destructor callback
 blueprint.RegisterDestructor(func() error {
  return a.httpServer.Shutdown(a.container.GetContext())
 })

 // Start  application - http server
 a.container.Run(func(app interface{}) error {
  go func() {
   a.logger.Infof("Running Sample Application API at https://%s:%d/v1/hello", a.httpServer.Config.Host, a.httpServer.Config.Port)
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
