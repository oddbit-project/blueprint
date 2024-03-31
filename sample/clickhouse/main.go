package main

import (
	"fmt"
	"github.com/oddbit-project/blueprint/provider/clickhouse"
	"log"
	"os"
)

func main() {
	chConfig := &clickhouse.ClientConfig{
		DSN: "clickhouse://default:password@localhost:9000/default",
	}

	client, err := clickhouse.NewClient(chConfig)
	if err != nil {
		log.Fatal(err)
	}
	if err = client.Connect(); err != nil {
		log.Fatal(err)
	}
	defer client.Disconnect()

	var greeting string
	err = client.Conn.QueryRow("select 'Hello, world!'").Scan(&greeting)
	if err != nil {
		fmt.Fprintf(os.Stderr, "QueryRow failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Greeting: ", greeting)
	fmt.Println("Done!")
}
