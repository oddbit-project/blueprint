# Logging System

Blueprint provides a structured logging system built on top of zerolog that offers consistent logging patterns across different components.

## Features

- **Structured logging** with standardized field names
- **Context-aware logging** for tracking requests across multiple services
- **Distributed tracing** with trace and request IDs
- **Component-specific logging** for HTTP and Kafka operations
- **Log levels** for different severity of messages
- **Performance-oriented** with minimal allocations

## Basic Usage

```go
import "github.com/oddbit-project/blueprint/log"

// Create a logger for a module
logger := log.New("mymodule")

// Log messages at different levels
logger.Info("Application started", log.KV{
    "version": "1.0.0",
})

logger.Debug("Processing item", log.KV{
    "item_id": 123,
})

// Log errors with stack traces
err := someOperation()
if err != nil {
    logger.Error(err, "Failed to process item", log.KV{
        "item_id": 123,
    })
}
```

## Context-Aware Logging

```go
import "github.com/oddbit-project/blueprint/log"

// Create a request context with trace ID
ctx, logger := log.NewRequestContext(context.Background(), "api")

// Add fields to the logger
ctx = log.WithField(ctx, "user_id", userId)

// Log using the context
log.Info(ctx, "Processing user request")

// Pass the context to other functions
processRequest(ctx, request)

// In other functions, retrieve the logger from context
func processRequest(ctx context.Context, req Request) {
    logger := log.FromContext(ctx)
    logger.Info("Processing request details")
    
    // Or use helper functions
    log.Info(ctx, "Alternative way to log")
}
```

## HTTP Request Logging

```go
import (
    "github.com/gin-gonic/gin"
    "github.com/oddbit-project/blueprint/log"
    "github.com/oddbit-project/provider/httpserver"
)

// Use the HTTP logging middleware
router.Use(httpserver.HTTPLogMiddleware("api"))

// Log within handlers
func handler(c *gin.Context) {
    // Get the request logger
    logger := httpserver.GetRequestLogger(c)
    
    // Or use helper functions
    httpserver.RequestInfo(c, "Processing API request", log.KV{
        "request_data": someData,
    })
    
    // Error handling
    if err := someOperation(); err != nil {
        httpserver.RequestError(c, err, "Operation failed")
        return
    }
}
```

## Kafka Message Logging

Kafka consumer example:
```go
import (
	"context"
    "github.com/oddbit-project/blueprint/log"
    "github.com/oddbit-project/blueprint/provider/kafka"
)

ctx := context.Background()

// Producer logging
producer, _ := kafka.NewProducer(cfg, nil)
err := producer.WriteJson(ctx, data)

// Consumer with logging
consumer, _ := kafka.NewConsumer(cfg, nil)
consumer.Subscribe(ctx, func(ctx context.Context, msg kafka.Message) error {
    // use consumer logger
	consumer.Logger.Info("processing message...")
	
	// log kafka message
	kafka.LogMessageReceived(consumer.Logger, msg, consumer.GetConfig().Group)    
    
	// Add your processing logic
    // ...
    
    return nil
})

```

## Configuration

```go
import "github.com/oddbit-project/blueprint/log"

// Create a configuration
cfg := log.NewDefaultConfig()
cfg.Level = "debug"           // log level: debug, info, warn, error
cfg.Format = "console"        // output format: console or json
cfg.IncludeTimestamp = true   // include timestamp in logs
cfg.IncludeCaller = true      // include caller information
cfg.IncludeHostname = true    // include hostname

// Configure the global logger
err := log.Configure(cfg)
if err != nil {
    panic(err)
}
```

## Best Practices

1. **Include relevant fields**: Add meaningful fields to help with debugging and analysis
2. **Be consistent with log levels**:
   - DEBUG: Detailed information for debugging
   - INFO: General operational information
   - WARN: Situations that might cause issues
   - ERROR: Errors that prevent normal operation
   - FATAL: Critical errors that require shutdown
3. **Sanitize sensitive data**: Don't log passwords, tokens, or other sensitive information
4. **Use structured logging**: Avoid string concatenation or formatting in log messages