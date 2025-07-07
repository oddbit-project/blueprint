package main

import (
	"context"
	"fmt"
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
	config.Password = "password" // Uses DefaultCredentialConfig

	// Connect to ClickHouse
	client, err := clickhouse.NewClient(config)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	// Test connection with a ping
	ctx := context.Background()
	if err = client.Ping(ctx); err != nil {
		log.Fatalf("Failed to ping server: %v", err)
	}
	fmt.Println("Connected to ClickHouse server")

	// Check server version
	if client.Version != nil {
		fmt.Printf("Server version: %s\n", client.Version.String())
	}

	// Simple query using the connection directly
	var greeting string
	row := client.Conn.QueryRow(ctx, "SELECT 'Hello, ClickHouse!'")
	if err = row.Scan(&greeting); err != nil {
		fmt.Fprintf(os.Stderr, "QueryRow failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Greeting: %s\n", greeting)

	// Create a repository for a table (if table exists)
	repo := client.NewRepository(ctx, "sample_table")
	fmt.Printf("Repository created for table: %s\n", repo.Name())

	// Example: Create table (would normally be done with proper migrations)
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS sample_table (
		id        Int32,
		name      String,
		value     Float64,
		timestamp DateTime,
		is_active UInt8
	) ENGINE = MergeTree() ORDER BY id
	`
	if err = client.Conn.Exec(ctx, createTableSQL); err != nil {
		log.Printf("Note: Could not create table: %v", err)
		// Continue anyway for demo purposes
	} else {
		fmt.Println("Sample table created")

		// Insert sample data using repository
		sampleRecords := []interface{}{
			&SampleRecord{
				ID:        1,
				Name:      "Sample 1",
				Value:     10.5,
				Timestamp: time.Now(),
				IsActive:  true,
			},
			&SampleRecord{
				ID:        2,
				Name:      "Sample 2",
				Value:     20.75,
				Timestamp: time.Now(),
				IsActive:  false,
			},
		}

		if err = repo.Insert(sampleRecords...); err != nil {
			log.Printf("Failed to insert records: %v", err)
		} else {
			fmt.Println("Inserted sample records")

			// Fetch records - note: ClickHouse uses 1/0 for booleans, not TRUE/FALSE
			var records []SampleRecord
			if err = repo.FetchWhere(map[string]any{"is_active": 1}, &records); err != nil {
				log.Printf("Failed to fetch records: %v", err)
			} else {
				fmt.Printf("Found %d active records:\n", len(records))
				for _, record := range records {
					fmt.Printf("  - ID: %d, Name: %s, Value: %.2f\n",
						record.ID, record.Name, record.Value)
				}
			}

			// Count records - ClickHouse COUNT() returns UInt64
			var count uint64
			row := client.Conn.QueryRow(ctx, "SELECT COUNT(*) FROM sample_table")
			if err := row.Scan(&count); err != nil {
				log.Printf("Failed to count records: %v", err)
			} else {
				fmt.Printf("Total records: %d\n", count)
			}
		}
	}

	fmt.Println("Sample complete!")
}
