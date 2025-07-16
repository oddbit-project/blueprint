package main

import (
	"context"
	"fmt"
	"github.com/oddbit-project/blueprint/db/migrations"
	"github.com/oddbit-project/blueprint/provider/clickhouse"
	"log"
	"os"
	"time"
)

// SampleRecord demonstrates a struct for repository operations
type SampleRecord struct {
	ID        int32     `ch:"id"` // Must use int32 to match Int32 in ClickHouse
	Name      string    `ch:"name"`
	Value     float64   `ch:"value"`
	Timestamp time.Time `ch:"timestamp"`
	IsActive  bool      `ch:"is_active"`
}

func main() {
	// Create a configuration
	config := clickhouse.NewClientConfig()
	config.Hosts = []string{"localhost:9000"}
	config.Database = "default"
	config.Username = "default"
	config.Password = "nphwvvfn" // Uses DefaultCredentialConfig

	// Connect to ClickHouse
	client, err := clickhouse.NewClient(config)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	src := migrations.NewMemorySource()
	src.Add("sample1.sql", "create table if not exists sample(id Int32) engine=TinyLog;")
	src.Add("sample2.sql", "insert into sample(id) values(1);")

	// create migration manager
	mgr, err := clickhouse.NewMigrationManager(context.Background(), client)
	if err != nil {
		fmt.Println("Error: ", err)
		os.Exit(1)
	}

	// list existing migrations, should be empty
	list, err := mgr.List(context.Background())
	if err != nil {
		fmt.Println("Error: ", err)
		os.Exit(1)
	}
	err = mgr.Run(context.Background(), src, migrations.DefaultProgressFn)
	if err != nil {
		fmt.Println("Error: ", err)
		os.Exit(1)
	}

	list, err = mgr.List(context.Background())
	for _, m := range list {
		fmt.Println(m.Created, m.Name, m.SHA2)
	}

	fmt.Println("Sample complete!")
}
