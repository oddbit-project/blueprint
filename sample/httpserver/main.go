package main

import (
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/oddbit-project/blueprint/log"
	"github.com/oddbit-project/blueprint/provider/httpserver"
	"net/http"
	"os"
)

func main() {

	log.Configure(log.NewDefaultConfig())

	srvConfig := httpserver.NewServerConfig()
	srvConfig.Host = "localhost"
	srvConfig.Port = 8089
	srvConfig.Debug = true

	server, err := httpserver.NewServer(srvConfig)
	if err != nil {
		log.Fatal(context.Background(), err, "could not start http server")
		os.Exit(1)
	}

	server.Route().GET("/hello", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "hello!",
		})
	})

	// start http server
	server.Start()

	fmt.Println("Done!")
}
