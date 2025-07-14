package main

import (
	"flag"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/oddbit-project/blueprint"
	"github.com/oddbit-project/blueprint/config/provider"
	"github.com/oddbit-project/blueprint/log"
	"github.com/oddbit-project/blueprint/provider/httpserver"
	"github.com/oddbit-project/blueprint/utils"
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
	utils.PanicOnError(log.Configure(log.NewDefaultConfig()))

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
