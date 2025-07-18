package main

import (
	"flag"
	"fmt"
	"github.com/oddbit-project/blueprint/log"
	"os"
)

// command-line args
var cliArgs = &CliArgs{
	ConfigFile: flag.String("c", "config.json", "Config file"),
	DumpConfig: flag.Bool("d", false, "Dump sample config file"),
}

func main() {
	// Configure logger
	log.Configure(log.NewDefaultConfig())
	logger := log.New("sample-api")

	// parse cli args
	flag.Parse()

	// if DumpConfig, show sample config file and exit
	if *cliArgs.DumpConfig {
		cfg, _ := DumpDefaultConfig()
		fmt.Println(cfg)
		os.Exit(0)
	}

	// Initialize application
	logger.Info("Initializing Application...")

	app, err := NewApplication(cliArgs, logger)
	if err != nil {
		logger.Error(err, "initialization failed")
		os.Exit(-1)
	}

	// build application
	app.Build()

	// execute application
	app.Run()
}
