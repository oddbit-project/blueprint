package main

import (
	"context"
	"fmt"
	"github.com/oddbit-project/blueprint/provider/kafka"
	tlsProvider "github.com/oddbit-project/blueprint/provider/tls"
	"log"
	"time"
)

func main() {
	ctx := context.Background()
	producerCfg := &kafka.ProducerConfig{
		Brokers:  "localhost:9093",
		Topic:    "test_topic",
		AuthType: "scram256",
		Username: "someUsername",
		Password: "somePassword",
		ClientConfig: tlsProvider.ClientConfig{
			TLSEnable: false,
		},
		ProducerOptions: kafka.ProducerOptions{},
	}
	consumerCfg := &kafka.ConsumerConfig{
		Brokers:  "localhost:9093",
		Topic:    "test_topic",
		Group:    "consumer_group_1",
		AuthType: "scram256",
		Username: "someUsername",
		Password: "somePassword",
		ClientConfig: tlsProvider.ClientConfig{
			TLSEnable: false,
		},
		ConsumerOptions: kafka.ConsumerOptions{},
	}

	producer, err := kafka.NewProducer(ctx, producerCfg)
	if err != nil {
		log.Fatal(err)
	}
	defer producer.Disconnect()

	timeout := 250 * time.Second
	consumerCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	consumer, err := kafka.NewConsumer(consumerCtx, consumerCfg)
	if err != nil {
		log.Fatal(err)
	}
	defer consumer.Disconnect()

	value1 := []byte("the quick brown fox jumps over the lazy dog")
	if err = producer.Write(value1); err != nil {
		log.Fatal(err)
	}

	if msg, err := consumer.ReadMessage(); err != nil {
		log.Fatal(err)
	} else {
		fmt.Print(msg)
	}
}
