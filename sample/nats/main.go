package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/oddbit-project/blueprint/log"
	"github.com/oddbit-project/blueprint/provider/nats"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

func main() {
	// Command line flags for mode selection
	mode := flag.String("mode", "both", "Run mode: producer, consumer, or both")
	url := flag.String("url", "nats://localhost:4222", "NATS server URL")
	subject := flag.String("subject", "blueprint.sample", "Subject to use")
	queue := flag.String("queue", "", "Queue group (optional)")
	flag.Parse()

	fmt.Printf("NATS Sample: Running in %s mode\n", *mode)
	fmt.Printf("NATS Server: %s\n", *url)
	fmt.Printf("Subject: %s\n", *subject)
	if *queue != "" {
		fmt.Printf("Queue Group: %s\n", *queue)
	}

	// Create a context that will be canceled on SIGINT or SIGTERM
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\nReceived shutdown signal")
		cancel()
	}()

	// Create a wait group to coordinate goroutines
	var wg sync.WaitGroup

	// Start producer if requested
	if *mode == "producer" || *mode == "both" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			runProducer(ctx, *url, *subject)
		}()
	}

	// Start consumer if requested
	if *mode == "consumer" || *mode == "both" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			runConsumer(ctx, *url, *subject, *queue)
		}()
	}

	// Wait for all goroutines to complete
	wg.Wait()
	fmt.Println("Sample completed")
}

// runProducer demonstrates NATS producer functionality
func runProducer(ctx context.Context, url, subject string) {
	// Create logger
	logger := log.New("NATS-SAMPLE-PRODUCER")

	// Create producer configuration
	config := &nats.ProducerConfig{
		URL:      url,
		Subject:  subject,
		AuthType: nats.AuthTypeNone,
	}

	// Create producer
	producer, err := nats.NewProducer(config, logger)
	if err != nil {
		logger.Fatal(err, "Failed to create NATS producer", nil)
		return
	}
	defer producer.Disconnect()

	logger.Info("Producer connected and ready", log.KV{
		"url":     url,
		"subject": subject,
	})

	// Send a message every second until context is canceled
	messageCount := 0
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			messageCount++
			message := fmt.Sprintf("Blueprint NATS sample message #%d at %s", 
				messageCount, time.Now().Format(time.RFC3339))
			
			if err := producer.Publish([]byte(message)); err != nil {
				logger.Error(err, "Failed to publish message", nil)
				continue
			}
			
			logger.Info("Published message", log.KV{
				"count": messageCount,
			})

		case <-ctx.Done():
			logger.Info("Producer shutting down", nil)
			return
		}
	}
}

// runConsumer demonstrates NATS consumer functionality
func runConsumer(ctx context.Context, url, subject, queue string) {
	// Create logger
	logger := log.New("NATS-SAMPLE-CONSUMER")

	// Create consumer configuration
	config := &nats.ConsumerConfig{
		URL:      url,
		Subject:  subject,
		AuthType: nats.AuthTypeNone,
		ConsumerOptions: nats.ConsumerOptions{
			QueueGroup: queue,
		},
	}

	// Create consumer
	consumer, err := nats.NewConsumer(config, logger)
	if err != nil {
		logger.Fatal(err, "Failed to create NATS consumer", nil)
		return
	}
	defer consumer.Disconnect()

	logger.Info("Consumer connected and ready", log.KV{
		"url":     url,
		"subject": subject,
		"queue":   queue,
	})

	// Define message handler
	handler := func(ctx context.Context, msg nats.Message) error {
		logger.Info("Received message", log.KV{
			"subject": msg.Subject,
			"data":    string(msg.Data),
		})
		return nil
	}

	// Subscribe to subject
	err = consumer.Subscribe(ctx, handler)
	if err != nil {
		logger.Error(err, "Failed to subscribe", nil)
		return
	}

	// Wait for context cancellation
	<-ctx.Done()
	logger.Info("Consumer shutting down", nil)
}