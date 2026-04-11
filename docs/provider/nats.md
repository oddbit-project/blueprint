# blueprint.provider.nats

Blueprint NATS client implementation for message publishing and consumption.

## Overview

The NATS client provides a simple interface for connecting to NATS servers, publishing messages, and consuming messages using subjects and queues. It supports:

- Multiple authentication methods (none, basic, token)
- TLS for secure connections
- Publish/Subscribe patterns
- Request/Reply patterns
- Queue groups for distributing message processing
- JSON serialization for structured messages
- JetStream persistent streams via dedicated `JSProducer` / `JSConsumer` types
  (see [JetStream](#jetstream))

## Configuration

### Producer Configuration

The effective shape below is flattened for readability. In the actual Go
type, `Password`/`Token` come from the embedded
`secure.DefaultCredentialConfig`, the TLS fields come from the embedded
`tls.ClientConfig`, and the connection-tuning fields come from the embedded
`ProducerOptions` struct.

```go
type ProducerConfig struct {
	URL      string // NATS server URL (e.g., "nats://localhost:4222")
	Subject  string // Default subject to publish to
	AuthType string // Authentication type: "none", "basic", "token"
	Username string // Username for basic auth
	Password string // Password for basic auth (embedded secure.DefaultCredentialConfig)
	Token    string // Auth token (embedded secure.DefaultCredentialConfig)

	// Connection settings (embedded ProducerOptions)
	PingInterval uint // PingInterval in seconds, defaults to 2 minutes
	MaxPingsOut  uint // MaxPingsOut value, defaults to 2
	Timeout      uint // Connection timeout in milliseconds, defaults to 2000
	DrainTimeout uint // Drain timeout in milliseconds, defaults to 30000

	// TLS Configuration (embedded tls.ClientConfig)
	TLSEnabled            bool   // Enable TLS
	TLSInsecureSkipVerify bool   // Skip certificate verification
	TLSCertFile           string // Client certificate file path
	TLSKeyFile            string // Client key file path
	TLSCaFile             string // CA certificate file path
}
```

### Consumer Configuration

As with `ProducerConfig`, the shape below is flattened for readability. In
the actual Go type, `Password`/`Token` come from the embedded
`secure.DefaultCredentialConfig`, the TLS fields come from the embedded
`tls.ClientConfig`, and `QueueGroup` together with the connection-tuning
fields come from the embedded `ConsumerOptions` struct. To set a queue group
you assign `cfg.ConsumerOptions = nats.ConsumerOptions{QueueGroup: "..."}`
(see the example below).

```go
type ConsumerConfig struct {
	URL        string // NATS server URL (e.g., "nats://localhost:4222")
	Subject    string // Subject pattern to subscribe to
	AuthType   string // Authentication type: "none", "basic", "token"
	Username   string // Username for basic auth
	Password   string // Password for basic auth (embedded secure.DefaultCredentialConfig)
	Token      string // Auth token (embedded secure.DefaultCredentialConfig)
	QueueGroup string // Queue group (embedded ConsumerOptions.QueueGroup)

	// Connection settings (embedded ConsumerOptions)
	PingInterval uint // PingInterval in seconds, defaults to 2 minutes
	MaxPingsOut  uint // MaxPingsOut value, defaults to 2
	Timeout      uint // Connection timeout in milliseconds, defaults to 2000
	DrainTimeout uint // Drain timeout in milliseconds, defaults to 30000

	// TLS Configuration (embedded tls.ClientConfig)
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
	logger := log.New("nats-producer")

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
	logger := log.New("nats-consumer")

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

`NextMsg` returns the upstream `nats.ErrTimeout` sentinel when the timeout
expires without a message arriving. Because the Blueprint provider package
is typically imported under the alias `nats`, the upstream package has to
be imported under a different alias to reach its error constants:

```go
import (
    natsio "github.com/nats-io/nats.go"
    "github.com/oddbit-project/blueprint/provider/nats"
)
```

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
	if errors.Is(err, natsio.ErrTimeout) {
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

## JetStream

JetStream is NATS' built-in persistent streaming layer. The Blueprint NATS
provider exposes JetStream through two dedicated types that are independent
from the core `Producer`/`Consumer`: `JSProducer` publishes to a stream and
waits for server acks, `JSConsumer` reads from a stream with explicit
acknowledgements and automatic redelivery on failure.

JetStream requires `nats-server` to be started with the `-js` flag (or
`jetstream: enabled` in the server configuration).

### Configuration

The JetStream types share a common connection config (`JSConnectionConfig`)
with the same URL/auth/TLS fields used by the core types.

```go
type JSConnectionConfig struct {
    URL          string // NATS server URL
    AuthType     string // "none" | "basic" | "token"
    Username     string // for basic auth
    // embedded secure.DefaultCredentialConfig: Password / Token
    ClientName   string // defaults to "natsJSProducer" / "natsJSConsumer"
    // embedded tls.ClientConfig: TLSEnabled, TLSCertFile, etc.
    PingInterval uint   // seconds
    MaxPingsOut  uint
    Timeout      uint   // milliseconds
}
```

#### Producer configuration

```go
type JSProducerConfig struct {
    JSConnectionConfig
    Subject          string       // default publish subject
    Stream           StreamConfig // stream to target
    AutoCreateStream bool         // create-or-update stream on startup
}

type StreamConfig struct {
    Name        string
    Description string
    Subjects    []string       // e.g. ["orders.>"]
    Retention   string         // "limits" | "interest" | "workqueue"
    Storage     string         // "file" | "memory"
    MaxAge      time.Duration
    MaxMsgs     int64
    MaxBytes    int64
    Replicas    int
    Duplicates  time.Duration

    // Native escape hatch: when set, all other StreamConfig fields are
    // ignored and the supplied jetstream.StreamConfig is used verbatim.
    Native *jetstream.StreamConfig
}
```

`AutoCreateStream` defaults to `false` for production safety — in that mode
the producer looks the stream up at startup and fails if it does not exist.
When `AutoCreateStream` is `true`, the provider will `CreateOrUpdateStream`
on startup, which is convenient for development and tests.

#### Consumer configuration

```go
type JSConsumerConfig struct {
    JSConnectionConfig
    StreamName    string // required; stream must already exist
    Durable       string // durable name; leave empty for ephemeral
    ConsumerName  string // optional explicit name
    FilterSubject string // optional subject filter under the stream

    AckPolicy     string        // "explicit" (default) | "all" | "none"
    AckWait       time.Duration
    MaxDeliver    int
    MaxAckPending int
    DeliverPolicy string        // "all" (default) | "last" | "new"

    // Native escape hatch: when set, all other policy fields are ignored.
    Native *jetstream.ConsumerConfig
}
```

Both config types expose a `Validate()` method that is called automatically
by the constructors before any network round-trips. Invalid policy strings
return `ErrInvalidAckPolicy` / `ErrInvalidDeliverPolicy` / `ErrInvalidRetention`
/ `ErrInvalidStorage`; missing required fields return `ErrMissingJSURL`,
`ErrMissingStreamName`, or `ErrMissingProducerTopic`.

### Producer usage

```go
package main

import (
    "context"
    "time"

    "github.com/oddbit-project/blueprint/log"
    "github.com/oddbit-project/blueprint/provider/nats"
)

func main() {
    cfg := &nats.JSProducerConfig{
        JSConnectionConfig: nats.JSConnectionConfig{
            URL:      "nats://localhost:4222",
            AuthType: nats.AuthTypeNone,
        },
        Subject: "orders.created",
        Stream: nats.StreamConfig{
            Name:     "ORDERS",
            Subjects: []string{"orders.>"},
            Storage:  "file",
            MaxAge:   24 * time.Hour,
            Replicas: 1,
        },
        AutoCreateStream: true,
    }

    logger := log.New("orders-producer")

    producer, err := nats.NewJSProducer(cfg, logger)
    if err != nil {
        logger.Fatal(err, "Failed to create JetStream producer", nil)
    }
    defer producer.Disconnect()

    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    // Publish and wait for server ack
    ack, err := producer.Publish(ctx, []byte(`{"id":"o-1"}`))
    if err != nil {
        logger.Error(err, "Publish failed", nil)
        return
    }
    logger.Info("Published", log.KV{
        "stream":   ack.Stream,
        "sequence": ack.Sequence,
    })

    // Publish JSON helper
    _, _ = producer.PublishJSON(ctx, map[string]any{"id": "o-2", "total": 99.0})

    // Publish to an explicit subject under the stream
    _, _ = producer.PublishMsg(ctx, "orders.updated", []byte(`{"id":"o-1"}`))

    // Async publish: returns a future whose Ok()/Err() channels signal the
    // eventual ack without blocking the caller.
    fut, err := producer.PublishAsync(ctx, "orders.created", []byte(`{"id":"o-3"}`))
    if err == nil {
        select {
        case <-fut.Ok():
        case err := <-fut.Err():
            logger.Error(err, "Async publish failed", nil)
        case <-time.After(2 * time.Second):
            logger.Error(nil, "Async publish timed out", nil)
        }
    }
}
```

### Consumer usage

`JSConsumer.Consume` starts continuous delivery; the handler is invoked once
per message. If the handler returns `nil` the message is automatically
acknowledged; if it returns a non-nil error the message is `Nak`'d and
redelivered (up to `MaxDeliver` times). Only one concurrent `Consume` session
is allowed per consumer — calling `Consume` twice returns `ErrAlreadyConsuming`.

```go
package main

import (
    "context"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/oddbit-project/blueprint/log"
    "github.com/oddbit-project/blueprint/provider/nats"
)

func main() {
    cfg := &nats.JSConsumerConfig{
        JSConnectionConfig: nats.JSConnectionConfig{
            URL:      "nats://localhost:4222",
            AuthType: nats.AuthTypeNone,
        },
        StreamName:    "ORDERS",
        Durable:       "orders-worker",
        FilterSubject: "orders.created",
        AckPolicy:     "explicit",
        AckWait:       30 * time.Second,
        MaxDeliver:    5,
        MaxAckPending: 256,
        DeliverPolicy: "all",
    }

    logger := log.New("orders-worker")

    consumer, err := nats.NewJSConsumer(cfg, logger)
    if err != nil {
        logger.Fatal(err, "Failed to create JetStream consumer", nil)
    }
    defer consumer.Disconnect()

    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    // Consume returns immediately; delivery runs in the background. The
    // handler receives the same ctx that was passed to Consume, so it can
    // honour cancellation from the outer scope.
    err = consumer.Consume(ctx, func(hctx context.Context, msg nats.JSMessage) error {
        logger.Info("Received", log.KV{
            "subject": msg.Subject(),
            "size":    len(msg.Data()),
        })
        // Returning nil auto-Acks; returning a non-nil error auto-Naks.
        return processOrder(hctx, msg.Data())
    })
    if err != nil {
        logger.Fatal(err, "Consume failed", nil)
    }

    // Block on SIGINT/SIGTERM. When a signal arrives cancel() stops the
    // consume session via the watcher goroutine, and the deferred
    // Disconnect() drains the connection.
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
    <-sigCh
}

func processOrder(ctx context.Context, _ []byte) error { return ctx.Err() }
```

#### Pull-style one-off fetch

For batch or polling workloads, use `Fetch` instead of `Consume`:

```go
msgs, err := consumer.Fetch(50, 2*time.Second)
if err != nil && len(msgs) == 0 {
    logger.Error(err, "Fetch failed", nil)
    return
}
for _, m := range msgs {
    if err := processOrder(m.Data()); err != nil {
        _ = m.Nak()
        continue
    }
    _ = m.Ack()
}
```

Note that `Fetch` may return a non-nil slice **and** a non-nil error
simultaneously when the batch was interrupted mid-delivery. Handle both.

### Manual acknowledgement control

`JSMessage` exposes the full ack API when automatic ack-on-return is not
flexible enough. For example, signalling "still working" on long-running
handlers:

```go
err := consumer.Consume(ctx, func(ctx context.Context, msg nats.JSMessage) error {
    ticker := time.NewTicker(10 * time.Second)
    defer ticker.Stop()

    done := make(chan error, 1)
    go func() { done <- longRunningWork(msg.Data()) }()

    for {
        select {
        case err := <-done:
            return err // auto-Ack on nil, auto-Nak on error
        case <-ticker.C:
            _ = msg.InProgress() // extend ack-wait
        case <-ctx.Done():
            _ = msg.Nak()
            return ctx.Err()
        }
    }
})
```

Other available methods: `msg.Ack()`, `msg.Nak()`, `msg.Term()` (permanent
reject — no redelivery), `msg.Metadata()` (sequence, delivery count,
timestamp), and `msg.Raw()` (the underlying `jetstream.Msg` for advanced
use cases).

### Managing streams outside the producer

The recommended pattern for production deployments is:

1. Provision streams at deploy time with a bootstrap `JSProducer` configured
   with `AutoCreateStream: true` — for example in a migration job or a
   one-shot CLI tool.
2. Run application producers with `AutoCreateStream: false` and a
   name-only `StreamConfig{Name: "..."}`. The producer will look the
   stream up at startup and fail if it does not exist, which catches
   misconfigured deployments early instead of silently recreating a stream
   with different parameters.

`EnsureStream(ctx, js, cfg)` is exposed as a helper for code that holds its
own `jetstream.JetStream` handle (for example, a deploy tool or an
integration test harness):

```go
streamCfg := nats.StreamConfig{
    Name:     "ORDERS",
    Subjects: []string{"orders.>"},
    Storage:  "file",
    Replicas: 1,
}
stream, err := nats.EnsureStream(ctx, js, streamCfg)
if err != nil {
    return err
}
_ = stream // jetstream.Stream handle for further inspection
```

If `StreamConfig.Native` is set, the other fields on `StreamConfig` are
ignored and the supplied `jetstream.StreamConfig` is passed through verbatim.
This is the escape hatch for fields not exposed on the friendly wrapper (for
example `Placement`, `Sources`, `SubjectTransform`).

### Error constants

JetStream-specific error sentinels (all declared in `provider/nats/js_common.go`):

| Constant | Meaning |
|---|---|
| `ErrMissingJSURL` | `JSConnectionConfig.URL` was empty |
| `ErrMissingStreamName` | `JSConsumerConfig.StreamName` / `StreamConfig.Name` was empty when required |
| `ErrJSNoConsumer` | Consumer handle was not initialized |
| `ErrAlreadyConsuming` | `Consume()` called while a session is already active |
| `ErrInvalidAckPolicy` | `AckPolicy` was not `""`, `"explicit"`, `"all"`, or `"none"` |
| `ErrInvalidDeliverPolicy` | `DeliverPolicy` was not `""`, `"all"`, `"last"`, or `"new"` |
| `ErrInvalidRetention` | `StreamConfig.Retention` was not `""`, `"limits"`, `"interest"`, or `"workqueue"` |
| `ErrInvalidStorage` | `StreamConfig.Storage` was not `""`, `"file"`, or `"memory"` |

The existing core-NATS sentinels (`ErrMissingProducerTopic`, `ErrInvalidAuthType`,
`ErrNilConfig`, `ErrConsumerClosed`, `ErrProducerClosed`) are also returned by
the JetStream paths where appropriate.