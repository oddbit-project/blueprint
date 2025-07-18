package main

import (
	"encoding/json"
	"github.com/oddbit-project/blueprint"
	"github.com/oddbit-project/blueprint/config/provider"
	"github.com/oddbit-project/blueprint/log"
)

// CliArgs Command-line options
type CliArgs struct {
	ConfigFile *string
	DumpConfig *bool
}

// Application application runtime
type Application struct {
	*blueprint.Container          // runnable application runtime
	args                 *CliArgs // cli args
	logger               *log.Logger
}

// NewApplication application factory
func NewApplication(args *CliArgs, logger *log.Logger) (*Application, error) {
	cfg, err := provider.NewJsonProvider(*args.ConfigFile)
	if err != nil {
		return nil, err
	}
	if logger == nil {
		panic("logger is nil")
	}
	return &Application{
		Container: blueprint.NewContainer(cfg),
		args:      args,
		logger:    logger,
	}, nil
}

// Build runtime
func (a *Application) Build() {
	var err error

	// parse & validate config
	a.logger.Info("Loading Configuration...")

	cfg := NewConfig()
	a.AbortFatal(a.Config.Get(cfg))
	a.AbortFatal(cfg.Validate())

	// initialize global logger
	a.logger.Info("Initializing Logging...")

	// create a new global logger based on the configuration
	a.AbortFatal(log.Configure(cfg.Log))

	a.logger.Info("Initializing API...")
	server, err := NewApiServer(cfg)
	a.AbortFatal(err)

	// register api server shutdown
	blueprint.RegisterDestructor(func() error {
		_ = server.Stop(a.Context)
		return nil
	})

	// Start http server
	go func() {
		a.logger.Infof("Running API server at %s:%d", cfg.Api.Host, cfg.Api.Port)
		a.AbortFatal(server.Start())
	}()
}

func (a *Application) Start() {
	// typically, one would run a list of anonymous functions to initialize different
	// components:
	// a.Run(func(app interface{}) error {
	// 	 go func() {
	//		 // initialize myComponent in a separate goroutine
	//		 a.myComponent.Start(a.Context)
	// 	 }()
	// 	 return nil
	// })

	// in this case, API server was already initialized in the build phase
	// so no custom initialization required
	a.Run()
}

func DumpDefaultConfig() (string, error) {
	cfg := NewConfig()
	serialized, err := json.MarshalIndent(cfg, "", "  ")
	return string(serialized), err
}
