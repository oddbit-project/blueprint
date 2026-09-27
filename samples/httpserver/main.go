package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/oddbit-project/blueprint/log"
	"github.com/oddbit-project/blueprint/provider/httpserver"
	"net/http"
	"os"
)

func main() {
	// config logger
	_ = log.Configure(log.NewDefaultConfig())
	logger := log.New("http-server")

	srvConfig := httpserver.NewServerConfig()
	srvConfig.Host = "localhost"
	srvConfig.Port = 8089
	srvConfig.Debug = true

	server, err := httpserver.NewServer(srvConfig, logger)
	if err != nil {
		logger.Fatal(err, "could not start http server")
		os.Exit(1)
	}

	server.Route().GET("/hello", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "hello!",
		})
	})

	// start http server
	if err := server.Start(); err != nil {
		logger.Fatal(err, "could not start http server")
	}

	fmt.Println("Done!")
}
