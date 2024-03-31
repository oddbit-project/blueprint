package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/oddbit-project/blueprint/provider/httpserver"
	"log"
	"net/http"
)

func main() {

	srvConfig := httpserver.NewServerConfig()
	srvConfig.Host = "localhost"
	srvConfig.Port = 8089
	srvConfig.Debug = true

	server, err := httpserver.NewServer(srvConfig)
	if err != nil {
		log.Fatal(err)
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
