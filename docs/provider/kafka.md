# blueprint.provider.kafka

Blueprint Kafka client with enhanced security and performance features.

The client supports the following authentication modes:
- `none` - No authentication
- `plain` - SASL PLAIN authentication
- `scram256` - SCRAM-SHA-256 authentication
- `scram512` - SCRAM-SHA-512 authentication

## Features

- Secure credential handling with in-memory encryption
- TLS support for secure connections
- Configurable producer options for performance tuning
- JSON message support
- Context-aware logging with tracing
- Batch operations

## Using the Kafka Producer

```go
package main

import (
    "context"
    "fmt"
    "github.com/oddbit-project/blueprint/log"
    "github.com/oddbit-project/blueprint/provider/kafka"
	tlsProvider "github.com/oddbit-project/blueprint/provider/tls"
    "time"
)

func main() {
	ctx := context.Background()
	logger := log.NewLogger("kafka-example")
    
    // Configure the producer
	producerCfg := &kafka.ProducerConfig{
        Brokers:  "localhost:9093",
        Topic:    "test_topic",
        AuthType: "scram256",
        Username: "someUsername",
        DefaultCredentialConfig: secure.DefaultCredentialConfig{
            Password: "somePassword",
            // Or use environment variables or files
            // PasswordEnvVar: "KAFKA_PASSWORD",
            // PasswordFile: "/path/to/password.txt",
        },
        ClientConfig: tlsProvider.ClientConfig{
            TLSEnable: true,
            TLSCA: "/path/to/ca.crt",
        },
        ProducerOptions: kafka.ProducerOptions{
            BatchSize: 100,
            BatchTimeout: 1000, // ms
            RequiredAcks: "one",
            Async: true,
        },
    }

    // Create the producer
	producer, err := kafka.NewProducer(producerCfg, logger)
	if err != nil {
        logger.Fatal(err, "Failed to create Kafka producer", nil)
    }
	defer producer.Disconnect()

    // Write a simple message
	err = producer.Write(ctx, []byte("Hello, Kafka!"))
	if err != nil {
        logger.Error(err, "Failed to write message", nil)
    }
    
    // Write with a key
	err = producer.Write(ctx, []byte("Message with key"), []byte("user-123"))
	if err != nil {
        logger.Error(err, "Failed to write message with key", nil)
    }
    
    // Write a JSON message
	type User struct {
        ID   string `json:"id"`
        Name string `json:"name"`
    }
    
	user := User{
        ID:   "123",
        Name: "John Doe",
    }
    
	err = producer.WriteJson(ctx, user)
	if err != nil {
        logger.Error(err, "Failed to write JSON message", nil)
    }
    
    // Write multiple messages
	messages := [][]byte{
        []byte("Message 1"),
        []byte("Message 2"),
        []byte("Message 3"),
    }
    
	err = producer.WriteMulti(ctx, messages...)
	if err != nil {
        logger.Error(err, "Failed to write multiple messages", nil)
    }
}
```

## Using the Kafka Consumer

```go
package main

import (
    "context"
    "fmt"
    "github.com/oddbit-project/blueprint/log"
    "github.com/oddbit-project/blueprint/provider/kafka"
	tlsProvider "github.com/oddbit-project/blueprint/provider/tls"
    "time"
)

func main() {
	logger := log.NewLogger("kafka-consumer")
    
    // Configure the consumer
	consumerCfg := &kafka.ConsumerConfig{
        Brokers:  "localhost:9093",
        Topic:    "test_topic",
        Group:    "consumer_group_1",
        AuthType: "scram256",
        Username: "someUsername",
        DefaultCredentialConfig: secure.DefaultCredentialConfig{
            Password: "somePassword",
            // Or use environment variables or files
            // PasswordEnvVar: "KAFKA_PASSWORD",
            // PasswordFile: "/path/to/password.txt",
        },
        ClientConfig: tlsProvider.ClientConfig{
            TLSEnable: true,
            TLSCA: "/path/to/ca.crt",
        },
        ConsumerOptions: kafka.ConsumerOptions{
            MinBytes: 10,
            MaxBytes: 10485760, // 10MB
            MaxWait: 1000,      // ms
        },
    }

    // Create a context with timeout
	timeout := 30 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

    // Create the consumer
	consumer, err := kafka.NewConsumer(consumerCfg, logger)
	if err != nil {
        logger.Fatal(err, "Failed to create Kafka consumer", nil)
    }
	defer consumer.Disconnect()

    // Read a single message
	msg, err := consumer.ReadMessage(ctx)
	if err != nil {
        logger.Error(err, "Failed to read message", nil)
    } else {
        logger.Info("Received message", log.KV{
            "value": string(msg.Value),
            "key": string(msg.Key),
            "topic": msg.Topic,
            "partition": msg.Partition,
            "offset": msg.Offset,
        })
    }
    
    // Process messages in a loop
	for {
        select {
        case <-ctx.Done():
            logger.Info("Context done, stopping consumer", nil)
            return
        default:
            msg, err := consumer.ReadMessage(ctx)
            if err != nil {
                logger.Error(err, "Error reading message", nil)
                continue
            }
            
            // Process the message
            logger.Info("Processing message", log.KV{
                "value_len": len(msg.Value),
            })
            
            // Parse JSON messages
            if consumer.IsJsonMessage(msg) {
                var data map[string]interface{}
                if err := consumer.DecodeJson(msg, &data); err != nil {
                    logger.Error(err, "Failed to decode JSON message", nil)
                } else {
                    logger.Info("Received JSON message", data)
                }
            }
        }
    }
}
```

## Performance Tuning

The Kafka producer can be tuned for performance using the `ProducerOptions` struct:

```go
type ProducerOptions struct {
	MaxAttempts     uint   // Maximum number of retries
	WriteBackoffMin uint   // Minimum backoff in milliseconds
	WriteBackoffMax uint   // Maximum backoff in milliseconds
	BatchSize       uint   // Number of messages in a batch
	BatchBytes      uint64 // Maximum batch size in bytes
	BatchTimeout    uint   // Time to wait for batch completion in milliseconds
	ReadTimeout     uint   // Read timeout in milliseconds
	WriteTimeout    uint   // Write timeout in milliseconds
	RequiredAcks    string // Acknowledgment level: "none", "one", "all"
	Async           bool   // Async mode (non-blocking writes)
}
```

Similarly, the consumer can be tuned using the `ConsumerOptions` struct:

```go
type ConsumerOptions struct {
	MinBytes        uint   // Minimum number of bytes to fetch
	MaxBytes        uint   // Maximum number of bytes to fetch
	MaxWait         uint   // Maximum time to wait for data in milliseconds
	ReadLagInterval uint   // Interval to update lag information in milliseconds
	HeartbeatInterval uint // Heartbeat interval in milliseconds
	CommitInterval uint    // Auto-commit interval in milliseconds
	StartOffset     string // Where to start reading: "newest", "oldest"
}
```

## Security Best Practices

1. Always enable TLS in production environments
2. Use SCRAM authentication instead of PLAIN when possible
3. Store passwords in environment variables or secure files
4. Rotate credentials regularly
5. Limit topic access with proper ACLs