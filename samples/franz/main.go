package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/oddbit-project/blueprint/log"
	"github.com/oddbit-project/blueprint/provider/franz"
)

func main() {
	ctx := context.Background()
	logger := log.New("sample-app")

	// Producer configuration
	producerCfg := &franz.ProducerConfig{
		BaseConfig: franz.BaseConfig{
			Brokers:  "localhost:9092",
			AuthType: franz.AuthTypeNone,
		},
		DefaultTopic: "test_topic",
		Acks:         franz.AcksLeader,
	}

	// Consumer configuration
	consumerCfg := &franz.ConsumerConfig{
		BaseConfig: franz.BaseConfig{
			Brokers:  "localhost:9092",
			AuthType: franz.AuthTypeNone,
		},
		Topics:      []string{"test_topic"},
		Group:       "consumer_group_1",
		StartOffset: franz.OffsetStart,
	}

	// Create producer
	producer, err := franz.NewProducer(producerCfg, logger)
	if err != nil {
		logger.Fatal(err, "failed to create producer")
		os.Exit(1)
	}
	defer producer.Close()

	// Create consumer
	consumer, err := franz.NewConsumer(consumerCfg, logger)
	if err != nil {
		logger.Fatal(err, "failed to create consumer")
		os.Exit(1)
	}
	defer consumer.Close()

	// Produce a message
	value := []byte("the quick brown fox jumps over the lazy dog")
	record := franz.NewRecord(value).WithKey([]byte("sample-key"))

	results, err := producer.Produce(ctx, record)
	if err != nil {
		logger.Fatal(err, "failed to produce message")
		os.Exit(1)
	}

	if results[0].Err != nil {
		logger.Fatal(results[0].Err, "failed to produce message")
		os.Exit(1)
	}

	logger.Info("Message produced successfully", log.KV{
		"partition": results[0].Partition,
		"offset":    results[0].Offset,
	})

	// Consume message with timeout
	timeout := 30 * time.Second
	consumerCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	records, err := consumer.PollRecords(consumerCtx, 1)
	if err != nil {
		logger.Fatal(err, "failed to read message")
		os.Exit(1)
	}

	if len(records) > 0 {
		fmt.Printf("Received message: %s\n", string(records[0].Value))
		fmt.Printf("Key: %s\n", string(records[0].Key))
		fmt.Printf("Topic: %s, Partition: %d, Offset: %d\n",
			records[0].Topic, records[0].Partition, records[0].Offset)
	} else {
		fmt.Println("No messages received within timeout")
	}
}
