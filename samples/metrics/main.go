package main

import (
	"fmt"
	"github.com/oddbit-project/blueprint/provider/metrics"
	"log"
)

func main() {
	srvConfig := metrics.NewConfig()
	server, err := srvConfig.NewServer()
	if err != nil {
		log.Fatal(err)
	}

	// Start prometheus http server on http://localhost:2201/metrics
	fmt.Println("exposing metrics on http://localhost:2201/metrics...")
	server.Start()

	fmt.Println("Done!")
}
