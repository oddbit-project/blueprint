# Blueprint Logging System

A structured, context-aware logging system for the Blueprint project that provides consistent logging patterns across different components.

## Overview

The Blueprint logging system builds on zerolog to provide:

- **Structured logging** with consistent field naming
- **Context propagation** for request tracing
- **Component-specific logging** (HTTP, Kafka, Database)
- **Performance-optimized** logging with minimal allocations
- **Trace and request ID** support for distributed systems

## Core Components

- **Logger**: The main logging interface with methods for each log level
- **Context**: Functions for context-aware logging and propagation

## Quick Start

```go
// Create a module logger
logger := log.New("myapp")

// Log at different levels
logger.Info("Application started", FV{
    "version": "1.0.0",
})

// Log errors with stack traces
if err := operation(); err != nil {
    logger.Error(err, "Operation failed", log.FV{
        "operation_id": 123,
    })
}
```

## Context-Aware Logging

```go
// Create a context with a logger
ctx, logger := log.NewRequestContext(context.Background(), "api")

// Log using context
log.Info(ctx, "Processing request")

// Pass context to other functions
processItem(ctx, item)

// Extract logger from context
func processItem(ctx context.Context, item Item) {
    logger := log.FromContext(ctx)
    logger.Info("Processing item", log.FV{
        "item_id": item.ID,
    })
}
```

## HTTP Integration

```go
// Add logging middleware to Gin router
router.Use(log.HTTPLogMiddleware("api"))

// Log in HTTP handlers
func handler(c *gin.Context) {
    log.RequestInfo(c, "Processing API request")
    
    // Log errors
    if err := operation(); err != nil {
        log.RequestError(c, err, "Failed to process request")
        c.AbortWithStatus(500)
        return
    }
}
```

## Configuration

Configure the logging system using:

```go
cfg := log.NewDefaultConfig()
cfg.Level = "info"
cfg.Format = "json"

err := log.Configure(cfg)
if err != nil {
    panic(err)
}
```

## Best Practices

1. Use context propagation to maintain request context across function calls
2. Include relevant fields but avoid logging sensitive information
3. Be consistent with log levels
4. Use structured logging instead of string concatenation
5. Properly handle errors and include them in log messages

## Complete Documentation

For detailed documentation, see [logging.md](../docs/log/logging.md).