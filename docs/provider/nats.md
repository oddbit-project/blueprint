# blueprint.provider.nats

Blueprint NATS client implementation for message publishing and consumption.

## Overview

The NATS client provides a simple interface for connecting to NATS servers, publishing messages, and consuming messages using subjects and queues. It supports:

- Multiple authentication methods (none, basic, token, NKey, JWT)
- TLS for secure connections
- Publish/Subscribe patterns
- Request/Reply patterns
- Queue groups for distributing message processing
- JSON serialization for structured messages

## Configuration

### Producer Configuration

```go
type ProducerConfig struct {
	URL      string // NATS server URL (e.g., "nats://localhost:4222")
	Subject  string // Default subject to publish to
	AuthType string // Authentication type: "none", "basic", "token", "nkey", "jwt"
	Username string // Username for basic auth
	Password string // Password for basic auth
	NKeyPath string // Path to NKey seed file
	JwtPath  string // Path to JWT file
	Token    string // Auth token

	// Connection settings
	PingInterval uint // PingInterval in seconds, defaults to 2 minutes
	MaxPingsOut  uint // MaxPingsOut value, defaults to 2
	Timeout      uint // Connection timeout in milliseconds, defaults to 2000
	DrainTimeout uint // Drain timeout in milliseconds, defaults to 30000
	// TLS Configuration
	TLSEnabled            bool   // Enable TLS
	TLSInsecureSkipVerify bool   // Skip certificate verification
	TLSCertFile           string // Client certificate file path
	TLSKeyFile            string // Client key file path
	TLSCaFile             string // CA certificate file path
}
```

### Consumer Configuration

```go
type ConsumerConfig struct {
	URL        string // NATS server URL (e.g., "nats://localhost:4222")
	Subject    string // Subject pattern to subscribe to
	AuthType   string // Authentication type: "none", "basic", "token", "nkey", "jwt"
	Username   string // Username for basic auth
	Password   string // Password for basic auth
	NKeyPath   string // Path to NKey seed file
	JwtPath    string // Path to JWT file
	Token      string // Auth token
	QueueGroup string // Queue group for distributing messages among subscribers

	// Connection settings
	PingInterval uint // PingInterval in seconds, defaults to 2 minutes
	MaxPingsOut  uint // MaxPingsOut value, defaults to 2
	Timeout      uint // Connection timeout in milliseconds, defaults to 2000
	DrainTimeout uint // Drain timeout in milliseconds, defaults to 30000
	// TLS Configuration
	TLSEnabled            bool   // Enable TLS
	TLSInsecureSkipVerify bool   // Skip certificate verification
	TLSCertFile           string // Client certificate file path
	TLSKeyFile            string // Client key file path
	TLSCaFile             string // CA certificate file path
}
```

## Producer Usage

### Creating a Producer

```go
package main

import (
    "github.com/oddbit-project/blueprint/log"
    "github.com/oddbit-project/blueprint/provider/nats"
)

func main() {
	// Create producer configuration
	config := &nats.ProducerConfig{
		URL:      "nats://localhost:4222",
		Subject:  "my.subject",
		AuthType: nats.AuthTypeNone,
	}

	// Create logger
	logger := log.NewLogger()

	// Create producer
	producer, err := nats.NewProducer(config, logger)
	if err != nil {
		logger.Fatal(err, "Failed to create NATS producer", nil)
	}
	defer producer.Disconnect()

	// Publish message
	err = producer.Publish([]byte("Hello, NATS!"))
	if err != nil {
		logger.Error(err, "Failed to publish message", nil)
	}
    
    // Publish JSON message
	type Person struct {
        Name string `json:"name"`
        Age  int    `json:"age"`
    }
    
	person := Person{
        Name: "John Doe",
        Age:  30,
    }
    
	err = producer.PublishJSON(person)
	if err != nil {
        logger.Error(err, "Failed to publish JSON message", nil)
    }
}
```

### Request-Reply Pattern

```go
// Send a request and wait for a response
msg, err := producer.Request("request.subject", []byte("Request data"), time.Second*5)
if err != nil {
	logger.Error(err, "Request failed", nil)
	return
}

// Process response
logger.Info("Received response", log.KV{
    "data": string(msg.Data),
})
```

## Consumer Usage

### Creating a Consumer

```go
package main

import (
    "context"
    "github.com/oddbit-project/blueprint/log"
    "github.com/oddbit-project/blueprint/provider/nats"
    "time"
)

func main() {
    // Create consumer configuration
	config := &nats.ConsumerConfig{
        URL:      "nats://localhost:4222",
        Subject:  "my.subject",
        AuthType: nats.AuthTypeNone,
        ConsumerOptions: nats.ConsumerOptions{
            QueueGroup: "my-service",
        },
    }
    
    // Create logger
	logger := log.NewLogger()
    
    // Create consumer
	consumer, err := nats.NewConsumer(config, logger)
	if err != nil {
		logger.Fatal(err, "Failed to create NATS consumer", nil)
	}
	defer consumer.Disconnect()
    
    // Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
    
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
    
    // Keep running for 1 minute
	time.Sleep(time.Minute)
}
```

### Synchronous Message Consumption

```go
// Subscribe synchronously
sub, err := consumer.SubscribeSync()
if err != nil {
	logger.Error(err, "Failed to subscribe", nil)
	return
}

// Get next message with timeout
msg, err := consumer.NextMsg(sub, time.Second*5)
if err != nil {
	if err == nats.ErrTimeout {
        logger.Info("No message received within timeout", nil)
    } else {
        logger.Error(err, "Failed to get next message", nil)
    }
	return
}

// Process message
logger.Info("Received message", log.KV{
    "subject": msg.Subject,
    "data":    string(msg.Data),
})

// Unsubscribe when done
consumer.Unsubscribe(sub)
```

## Authentication Methods

### Basic Authentication

```go
config := &nats.ProducerConfig{
	URL:      "nats://localhost:4222",
	Subject:  "my.subject",
	AuthType: nats.AuthTypeBasic,
	Username: "user",
	Password: "password",
}
```

### Token Authentication

```go
config := &nats.ProducerConfig{
	URL:      "nats://localhost:4222",
	Subject:  "my.subject",
	AuthType: nats.AuthTypeToken,
	Token:    "my-auth-token",
}
```

### NKey Authentication

```go
config := &nats.ProducerConfig{
	URL:      "nats://localhost:4222",
	Subject:  "my.subject",
	AuthType: nats.AuthTypeNKey,
	NKeyPath: "/path/to/nkey.seed",
}
```

### JWT Authentication

```go
config := &nats.ProducerConfig{
	URL:      "nats://localhost:4222",
	Subject:  "my.subject",
	AuthType: nats.AuthTypeJWT,
	JwtPath:  "/path/to/user.jwt",
}
```

## TLS Configuration

```go
config := &nats.ProducerConfig{
	URL:      "nats://localhost:4222",
	Subject:  "my.subject",
	AuthType: nats.AuthTypeNone,
	// TLS Configuration
	TLSEnabled:            true,
	TLSInsecureSkipVerify: false,
	TLSCertFile:           "/path/to/client.crt",
	TLSKeyFile:            "/path/to/client.key",
	TLSCaFile:             "/path/to/ca.crt",
}
```