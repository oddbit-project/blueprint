# Logging System

Blueprint provides a structured logging system built on top of zerolog that offers consistent logging patterns across different components.

## Features

- **Structured logging** with standardized field names
- **Context-aware logging** for tracking requests across multiple services
- **Distributed tracing** with trace and request IDs
- **Component-specific logging** for HTTP, Kafka, and database operations
- **Log levels** for different severity of messages
- **Performance-oriented** with minimal allocations

## Basic Usage

```go
import "github.com/oddbit-project/blueprint/log"

// Create a logger for a module
logger := log.New("mymodule")

// Log messages at different levels
logger.Info("Application started", map[string]interface{}{
    "version": "1.0.0",
})

logger.Debug("Processing item", map[string]interface{}{
    "item_id": 123,
})

// Log errors with stack traces
err := someOperation()
if err != nil {
    logger.Error(err, "Failed to process item", map[string]interface{}{
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
)

// Use the HTTP logging middleware
router.Use(log.HTTPLogMiddleware("api"))

// Log within handlers
func handler(c *gin.Context) {
    // Get the request logger
    logger := log.GetRequestLogger(c)
    
    // Or use helper functions
    log.RequestInfo(c, "Processing API request", map[string]interface{}{
        "request_data": someData,
    })
    
    // Error handling
    if err := someOperation(); err != nil {
        log.RequestError(c, err, "Operation failed")
        return
    }
}
```

## Kafka Message Logging

```go
import (
    "github.com/oddbit-project/blueprint/log"
    "github.com/oddbit-project/blueprint/provider/kafka"
)

// Producer logging
producer, _ := kafka.NewProducer(cfg)
err := producer.WriteJson(data)

// Consumer with logging
consumer, _ := kafka.NewConsumer(cfg)
consumer.Subscribe(func(ctx context.Context, msg kafka.Message) error {
    // Message is automatically logged by the updated consumer
    
    // Add your processing logic
    // ...
    
    return nil
})

// Manual Kafka logging
log.LogKafkaMessageReceived(ctx, msg, "mygroup")
log.LogKafkaMessageSent(ctx, msg)
```

## Database Query Logging

```go
import (
    "github.com/oddbit-project/blueprint/log"
    "database/sql"
)

// Log database operations
startTime := time.Now()
rows, err := db.QueryContext(ctx, query, args...)
duration := time.Since(startTime)

log.LogDBQuery(ctx, query, args, duration, err)

// Log database results
result, err := db.ExecContext(ctx, query, args...)
log.LogDBResult(ctx, result, err, "INSERT")

// Transaction logging
tx, err := db.BeginTx(ctx, nil)
log.LogDBTransaction(ctx, "begin", err)

// ... perform operations ...

err = tx.Commit()
log.LogDBTransaction(ctx, "commit", err)
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

1. **Use context propagation**: Always pass the context with logger between functions and services
2. **Include relevant fields**: Add meaningful fields to help with debugging and analysis
3. **Be consistent with log levels**:
   - DEBUG: Detailed information for debugging
   - INFO: General operational information
   - WARN: Situations that might cause issues
   - ERROR: Errors that prevent normal operation
   - FATAL: Critical errors that require shutdown
4. **Sanitize sensitive data**: Don't log passwords, tokens, or other sensitive information
5. **Use structured logging**: Avoid string concatenation or formatting in log messages