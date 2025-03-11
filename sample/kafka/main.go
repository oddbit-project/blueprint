package main

import (
	"context"
	"fmt"
	"github.com/oddbit-project/blueprint/crypt/secure"
	"github.com/oddbit-project/blueprint/log"
	"github.com/oddbit-project/blueprint/provider/kafka"
	tlsProvider "github.com/oddbit-project/blueprint/provider/tls"
	"os"
	"time"
)

func main() {
	ctx := context.Background()
	producerCfg := &kafka.ProducerConfig{
		Brokers:  "localhost:9093",
		Topic:    "test_topic",
		AuthType: "scram256",
		Username: "someUsername",
		DefaultCredentialConfig: secure.DefaultCredentialConfig{
			Password:       "somePassword",
			PasswordEnvVar: "",
			PasswordFile:   "",
		},
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
		DefaultCredentialConfig: secure.DefaultCredentialConfig{
			Password:       "somePassword",
			PasswordEnvVar: "",
			PasswordFile:   "",
		},
		ClientConfig: tlsProvider.ClientConfig{
			TLSEnable: false,
		},
		ConsumerOptions: kafka.ConsumerOptions{},
	}

	logger := log.New("sample-app")
	producer, err := kafka.NewProducer(producerCfg, logger)
	if err != nil {
		logger.Fatal(err, "failed to create producer")
		os.Exit(1)
	}
	defer producer.Disconnect()

	timeout := 250 * time.Second
	consumerCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	consumer, err := kafka.NewConsumer(consumerCfg, logger)
	if err != nil {
		logger.Fatal(err, "failed to create consumer")
		os.Exit(1)
	}
	defer consumer.Disconnect()

	value1 := []byte("the quick brown fox jumps over the lazy dog")
	if err = producer.Write(ctx, value1); err != nil {
		logger.Fatal(err, "failed to produce message")
		os.Exit(1)
	}

	if msg, err := consumer.ReadMessage(consumerCtx); err != nil {
		logger.Fatal(err, "failed to read message")
		os.Exit(1)
	} else {
		fmt.Print(msg)
	}
}
